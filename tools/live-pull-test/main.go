// Runs on the disposable Windows node; secrets never leave its process output.
package main

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/sys/windows"
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
		panic("remote pull probe failed; credential-bearing details omitted")
	}
}
func nonce() string {
	var b [16]byte
	_, e := rand.Read(b[:])
	need(e == nil)
	return hex.EncodeToString(b[:])
}

type config struct {
	Token    string                     `json:"token"`
	Panel    string                     `json:"panel_url"`
	Data     string                     `json:"data_directory"`
	Profiles map[string]json.RawMessage `json:"profiles"`
	Enabled  bool                       `json:"allow_remote_downloads"`
}
type progress struct {
	Identifier string
	Progress   float64
}

func main() {
	if len(os.Args) != 5 || os.Args[4] != "--allow-owned-fixture" {
		fmt.Fprintln(os.Stderr, "arguments: native-config public-HTTPS-base disposable-demo-UUID --allow-owned-fixture")
		os.Exit(2)
	}
	checks := map[string]bool{}
	report := map[string]any{"observed_utc": time.Now().UTC().Format(time.RFC3339), "identity_source": "operator master-authenticated pull API; synthetic signed source URLs", "browser_pull_tested": false, "checks": checks, "passed": false}
	var root *os.Root
	dir := ""
	owned := false
	failed := false
	defer func() {
		if recover() != nil {
			failed = true
		}
		if root != nil {
			if owned {
				for _, name := range []string{"pulled.bin", "async.bin", "cancel.bin", "source.bin", "cancel-source.bin"} {
					deadline := time.Now().Add(45 * time.Second)
					for {
						e := root.Remove(dir + "/" + name)
						if e == nil || os.IsNotExist(e) {
							break
						}
						if time.Now().After(deadline) {
							failed = true
							break
						}
						time.Sleep(50 * time.Millisecond)
					}
				}
				if e := root.Remove(dir); e != nil {
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
	var cfg config
	need(json.Unmarshal(b, &cfg) == nil && cfg.Enabled)
	base := strings.TrimRight(os.Args[2], "/")
	u, e := url.Parse(base)
	need(e == nil && u.Scheme == "https" && u.Host != "" && u.User == nil && u.Path == "" && u.RawQuery == "" && u.Fragment == "")
	id := os.Args[3]
	need(regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`).MatchString(id))
	_, ok := cfg.Profiles[id]
	need(ok)
	root, e = os.OpenRoot(filepath.Join(cfg.Data, id))
	need(e == nil)
	dir = "pull-probe-" + nonce()
	need(root.Mkdir(dir, 0700) == nil)
	owned = true
	data := make([]byte, 9<<20)
	_, e = rand.Read(data)
	need(e == nil)
	f, e := root.OpenFile(dir+"/source.bin", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	need(e == nil)
	_, e = f.Write(data)
	need(e == nil)
	need(f.Close() == nil)
	f, e = root.OpenFile(dir+"/cancel-source.bin", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	need(e == nil)
	var returned uint32
	e = windows.DeviceIoControl(windows.Handle(f.Fd()), windows.FSCTL_SET_SPARSE, nil, 0, nil, 0, &returned, nil)
	need(e == nil)
	need(f.Truncate(100<<20) == nil)
	need(f.Close() == nil)
	sourceURL := func(name string) string {
		token, e := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"iss": cfg.Panel, "aud": []string{base}, "jti": "pull-probe-" + nonce(), "iat": time.Now().Unix(), "nbf": time.Now().Add(-5 * time.Minute).Unix(), "exp": time.Now().Add(5 * time.Minute).Unix(), "scope": "file-download", "server_uuid": id, "user_uuid": "99999999-9999-4999-8999-" + nonce()[:12], "unique_id": nonce(), "file_path": "/" + dir + "/" + name}).SignedString([]byte(cfg.Token))
		need(e == nil)
		return base + "/download/file?token=" + url.QueryEscape(token)
	}
	client := &http.Client{Timeout: 130 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	send := func(method, suffix string, body any) (int, []byte) {
		var reader io.Reader
		if body != nil {
			b, e := json.Marshal(body)
			need(e == nil)
			reader = bytes.NewReader(b)
		}
		r, e := http.NewRequest(method, base+"/api/servers/"+id+"/files/pull"+suffix, reader)
		need(e == nil)
		r.Header.Set("Authorization", "Bearer "+cfg.Token)
		r.Header.Set("Content-Type", "application/json")
		resp, e := client.Do(r)
		need(e == nil)
		defer resp.Body.Close()
		b, e := io.ReadAll(io.LimitReader(resp.Body, 32<<10))
		need(e == nil)
		return resp.StatusCode, b
	}
	pull := func(name, source string, foreground bool) (int, []byte) {
		return send("POST", "", map[string]any{"root": "/" + dir, "url": source, "file_name": name, "foreground": foreground})
	}
	status, body := pull("pulled.bin", sourceURL("source.bin"), true)
	need(status == 200)
	var metadata struct {
		Name string `json:"name"`
		Size int64  `json:"size"`
	}
	need(json.Unmarshal(body, &metadata) == nil && metadata.Name == "pulled.bin" && metadata.Size == int64(len(data)))
	got, e := root.ReadFile(dir + "/pulled.bin")
	need(e == nil && sha256.Sum256(got) == sha256.Sum256(data))
	checks["foreground_public_https_nine_mib_integrity"] = true
	status, _ = pull("pulled.bin", sourceURL("source.bin"), true)
	need(status == 409)
	got, e = root.ReadFile(dir + "/pulled.bin")
	need(e == nil && sha256.Sum256(got) == sha256.Sum256(data))
	checks["existing_destination_preserved"] = true
	status, _ = pull("cancel.bin", "https://127.0.0.1/private", true)
	need(status == 422)
	checks["literal_loopback_refused"] = true
	status, _ = pull("cancel.bin", "https://localhost/private", true)
	need(status == 400)
	checks["dns_loopback_refused"] = true
	status, _ = pull("cancel.bin", "http://example.com/file", true)
	need(status == 422)
	checks["http_source_refused"] = true
	list := func() []progress {
		status, b := send("GET", "", nil)
		need(status == 200)
		var v struct {
			Downloads []progress `json:"downloads"`
		}
		need(json.Unmarshal(b, &v) == nil)
		need(!bytes.Contains(b, []byte("token=")) && !bytes.Contains(b, []byte("https://")))
		return v.Downloads
	}
	status, body = pull("async.bin", sourceURL("source.bin"), false)
	need(status == 202)
	var accepted struct {
		Identifier string `json:"identifier"`
	}
	need(json.Unmarshal(body, &accepted) == nil && accepted.Identifier != "")
	deadline := time.Now().Add(120 * time.Second)
	active := true
	for active {
		active = false
		for _, v := range list() {
			if v.Identifier == accepted.Identifier {
				need(v.Progress >= 0 && v.Progress <= 1)
				active = true
			}
		}
		if active {
			need(time.Now().Before(deadline))
			time.Sleep(50 * time.Millisecond)
		}
	}
	got, e = root.ReadFile(dir + "/async.bin")
	need(e == nil && sha256.Sum256(got) == sha256.Sum256(data))
	checks["background_accept_list_and_completion"] = true
	status, body = pull("cancel.bin", sourceURL("cancel-source.bin"), false)
	need(status == 202)
	need(json.Unmarshal(body, &accepted) == nil && accepted.Identifier != "")
	status, _ = send("DELETE", "/"+accepted.Identifier, nil)
	need(status == 204)
	deadline = time.Now().Add(15 * time.Second)
	active = true
	for active {
		active = false
		for _, v := range list() {
			if v.Identifier == accepted.Identifier {
				active = true
			}
		}
		if active {
			need(time.Now().Before(deadline))
			time.Sleep(50 * time.Millisecond)
		}
	}
	_, e = root.Lstat(dir + "/cancel.bin")
	need(os.IsNotExist(e))
	checks["early_cancel_removes_unpublished_file"] = true
	status, _ = send("DELETE", "/"+accepted.Identifier, nil)
	need(status == 204)
	checks["cancel_replay_is_idempotent"] = true
	f, e = root.Open(dir)
	need(e == nil)
	entries, e := f.ReadDir(-1)
	need(e == nil)
	need(f.Close() == nil)
	for _, entry := range entries {
		need(!strings.HasPrefix(entry.Name(), ".wings-pull-"))
	}
	checks["no_temporary_pulls_remain"] = true
	got, e = root.ReadFile(dir + "/source.bin")
	need(e == nil && sha256.Sum256(got) == sha256.Sum256(data))
	checks["source_binary_preserved"] = true
	report["source_bytes"] = len(data)
	report["cancel_source_bytes"] = 100 << 20
}
