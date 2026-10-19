package api

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
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
	if !((action == "list" || action == "contents") && r.Method == "GET" || action == "write" && r.Method == "POST") {
		failure(w, 501, "Only file listing, text reading and text writing are supported")
		return
	}
	key := "file"
	if action == "list" {
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
	if action == "write" {
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 8<<20))
		if err != nil {
			failure(w, 413, "Text files must be at most 8 MiB")
			return
		}
		if err = root.WriteFile(name, body, 0600); err != nil {
			failure(w, 422, "Cannot write file inside server directory")
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
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		io.Copy(w, io.LimitReader(f, 8<<20))
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
		result = append(result, map[string]any{"name": entry.Name(), "mode": info.Mode().String(), "mode_bits": fmt.Sprintf("%o", info.Mode().Perm()), "size": info.Size(), "is_file": info.Mode().IsRegular(), "is_symlink": info.Mode()&os.ModeSymlink != 0, "mimetype": "application/octet-stream", "created_at": info.ModTime(), "modified_at": info.ModTime()})
	}
	respond(w, 200, result)
}
