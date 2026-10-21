package identity

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"golang.org/x/sys/windows"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"wings-windows/internal/testutil"
)

func TestCredentialLifecycleWithoutAccountLogon(t *testing.T) {
	var random [16]byte
	rand.Read(random[:])
	id := fmt.Sprintf("%x-%x-%x-%x-%x", random[:4], random[4:6], random[6:8], random[8:10], random[10:])
	t.Cleanup(func() {
		if _, err := RemoveCredential(id); err != nil {
			t.Error("temporary credential cleanup failed", err)
		}
	})
	status, err := CredentialStatus(id)
	if err != nil || status.Present {
		t.Fatal("initial missing status", status, err)
	}
	if removed, err := RemoveCredential(id); err != nil || removed {
		t.Fatal("missing removal should be idempotent", removed, err)
	}
	path, err := filepath.Abs("../../scripts/Store-GameCredential.ps1")
	if err != nil {
		t.Fatal(err)
	}
	store := func(user, password string, replace bool) error {
		command := "& '" + strings.ReplaceAll(path, "'", "''") + "' -ServerUUID '" + id + "' -Credential ([PSCredential]::new('" + user + "', (ConvertTo-SecureString '" + password + "' -AsPlainText -Force)))"
		if replace {
			command += " -Replace"
		}
		// All values here are fixed fake fixtures, never real account secrets.
		return testutil.PowerShell(context.Background(), "-NoProfile", "-NonInteractive", "-Command", command).Run()
	}
	const user = `.\WingsTestNeverLogOn`
	const first = "TEST-only-secret-first"
	const second = "TEST-only-secret-second"
	if err := store(user, first, true); err == nil {
		t.Fatal("rotation created a missing credential")
	}
	if err := store(user, first, false); err != nil {
		t.Fatal("create fixture", err)
	}
	status, err = CredentialStatus(strings.ToUpper(id))
	if err != nil || !status.Present || status.Username != user || status.Persistence != "local_machine" {
		t.Fatal("metadata", status, err)
	}
	if _, err := time.Parse(time.RFC3339Nano, status.LastWritten); err != nil {
		t.Fatal("invalid write timestamp", err)
	}
	encoded, err := json.Marshal(status)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), first) || strings.Contains(string(encoded), "password") {
		t.Fatal("metadata exposed secret fields")
	}
	verify := func(want string) {
		t.Helper()
		_, secret, err := readCredential(status.Target)
		if err != nil {
			t.Fatal(err)
		}
		defer clear(secret)
		if windows.UTF16ToString(secret) != want {
			t.Fatal("stored fixture secret changed unexpectedly")
		}
	}
	if err := store(user, second, false); err == nil {
		t.Fatal("accidental overwrite succeeded")
	}
	verify(first)
	if err := store(`.\DifferentFakeAccount`, second, true); err == nil {
		t.Fatal("rotation changed identity")
	}
	verify(first)
	if err := store(user, second, true); err != nil {
		t.Fatal("explicit rotation", err)
	}
	verify(second)
	if removed, err := RemoveCredential(id); err != nil || !removed {
		t.Fatal("remove fixture", removed, err)
	}
	if removed, err := RemoveCredential(id); err != nil || removed {
		t.Fatal("repeat removal", removed, err)
	}
	status, err = CredentialStatus(id)
	if err != nil || status.Present {
		t.Fatal("removed entry still present", status, err)
	}
}

func TestCredentialOperationsRejectUnscopedTargets(t *testing.T) {
	for _, bad := range []string{"", "*", "WingsWindows/*", "other-app/secret", "11111111-1111-4111-8111-111111111111/extra"} {
		if _, err := CredentialStatus(bad); err == nil {
			t.Fatal("status accepted unscoped target", bad)
		}
		if _, err := RemoveCredential(bad); err == nil {
			t.Fatal("remove accepted unscoped target", bad)
		}
	}
}
