//go:build !windows

package consolelog

import (
	"fmt"
	"time"
	"wings-windows/internal/config"
)

func openDisk(config.ConsoleLogs, string) (backend, []string, uint64, error) {
	return nil, nil, 0, fmt.Errorf("persistent console logs require Windows")
}

func openRecoveryDisk(config.ConsoleLogs, string) (recoveryDisk, []time.Time, error) {
	return nil, nil, fmt.Errorf("persistent recovery requires Windows")
}

func openIntentDisk(config.ConsoleLogs, string) (intentBackend, bool, error) {
	return nil, false, fmt.Errorf("persistent desired state requires Windows")
}
