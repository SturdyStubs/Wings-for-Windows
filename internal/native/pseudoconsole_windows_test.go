package native

import (
	"bytes"
	"golang.org/x/sys/windows"
	"io"
	"os"
	"strings"
	"testing"
	"time"
	"wings-windows/internal/identity"
)

func TestPseudoconsoleWindowsConsoleRoundTrip(t *testing.T) {
	p, e := Start(Spec{Pseudoconsole: true, Executable: fixture, Arguments: []string{"winconsole"}, Directory: t.TempDir(), Environment: []string{"SystemRoot=" + os.Getenv("SystemRoot")}, MemoryMB: 128})
	if e != nil {
		t.Fatal(e)
	}
	defer p.Kill()
	defer p.Stdout.Close()
	output := make(chan string, 64)
	go func() {
		defer close(output)
		b := make([]byte, 4096)
		for {
			n, e := p.Stdout.Read(b)
			if n > 0 {
				output <- string(b[:n])
			}
			if e != nil {
				return
			}
		}
	}()
	wait := func(needle string) {
		t.Helper()
		var data string
		timer := time.NewTimer(5 * time.Second)
		defer timer.Stop()
		for {
			select {
			case s, ok := <-output:
				if !ok {
					<-p.Done
					t.Fatal("console ended before", needle, data, "exit", p.ExitCode)
				}
				data += s
				if strings.Contains(data, needle) {
					return
				}
				if len(data) > 64<<10 {
					t.Fatal("excess console data")
				}
			case <-timer.C:
				t.Fatal("console timeout", needle, data)
			}
		}
	}
	wait("WINCON_READY")
	if e = p.Command("hello"); e != nil {
		t.Fatal(e)
	}
	wait("WINCON_ECHO:hello")
	if e = p.Command("stop"); e != nil {
		t.Fatal(e)
	}
	wait("WINCON_STOPPED")
	select {
	case <-p.Done:
	case <-time.After(5 * time.Second):
		t.Fatal("console process did not exit")
	}
	if p.ExitCode != 0 || p.CleanupError != nil {
		t.Fatal(p.ExitCode, p.CleanupError)
	}
}
func TestPseudoconsoleKillWithoutOutputConsumer(t *testing.T) {
	p, e := Start(Spec{Pseudoconsole: true, Executable: fixture, Arguments: []string{"winconsole"}, Directory: t.TempDir(), Environment: []string{"SystemRoot=" + os.Getenv("SystemRoot")}})
	if e != nil {
		t.Fatal(e)
	}
	defer p.Stdout.Close()
	defer p.Kill()
	time.Sleep(100 * time.Millisecond)
	if e = p.Kill(); e != nil {
		t.Fatal(e)
	}
	select {
	case <-p.Done:
	case <-time.After(5 * time.Second):
		t.Fatal("undrained console blocked kill")
	}
}
func TestPseudoconsoleOutputBoundAndConsumerClose(t *testing.T) {
	out := newTerminalOutput()
	out.append(bytes.Repeat([]byte("x"), terminalBufferBytes*2))
	out.mu.Lock()
	n := len(out.data)
	out.mu.Unlock()
	if n > terminalBufferBytes {
		t.Fatal(n)
	}
	out.finish()
	b, e := io.ReadAll(out)
	if e != nil || !bytes.Contains(b, []byte("output truncated")) {
		t.Fatal(e)
	}
	out.Close()
	out.append([]byte("after close"))
	if n, e := out.Read(make([]byte, 10)); n != 0 || e != io.EOF {
		t.Fatal(n, e)
	}
}
func TestPseudoconsolePipeModeRefusesNativeConsole(t *testing.T) {
	p, e := Start(Spec{Executable: fixture, Arguments: []string{"winconsole"}, Directory: t.TempDir(), Environment: []string{"SystemRoot=" + os.Getenv("SystemRoot")}})
	if e != nil {
		t.Fatal(e)
	}
	defer p.Kill()
	defer p.Stdout.Close()
	select {
	case <-p.Done:
	case <-time.After(5 * time.Second):
		t.Fatal("fixture did not exit")
	}
	if p.ExitCode != 12 {
		t.Fatal(p.ExitCode)
	}
}

func TestPseudoconsoleRestrictedToken(t *testing.T) {
	var original windows.Token
	if e := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_DUPLICATE|windows.TOKEN_QUERY|windows.TOKEN_ASSIGN_PRIMARY, &original); e != nil {
		t.Fatal(e)
	}
	defer original.Close()
	token, e := identity.Restrict(original)
	if e != nil {
		t.Fatal(e)
	}
	defer token.Close()
	p, e := startWithToken(Spec{Pseudoconsole: true, Executable: fixture, Arguments: []string{"winconsole"}, Directory: t.TempDir(), Environment: []string{"SystemRoot=" + os.Getenv("SystemRoot")}}, token)
	if e != nil {
		t.Fatal(e)
	}
	defer p.Kill()
	defer p.Stdout.Close()
	done := make(chan []byte, 1)
	go func() { b, _ := io.ReadAll(p.Stdout); done <- b }()
	if _, e = p.Stdin.Write([]byte("restricted\rstop\r")); e != nil {
		t.Fatal(e)
	}
	select {
	case <-p.Done:
	case <-time.After(5 * time.Second):
		t.Fatal("restricted console did not exit")
	}
	select {
	case b := <-done:
		if !bytes.Contains(b, []byte("WINCON_ECHO:restricted")) || p.ExitCode != 0 {
			t.Fatal(string(b), p.ExitCode)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("console output not drained")
	}
}
func TestPseudoconsoleJobDescendants(t *testing.T) {
	p, e := Start(Spec{Pseudoconsole: true, Executable: fixture, Arguments: []string{"spawn"}, Directory: t.TempDir(), Environment: []string{"SystemRoot=" + os.Getenv("SystemRoot")}})
	if e != nil {
		t.Fatal(e)
	}
	defer p.Kill()
	defer p.Stdout.Close()
	var handles []windows.Handle
	deadline := time.Now().Add(3 * time.Second)
	for {
		p.implementation.state.Lock()
		handles, e = jobProcessHandles(p.implementation.state.job)
		p.implementation.state.Unlock()
		if e != nil {
			t.Fatal(e)
		}
		if len(handles) >= 2 {
			break
		}
		for _, h := range handles {
			windows.CloseHandle(h)
		}
		if time.Now().After(deadline) {
			t.Fatal("descendant not observed")
		}
		time.Sleep(20 * time.Millisecond)
	}
	defer func() {
		for _, h := range handles {
			windows.CloseHandle(h)
		}
	}()
	if e = p.Kill(); e != nil {
		t.Fatal(e)
	}
	select {
	case <-p.Done:
	case <-time.After(5 * time.Second):
		t.Fatal("job did not finish")
	}
	for _, h := range handles {
		state, e := windows.WaitForSingleObject(h, 3000)
		if e != nil || state != windows.WAIT_OBJECT_0 {
			t.Fatal("descendant survived", state, e)
		}
	}
}
func TestPseudoconsoleStartupFailureCleanup(t *testing.T) {
	for i := 0; i < 4; i++ {
		p, e := Start(Spec{Pseudoconsole: true, Executable: fixture, Arguments: []string{"bad\x00arg"}, Directory: t.TempDir()})
		if e == nil || p != nil {
			t.Fatal("bad arguments accepted")
		}
	}
}
