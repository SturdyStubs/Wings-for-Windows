package api

import (
	"fmt"
	"golang.org/x/sys/windows"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

func fileCreated(info os.FileInfo) time.Time {
	if data, ok := info.Sys().(*syscall.Win32FileAttributeData); ok {
		return time.Unix(0, data.CreationTime.Nanoseconds())
	}
	return info.ModTime()
}
func finalPath(f *os.File) (string, error) {
	buffer := make([]uint16, 32768)
	count, err := windows.GetFinalPathNameByHandle(windows.Handle(f.Fd()), &buffer[0], uint32(len(buffer)), 0)
	if err != nil {
		return "", err
	}
	if count >= uint32(len(buffer)) {
		return "", fmt.Errorf("resolved file path exceeds supported length")
	}
	value := windows.UTF16ToString(buffer[:count])
	if strings.HasPrefix(value, `\\?\UNC\`) {
		value = `\\` + strings.TrimPrefix(value, `\\?\UNC\`)
	} else {
		value = strings.TrimPrefix(value, `\\?\`)
	}
	return value, nil
}
func resolvedFilePath(root *os.Root, f *os.File) (string, error) {
	base, err := root.Open(".")
	if err != nil {
		return "", err
	}
	defer base.Close()
	rootPath, err := finalPath(base)
	if err != nil {
		return "", err
	}
	filePath, err := finalPath(f)
	if err != nil {
		return "", err
	}
	relative, err := filepath.Rel(rootPath, filePath)
	if err != nil || relative == ".." || strings.HasPrefix(relative, `..\`) {
		return "", fmt.Errorf("file resolves outside server root")
	}
	return filepath.ToSlash(relative), nil
}
