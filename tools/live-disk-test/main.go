// Opt-in on-node disk measurement probe. Credentials never leave the node.
package main

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

func need(ok bool) {
	if !ok {
		panic("disk probe failed; sensitive details omitted")
	}
}

type disk struct {
	Bytes       int64  `json:"bytes"`
	Files       int    `json:"files"`
	Directories int    `json:"directories"`
	Available   bool   `json:"available"`
	Stale       bool   `json:"stale"`
	Scanning    bool   `json:"scanning"`
	Measured    string `json:"measured_at"`
	Quota       bool   `json:"quota_enforced"`
	Scope       string `json:"scope"`
}

func main() {
	if len(os.Args) != 5 || os.Args[4] != "--allow-owned-fixture" {
		fmt.Fprintln(os.Stderr, "arguments: native-config public-HTTPS-base disposable-demo-UUID --allow-owned-fixture")
		os.Exit(2)
	}
	checks := map[string]bool{}
	report := map[string]any{"observed_utc": time.Now().UTC().Format(time.RFC3339), "identity_source": "operator master-authenticated HTTPS; not panel browser", "checks": checks, "passed": false}
	failed := false
	owned := false
	var root *os.Root
	dir := ""
	defer func() {
		if recover() != nil {
			failed = true
		}
		if root != nil {
			if owned {
				for _, name := range []string{"binary", ".hidden", "empty"} {
					if e := root.Remove(dir + "/" + name); e != nil && !os.IsNotExist(e) {
						failed = true
					}
				}
				if e := root.Remove(dir); e != nil && !os.IsNotExist(e) {
					failed = true
				}
				_, e := root.Lstat(dir)
				checks["owned_fixture_removed"] = os.IsNotExist(e)
				if !checks["owned_fixture_removed"] {
					failed = true
				}
			}
			root.Close()
		}
		report["passed"] = !failed
		json.NewEncoder(os.Stdout).Encode(report)
		if failed {
			os.Exit(1)
		}
	}()
	b, e := os.ReadFile(os.Args[1])
	need(e == nil)
	var cfg struct {
		Token    string                     `json:"token"`
		Data     string                     `json:"data_directory"`
		Profiles map[string]json.RawMessage `json:"profiles"`
	}
	need(json.Unmarshal(b, &cfg) == nil)
	base := strings.TrimRight(os.Args[2], "/")
	u, e := url.Parse(base)
	need(e == nil && u.Scheme == "https" && u.Host != "" && u.Path == "" && u.User == nil && u.RawQuery == "" && u.Fragment == "")
	id := os.Args[3]
	need(regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`).MatchString(id))
	_, ok := cfg.Profiles[id]
	need(ok)
	root, e = os.OpenRoot(filepath.Join(cfg.Data, id))
	need(e == nil)
	client := &http.Client{Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	get := func() (disk, int64) {
		r, e := http.NewRequest("GET", base+"/api/servers/"+id, nil)
		need(e == nil)
		r.Header.Set("Authorization", "Bearer "+cfg.Token)
		response, e := client.Do(r)
		need(e == nil)
		defer response.Body.Close()
		need(response.StatusCode == 200)
		var v struct {
			Util struct {
				Bytes  int64 `json:"disk_bytes"`
				Native struct {
					Disk      disk `json:"disk"`
					Available bool `json:"disk_available"`
				} `json:"native_metrics"`
			} `json:"utilization"`
		}
		need(json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&v) == nil)
		need(v.Util.Native.Available == v.Util.Native.Disk.Available)
		return v.Util.Native.Disk, v.Util.Bytes
	}
	wait := func(after string, expected *disk) disk {
		deadline := time.Now().Add(48 * time.Second)
		for time.Now().Before(deadline) {
			d, n := get()
			need(n == d.Bytes)
			if d.Available && !d.Stale && !d.Scanning && d.Measured != after && (expected == nil || d.Bytes == expected.Bytes && d.Files == expected.Files && d.Directories == expected.Directories) {
				return d
			}
			time.Sleep(250 * time.Millisecond)
		}
		panic("disk refresh did not reach expected state")
	}
	before := wait("", nil)
	need(!before.Quota && before.Scope == "data_root_logical_files")
	checks["baseline_available_without_quota_claim"] = true
	var random [16]byte
	_, e = rand.Read(random[:])
	need(e == nil)
	dir = "disk-probe-" + hex.EncodeToString(random[:])
	need(root.Mkdir(dir, 0700) == nil)
	owned = true
	for name, size := range map[string]int{"binary": 9 << 20, ".hidden": 3, "empty": 0} {
		f, e := root.OpenFile(dir+"/"+name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		need(e == nil)
		_, e = io.CopyN(f, bytes.NewReader(make([]byte, size)), int64(size))
		need(e == nil)
		need(f.Close() == nil)
	}
	expected := disk{Bytes: before.Bytes + (9 << 20) + 3, Files: before.Files + 3, Directories: before.Directories + 1}
	after := wait(before.Measured, &expected)
	checks["nine_mib_hidden_empty_files_counted_exactly"] = true
	checks["public_disk_bytes_matches_native_measurement"] = true
	checks["refresh_advances_measurement_time"] = after.Measured != before.Measured
	for _, name := range []string{"binary", ".hidden", "empty"} {
		need(root.Remove(dir+"/"+name) == nil)
	}
	need(root.Remove(dir) == nil)
	restored := wait(after.Measured, &before)
	checks["deletion_restores_baseline"] = restored.Bytes == before.Bytes
	report["fixture_bytes"] = (9 << 20) + 3
	report["measurement_semantics"] = "logical unnamed file stream bytes, not allocated disk space"
	report["browser_dashboard_tested"] = false
}
