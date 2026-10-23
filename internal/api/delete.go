package api

import (
	"context"
	"io"
	"net/http"
	"os"
	"path"
	"strings"
	"sync"
	"wings-windows/internal/node"
)

const maxDeleteEntries = 1024

type deleteEntry struct {
	file      *os.File
	canonical string
	directory bool
	children  []*deleteEntry
}

func (p *deleteEntry) close() {
	if p == nil {
		return
	}
	for _, child := range p.children {
		child.close()
	}
	if p.file != nil {
		p.file.Close()
		p.file = nil
	}
}

func (a *API) deleteFiles(w http.ResponseWriter, r *http.Request, s *node.Server) {
	var input struct {
		Root  string   `json:"root"`
		Files []string `json:"files"`
	}
	if !decode(w, r, &input) {
		return
	}
	if len(input.Files) < 1 || len(input.Files) > 100 {
		failure(w, 422, "Provide between 1 and 100 delete entries")
		return
	}
	names := make([]string, len(input.Files))
	for i, name := range input.Files {
		parts, err := directoryParts(input.Root, name)
		if err != nil {
			failure(w, 422, "Invalid delete path; server root and traversal are forbidden")
			return
		}
		names[i] = strings.Join(parts, "/")
		for j := 0; j < i; j++ {
			if pathOverlaps(names[i], names[j]) {
				failure(w, 422, "Delete entries overlap; select each tree only once")
				return
			}
		}
	}
	value, _ := a.deleteLocks.LoadOrStore(s.ID(), &sync.Mutex{})
	lock := value.(*sync.Mutex)
	if !lock.TryLock() {
		failure(w, 409, "Another delete batch is active for this server")
		return
	}
	defer lock.Unlock()
	root, err := os.OpenRoot(s.Root)
	if err != nil {
		failure(w, 500, "Cannot open server directory")
		return
	}
	defer root.Close()
	plans := make([]*deleteEntry, 0, len(names))
	parents := make([]*os.File, 0, len(names))
	defer func() {
		for _, p := range plans {
			p.close()
		}
		for _, p := range parents {
			p.Close()
		}
	}()
	remaining := maxDeleteEntries
	for _, name := range names {
		if r.Context().Err() != nil {
			deleteFailure(w, &directoryFailure{408, "Delete request cancelled"}, false)
			return
		}
		if err = renamePolicy(s, name, false); err != nil {
			deleteFailure(w, err, false)
			return
		}
		parent, e := renameParent(r.Context(), root, s, name)
		if os.IsNotExist(e) {
			continue
		}
		if e != nil {
			deleteFailure(w, e, false)
			return
		}
		parents = append(parents, parent)
		p, e := pinDeleteEntry(r.Context(), root, s, parent, path.Base(name), name, 0, &remaining)
		if e != nil {
			deleteFailure(w, e, false)
			return
		}
		if p == nil {
			continue
		}
		plans = append(plans, p)
		for _, q := range plans[:len(plans)-1] {
			if pathOverlaps(p.canonical, q.canonical) {
				failure(w, 422, "Resolved delete entries overlap")
				return
			}
		}
	}
	for _, p := range plans {
		if err = applyDeletion(r.Context(), root, s, p); err != nil {
			deleteFailure(w, err, true)
			return
		}
	}
	respond(w, 204, nil)
}

// All objects in the bounded batch are pinned and validated before mutation.
// Children are opened relative to their already checked directory handle.
func pinDeleteEntry(ctx context.Context, root *os.Root, s *node.Server, parent *os.File, leaf, requested string, depth int, remaining *int) (p *deleteEntry, err error) {
	if ctx.Err() != nil {
		return nil, &directoryFailure{408, "Delete request cancelled"}
	}
	if depth > 32 {
		return nil, &directoryFailure{413, "Delete tree exceeds 32 levels"}
	}
	if *remaining <= 0 {
		return nil, &directoryFailure{413, "Delete scan exceeds 1024 entries"}
	}
	if _, err = directoryParts("", requested); err != nil {
		return nil, &directoryFailure{422, "Delete tree contains unsupported Windows paths"}
	}
	if err = renamePolicy(s, requested, false); err != nil {
		return nil, err
	}
	info, err := root.Lstat(requested)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if fileIsReparse(info) {
		return nil, &directoryFailure{403, "Delete refuses links and reparse points"}
	}
	f, err := openRenameSource(parent, leaf)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	p = &deleteEntry{file: f}
	defer func() {
		if err != nil {
			p.close()
			p = nil
		}
	}()
	if err = inspectRenameEntry(f); err != nil {
		return p, err
	}
	info, err = f.Stat()
	if err != nil {
		return p, err
	}
	if !info.IsDir() && !info.Mode().IsRegular() {
		return p, &directoryFailure{422, "Delete supports regular files and directories only"}
	}
	p.directory = info.IsDir()
	p.canonical, err = resolvedFilePath(root, f)
	if err != nil {
		return p, err
	}
	if p.canonical == "." {
		return p, &directoryFailure{422, "Server root cannot be deleted"}
	}
	if err = renamePolicy(s, requested, p.directory); err != nil {
		return p, err
	}
	if err = renamePolicy(s, p.canonical, p.directory); err != nil {
		return p, err
	}
	*remaining--
	if !p.directory {
		return p, nil
	}
	entries, e := f.ReadDir(*remaining + 1)
	if e != nil && e != io.EOF {
		return p, e
	}
	if len(entries) > *remaining {
		return p, &directoryFailure{413, "Delete scan exceeds 1024 entries"}
	}
	for _, entry := range entries {
		if !validRenameLeaf(entry.Name()) {
			return p, &directoryFailure{422, "Delete tree contains an unsupported Windows name"}
		}
		child, e := pinDeleteEntry(ctx, root, s, f, entry.Name(), path.Join(p.canonical, entry.Name()), depth+1, remaining)
		if e != nil {
			return p, e
		}
		if child != nil {
			p.children = append(p.children, child)
		}
	}
	return p, nil
}

func applyDeletion(ctx context.Context, root *os.Root, s *node.Server, p *deleteEntry) error {
	if ctx.Err() != nil {
		return &directoryFailure{408, "Delete request cancelled"}
	}
	current, err := resolvedFilePath(root, p.file)
	if err != nil {
		return err
	}
	if !strings.EqualFold(current, p.canonical) {
		return &directoryFailure{409, "Delete namespace changed; reload the file listing"}
	}
	if err = renamePolicy(s, current, p.directory); err != nil {
		return err
	}
	for _, child := range p.children {
		if err = applyDeletion(ctx, root, s, child); err != nil {
			return err
		}
	}
	// No recursive path fallback or re-scan: a late new child makes a folder
	// nonempty and prevents its deletion, preserving that unvalidated child.
	if err = deletePinnedEntry(p.file); err != nil {
		return err
	}
	err = p.file.Close()
	p.file = nil
	return err
}

func deleteFailure(w http.ResponseWriter, err error, partial bool) {
	status, message := deleteErrorStatus(err)
	if p, ok := err.(*directoryFailure); ok {
		status = p.status
		message = strings.ReplaceAll(p.message, "Rename", "Delete")
	}
	if partial {
		message += "; earlier deletions may have completed"
	}
	failure(w, status, message)
}

func deleteErrorStatus(err error) (int, string) {
	if os.IsPermission(err) {
		return 403, "Delete access denied"
	}
	if status := deleteWindowsStatus(err); status != 0 {
		return status, "Delete blocked by a locked file or changed directory"
	}
	return 422, "Cannot delete pinned server data"
}
