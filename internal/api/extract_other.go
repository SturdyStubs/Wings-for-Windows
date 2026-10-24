//go:build !windows

package api

import "os"

func createExtractDirectory(*os.File, string) (*os.File, error) { return nil, os.ErrInvalid }
func createExtractStage(*os.File, string) (*os.File, error)     { return nil, os.ErrInvalid }
