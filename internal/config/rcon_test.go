package config

import "testing"

func TestRCONConfiguration(t *testing.T) {
	for _, mode := range []string{"valid", "ipv6", "public", "hostname", "bad-port", "zero-port", "secret-name", "timeout", "forward-secret", "duplicate"} {
		t.Run(mode, func(t *testing.T) {
			c := valid(t)
			p := c.Profiles[id]
			p.SourceRCON = &SourceRCON{Address: "127.0.0.1:27015", PasswordEnvironment: "WINGS_RCON_TEST"}
			switch mode {
			case "ipv6":
				p.SourceRCON.Address = "[::1]:27015"
			case "public":
				p.SourceRCON.Address = "192.0.2.1:27015"
			case "hostname":
				p.SourceRCON.Address = "localhost:27015"
			case "bad-port":
				p.SourceRCON.Address = "127.0.0.1:027015"
			case "zero-port":
				p.SourceRCON.Address = "127.0.0.1:0"
			case "secret-name":
				p.SourceRCON.PasswordEnvironment = "PATH"
			case "timeout":
				p.SourceRCON.TimeoutSeconds = 11
			case "forward-secret":
				p.AllowedEnvironment = []string{"WINGS_RCON_TEST"}
			case "duplicate":
				c.Profiles["22222222-2222-4222-8222-222222222222"] = p
			}
			c.Profiles[id] = p
			err := c.Validate()
			if (err == nil) != (mode == "valid" || mode == "ipv6") {
				t.Fatal(err)
			}
			if err == nil && p.SourceRCON.TimeoutSeconds != 3 {
				t.Fatal("timeout default missing")
			}
		})
	}
}

func TestRCONCredentialReferenceApproval(t *testing.T) {
	for _, mode := range []string{"valid", "both", "missing", "other-server", "game-target"} {
		t.Run(mode, func(t *testing.T) {
			c := valid(t)
			p := c.Profiles[id]
			p.SourceRCON = &SourceRCON{Address: "127.0.0.1:27015", PasswordCredentialTarget: "WingsWindowsRCON/" + id}
			switch mode {
			case "both":
				p.SourceRCON.PasswordEnvironment = "WINGS_RCON_TEST"
			case "missing":
				p.SourceRCON.PasswordCredentialTarget = ""
			case "other-server":
				p.SourceRCON.PasswordCredentialTarget = "WingsWindowsRCON/22222222-2222-4222-8222-222222222222"
			case "game-target":
				p.SourceRCON.PasswordCredentialTarget = "WingsWindows/" + id
			}
			c.Profiles[id] = p
			if err := c.Validate(); (err == nil) != (mode == "valid") {
				t.Fatal(mode, err)
			}
		})
	}
}
