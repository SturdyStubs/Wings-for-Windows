package logtail

import (
	"context"
	"errors"
	"golang.org/x/sys/windows"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
	"wings-windows/internal/config"
)

const MaxLine = 16 << 10
const MaxRead = 64 << 10
const MaxLines = 256

var ErrUnsafe = errors.New("log file is unsafe or unavailable")
var ErrText = errors.New("log file text exceeds supported limits")

type fileID struct{ volume, high, low uint32 }

func id(info windows.ByHandleFileInformation) fileID {
	return fileID{info.VolumeSerialNumber, info.FileIndexHigh, info.FileIndexLow}
}

type Reader struct {
	path        string
	pins        []windows.Handle
	protected   map[fileID]bool
	current     fileID
	known       bool
	offset      int64
	buffer      []byte
	skipPartial bool
	atStart     bool
	closed      bool
}

func (r *Reader) Close() error {
	if r.closed {
		return nil
	}
	r.closed = true
	var err error
	for i := len(r.pins) - 1; i >= 0; i-- {
		err = errors.Join(err, windows.CloseHandle(r.pins[i]))
	}
	r.pins = nil
	clear(r.buffer)
	r.buffer = nil
	return err
}
func Open(ctx context.Context, root, name string, protected []string) (result *Reader, err error) {
	if config.EggFileName(name) != nil || !filepath.IsAbs(root) || filepath.Clean(root) != root || len(filepath.VolumeName(root)) != 2 || len(protected) > 4096 {
		return nil, ErrUnsafe
	}
	r := &Reader{path: filepath.Join(root, filepath.FromSlash(name)), protected: map[fileID]bool{}, atStart: true}
	defer func() {
		if err != nil {
			r.Close()
		}
	}()
	parent := filepath.Dir(r.path)
	cursor := filepath.VolumeName(parent) + `\`
	drive, _ := windows.UTF16PtrFromString(cursor)
	if windows.GetDriveType(drive) == windows.DRIVE_REMOTE {
		return nil, ErrUnsafe
	}
	relative, e := filepath.Rel(cursor, parent)
	if e != nil {
		return nil, ErrUnsafe
	}
	components := []string{cursor}
	if relative != "." {
		for _, part := range strings.Split(relative, `\`) {
			if config.EggFileName(part) != nil {
				return nil, ErrUnsafe
			}
			cursor = filepath.Join(cursor, part)
			components = append(components, cursor)
		}
	}
	if len(components) > 64 {
		return nil, ErrUnsafe
	}
	for _, component := range components {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		p, e := windows.UTF16PtrFromString(component)
		if e != nil {
			return nil, ErrUnsafe
		}
		h, e := windows.CreateFile(p, windows.GENERIC_READ, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_OPEN_REPARSE_POINT|windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
		if e != nil {
			return nil, ErrUnsafe
		}
		r.pins = append(r.pins, h)
		var info windows.ByHandleFileInformation
		if windows.GetFileInformationByHandle(h, &info) != nil || info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 || info.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY == 0 {
			return nil, ErrUnsafe
		}
	}
	for _, path := range protected {
		if path == "" {
			continue
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if strings.EqualFold(filepath.Clean(path), r.path) {
			return nil, ErrUnsafe
		}
		p, e := windows.UTF16PtrFromString(path)
		if e != nil {
			return nil, ErrUnsafe
		}
		h, e := windows.CreateFile(p, windows.FILE_READ_ATTRIBUTES, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
		if errors.Is(e, windows.ERROR_FILE_NOT_FOUND) || errors.Is(e, windows.ERROR_PATH_NOT_FOUND) {
			continue
		}
		if e != nil {
			return nil, ErrUnsafe
		}
		var info windows.ByHandleFileInformation
		e = windows.GetFileInformationByHandle(h, &info)
		windows.CloseHandle(h)
		if e != nil {
			return nil, ErrUnsafe
		}
		r.protected[id(info)] = true
	}
	file, info, err := r.open()
	if err != nil {
		return nil, err
	}
	if file != nil {
		defer file.Close()
		r.known = true
		r.current = id(info)
		r.offset = size(info)
		r.atStart = r.offset == 0
		if r.offset > 0 {
			var last [1]byte
			if _, e = file.ReadAt(last[:], r.offset-1); e != nil {
				return nil, ErrUnsafe
			}
			r.skipPartial = last[0] != '\n'
		}
	}
	return r, nil
}
func size(info windows.ByHandleFileInformation) int64 {
	return int64(uint64(info.FileSizeHigh)<<32 | uint64(info.FileSizeLow))
}
func (r *Reader) open() (*os.File, windows.ByHandleFileInformation, error) {
	var info windows.ByHandleFileInformation
	p, e := windows.UTF16PtrFromString(r.path)
	if e != nil {
		return nil, info, ErrUnsafe
	}
	h, e := windows.CreateFile(p, windows.GENERIC_READ, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_OPEN_REPARSE_POINT|windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if errors.Is(e, windows.ERROR_FILE_NOT_FOUND) {
		return nil, info, nil
	}
	if e != nil {
		return nil, info, ErrUnsafe
	}
	f := os.NewFile(uintptr(h), r.path)
	if windows.GetFileInformationByHandle(h, &info) != nil || info.FileAttributes&(windows.FILE_ATTRIBUTE_DIRECTORY|windows.FILE_ATTRIBUTE_REPARSE_POINT) != 0 || info.NumberOfLinks != 1 || r.protected[id(info)] || size(info) < 0 || size(info) > 1<<40 {
		f.Close()
		return nil, info, ErrUnsafe
	}
	kind, e := windows.GetFileType(h)
	if e != nil || kind != windows.FILE_TYPE_DISK {
		f.Close()
		return nil, info, ErrUnsafe
	}
	return f, info, nil
}
func (r *Reader) Poll(ctx context.Context) (lines []string, err error) {
	if r.closed {
		return nil, ErrUnsafe
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	f, info, err := r.open()
	if err != nil {
		return nil, err
	}
	if f == nil {
		return nil, nil
	}
	defer f.Close()
	if !r.known || id(info) != r.current || size(info) < r.offset {
		r.current = id(info)
		r.known = true
		r.offset = 0
		r.buffer = nil
		r.skipPartial = false
		r.atStart = true
	}
	readBytes := 0
	for len(lines) < MaxLines {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if i := strings.IndexByte(string(r.buffer), '\n'); i >= 0 {
			raw := r.buffer[:i]
			r.buffer = r.buffer[i+1:]
			if r.skipPartial {
				r.skipPartial = false
				continue
			}
			raw = []byte(strings.TrimSuffix(string(raw), "\r"))
			if r.atStart {
				raw = []byte(strings.TrimPrefix(string(raw), "\xef\xbb\xbf"))
				r.atStart = false
			}
			if len(raw) > MaxLine || !utf8.Valid(raw) || strings.ContainsRune(string(raw), 0) {
				return nil, ErrText
			}
			lines = append(lines, string(raw))
			continue
		}
		if len(r.buffer) > MaxLine+4 {
			return nil, ErrText
		}
		if readBytes >= MaxRead {
			break
		}
		chunk := make([]byte, min(MaxLine, MaxRead-readBytes))
		n, e := f.ReadAt(chunk, r.offset)
		r.offset += int64(n)
		readBytes += n
		if r.skipPartial && strings.IndexByte(string(chunk[:n]), '\n') < 0 {
			r.buffer = nil
		} else {
			r.buffer = append(r.buffer, chunk[:n]...)
		}
		if e != nil && e != io.EOF {
			return nil, ErrUnsafe
		}
		if n == 0 {
			break
		}
	}
	var after windows.ByHandleFileInformation
	if windows.GetFileInformationByHandle(windows.Handle(f.Fd()), &after) != nil || after.NumberOfLinks != 1 || after.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		return nil, ErrUnsafe
	}
	return lines, nil
}
