//go:build !windows

package main

func serviceMain(_, _ string) bool { return false }
