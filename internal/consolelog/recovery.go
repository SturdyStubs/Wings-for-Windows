package consolelog

import (
	"context"
	"errors"
	"sync"
	"time"
	"wings-windows/internal/config"
)

type recoveryDisk interface {
	save([]time.Time) error
	close() error
}

// RecoveryBudget owns a separate private budget, never inferred from console text.
type RecoveryBudget struct {
	mu       sync.Mutex
	disk     recoveryDisk
	failed   error
	closed   bool
	once     sync.Once
	done     chan struct{}
	closeErr error
}

func OpenRecovery(options config.ConsoleLogs, server string) (*RecoveryBudget, []time.Time, error) {
	disk, history, err := openRecoveryDisk(options, server)
	if err != nil {
		return nil, nil, err
	}
	return &RecoveryBudget{disk: disk, done: make(chan struct{})}, history, nil
}

// Save completes file Sync before returning success. Call outside server locks.
// A slow OS write can delay the caller; cancellation never makes an unconfirmed
// reservation safe to launch. Close has a bounded caller wait.
func (b *RecoveryBudget) Save(history []time.Time) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return errors.New("recovery budget closed")
	}
	if b.failed != nil {
		return b.failed
	}
	if err := b.disk.save(history); err != nil {
		b.failed = err
		return err
	}
	return nil
}
func (b *RecoveryBudget) Close(ctx context.Context) error {
	b.once.Do(func() {
		go func() { b.mu.Lock(); b.closed = true; b.closeErr = b.disk.close(); b.mu.Unlock(); close(b.done) }()
	})
	select {
	case <-b.done:
		return b.closeErr
	case <-ctx.Done():
		return ctx.Err()
	}
}
