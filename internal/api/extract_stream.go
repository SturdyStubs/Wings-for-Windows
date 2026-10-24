package api

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"io"
	"os"
	"sort"
	"strings"
)

type extractHeader struct {
	name      string
	size      int64
	directory bool
}

type extractStream struct {
	next   func() (extractHeader, io.Reader, error)
	finish func() error
	close  func()
}

func openExtractStream(source *os.File, size int64, check func() error) (*extractStream, error) {
	var magic [4]byte
	if _, e := source.ReadAt(magic[:], 0); e != nil {
		return nil, e
	}
	if bytes.Equal(magic[:], []byte{'P', 'K', 3, 4}) || bytes.Equal(magic[:], []byte{'P', 'K', 5, 6}) {
		return openExtractZIP(source, size, check)
	}
	reader := bufio.NewReader(downloadReader{r: source, check: check})
	var payload io.Reader = reader
	closeStream := func() {}
	compressed := magic[0] == 0x1f && magic[1] == 0x8b
	if compressed {
		z, e := gzip.NewReader(reader)
		if e != nil {
			return nil, &directoryFailure{400, "Expected a valid gzip tar archive"}
		}
		z.Multistream(false)
		payload = z
		closeStream = func() { _ = z.Close() }
	}
	decoded := &extractBudget{reader: payload, remaining: maxExtractStream, check: check}
	tr := tar.NewReader(decoded)
	return &extractStream{
		close: closeStream,
		next: func() (extractHeader, io.Reader, error) {
			before := decoded.used
			h, e := tr.Next()
			if e == io.EOF && decoded.used-before < 1024 {
				return extractHeader{}, nil, &directoryFailure{400, "Tar end markers are missing"}
			}
			if e != nil {
				return extractHeader{}, nil, e
			}
			dir := h.Typeflag == tar.TypeDir
			if !dir && h.Typeflag != tar.TypeReg && h.Typeflag != tar.TypeRegA {
				return extractHeader{}, nil, &directoryFailure{422, "Only regular files and directories may be extracted"}
			}
			if h.Linkname != "" || h.Size < 0 || dir && h.Size != 0 {
				return extractHeader{}, nil, &directoryFailure{422, "Invalid archive entry"}
			}
			for key := range h.PAXRecords {
				if strings.HasPrefix(key, "GNU.sparse.") {
					return extractHeader{}, nil, &directoryFailure{422, "Sparse archive records are unsupported"}
				}
			}
			return extractHeader{h.Name, h.Size, dir}, tr, nil
		},
		finish: func() error {
			if _, e := io.CopyBuffer(extractPadding{}, decoded, make([]byte, 64<<10)); e != nil {
				return e
			}
			if compressed {
				if _, e := reader.Peek(1); e != io.EOF {
					if e != nil {
						return e
					}
					return &directoryFailure{422, "Trailing gzip members or bytes are unsupported"}
				}
			}
			return nil
		},
	}, nil
}

// Bound the central directory before archive/zip can allocate one File per entry.
// This deliberately supports single-disk, non-ZIP64 archives without SFX prefixes,
// gaps or appended data. Local records must agree with their central records.
func openExtractZIP(source *os.File, size int64, check func() error) (*extractStream, error) {
	bad := func() error { return &directoryFailure{422, "Unsupported or inconsistent ZIP structure"} }
	read := func(offset int64, n int) ([]byte, error) {
		if e := check(); e != nil {
			return nil, e
		}
		if offset < 0 || int64(n) > size-offset {
			return nil, io.ErrUnexpectedEOF
		}
		b := make([]byte, n)
		_, e := source.ReadAt(b, offset)
		return b, e
	}
	u16 := func(b []byte) uint16 { return binary.LittleEndian.Uint16(b) }
	u32 := func(b []byte) uint32 { return binary.LittleEndian.Uint32(b) }
	n := min(size, 22+65535)
	tail, e := read(size-n, int(n))
	if e != nil {
		return nil, e
	}
	end := -1
	for i := len(tail) - 22; i >= 0; i-- {
		if u32(tail[i:]) == 0x06054b50 && i+22+int(u16(tail[i+20:])) == len(tail) {
			end = i
			break
		}
	}
	if end < 0 {
		return nil, bad()
	}
	// archive/zip scans backwards for its end record and optionally follows a
	// ZIP64 locator. Reject ambiguous signatures so it cannot select metadata
	// other than the bounded directory validated below.
	if bytes.Contains(tail[end+4:], []byte{'P', 'K', 5, 6}) {
		return nil, bad()
	}
	h := tail[end:]
	count := int(u16(h[10:]))
	centralSize := int64(u32(h[12:]))
	offset := int64(u32(h[16:]))
	centralEnd := size - n + int64(end)
	if centralEnd >= 20 {
		locator, e := read(centralEnd-20, 4)
		if e != nil {
			return nil, e
		}
		if u32(locator) == 0x07064b50 {
			return nil, bad()
		}
	}
	if u16(h[4:]) != 0 || u16(h[6:]) != 0 || int(u16(h[8:])) != count || count == 65535 || u32(h[12:]) == 0xffffffff || u32(h[16:]) == 0xffffffff {
		return nil, bad()
	}
	if count > maxArchiveEntries || centralSize > 1<<20 {
		return nil, &directoryFailure{413, "ZIP directory exceeds 256 entries or 1 MiB"}
	}
	if offset+centralSize != centralEnd {
		return nil, bad()
	}
	central, e := read(offset, int(centralSize))
	if e != nil {
		return nil, e
	}
	type record struct {
		offset int64
		header []byte
		name   []byte
	}
	var records []record
	for pos := 0; pos < len(central); {
		if len(records) >= count || len(central)-pos < 46 {
			return nil, bad()
		}
		h := central[pos : pos+46]
		length := 46 + int(u16(h[28:])) + int(u16(h[30:])) + int(u16(h[32:]))
		if u32(h) != 0x02014b50 || length > len(central)-pos || u16(h[34:]) != 0 || u32(h[20:]) == 0xffffffff || u32(h[24:]) == 0xffffffff || u32(h[42:]) == 0xffffffff {
			return nil, bad()
		}
		// Only UTF-8, data-descriptor and Deflate option bits are accepted.
		flags, method := u16(h[8:]), u16(h[10:])
		if flags & ^uint16(0x080e) != 0 || method != zip.Store && method != zip.Deflate || method == zip.Store && flags&6 != 0 {
			return nil, bad()
		}
		records = append(records, record{int64(u32(h[42:])), h, central[pos+46 : pos+46+int(u16(h[28:]))]})
		pos += length
	}
	if len(records) != count {
		return nil, bad()
	}
	sort.Slice(records, func(i, j int) bool { return records[i].offset < records[j].offset })
	expected := int64(0)
	for _, rec := range records {
		if rec.offset != expected {
			return nil, bad()
		}
		h, e := read(rec.offset, 30)
		if e != nil {
			return nil, e
		}
		c := rec.header
		if u32(h) != 0x04034b50 || u16(h[6:]) != u16(c[8:]) || u16(h[8:]) != u16(c[10:]) || int(u16(h[26:])) != len(rec.name) {
			return nil, bad()
		}
		name, e := read(rec.offset+30, len(rec.name))
		if e != nil {
			return nil, e
		}
		if !bytes.Equal(name, rec.name) {
			return nil, bad()
		}
		expected = rec.offset + 30 + int64(u16(h[26:])) + int64(u16(h[28:])) + int64(u32(c[20:]))
		if expected > offset {
			return nil, bad()
		}
		if u16(c[8:])&8 == 0 {
			if !bytes.Equal(h[14:26], c[16:28]) {
				return nil, bad()
			}
		} else {
			d, e := read(expected, 12)
			if e != nil {
				return nil, e
			}
			if u32(d) == 0x08074b50 {
				expected += 4
				d, e = read(expected, 12)
				if e != nil {
					return nil, e
				}
			}
			if !bytes.Equal(d, c[16:28]) {
				return nil, bad()
			}
			expected += 12
		}
	}
	if expected != offset {
		return nil, bad()
	}
	z, e := zip.NewReader(source, size)
	if e != nil {
		return nil, e
	}
	if len(z.File) != count {
		return nil, bad()
	}
	i := 0
	var active io.ReadCloser
	return &extractStream{
		close: func() {
			if active != nil {
				_ = active.Close()
			}
		},
		finish: check,
		next: func() (extractHeader, io.Reader, error) {
			if active != nil {
				_ = active.Close()
				active = nil
			}
			if e := check(); e != nil {
				return extractHeader{}, nil, e
			}
			if i == len(z.File) {
				return extractHeader{}, nil, io.EOF
			}
			f := z.File[i]
			i++
			mode := f.Mode()
			if !mode.IsRegular() && !mode.IsDir() || mode.IsDir() != strings.HasSuffix(f.Name, "/") || f.NonUTF8 || mode.IsDir() && (f.UncompressedSize64 != 0 || f.CRC32 != 0) {
				return extractHeader{}, nil, &directoryFailure{422, "ZIP requires UTF-8 regular files and directories"}
			}
			if f.UncompressedSize64 > uint64(maxArchiveInput) {
				return extractHeader{}, nil, &directoryFailure{413, "Expanded files exceed 100 MiB aggregate"}
			}
			active, e = f.Open()
			if e != nil {
				return extractHeader{}, nil, e
			}
			// Empty directory payloads also need their CRC/Deflate validation.
			if mode.IsDir() {
				var one [1]byte
				n, e := active.Read(one[:])
				if n != 0 || e != io.EOF {
					return extractHeader{}, nil, bad()
				}
			}
			return extractHeader{f.Name, int64(f.UncompressedSize64), mode.IsDir()}, active, nil
		},
	}, nil
}
