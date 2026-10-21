package consolelog

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"golang.org/x/sys/windows"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"wings-windows/internal/config"
)

type diskStore struct {
	root         *os.Root
	path         string
	guard        io.Closer
	lock, active *os.File
	size, limit  int64
	files        int
}

func fileName(index int) string {
	if index == 0 {
		return "console.jsonl"
	}
	return "console." + strconv.Itoa(index) + ".jsonl"
}

type directoryGuard struct{ handles []windows.Handle }

func (g *directoryGuard) Close() error {
	var err error
	for i := len(g.handles) - 1; i >= 0; i-- {
		err = errors.Join(err, windows.CloseHandle(g.handles[i]))
	}
	g.handles = nil
	return err
}
func pinDirectory(path string) (guard io.Closer, err error) {
	g := &directoryGuard{}
	defer func() {
		if err != nil {
			g.Close()
		}
	}()
	volume := filepath.VolumeName(path) + `\`
	driveName, e := windows.UTF16PtrFromString(volume)
	if e != nil {
		return nil, e
	}
	if windows.GetDriveType(driveName) == windows.DRIVE_REMOTE {
		return nil, fmt.Errorf("network drives are unsupported for console logs")
	}
	cursor := volume
	paths := []string{volume}
	rel, e := filepath.Rel(volume, path)
	if e != nil {
		return nil, e
	}
	if rel != "." {
		for _, part := range strings.Split(rel, `\`) {
			cursor = filepath.Join(cursor, part)
			paths = append(paths, cursor)
		}
	}
	if len(paths) > 64 {
		return nil, fmt.Errorf("console directory exceeds 64 components")
	}
	for _, part := range paths {
		name, e := windows.UTF16PtrFromString(part)
		if e != nil {
			return nil, e
		}
		// Allow child writes/rotation; deny replacement of each directory itself.
		h, e := windows.CreateFile(name, windows.GENERIC_READ, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
		if e != nil {
			return nil, e
		}
		g.handles = append(g.handles, h)
		var info windows.ByHandleFileInformation
		if e = windows.GetFileInformationByHandle(h, &info); e != nil {
			return nil, e
		}
		if info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 || info.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY == 0 {
			return nil, fmt.Errorf("console path requires directories without reparse points")
		}
	}
	return g, nil
}

func retrySharing(operation func() error) error {
	until := time.Now().Add(time.Second)
	for {
		err := operation()
		if (!errors.Is(err, windows.ERROR_SHARING_VIOLATION) && !errors.Is(err, windows.ERROR_LOCK_VIOLATION)) || !time.Now().Before(until) {
			return err
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// Retain ancestor handles while creating each next component. This rejects
// observed links but does not defend concurrent ACL edits or writable mappings.
func guardedDirectory(path string) (io.Closer, error) {
	volume := filepath.VolumeName(path) + `\`
	parts, err := filepath.Rel(volume, path)
	if err != nil {
		return nil, err
	}
	guard, err := pinDirectory(volume)
	if err != nil {
		return nil, err
	}
	cursor := volume
	for _, part := range bytes.Split([]byte(parts), []byte{'\\'}) {
		cursor = filepath.Join(cursor, string(part))
		if err = os.Mkdir(cursor, 0700); err != nil && !os.IsExist(err) {
			guard.Close()
			return nil, err
		}
		next, e := pinDirectory(cursor)
		guard.Close()
		if e != nil {
			return nil, e
		}
		guard = next
	}
	return guard, nil
}
func openFile(path string, create bool, writable bool) (*os.File, error) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	access := uint32(windows.GENERIC_READ)
	if writable {
		access |= windows.GENERIC_WRITE
	}
	disposition := uint32(windows.OPEN_EXISTING)
	if create {
		disposition = windows.OPEN_ALWAYS
	}
	h, err := windows.CreateFile(p, access, windows.FILE_SHARE_READ, nil, disposition, windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return nil, err
	}
	var info windows.ByHandleFileInformation
	if err = windows.GetFileInformationByHandle(h, &info); err != nil {
		windows.CloseHandle(h)
		return nil, err
	}
	if info.FileAttributes&(windows.FILE_ATTRIBUTE_REPARSE_POINT|windows.FILE_ATTRIBUTE_DIRECTORY) != 0 || info.NumberOfLinks != 1 {
		windows.CloseHandle(h)
		return nil, fmt.Errorf("console log files must be regular single-link files")
	}
	return os.NewFile(uintptr(h), path), nil
}
func openDisk(options config.ConsoleLogs, server string) (d *diskStore, history []string, corrupt uint64, err error) {
	if !config.UUID.MatchString(server) {
		return nil, nil, 0, fmt.Errorf("console log requires full server UUID")
	}
	if !filepath.IsAbs(options.Directory) || filepath.Clean(options.Directory) != options.Directory {
		return nil, nil, 0, fmt.Errorf("console log directory must be canonical and absolute")
	}
	// Also validate direct package callers; data overlap is checked by Config.
	if options.MaxFileBytes < 1<<20 || options.MaxFileBytes > 64<<20 || options.MaxFiles < 1 || options.MaxFiles > 8 {
		return nil, nil, 0, fmt.Errorf("invalid console log limits")
	}
	path := filepath.Join(options.Directory, server)
	d = &diskStore{path: path, limit: options.MaxFileBytes, files: options.MaxFiles}
	defer func() {
		if err != nil {
			d.close()
		}
	}()
	d.guard, err = guardedDirectory(path)
	if err != nil {
		return d, nil, 0, err
	}
	d.root, err = os.OpenRoot(path)
	if err != nil {
		return d, nil, 0, err
	}
	d.lock, err = openFile(filepath.Join(path, ".writer.lock"), true, true)
	if err != nil {
		return d, nil, 0, fmt.Errorf("console log writer lock: %w", err)
	}
	for index := 7; index >= 0; index-- {
		f, e := openFile(filepath.Join(path, fileName(index)), false, false)
		if errors.Is(e, windows.ERROR_FILE_NOT_FOUND) {
			continue
		}
		if e != nil {
			return d, nil, 0, e
		}
		info, e := f.Stat()
		if e != nil {
			f.Close()
			return d, nil, 0, e
		}
		if info.Size() > d.limit {
			f.Close()
			return d, nil, 0, fmt.Errorf("existing console log exceeds configured size; archive it before lowering limits")
		}
		if index < d.files {
			lines, bad, e := readTail(f)
			if e != nil {
				f.Close()
				return d, nil, 0, e
			}
			corrupt += bad
			history = append(history, lines...)
			history = trimHistory(history)
		}
		f.Close()
		if index >= d.files {
			if e = d.root.Remove(fileName(index)); e != nil {
				return d, nil, 0, e
			}
		}
	}
	d.active, err = openFile(filepath.Join(path, fileName(0)), true, true)
	if err != nil {
		return d, nil, 0, err
	}
	info, err := d.active.Stat()
	if err != nil {
		return d, nil, 0, err
	}
	d.size = info.Size()
	// Discard only an incomplete final record, at most 1 MiB, before appending.
	if d.size > 0 {
		n := min(d.size, int64(1<<20))
		tail := make([]byte, int(n))
		if _, err = d.active.ReadAt(tail, d.size-n); err != nil {
			return d, nil, 0, err
		}
		if tail[len(tail)-1] != '\n' {
			last := bytes.LastIndexByte(tail, '\n')
			if last < 0 && d.size > n {
				return d, nil, 0, fmt.Errorf("unbounded incomplete console log tail")
			}
			d.size = d.size - n + int64(last+1)
			if err = d.active.Truncate(d.size); err != nil {
				return d, nil, 0, err
			}
		}
	}
	_, err = d.active.Seek(0, io.SeekEnd)
	return d, history, corrupt, err
}
func trimHistory(lines []string) []string {
	bytes := 0
	start := len(lines)
	for start > 0 && len(lines)-start < 1000 {
		n := len(lines[start-1])
		if bytes+n > 1<<20 {
			break
		}
		bytes += n
		start--
	}
	return append([]string(nil), lines[start:]...)
}
func readTail(f *os.File) ([]string, uint64, error) {
	info, err := f.Stat()
	if err != nil {
		return nil, 0, err
	}
	start := max(int64(0), info.Size()-(1<<20))
	if _, err = f.Seek(start, io.SeekStart); err != nil {
		return nil, 0, err
	}
	reader := bufio.NewReader(io.LimitReader(f, 1<<20))
	if start > 0 {
		if _, err = reader.ReadBytes('\n'); err != nil && err != io.EOF {
			return nil, 0, err
		}
	}
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, 0, err
	}
	records := bytes.Split(data, []byte{'\n'})
	var lines []string
	var bad uint64
	if len(records[len(records)-1]) > 0 {
		bad++
	}
	for _, record := range records[:len(records)-1] {
		var value entry
		if json.Unmarshal(record, &value) != nil || value.Time.IsZero() || len(value.Line) > maxLineBytes+64 {
			bad++
			continue
		}
		lines = append(lines, value.Line)
	}
	return trimHistory(lines), bad, nil
}
func (d *diskStore) rotate() error {
	if err := d.active.Sync(); err != nil {
		return err
	}
	if err := d.active.Close(); err != nil {
		return err
	}
	d.active = nil
	// Validate all owned slots before the first rename/remove.
	for i := 0; i < d.files; i++ {
		f, err := openFile(filepath.Join(d.path, fileName(i)), false, false)
		if errors.Is(err, windows.ERROR_FILE_NOT_FOUND) {
			continue
		}
		if err != nil {
			return err
		}
		f.Close()
	}
	last := fileName(d.files - 1)
	if err := retrySharing(func() error { return d.root.Remove(last) }); err != nil && !os.IsNotExist(err) {
		return err
	}
	for i := d.files - 2; i >= 0; i-- {
		if err := retrySharing(func() error { return d.root.Rename(fileName(i), fileName(i+1)) }); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	var err error
	d.active, err = openFile(filepath.Join(d.path, fileName(0)), true, true)
	if err == nil {
		d.size = 0
	}
	return err
}
func (d *diskStore) append(data []byte) error {
	if int64(len(data)) > d.limit {
		return fmt.Errorf("console record exceeds file size")
	}
	if d.size+int64(len(data)) > d.limit {
		if err := d.rotate(); err != nil {
			return err
		}
	}
	n, err := d.active.Write(data)
	d.size += int64(n)
	if err == nil && n != len(data) {
		err = io.ErrShortWrite
	}
	return err
}
func (d *diskStore) close() error {
	var result error
	if d.active != nil {
		result = errors.Join(result, d.active.Sync(), d.active.Close())
		d.active = nil
	}
	if d.lock != nil {
		result = errors.Join(result, d.lock.Close())
		d.lock = nil
	}
	if d.root != nil {
		result = errors.Join(result, d.root.Close())
		d.root = nil
	}
	if d.guard != nil {
		result = errors.Join(result, d.guard.Close())
		d.guard = nil
	}
	return result
}
