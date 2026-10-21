package eggconfig

import (
	"context"
	"errors"
	"fmt"
	"golang.org/x/sys/windows"
	"io"
	"os"
	"path/filepath"
	"strings"
	"wings-windows/internal/config"
)

type pinnedDirs []windows.Handle

func (p pinnedDirs) close() {
	for i := len(p) - 1; i >= 0; i-- {
		windows.CloseHandle(p[i])
	}
}
func pinParents(ctx context.Context, directory string) (result pinnedDirs, err error) {
	pins := pinnedDirs{}
	defer func() {
		if err != nil {
			pins.close()
		}
	}()
	if !filepath.IsAbs(directory) || filepath.Clean(directory) != directory || len(filepath.VolumeName(directory)) != 2 {
		return nil, errors.New("invalid egg file root")
	}
	cursor := filepath.VolumeName(directory) + `\`
	drive, _ := windows.UTF16PtrFromString(cursor)
	if windows.GetDriveType(drive) == windows.DRIVE_REMOTE {
		return nil, errors.New("network egg file roots unsupported")
	}
	relative, err := filepath.Rel(cursor, directory)
	if err != nil {
		return nil, err
	}
	paths := []string{cursor}
	for _, part := range strings.Split(relative, `\`) {
		if part != "." {
			cursor = filepath.Join(cursor, part)
			paths = append(paths, cursor)
		}
	}
	if len(paths) > 64 {
		return nil, errors.New("egg file path exceeds depth limit")
	}
	for _, path := range paths {
		if err = ctx.Err(); err != nil {
			return nil, err
		}
		name, e := windows.UTF16PtrFromString(path)
		if e != nil {
			return nil, e
		}
		handle, e := windows.CreateFile(name, windows.GENERIC_READ, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
		if e != nil {
			return nil, e
		}
		pins = append(pins, handle)
		var info windows.ByHandleFileInformation
		if err = windows.GetFileInformationByHandle(handle, &info); err != nil {
			return nil, err
		}
		if info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 || info.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY == 0 {
			return nil, errors.New("egg file parent must be a directory without links")
		}
	}
	return pins, nil
}

type target struct {
	file          *os.File
	pins          pinnedDirs
	before, after []byte
}

func (t *target) close() {
	if t.file != nil {
		t.file.Close()
	}
	t.pins.close()
}
func sameFile(a, b windows.ByHandleFileInformation) bool {
	return a.VolumeSerialNumber == b.VolumeSerialNumber && a.FileIndexHigh == b.FileIndexHigh && a.FileIndexLow == b.FileIndexLow
}
func openTarget(ctx context.Context, root string, plan File, protected []string) (result *target, err error) {
	if err = config.EggFileName(plan.Name); err != nil {
		return nil, err
	}
	path := filepath.Join(root, filepath.FromSlash(plan.Name))
	t := &target{}
	defer func() {
		if err != nil {
			t.close()
		}
	}()
	t.pins, err = pinParents(ctx, filepath.Dir(path))
	if err != nil {
		return nil, err
	}
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	handle, err := windows.CreateFile(name, windows.GENERIC_READ|windows.GENERIC_WRITE, windows.FILE_SHARE_READ, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return nil, err
	}
	t.file = os.NewFile(uintptr(handle), path)
	var info windows.ByHandleFileInformation
	if err = windows.GetFileInformationByHandle(handle, &info); err != nil {
		return nil, err
	}
	if info.FileAttributes&(windows.FILE_ATTRIBUTE_DIRECTORY|windows.FILE_ATTRIBUTE_REPARSE_POINT) != 0 || info.NumberOfLinks != 1 {
		return nil, errors.New("egg target must be a regular single-link file")
	}
	if len(protected) > 4096 {
		return nil, errors.New("too many protected files for rewriting")
	}
	for _, path := range protected {
		if path == "" {
			continue
		}
		if err = ctx.Err(); err != nil {
			return nil, err
		}
		var p *uint16
		p, err = windows.UTF16PtrFromString(path)
		if err != nil {
			return nil, err
		}
		var other windows.Handle
		other, err = windows.CreateFile(p, windows.FILE_READ_ATTRIBUTES, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
		if errors.Is(err, windows.ERROR_FILE_NOT_FOUND) || errors.Is(err, windows.ERROR_PATH_NOT_FOUND) {
			err = nil
			continue
		}
		if err != nil {
			return nil, err
		}
		var identity windows.ByHandleFileInformation
		err = windows.GetFileInformationByHandle(other, &identity)
		windows.CloseHandle(other)
		if err != nil {
			return nil, err
		}
		if sameFile(info, identity) {
			return nil, errors.New("egg target is protected node configuration, TLS or code")
		}
	}
	t.before, err = io.ReadAll(io.LimitReader(t.file, MaxFileBytes+1))
	if err != nil {
		return nil, err
	}
	t.after, err = Render(ctx, t.before, plan)
	if err != nil {
		return nil, err
	}
	return t, nil
}

// Apply renders and pins all existing targets before writing any of them.
// In-place writes preserve DACLs; this is not a multi-file transaction.
func Apply(ctx context.Context, root string, plans []File, protected []string) error {
	if len(plans) > 16 {
		return errors.New("too many egg files")
	}
	var opened []*target
	defer func() {
		for _, t := range opened {
			t.close()
		}
	}()
	for i, plan := range plans {
		t, err := openTarget(ctx, root, plan, protected)
		if err != nil {
			return fmt.Errorf("egg file %d preflight failed", i+1)
		}
		opened = append(opened, t)
	}
	for i, t := range opened {
		if err := ctx.Err(); err != nil {
			return err
		}
		var info windows.ByHandleFileInformation
		if windows.GetFileInformationByHandle(windows.Handle(t.file.Fd()), &info) != nil || info.NumberOfLinks != 1 {
			return errors.New("egg target identity changed; partial edits possible")
		}
		if string(t.before) == string(t.after) {
			continue
		}
		if _, err := t.file.Seek(0, io.SeekStart); err != nil {
			return fmt.Errorf("egg file %d seek failed; partial edits possible", i+1)
		}
		if err := t.file.Truncate(0); err != nil {
			return fmt.Errorf("egg file %d truncate failed; partial edits possible", i+1)
		}
		if _, err := t.file.Write(t.after); err != nil {
			return fmt.Errorf("egg file %d write failed; partial edits possible", i+1)
		}
		if err := t.file.Sync(); err != nil {
			return fmt.Errorf("egg file %d sync failed; partial edits possible", i+1)
		}
	}
	return nil
}
