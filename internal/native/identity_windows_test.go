package native

import (
	"context"
	"errors"
	"golang.org/x/sys/windows"
	"os"
	"strings"
	"testing"
	"time"
	"unsafe"
	"wings-windows/internal/identity"
)

func TestRestrictedTokenLaunchPreservesConsoleAndJob(t *testing.T) {
	var original windows.Token
	if err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_DUPLICATE|windows.TOKEN_QUERY|windows.TOKEN_ASSIGN_PRIMARY, &original); err != nil {
		t.Fatal(err)
	}
	defer original.Close()
	token, err := identity.Restrict(original)
	if err != nil {
		t.Fatal(err)
	}
	defer token.Close()
	p, err := startWithToken(Spec{Executable: fixture, Directory: t.TempDir(), Environment: []string{"SystemRoot=" + os.Getenv("SystemRoot")}}, token)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { p.Kill(); <-p.Done; p.Stdout.Close() })
	out := lines(p)
	if got := line(t, out); got != "SERVER_READY" {
		t.Fatal(got)
	}
	if _, err := p.Stdin.Write([]byte("identity console test\r\n")); err != nil {
		t.Fatal(err)
	}
	if got := line(t, out); got != "ECHO:identity console test" {
		t.Fatal(got)
	}
	p.implementation.state.Lock()
	var child windows.Token
	err = windows.OpenProcessToken(p.implementation.state.process, windows.TOKEN_QUERY, &child)
	p.implementation.state.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	defer child.Close()
	var size uint32
	windows.GetTokenInformation(child, windows.TokenPrivileges, nil, 0, &size)
	buffer := make([]byte, size)
	if err := windows.GetTokenInformation(child, windows.TokenPrivileges, &buffer[0], size, &size); err != nil {
		t.Fatal(err)
	}
	privileges := (*windows.Tokenprivileges)(unsafe.Pointer(&buffer[0]))
	name, _ := windows.UTF16PtrFromString("SeChangeNotifyPrivilege")
	var allowed windows.LUID
	if err := windows.LookupPrivilegeValue(nil, name, &allowed); err != nil {
		t.Fatal(err)
	}
	for _, privilege := range privileges.AllPrivileges() {
		if privilege.Luid != allowed {
			t.Fatal("child retained unexpected privilege")
		}
	}
	if err := p.Kill(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-p.Done:
	case <-time.After(3 * time.Second):
		t.Fatal("restricted process did not terminate with job")
	}
}

func TestCancelledNativeLaunchDoesNotStartProcess(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if p, err := Start(Spec{Context: ctx, Executable: fixture, Directory: t.TempDir()}); p != nil || !errors.Is(err, context.Canceled) {
		if p != nil {
			p.Kill()
			p.Stdout.Close()
		}
		t.Fatal("cancelled launch was accepted", err)
	}
}

func TestInvalidIdentityNeverFallsBackToDaemon(t *testing.T) {
	_, err := Start(Spec{Executable: fixture, Directory: t.TempDir(), RunAs: &identity.Reference{SID: "S-1-5-18", CredentialTarget: "invalid"}})
	if err == nil || !strings.Contains(err.Error(), "run_as") {
		t.Fatal("invalid identity launch did not fail closed", err)
	}
}
