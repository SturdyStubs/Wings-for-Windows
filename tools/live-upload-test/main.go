// Opt-in live probe. Node secrets stay on the node and are never printed.
package main

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/golang-jwt/jwt/v5"
	"io"
	"mime/multipart"
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
		panic("probe failed; credential-bearing details omitted")
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
}
type claims struct {
	jwt.RegisteredClaims
	Scope  string `json:"scope"`
	Server string `json:"server_uuid"`
	User   string `json:"user_uuid"`
	Unique string `json:"unique_id"`
}

func main() {
	if len(os.Args) != 5 || os.Args[4] != "--allow-owned-fixture" {
		fmt.Fprintln(os.Stderr, "arguments: native-config public-HTTPS-base disposable-demo-UUID --allow-owned-fixture")
		os.Exit(2)
	}
	checks := map[string]bool{}
	report := map[string]any{"observed_utc": time.Now().UTC().Format(time.RFC3339), "identity_source": "operator-minted synthetic upload JWTs; not real panel-issued links", "browser_upload_tested": false, "checks": checks, "passed": false}
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
				for _, name := range []string{"small.bin", "empty.bin", "large.bin", "local-large.bin", "after.bin"} {
					if e := root.Remove(dir + "/" + name); e != nil && !os.IsNotExist(e) {
						failed = true
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
	need(json.Unmarshal(b, &cfg) == nil)
	base := strings.TrimRight(os.Args[2], "/")
	u, e := url.Parse(base)
	need(e == nil && u.Scheme == "https" && u.Host != "" && u.User == nil && u.Path == "" && u.RawQuery == "" && u.Fragment == "")
	id := os.Args[3]
	need(regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`).MatchString(id))
	_, ok := cfg.Profiles[id]
	need(ok)
	root, e = os.OpenRoot(filepath.Join(cfg.Data, id))
	need(e == nil)
	dir = "upload-probe-" + nonce()
	need(root.Mkdir(dir, 0700) == nil)
	owned = true
	user := "99999999-9999-4999-8999-" + nonce()[:12]
	jti := "upload-probe-" + nonce()
	fresh := func() claims {
		return claims{RegisteredClaims: jwt.RegisteredClaims{Issuer: cfg.Panel, Audience: jwt.ClaimStrings{base}, ID: jti, IssuedAt: jwt.NewNumericDate(time.Now()), NotBefore: jwt.NewNumericDate(time.Now().Add(-5 * time.Minute)), ExpiresAt: jwt.NewNumericDate(time.Now().Add(5 * time.Minute))}, Scope: "file-upload", Server: id, User: user, Unique: nonce()}
	}
	client := &http.Client{Timeout: 60 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	send := func(endpoint string, c claims, files map[string][]byte) int {
		var b bytes.Buffer
		m := multipart.NewWriter(&b)
		for name, data := range files {
			p, e := m.CreateFormFile("files", name)
			need(e == nil)
			_, e = p.Write(data)
			need(e == nil)
		}
		need(m.Close() == nil)
		token, e := jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString([]byte(cfg.Token))
		need(e == nil)
		r, e := http.NewRequest("POST", endpoint+"/upload/file?token="+url.QueryEscape(token)+"&directory="+url.QueryEscape("/"+dir), &b)
		need(e == nil)
		r.Header.Set("Content-Type", m.FormDataContentType())
		resp, e := client.Do(r)
		need(e == nil)
		defer resp.Body.Close()
		_, e = io.Copy(io.Discard, io.LimitReader(resp.Body, 8192))
		need(e == nil)
		return resp.StatusCode
	}
	verify := func(name string, want []byte) {
		b, e := root.ReadFile(dir + "/" + name)
		need(e == nil && sha256.Sum256(b) == sha256.Sum256(want))
	}
	small := []byte{0, 255, 1, 2, 0, 3}
	c := fresh()
	need(send(base, c, map[string][]byte{"small.bin": small, "empty.bin": nil}) == 200)
	verify("small.bin", small)
	verify("empty.bin", nil)
	checks["public_binary_multipart_and_empty"] = true
	need(send(base, c, map[string][]byte{"small.bin": []byte("replacement")}) == 404)
	checks["one_use_replay_rejected"] = true
	need(send(base, fresh(), map[string][]byte{"small.bin": []byte("replacement")}) == 409)
	verify("small.bin", small)
	checks["existing_file_preserved"] = true
	c = fresh()
	c.Scope = "file-download"
	need(send(base, c, map[string][]byte{"after.bin": nil}) == 401)
	checks["wrong_scope_rejected"] = true
	need(send(base, fresh(), map[string][]byte{"../escape.bin": small}) == 422)
	checks["raw_traversal_filename_rejected"] = true
	large := bytes.Repeat([]byte{0, 255, 7}, 3<<20)
	status := send(base, fresh(), map[string][]byte{"large.bin": large})
	report["public_large_upload_status"] = status
	report["large_bytes"] = len(large)
	if status == 200 {
		verify("large.bin", large)
		checks["public_large_binary_integrity"] = true
		report["public_large_upload_verified"] = true
	} else {
		need(status == 413)
		report["public_large_upload_verified"] = false
		report["public_proxy_size_gate_pending"] = true
	}
	need(send("http://127.0.0.1:8080", fresh(), map[string][]byte{"local-large.bin": large}) == 200)
	verify("local-large.bin", large)
	checks["retained_service_large_binary_integrity"] = true
	c = fresh()
	body, e := json.Marshal(map[string]any{"user": strings.ToUpper(user), "servers": []string{id}})
	need(e == nil)
	r, e := http.NewRequest("POST", base+"/api/deauthorize-user", bytes.NewReader(body))
	need(e == nil)
	r.Header.Set("Authorization", "Bearer "+cfg.Token)
	r.Header.Set("Content-Type", "application/json")
	resp, e := client.Do(r)
	need(e == nil)
	need(resp.StatusCode == 204)
	resp.Body.Close()
	need(send(base, c, map[string][]byte{"after.bin": nil}) == 401)
	checks["synthetic_user_revocation_enforced"] = true
	time.Sleep(1200 * time.Millisecond)
	need(send(base, fresh(), map[string][]byte{"after.bin": nil}) == 200)
	checks["fresh_post_revocation_link_accepted"] = true
	folder, e := root.Open(dir)
	need(e == nil)
	entries, e := folder.ReadDir(-1)
	need(folder.Close() == nil)
	need(e == nil)
	for _, entry := range entries {
		need(!strings.HasPrefix(entry.Name(), ".wings-upload-"))
	}
	checks["no_temporary_uploads_remain"] = true
}
