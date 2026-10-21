package consolelog

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"golang.org/x/sys/windows"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
	"wings-windows/internal/config"
)

const testID = "11111111-1111-4111-8111-111111111111"

func options(t *testing.T) config.ConsoleLogs {
	return config.ConsoleLogs{Directory: filepath.Join(t.TempDir(), "console"), MaxFileBytes: 1 << 20, MaxFiles: 2}
}
func closeStore(t *testing.T, s *Store) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := s.Close(ctx); err != nil {
		t.Fatal(err)
	}
}
func TestConsoleHistorySurvivesReopen(t *testing.T) {
	p := options(t)
	s, lines, err := Open(p, testID)
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 0 {
		t.Fatal(lines)
	}
	want := []string{"first", "unicode \u2603\ninside record", "last"}
	for _, line := range want {
		s.Enqueue(line)
	}
	closeStore(t, s)
	s, lines, err = Open(p, testID)
	if err != nil {
		t.Fatal(err)
	}
	defer closeStore(t, s)
	if !slices.Equal(lines, want) || s.Status().Restored != 3 {
		t.Fatalf("bad restored history: %v", lines)
	}
}
func TestConsoleRotationAndQuota(t *testing.T) {
	p := options(t)
	disk, _, _, err := openDisk(p, testID)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 300; i++ {
		data, _ := json.Marshal(entry{Time: time.Now().UTC(), Line: fmt.Sprintf("%03d:", i) + strings.Repeat("x", 10000)})
		if err := disk.append(append(data, '\n')); err != nil {
			disk.close()
			t.Fatal(err)
		}
	}
	if err := disk.close(); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(filepath.Join(p.Directory, testID))
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".jsonl") {
			count++
			info, _ := e.Info()
			if info.Size() > p.MaxFileBytes {
				t.Fatal("quota exceeded")
			}
		}
	}
	if count != 2 {
		t.Fatalf("file count %d", count)
	}
	s, lines, err := Open(p, testID)
	if err != nil {
		t.Fatal(err)
	}
	defer closeStore(t, s)
	if len(lines) == 0 || !strings.HasPrefix(lines[len(lines)-1], "299:") {
		t.Fatal("newest record missing")
	}
	for _, line := range lines {
		if strings.HasPrefix(line, "000:") {
			t.Fatal("expired rotation retained")
		}
	}
}
func TestConsoleWriterOwnership(t *testing.T) {
	p := options(t)
	s, _, err := Open(p, testID)
	if err != nil {
		t.Fatal(err)
	}
	if second, _, err := Open(p, testID); err == nil {
		closeStore(t, second)
		closeStore(t, s)
		t.Fatal("duplicate writer accepted")
	}
	closeStore(t, s)
	s, _, err = Open(p, testID)
	if err != nil {
		t.Fatal(err)
	}
	closeStore(t, s)
}
func TestConsoleRejectsLinkedFiles(t *testing.T) {
	for _, name := range []string{"console.jsonl", "console.1.jsonl", ".writer.lock"} {
		t.Run(name, func(t *testing.T) {
			p := options(t)
			directory := filepath.Join(p.Directory, testID)
			if err := os.MkdirAll(directory, 0700); err != nil {
				t.Fatal(err)
			}
			source := filepath.Join(t.TempDir(), "outside.txt")
			os.WriteFile(source, []byte("untouched"), 0600)
			if err := os.Link(source, filepath.Join(directory, name)); err != nil {
				t.Fatal(err)
			}
			if s, _, err := Open(p, testID); err == nil {
				closeStore(t, s)
				t.Fatal("hardlinked log accepted")
			}
			data, _ := os.ReadFile(source)
			if string(data) != "untouched" {
				t.Fatal("outside file changed")
			}
		})
	}
}
func TestConsoleRejectsJunctionAncestor(t *testing.T) {
	p := options(t)
	outside := t.TempDir()
	if output, err := exec.Command("cmd.exe", "/c", "mklink", "/J", p.Directory, outside).CombinedOutput(); err != nil {
		t.Fatalf("junction: %v %s", err, output)
	}
	if s, _, err := Open(p, testID); err == nil {
		closeStore(t, s)
		t.Fatal("junction accepted")
	}
	if _, err := os.Stat(filepath.Join(outside, testID)); !os.IsNotExist(err) {
		t.Fatal("log directory created outside linked ancestor")
	}
}
func TestConsoleRepairsPartialTailAndSkipsCorruption(t *testing.T) {
	p := options(t)
	directory := filepath.Join(p.Directory, testID)
	os.MkdirAll(directory, 0700)
	valid, _ := json.Marshal(entry{Time: time.Now().UTC(), Line: "good"})
	data := append(valid, []byte("\ninvalid\n{partial")...)
	os.WriteFile(filepath.Join(directory, fileName(0)), data, 0600)
	s, lines, err := Open(p, testID)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(lines, []string{"good"}) || s.Status().Corrupt != 2 {
		t.Fatalf("corrupt history: %v %+v", lines, s.Status())
	}
	s.Enqueue("new")
	closeStore(t, s)
	s, lines, err = Open(p, testID)
	if err != nil {
		t.Fatal(err)
	}
	defer closeStore(t, s)
	if !slices.Equal(lines, []string{"good", "new"}) {
		t.Fatal(lines)
	}
}
func TestConsoleRejectsOversizedExistingFile(t *testing.T) {
	p := options(t)
	directory := filepath.Join(p.Directory, testID)
	os.MkdirAll(directory, 0700)
	path := filepath.Join(directory, fileName(0))
	os.WriteFile(path, []byte(strings.Repeat("x", (1<<20)+1)), 0600)
	if s, _, err := Open(p, testID); err == nil {
		closeStore(t, s)
		t.Fatal("oversized file accepted")
	}
	info, _ := os.Stat(path)
	if info.Size() != (1<<20)+1 {
		t.Fatal("oversized file modified")
	}
}
func TestConsoleTruncatesOnlyDiskRecord(t *testing.T) {
	p := options(t)
	s, _, err := Open(p, testID)
	if err != nil {
		t.Fatal(err)
	}
	s.Enqueue(strings.Repeat("a", maxLineBytes+100))
	closeStore(t, s)
	if s.Status().Truncated != 1 {
		t.Fatal("truncation not reported")
	}
	s, lines, err := Open(p, testID)
	if err != nil {
		t.Fatal(err)
	}
	defer closeStore(t, s)
	if len(lines) != 1 || !strings.HasSuffix(lines[0], "[disk log truncated]") {
		t.Fatal("disk record was not bounded")
	}
}

type blockedBackend struct {
	entered   chan struct{}
	release   chan struct{}
	once      sync.Once
	fail      bool
	closeFail bool
}

func (b *blockedBackend) append([]byte) error {
	if b.entered != nil {
		b.once.Do(func() { close(b.entered) })
		<-b.release
	}
	if b.fail {
		return os.ErrPermission
	}
	return nil
}
func (b *blockedBackend) close() error {
	if b.closeFail {
		return os.ErrPermission
	}
	return nil
}
func TestConsoleBackpressureAndBoundedClose(t *testing.T) {
	b := &blockedBackend{entered: make(chan struct{}), release: make(chan struct{})}
	s := newStore(b, 0)
	s.Enqueue("first")
	<-b.entered
	for i := 0; i < queueCapacity+20; i++ {
		s.Enqueue("queued")
	}
	if s.Status().Dropped != 20 {
		t.Fatal("queue overflow not counted")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if !errors.Is(s.Close(ctx), context.DeadlineExceeded) {
		t.Fatal("close did not respect deadline")
	}
	close(b.release)
	closeStore(t, s)
	if s.Status().Persisted != queueCapacity+1 {
		t.Fatal("queued records lost on graceful close")
	}
}
func TestConsoleReportsWriteAndCloseFailure(t *testing.T) {
	for _, kind := range []string{"write", "close"} {
		t.Run(kind, func(t *testing.T) {
			s := newStore(&blockedBackend{fail: kind == "write", closeFail: kind == "close"}, 0)
			s.Enqueue("record")
			if err := s.Close(context.Background()); err == nil {
				t.Fatal("failure hidden")
			}
			if s.Status().Error == "" {
				t.Fatal("status omitted error")
			}
		})
	}
}
func TestConsoleConcurrentCloseAndEnqueue(t *testing.T) {
	s := newStore(&blockedBackend{}, 0)
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				s.Enqueue("concurrent")
			}
		}()
	}
	closeStore(t, s)
	wg.Wait()
}

func TestConsoleNormalizesInvalidUTF8(t *testing.T) {
	p := options(t)
	s, _, err := Open(p, testID)
	if err != nil {
		t.Fatal(err)
	}
	s.Enqueue(strings.Repeat(string([]byte{255}), maxLineBytes))
	closeStore(t, s)
	s, lines, err := Open(p, testID)
	if err != nil {
		t.Fatal(err)
	}
	defer closeStore(t, s)
	if len(lines) != 1 || s.Status().Corrupt != 0 {
		t.Fatal("writer produced an unreadable record")
	}
}
func TestConsoleLockedRotationReportsFailure(t *testing.T) {
	p := options(t)
	s, _, err := Open(p, testID)
	if err != nil {
		t.Fatal(err)
	}
	// Hold the active file without delete sharing, as an incompatible viewer would.
	name, _ := windows.UTF16PtrFromString(filepath.Join(p.Directory, testID, fileName(0)))
	handle, err := windows.CreateFile(name, windows.GENERIC_READ, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	locked := os.NewFile(uintptr(handle), "locked log")
	if err != nil {
		t.Fatal(err)
	}
	defer locked.Close()
	for i := 0; i < 100; i++ {
		s.Enqueue(strings.Repeat("x", 16000))
		deadline := time.Now().Add(time.Second)
		for s.Status().Persisted < uint64(i+1) && s.Status().Error == "" && time.Now().Before(deadline) {
			time.Sleep(time.Millisecond)
		}
		if s.Status().Error != "" {
			break
		}
	}
	if err := s.Close(context.Background()); err == nil || s.Status().Error == "" {
		t.Fatal("locked rotation was not surfaced")
	}
}
