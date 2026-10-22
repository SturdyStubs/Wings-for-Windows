package config

import "testing"

func TestLogFileApproval(t *testing.T) {
	for _, name := range []string{"server.log", "logs/server.log", "../secret", "C:/secret.log", "logs\\file.log", "logs/CON.txt", "log.txt:secret", "log*.txt", "logs//file.log"} {
		t.Run(name, func(t *testing.T) {
			c := valid(t)
			p := c.Profiles[id]
			p.LogFile = &LogFile{Path: name, Readiness: true}
			c.Profiles[id] = p
			err := c.Validate()
			if (err == nil) != (name == "server.log" || name == "logs/server.log") {
				t.Fatal(err)
			}
		})
	}
}
