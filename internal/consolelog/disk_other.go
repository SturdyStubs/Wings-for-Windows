//go:build !windows

package consolelog

import (
	"fmt"
	"wings-windows/internal/config"
)

func openDisk(config.ConsoleLogs, string) (backend, []string, uint64, error) {
	return nil, nil, 0, fmt.Errorf("persistent console logs require Windows")
}
