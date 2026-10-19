package native

import (
	"golang.org/x/sys/windows"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type observingPolicy struct {
	closed atomic.Int32
	child  atomic.Uintptr
	alive  atomic.Bool
}

func (p *observingPolicy) Close() error {
	p.closed.Add(1)
	state, err := windows.WaitForSingleObject(windows.Handle(p.child.Load()), 0)
	if err != nil || state != windows.WAIT_OBJECT_0 {
		p.alive.Store(true)
	}
	return nil
}

func TestNetworkLeaseReleasedOnlyAfterJobDescendantsExit(t *testing.T) {
	policy := &observingPolicy{}
	p, err := startWithTokenAndPolicy(Spec{Executable: fixture, Arguments: []string{"spawn"}, Directory: t.TempDir(), Environment: []string{"SystemRoot=" + os.Getenv("SystemRoot")}}, 0, policy)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { p.Kill(); <-p.Done; p.Stdout.Close() })
	out := lines(p)
	line(t, out)
	child := line(t, out)
	pid, err := strconv.Atoi(strings.TrimPrefix(child, "CHILD:"))
	if err != nil {
		t.Fatal(child, err)
	}
	childHandle, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		t.Fatal(err)
	}
	policy.child.Store(uintptr(childHandle))
	defer windows.CloseHandle(childHandle)
	if _, err := p.Stdin.Write([]byte("stop\r\n")); err != nil {
		t.Fatal(err)
	}
	select {
	case <-p.Done:
	case <-time.After(7 * time.Second):
		t.Fatal("job did not exit")
	}
	if p.CleanupError != nil || policy.closed.Load() != 1 || policy.alive.Load() {
		t.Fatal("network policy released before descendants stopped", p.CleanupError)
	}
}
