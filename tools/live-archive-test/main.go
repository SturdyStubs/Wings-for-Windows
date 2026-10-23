// Live archive creation probe; credentials remain on the node.
package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
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

func need(ok bool) {
	if !ok {
		panic("archive probe failed; credential-bearing details omitted")
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
type metadata struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
	File bool   `json:"file"`
	Mime string `json:"mime"`
}

func main() {
	if len(os.Args) != 5 || os.Args[4] != "--allow-owned-fixture" {
		fmt.Fprintln(os.Stderr, "arguments: native-config public-HTTPS-base disposable-demo-UUID --allow-owned-fixture")
		os.Exit(2)
	}
	checks := map[string]bool{}
	report := map[string]any{"observed_utc": time.Now().UTC().Format(time.RFC3339), "browser_compress_tested": false, "identity_source": "operator master-authenticated compression and synthetic download JWTs", "checks": checks, "passed": false}
	var root *os.Root
	dir := ""
	owned := false
	failed := false
	var archives []string
	defer func() {
		if recover() != nil {
			failed = true
		}
		if root != nil {
			if owned {
				for _, name := range archives {
					if e := root.Remove(dir + "/" + name); e != nil && !os.IsNotExist(e) {
						failed = true
					}
				}
				for _, name := range []string{"nested/Café.bin", "empty.bin", "nested", "empty-dir"} {
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
	dir = "archive-probe-" + nonce()
	need(root.Mkdir(dir, 0700) == nil)
	owned = true
	need(root.Mkdir(dir+"/nested", 0700) == nil)
	need(root.Mkdir(dir+"/empty-dir", 0700) == nil)
	large := make([]byte, 9<<20)
	_, e = rand.Read(large)
	need(e == nil)
	for name, value := range map[string][]byte{"nested/Café.bin": large, "empty.bin": nil} {
		f, e := root.OpenFile(dir+"/"+name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		need(e == nil)
		_, e = f.Write(value)
		need(e == nil)
		need(f.Close() == nil)
	}
	client := &http.Client{Timeout: 75 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	send := func(files []string) (int, metadata) {
		b, e := json.Marshal(map[string]any{"root": "/" + dir, "files": files})
		need(e == nil)
		r, e := http.NewRequest("POST", base+"/api/servers/"+id+"/files/compress", bytes.NewReader(b))
		need(e == nil)
		r.Header.Set("Authorization", "Bearer "+cfg.Token)
		r.Header.Set("Content-Type", "application/json")
		resp, e := client.Do(r)
		need(e == nil)
		defer resp.Body.Close()
		var m metadata
		if resp.StatusCode == 200 {
			need(json.NewDecoder(io.LimitReader(resp.Body, 8192)).Decode(&m) == nil)
			need(regexp.MustCompile(`^archive-[0-9TZ-]+-[0-9a-f]{12}\.tar\.gz$`).MatchString(m.Name))
			archives = append(archives, m.Name)
		} else {
			_, e = io.Copy(io.Discard, io.LimitReader(resp.Body, 8192))
			need(e == nil)
		}
		return resp.StatusCode, m
	}
	status, m := send([]string{"nested", "empty.bin", "empty-dir"})
	need(status == 200 && m.File && m.Mime == "application/tar+gzip" && m.Size > int64(len(large)))
	checks["public_compress_metadata"] = true
	token, e := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"iss": cfg.Panel, "aud": []string{base}, "jti": "archive-probe-" + nonce(), "iat": time.Now().Unix(), "nbf": time.Now().Add(-5 * time.Minute).Unix(), "exp": time.Now().Add(5 * time.Minute).Unix(), "scope": "file-download", "server_uuid": id, "user_uuid": "99999999-9999-4999-8999-" + nonce()[:12], "unique_id": nonce(), "file_path": "/" + dir + "/" + m.Name}).SignedString([]byte(cfg.Token))
	need(e == nil)
	r, e := http.NewRequest("GET", base+"/download/file?token="+url.QueryEscape(token), nil)
	need(e == nil)
	resp, e := client.Do(r)
	need(e == nil)
	need(resp.StatusCode == 200 && resp.ContentLength == m.Size)
	archiveHash := sha256.New()
	z, e := gzip.NewReader(io.TeeReader(resp.Body, archiveHash))
	need(e == nil)
	tr := tar.NewReader(z)
	seen := map[string]bool{}
	for {
		h, e := tr.Next()
		if e == io.EOF {
			break
		}
		need(e == nil && !seen[h.Name])
		seen[h.Name] = true
		switch h.Name {
		case "nested/", "empty-dir/":
			need(h.Typeflag == tar.TypeDir && h.Size == 0)
		case "empty.bin":
			need(h.Typeflag == tar.TypeReg && h.Size == 0)
		case "nested/Café.bin":
			need(h.Typeflag == tar.TypeReg && h.Size == int64(len(large)))
			sum := sha256.New()
			n, e := io.Copy(sum, tr)
			need(e == nil && n == int64(len(large)))
			want := sha256.Sum256(large)
			need(bytes.Equal(sum.Sum(nil), want[:]))
		default:
			need(false)
		}
	}
	_, e = io.Copy(io.Discard, z)
	need(e == nil)
	need(z.Close() == nil)
	need(resp.Body.Close() == nil)
	need(len(seen) == 4)
	checks["public_signed_download_and_tar_round_trip"] = true
	checks["unicode_binary_empty_and_directory_entries"] = true
	report["source_bytes"] = len(large)
	report["archive_bytes"] = m.Size
	first, e := root.ReadFile(dir + "/" + m.Name)
	need(e == nil)
	firstHash := sha256.Sum256(first)
	need(bytes.Equal(archiveHash.Sum(nil), firstHash[:]))
	checks["archive_bytes_match_public_download"] = true
	status, m2 := send([]string{"nested", "empty.bin", "empty-dir"})
	need(status == 200 && m2.Name != m.Name)
	retained, e := root.ReadFile(dir + "/" + m.Name)
	need(e == nil && sha256.Sum256(retained) == firstHash)
	checks["repeat_request_preserves_prior_archive"] = true
	status, _ = send([]string{"../install.marker"})
	need(status == 422)
	checks["traversal_refused"] = true
	status, _ = send([]string{"nested", "nested/Café.bin"})
	need(status == 422)
	checks["overlapping_selection_refused"] = true
	locked, e := root.OpenFile(dir+"/nested/Café.bin", os.O_WRONLY, 0)
	need(e == nil)
	status, _ = send([]string{"nested/Café.bin"})
	need(locked.Close() == nil)
	need(status == 423)
	checks["active_writer_refused"] = true
	current, e := root.ReadFile(dir + "/nested/Café.bin")
	need(e == nil && sha256.Sum256(current) == sha256.Sum256(large))
	checks["source_binary_preserved"] = true
	f, e := root.Open(dir)
	need(e == nil)
	entries, e := f.ReadDir(-1)
	need(e == nil)
	need(f.Close() == nil)
	for _, entry := range entries {
		need(!strings.HasPrefix(entry.Name(), ".wings-archive-"))
	}
	checks["no_temporary_archives_remain"] = true
}
