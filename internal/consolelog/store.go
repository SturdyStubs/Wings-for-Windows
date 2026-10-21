// Package consolelog persists bounded console history without blocking stdout.
package consolelog

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
	"wings-windows/internal/config"
)

const maxLineBytes = 16 << 10
const queueCapacity = 64

type entry struct {
	Time time.Time `json:"time"`
	Line string    `json:"line"`
}
type Status struct {
	Enabled   bool   `json:"enabled"`
	Persisted uint64 `json:"persisted"`
	Dropped   uint64 `json:"dropped"`
	Truncated uint64 `json:"truncated"`
	Corrupt   uint64 `json:"corrupt_records"`
	Restored  int    `json:"restored_lines"`
	Error     string `json:"error,omitempty"`
}
type backend interface {
	append([]byte) error
	close() error
}
type Store struct {
	mu     sync.Mutex
	queue  chan entry
	done   chan struct{}
	closed bool
	status Status
}

func Open(options config.ConsoleLogs, server string) (*Store, []string, error) {
	disk, history, corrupt, err := openDisk(options, server)
	if err != nil {
		return nil, nil, err
	}
	s := newStore(disk, corrupt)
	s.mu.Lock()
	s.status.Restored = len(history)
	s.mu.Unlock()
	return s, history, nil
}
func newStore(disk backend, corrupt uint64) *Store {
	s := &Store{queue: make(chan entry, queueCapacity), done: make(chan struct{}), status: Status{Enabled: true, Corrupt: corrupt}}
	go func() {
		defer close(s.done)
		failed := false
		for item := range s.queue {
			if failed {
				s.mu.Lock()
				s.status.Dropped++
				s.mu.Unlock()
				continue
			}
			data, err := json.Marshal(item)
			if err == nil {
				err = disk.append(append(data, '\n'))
			}
			s.mu.Lock()
			if err != nil {
				failed = true
				s.status.Error = "storage_write_failed"
				s.status.Dropped++
			} else {
				s.status.Persisted++
			}
			s.mu.Unlock()
		}
		if err := disk.close(); err != nil {
			s.mu.Lock()
			s.status.Error = "storage_close_failed"
			s.mu.Unlock()
		}
	}()
	return s
}
func (s *Store) Enqueue(line string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	if s.status.Error != "" {
		s.status.Dropped++
		return
	}
	truncated := len(line) > maxLineBytes
	if truncated {
		line = line[:maxLineBytes]
	}
	line = strings.ToValidUTF8(line, "\uFFFD")
	if len(line) > maxLineBytes {
		line = strings.ToValidUTF8(line[:maxLineBytes], "\uFFFD")
		truncated = true
	}
	if truncated {
		line = strings.Clone(line) + " [disk log truncated]"
		s.status.Truncated++
	}
	select {
	case s.queue <- entry{Time: time.Now().UTC(), Line: line}:
	default:
		s.status.Dropped++
	}
}
func (s *Store) Status() Status { s.mu.Lock(); defer s.mu.Unlock(); return s.status }
func (s *Store) Close(ctx context.Context) error {
	s.mu.Lock()
	if !s.closed {
		s.closed = true
		close(s.queue)
	}
	s.mu.Unlock()
	select {
	case <-s.done:
		if status := s.Status(); status.Error != "" {
			return fmt.Errorf("persistent console log: %s", status.Error)
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
