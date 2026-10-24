package api

import (
	"context"
	"io"
	"os"
	"path"
	"strings"
	"sync"
	"time"
	"wings-windows/internal/config"
	"wings-windows/internal/node"
	"wings-windows/internal/sftpserver"
)

type SFTPBackend struct{ API *API }

func (b SFTPBackend) Authenticate(ctx context.Context, r sftpserver.AuthRequest) (sftpserver.Access, error) {
	start := time.Now()
	bad := func() (sftpserver.Access, error) { return sftpserver.Access{}, os.ErrPermission }
	if r.Type != "password" && r.Type != "public_key" || len(r.User) > 200 || len(r.Pass) == 0 || len(r.Pass) > 8192 || strings.ContainsAny(r.User, "\x00\r\n/\\") {
		return bad()
	}
	cut := strings.LastIndex(r.User, ".")
	if cut < 1 || len(r.User)-cut != 9 {
		return bad()
	}
	short := strings.ToLower(r.User[cut+1:])
	var selected *node.Server
	for _, s := range b.API.Manager.All() {
		if strings.HasPrefix(s.ID(), short) {
			if selected != nil {
				return bad()
			}
			selected = s
		}
	}
	if selected == nil || selected.SuspendedOrClosing() {
		return bad()
	}
	var reply struct {
		User        string   `json:"user"`
		Server      string   `json:"server"`
		Permissions []string `json:"permissions"`
	}
	if b.API.Manager.Panel.SFTPAuth(ctx, r, &reply) != nil || reply.Server != selected.ID() || !config.UUID.MatchString(reply.User) || len(reply.Permissions) > 128 {
		return bad()
	}
	for _, p := range reply.Permissions {
		if len(p) > 128 {
			return bad()
		}
	}
	a := sftpserver.Access{Server: reply.Server, User: strings.ToLower(reply.User), Permissions: reply.Permissions, Issued: start}
	if !a.Can("file.sftp") || !b.Allowed(a) {
		return bad()
	}
	return a, nil
}
func (b SFTPBackend) Allowed(a sftpserver.Access) bool {
	if a.User == "" || a.Issued.IsZero() {
		return false
	}
	select {
	case <-b.API.done:
		return false
	default:
	}
	s := b.API.Manager.Get(a.Server)
	if s == nil || s.SuspendedOrClosing() {
		return false
	}
	if t, ok := b.API.revoked.Load("user:" + strings.ToLower(a.Server) + ":" + strings.ToLower(a.User)); ok && !a.Issued.After(t.(time.Time)) {
		return false
	}
	return true
}
func sftpPath(name string) (string, error) {
	if name == "" || name == "." || name == "/" {
		return "", nil
	}
	parts, e := directoryParts("", strings.Trim(name, "/"))
	if e != nil {
		return "", os.ErrPermission
	}
	return strings.Join(parts, "/"), nil
}
func (b SFTPBackend) parent(ctx context.Context, a sftpserver.Access, name string) (*os.Root, *os.File, *node.Server, error) {
	if !b.Allowed(a) {
		return nil, nil, nil, os.ErrPermission
	}
	s := b.API.Manager.Get(a.Server)
	if s == nil {
		return nil, nil, nil, os.ErrPermission
	}
	root, e := os.OpenRoot(s.Root)
	if e != nil {
		return nil, nil, nil, os.ErrPermission
	}
	f, e := renameParent(ctx, root, s, name)
	if e != nil {
		root.Close()
		return nil, nil, nil, os.ErrPermission
	}
	return root, f, s, nil
}

type sftpRead struct {
	file, parent    *os.File
	root            *os.Root
	backend         SFTPBackend
	access          sftpserver.Access
	name, canonical string
	size            int64
	once            sync.Once
}

func (f *sftpRead) ReadAt(p []byte, offset int64) (int, error) {
	if !f.backend.Allowed(f.access) {
		return 0, os.ErrPermission
	}
	s := f.backend.API.Manager.Get(f.access.Server)
	if s == nil || renamePolicy(s, f.name, false) != nil || renamePolicy(s, f.canonical, false) != nil {
		return 0, os.ErrPermission
	}
	if len(p) == 0 {
		return 0, nil
	}
	if offset < 0 {
		return 0, os.ErrPermission
	}
	if offset >= f.size {
		return 0, io.EOF
	}
	if int64(len(p)) > f.size-offset {
		n, e := f.file.ReadAt(p[:f.size-offset], offset)
		if e == nil {
			e = io.EOF
		}
		return n, e
	}
	return f.file.ReadAt(p, offset)
}
func (f *sftpRead) Close() error {
	f.once.Do(func() {
		if f.file != nil {
			f.file.Close()
		}
		if f.parent != nil {
			f.parent.Close()
		}
		if f.root != nil {
			f.root.Close()
		}
	})
	return nil
}
func (b SFTPBackend) Read(ctx context.Context, a sftpserver.Access, name string) (sftpserver.ReadHandle, error) {
	name, e := sftpPath(name)
	if e != nil || name == "" || !a.Can("file.read-content") {
		return nil, os.ErrPermission
	}
	root, parent, s, e := b.parent(ctx, a, name)
	if e != nil {
		return nil, e
	}
	f := &sftpRead{root: root, parent: parent, backend: b, access: a, name: name}
	ok := false
	defer func() {
		if !ok {
			f.Close()
		}
	}()
	if renamePolicy(s, name, false) != nil {
		return nil, os.ErrPermission
	}
	f.file, e = openCopyHandle(parent, path.Base(name), false)
	if e != nil {
		return nil, os.ErrPermission
	}
	if inspectRenameEntry(f.file) != nil {
		return nil, os.ErrPermission
	}
	info, e := f.file.Stat()
	if e != nil || !info.Mode().IsRegular() || info.Size() < 0 || info.Size() > maxDownloadBytes {
		return nil, os.ErrPermission
	}
	f.canonical, e = resolvedFilePath(root, f.file)
	if e != nil || renamePolicy(s, f.canonical, false) != nil || !b.Allowed(a) {
		return nil, os.ErrPermission
	}
	f.size = info.Size()
	ok = true
	return f, nil
}
func (b SFTPBackend) List(ctx context.Context, a sftpserver.Access, name string) ([]os.FileInfo, error) {
	name, e := sftpPath(name)
	if e != nil || !a.Can("file.read") {
		return nil, os.ErrPermission
	}
	root, f, s, e := b.parent(ctx, a, path.Join(name, ".sftp-list-check"))
	if e != nil {
		return nil, e
	}
	defer root.Close()
	defer f.Close()
	canonical, e := resolvedFilePath(root, f)
	if e != nil || renamePolicy(s, name, true) != nil || renamePolicy(s, canonical, true) != nil {
		return nil, os.ErrPermission
	}
	var result []os.FileInfo
	count := 0
	for {
		entries, readErr := f.ReadDir(64)
		for _, entry := range entries {
			count++
			if count > 1000 || ctx.Err() != nil || !b.Allowed(a) {
				return nil, os.ErrPermission
			}
			requested := path.Join(name, entry.Name())
			actual := path.Join(canonical, entry.Name())
			if renamePolicy(s, requested, entry.IsDir()) != nil || renamePolicy(s, actual, entry.IsDir()) != nil {
				continue
			}
			child, e := openCopyHandle(f, entry.Name(), false)
			if e != nil {
				continue
			}
			info, e := child.Stat()
			safe := inspectRenameEntry(child) == nil
			child.Close()
			if e != nil || !safe || !info.IsDir() && !info.Mode().IsRegular() {
				continue
			}
			if renamePolicy(s, requested, info.IsDir()) != nil || renamePolicy(s, actual, info.IsDir()) != nil {
				continue
			}
			result = append(result, &sftpInfo{FileInfo: info, backend: b, access: a, requested: requested, canonical: actual})
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return nil, os.ErrPermission
		}
	}
	if !b.Allowed(a) {
		return nil, os.ErrPermission
	}
	return result, nil
}
func (b SFTPBackend) Stat(ctx context.Context, a sftpserver.Access, name string) (os.FileInfo, error) {
	name, e := sftpPath(name)
	if e != nil || !a.Can("file.read") {
		return nil, os.ErrPermission
	}
	if name == "" {
		root, f, s, e := b.parent(ctx, a, ".sftp-stat-check")
		if e != nil {
			return nil, e
		}
		defer root.Close()
		defer f.Close()
		if renamePolicy(s, ".", true) != nil {
			return nil, os.ErrPermission
		}
		info, e := f.Stat()
		if e != nil {
			return nil, os.ErrPermission
		}
		return &sftpInfo{FileInfo: info, backend: b, access: a, requested: ".", canonical: "."}, nil
	}
	root, parent, s, e := b.parent(ctx, a, name)
	if e != nil {
		return nil, e
	}
	defer root.Close()
	defer parent.Close()
	f, e := openCopyHandle(parent, path.Base(name), false)
	if e != nil {
		return nil, os.ErrPermission
	}
	defer f.Close()
	if inspectRenameEntry(f) != nil {
		return nil, os.ErrPermission
	}
	info, e := f.Stat()
	if e != nil {
		return nil, os.ErrPermission
	}
	canonical, e := resolvedFilePath(root, f)
	if e != nil || renamePolicy(s, name, info.IsDir()) != nil || renamePolicy(s, canonical, info.IsDir()) != nil || !b.Allowed(a) {
		return nil, os.ErrPermission
	}
	return &sftpInfo{FileInfo: info, backend: b, access: a, requested: name, canonical: canonical}, nil
}

type sftpInfo struct {
	os.FileInfo
	backend              SFTPBackend
	access               sftpserver.Access
	requested, canonical string
}

func (i *sftpInfo) Allowed() bool {
	s := i.backend.API.Manager.Get(i.access.Server)
	return i.backend.Allowed(i.access) && renamePolicy(s, i.requested, i.IsDir()) == nil && renamePolicy(s, i.canonical, i.IsDir()) == nil
}
