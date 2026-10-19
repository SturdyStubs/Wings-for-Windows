package identity

import (
	"context"
	"fmt"
	"golang.org/x/sys/windows"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type pathAudit struct {
	ctx            context.Context
	checked, limit int
}
type pathCheck func(string, *windows.SECURITY_DESCRIPTOR, uint32) error

func localComponents(path string) ([]string, error) {
	path = filepath.FromSlash(path)
	if !filepath.IsAbs(path) || strings.HasPrefix(path, `\\`) || filepath.Clean(path) != path {
		return nil, fmt.Errorf("audit path must be a clean absolute local path: %q", path)
	}
	volume := filepath.VolumeName(path)
	if len(volume) != 2 || volume[1] != ':' {
		return nil, fmt.Errorf("unsupported audit volume: %q", path)
	}
	root := volume + `\`
	paths := []string{root}
	rel := strings.TrimPrefix(path, root)
	if rel == "" {
		return paths, nil
	}
	for _, part := range strings.Split(rel, `\`) {
		if part == "" || strings.TrimRight(part, " .") != part || strings.ContainsAny(part, ":\x00") {
			return nil, fmt.Errorf("ambiguous audit path: %q", path)
		}
		base := strings.ToUpper(strings.SplitN(part, ".", 2)[0])
		if base == "CON" || base == "PRN" || base == "AUX" || base == "NUL" || (len(base) == 4 && (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) && base[3] >= '1' && base[3] <= '9') {
			return nil, fmt.Errorf("device alias in audit path: %q", path)
		}
		paths = append(paths, filepath.Join(paths[len(paths)-1], part))
	}
	if len(paths) > 64 {
		return nil, fmt.Errorf("audit path exceeds 64 components")
	}
	return paths, nil
}

// withPath pins each component against delete sharing while checking
// handle metadata and ACLs, rejecting observed reparse-point components.
// Concurrent ACL/reparse edits are still outside the snapshot guarantee.
func (a *pathAudit) withPath(path string, forbidden uint32, check pathCheck, visit func(bool) error) error {
	components, err := localComponents(path)
	if err != nil {
		return err
	}
	for i, component := range components {
		if err := a.ctx.Err(); err != nil {
			return err
		}
		name, _ := windows.UTF16PtrFromString(component)
		// Read data/list access makes the no-delete sharing restriction effective;
		// metadata-only opens do not reliably participate in sharing checks.
		handle, err := windows.CreateFile(name, windows.GENERIC_READ, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
		if err != nil {
			return fmt.Errorf("open audit component %q: %w", component, err)
		}
		defer windows.CloseHandle(handle)
		var info windows.ByHandleFileInformation
		if err := windows.GetFileInformationByHandle(handle, &info); err != nil {
			return fmt.Errorf("audit metadata %q: %w", component, err)
		}
		if info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
			return fmt.Errorf("reparse points are unsupported in protected paths: %q", component)
		}
		directory := info.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY != 0
		if !directory && info.NumberOfLinks != 1 {
			return fmt.Errorf("protected file must have exactly one hardlink: %q", component)
		}
		sd, err := windows.GetSecurityInfo(handle, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION|windows.GROUP_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION)
		if err != nil {
			return fmt.Errorf("read protected path ACL %q: %w", component, err)
		}
		mask := AncestorAccess
		if i == len(components)-1 {
			mask = forbidden
		}
		if err := check(component, sd, mask); err != nil {
			return err
		}
		if i == len(components)-1 {
			return visit(directory)
		}
		if !directory {
			return fmt.Errorf("audit ancestor is not a directory: %q", component)
		}
	}
	return nil
}

func (a *pathAudit) walk(rule PathRule, check pathCheck) error {
	if err := a.ctx.Err(); err != nil {
		return err
	}
	a.checked++
	if a.checked > a.limit {
		return fmt.Errorf("protected path audit exceeds %d entries", a.limit)
	}
	return a.withPath(rule.Path, rule.Forbidden, check, func(directory bool) error {
		if !rule.Recursive || !directory {
			return nil
		}
		f, err := os.Open(rule.Path)
		if err != nil {
			return fmt.Errorf("enumerate protected directory: %w", err)
		}
		defer f.Close()
		for {
			if err := a.ctx.Err(); err != nil {
				return err
			}
			entries, err := f.ReadDir(128)
			for _, entry := range entries {
				if err := a.walk(PathRule{Path: filepath.Join(rule.Path, entry.Name()), Forbidden: rule.Forbidden, Recursive: true}, check); err != nil {
					return err
				}
			}
			if err == io.EOF {
				return nil
			}
			if err != nil {
				return fmt.Errorf("enumerate protected directory: %w", err)
			}
		}
	})
}
