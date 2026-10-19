package api

import (
	"fmt"
	"github.com/gabriel-vasile/mimetype"
	"io"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"time"
	"wings-windows/internal/node"
)

// Windows paths also have device names, ADS and trailing-dot aliases. Reject
// those before using os.Root, which enforces confinement across symlinks.
func filePath(value string) (string, error) {
	if strings.ContainsAny(value, "\\:\x00") {
		return "", fmt.Errorf("invalid path")
	}
	value = strings.TrimLeft(value, "/")
	if value == "" {
		return ".", nil
	}
	for _, part := range strings.Split(value, "/") {
		if part == "" || part == "." || part == ".." || strings.TrimRight(part, ". ") != part {
			return "", fmt.Errorf("invalid path component")
		}
		base := strings.ToUpper(strings.SplitN(part, ".", 2)[0])
		if base == "CON" || base == "PRN" || base == "AUX" || base == "NUL" || strings.HasPrefix(base, "COM") && len(base) == 4 && base[3] >= '1' && base[3] <= '9' || strings.HasPrefix(base, "LPT") && len(base) == 4 && base[3] >= '1' && base[3] <= '9' {
			return "", fmt.Errorf("device path rejected")
		}
	}
	return value, nil
}
func (a *API) files(w http.ResponseWriter, r *http.Request, s *node.Server, action string) {
	listing := action == "list-directory" || action == "list"
	if !((listing || action == "contents") && r.Method == "GET" || action == "write" && r.Method == "POST") {
		failure(w, 501, "Only file listing, text reading and text writing are supported")
		return
	}
	key := "file"
	if listing {
		key = "directory"
	}
	name, err := filePath(r.URL.Query().Get(key))
	if err != nil {
		failure(w, 422, "Invalid file path")
		return
	}
	root, err := os.OpenRoot(s.Root)
	if err != nil {
		failure(w, 500, "Cannot open server directory")
		return
	}
	defer root.Close()
	directory := listing
	if info, err := root.Stat(name); err == nil {
		directory = info.IsDir()
	}
	if s.FileDenied(name, directory) {
		failure(w, 403, "File is restricted by the egg denylist")
		return
	}
	allowed := func(f *os.File, dir bool) bool {
		canonical, err := resolvedFilePath(root, f)
		return err == nil && !s.FileDenied(canonical, dir)
	}
	if action == "write" {
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 8<<20))
		if err != nil {
			failure(w, 413, "Text files must be at most 8 MiB")
			return
		}
		parent, err := root.Open(path.Dir(name))
		if err != nil {
			failure(w, 422, "Cannot open parent directory")
			return
		}
		parentName, err := resolvedFilePath(root, parent)
		parent.Close()
		if err != nil || s.FileDenied(path.Join(parentName, path.Base(name)), false) {
			failure(w, 403, "File is restricted by the egg denylist")
			return
		}
		// Check the opened target before truncating it: a junction/symlink may
		// resolve to a denied file even when its requested name is allowed.
		f, err := root.OpenFile(name, os.O_WRONLY|os.O_CREATE, 0600)
		if err != nil {
			failure(w, 422, "Cannot write file inside server directory")
			return
		}
		defer f.Close()
		if !allowed(f, false) {
			failure(w, 403, "File is restricted by the egg denylist")
			return
		}
		if info, err := f.Stat(); err != nil || !info.Mode().IsRegular() {
			failure(w, 422, "A regular file is required")
			return
		}
		if err = f.Truncate(0); err == nil {
			_, err = f.Write(body)
		}
		if err != nil {
			failure(w, 500, "Cannot complete file write")
			return
		}
		respond(w, 204, nil)
		return
	}
	f, err := root.Open(name)
	if err != nil {
		failure(w, 404, "File or directory unavailable")
		return
	}
	defer f.Close()
	if !allowed(f, directory) {
		failure(w, 403, "File is restricted by the egg denylist")
		return
	}
	if action == "contents" {
		info, err := f.Stat()
		if err != nil || !info.Mode().IsRegular() {
			failure(w, 422, "A regular file is required")
			return
		}
		if info.Size() > 8<<20 {
			failure(w, 413, "Text files must be at most 8 MiB")
			return
		}
		mime := fileMIME(f)
		w.Header().Set("Content-Type", mime)
		w.Header().Set("X-Mime-Type", mime)
		w.Header().Set("Content-Length", strconv.FormatInt(info.Size(), 10))
		if r.URL.Query().Get("download") != "" {
			w.Header().Set("Content-Disposition", "attachment; filename="+strconv.Quote(info.Name()))
			w.Header().Set("Content-Type", "application/octet-stream")
		}
		io.Copy(w, io.LimitReader(f, info.Size()))
		return
	}
	entries, err := f.ReadDir(10001)
	if err != nil && err != io.EOF {
		failure(w, 422, "Cannot list directory")
		return
	}
	if len(entries) > 10000 {
		failure(w, 413, "Directory contains too many files")
		return
	}
	result := []any{}
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}
		childName := path.Join(name, entry.Name())
		if s.FileDenied(childName, info.IsDir()) {
			continue
		}
		child, err := root.Open(childName)
		if err != nil {
			continue
		}
		if !allowed(child, info.IsDir()) {
			child.Close()
			continue
		}
		mime := "inode/directory"
		if info.Mode().IsRegular() {
			mime = fileMIME(child)
		}
		child.Close()
		result = append(result, fileMetadata{Name: entry.Name(), Mode: info.Mode().String(), ModeBits: fmt.Sprintf("%o", info.Mode().Perm()), Size: info.Size(), File: !info.IsDir(), Directory: info.IsDir(), Symlink: info.Mode()&os.ModeSymlink != 0, Mime: mime, Created: fileCreated(info).UTC().Format(time.RFC3339), Modified: info.ModTime().UTC().Format(time.RFC3339)})
	}
	respond(w, 200, result)
}

type fileMetadata struct {
	Name      string `json:"name"`
	Mode      string `json:"mode"`
	ModeBits  string `json:"mode_bits"`
	Size      int64  `json:"size"`
	File      bool   `json:"file"`
	Directory bool   `json:"directory"`
	Symlink   bool   `json:"symlink"`
	Mime      string `json:"mime"`
	Created   string `json:"created"`
	Modified  string `json:"modified"`
}

func fileMIME(f *os.File) string {
	mime, err := mimetype.DetectReader(io.LimitReader(f, 3072))
	f.Seek(0, io.SeekStart)
	if err != nil {
		return "application/octet-stream"
	}
	return mime.String()
}
