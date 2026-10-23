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
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

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
	File   string `json:"file_path"`
	Unique string `json:"unique_id"`
}

func need(ok bool) {
	if !ok {
		panic("probe failed; credential-bearing details omitted")
	}
}
func nonce() string {
	b := make([]byte, 16)
	_, e := rand.Read(b)
	need(e == nil)
	return hex.EncodeToString(b)
}
func main() {
	if len(os.Args) != 5 || os.Args[4] != "--allow-owned-fixture" {
		fmt.Fprintln(os.Stderr, "arguments: native-config public-HTTPS-base disposable-demo-UUID --allow-owned-fixture")
		os.Exit(2)
	}
	checks := map[string]bool{}
	report := map[string]any{"observed_utc": time.Now().UTC().Format(time.RFC3339), "identity_source": "operator-minted synthetic file JWTs; not real panel-issued links", "browser_download_tested": false, "checks": checks, "passed": false}
	var root *os.Root
	var dir string
	owned := false
	failed := false
	defer func() {
		if recover() != nil {
			failed = true
		}
		if root != nil {
			if owned {
				for _, f := range []string{"large.bin", "small.bin", "empty.bin"} {
					if e := root.Remove(dir + "/" + f); e != nil && !os.IsNotExist(e) {
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
	var c config
	need(json.Unmarshal(b, &c) == nil)
	base := strings.TrimRight(os.Args[2], "/")
	u, e := url.Parse(base)
	need(e == nil && u.Scheme == "https" && u.Host != "" && u.User == nil && u.Path == "" && u.RawQuery == "" && u.Fragment == "")
	id := os.Args[3]
	need(regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`).MatchString(id))
	_, ok := c.Profiles[id]
	need(ok)
	root, e = os.OpenRoot(filepath.Join(c.Data, id))
	need(e == nil)
	dir = "download-probe-" + nonce()
	need(root.Mkdir(dir, 0700) == nil)
	owned = true
	large := bytes.Repeat([]byte{0, 255, 7}, 3<<20)
	small := []byte("signed-file-content")
	for name, data := range map[string][]byte{"large.bin": large, "small.bin": small, "empty.bin": nil} {
		f, e := root.OpenFile(dir+"/"+name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		need(e == nil)
		_, e = f.Write(data)
		need(e == nil)
		need(f.Close() == nil)
	}
	user := "99999999-9999-4999-8999-" + nonce()[:12]
	jti := "download-probe-" + nonce()
	fresh := func(file string) claims {
		return claims{RegisteredClaims: jwt.RegisteredClaims{Issuer: c.Panel, Audience: jwt.ClaimStrings{base}, ID: jti, IssuedAt: jwt.NewNumericDate(time.Now()), NotBefore: jwt.NewNumericDate(time.Now().Add(-5 * time.Minute)), ExpiresAt: jwt.NewNumericDate(time.Now().Add(5 * time.Minute))}, Scope: "file-download", Server: id, User: user, File: "/" + dir + "/" + file, Unique: nonce()}
	}
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.DisableCompression = true
	defer tr.CloseIdleConnections()
	client := &http.Client{Transport: tr, Timeout: 45 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	get := func(cl claims, extra, rangeValue string) *http.Response {
		signed, e := jwt.NewWithClaims(jwt.SigningMethodHS256, cl).SignedString([]byte(c.Token))
		need(e == nil)
		r, e := http.NewRequest("GET", base+"/download/file?token="+url.QueryEscape(signed)+extra, nil)
		need(e == nil)
		if rangeValue != "" {
			r.Header.Set("Range", rangeValue)
		}
		resp, e := client.Do(r)
		need(e == nil)
		return resp
	}
	discard := func(resp *http.Response, status int) {
		defer resp.Body.Close()
		need(resp.StatusCode == status)
		_, e := io.Copy(io.Discard, io.LimitReader(resp.Body, 8192))
		need(e == nil)
	}
	verify := func(resp *http.Response, data []byte) {
		defer resp.Body.Close()
		need(resp.StatusCode == 200)
		h := sha256.New()
		n, e := io.CopyBuffer(h, resp.Body, make([]byte, 64<<10))
		need(e == nil && n == int64(len(data)))
		sum := sha256.Sum256(data)
		need(bytes.Equal(h.Sum(nil), sum[:]))
		need(resp.ContentLength == int64(len(data)) && resp.Header.Get("Content-Type") == "application/octet-stream" && strings.HasPrefix(resp.Header.Get("Content-Disposition"), "attachment;"))
	}
	cl := fresh("large.bin")
	verify(get(cl, "", ""), large)
	checks["large_binary_integrity_and_headers"] = true
	report["large_bytes"] = len(large)
	discard(get(cl, "", ""), 404)
	checks["one_use_replay_rejected"] = true
	cl = fresh("small.bin")
	verify(get(cl, "", ""), small)
	cl.Unique = nonce()
	verify(get(cl, "", ""), small)
	checks["same_jti_new_unique_accepted"] = true
	cl = fresh("small.bin")
	verify(get(cl, "&file=/install.marker", ""), small)
	checks["query_path_override_ignored"] = true
	cl = fresh("small.bin")
	discard(get(cl, "", "bytes=0-1"), 416)
	verify(get(cl, "", ""), small)
	checks["range_refusal_preserves_full_link"] = true
	cl = fresh("small.bin")
	cl.Scope = "websocket"
	discard(get(cl, "", ""), 401)
	checks["wrong_scope_rejected"] = true
	cl = fresh("empty.bin")
	verify(get(cl, "", ""), nil)
	checks["empty_file_download"] = true
	cl = fresh("small.bin")
	body, e := json.Marshal(map[string]any{"user": strings.ToUpper(user), "servers": []string{id}})
	need(e == nil)
	r, e := http.NewRequest("POST", base+"/api/deauthorize-user", bytes.NewReader(body))
	need(e == nil)
	r.Header.Set("Authorization", "Bearer "+c.Token)
	r.Header.Set("Content-Type", "application/json")
	resp, e := client.Do(r)
	need(e == nil)
	discard(resp, 204)
	discard(get(cl, "", ""), 401)
	checks["synthetic_user_revocation_enforced"] = true
	time.Sleep(1200 * time.Millisecond)
	verify(get(fresh("small.bin"), "", ""), small)
	checks["fresh_post_revocation_link_accepted"] = true
	need(len(checks) == 9)
}
