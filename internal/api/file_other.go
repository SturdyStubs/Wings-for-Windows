//go:build !windows

package api

import (
	"os"
	"path/filepath"
	"time"
)

func fileCreated(info os.FileInfo) time.Time { return info.ModTime() }
func fileIsReparse(info os.FileInfo) bool    { return info.Mode()&os.ModeSymlink != 0 }
func resolvedFilePath(root *os.Root, f *os.File) (string, error) {
	name, err := filepath.EvalSymlinks(f.Name())
	if err != nil {
		return "", err
	}
	return filepath.Rel(root.Name(), name)
}
