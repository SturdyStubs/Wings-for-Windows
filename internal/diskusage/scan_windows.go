package diskusage

import (
	"context"
	"errors"
	"golang.org/x/sys/windows"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"unsafe"
)

func Scan(ctx context.Context, root string) (Result, error) {
	return scan(ctx, root, 100000, 32)
}

func scan(ctx context.Context, root string, maxEntries, maxDepth int) (Result, error) {
	var result Result
	if !filepath.IsAbs(root) || filepath.Clean(root) != root || len(filepath.VolumeName(root)) != 2 {
		return Result{}, errors.New("unsupported disk root")
	}
	cursor := filepath.VolumeName(root) + `\`
	drive, _ := windows.UTF16PtrFromString(cursor)
	if windows.GetDriveType(drive) != windows.DRIVE_FIXED {
		return Result{}, errors.New("fixed local volume required")
	}
	rel, e := filepath.Rel(cursor, root)
	if e != nil {
		return Result{}, e
	}
	paths := []string{cursor}
	for _, part := range strings.Split(rel, `\`) {
		if part != "." {
			cursor = filepath.Join(cursor, part)
			paths = append(paths, cursor)
		}
	}
	if len(paths) > 64 {
		return Result{}, errors.New("root depth limit")
	}
	var pins []*os.File
	defer func() {
		for i := len(pins) - 1; i >= 0; i-- {
			pins[i].Close()
		}
	}()
	for _, name := range paths {
		if e = ctx.Err(); e != nil {
			return Result{}, e
		}
		p, e := windows.UTF16PtrFromString(name)
		if e != nil {
			return Result{}, e
		}
		h, e := windows.CreateFile(p, windows.GENERIC_READ, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
		if e != nil {
			return Result{}, e
		}
		f := os.NewFile(uintptr(h), name)
		pins = append(pins, f)
		info, e := inspect(f)
		if e != nil {
			return Result{}, e
		}
		if info.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY == 0 {
			return Result{}, errors.New("root ancestor is not a directory")
		}
	}
	remaining := maxEntries
	var walk func(*os.File, int) error
	walk = func(parent *os.File, depth int) error {
		if e := ctx.Err(); e != nil {
			return e
		}
		for {
			items, err := parent.ReadDir(256)
			for _, item := range items {
				if e := ctx.Err(); e != nil {
					return e
				}
				remaining--
				if remaining < 0 {
					return errors.New("disk entry limit")
				}
				f, e := openChild(parent, item.Name(), item.IsDir())
				if e != nil {
					return e
				}
				e = func() error {
					defer f.Close()
					info, e := inspect(f)
					if e != nil {
						return e
					}
					dir := info.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY != 0
					if dir != item.IsDir() {
						return errors.New("disk entry changed")
					}
					if dir {
						if depth >= maxDepth {
							return errors.New("disk depth limit")
						}
						result.Directories++
						return walk(f, depth+1)
					}
					size := uint64(info.FileSizeHigh)<<32 | uint64(info.FileSizeLow)
					if size > math.MaxInt64 || int64(size) > math.MaxInt64-result.Bytes {
						return errors.New("disk size overflow")
					}
					result.Bytes += int64(size)
					result.Files++
					return nil
				}()
				if e != nil {
					return e
				}
			}
			if err == io.EOF {
				return nil
			}
			if err != nil {
				return err
			}
		}
	}
	if e = walk(pins[len(pins)-1], 0); e != nil {
		return Result{}, e
	}
	if e = ctx.Err(); e != nil {
		return Result{}, e
	}
	return result, nil
}

func inspect(f *os.File) (windows.ByHandleFileInformation, error) {
	var info windows.ByHandleFileInformation
	e := windows.GetFileInformationByHandle(windows.Handle(f.Fd()), &info)
	if e == nil && (info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 || info.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY == 0 && info.NumberOfLinks != 1) {
		e = errors.New("linked disk entry refused")
	}
	return info, e
}

func openChild(parent *os.File, leaf string, dir bool) (*os.File, error) {
	if leaf == "" || leaf == "." || leaf == ".." || strings.ContainsAny(leaf, "/\\:\x00") {
		return nil, errors.New("invalid disk entry")
	}
	name, e := windows.NewNTUnicodeString(leaf)
	if e != nil {
		return nil, e
	}
	attrs := windows.OBJECT_ATTRIBUTES{RootDirectory: windows.Handle(parent.Fd()), ObjectName: name, Attributes: windows.OBJ_CASE_INSENSITIVE | windows.OBJ_DONT_REPARSE}
	attrs.Length = uint32(unsafe.Sizeof(attrs))
	access := uint32(windows.FILE_READ_ATTRIBUTES | windows.SYNCHRONIZE)
	share := uint32(windows.FILE_SHARE_READ | windows.FILE_SHARE_WRITE | windows.FILE_SHARE_DELETE)
	if dir {
		access = windows.FILE_GENERIC_READ
		share = windows.FILE_SHARE_READ | windows.FILE_SHARE_WRITE
	}
	var h windows.Handle
	e = windows.NtCreateFile(&h, access, &attrs, &windows.IO_STATUS_BLOCK{}, nil, windows.FILE_ATTRIBUTE_NORMAL, share, windows.FILE_OPEN, windows.FILE_OPEN_REPARSE_POINT|windows.FILE_SYNCHRONOUS_IO_NONALERT, 0, 0)
	if e != nil {
		return nil, e
	}
	return os.NewFile(uintptr(h), leaf), nil
}
