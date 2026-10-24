// Package diskusage measures logical file sizes, not allocated space or quotas.
package diskusage

import (
	"context"
	"sync"
	"time"
)

type Result struct {
	Bytes       int64 `json:"bytes"`
	Files       int   `json:"files"`
	Directories int   `json:"directories"`
}

type Status struct {
	Result
	Available     bool   `json:"available"`
	Scanning      bool   `json:"scanning"`
	Stale         bool   `json:"stale"`
	MeasuredAt    string `json:"measured_at,omitempty"`
	LastAttempt   string `json:"last_attempt,omitempty"`
	Error         string `json:"error,omitempty"`
	Scope         string `json:"scope"`
	QuotaEnforced bool   `json:"quota_enforced"`
}

type Meter struct {
	mu     sync.Mutex
	root   string
	slots  chan struct{}
	scan   func(context.Context, string) (Result, error)
	now    func() time.Time
	status Status
	next   time.Time
	closed bool
	cancel context.CancelFunc
	done   chan struct{}
}

func New(root string, slots chan struct{}) *Meter {
	return &Meter{root: root, slots: slots, scan: Scan, now: time.Now, status: Status{Scope: "data_root_logical_files", Stale: true}}
}

// Snapshot never walks disk on the caller. Failed scans preserve the last good
// result but explicitly mark it stale; a partial total is never published.
func (m *Meter) Snapshot() Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.now()
	if !m.next.IsZero() && !now.Before(m.next) {
		m.status.Stale = true
	}
	if m.closed || m.status.Scanning || now.Before(m.next) {
		return m.status
	}
	select {
	case m.slots <- struct{}{}:
	default:
		m.status.Stale = true
		m.status.Error = "Disk scan capacity reached"
		m.next = now.Add(5 * time.Second)
		return m.status
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	m.cancel = cancel
	m.done = make(chan struct{})
	m.status.Scanning = true
	m.status.LastAttempt = now.UTC().Format(time.RFC3339Nano)
	m.status.Error = ""
	go m.run(ctx, cancel)
	return m.status
}

func (m *Meter) run(ctx context.Context, cancel context.CancelFunc) {
	defer cancel()
	result, err := m.scan(ctx, m.root)
	if err == nil {
		err = ctx.Err()
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	<-m.slots
	m.status.Scanning = false
	now := m.now()
	if err != nil {
		m.status.Stale = true
		m.status.Error = "Disk scan incomplete or unavailable"
		m.next = now.Add(5 * time.Second)
	} else {
		m.status.Result = result
		m.status.Available = true
		m.status.Stale = false
		m.status.Error = ""
		m.status.MeasuredAt = now.UTC().Format(time.RFC3339Nano)
		m.next = now.Add(30 * time.Second)
	}
	close(m.done)
}

// Close cancels active work. The context bounds shutdown waiting; native disk
// calls remain cooperative and can outlive that wait.
func (m *Meter) Close(ctx context.Context) error {
	m.mu.Lock()
	m.closed = true
	if m.cancel != nil {
		m.cancel()
	}
	done := m.done
	m.mu.Unlock()
	if done == nil {
		return nil
	}
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
