package config

import (
	"testing"
	"wings-windows/internal/identity"
	"wings-windows/internal/networkpolicy"
)

func TestNetworkPolicyRequiresDedicatedIdentityAndKnownMode(t *testing.T) {
	c := valid(t)
	p := c.Profiles[id]
	p.NetworkPolicy = networkpolicy.Policy{Mode: "block_connections"}
	c.Profiles[id] = p
	if c.Validate() == nil {
		t.Fatal("shared-account network blocking accepted")
	}
	p.RunAs = &identity.Reference{SID: "S-1-5-21-1-2-3-1001", CredentialTarget: "WingsWindows/" + id}
	c.Profiles[id] = p
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	p.NetworkPolicy.Mode = "allocation_allowlist"
	c.Profiles[id] = p
	if c.Validate() == nil {
		t.Fatal("unimplemented allocation policy accepted")
	}
}
