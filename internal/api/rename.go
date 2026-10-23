package api

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strings"
	"sync"
	"wings-windows/internal/node"
)

type renamePair struct {
	From string `json:"from"`
	To   string `json:"to"`
}
type renamePlan struct {
	from, to                   string
	canonicalFrom, canonicalTo string
	source, parent             *os.File
	leaf                       string
	noop                       bool
}

func validRenameLeaf(leaf string) bool {
	parts, err := directoryParts("", leaf)
	return err == nil && len(parts) == 1 && parts[0] == leaf
}

// The panel's move dialog accepts paths relative to its current directory,
// including ../ within the server. Validate components before lexical cleanup.
func renamePath(root, value string) (string, error) {
	if strings.Trim(value, "/") == "" || len(root)+len(value) > 4096 {
		return "", fmt.Errorf("a bounded non-root path is required")
	}
	parts := strings.Split(strings.Trim(root, "/"), "/")
	if strings.Trim(root, "/") == "" {
		parts = nil
	} else {
		if _, err := directoryParts("", strings.Trim(root, "/")); err != nil {
			return "", err
		}
	}
	raw := strings.Split(strings.Trim(value, "/"), "/")
	if len(raw)+len(parts) > 64 {
		return "", fmt.Errorf("too many input components")
	}
	for _, part := range raw {
		switch part {
		case ".":
			continue
		case "..":
			if len(parts) == 0 {
				return "", fmt.Errorf("path escapes server root")
			}
			parts = parts[:len(parts)-1]
		default:
			if _, err := directoryParts("", part); err != nil {
				return "", err
			}
			parts = append(parts, part)
		}
	}
	if len(parts) == 0 {
		return "", fmt.Errorf("server root cannot be renamed")
	}
	result := strings.Join(parts, "/")
	if _, err := directoryParts("", result); err != nil {
		return "", err
	}
	return result, nil
}

func pathOverlaps(a, b string) bool {
	a, b = strings.ToLower(a), strings.ToLower(b)
	return a == b || strings.HasPrefix(a, b+"/") || strings.HasPrefix(b, a+"/")
}

func renamePolicy(s *node.Server, name string, dir bool) error {
	parts := strings.Split(name, "/")
	for i := range parts {
		if s.FileDenied(strings.Join(parts[:i+1], "/"), i < len(parts)-1 || dir) {
			return &directoryFailure{403, "Rename path is restricted by the egg denylist"}
		}
	}
	return nil
}

func (a *API) renameFiles(w http.ResponseWriter, r *http.Request, s *node.Server) {
	var input struct {
		Root  string       `json:"root"`
		Files []renamePair `json:"files"`
	}
	if !decode(w, r, &input) {
		return
	}
	if len(input.Files) == 0 || len(input.Files) > 100 {
		failure(w, 422, "Provide between 1 and 100 rename entries")
		return
	}
	plans := make([]renamePlan, len(input.Files))
	for i, pair := range input.Files {
		from, err := renamePath(input.Root, pair.From)
		if err != nil {
			failure(w, 422, "Invalid rename source path")
			return
		}
		to, err := renamePath(input.Root, pair.To)
		if err != nil {
			failure(w, 422, "Invalid rename destination path")
			return
		}
		if !strings.EqualFold(from, to) && pathOverlaps(from, to) {
			failure(w, 422, "Cannot move a path into itself or an ancestor")
			return
		}
		for j := 0; j < i; j++ {
			for _, previous := range []string{plans[j].from, plans[j].to} {
				if pathOverlaps(from, previous) || pathOverlaps(to, previous) {
					failure(w, 422, "Rename entries overlap; save each dependent move separately")
					return
				}
			}
		}
		plans[i] = renamePlan{from: from, to: to, leaf: path.Base(to), noop: from == to}
	}
	lockValue, _ := a.renameLocks.LoadOrStore(s.ID(), &sync.Mutex{})
	lock := lockValue.(*sync.Mutex)
	if !lock.TryLock() {
		failure(w, 409, "Another rename batch is active for this server")
		return
	}
	defer lock.Unlock()
	root, err := os.OpenRoot(s.Root)
	if err != nil {
		failure(w, 500, "Cannot open server directory")
		return
	}
	defer root.Close()
	defer func() {
		for _, p := range plans {
			if p.source != nil {
				p.source.Close()
			}
			if p.parent != nil {
				p.parent.Close()
			}
		}
	}()
	remaining := 10000
	for i := range plans {
		if err = prepareRename(r.Context(), root, s, &plans[i], &remaining); err != nil {
			renameFailure(w, err, false)
			return
		}
	}
	// Resolve aliases before accepting the batch: distinct requested paths can
	// refer to overlapping physical namespaces (for example NTFS short names).
	for i, p := range plans {
		if p.source == nil {
			continue
		}
		for j := 0; j < i; j++ {
			q := plans[j]
			if q.source == nil {
				continue
			}
			for _, previous := range []string{q.canonicalFrom, q.canonicalTo} {
				if pathOverlaps(p.canonicalFrom, previous) || pathOverlaps(p.canonicalTo, previous) {
					failure(w, 422, "Resolved rename entries overlap")
					return
				}
			}
		}
	}
	for _, p := range plans {
		if r.Context().Err() != nil {
			renameFailure(w, &directoryFailure{408, "Rename cancelled; earlier moves may have completed"}, true)
			return
		}
		if p.source == nil || p.noop {
			continue
		}
		if err = renameNoReplace(p.source, p.parent, p.leaf); err != nil {
			renameFailure(w, err, true)
			return
		}
	}
	respond(w, 204, nil)
}

func renameFailure(w http.ResponseWriter, err error, started bool) {
	if p, ok := err.(*directoryFailure); ok {
		failure(w, p.status, p.message)
		return
	}
	status, message := 422, "Cannot prepare rename inside server data"
	if os.IsExist(err) {
		status, message = 400, "Cannot move or rename file, destination already exists"
	}
	if started {
		message += "; earlier moves may have completed"
	}
	failure(w, status, message)
}

func renameParent(ctx context.Context, root *os.Root, s *node.Server, name string) (*os.File, error) {
	parent := path.Dir(name)
	if parent != "." {
		if err := walkDirectories(ctx, root, strings.Split(parent, "/"), s, false); err != nil {
			return nil, err
		}
	}
	// OpenRoot must succeed: rename never manufactures destination parents.
	dir, err := root.OpenRoot(parent)
	if err != nil {
		return nil, err
	}
	defer dir.Close()
	f, err := dir.Open(".")
	if err != nil {
		return nil, err
	}
	canonical, err := resolvedFilePath(root, f)
	if err == nil {
		err = renamePolicy(s, path.Join(canonical, path.Base(name)), false)
	}
	if err != nil {
		f.Close()
		return nil, err
	}
	return f, nil
}

func prepareRename(ctx context.Context, root *os.Root, s *node.Server, p *renamePlan, remaining *int) error {
	if ctx.Err() != nil {
		return &directoryFailure{408, "Rename request cancelled"}
	}
	if err := renamePolicy(s, p.from, false); err != nil {
		return err
	}
	if err := renamePolicy(s, p.to, false); err != nil {
		return err
	}
	parent, err := renameParent(ctx, root, s, p.from)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	defer parent.Close()
	if info, statErr := root.Lstat(p.from); statErr == nil && fileIsReparse(info) {
		return &directoryFailure{403, "Rename refuses links and reparse points"}
	} else if statErr != nil && !os.IsNotExist(statErr) {
		return statErr
	}
	source, err := openRenameSource(parent, path.Base(p.from))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	p.source = source
	if err = inspectRenameEntry(source); err != nil {
		return err
	}
	info, err := source.Stat()
	if err != nil {
		return err
	}
	if !info.IsDir() && !info.Mode().IsRegular() {
		return &directoryFailure{422, "Only regular files and directories can be renamed"}
	}
	canonical, err := resolvedFilePath(root, source)
	if err != nil {
		return err
	}
	if err = renamePolicy(s, canonical, info.IsDir()); err != nil {
		return err
	}
	if err = renamePolicy(s, p.to, info.IsDir()); err != nil {
		return err
	}
	p.parent, err = renameParent(ctx, root, s, p.to)
	if err != nil {
		return err
	}
	destCanonical, err := resolvedFilePath(root, p.parent)
	if err != nil {
		return err
	}
	destCanonical = path.Join(destCanonical, p.leaf)
	p.canonicalFrom, p.canonicalTo = canonical, destCanonical
	if !strings.EqualFold(canonical, destCanonical) && pathOverlaps(canonical, destCanonical) {
		return &directoryFailure{422, "Cannot move a path into itself or an ancestor"}
	}
	if err = renamePolicy(s, destCanonical, info.IsDir()); err != nil {
		return err
	}
	if target, err := root.Lstat(p.to); err == nil {
		if !strings.EqualFold(p.from, p.to) || !os.SameFile(info, target) {
			return os.ErrExist
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	if info.IsDir() {
		dir, err := root.OpenRoot(p.from)
		if err != nil {
			return err
		}
		defer dir.Close()
		return scanRenameTree(ctx, dir, s, canonical, destCanonical, 0, remaining)
	}
	*remaining--
	if *remaining < 0 {
		return &directoryFailure{413, "Rename scan exceeds 10000 entries"}
	}
	return nil
}

func scanRenameTree(ctx context.Context, dir *os.Root, s *node.Server, from, to string, depth int, remaining *int) error {
	if ctx.Err() != nil {
		return &directoryFailure{408, "Rename request cancelled"}
	}
	if depth > 32 {
		return &directoryFailure{413, "Rename directory scan exceeds 32 levels"}
	}
	*remaining--
	if *remaining < 0 {
		return &directoryFailure{413, "Rename scan exceeds 10000 entries"}
	}
	f, err := dir.Open(".")
	if err != nil {
		return err
	}
	entries, err := f.ReadDir(*remaining + 1)
	f.Close()
	if err != nil && err != io.EOF {
		return err
	}
	if len(entries) > *remaining {
		return &directoryFailure{413, "Rename scan exceeds 10000 entries"}
	}
	for _, entry := range entries {
		if ctx.Err() != nil {
			return &directoryFailure{408, "Rename request cancelled"}
		}
		if _, err := directoryParts("", entry.Name()); err != nil {
			return &directoryFailure{422, "Directory contains an unsupported Windows name"}
		}
		info, err := dir.Lstat(entry.Name())
		if err != nil {
			return err
		}
		if fileIsReparse(info) {
			return &directoryFailure{403, "Rename refuses directory links and reparse points"}
		}
		pf, pt := path.Join(from, entry.Name()), path.Join(to, entry.Name())
		if err = renamePolicy(s, pf, info.IsDir()); err != nil {
			return err
		}
		if err = renamePolicy(s, pt, info.IsDir()); err != nil {
			return err
		}
		if info.IsDir() {
			child, err := dir.OpenRoot(entry.Name())
			if err != nil {
				return err
			}
			err = scanRenameTree(ctx, child, s, pf, pt, depth+1, remaining)
			child.Close()
			if err != nil {
				return err
			}
		} else {
			*remaining--
			if *remaining < 0 {
				return &directoryFailure{413, "Rename scan exceeds 10000 entries"}
			}
			if !info.Mode().IsRegular() {
				return &directoryFailure{422, "Directory contains an unsupported entry type"}
			}
			file, err := dir.Open(entry.Name())
			if err != nil {
				return err
			}
			err = inspectRenameEntry(file)
			file.Close()
			if err != nil {
				return err
			}
		}
	}
	return nil
}
