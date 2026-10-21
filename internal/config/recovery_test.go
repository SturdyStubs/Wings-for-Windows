package config

import "testing"

func TestCrashRecoveryPolicy(t *testing.T) {
	for _, name := range []string{"defaults", "disabled defaults", "valid", "negative retries", "excess retries", "short window", "long window", "negative delay", "long delay", "delay outside window"} {
		t.Run(name, func(t *testing.T) {
			p := CrashRecovery{Enabled: true}
			switch name {
			case "disabled defaults":
				p.Enabled = false
			case "valid":
				p.MaxRestarts = 10
				p.WindowSeconds = 3600
				p.DelaySeconds = 300
			case "negative retries":
				p.MaxRestarts = -1
			case "excess retries":
				p.MaxRestarts = 11
			case "short window":
				p.WindowSeconds = 29
			case "long window":
				p.WindowSeconds = 3601
			case "negative delay":
				p.DelaySeconds = -1
			case "long delay":
				p.DelaySeconds = 301
			case "delay outside window":
				p.WindowSeconds = 30
				p.DelaySeconds = 30
			}
			err := p.Normalize()
			valid := name == "defaults" || name == "disabled defaults" || name == "valid"
			if (err == nil) != valid {
				t.Fatalf("unexpected validation result: %v", err)
			}
			if name == "defaults" && (p.MaxRestarts != 3 || p.WindowSeconds != 300 || p.DelaySeconds != 5) {
				t.Fatal("defaults missing")
			}
		})
	}
}

func TestConfigCrashRecoveryValidation(t *testing.T) {
	c := valid(t)
	p := c.Profiles[id]
	p.CrashRecovery = &CrashRecovery{Enabled: true}
	c.Profiles[id] = p
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	if c.Profiles[id].CrashRecovery.MaxRestarts != 3 {
		t.Fatal("configuration did not normalize recovery defaults")
	}
	p.CrashRecovery.MaxRestarts = 11
	if c.Validate() == nil {
		t.Fatal("configuration accepted unbounded recovery")
	}
}
