//go:build !windows

package api

import "os"

func openCopyHandle(*os.File, string, bool) (*os.File, error) { return nil, os.ErrInvalid }
func discardCopyStage(*os.File) error                         { return os.ErrInvalid }
func copyDiskFull(error) bool                                 { return false }
