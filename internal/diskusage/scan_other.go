//go:build !windows

package diskusage

import (
	"context"
	"errors"
)

func Scan(context.Context, string) (Result, error) {
	return Result{}, errors.New("Windows disk measurement required")
}
