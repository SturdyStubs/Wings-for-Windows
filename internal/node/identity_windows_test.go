package node

import (
	"os"
	"path/filepath"
	"strings"
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
	logDirectory := filepath.Join(t.TempDir(), "logs")
	s.panel.Config.ConsoleLogs = &config.ConsoleLogs{Directory: logDirectory}
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
	want := map[string]uint32{logDirectory: identity.ReadWriteAccess, secret: identity.ReadWriteAccess, key: identity.ReadWriteAccess, binary: identity.WriteAccess, filepath.Dir(binary): identity.ReplaceChildAccess, filepath.Join(m.Config.DataDirectory, other): identity.ReadWriteAccess}
	for _, rule := range spec.AccessRules {
		if rule.Path == logDirectory && !rule.Recursive {
			t.Fatal("console logs were not recursively protected")
		}
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

func TestIdentityRulesProtectDeclaredCodeAndMergeSharedFiles(t *testing.T) {
	_, s, _ := setup(t)
	root := t.TempDir()
	s.Profile.RunAs = &identity.Reference{SID: "S-1-5-21-1-2-3-1001", CredentialTarget: "WingsWindows/" + testID}
	s.Profile.Executable = filepath.Join(root, "game.exe")
	shared := filepath.Join(root, "runtime.dll")
	s.Profile.Requirements.RequiredFiles = []string{shared, shared}
	const other = "22222222-2222-4222-8222-222222222222"
	otherExe := filepath.Join(t.TempDir(), "other.exe")
	s.panel.Config.Profiles[other] = config.Profile{Executable: otherExe, Requirements: config.Requirements{RequiredFiles: []string{shared}}}
	rules, err := s.identityAccessRules()
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	for _, rule := range rules {
		key := strings.ToLower(rule.Path)
		counts[key]++
		if key == strings.ToLower(shared) || rule.Path == otherExe || rule.Path == s.Profile.Executable {
			if rule.Forbidden&identity.WriteAccess != identity.WriteAccess {
				t.Fatal("code not protected against writes", rule)
			}
		}
	}
	for _, path := range []string{shared, s.Profile.Executable, otherExe, filepath.Dir(shared), filepath.Dir(otherExe)} {
		if counts[strings.ToLower(path)] != 1 {
			t.Fatal("missing or duplicated code rule", path, counts)
		}
	}
	// A secret sharing a dependency path must retain the stricter read ban.
	s.panel.Config.TLS.Key = shared
	rules, err = s.identityAccessRules()
	if err != nil {
		t.Fatal(err)
	}
	for _, rule := range rules {
		if strings.EqualFold(rule.Path, shared) && rule.Forbidden&identity.ReadWriteAccess != identity.ReadWriteAccess {
			t.Fatal("merged rule weakened secret policy")
		}
	}
}

func TestIdentityRuleMergeRetainsCaseVariants(t *testing.T) {
	_, s, _ := setup(t)
	s.Profile.RunAs = &identity.Reference{SID: "S-1-5-21-1-2-3-1001", CredentialTarget: "WingsWindows/" + testID}
	file := filepath.Join(t.TempDir(), "runtime.dll")
	s.Profile.Requirements.RequiredFiles = []string{file, strings.ToUpper(file)}
	rules, err := s.identityAccessRules()
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, rule := range rules {
		seen[rule.Path] = true
	}
	if !seen[file] || !seen[strings.ToUpper(file)] {
		t.Fatal("case-distinct declaration was dropped")
	}
}

func TestIdentitySpecCarriesLaunchFilesAndTemporaryDirectory(t *testing.T) {
	_, s, _ := setup(t)
	s.Profile.RunAs = &identity.Reference{SID: "S-1-5-21-1-2-3-1001", CredentialTarget: "WingsWindows/" + testID}
	dependency := filepath.Join(t.TempDir(), "runtime.dll")
	s.Profile.Requirements.RequiredFiles = []string{dependency}
	spec, err := s.spec()
	if err != nil {
		t.Fatal(err)
	}
	if spec.TempDirectory != filepath.Join(s.Root, ".tmp") || len(spec.RequiredFiles) != 1 || spec.RequiredFiles[0] != dependency {
		t.Fatal("launch access inputs lost", spec)
	}
	if info, err := os.Stat(spec.TempDirectory); err != nil || !info.IsDir() {
		t.Fatal("temporary directory must exist before the launch guard", err)
	}
	if !strings.Contains(strings.Join(spec.Environment, "\n"), "TEMP="+spec.TempDirectory+"\n") {
		t.Fatal("launch check and child TEMP disagree")
	}
	s.Profile.Requirements.RequiredFiles[0] = "changed after spec creation"
	if spec.RequiredFiles[0] != dependency {
		t.Fatal("launch input changed after snapshot")
	}
}
