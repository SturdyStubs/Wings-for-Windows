//go:build !windows

package logtail

import (
	"context"
	"errors"
)

type Reader struct{}

func Open(context.Context, string, string, []string) (*Reader, error) {
	return nil, errors.New("log-file adapter requires Windows")
}
func (*Reader) Close() error { return nil }
func (*Reader) Poll(context.Context) ([]string, error) {
	return nil, errors.New("log-file adapter requires Windows")
}
