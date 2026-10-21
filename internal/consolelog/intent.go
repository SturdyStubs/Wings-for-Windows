package consolelog

import (
	"context"
	"errors"
	"sync"
	"wings-windows/internal/config"
)

type intentBackend interface {
	save(bool) error
	close() error
}

// Intent owns separate private running/stopped intent, never inferred from console text.
type Intent struct {
	mu       sync.Mutex
	disk     intentBackend
	failed   error
	closed   bool
	once     sync.Once
	done     chan struct{}
	closeErr error
}

func OpenIntent(options config.ConsoleLogs, server string) (*Intent, bool, error) {
	disk, running, err := openIntentDisk(options, server)
	if err != nil {
		return nil, false, err
	}
	return &Intent{disk: disk, done: make(chan struct{})}, running, nil
}

// Save completes file Sync before returning success. Call outside server locks.
// A slow OS write can delay the caller; cancellation never makes an unconfirmed
// intent safe to launch. Close has a bounded caller wait.
func (b *Intent) Save(running bool) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return errors.New("intent store closed")
	}
	if b.failed != nil {
		return b.failed
	}
	if err := b.disk.save(running); err != nil {
		b.failed = err
		return err
	}
	return nil
}
func (b *Intent) Close(ctx context.Context) error {
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
