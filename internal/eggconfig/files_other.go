//go:build !windows

package eggconfig

import (
	"context"
	"fmt"
)

func Apply(context.Context, string, []File, []string) error {
	return fmt.Errorf("egg rewriting requires Windows")
}
