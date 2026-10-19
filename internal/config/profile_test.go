package config

import (
	"testing"
	"wings-windows/internal/identity"
)

func TestProfileRequirementsValidation(t *testing.T) {
	for _, test := range []struct {
		name string
		p    Profile
	}{
		{"negative version", Profile{SchemaVersion: -1}},
		{"future version", Profile{SchemaVersion: 2}},
		{"unsupported architecture", Profile{Requirements: Requirements{Architecture: "arm64"}}},
		{"negative build", Profile{Requirements: Requirements{MinimumWindowsBuild: -1}}},
		{"too many dependencies", Profile{Requirements: Requirements{RequiredFiles: make([]string, 65)}}},
		{"relative file", Profile{Requirements: Requirements{RequiredFiles: []string{"runtime.dll"}}}},
		{"network file", Profile{Requirements: Requirements{RequiredFiles: []string{`\\server\share\runtime.dll`}}}},
		{"device file", Profile{Requirements: Requirements{RequiredFiles: []string{`\\?\C:\runtime.dll`}}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := test.p.NormalizeRequirements(); err == nil {
				t.Fatal("invalid requirements accepted")
			}
		})
	}
	c := valid(t)
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	if c.Profiles[id].SchemaVersion != ProfileSchemaVersion {
		t.Fatal("legacy profile was not normalized")
	}
	p := c.Profiles[id]
	p.SchemaVersion = 99
	c.Profiles[id] = p
	if c.Validate() == nil {
		t.Fatal("configuration accepted a future profile version")
	}
}

func TestRunAsBindingAndUniqueAccounts(t *testing.T) {
	c := valid(t)
	p := c.Profiles[id]
	p.RunAs = &identity.Reference{SID: "S-1-5-21-1-2-3-1001", CredentialTarget: "WingsWindows/" + id}
	c.Profiles[id] = p
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	const second = "22222222-2222-4222-8222-222222222222"
	c.Profiles[second] = p
	if c.Validate() == nil {
		t.Fatal("cross-server credential target accepted")
	}
	p.RunAs = &identity.Reference{SID: p.RunAs.SID, CredentialTarget: "WingsWindows/" + second}
	c.Profiles[second] = p
	if c.Validate() == nil {
		t.Fatal("shared game identity accepted")
	}
	p.RunAs.SID = "S-1-5-21-1-2-3-1002"
	c.Profiles[second] = p
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
}
