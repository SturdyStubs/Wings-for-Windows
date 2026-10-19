package native

import (
	"bufio"
	"encoding/json"
	"fmt"
	"golang.org/x/sys/windows"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

var fixture string

func TestMain(m *testing.M) {
	directory, err := os.MkdirTemp("", "native-runtime-test-")
	if err != nil {
		panic(err)
	}
	fixture = filepath.Join(directory, "fixture.exe")
	cmd := exec.Command("go", "build", "-o", fixture, "../../cmd/testserver")
	if b, err := cmd.CombinedOutput(); err != nil {
		panic(fmt.Sprintf("fixture build: %v %s", err, b))
	}
	code := m.Run()
	os.RemoveAll(directory)
	os.Exit(code)
}
func launch(t *testing.T, mode string) *Process {
	t.Helper()
	p, err := Start(Spec{Executable: fixture, Arguments: []string{mode}, Environment: []string{"SystemRoot=" + os.Getenv("SystemRoot")}, Directory: t.TempDir(), MemoryMB: 128, CPUPercent: 100})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		p.Kill()
		select {
		case <-p.Done:
		case <-time.After(5 * time.Second):
			t.Error("process did not exit")
		}
		p.Stdout.Close()
	})
	return p
}
func lines(p *Process) <-chan string {
	out := make(chan string, 16)
	go func() {
		defer close(out)
		scanner := bufio.NewScanner(p.Stdout)
		for scanner.Scan() {
			out <- scanner.Text()
		}
	}()
	return out
}
func line(t *testing.T, in <-chan string) string {
	t.Helper()
	select {
	case value := <-in:
		return value
	case <-time.After(5 * time.Second):
		t.Fatal("timed out reading fixture output")
		return ""
	}
}
func TestNativeConsoleAndGracefulStop(t *testing.T) {
	p := launch(t, "")
	out := lines(p)
	if got := line(t, out); got != "SERVER_READY" {
		t.Fatal(got)
	}
	if err := p.Command(`say "hello world" café`); err != nil {
		t.Fatal(err)
	}
	if got := line(t, out); got != `ECHO:say "hello world" café` {
		t.Fatal(got)
	}
	stats, err := p.Stats()
	if err != nil || stats.MemoryBytes == 0 {
		t.Fatalf("working-set metrics: %+v, %v", stats, err)
	}
	if err = p.Command("stop"); err != nil {
		t.Fatal(err)
	}
	select {
	case <-p.Done:
		if p.ExitCode != 0 {
			t.Fatalf("exit %d", p.ExitCode)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("graceful stop timed out")
	}
}
func TestJobObjectKillsDescendants(t *testing.T) {
	for _, action := range []string{"kill", "parent exit"} {
		t.Run(action, func(t *testing.T) {
			p := launch(t, "spawn")
			out := lines(p)
			line(t, out)
			child := line(t, out)
			pid, err := strconv.Atoi(strings.TrimPrefix(child, "CHILD:"))
			if err != nil {
				t.Fatal(child)
			}
			handle, err := windows.OpenProcess(windows.SYNCHRONIZE|windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
			if err != nil {
				t.Fatal(err)
			}
			defer windows.CloseHandle(handle)
			if action == "kill" {
				err = p.Kill()
			} else {
				err = p.Command("stop")
			}
			if err != nil {
				t.Fatal(err)
			}
			select {
			case <-p.Done:
			case <-time.After(5 * time.Second):
				t.Fatal("parent alive")
			}
			status, err := windows.WaitForSingleObject(handle, 5000)
			if err != nil || status != windows.WAIT_OBJECT_0 {
				t.Fatalf("descendant escaped job: %d %v", status, err)
			}
		})
	}
}
func TestStartRejectsMissingExecutable(t *testing.T) {
	if _, err := Start(Spec{Executable: filepath.Join(t.TempDir(), "missing.exe"), Directory: t.TempDir()}); err == nil {
		t.Fatal("missing executable accepted")
	}
}
func TestWindowsArgumentQuoting(t *testing.T) {
	args := []string{"space in name", `a"quoted"value`, `C:\path with spaces\`, "café", ""}
	p, err := Start(Spec{Executable: fixture, Arguments: append([]string{"arguments"}, args...), Environment: []string{"SystemRoot=" + os.Getenv("SystemRoot")}, Directory: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { p.Kill(); <-p.Done; p.Stdout.Close() }()
	out := lines(p)
	line(t, out)
	value := strings.TrimPrefix(line(t, out), "ARGS:")
	var actual []string
	if err = json.Unmarshal([]byte(value), &actual); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(actual, args) {
		t.Fatalf("argument corruption: %#v", actual)
	}
}
func TestUnreadConsoleInputIsBounded(t *testing.T) {
	p := launch(t, "noinput")
	out := lines(p)
	line(t, out)
	started := time.Now()
	if err := p.Command(strings.Repeat("x", 4096)); err == nil {
		t.Fatal("expected full-pipe write to time out")
	}
	if time.Since(started) > 3*time.Second {
		t.Fatal("console input blocked the caller")
	}
	if err := p.Kill(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-p.Done:
	case <-time.After(5 * time.Second):
		t.Fatal("blocked writer prevented process cleanup")
	}
}
