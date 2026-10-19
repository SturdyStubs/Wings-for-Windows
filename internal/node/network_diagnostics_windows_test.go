package node

import (
	"context"
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"wings-windows/internal/config"
	"wings-windows/internal/identity"
	"wings-windows/internal/networkpolicy"
)

func TestNetworkDiagnosticFailsBeforeWFPOnMissingCredential(t *testing.T) {
	var b [16]byte
	rand.Read(b[:])
	id := fmt.Sprintf("%x-%x-%x-%x-%x", b[:4], b[4:6], b[6:8], b[8:10], b[10:])
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(t.TempDir(), "must-not-be-created")
	c := config.Config{DataDirectory: root, Profiles: map[string]config.Profile{id: {Executable: exe, NetworkPolicy: networkpolicy.Policy{Mode: "block_connections"}, RunAs: &identity.Reference{SID: "S-1-5-21-1-2-3-1001", CredentialTarget: "WingsWindows/" + id}}}}
	result := CheckNetworkPolicy(context.Background(), c, id)
	if result.Passed || result.Policy != nil || result.Identity == nil || result.Identity.Passed {
		t.Fatal("missing credential reached WFP", result)
	}
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Fatal("network diagnostic created a root", err)
	}
}

func TestNetworkDiagnosticRequiresExplicitConfiguredPolicy(t *testing.T) {
	c := config.Config{Profiles: map[string]config.Profile{testID: {}}}
	for _, id := range []string{testID, "bad"} {
		result := CheckNetworkPolicy(context.Background(), c, id)
		if result.Passed || result.Identity != nil || result.Policy != nil || result.Error == "" {
			t.Fatal("unselected policy reached preflight", result)
		}
	}
}
