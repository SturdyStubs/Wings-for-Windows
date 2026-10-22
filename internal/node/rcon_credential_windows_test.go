package node

import (
	"context"
	"crypto/rand"
	"fmt"
	"net"
	"path/filepath"
	"strings"
	"testing"
	"wings-windows/internal/config"
	"wings-windows/internal/identity"
	"wings-windows/internal/testutil"
)

func TestRCONCredentialLookupRotationAndRemoval(t *testing.T) {
	var b [16]byte
	rand.Read(b[:])
	id := fmt.Sprintf("%x-%x-%x-%x-%x", b[:4], b[4:6], b[6:8], b[8:10], b[10:])
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := l.Addr().String()
	l.Close()
	_, s, _ := setupConfiguredID(t, id, nil)
	target, _ := identity.RCONCredentialTarget(id)
	s.Profile.Arguments = []string{"rcon", address}
	s.Profile.SourceRCON = &config.SourceRCON{Address: address, PasswordCredentialTarget: target, TimeoutSeconds: 2}
	t.Setenv("WINGS_RCON_FIXTURE", "fixture-rcon-only")
	if err = s.start(); err != nil {
		t.Fatal(err)
	}
	await(t, s, "running")
	if err = s.Command("missing-secret"); err == nil {
		t.Fatal("missing vault entry fell back to environment")
	}
	script, _ := filepath.Abs("../../scripts/Store-RCONCredential.ps1")
	store := func(value string, replace bool) {
		t.Helper()
		text := "& '" + strings.ReplaceAll(script, "'", "''") + "' -ServerUUID '" + id + "' -Password (ConvertTo-SecureString '" + value + "' -AsPlainText -Force)"
		if replace {
			text += " -Replace"
		}
		if err := testutil.PowerShell(context.Background(), "-NoProfile", "-NonInteractive", "-Command", text).Run(); err != nil {
			t.Fatal(err)
		}
	}
	store("wrong-fixture", false)
	t.Cleanup(func() { identity.RemoveRCONCredential(id) })
	if err = s.Command("wrong-secret"); err == nil {
		t.Fatal("wrong vault secret authenticated")
	}
	store("fixture-rcon-only", true)
	if err = s.Command("vault-secret"); err != nil {
		t.Fatal(err)
	}
	if _, err = identity.RemoveRCONCredential(id); err != nil {
		t.Fatal(err)
	}
	if err = s.Command("removed-secret"); err == nil {
		t.Fatal("removed vault entry fell back to environment")
	}
	if err = s.Power("kill"); err != nil {
		t.Fatal(err)
	}
	await(t, s, "offline")
}
