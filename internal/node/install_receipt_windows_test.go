package node

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"wings-windows/internal/config"
	"wings-windows/internal/consolelog"
)

func receiptConfig(t *testing.T) (config.Config, *atomic.Bool, *atomic.Int32, <-chan installResult) {
	t.Helper()
	reject, count := &atomic.Bool{}, &atomic.Int32{}
	results := make(chan installResult, 16)
	remote := RemoteServer{UUID: testID, Settings: Settings{UUID: testID, Environment: map[string]any{}}}
	panel := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test."+strings.Repeat("s", 32) {
			t.Error("incorrect receipt callback credentials")
			w.WriteHeader(401)
			return
		}
		if r.Method == "POST" {
			count.Add(1)
			var result installResult
			if err := json.NewDecoder(r.Body).Decode(&result); err != nil {
				t.Error(err)
			}
			results <- result
			if reject.Load() {
				w.WriteHeader(503)
			} else {
				w.WriteHeader(204)
			}
			return
		}
		if strings.HasSuffix(r.URL.Path, "/servers") {
			json.NewEncoder(w).Encode(map[string]any{"data": []RemoteServer{remote}})
		} else {
			json.NewEncoder(w).Encode(remote)
		}
	}))
	t.Cleanup(panel.Close)
	c := config.Config{PanelURL: panel.URL, TokenID: "test", Token: strings.Repeat("s", 32), DataDirectory: t.TempDir(), ConsoleLogs: &config.ConsoleLogs{Directory: filepath.Join(t.TempDir(), "private-logs"), MaxFileBytes: 1 << 20, MaxFiles: 2}, Profiles: map[string]config.Profile{testID: {Executable: fixture, ReadyPattern: "SERVER_READY", StopCommand: "stop", StopTimeoutSeconds: 1, Installer: &config.Installer{Executable: fixture, Arguments: []string{"install", "success"}, TimeoutSeconds: 5, MemoryMB: 128, PersistState: true}}}}
	return c, reject, count, results
}
func receiptManager(t *testing.T, c config.Config) (*Manager, *Server) {
	t.Helper()
	m := NewManager(c)
	if err := m.Sync(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(m.Close)
	return m, m.Get(testID)
}
func TestInstallReceiptRetryAfterRestartNeverRerunsInstaller(t *testing.T) {
	c, reject, count, results := receiptConfig(t)
	reject.Store(true)
	m, s := receiptManager(t, c)
	if err := s.EnqueueInstall(true); err != nil {
		t.Fatal(err)
	}
	waitInstall(t, s)
	if r := <-results; !r.Successful || !r.Reinstall {
		t.Fatal(r)
	}
	if s.installRecord.Phase != "result" || s.installRecord.AttemptID == "" {
		t.Fatal(s.Response())
	}
	if err := s.EnqueueInstall(false); !errors.Is(err, ErrInstallUnresolved) {
		t.Fatal(err)
	}
	marker := filepath.Join(s.Root, "install.marker")
	if err := os.WriteFile(marker, []byte("NEVER_RERUN"), 0600); err != nil {
		t.Fatal(err)
	}
	attempt := s.installRecord.AttemptID
	m.Close()
	reject.Store(false)
	m, s = receiptManager(t, c)
	if count.Load() != 1 || s.installStage != "awaiting_notification" {
		t.Fatal("automatic retry or lost receipt", count.Load(), s.Response())
	}
	if err := s.ReconcileInstall(context.Background(), newInstallAttempt(), false); !errors.Is(err, ErrInstallAttempt) {
		t.Fatal(err)
	}
	if err := s.ReconcileInstall(context.Background(), attempt, true); !errors.Is(err, ErrInstallNoNotification) {
		t.Fatal(err)
	}
	if err := s.ReconcileInstall(context.Background(), attempt, false); err != nil {
		t.Fatal(err)
	}
	if r := <-results; !r.Successful || !r.Reinstall {
		t.Fatal(r)
	}
	if err := s.ReconcileInstall(context.Background(), attempt, false); !errors.Is(err, ErrInstallNoNotification) {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(marker); err != nil || string(b) != "NEVER_RERUN" {
		t.Fatal("notification reran installer", err)
	}
	m.Close()
	_, s = receiptManager(t, c)
	if s.installRecord.Phase != "acknowledged" || count.Load() != 2 {
		t.Fatal(s.Response(), count.Load())
	}
	if err := s.EnqueueInstall(false); err != nil {
		t.Fatal(err)
	}
	waitInstall(t, s)
	if s.installRecord.AttemptID == attempt || s.installRecord.Phase != "acknowledged" {
		t.Fatal(s.Response())
	}
}
func TestInterruptedInstallRequiresExplicitFailureResolution(t *testing.T) {
	c, _, count, results := receiptConfig(t)
	m, s := receiptManager(t, c)
	if err := s.beginInstallRecord(true); err != nil {
		t.Fatal(err)
	}
	attempt := s.installRecord.AttemptID
	m.Close()
	_, s = receiptManager(t, c)
	if s.installStage != "interrupted" || count.Load() != 0 {
		t.Fatal(s.Response())
	}
	if err := s.Power("start"); !errors.Is(err, ErrInstallUnresolved) {
		t.Fatal(err)
	}
	if err := s.EnqueuePower("restart", 0); !errors.Is(err, ErrInstallUnresolved) {
		t.Fatal(err)
	}
	if err := s.EnqueueInstall(false); !errors.Is(err, ErrInstallUnresolved) {
		t.Fatal(err)
	}
	if err := s.ReconcileInstall(context.Background(), attempt, false); !errors.Is(err, ErrInstallNoNotification) {
		t.Fatal(err)
	}
	if err := s.ReconcileInstall(context.Background(), attempt, true); err != nil {
		t.Fatal(err)
	}
	if r := <-results; r.Successful || !r.Reinstall {
		t.Fatal(r)
	}
	if _, err := os.Stat(filepath.Join(s.Root, "install.marker")); !os.IsNotExist(err) {
		t.Fatal("resolution executed installer", err)
	}
}

type failInstallStore struct {
	installPersistence
	at, calls int
}

func (f *failInstallStore) Save(record consolelog.InstallRecord) error {
	f.calls++
	if f.calls == f.at {
		return errors.New("synthetic disk failure")
	}
	return f.installPersistence.Save(record)
}
func TestInstallReceiptStorageFailuresFailClosed(t *testing.T) {
	for _, step := range []int{1, 2, 3} {
		t.Run([]string{"", "begin", "result", "acknowledgement"}[step], func(t *testing.T) {
			c, _, count, _ := receiptConfig(t)
			_, s := receiptManager(t, c)
			s.installStore = &failInstallStore{installPersistence: s.installStore, at: step}
			if err := s.EnqueueInstall(false); err != nil {
				t.Fatal(err)
			}
			waitInstall(t, s)
			if s.installStage != "result_unconfirmed" || s.installStorageError == "" {
				t.Fatal(s.Response())
			}
			expected := int32(0)
			if step == 3 {
				expected = 1
			}
			if count.Load() != expected {
				t.Fatal("unconfirmed result notified", count.Load())
			}
			_, err := os.Stat(filepath.Join(s.Root, "install.marker"))
			if step == 1 && !os.IsNotExist(err) {
				t.Fatal("unconfirmed begin executed installer", err)
			}
			if err := s.EnqueueInstall(false); !errors.Is(err, ErrInstallStorage) {
				t.Fatal(err)
			}
		})
	}
}
