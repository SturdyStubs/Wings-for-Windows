package api

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
	"wings-windows/internal/node"
)

const maxArchiveEntries = 256
const maxArchiveInput int64 = 100 << 20
const maxArchiveOutput int64 = 101 << 20

type archiveEntry struct {
	file      *os.File
	canonical string
	name      string
	info      os.FileInfo
}

func (a *API) compressFiles(w http.ResponseWriter, r *http.Request, s *node.Server) {
	var input struct {
		Root  string   `json:"root"`
		Files []string `json:"files"`
	}
	if !decode(w, r, &input) {
		return
	}
	if len(input.Files) < 1 || len(input.Files) > 100 {
		failure(w, 422, "Select between 1 and 100 archive paths")
		return
	}
	base := strings.Trim(input.Root, "/")
	if !utf8.ValidString(base) {
		failure(w, 422, "Archive root must be valid UTF-8")
		return
	}
	if base == "" {
		base = "."
	} else if parts, e := directoryParts("", base); e != nil {
		failure(w, 422, "Invalid archive root")
		return
	} else {
		base = strings.Join(parts, "/")
	}
	names := make([]string, len(input.Files))
	for i, value := range input.Files {
		parts, e := directoryParts("", value)
		if e != nil || !utf8.ValidString(value) {
			failure(w, 422, "Invalid archive selection")
			return
		}
		names[i] = strings.Join(parts, "/")
		if _, e = directoryParts(input.Root, names[i]); e != nil {
			failure(w, 422, "Archive selection exceeds path bounds")
			return
		}
		for j := 0; j < i; j++ {
			if pathOverlaps(names[i], names[j]) {
				failure(w, 422, "Archive selections overlap; select each tree once")
				return
			}
		}
	}
	value, _ := a.archiveLocks.LoadOrStore(s.ID(), &sync.Mutex{})
	lock := value.(*sync.Mutex)
	if !lock.TryLock() {
		failure(w, 409, "Another archive is active for this server")
		return
	}
	defer lock.Unlock()
	select {
	case a.archiveSlots <- struct{}{}:
		defer func() { <-a.archiveSlots }()
	default:
		failure(w, 503, "Archive capacity reached")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	controller := http.NewResponseController(w)
	if e := controller.SetWriteDeadline(time.Now().Add(65 * time.Second)); e != nil && !errors.Is(e, http.ErrNotSupported) {
		failure(w, 500, "Cannot prepare archive response")
		return
	}
	check := func() error {
		if ctx.Err() != nil {
			return &directoryFailure{408, "Archive cancelled or time limit reached"}
		}
		select {
		case <-a.done:
			return &directoryFailure{503, "Node is stopping"}
		default:
		}
		return nil
	}
	metadata, err := createArchive(ctx, s, base, names, check)
	_ = controller.SetWriteDeadline(time.Now().Add(10 * time.Second))
	if err != nil {
		status, message := 422, "Archive could not be completed"
		var p *directoryFailure
		if errors.As(err, &p) {
			status, message = p.status, strings.ReplaceAll(strings.ReplaceAll(p.message, "Rename", "Archive"), "Copy", "Archive")
		} else if copyDiskFull(err) {
			status, message = 507, "Insufficient storage for archive"
		} else if os.IsNotExist(err) {
			status, message = 404, "Archive source or parent unavailable"
		} else if os.IsExist(err) {
			status, message = 409, "Archive destination already exists"
		}
		failure(w, status, message)
		return
	}
	respond(w, 200, metadata)
}

func createArchive(ctx context.Context, s *node.Server, base string, names []string, check func() error) (metadata fileMetadata, err error) {
	if err = check(); err != nil {
		return
	}
	root, e := os.OpenRoot(s.Root)
	if e != nil {
		err = &directoryFailure{500, "Cannot open archive data"}
		return
	}
	defer root.Close()
	parent, e := renameParent(ctx, root, s, path.Join(base, ".archive-parent-check"))
	if e != nil {
		err = e
		return
	}
	defer parent.Close()
	parentName, e := resolvedFilePath(root, parent)
	if e != nil {
		err = e
		return
	}
	current := func(name string, dir bool) error {
		if e := check(); e != nil {
			return e
		}
		actual, e := resolvedFilePath(root, parent)
		if e != nil || !strings.EqualFold(actual, parentName) {
			return &directoryFailure{409, "Archive parent changed"}
		}
		if renamePolicy(s, path.Join(base, name), dir) != nil || renamePolicy(s, path.Join(actual, name), dir) != nil {
			return &directoryFailure{403, "Archive path is restricted"}
		}
		return nil
	}
	var entries []*archiveEntry
	defer func() {
		for _, entry := range entries {
			entry.file.Close()
		}
	}()
	var total int64
	var pin func(*os.File, string, string, int) error
	pin = func(dir *os.File, leaf, relative string, depth int) error {
		if e := current(relative, false); e != nil {
			return e
		}
		if depth > 32 || len(entries) >= maxArchiveEntries {
			return &directoryFailure{413, "Archive exceeds 256 entries or path depth"}
		}
		if !utf8.ValidString(relative) {
			return &directoryFailure{422, "Archive names must be valid UTF-8"}
		}
		validationBase := base
		if validationBase == "." {
			validationBase = ""
		}
		if _, e := directoryParts(validationBase, relative); e != nil {
			return &directoryFailure{422, "Archive source exceeds Windows path bounds"}
		}
		f, e := openCopyHandle(dir, leaf, false)
		if e != nil {
			if os.IsNotExist(e) {
				return e
			}
			return &directoryFailure{423, "Archive source is unavailable or locked"}
		}
		owned := false
		defer func() {
			if !owned {
				f.Close()
			}
		}()
		if e = inspectRenameEntry(f); e != nil {
			return &directoryFailure{403, "Archives refuse links, reparse points and multi-hardlink files"}
		}
		info, e := f.Stat()
		if e != nil {
			return e
		}
		if !info.IsDir() && !info.Mode().IsRegular() {
			return &directoryFailure{422, "Only regular files and directories can be archived"}
		}
		canonical, e := resolvedFilePath(root, f)
		if e != nil {
			return e
		}
		if !strings.EqualFold(canonical, path.Join(parentName, relative)) {
			return &directoryFailure{403, "Archive source path changed"}
		}
		if e = current(relative, info.IsDir()); e != nil {
			return e
		}
		entry := &archiveEntry{file: f, canonical: canonical, name: relative, info: info}
		entries = append(entries, entry)
		owned = true
		if !info.IsDir() {
			if info.Size() < 0 || info.Size() > maxArchiveInput-total {
				return &directoryFailure{413, "Archive sources exceed 100 MiB aggregate"}
			}
			total += info.Size()
			return nil
		}
		for {
			children, readErr := f.ReadDir(32)
			if readErr != nil && readErr != io.EOF {
				return readErr
			}
			for _, child := range children {
				if !validRenameLeaf(child.Name()) {
					return &directoryFailure{422, "Invalid archive child name"}
				}
				if e = pin(f, child.Name(), path.Join(relative, child.Name()), depth+1); e != nil {
					return e
				}
			}
			if readErr == io.EOF {
				break
			}
		}
		return nil
	}
	for _, name := range names {
		if e = current(name, false); e != nil {
			err = e
			return
		}
		sourceParent, e := renameParent(ctx, root, s, path.Join(base, name))
		if e != nil {
			err = e
			return
		}
		e = pin(sourceParent, path.Base(name), name, 0)
		sourceParent.Close()
		if e != nil {
			err = e
			return
		}
	}
	var random [16]byte
	if _, err = rand.Read(random[:]); err != nil {
		return
	}
	id := hex.EncodeToString(random[:])
	stageName := ".wings-archive-" + id + ".tmp"
	finalName := "archive-" + time.Now().UTC().Format("2006-01-02T150405Z") + "-" + id[:12] + ".tar.gz"
	for _, name := range []string{stageName, finalName} {
		if err = current(name, false); err != nil {
			return
		}
	}
	stage, e := openCopyHandle(parent, stageName, true)
	if e != nil {
		err = e
		return
	}
	published := false
	defer func() {
		if !published {
			if discardCopyStage(stage) != nil {
				err = &directoryFailure{500, "Archive failed; an owned temporary archive may remain"}
			}
		}
		stage.Close()
	}()
	output := &archiveOutput{writer: stage, remaining: maxArchiveOutput, check: func() error { return current(finalName, false) }}
	zipper := gzip.NewWriter(output)
	writer := tar.NewWriter(zipper)
	checkEntry := func(entry *archiveEntry) error {
		if e := current(entry.name, entry.info.IsDir()); e != nil {
			return e
		}
		actual, e := resolvedFilePath(root, entry.file)
		if e != nil || !strings.EqualFold(actual, entry.canonical) {
			return &directoryFailure{409, "Archive source changed"}
		}
		return nil
	}
	for _, entry := range entries {
		if err = checkEntry(entry); err != nil {
			return
		}
		header := &tar.Header{Name: entry.name, Mode: 0644, ModTime: entry.info.ModTime().UTC(), Typeflag: tar.TypeReg, Size: entry.info.Size(), Format: tar.FormatPAX}
		if entry.info.IsDir() {
			header.Name += "/"
			header.Mode = 0755
			header.Typeflag = tar.TypeDir
			header.Size = 0
		}
		if err = writer.WriteHeader(header); err != nil {
			return
		}
		if !entry.info.IsDir() {
			checkSource := func() error { return checkEntry(entry) }
			err = copyBytes(ctx, downloadReader{r: entry.file, check: checkSource}, writer, entry.info.Size())
			if err != nil {
				return
			}
		}
	}
	if err = writer.Close(); err != nil {
		return
	}
	if err = zipper.Close(); err != nil {
		return
	}
	if err = stage.Sync(); err != nil {
		return
	}
	// Refresh every source's policy before making the archive visible.
	for _, entry := range entries {
		if err = checkEntry(entry); err != nil {
			return
		}
	}
	if err = current(stageName, false); err != nil {
		return
	}
	if err = current(finalName, false); err != nil {
		return
	}
	info, e := stage.Stat()
	if e != nil {
		err = e
		return
	}
	if err = renameNoReplace(stage, parent, finalName); err != nil {
		return
	}
	published = true
	metadata = fileMetadata{Name: finalName, Mode: info.Mode().String(), ModeBits: fmt.Sprintf("%o", info.Mode().Perm()), Size: info.Size(), File: true, Mime: "application/tar+gzip", Created: fileCreated(info).UTC().Format(time.RFC3339), Modified: info.ModTime().UTC().Format(time.RFC3339)}
	return
}

type archiveOutput struct {
	writer    io.Writer
	remaining int64
	check     func() error
}

func (w *archiveOutput) Write(p []byte) (int, error) {
	if err := w.check(); err != nil {
		return 0, err
	}
	if int64(len(p)) > w.remaining {
		return 0, &directoryFailure{413, "Compressed archive exceeds 101 MiB"}
	}
	n, err := w.writer.Write(p)
	w.remaining -= int64(n)
	return n, err
}
