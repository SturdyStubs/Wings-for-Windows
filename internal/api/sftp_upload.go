package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"io"
	"os"
	"path"
	"strings"
	"sync"
	"time"
	"wings-windows/internal/sftpserver"
)

// sftpUpload publishes a new destination only on a successful CLOSE. Transport
// failure notifications poison it before pkg/sftp closes outstanding handles.
type sftpUpload struct {
	mu                         sync.Mutex
	backend                    SFTPBackend
	access                     sftpserver.Access
	ctx                        context.Context
	root                       *os.Root
	parent, stage              *os.File
	name, canonical, stageName string
	cancel                     context.CancelFunc
	size, total                int64
	ranges                     []uploadRange
	failed, closed             bool
	result                     error
	release                    func()
}
type uploadRange struct{ start, end int64 }

const maxSFTPUploadRanges = 4096

func (b SFTPBackend) Upload(ctx context.Context, a sftpserver.Access, name string) (sftpserver.WriteHandle, error) {
	if b.API.Manager.Config.SFTP == nil || !b.API.Manager.Config.SFTP.AllowUploads || !a.Can("file.create") || !b.Allowed(a) || ctx.Err() != nil {
		return nil, os.ErrPermission
	}
	name, e := sftpPath(name)
	if e != nil || name == "" {
		return nil, os.ErrPermission
	}
	if _, loaded := b.API.sftpUploadLocks.LoadOrStore(a.Server, true); loaded {
		return nil, os.ErrPermission
	}
	releaseServer := func() { b.API.sftpUploadLocks.Delete(a.Server) }
	select {
	case b.API.uploadSlots <- struct{}{}:
	default:
		releaseServer()
		return nil, os.ErrPermission
	}
	transferCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	u := &sftpUpload{backend: b, access: a, ctx: transferCtx, cancel: cancel, name: name, release: func() { <-b.API.uploadSlots; releaseServer() }}
	success := false
	defer func() {
		if !success {
			u.failed = true
			u.Close()
		}
	}()
	var err error
	u.root, u.parent, _, err = b.parent(ctx, a, name)
	if err != nil {
		return nil, os.ErrPermission
	}
	u.canonical, err = resolvedFilePath(u.root, u.parent)
	if err != nil {
		return nil, os.ErrPermission
	}
	if u.check() != nil {
		return nil, os.ErrPermission
	}
	if _, err = u.root.Lstat(path.Join(u.canonical, path.Base(name))); err == nil || !os.IsNotExist(err) {
		return nil, os.ErrPermission
	}
	var random [16]byte
	if _, err = rand.Read(random[:]); err != nil {
		return nil, os.ErrPermission
	}
	u.stageName = ".wings-sftp-" + hex.EncodeToString(random[:]) + ".tmp"
	if u.check() != nil {
		return nil, os.ErrPermission
	}
	u.stage, err = openCopyHandle(u.parent, u.stageName, true)
	if err != nil {
		return nil, os.ErrPermission
	}
	success = true
	go func() {
		<-transferCtx.Done()
		u.mu.Lock()
		if u.closed {
			u.mu.Unlock()
			return
		}
		u.failed = true
		u.mu.Unlock()
		u.Close()
	}()
	return u, nil
}
func (u *sftpUpload) check() error {
	if u.ctx.Err() != nil || !u.backend.Allowed(u.access) || !u.access.Can("file.create") {
		return os.ErrPermission
	}
	s := u.backend.API.Manager.Get(u.access.Server)
	if s == nil || renamePolicy(s, u.name, false) != nil || renamePolicy(s, path.Join(u.canonical, path.Base(u.name)), false) != nil {
		return os.ErrPermission
	}
	actual, e := resolvedFilePath(u.root, u.parent)
	if e != nil || !strings.EqualFold(actual, u.canonical) {
		return os.ErrPermission
	}
	if u.stageName != "" && (renamePolicy(s, path.Join(path.Dir(u.name), u.stageName), false) != nil || renamePolicy(s, path.Join(actual, u.stageName), false) != nil) {
		return os.ErrPermission
	}
	return nil
}
func (u *sftpUpload) TransferError(error) { u.mu.Lock(); u.failed = true; u.mu.Unlock() }
func (u *sftpUpload) WriteAt(p []byte, off int64) (n int, err error) {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.closed || u.failed {
		return 0, os.ErrPermission
	}
	defer func() {
		if err != nil {
			u.failed = true
		}
	}()
	if u.check() != nil || off < 0 || len(p) > 64<<10 || off > maxUploadBytes || int64(len(p)) > maxUploadBytes-off || int64(len(p)) > maxUploadBytes-u.total {
		return 0, os.ErrPermission
	}
	if len(p) == 0 {
		return 0, nil
	}
	// Merge a bounded coverage map before allocating space on disk. Random-order
	// packets are accepted, but holes must be filled before publication.
	merged := make([]uploadRange, 0, len(u.ranges)+1)
	current := uploadRange{off, off + int64(len(p))}
	inserted := false
	for _, r := range u.ranges {
		if r.end < current.start {
			merged = append(merged, r)
		} else if current.end < r.start {
			if !inserted {
				merged = append(merged, current)
				inserted = true
			}
			merged = append(merged, r)
		} else {
			if r.start < current.start {
				current.start = r.start
			}
			if r.end > current.end {
				current.end = r.end
			}
		}
	}
	if !inserted {
		merged = append(merged, current)
	}
	if len(merged) > maxSFTPUploadRanges {
		return 0, os.ErrPermission
	}
	n, err = u.stage.WriteAt(p, off)
	if err == nil && n != len(p) {
		err = io.ErrShortWrite
	}
	if err != nil {
		return n, os.ErrPermission
	}
	u.ranges = merged
	u.total += int64(n)
	if end := off + int64(n); end > u.size {
		u.size = end
	}
	return n, nil
}
func (u *sftpUpload) Close() (err error) {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.closed {
		return u.result
	}
	u.closed = true
	defer func() { err = u.result }()
	defer u.cancel()
	defer u.release()
	defer func() {
		if u.parent != nil {
			u.parent.Close()
		}
		if u.root != nil {
			u.root.Close()
		}
	}()
	success := false
	if u.stage != nil {
		defer func() {
			if !success && discardCopyStage(u.stage) != nil {
				u.result = os.ErrPermission
			}
			if u.stage.Close() != nil {
				u.result = os.ErrPermission
			}
		}()
	}
	u.result = os.ErrPermission
	if u.failed || u.stage == nil || u.check() != nil || u.size != 0 && (len(u.ranges) != 1 || u.ranges[0].start != 0 || u.ranges[0].end != u.size) {
		return u.result
	}
	if inspectRenameEntry(u.stage) != nil || u.stage.Sync() != nil || u.check() != nil {
		return u.result
	}
	if renameNoReplace(u.stage, u.parent, path.Base(u.name)) != nil {
		return u.result
	}
	success = true
	u.result = nil
	return nil
}
