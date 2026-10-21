package identity

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestDiagnosticStopsBeforeLogonWhenCredentialIsMissing(t *testing.T) {
	var random [16]byte
	rand.Read(random[:])
	id := fmt.Sprintf("%x-%x-%x-%x-%x", random[:4], random[4:6], random[6:8], random[8:10], random[10:])
	checks := AuditIdentity(context.Background(), Reference{SID: "S-1-5-21-1-2-3-1001", CredentialTarget: "WingsWindows/" + id}, nil, LaunchAccess{})
	if checks[0].Status != "passed" || checks[1].Status != "failed" || !strings.Contains(checks[1].Error, "missing") {
		t.Fatal(checks)
	}
	for _, check := range checks[2:] {
		if check.Status != "not_run" {
			t.Fatal("later stage ran after missing credential", checks)
		}
	}
	b, err := json.Marshal(checks)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "password") {
		t.Fatal("diagnostic emitted a password field")
	}
}

func TestDiagnosticReferenceAndCancellationStopAllLaterChecks(t *testing.T) {
	checks := AuditIdentity(context.Background(), Reference{}, nil, LaunchAccess{})
	if checks[0].Status != "failed" {
		t.Fatal("invalid reference was accepted", checks)
	}
	for _, check := range checks[1:] {
		if check.Status != "not_run" {
			t.Fatal("invalid reference reached later stage")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	checks = AuditIdentity(ctx, Reference{}, nil, LaunchAccess{})
	if checks[0].Status != "failed" || !strings.Contains(checks[0].Error, "canceled") {
		t.Fatal(checks)
	}
}
