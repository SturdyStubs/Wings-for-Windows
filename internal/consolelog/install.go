package consolelog

import (
	"context"
	"errors"
	"sync"
	"wings-windows/internal/config"
)

type installBackend interface {
	save(InstallRecord) error
	close() error
}

// InstallState owns the latest private installer receipt, never inferred from output.
type InstallState struct {
	mu       sync.Mutex
	disk     installBackend
	failed   error
	closed   bool
	once     sync.Once
	done     chan struct{}
	closeErr error
}

func OpenInstallState(options config.ConsoleLogs, server string) (*InstallState, InstallRecord, error) {
	disk, record, err := openInstallDisk(options, server)
	if err != nil {
		return nil, InstallRecord{}, err
	}
	return &InstallState{disk: disk, done: make(chan struct{})}, record, nil
}

// Save completes file Sync before returning success. Call outside server locks.
// A slow OS write can delay the caller; cancellation never makes an unconfirmed
// install result safe to notify. Close has a bounded caller wait.
func (b *InstallState) Save(record InstallRecord) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return errors.New("install store closed")
	}
	if b.failed != nil {
		return b.failed
	}
	if err := b.disk.save(record); err != nil {
		b.failed = err
		return err
	}
	return nil
}
func (b *InstallState) Close(ctx context.Context) error {
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
