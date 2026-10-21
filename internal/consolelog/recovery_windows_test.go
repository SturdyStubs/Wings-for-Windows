package consolelog

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"wings-windows/internal/config"
)

const budgetTestID = "11111111-1111-4111-8111-111111111111"

func budgetOptions(t *testing.T) config.ConsoleLogs {
	t.Helper()
	return config.ConsoleLogs{Directory: filepath.Join(t.TempDir(), "console"), MaxFileBytes: 1 << 20, MaxFiles: 2}
}
func closeBudget(t *testing.T, b *RecoveryBudget) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := b.Close(ctx); err != nil {
		t.Fatal(err)
	}
}
func TestRecoveryBudgetRoundTripAndConsoleCoexistence(t *testing.T) {
	options := budgetOptions(t)
	console, _, err := Open(options, budgetTestID)
	if err != nil {
		t.Fatal(err)
	}
	defer console.Close(context.Background())
	b, history, err := OpenRecovery(options, budgetTestID)
	if err != nil || len(history) != 0 {
		t.Fatal(history, err)
	}
	if other, _, err := OpenRecovery(options, budgetTestID); err == nil {
		other.Close(context.Background())
		t.Fatal("duplicate writer accepted")
	}
	now := time.Now().UTC()
	var expected []time.Time
	for i := range 10 {
		expected = append(expected, now.Add(time.Duration(i)*time.Second))
		if err := b.Save(expected); err != nil {
			t.Fatal(err)
		}
	}
	closeBudget(t, b)
	b, history, err = OpenRecovery(options, budgetTestID)
	if err != nil {
		t.Fatal(err)
	}
	defer closeBudget(t, b)
	if len(history) != 10 {
		t.Fatal(history)
	}
	for i := range history {
		if !history[i].Equal(expected[i]) {
			t.Fatal(history)
		}
	}
	for i := range 2 {
		info, err := os.Stat(filepath.Join(options.Directory, budgetTestID, []string{"recovery.0.json", "recovery.1.json"}[i]))
		if err != nil || info.Size() > 4096 {
			t.Fatal(info, err)
		}
	}
}
func TestRecoveryBudgetRejectsCorruptState(t *testing.T) {
	cases := []string{"json", "checksum", "uuid", "version", "unknown", "oversize", "missing-slot", "parity", "sequence-gap", "zero-time", "too-many", "trailing"}
	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			options := budgetOptions(t)
			root := filepath.Join(options.Directory, budgetTestID)
			if err := os.MkdirAll(root, 0700); err != nil {
				t.Fatal(err)
			}
			raw, err := snapshotBytes(budgetPayload{1, budgetTestID, 1, []time.Time{time.Now().UTC()}})
			if err != nil {
				t.Fatal(err)
			}
			target := "recovery.1.json"
			switch name {
			case "json":
				raw = []byte(`{"payload":`)
			case "checksum":
				raw = []byte(strings.Replace(string(raw), `"sha256":"`, `"sha256":"bad`, 1))
			case "uuid":
				raw, _ = snapshotBytes(budgetPayload{1, "22222222-2222-4222-8222-222222222222", 1, nil})
			case "version":
				raw, _ = snapshotBytes(budgetPayload{2, budgetTestID, 1, nil})
			case "unknown":
				raw = []byte(strings.Replace(string(raw), `"payload":`, `"extra":true,"payload":`, 1))
			case "oversize":
				raw = []byte(strings.Repeat("x", 4097))
			case "missing-slot":
				target = "recovery.0.json"
				raw, _ = snapshotBytes(budgetPayload{1, budgetTestID, 2, nil})
			case "parity":
				target = "recovery.0.json"
			case "sequence-gap":
				other, _ := snapshotBytes(budgetPayload{1, budgetTestID, 4, nil})
				os.WriteFile(filepath.Join(root, "recovery.0.json"), other, 0600)
			case "zero-time":
				raw = []byte(`{"payload":{"version":1,"server":"` + budgetTestID + `","sequence":1,"attempts":["0001-01-01T00:00:00Z"]},"sha256":"bad"}`)
			case "too-many":
				raw = []byte(`{"payload":{"version":1,"server":"` + budgetTestID + `","sequence":1,"attempts":[` + strings.TrimSuffix(strings.Repeat(`"2026-01-01T00:00:00Z",`, 11), ",") + `]},"sha256":"bad"}`)
			case "trailing":
				raw = append(raw, []byte(" {}")...)
			}
			if err := os.WriteFile(filepath.Join(root, target), raw, 0600); err != nil {
				t.Fatal(err)
			}
			if b, _, err := OpenRecovery(options, budgetTestID); err == nil {
				closeBudget(t, b)
				t.Fatal("accepted damaged state")
			}
			// Failure must release lease/guard handles without modifying the damaged file.
			read, err := os.ReadFile(filepath.Join(root, target))
			if err != nil || string(read) != string(raw) {
				t.Fatal("changed damaged snapshot", err)
			}
			if err := os.Remove(filepath.Join(root, target)); err != nil {
				t.Fatal(err)
			}
		})
	}
}
func TestRecoveryBudgetRejectsHardlinkSlots(t *testing.T) {
	for _, name := range []string{".recovery.lock", "recovery.0.json", "recovery.1.json"} {
		t.Run(name, func(t *testing.T) {
			options := budgetOptions(t)
			root := filepath.Join(options.Directory, budgetTestID)
			os.MkdirAll(root, 0700)
			source := filepath.Join(t.TempDir(), "outside")
			os.WriteFile(source, nil, 0600)
			if err := os.Link(source, filepath.Join(root, name)); err != nil {
				t.Fatal(err)
			}
			if b, _, err := OpenRecovery(options, budgetTestID); err == nil {
				closeBudget(t, b)
				t.Fatal("hardlink accepted")
			}
		})
	}
}

type failedBudgetDisk struct {
	calls   int
	blocked chan struct{}
}

func (d *failedBudgetDisk) save([]time.Time) error {
	d.calls++
	if d.blocked != nil {
		<-d.blocked
	}
	return errors.New("disk failure")
}
func (d *failedBudgetDisk) close() error { return nil }
func TestRecoveryBudgetStickyFailureAndBoundedClose(t *testing.T) {
	disk := &failedBudgetDisk{blocked: make(chan struct{})}
	b := &RecoveryBudget{disk: disk, done: make(chan struct{})}
	done := make(chan error, 1)
	go func() { done <- b.Save(nil) }()
	// Wait on the mutex rather than reading a worker-owned test counter.
	deadline := time.Now().Add(time.Second)
	for b.mu.TryLock() {
		b.mu.Unlock()
		if time.Now().After(deadline) {
			t.Fatal("save did not start")
		}
		time.Sleep(time.Millisecond)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if err := b.Close(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal(err)
	}
	close(disk.blocked)
	if err := <-done; err == nil {
		t.Fatal("lost save failure")
	}
	closeBudget(t, b)
	if err := b.Save(nil); err == nil {
		t.Fatal("closed budget accepted save")
	}
	if disk.calls != 1 {
		t.Fatal(disk.calls)
	}
	disk = &failedBudgetDisk{}
	b = &RecoveryBudget{disk: disk, done: make(chan struct{})}
	if b.Save(nil) == nil || b.Save(nil) == nil || disk.calls != 1 {
		t.Fatal("write failure was not sticky")
	}
	closeBudget(t, b)
}
