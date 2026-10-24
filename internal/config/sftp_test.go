package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestSFTPExplicitLoopbackConfiguration(t *testing.T) {
	for _, kind := range []string{"disabled", "valid", "uploads", "management", "public", "hostname", "zero", "invalid-port", "same-listener", "relative-key", "data-key", "different-parent", "same-config"} {
		t.Run(kind, func(t *testing.T) {
			c := valid(t)
			dir := t.TempDir()
			filename := filepath.Join(dir, "config.json")
			if kind != "disabled" {
				c.SFTP = &SFTP{Listen: "127.0.0.1:2203", HostKey: filepath.Join(dir, "host-key")}
			}
			switch kind {
			case "management":
				c.SFTP.AllowFileManagement = true
			case "uploads":
				c.SFTP.AllowUploads = true
			case "public":
				c.SFTP.Listen = "0.0.0.0:2203"
			case "hostname":
				c.SFTP.Listen = "localhost:2203"
			case "zero":
				c.SFTP.Listen = "127.0.0.1:0"
			case "invalid-port":
				c.SFTP.Listen = "127.0.0.1:70000"
			case "same-listener":
				c.SFTP.Listen = c.Listen
			case "relative-key":
				c.SFTP.HostKey = "host-key"
			case "data-key":
				c.SFTP.HostKey = filepath.Join(c.DataDirectory, "host-key")
			case "different-parent":
				c.SFTP.HostKey = filepath.Join(t.TempDir(), "host-key")
			case "same-config":
				c.SFTP.HostKey = filename
			}
			b, e := json.Marshal(c)
			if e != nil {
				t.Fatal(e)
			}
			if e = os.WriteFile(filename, b, 0600); e != nil {
				t.Fatal(e)
			}
			loaded, e := Load(filename)
			want := kind == "valid" || kind == "disabled" || kind == "uploads" || kind == "management"
			if (e == nil) != want {
				t.Fatal(e)
			}
			if kind == "uploads" && !loaded.SFTP.AllowUploads {
				t.Fatal("upload opt-in lost")
			}
			if kind == "management" && !loaded.SFTP.AllowFileManagement {
				t.Fatal("management opt-in lost")
			}
			if kind == "valid" && loaded.SFTP == nil {
				t.Fatal("configuration lost")
			}
		})
	}
}
