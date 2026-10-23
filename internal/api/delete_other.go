//go:build !windows

package api

import "os"

func deletePinnedEntry(*os.File) error { return os.ErrInvalid }
func deleteWindowsStatus(error) int    { return 0 }
