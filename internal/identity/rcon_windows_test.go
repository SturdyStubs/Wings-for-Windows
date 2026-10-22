package identity

import (
	"context"
	"crypto/rand"
	"fmt"
	"golang.org/x/sys/windows"
	"path/filepath"
	"strings"
	"testing"
	"unsafe"
	"wings-windows/internal/testutil"
)

func TestRCONCredentialFormatRefusal(t *testing.T) {
	for _, mode := range []string{"marker", "nul", "surrogate"} {
		t.Run(mode, func(t *testing.T) {
			var b [16]byte
			rand.Read(b[:])
			id := fmt.Sprintf("%x-%x-%x-%x-%x", b[:4], b[4:6], b[6:8], b[8:10], b[10:])
			target, _ := RCONCredentialTarget(id)
			targetPtr, _ := windows.UTF16PtrFromString(target)
			user := ".\\WingsRCONSecret-v1"
			units := []uint16{'x'}
			switch mode {
			case "marker":
				user = ".\\WrongMarker"
			case "nul":
				units = []uint16{'a', 0, 'b'}
			case "surrogate":
				units = []uint16{0xd800}
			}
			userPtr, _ := windows.UTF16PtrFromString(user)
			c := credential{Type: 1, TargetName: targetPtr, UserName: userPtr, Persist: 2, Blob: (*byte)(unsafe.Pointer(&units[0])), BlobSize: uint32(len(units) * 2)}
			if ok, _, err := advapi.NewProc("CredWriteW").Call(uintptr(unsafe.Pointer(&c)), 0); ok == 0 {
				t.Fatal(err)
			}
			t.Cleanup(func() { RemoveRCONCredential(id) })
			if _, err := RCONPassword(id); err == nil {
				t.Fatal("malformed secret accepted")
			}
		})
	}
}

func TestRCONCredentialLifecycleAndNamespace(t *testing.T) {
	var b [16]byte
	rand.Read(b[:])
	id := fmt.Sprintf("%x-%x-%x-%x-%x", b[:4], b[4:6], b[6:8], b[8:10], b[10:])
	target, _ := RCONCredentialTarget(id)
	status, err := RCONCredentialStatus(id)
	if err != nil || status.Present {
		t.Fatal(status, err)
	}
	if _, err = RCONPassword(id); err == nil {
		t.Fatal("missing password succeeded")
	}
	script, _ := filepath.Abs("../../scripts/Store-RCONCredential.ps1")
	store := func(value string, replace bool) error {
		text := "& '" + strings.ReplaceAll(script, "'", "''") + "' -ServerUUID '" + id + "' -Password (ConvertTo-SecureString '" + value + "' -AsPlainText -Force)"
		if replace {
			text += " -Replace"
		}
		return testutil.PowerShell(context.Background(), "-NoProfile", "-NonInteractive", "-Command", text).Run()
	}
	if store("first", true) == nil {
		t.Fatal("replace created missing credential")
	}
	if err = store("RCON-only-雪-😀", false); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := RemoveRCONCredential(id); err != nil {
			t.Error(err)
		}
	})
	if store("accidental-overwrite", false) == nil {
		t.Fatal("overwrite accepted")
	}
	value, err := RCONPassword(id)
	if err != nil || value != "RCON-only-雪-😀" {
		t.Fatal("secret round trip failed", err)
	}
	status, err = RCONCredentialStatus(strings.ToUpper(id))
	if err != nil || !status.Present || status.Target != target || status.Username != "" {
		t.Fatal(status, err)
	}
	game, err := CredentialStatus(id)
	if err != nil || game.Present {
		t.Fatal("RCON leaked into game namespace", err)
	}
	if removed, err := RemoveCredential(id); err != nil || removed {
		t.Fatal("game removal affected RCON", err)
	}
	if err = store("deliberate-rotation", true); err != nil {
		t.Fatal(err)
	}
	if value, err = RCONPassword(id); err != nil || value != "deliberate-rotation" {
		t.Fatal("rotation failed", err)
	}
	if store(strings.Repeat("x", 1281), true) == nil {
		t.Fatal("oversized UTF-16 credential accepted")
	}
	if store("bad\nvalue", true) == nil {
		t.Fatal("newline secret accepted")
	}
	if removed, err := RemoveRCONCredential(id); err != nil || !removed {
		t.Fatal(removed, err)
	}
	if removed, err := RemoveRCONCredential(id); err != nil || removed {
		t.Fatal("missing remove not idempotent", err)
	}
	if _, err = RCONPassword(id); err == nil {
		t.Fatal("removed password still available")
	}
	for _, bad := range []string{"", "../secret", id + "\n"} {
		if _, err = RCONCredentialTarget(bad); err == nil {
			t.Fatal("invalid UUID accepted")
		}
	}
}
