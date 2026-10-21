package identity

import (
	"context"
	"fmt"
	"path/filepath"

	"golang.org/x/sys/windows"
)

// CheckLaunchAccessContext checks effective DACL grants, not successful process
// creation or writes to existing children. It never impersonates, creates files
// or adjusts permissions. Ancestor traversal remains subject to the restricted
// token's Windows traverse privilege; it is not inferred from directory ACLs.
func CheckLaunchAccessContext(ctx context.Context, token windows.Token, plan LaunchAccess) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if plan.Executable == "" || plan.Directory == "" || plan.TempDirectory == "" {
		return fmt.Errorf("launch access requires executable, working directory and temporary directory")
	}
	if len(plan.RequiredFiles) > 256 {
		return fmt.Errorf("launch access exceeds 256 declared dependencies")
	}
	var impersonation windows.Token
	if err := windows.DuplicateTokenEx(token, windows.TOKEN_QUERY, nil, windows.SecurityImpersonation, windows.TokenImpersonation, &impersonation); err != nil {
		return fmt.Errorf("duplicate token for launch access: %w", err)
	}
	defer impersonation.Close()
	// Require directory enumeration, traversal, metadata and creation of files
	// and subdirectories. Existing children's modify/delete rights are separate.
	// FILE_WRITE_DATA / FILE_APPEND_DATA are the directory aliases
	// FILE_ADD_FILE / FILE_ADD_SUBDIRECTORY in the Windows access mask.
	const directoryAccess = windows.FILE_LIST_DIRECTORY | windows.FILE_TRAVERSE | windows.FILE_READ_ATTRIBUTES | windows.FILE_WRITE_DATA | windows.FILE_APPEND_DATA
	type requirement struct {
		path, label string
		mask        uint32
		directory   bool
	}
	requirements := []requirement{
		{plan.Executable, "executable read/execute", windows.FILE_GENERIC_READ | windows.FILE_GENERIC_EXECUTE, false},
		{plan.Directory, "working directory list/traverse/create", directoryAccess, true},
		{plan.TempDirectory, "temporary directory list/traverse/create", directoryAccess, true},
	}
	for _, file := range plan.RequiredFiles {
		requirements = append(requirements, requirement{file, "declared dependency read", windows.FILE_GENERIC_READ, false})
	}
	audit := pathAudit{ctx: ctx, limit: 100000}
	for _, required := range requirements {
		path := filepath.FromSlash(required.path)
		err := audit.withPath(path, 0, func(current string, sd *windows.SECURITY_DESCRIPTOR, _ uint32) error {
			if current != path {
				return nil
			}
			granted, err := grantedAccess(impersonation, sd)
			if err != nil {
				return err
			}
			if missing := required.mask &^ granted; missing != 0 {
				return fmt.Errorf("game identity lacks %s access to %q (missing mask 0x%x)", required.label, path, missing)
			}
			return nil
		}, func(directory bool) error {
			if directory != required.directory {
				return fmt.Errorf("unexpected file/directory type for %s: %q", required.label, path)
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("launch access (%s): %w", required.label, err)
		}
	}
	return ctx.Err()
}
