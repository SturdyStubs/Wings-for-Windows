//go:build !windows

package api

import "os"

func createTestLink(target, link string) error { return os.Symlink(target, link) }
