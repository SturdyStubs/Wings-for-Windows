package api

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path"
	"strings"
	"unicode/utf16"
	"wings-windows/internal/node"
)

type directoryFailure struct {
	status  int
	message string
}

func (e *directoryFailure) Error() string { return e.message }

// Validate both inputs before joining: path.Join would erase traversal and
// empty components. Only outer slashes are normalized for the panel contract.
func directoryParts(parent, name string) ([]string, error) {
	parent, name = strings.Trim(parent, "/"), strings.Trim(name, "/")
	if name == "" {
		return nil, fmt.Errorf("a directory name is required")
	}
	value := name
	if parent != "" {
		value = parent + "/" + name
	}
	if len(value) > 4096 {
		return nil, fmt.Errorf("directory path exceeds 4096 bytes")
	}
	if _, err := filePath(value); err != nil {
		return nil, err
	}
	parts := strings.Split(value, "/")
	if len(parts) > 32 {
		return nil, fmt.Errorf("directory path exceeds 32 components")
	}
	for _, part := range parts {
		if len(utf16.Encode([]rune(part))) > 255 {
			return nil, fmt.Errorf("directory component exceeds 255 UTF-16 units")
		}
		for _, r := range part {
			if r < 32 || strings.ContainsRune(`<>"|?*`, r) {
				return nil, fmt.Errorf("invalid Windows directory name")
			}
		}
		// Win32 recognizes these superscript digits as device-number aliases too.
		base := strings.ToUpper(strings.SplitN(part, ".", 2)[0])
		if (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) && len([]rune(base)) == 4 && strings.ContainsRune("¹²³", []rune(base)[3]) {
			return nil, fmt.Errorf("device directory name rejected")
		}
	}
	return parts, nil
}

func (a *API) createDirectory(w http.ResponseWriter, r *http.Request, s *node.Server) {
	var input struct {
		Name string `json:"name"`
		Path string `json:"path"`
	}
	if !decode(w, r, &input) {
		return
	}
	parts, err := directoryParts(input.Path, input.Name)
	if err != nil {
		failure(w, 422, "Invalid directory path: "+err.Error())
		return
	}
	root, err := os.OpenRoot(s.Root)
	if err != nil {
		failure(w, 500, "Cannot open server directory")
		return
	}
	defer root.Close()
	// Preflight the entire requested path and every existing component before
	// creating anything. A later I/O/cancellation failure can still leave new
	// empty parents; never recursively delete a path a game may now be using.
	for i := range parts {
		if s.FileDenied(strings.Join(parts[:i+1], "/"), true) {
			failure(w, 403, "Directory is restricted by the egg denylist")
			return
		}
	}
	if err = walkDirectories(r.Context(), root, parts, s, false); err == nil {
		err = walkDirectories(r.Context(), root, parts, s, true)
	}
	if err != nil {
		if problem, ok := err.(*directoryFailure); ok {
			failure(w, problem.status, problem.message)
		} else {
			failure(w, 422, "Cannot create directory inside server root")
		}
		return
	}
	respond(w, http.StatusNoContent, nil)
}

// Each child is opened relative to its confined parent. Existing reparse/link
// entries are refused rather than used as aliases, even inside this server.
func walkDirectories(ctx context.Context, anchor *os.Root, parts []string, s *node.Server, create bool) error {
	parent := anchor
	for _, part := range parts {
		if ctx.Err() != nil {
			return &directoryFailure{408, "Directory request was cancelled; created parents may remain"}
		}
		f, err := parent.Open(".")
		if err != nil {
			return err
		}
		canonical, err := resolvedFilePath(anchor, f)
		f.Close()
		if err != nil {
			return err
		}
		if s.FileDenied(canonical, true) || s.FileDenied(path.Join(canonical, part), true) {
			return &directoryFailure{403, "Directory is restricted by the egg denylist"}
		}
		info, err := parent.Lstat(part)
		if os.IsNotExist(err) {
			if !create {
				return nil
			}
			if err = parent.Mkdir(part, 0700); err != nil && !os.IsExist(err) {
				return err
			}
			info, err = parent.Lstat(part)
		}
		if err != nil {
			return err
		}
		if fileIsReparse(info) {
			return &directoryFailure{403, "Directory links and reparse points are not supported"}
		}
		if !info.IsDir() {
			return &directoryFailure{400, "Part of the path being created is not a directory (ENOTDIR)"}
		}
		child, err := parent.OpenRoot(part)
		if err != nil {
			return err
		}
		defer child.Close()
		parent = child
	}
	// Validate the final opened directory as well as every parent.
	f, err := parent.Open(".")
	if err != nil {
		return err
	}
	defer f.Close()
	canonical, err := resolvedFilePath(anchor, f)
	if err != nil {
		return err
	}
	if s.FileDenied(canonical, true) {
		return &directoryFailure{403, "Directory is restricted by the egg denylist"}
	}
	return nil
}
