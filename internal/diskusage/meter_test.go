package diskusage

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func waitMeter(t *testing.T, m *Meter) Status {
	t.Helper()
	until := time.Now().Add(3 * time.Second)
	for time.Now().Before(until) {
		m.mu.Lock()
		s := m.status
		m.mu.Unlock()
		if !s.Scanning {
			return s
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("scan did not complete")
	return Status{}
}

func TestMeterAsyncCacheRetryAndLastGood(t *testing.T) {
	m := New("unused", make(chan struct{}, 2))
	now := time.Now()
	m.now = func() time.Time { return now }
	gate := make(chan struct{})
	calls := 0
	m.scan = func(context.Context, string) (Result, error) {
		calls++
		<-gate
		if calls == 2 {
			return Result{Bytes: 999}, errors.New("sensitive path or credential")
		}
		return Result{Bytes: 123, Files: 1}, nil
	}
	first := m.Snapshot()
	if !first.Scanning || first.Available || !first.Stale {
		t.Fatal(first)
	}
	for i := 0; i < 10; i++ {
		if !m.Snapshot().Scanning {
			t.Fatal("duplicate scan")
		}
	}
	close(gate)
	s := waitMeter(t, m)
	if !s.Available || s.Stale || s.Bytes != 123 || calls != 1 {
		t.Fatal(s, calls)
	}
	if s = m.Snapshot(); s.Scanning {
		t.Fatal("cache not used")
	}
	now = now.Add(31 * time.Second)
	m.Snapshot()
	s = waitMeter(t, m)
	if !s.Available || !s.Stale || s.Bytes != 123 || s.Error == "" || strings.Contains(s.Error, "sensitive") {
		t.Fatal(s)
	}
	if m.Snapshot().Scanning {
		t.Fatal("failure backoff ignored")
	}
	now = now.Add(6 * time.Second)
	m.Snapshot()
	s = waitMeter(t, m)
	if s.Stale || s.Error != "" || calls != 3 {
		t.Fatal(s, calls)
	}
	if e := m.Close(context.Background()); e != nil {
		t.Fatal(e)
	}
}

func TestMeterCapacityAndShutdown(t *testing.T) {
	slots := make(chan struct{}, 2)
	meters := []*Meter{New("one", slots), New("two", slots), New("three", slots)}
	for _, m := range meters {
		m.scan = func(ctx context.Context, _ string) (Result, error) { <-ctx.Done(); return Result{}, ctx.Err() }
	}
	if !meters[0].Snapshot().Scanning || !meters[1].Snapshot().Scanning {
		t.Fatal("capacity unavailable")
	}
	s := meters[2].Snapshot()
	if s.Scanning || s.Available || s.Error == "" {
		t.Fatal(s)
	}
	for _, m := range meters {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		if e := m.Close(ctx); e != nil {
			t.Fatal(e)
		}
		cancel()
		if m.Snapshot().Scanning {
			t.Fatal("closed meter restarted")
		}
	}
	if len(slots) != 0 {
		t.Fatal("slot leaked")
	}
}

func TestMeterCloseWaitBound(t *testing.T) {
	m := New("unused", make(chan struct{}, 1))
	gate := make(chan struct{})
	m.scan = func(context.Context, string) (Result, error) { <-gate; return Result{}, errors.New("blocked") }
	m.Snapshot()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if !errors.Is(m.Close(ctx), context.Canceled) {
		t.Fatal("shutdown wait ignored context")
	}
	close(gate)
	waitMeter(t, m)
	if e := m.Close(context.Background()); e != nil {
		t.Fatal(e)
	}
}
