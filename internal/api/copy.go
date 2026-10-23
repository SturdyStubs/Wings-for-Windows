package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strings"
	"sync"
	"wings-windows/internal/node"
)

const maxCopyBytes int64 = 1 << 30

func copyName(name string, index int) string {
	ext := path.Ext(name)
	base := strings.TrimSuffix(name, ext)
	if strings.HasSuffix(base, ".tar") {
		base = strings.TrimSuffix(base, ".tar")
		ext = ".tar" + ext
	}
	suffix := " copy"
	if index > 0 {
		suffix = fmt.Sprintf(" copy %d", index)
	}
	return base + suffix + ext
}

func (a *API) copyFile(w http.ResponseWriter, r *http.Request, s *node.Server) {
	var input struct {
		Location string `json:"location"`
	}
	if !decode(w, r, &input) {
		return
	}
	parts, err := directoryParts("", strings.Trim(input.Location, "/"))
	if err != nil {
		failure(w, 422, "Invalid copy source path")
		return
	}
	value, _ := a.copyLocks.LoadOrStore(s.ID(), &sync.Mutex{})
	lock := value.(*sync.Mutex)
	if !lock.TryLock() {
		failure(w, 409, "Another file copy is active for this server")
		return
	}
	defer lock.Unlock()
	root, err := os.OpenRoot(s.Root)
	if err != nil {
		failure(w, 500, "Cannot open server directory")
		return
	}
	defer root.Close()
	if err = copyRegularFile(r.Context(), root, s, strings.Join(parts, "/")); err != nil {
		copyFailure(w, err)
		return
	}
	respond(w, 204, nil)
}

func copyFailure(w http.ResponseWriter, err error) {
	if p, ok := err.(*directoryFailure); ok {
		failure(w, p.status, strings.ReplaceAll(p.message, "Rename", "Copy"))
		return
	}
	status, message := 422, "Cannot complete copy inside server data"
	if os.IsNotExist(err) {
		status, message = 404, "Copy source or parent is unavailable"
	}
	if copyDiskFull(err) {
		status, message = 507, "Insufficient storage for file copy"
	}
	failure(w, status, message)
}

func copyRegularFile(ctx context.Context, root *os.Root, s *node.Server, name string) (err error) {
	if ctx.Err() != nil {
		return &directoryFailure{408, "Copy request cancelled"}
	}
	if err = renamePolicy(s, name, false); err != nil {
		return err
	}
	parent, err := renameParent(ctx, root, s, name)
	if err != nil {
		return err
	}
	defer parent.Close()
	info, err := root.Lstat(name)
	if err != nil {
		return err
	}
	if fileIsReparse(info) {
		return &directoryFailure{403, "Copy refuses links and reparse points"}
	}
	if !info.Mode().IsRegular() {
		return &directoryFailure{404, "Only regular files can be copied"}
	}
	source, err := openCopyHandle(parent, path.Base(name), false)
	if err != nil {
		return err
	}
	defer source.Close()
	if err = inspectRenameEntry(source); err != nil {
		return err
	}
	info, err = source.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return &directoryFailure{404, "Only regular files can be copied"}
	}
	if info.Size() < 0 || info.Size() > maxCopyBytes {
		return &directoryFailure{413, "File copy supports sources up to 1 GiB"}
	}
	canonical, err := resolvedFilePath(root, source)
	if err != nil {
		return err
	}
	if err = renamePolicy(s, canonical, false); err != nil {
		return err
	}
	parentName := path.Dir(canonical)
	base := path.Base(canonical)
	index, candidate, err := findCopyName(root, s, parentName, base, 0)
	if err != nil {
		return err
	}
	var random [16]byte
	if _, err = rand.Read(random[:]); err != nil {
		return err
	}
	stageName := ".wings-copy-" + hex.EncodeToString(random[:]) + ".tmp"
	if err = renamePolicy(s, path.Join(parentName, stageName), false); err != nil {
		return err
	}
	stage, err := openCopyHandle(parent, stageName, true)
	if err != nil {
		return err
	}
	published := false
	defer func() {
		if !published {
			if cleanupErr := discardCopyStage(stage); cleanupErr != nil {
				err = &directoryFailure{500, "Copy failed; an owned temporary copy may remain"}
			}
		}
		stage.Close()
	}()
	if err = copyBytes(ctx, source, stage, info.Size()); err != nil {
		return err
	}
	if err = stage.Sync(); err != nil {
		return err
	}
	for {
		if ctx.Err() != nil {
			return &directoryFailure{408, "Copy cancelled before publication"}
		}
		currentParent, checkErr := resolvedFilePath(root, parent)
		if checkErr != nil {
			return checkErr
		}
		if !strings.EqualFold(currentParent, parentName) {
			return &directoryFailure{409, "Copy parent changed; reload the file listing"}
		}
		if err = renamePolicy(s, path.Join(currentParent, candidate), false); err != nil {
			return err
		}
		if err = renameNoReplace(stage, parent, candidate); err == nil {
			published = true
			return nil
		}
		if !os.IsExist(err) {
			return err
		}
		index, candidate, err = findCopyName(root, s, parentName, base, index+1)
		if err != nil {
			return err
		}
	}
}

func findCopyName(root *os.Root, s *node.Server, parent, base string, start int) (int, string, error) {
	for i := start; i <= 50; i++ {
		name := copyName(base, i)
		if !validRenameLeaf(name) {
			return 0, "", &directoryFailure{422, "Generated copy name exceeds Windows filename bounds"}
		}
		target := path.Join(parent, name)
		if err := renamePolicy(s, target, false); err != nil {
			return 0, "", err
		}
		if _, err := root.Lstat(target); os.IsNotExist(err) {
			return i, name, nil
		} else if err != nil {
			return 0, "", err
		}
	}
	return 0, "", &directoryFailure{409, "All 51 supported copy names are occupied"}
}

type copyReader struct {
	ctx context.Context
	r   io.Reader
}

func (r copyReader) Read(p []byte) (int, error) {
	if r.ctx.Err() != nil {
		return 0, &directoryFailure{408, "Copy cancelled"}
	}
	return r.r.Read(p)
}

type copyWriter struct {
	ctx context.Context
	w   io.Writer
}

func (w copyWriter) Write(p []byte) (int, error) {
	if w.ctx.Err() != nil {
		return 0, &directoryFailure{408, "Copy cancelled"}
	}
	return w.w.Write(p)
}
func copyBytes(ctx context.Context, source io.Reader, destination io.Writer, size int64) error {
	r := copyReader{ctx, source}
	n, err := io.CopyBuffer(copyWriter{ctx, destination}, io.LimitReader(r, size), make([]byte, 64<<10))
	if err != nil {
		return err
	}
	if n != size {
		return io.ErrUnexpectedEOF
	}
	var extra [1]byte
	nExtra, err := r.Read(extra[:])
	if nExtra != 0 {
		return fmt.Errorf("source grew during copy")
	}
	if err != io.EOF {
		if err != nil {
			return err
		}
		return io.ErrNoProgress
	}
	return nil
}
