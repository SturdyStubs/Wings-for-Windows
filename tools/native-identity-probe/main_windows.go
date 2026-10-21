package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"golang.org/x/sys/windows"
	"os"
	"path/filepath"
	"strings"
	"time"
	"wings-windows/internal/identity"
	"wings-windows/internal/native"
)

type Plan struct{ SID, ServerID, Root, Peer, Secret string }
type Result struct {
	Passed bool              `json:"passed"`
	Checks map[string]string `json:"checks"`
	Error  string            `json:"error,omitempty"`
	Child  map[string]any    `json:"child,omitempty"`
}

func fixture() {
	root, peer, secret := os.Args[2], os.Args[3], os.Args[4]
	out := map[string]any{}
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err == nil {
		out["sid"] = user.User.Sid.String()
	}
	out["own_write"] = os.WriteFile(filepath.Join(root, "own-write.txt"), []byte("harmless fixture"), 0600) == nil
	out["temp_write"] = os.WriteFile(filepath.Join(root, ".tmp", "temp-write.txt"), []byte("harmless fixture"), 0600) == nil
	_, err = os.ReadFile(peer)
	out["peer_read_denied"] = os.IsPermission(err)
	_, err = os.ReadFile(secret)
	out["secret_read_denied"] = os.IsPermission(err)
	h, err := windows.OpenProcess(windows.PROCESS_VM_READ, false, uint32(os.Getppid()))
	out["daemon_memory_denied"] = err == windows.ERROR_ACCESS_DENIED
	if err == nil {
		windows.CloseHandle(h)
	}
	b, _ := json.Marshal(out)
	fmt.Println("PROBE:" + string(b))
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		if scanner.Text() == "stop" {
			return
		}
		fmt.Println("ECHO:" + scanner.Text())
	}
}

func run(path string) (r Result) {
	r.Checks = map[string]string{}
	stage := "plan"
	fail := func(err error) Result { r.Checks[stage] = "failed"; r.Error = err.Error(); return r }
	data, err := os.ReadFile(path)
	if err != nil {
		return fail(err)
	}
	var plan Plan
	if err = json.Unmarshal(data, &plan); err != nil {
		return fail(err)
	}
	exe, _ := os.Executable()
	temp := filepath.Join(plan.Root, ".tmp")
	ref := identity.Reference{SID: plan.SID, CredentialTarget: "WingsWindows/" + plan.ServerID}
	rules := []identity.PathRule{{Path: exe, Forbidden: identity.WriteAccess}, {Path: filepath.Dir(exe), Forbidden: identity.ReplaceChildAccess}, {Path: plan.Secret, Forbidden: identity.ReadWriteAccess}, {Path: filepath.Dir(plan.Secret), Forbidden: identity.ReplaceChildAccess}, {Path: filepath.Dir(plan.Peer), Forbidden: identity.ReadWriteAccess, Recursive: true}}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	stage = "batch_logon_and_restricted_token"
	token, err := identity.Open(ref)
	if err != nil {
		return fail(err)
	}
	defer token.Close()
	r.Checks[stage] = "passed"
	stage = "daemon_process_token_guard"
	if err = identity.CheckDaemonAccess(token); err != nil {
		return fail(err)
	}
	r.Checks[stage] = "passed"
	stage = "protected_filesystem_guard"
	if err = identity.CheckAccessContext(ctx, token, rules); err != nil {
		return fail(err)
	}
	r.Checks[stage] = "passed"
	stage = "required_launch_permissions"
	if err = identity.CheckLaunchAccessContext(ctx, token, identity.LaunchAccess{Executable: exe, Directory: plan.Root, TempDirectory: temp}); err != nil {
		return fail(err)
	}
	r.Checks[stage] = "passed"
	stage = "native_separate_account_launch"
	p, err := native.Start(native.Spec{Context: ctx, Executable: exe, Arguments: []string{"--fixture", plan.Root, plan.Peer, plan.Secret}, Directory: plan.Root, TempDirectory: temp, RunAs: &ref, AccessRules: rules, Environment: []string{"SystemRoot=" + os.Getenv("SystemRoot"), "WINDIR=" + os.Getenv("SystemRoot"), "TEMP=" + temp, "TMP=" + temp}, MemoryMB: 128, CPUPercent: 100})
	if err != nil {
		return fail(err)
	}
	defer func() {
		p.Kill()
		select {
		case <-p.Done:
		case <-time.After(5 * time.Second):
		}
		p.Stdout.Close()
	}()
	r.Checks[stage] = "passed"
	lines := make(chan string, 8)
	go func() {
		defer close(lines)
		s := bufio.NewScanner(p.Stdout)
		for s.Scan() {
			lines <- s.Text()
		}
	}()
	stage = "actual_child_identity_and_access"
	select {
	case line := <-lines:
		if !strings.HasPrefix(line, "PROBE:") {
			return fail(fmt.Errorf("unexpected fixture output: %s", line))
		}
		if err = json.Unmarshal([]byte(strings.TrimPrefix(line, "PROBE:")), &r.Child); err != nil {
			return fail(err)
		}
	case <-time.After(5 * time.Second):
		return fail(fmt.Errorf("fixture readiness timed out"))
	}
	if r.Child["sid"] != plan.SID {
		return fail(fmt.Errorf("child SID mismatch"))
	}
	for _, name := range []string{"own_write", "temp_write", "peer_read_denied", "secret_read_denied", "daemon_memory_denied"} {
		if r.Child[name] != true {
			return fail(fmt.Errorf("child check failed: %s", name))
		}
	}
	r.Checks[stage] = "passed"
	stage = "stdio_console"
	if err = p.Command("identity console probe"); err != nil {
		return fail(err)
	}
	select {
	case line := <-lines:
		if line != "ECHO:identity console probe" {
			return fail(fmt.Errorf("console echo mismatch"))
		}
	case <-time.After(5 * time.Second):
		return fail(fmt.Errorf("console timeout"))
	}
	r.Checks[stage] = "passed"
	stage = "graceful_job_exit"
	if err = p.Command("stop"); err != nil {
		return fail(err)
	}
	select {
	case <-p.Done:
		if p.ExitCode != 0 {
			return fail(fmt.Errorf("exit %d", p.ExitCode))
		}
	case <-time.After(5 * time.Second):
		return fail(fmt.Errorf("stop timeout"))
	}
	r.Checks[stage] = "passed"
	r.Passed = true
	return r
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--fixture" {
		fixture()
		return
	}
	if len(os.Args) != 2 {
		os.Exit(2)
	}
	r := run(os.Args[1])
	json.NewEncoder(os.Stdout).Encode(r)
	if !r.Passed {
		os.Exit(1)
	}
}
