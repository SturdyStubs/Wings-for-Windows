package node

import (
	"os"
	"path/filepath"
	"testing"
	"wings-windows/internal/config"
	"wings-windows/internal/identity"
)

func TestIdentitySpecProtectsSecretsDaemonAndOtherServers(t *testing.T) {
	m, s, _ := setup(t)
	const other = "22222222-2222-4222-8222-222222222222"
	s.Profile.RunAs = &identity.Reference{SID: "S-1-5-21-1-2-3-1001", CredentialTarget: "WingsWindows/" + testID}
	secret := filepath.Join(t.TempDir(), "node-config.json")
	key := filepath.Join(t.TempDir(), "node.key")
	s.panel.Config.SourcePath = secret
	s.panel.Config.TLS.Key = key
	s.panel.Config.Profiles[other] = config.Profile{}
	spec, err := s.spec()
	if err != nil {
		t.Fatal(err)
	}
	if spec.RunAs != s.Profile.RunAs {
		t.Fatal("execution identity was lost")
	}
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]uint32{secret: identity.ReadWriteAccess, key: identity.ReadWriteAccess, binary: identity.WriteAccess, filepath.Dir(binary): identity.ReplaceChildAccess, filepath.Join(m.Config.DataDirectory, other): identity.ReadWriteAccess}
	for _, rule := range spec.AccessRules {
		if rule.Path == filepath.Join(m.Config.DataDirectory, other) && !rule.Recursive {
			t.Fatal("other server root was not recursive")
		}
		if required, ok := want[rule.Path]; ok && rule.Forbidden == required {
			delete(want, rule.Path)
		}
		if rule.Path == s.Root {
			t.Fatal("own root treated as another server")
		}
	}
	if len(want) != 0 {
		t.Fatal("missing protected paths", want)
	}
}
