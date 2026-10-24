// Opt-in extraction probe: node credentials stay on the node.
package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/rand"
	"crypto/sha256"
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
		panic("extraction probe failed; credential-bearing details omitted")
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
	Data     string                     `json:"data_directory"`
	Profiles map[string]json.RawMessage `json:"profiles"`
}

func main() {
	if len(os.Args) != 5 || os.Args[4] != "--allow-owned-fixture" {
		fmt.Fprintln(os.Stderr, "arguments: native-config public-HTTPS-base disposable-demo-UUID --allow-owned-fixture")
		os.Exit(2)
	}
	checks := map[string]bool{}
	report := map[string]any{"observed_utc": time.Now().UTC().Format(time.RFC3339), "identity_source": "operator master-authenticated HTTPS requests; not panel browser actions", "browser_extract_tested": false, "checks": checks, "passed": false}
	var root *os.Root
	dir := ""
	owned := false
	failed := false
	var archive string
	defer func() {
		if recover() != nil {
			failed = true
		}
		if root != nil {
			if owned {
				files := []string{"target/tree/Café.bin", "target/empty.bin", "target/unsafe.tar.gz", "target/checksum.tar.gz", "target/link.tar.gz", "target/safe.txt", "target/rejected-link", "source/tree/Café.bin", "source/empty.bin", "sentinel", "escape.bin"}
				if archive != "" {
					files = append(files, "target/"+archive, "source/"+archive)
				}
				for _, name := range files {
					if e := root.Remove(dir + "/" + name); e != nil && !os.IsNotExist(e) {
						failed = true
					}
				}
				for _, name := range []string{"target/zip", "target/tree", "target/empty-dir", "target", "source/tree", "source/empty-dir", "source"} {
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
	dir = "extract-probe-" + nonce()
	need(root.Mkdir(dir, 0700) == nil)
	owned = true
	for _, name := range []string{"source", "source/tree", "source/empty-dir", "target"} {
		need(root.Mkdir(dir+"/"+name, 0700) == nil)
	}
	put := func(name string, data []byte) {
		f, e := root.OpenFile(dir+"/"+name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		need(e == nil)
		_, e = f.Write(data)
		need(e == nil)
		need(f.Close() == nil)
	}
	large := make([]byte, 9<<20)
	_, e = rand.Read(large)
	need(e == nil)
	put("source/tree/Café.bin", large)
	put("source/empty.bin", nil)
	put("sentinel", []byte("retained"))
	client := &http.Client{Timeout: 75 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	send := func(action string, body any) (int, []byte) {
		b, e := json.Marshal(body)
		need(e == nil)
		r, e := http.NewRequest("POST", base+"/api/servers/"+id+"/files/"+action, bytes.NewReader(b))
		need(e == nil)
		r.Header.Set("Authorization", "Bearer "+cfg.Token)
		r.Header.Set("Content-Type", "application/json")
		resp, e := client.Do(r)
		need(e == nil)
		defer resp.Body.Close()
		b, e = io.ReadAll(io.LimitReader(resp.Body, 8192))
		need(e == nil)
		return resp.StatusCode, b
	}
	status, b := send("compress", map[string]any{"root": "/" + dir + "/source", "files": []string{"tree", "empty.bin", "empty-dir"}})
	need(status == 200)
	var metadata struct {
		Name string `json:"name"`
		Size int64  `json:"size"`
	}
	need(json.Unmarshal(b, &metadata) == nil)
	need(regexp.MustCompile(`^archive-[0-9TZ-]+-[0-9a-f]{12}\.tar\.gz$`).MatchString(metadata.Name))
	archive = metadata.Name
	checks["public_archive_creation_regression"] = true
	need(root.Rename(dir+"/source/"+archive, dir+"/target/"+archive) == nil)
	extract := func(name string) int {
		status, _ := send("decompress", map[string]string{"root": "/" + dir + "/target", "file": name})
		return status
	}
	need(extract(archive) == 204)
	checks["public_extraction_status"] = true
	current, e := root.ReadFile(dir + "/target/tree/Café.bin")
	need(e == nil && sha256.Sum256(current) == sha256.Sum256(large))
	checks["nine_mib_unicode_binary_integrity"] = true
	current, e = root.ReadFile(dir + "/target/empty.bin")
	need(e == nil && len(current) == 0)
	f, e := root.Open(dir + "/target/empty-dir")
	need(e == nil)
	info, e := f.Stat()
	need(e == nil && info.IsDir())
	need(f.Close() == nil)
	checks["empty_file_directory_and_implicit_parents"] = true
	need(extract(archive) == 409)
	current, e = root.ReadFile(dir + "/target/tree/Café.bin")
	need(e == nil && sha256.Sum256(current) == sha256.Sum256(large))
	checks["repeat_extraction_preserves_destinations"] = true
	pack := func(kind string) []byte {
		var body bytes.Buffer
		z := gzip.NewWriter(&body)
		tw := tar.NewWriter(z)
		data := []byte("never publish")
		need(tw.WriteHeader(&tar.Header{Name: "safe.txt", Size: int64(len(data)), Mode: 07777}) == nil)
		_, e := tw.Write(data)
		need(e == nil)
		if kind == "traversal" {
			need(tw.WriteHeader(&tar.Header{Name: "../escape.bin", Size: int64(len(data)), Mode: 07777}) == nil)
			_, e = tw.Write(data)
			need(e == nil)
		}
		if kind == "link" {
			need(tw.WriteHeader(&tar.Header{Name: "rejected-link", Typeflag: tar.TypeSymlink, Linkname: "../sentinel", Mode: 07777}) == nil)
		}
		need(tw.Close() == nil)
		need(z.Close() == nil)
		b := body.Bytes()
		if kind == "checksum" {
			b[len(b)-8] ^= 1
		}
		return b
	}
	for _, tc := range []struct {
		kind, name string
		status     int
	}{{"traversal", "unsafe.tar.gz", 422}, {"link", "link.tar.gz", 422}, {"checksum", "checksum.tar.gz", 400}} {
		put("target/"+tc.name, pack(tc.kind))
		need(extract(tc.name) == tc.status)
		_, e = root.Lstat(dir + "/target/safe.txt")
		need(os.IsNotExist(e))
		checks[tc.kind+"_refused_before_publication"] = true
	}
	// ZIP is independently produced, with Store/Deflate, Unicode and a comment.
	var zipBody bytes.Buffer
	zw := zip.NewWriter(&zipBody)
	need(zw.SetComment("live Windows ZIP fixture") == nil)
	zf, e := zw.CreateHeader(&zip.FileHeader{Name: "zip/Caf?.bin", Method: zip.Deflate})
	need(e == nil)
	_, e = zf.Write(large)
	need(e == nil)
	need(zw.Close() == nil)
	put("target/bundle.zip", zipBody.Bytes())
	need(extract("bundle.zip") == 204)
	current, e = root.ReadFile(dir + "/target/zip/Caf?.bin")
	need(e == nil && sha256.Sum256(current) == sha256.Sum256(large))
	checks["zip_nine_mib_unicode_binary_integrity"] = true
	need(extract("bundle.zip") == 409)
	checks["zip_repeat_preserves_existing_data"] = true
	var badBody bytes.Buffer
	badZIP := zip.NewWriter(&badBody)
	for _, name := range []string{"safe.txt", "../escape.bin"} {
		f, e := badZIP.Create(name)
		need(e == nil)
		_, e = f.Write([]byte("never publish"))
		need(e == nil)
	}
	need(badZIP.Close() == nil)
	put("target/bad.zip", badBody.Bytes())
	need(extract("bad.zip") == 422)
	_, e = root.Lstat(dir + "/target/safe.txt")
	need(os.IsNotExist(e))
	checks["zip_traversal_refused_before_publication"] = true
	_, e = root.Lstat(dir + "/escape.bin")
	need(os.IsNotExist(e))
	current, e = root.ReadFile(dir + "/sentinel")
	need(e == nil && string(current) == "retained")
	checks["outside_destination_sentinel_preserved"] = true
	locked, e := root.OpenFile(dir+"/target/"+archive, os.O_WRONLY, 0)
	need(e == nil)
	status = extract(archive)
	need(locked.Close() == nil)
	need(status == 423)
	checks["conflicting_archive_writer_refused"] = true
	current, e = root.ReadFile(dir + "/source/tree/Café.bin")
	need(e == nil && sha256.Sum256(current) == sha256.Sum256(large))
	checks["original_source_binary_preserved"] = true
	f, e = root.Open(dir + "/target")
	need(e == nil)
	list, e := f.ReadDir(-1)
	need(e == nil)
	need(f.Close() == nil)
	for _, item := range list {
		need(!strings.HasPrefix(item.Name(), ".wings-extract-"))
	}
	checks["no_temporary_extraction_files_remain"] = true
	report["source_bytes"] = len(large)
	report["archive_bytes"] = metadata.Size
}
