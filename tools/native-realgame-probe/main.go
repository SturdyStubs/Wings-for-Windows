package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"wings-windows/internal/native"
)

func main() {
	if len(os.Args) != 4 || os.Args[3] != "--allow-local-game" {
		fmt.Fprintln(os.Stderr, "arguments: approved-openttd.exe disposable-data-directory --allow-local-game")
		os.Exit(2)
	}
	exe, data := os.Args[1], os.Args[2]
	if !filepath.IsAbs(exe) || !filepath.IsAbs(data) || !strings.EqualFold(filepath.Base(exe), "openttd.exe") {
		os.Exit(2)
	}
	tcpProbe, e := net.Listen("tcp", "127.0.0.1:43979")
	if e != nil {
		fmt.Println("test port unavailable")
		os.Exit(1)
	}
	tcpProbe.Close()
	cfg := filepath.Join(data, "openttd.cfg")
	p, e := native.Start(native.Spec{Pseudoconsole: true, Executable: exe, Arguments: []string{"-D", "127.0.0.1:43979", "-c", cfg}, Directory: data, Environment: []string{"SystemRoot=" + os.Getenv("SystemRoot")}, MemoryMB: 1024})
	if e != nil {
		fmt.Println("launch failed")
		os.Exit(1)
	}
	var mu sync.Mutex
	var out bytes.Buffer
	drained := make(chan struct{})
	go func() {
		defer close(drained)
		defer p.Stdout.Close()
		b := make([]byte, 8192)
		for {
			n, e := p.Stdout.Read(b)
			if n > 0 {
				mu.Lock()
				if out.Len() < 1<<20 {
					out.Write(b[:n])
				}
				mu.Unlock()
			}
			if e != nil {
				return
			}
		}
	}()
	time.Sleep(4 * time.Second)
	stats, _ := p.Stats()
	conn, dialErr := net.DialTimeout("tcp", "127.0.0.1:43979", time.Second)
	if conn != nil {
		conn.Close()
	}
	udpIdentity := false
	udp, udpErr := net.DialTimeout("udp", "127.0.0.1:43979", time.Second)
	if udpErr == nil {
		udp.SetDeadline(time.Now().Add(2 * time.Second))
		_, udpErr = udp.Write([]byte{3, 0, 0})
		if udpErr == nil {
			b := make([]byte, 4096)
			n, e := udp.Read(b)
			udpIdentity = e == nil && n >= 4 && int(binary.LittleEndian.Uint16(b[:2])) == n && b[2] == 1 && bytes.Contains(b[:n], []byte("Wings Windows OpenTTD"))
		}
		udp.Close()
	}
	commandErr := p.Command("echo WINGS_REAL_GAME_TEST")
	versionErr := p.Command("version")
	time.Sleep(500 * time.Millisecond)
	stopErr := p.Command("quit")
	forced := false
	select {
	case <-p.Done:
	case <-time.After(10 * time.Second):
		forced = true
		p.Kill()
		<-p.Done
	}
	select {
	case <-drained:
	case <-time.After(3 * time.Second):
		p.Stdout.Close()
	}
	mu.Lock()
	text := out.String()
	mu.Unlock()
	_ = os.WriteFile(filepath.Join(data, "probe-console.log"), []byte(text), 0600)
	passed := !forced && commandErr == nil && versionErr == nil && stopErr == nil && p.ExitCode == 0 && strings.Contains(text, "WINGS_REAL_GAME_TEST")
	json.NewEncoder(os.Stdout).Encode(map[string]any{"passed": passed, "exit": p.ExitCode, "forced": forced, "job_cleanup_ok": p.CleanupError == nil, "console_echo": strings.Contains(text, "WINGS_REAL_GAME_TEST"), "udp_server_identity": udpIdentity, "listening": dialErr == nil, "memory_bytes": stats.MemoryBytes, "console_bytes": len(text)})
	if !passed {
		os.Exit(1)
	}
}
