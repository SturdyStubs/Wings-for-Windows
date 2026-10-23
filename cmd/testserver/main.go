// A harmless stdio server used only by local runtime integration tests.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
	"wings-windows/internal/identity"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "install" {
		runInstallerFixture()
	}
	if len(os.Args) > 2 && os.Args[1] == "rcon" {
		startRCONFixture(os.Args[2])
	}
	if len(os.Args) > 1 && os.Args[1] == "child" {
		for {
			time.Sleep(time.Second)
		}
	}
	if len(os.Args) > 1 && os.Args[1] == "crash-on-start" {
		os.Exit(23)
	}
	if len(os.Args) > 1 && os.Args[1] == "colored" {
		fmt.Println("\x1b[32mSERVER_READY\x1b[0m")
	} else {
		fmt.Println("SERVER_READY")
	}
	if len(os.Args) > 1 && os.Args[1] == "noinput" {
		for {
			time.Sleep(time.Second)
		}
	}
	if len(os.Args) > 1 && os.Args[1] == "arguments" {
		b, _ := json.Marshal(os.Args[2:])
		fmt.Println("ARGS:" + string(b))
	}
	if len(os.Args) > 1 && (os.Args[1] == "spawn" || os.Args[1] == "rcon") {
		p := exec.Command(os.Args[0], "child")
		if p.Start() == nil {
			fmt.Printf("CHILD:%d\n", p.Process.Pid)
		}
	}
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := scanner.Text()
		if probeRCONCredential(line) {
			continue
		}
		if writeFixtureLog(line) {
			continue
		}
		if line == "probe-code-write" {
			file, err := os.OpenFile(os.Getenv("CODE_WRITE_PROBE_PATH"), os.O_WRONLY, 0)
			switch {
			case os.IsPermission(err):
				fmt.Println("CODE_WRITE_PROBE_DENIED")
			case err == nil:
				file.Close()
				fmt.Println("CODE_WRITE_PROBE_ALLOWED")
			default:
				fmt.Println("CODE_WRITE_PROBE_ERROR")
			}
			continue
		}
		if line == "probe-log-access" || line == "probe-recovery-access" || line == "probe-intent-access" || line == "probe-install-access" || line == "probe-other-data-access" {
			variable, prefix := "LOG_PROBE_PATH", "LOG_PROBE"
			if line == "probe-recovery-access" {
				variable, prefix = "RECOVERY_PROBE_PATH", "RECOVERY_PROBE"
			}
			if line == "probe-intent-access" {
				variable, prefix = "INTENT_PROBE_PATH", "INTENT_PROBE"
			}
			if line == "probe-install-access" {
				variable, prefix = "INSTALL_STATE_PROBE_PATH", "INSTALL_STATE_PROBE"
			}
			if line == "probe-other-data-access" {
				variable, prefix = "OTHER_DATA_PROBE_PATH", "OTHER_DATA_PROBE"
			}
			_, err := os.ReadFile(os.Getenv(variable))
			switch {
			case os.IsPermission(err):
				fmt.Println(prefix + "_DENIED")
			case err == nil:
				fmt.Println(prefix + "_ALLOWED")
			default:
				fmt.Println(prefix + "_ERROR")
			}
			continue
		}
		if line == "crash" {
			os.Exit(23)
		}
		if line == "stop" {
			fmt.Println("STOPPING")
			return
		}
		fmt.Println("ECHO:" + line)
	}
}

func probeRCONCredential(line string) bool {
	if !strings.HasPrefix(line, "probe-rcon-credential ") {
		return false
	}
	status, err := identity.RCONCredentialStatus(strings.TrimPrefix(line, "probe-rcon-credential "))
	if err == nil && status.Present {
		fmt.Println("RCON_CREDENTIAL_PRESENT")
	} else {
		fmt.Println("RCON_CREDENTIAL_UNAVAILABLE")
	}
	return true
}
