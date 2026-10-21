package main

import (
	"context"
	"errors"
	"golang.org/x/sys/windows/svc"
	"strings"
	"sync"
	"testing"
	"time"
)

type recordedEvent struct {
	id      uint32
	text    string
	failure bool
}
type fakeEvents struct {
	mu      sync.Mutex
	records []recordedEvent
	closed  bool
	fail    bool
}

func (f *fakeEvents) record(id uint32, text string, failure bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.records = append(f.records, recordedEvent{id, text, failure})
	if f.fail {
		return errors.New("sink unavailable")
	}
	return nil
}
func (f *fakeEvents) Info(id uint32, text string) error  { return f.record(id, text, false) }
func (f *fakeEvents) Error(id uint32, text string) error { return f.record(id, text, true) }
func (f *fakeEvents) Close() error                       { f.mu.Lock(); defer f.mu.Unlock(); f.closed = true; return nil }

func waitServiceStatus(t *testing.T, status <-chan svc.Status, want svc.State) svc.Status {
	t.Helper()
	timeout := time.After(5 * time.Second)
	for {
		select {
		case s := <-status:
			if s.State == want {
				return s
			}
			t.Fatalf("unexpected service status %v, want %v", s.State, want)
		case <-timeout:
			t.Fatal("timed out waiting for service state")
		}
	}
}
func waitServiceExit(t *testing.T, done <-chan uint32, want uint32) {
	t.Helper()
	select {
	case got := <-done:
		if got != want {
			t.Fatalf("exit %d, want %d", got, want)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("service did not exit")
	}
}

func TestServiceReadinessStopAndInterrogate(t *testing.T) {
	for _, cmd := range []svc.Cmd{svc.Stop, svc.Shutdown} {
		t.Run(map[svc.Cmd]string{svc.Stop: "stop", svc.Shutdown: "shutdown"}[cmd], func(t *testing.T) {
			events := &fakeEvents{}
			requests := make(chan svc.ChangeRequest, 4)
			status := make(chan svc.Status, 16)
			done := make(chan uint32, 1)
			release := make(chan struct{})
			entered := make(chan struct{})
			h := handler{openEvents: func() (serviceEvents, error) { return events, nil }, runner: func(ctx context.Context, _ string, _ string, ready func()) error {
				close(entered)
				select {
				case <-release:
				case <-ctx.Done():
					return ctx.Err()
				}
				ready()
				ready()
				<-ctx.Done()
				return nil
			}}
			go func() { _, code := h.Execute(nil, requests, status); done <- code }()
			waitServiceStatus(t, status, svc.StartPending)
			<-entered
			requests <- svc.ChangeRequest{Cmd: svc.Interrogate, CurrentStatus: svc.Status{State: svc.Running}}
			waitServiceStatus(t, status, svc.StartPending)
			close(release)
			s := waitServiceStatus(t, status, svc.Running)
			if s.Accepts != svc.AcceptStop|svc.AcceptShutdown {
				t.Fatal(s)
			}
			requests <- svc.ChangeRequest{Cmd: cmd}
			waitServiceStatus(t, status, svc.StopPending)
			waitServiceExit(t, done, 0)
			events.mu.Lock()
			defer events.mu.Unlock()
			if !events.closed || len(events.records) != 4 {
				t.Fatalf("events %+v closed=%v", events.records, events.closed)
			}
			for i, id := range []uint32{100, 101, 102, 103} {
				if events.records[i].id != id || !strings.Contains(events.records[i].text, "pid=") {
					t.Fatal(events.records)
				}
			}
		})
	}
}

func TestServiceFailureEventsExcludeUntrustedDetails(t *testing.T) {
	for _, stage := range []string{"configuration", "panel_sync", "tls", "listener", "http", "shutdown", "secret-stage"} {
		t.Run(stage, func(t *testing.T) {
			events := &fakeEvents{}
			status := make(chan svc.Status, 16)
			h := handler{openEvents: func() (serviceEvents, error) { return events, nil }, runner: func(context.Context, string, string, func()) error {
				return &runtimeFailure{stage, errors.New("token=super-secret https://user:password@panel/private-path")}
			}}
			_, code := h.Execute(nil, make(chan svc.ChangeRequest), status)
			if code != 1 {
				t.Fatal(code)
			}
			for len(status) > 0 {
				if (<-status).State == svc.Running {
					t.Fatal("reported running before readiness")
				}
			}
			if !events.closed || len(events.records) != 2 || events.records[1].id != 200 || !events.records[1].failure {
				t.Fatal(events)
			}
			text := events.records[1].text
			if strings.Contains(text, "super-secret") || strings.Contains(text, "password") || strings.Contains(text, "private-path") || strings.Contains(text, "secret-stage") {
				t.Fatal(text)
			}
			want := stage
			if stage == "secret-stage" {
				want = "runtime"
			}
			if !strings.Contains(text, "stage="+want) {
				t.Fatal(text)
			}
		})
	}
}

func TestServiceDiagnosticsFailureDoesNotBlockStop(t *testing.T) {
	for _, mode := range []string{"open", "write", "startup-cancel", "closed-requests"} {
		t.Run(mode, func(t *testing.T) {
			events := &fakeEvents{fail: mode == "write"}
			requests := make(chan svc.ChangeRequest, 1)
			status := make(chan svc.Status, 16)
			done := make(chan uint32, 1)
			h := handler{openEvents: func() (serviceEvents, error) {
				if mode == "open" {
					return nil, errors.New("not registered")
				}
				return events, nil
			}, runner: func(ctx context.Context, _ string, _ string, ready func()) error {
				if mode != "startup-cancel" {
					ready()
				}
				<-ctx.Done()
				return &runtimeFailure{"panel_sync", ctx.Err()}
			}}
			go func() { _, code := h.Execute(nil, requests, status); done <- code }()
			waitServiceStatus(t, status, svc.StartPending)
			if mode != "startup-cancel" {
				waitServiceStatus(t, status, svc.Running)
			}
			if mode == "closed-requests" {
				close(requests)
			} else {
				requests <- svc.ChangeRequest{Cmd: svc.Stop}
			}
			waitServiceStatus(t, status, svc.StopPending)
			waitServiceExit(t, done, 0)
		})
	}
}
