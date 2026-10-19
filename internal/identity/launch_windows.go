package identity

import (
	"context"
	"errors"
	"fmt"
	"io"

	"golang.org/x/sys/windows"
)

const maxLaunchHandles = 1024

type launchGuard struct{ handles []windows.Handle }

func (g *launchGuard) Close() error {
	var result error
	for i := len(g.handles) - 1; i >= 0; i-- {
		result = errors.Join(result, windows.CloseHandle(g.handles[i]))
	}
	g.handles = nil
	return result
}

// PinLaunchPaths holds local path components without write/delete sharing until
// Close. Acquire before auditing ACLs and retain through suspended creation and
// resume. This is a bounded startup guard, not a recursive filesystem lock: ACL
// edits, pre-existing writable mappings and undeclared loader inputs remain out
// of scope. Handles are never inherited by a child.
func PinLaunchPaths(ctx context.Context, paths []string) (guard io.Closer, err error) {
	if ctx == nil {
		ctx = context.Background()
	}
	g := &launchGuard{}
	defer func() {
		if err != nil {
			err = errors.Join(err, g.Close())
		}
	}()
	if len(paths) == 0 {
		return nil, errors.New("launch paths are required")
	}
	seen := map[string]bool{}
	for _, path := range paths {
		components, e := localComponents(path)
		if e != nil {
			return nil, e
		}
		for i, component := range components {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			// Do not fold case: NTFS can enable case-sensitive directories.
			if seen[component] {
				continue
			}
			if len(g.handles) == maxLaunchHandles {
				return nil, fmt.Errorf("launch guard exceeds %d handles", maxLaunchHandles)
			}
			name, _ := windows.UTF16PtrFromString(component)
			// Attribute-only opens do not participate in data-sharing checks.
			// GENERIC_READ also requests list access on directory components.
			h, e := windows.CreateFile(name, windows.GENERIC_READ, windows.FILE_SHARE_READ, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
			if e != nil {
				return nil, fmt.Errorf("pin launch component %q (stop writers/updaters before starting): %w", component, e)
			}
			g.handles = append(g.handles, h)
			var info windows.ByHandleFileInformation
			if e := windows.GetFileInformationByHandle(h, &info); e != nil {
				return nil, fmt.Errorf("launch metadata %q: %w", component, e)
			}
			if info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
				return nil, fmt.Errorf("reparse point in launch path: %q", component)
			}
			directory := info.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY != 0
			if !directory && info.NumberOfLinks != 1 {
				return nil, fmt.Errorf("launch file must have exactly one hardlink: %q", component)
			}
			if !directory && i != len(components)-1 {
				return nil, fmt.Errorf("launch ancestor is not a directory: %q", component)
			}
			seen[component] = true
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return g, nil
}
