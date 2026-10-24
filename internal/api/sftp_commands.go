package api

import (
	"context"
	"io"
	"os"
	"path"
	"sync"
	"time"
	"wings-windows/internal/sftpserver"
)

// Command uses the same native handle primitives and rename/delete locks as the
// HTTP API. REMOVE never recurses; RMDIR must still be empty at native deletion.
func (b SFTPBackend) Command(ctx context.Context, a sftpserver.Access, method, name, target string) error {
	permission := sftpserver.CommandPermission(method)
	if b.API.Manager.Config.SFTP == nil || !b.API.Manager.Config.SFTP.AllowFileManagement || permission == "" || !a.Can(permission) || !b.Allowed(a) || ctx.Err() != nil {
		return os.ErrPermission
	}
	name, e := sftpPath(name)
	if e != nil || name == "" {
		return os.ErrPermission
	}
	if method == "Rename" {
		target, e = sftpPath(target)
		if e != nil || target == "" {
			return os.ErrPermission
		}
	}
	select {
	case b.API.sftpCommandSlots <- struct{}{}:
		defer func() { <-b.API.sftpCommandSlots }()
	default:
		return os.ErrPermission
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	s := b.API.Manager.Get(a.Server)
	if s == nil {
		return os.ErrPermission
	}
	policy := s.FilePolicySnapshot()
	check := func() error {
		if ctx.Err() != nil || !b.Allowed(a) || s.FilePolicySnapshot() != policy {
			return os.ErrPermission
		}
		return nil
	}
	// Share each mutation family's contention guard with its HTTP counterpart.
	var locks *sync.Map
	switch method {
	case "Rename":
		locks = &b.API.renameLocks
	case "Remove", "Rmdir":
		locks = &b.API.deleteLocks
	}
	if locks != nil {
		value, _ := locks.LoadOrStore(a.Server, &sync.Mutex{})
		lock := value.(*sync.Mutex)
		if !lock.TryLock() {
			return os.ErrPermission
		}
		defer lock.Unlock()
	}
	if method == "Rename" {
		root, e := os.OpenRoot(s.Root)
		if e != nil {
			return os.ErrPermission
		}
		defer root.Close()
		plan := renamePlan{from: name, to: target, leaf: path.Base(target), noop: name == target}
		defer func() {
			if plan.source != nil {
				plan.source.Close()
			}
			if plan.parent != nil {
				plan.parent.Close()
			}
		}()
		remaining := 1000
		if prepareRename(ctx, root, s, &plan, &remaining) != nil || plan.source == nil || check() != nil {
			return os.ErrPermission
		}
		info, e := plan.source.Stat()
		if e != nil {
			return os.ErrPermission
		}
		for _, p := range []string{name, target, plan.canonicalFrom, plan.canonicalTo} {
			if renamePolicy(s, p, info.IsDir()) != nil {
				return os.ErrPermission
			}
		}
		if plan.noop {
			return nil
		}
		if renameNoReplace(plan.source, plan.parent, plan.leaf) != nil {
			return os.ErrPermission
		}
		return nil
	}
	root, parent, s, e := b.parent(ctx, a, name)
	if e != nil {
		return os.ErrPermission
	}
	defer root.Close()
	defer parent.Close()
	canonical, e := resolvedFilePath(root, parent)
	if e != nil {
		return os.ErrPermission
	}
	requestedDir := method == "Mkdir" || method == "Rmdir"
	current := func() error {
		if check() != nil || renamePolicy(s, name, requestedDir) != nil {
			return os.ErrPermission
		}
		actual, e := resolvedFilePath(root, parent)
		if e != nil || actual != canonical || renamePolicy(s, path.Join(actual, path.Base(name)), requestedDir) != nil {
			return os.ErrPermission
		}
		return nil
	}
	if current() != nil {
		return os.ErrPermission
	}
	if method == "Mkdir" {
		f, e := createExtractDirectory(parent, path.Base(name))
		if e != nil {
			return os.ErrPermission
		}
		defer f.Close()
		if current() != nil || inspectRenameEntry(f) != nil {
			_ = deletePinnedEntry(f)
			return os.ErrPermission
		}
		actual, e := resolvedFilePath(root, f)
		if e != nil || renamePolicy(s, actual, true) != nil {
			_ = deletePinnedEntry(f)
			return os.ErrPermission
		}
		return nil
	}
	f, e := openRenameSource(parent, path.Base(name))
	if e != nil {
		return os.ErrPermission
	}
	defer f.Close()
	if inspectRenameEntry(f) != nil {
		return os.ErrPermission
	}
	info, e := f.Stat()
	if e != nil || info.IsDir() != requestedDir || !requestedDir && !info.Mode().IsRegular() {
		return os.ErrPermission
	}
	actual, e := resolvedFilePath(root, f)
	if e != nil || actual == "." || renamePolicy(s, actual, requestedDir) != nil {
		return os.ErrPermission
	}
	if requestedDir {
		entries, e := f.ReadDir(1)
		if len(entries) != 0 || e != io.EOF {
			return os.ErrPermission
		}
	}
	if current() != nil || renamePolicy(s, actual, requestedDir) != nil {
		return os.ErrPermission
	}
	if deletePinnedEntry(f) != nil || f.Close() != nil {
		return os.ErrPermission
	}
	return nil
}
