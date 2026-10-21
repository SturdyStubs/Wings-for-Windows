package node

import (
	"context"
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"wings-windows/internal/config"
	"wings-windows/internal/identity"
)

func TestOfflineIdentityReportNeverCreatesRootsOrContactsPanel(t *testing.T) {
	var random [16]byte
	rand.Read(random[:])
	id := fmt.Sprintf("%x-%x-%x-%x-%x", random[:4], random[4:6], random[6:8], random[8:10], random[10:])
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(t.TempDir(), "must-not-exist")
	c := config.Config{PanelURL: "https://must-not-be-contacted.invalid", DataDirectory: root, Profiles: map[string]config.Profile{id: {Executable: exe, ReadyPattern: "READY", RunAs: &identity.Reference{SID: "S-1-5-21-1-2-3-1001", CredentialTarget: "WingsWindows/" + id}}}}
	report := CheckIdentity(context.Background(), c, strings.ToUpper(id))
	if report.Passed || report.GameLaunched || report.PanelTested || report.NetworkPolicyTested {
		t.Fatal("report overclaimed verification", report)
	}
	if report.ServerUUID != id || report.Checks[4].Name != "credential_presence" || report.Checks[4].Status != "failed" {
		t.Fatal(report)
	}
	if len(report.Checks) != 10 || report.Checks[8].Name != "launch_access" || report.Checks[8].Status != "not_run" {
		t.Fatal("missing credential must leave launch readiness unverified", report)
	}
	for _, check := range report.Checks[5:] {
		if check.Status != "not_run" {
			t.Fatal("missing credential reached account logon", report)
		}
	}
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Fatal("diagnostic created data roots", err)
	}
}

func TestIdentityReportRejectsSharedUnknownAndMissingExecutables(t *testing.T) {
	c := config.Config{Profiles: map[string]config.Profile{testID: {}}}
	for _, id := range []string{testID, "22222222-2222-4222-8222-222222222222", "not-a-uuid"} {
		report := CheckIdentity(context.Background(), c, id)
		if report.Passed || report.Checks[0].Status != "failed" {
			t.Fatal(report)
		}
		for _, check := range report.Checks[1:] {
			if check.Status != "not_run" {
				t.Fatal("shared/unknown profile reached later checks", report)
			}
		}
	}
	c.Profiles[testID] = config.Profile{Executable: filepath.Join(t.TempDir(), "missing.exe"), RunAs: &identity.Reference{SID: "S-1-5-21-1-2-3-1001", CredentialTarget: "WingsWindows/" + testID}}
	report := CheckIdentity(context.Background(), c, testID)
	if report.Passed || report.Checks[1].Status != "failed" {
		t.Fatal(report)
	}
	for _, check := range report.Checks[2:] {
		if check.Status != "not_run" {
			t.Fatal("missing executable reached credential checks", report)
		}
	}
}
