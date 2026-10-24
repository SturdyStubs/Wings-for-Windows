package api

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"os"
	"path"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
	"wings-windows/internal/node"
)

const maxExtractStream int64 = 102 << 20

type extractEntry struct {
	name      string
	directory bool
	explicit  bool
	file      *os.File
	published bool
	stage     string
}

func (a *API) decompressFile(w http.ResponseWriter, r *http.Request, s *node.Server) {
	var input struct {
		Root string `json:"root"`
		File string `json:"file"`
	}
	if !decode(w, r, &input) {
		return
	}
	base := strings.Trim(input.Root, "/")
	if base != "" {
		parts, e := directoryParts("", base)
		if e != nil {
			failure(w, 422, "Invalid extraction root")
			return
		}
		base = strings.Join(parts, "/")
	}
	parts, e := directoryParts(base, input.File)
	if e != nil {
		failure(w, 422, "Invalid archive path")
		return
	}
	name := strings.Join(parts, "/")
	value, _ := a.archiveLocks.LoadOrStore(s.ID(), &sync.Mutex{})
	lock := value.(*sync.Mutex)
	if !lock.TryLock() {
		failure(w, 409, "Another archive operation is active for this server")
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
	if e = controller.SetWriteDeadline(time.Now().Add(65 * time.Second)); e != nil && !errors.Is(e, http.ErrNotSupported) {
		failure(w, 500, "Cannot prepare extraction response")
		return
	}
	check := func() error {
		if ctx.Err() != nil {
			return &directoryFailure{408, "Extraction cancelled or time limit reached"}
		}
		select {
		case <-a.done:
			return &directoryFailure{503, "Node is stopping"}
		default:
		}
		return nil
	}
	e = extractArchive(ctx, s, base, name, check)
	_ = controller.SetWriteDeadline(time.Now().Add(10 * time.Second))
	if e != nil {
		status, message := 400, "Archive extraction failed; reload the file listing"
		var p *directoryFailure
		if errors.As(e, &p) {
			status, message = p.status, strings.ReplaceAll(strings.ReplaceAll(p.message, "Rename", "Extraction"), "Copy", "Extraction")
		} else if os.IsNotExist(e) {
			status, message = 404, "Archive or extraction parent unavailable"
		} else if os.IsExist(e) {
			status, message = 409, "Extraction destination already exists"
		} else if copyDiskFull(e) {
			status, message = 507, "Insufficient storage for extraction"
		}
		failure(w, status, message)
		return
	}
	respond(w, 204, nil)
}

func extractArchive(ctx context.Context, s *node.Server, base, sourceName string, check func() error) (err error) {
	if err = check(); err != nil {
		return
	}
	root, e := os.OpenRoot(s.Root)
	if e != nil {
		return &directoryFailure{500, "Cannot open extraction data"}
	}
	defer root.Close()
	parent, e := renameParent(ctx, root, s, path.Join(base, ".extract-parent-check"))
	if e != nil {
		return e
	}
	defer parent.Close()
	parentName, e := resolvedFilePath(root, parent)
	if e != nil {
		return e
	}
	current := func(name string, dir bool) error {
		if e := check(); e != nil {
			return e
		}
		actual, e := resolvedFilePath(root, parent)
		if e != nil || !strings.EqualFold(actual, parentName) {
			return &directoryFailure{409, "Extraction parent changed"}
		}
		if renamePolicy(s, path.Join(base, name), dir) != nil || renamePolicy(s, path.Join(actual, name), dir) != nil {
			return &directoryFailure{403, "Extraction path is restricted"}
		}
		return nil
	}
	if e = renamePolicy(s, sourceName, false); e != nil {
		return &directoryFailure{403, "Archive source is restricted"}
	}
	sourceParent, e := renameParent(ctx, root, s, sourceName)
	if e != nil {
		return e
	}
	defer sourceParent.Close()
	source, e := openCopyHandle(sourceParent, path.Base(sourceName), false)
	if e != nil {
		if os.IsNotExist(e) {
			return e
		}
		return &directoryFailure{423, "Archive source is unavailable or locked"}
	}
	defer source.Close()
	if e = inspectRenameEntry(source); e != nil {
		return &directoryFailure{403, "Extraction refuses linked archive sources"}
	}
	info, e := source.Stat()
	if e != nil {
		return e
	}
	if !info.Mode().IsRegular() {
		return &directoryFailure{422, "A regular gzip tar archive is required"}
	}
	if info.Size() > maxArchiveOutput {
		return &directoryFailure{413, "Compressed archive exceeds 101 MiB"}
	}
	canonical, e := resolvedFilePath(root, source)
	if e != nil {
		return e
	}
	checkSource := func() error {
		if e := current(".extract-parent-check", false); e != nil {
			return e
		}
		actual, e := resolvedFilePath(root, source)
		if e != nil || !strings.EqualFold(actual, canonical) {
			return &directoryFailure{409, "Archive source changed"}
		}
		if renamePolicy(s, sourceName, false) != nil || renamePolicy(s, actual, false) != nil {
			return &directoryFailure{403, "Archive source is restricted"}
		}
		return nil
	}
	reader := bufio.NewReader(downloadReader{r: source, check: checkSource})
	z, e := gzip.NewReader(reader)
	if e != nil {
		return &directoryFailure{400, "Expected a valid gzip tar archive"}
	}
	defer z.Close()
	z.Multistream(false)
	decoded := &extractBudget{reader: z, remaining: maxExtractStream, check: checkSource}
	tr := tar.NewReader(decoded)
	var entries []*extractEntry
	index := map[string]*extractEntry{}
	defer func() {
		for i := len(entries) - 1; i >= 0; i-- {
			entry := entries[i]
			if entry.file == nil {
				continue
			}
			if !entry.published {
				if discardCopyStage(entry.file) != nil {
					err = &directoryFailure{500, "Extraction failed; an owned temporary file may remain"}
				}
			}
			entry.file.Close()
		}
	}()
	add := func(name string, dir, explicit bool) (*extractEntry, error) {
		key := strings.ToLower(name)
		if old := index[key]; old != nil {
			if old.name != name || old.directory != dir || !dir || explicit && old.explicit {
				return nil, &directoryFailure{422, "Duplicate, aliased or conflicting archive paths"}
			}
			old.explicit = old.explicit || explicit
			return old, nil
		}
		if len(entries) >= maxArchiveEntries {
			return nil, &directoryFailure{413, "Extraction exceeds 256 paths including parents"}
		}
		if e := current(name, dir); e != nil {
			return nil, e
		}
		entry := &extractEntry{name: name, directory: dir, explicit: explicit}
		index[key] = entry
		entries = append(entries, entry)
		return entry, nil
	}
	var total int64
	headers := 0
	buf := make([]byte, 64<<10)
	for {
		if e = checkSource(); e != nil {
			return e
		}
		beforeHeader := decoded.used
		h, e := tr.Next()
		if e == io.EOF {
			if decoded.used-beforeHeader < 1024 {
				return &directoryFailure{400, "Tar end markers are missing"}
			}
			break
		}
		if e != nil {
			return e
		}
		headers++
		if headers > maxArchiveEntries {
			return &directoryFailure{413, "Extraction exceeds 256 archive headers"}
		}
		dir := h.Typeflag == tar.TypeDir
		if !dir && h.Typeflag != tar.TypeReg && h.Typeflag != tar.TypeRegA {
			return &directoryFailure{422, "Only regular files and directories may be extracted"}
		}
		if h.Linkname != "" || h.Size < 0 || dir && h.Size != 0 {
			return &directoryFailure{422, "Invalid archive entry"}
		}
		for key := range h.PAXRecords {
			if strings.HasPrefix(key, "GNU.sparse.") {
				return &directoryFailure{422, "Sparse archive records are unsupported"}
			}
		}
		name := h.Name
		if dir {
			name = strings.TrimSuffix(name, "/")
		}
		if name == "" || strings.HasPrefix(name, "/") || !utf8.ValidString(name) {
			return &directoryFailure{422, "Invalid archive entry path"}
		}
		parts, e := directoryParts("", name)
		if e != nil || strings.Join(parts, "/") != name {
			return &directoryFailure{422, "Unsafe Windows archive path"}
		}
		if _, e = directoryParts(base, name); e != nil {
			return &directoryFailure{422, "Extraction path exceeds Windows bounds"}
		}
		for i := 1; i < len(parts); i++ {
			if _, e = add(strings.Join(parts[:i], "/"), true, false); e != nil {
				return e
			}
		}
		entry, e := add(name, dir, true)
		if e != nil {
			return e
		}
		if dir {
			continue
		}
		if h.Size > maxArchiveInput-total {
			return &directoryFailure{413, "Expanded files exceed 100 MiB aggregate"}
		}
		total += h.Size
		var random [16]byte
		if _, e = rand.Read(random[:]); e != nil {
			return e
		}
		entry.stage = ".wings-extract-" + hex.EncodeToString(random[:]) + ".tmp"
		if e = current(entry.stage, false); e != nil {
			return e
		}
		entry.file, e = openCopyHandle(parent, entry.stage, true)
		if e != nil {
			return e
		}
		n, e := io.CopyBuffer(copyWriter{ctx: ctx, w: entry.file}, downloadReader{r: tr, check: func() error { return current(entry.name, false) }}, buf)
		if e != nil {
			return e
		}
		if n != h.Size {
			return io.ErrUnexpectedEOF
		}
		if e = entry.file.Sync(); e != nil {
			return e
		}
	}
	if headers == 0 {
		return &directoryFailure{400, "Archive contains no extractable entries"}
	}
	// Fully consume the single gzip member: verify CRC/truncation and permit only
	// zero tar padding after EOF. Refuse additional members or trailing payload.
	if _, e = io.CopyBuffer(extractPadding{}, decoded, buf); e != nil {
		return e
	}
	if _, e = reader.Peek(1); e != io.EOF {
		if e != nil {
			return e
		}
		return &directoryFailure{422, "Trailing gzip members or bytes are unsupported"}
	}
	for _, entry := range entries {
		if e = checkSource(); e != nil {
			return e
		}
		if e = current(entry.name, entry.directory); e != nil {
			return e
		}
		if !strings.Contains(entry.name, "/") {
			if _, e = root.Lstat(path.Join(parentName, entry.name)); e == nil {
				return &directoryFailure{409, "Extraction destination already exists; use an empty destination"}
			} else if !os.IsNotExist(e) {
				return e
			}
		}
	}
	if e = checkSource(); e != nil {
		return e
	}
	sort.SliceStable(entries, func(i, j int) bool {
		a, b := strings.Count(entries[i].name, "/"), strings.Count(entries[j].name, "/")
		if a != b {
			return a < b
		}
		return entries[i].name < entries[j].name
	})
	for _, entry := range entries {
		if e = checkSource(); e != nil {
			return e
		}
		if e = current(entry.name, entry.directory); e != nil {
			return e
		}
		destination := parent
		if name := path.Dir(entry.name); name != "." {
			owner := index[strings.ToLower(name)]
			if owner == nil || !owner.directory || !owner.published {
				return &directoryFailure{500, "Extraction parent unavailable"}
			}
			destination = owner.file
			actual, e := resolvedFilePath(root, destination)
			if e != nil || !strings.EqualFold(actual, path.Join(parentName, name)) {
				return &directoryFailure{409, "Extraction directory changed"}
			}
		}
		if entry.directory {
			entry.file, e = createExtractDirectory(destination, path.Base(entry.name))
			if e != nil {
				return e
			}
		} else {
			if e = current(entry.stage, false); e != nil {
				return e
			}
			actual, e := resolvedFilePath(root, entry.file)
			if e != nil || !strings.EqualFold(actual, path.Join(parentName, entry.stage)) {
				return &directoryFailure{409, "Extraction staging changed"}
			}
			if e = renameNoReplace(entry.file, destination, path.Base(entry.name)); e != nil {
				return e
			}
		}
		entry.published = true
	}
	return nil
}

type extractBudget struct {
	reader    io.Reader
	remaining int64
	used      int64
	check     func() error
}

func (r *extractBudget) Read(p []byte) (int, error) {
	if e := r.check(); e != nil {
		return 0, e
	}
	if int64(len(p)) > r.remaining+1 {
		p = p[:r.remaining+1]
	}
	n, e := r.reader.Read(p)
	r.used += int64(n)
	r.remaining -= int64(n)
	if r.remaining < 0 {
		return 0, &directoryFailure{413, "Decoded tar stream exceeds 102 MiB"}
	}
	return n, e
}

type extractPadding struct{}

func (extractPadding) Write(p []byte) (int, error) {
	for _, b := range p {
		if b != 0 {
			return 0, &directoryFailure{422, "Nonzero trailing tar payload is unsupported"}
		}
	}
	return len(p), nil
}
