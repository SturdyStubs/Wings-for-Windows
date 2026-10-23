//go:build !windows

package api

import "os"

func openRenameSource(*os.File, string) (*os.File, error) { return nil, os.ErrInvalid }
func inspectRenameEntry(*os.File) error                   { return os.ErrInvalid }
func renameNoReplace(*os.File, *os.File, string) error    { return os.ErrInvalid }
