package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/golang-jwt/jwt/v5"
	"io"
	"mime/multipart"
	"net/http/httptest"
	"net/textproto"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
	"wings-windows/internal/node"
)

type uploadEntry struct {
	name string
	data []byte
}

func uploadMultipart(t *testing.T, entries ...uploadEntry) ([]byte, string) {
	t.Helper()
	var b bytes.Buffer
	m := multipart.NewWriter(&b)
	for _, e := range entries {
		h := textproto.MIMEHeader{}
		h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="files"; filename=%q`, e.name))
		p, err := m.CreatePart(h)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = p.Write(e.data); err != nil {
			t.Fatal(err)
		}
	}
	if err := m.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes(), m.FormDataContentType()
}
func freshUpload(a *API) downloadClaims {
	c := freshDownload(a)
	c.Scope = "file-upload"
	c.FilePath = ""
	return c
}
func uploadRequest(t *testing.T, a *API, c downloadClaims, dir string, b []byte, ct string, edit func(*httptest.ResponseRecorder, *io.ReadCloser)) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest("POST", "/upload/file?token="+url.QueryEscape(signDownload(t, c))+"&directory="+url.QueryEscape(dir), bytes.NewReader(b))
	r.Header.Set("Content-Type", ct)
	w := httptest.NewRecorder()
	if edit != nil {
		edit(w, &r.Body)
	}
	a.ServeHTTP(w, r)
	return w
}
func uploadClean(t *testing.T, a *API, dir string, want map[string][]byte) {
	t.Helper()
	root := filepath.Join(a.Manager.Get(serverID).Root, filepath.FromSlash(dir))
	entries, e := os.ReadDir(root)
	if e != nil {
		t.Fatal(e)
	}
	if len(entries) != len(want) {
		t.Fatalf("unexpected files: %v", entries)
	}
	for _, entry := range entries {
		expected, ok := want[entry.Name()]
		if !ok {
			t.Fatal("unexpected staging or published file", entry.Name())
		}
		b, e := os.ReadFile(filepath.Join(root, entry.Name()))
		if e != nil || !bytes.Equal(b, expected) {
			t.Fatal("content mismatch", entry.Name(), e)
		}
	}
}
func TestUploadPanelContract(t *testing.T) {
	a, _ := setup(t)
	raw, e := os.ReadFile("testdata/upstream-upload-claims.json")
	if e != nil {
		t.Fatal(e)
	}
	var c downloadClaims
	if e = json.Unmarshal(raw, &c); e != nil {
		t.Fatal(e)
	}
	c.Issuer = a.Manager.Config.PanelURL
	c.IssuedAt = jwt.NewNumericDate(time.Now())
	c.NotBefore = jwt.NewNumericDate(time.Now().Add(-5 * time.Minute))
	c.ExpiresAt = jwt.NewNumericDate(time.Now().Add(15 * time.Minute))
	putDownload(t, a, "config/retained", []byte("old"))
	data := []byte{0, 255, 1, 2, 0, 3}
	b, ct := uploadMultipart(t, uploadEntry{"Café.bin", data}, uploadEntry{"empty", nil})
	w := uploadRequest(t, a, c, "/config/", b, ct, nil)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	if w.Header().Get("Cache-Control") != "no-store" || w.Header().Get("Referrer-Policy") != "no-referrer" {
		t.Fatal(w.Header())
	}
	uploadClean(t, a, "config", map[string][]byte{"retained": []byte("old"), "Café.bin": data, "empty": nil})
	if w = uploadRequest(t, a, c, "/", b, ct, nil); w.Code != 404 {
		t.Fatal("replay", w.Code)
	}
}
func TestUploadLargeStreamAndLimit(t *testing.T) {
	for _, size := range []int{9 << 20, int(maxUploadBytes), int(maxUploadBytes) + 1} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			a, _ := setup(t)
			// Streaming request avoids allocating the upload-size limit in memory.
			pr, pw := io.Pipe()
			m := multipart.NewWriter(pw)
			r := httptest.NewRequest("POST", "/upload/file?token="+url.QueryEscape(signDownload(t, freshUpload(a))), pr)
			r.Header.Set("Content-Type", m.FormDataContentType())
			done := make(chan struct{})
			go func() {
				defer close(done)
				defer pw.Close()
				p, e := m.CreateFormFile("files", "big.bin")
				if e == nil {
					_, e = io.CopyN(p, zeroUploadReader{}, int64(size))
				}
				if e == nil {
					m.Close()
				}
			}()
			w := httptest.NewRecorder()
			a.ServeHTTP(w, r)
			pr.Close()
			<-done
			if size > int(maxUploadBytes) {
				if w.Code != 413 {
					t.Fatal(w.Code, w.Body)
				}
				uploadClean(t, a, "", nil)
			} else {
				if w.Code != 200 {
					t.Fatal(w.Code, w.Body)
				}
				info, e := os.Stat(filepath.Join(a.Manager.Get(serverID).Root, "big.bin"))
				if e != nil || info.Size() != int64(size) {
					t.Fatal(info, e)
				}
			}
		})
	}
}

type zeroUploadReader struct{}

func (zeroUploadReader) Read(p []byte) (int, error) { clear(p); return len(p), nil }

func TestUploadJWTGuards(t *testing.T) {
	a, _ := setup(t)
	b, ct := uploadMultipart(t, uploadEntry{"data", []byte("private")})
	for _, tc := range []struct {
		name   string
		change func(*downloadClaims)
	}{
		{"issuer", func(c *downloadClaims) { c.Issuer = "https://other.invalid" }},
		{"scope", func(c *downloadClaims) { c.Scope = "file-download" }},
		{"missing-user", func(c *downloadClaims) { c.User = "" }},
		{"missing-unique", func(c *downloadClaims) { c.Unique = "" }},
		{"missing-jti", func(c *downloadClaims) { c.ID = "" }},
		{"expired", func(c *downloadClaims) { c.ExpiresAt = jwt.NewNumericDate(time.Now().Add(-time.Second)) }},
		{"before-boot", func(c *downloadClaims) { c.IssuedAt = jwt.NewNumericDate(a.boot.Add(-time.Second)) }},
		{"future-iat", func(c *downloadClaims) { c.IssuedAt = jwt.NewNumericDate(time.Now().Add(time.Minute)) }},
		{"long-lived", func(c *downloadClaims) { c.ExpiresAt = jwt.NewNumericDate(time.Now().Add(time.Hour)) }},
		{"revoked-user", func(c *downloadClaims) { a.revoked.Store("user:"+c.Server+":"+c.User, time.Now()) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a.revoked.Delete("user:" + serverID + ":" + userID)
			c := freshUpload(a)
			tc.change(&c)
			if w := uploadRequest(t, a, c, "/", b, ct, nil); w.Code != 401 {
				t.Fatal(w.Code, w.Body)
			}
		})
	}
	uploadClean(t, a, "", nil)
}
func TestUploadFilenameGuards(t *testing.T) {
	for _, name := range []string{"../escape", "a/b", `a\b`, "C:stream", "NUL.txt", "COM¹.txt", "trailing.", "trailing ", ".", "..", "", strings.Repeat("a", 256)} {
		t.Run(fmt.Sprintf("%q", name), func(t *testing.T) {
			a, _ := setup(t)
			b, ct := uploadMultipart(t, uploadEntry{name, []byte("x")})
			w := uploadRequest(t, a, freshUpload(a), "/", b, ct, nil)
			if w.Code != 422 {
				t.Fatal(w.Code, w.Body)
			}
			uploadClean(t, a, "", nil)
		})
	}
}
func TestUploadPreflightAndCleanup(t *testing.T) {
	for _, kind := range []string{"existing", "duplicate-case", "truncated", "wrong-field", "no-files", "bad-content-type", "too-many", "headers", "trailing-body", "declared-large", "directory-traversal", "missing-parent", "unknown-server", "shutdown", "restricted"} {
		t.Run(kind, func(t *testing.T) {
			a, _ := setupWithSettings(t, func(s *node.Settings) {
				if kind == "restricted" {
					s.Egg.FileDenylist = []string{"*.secret"}
				}
			})
			c := freshUpload(a)
			entries := []uploadEntry{{"first.bin", []byte("new")}}
			want := map[string][]byte{}
			dir := "/"
			expected := 400
			switch kind {
			case "existing":
				putDownload(t, a, "first.bin", []byte("old"))
				want["first.bin"] = []byte("old")
				expected = 409
			case "duplicate-case":
				entries = append(entries, uploadEntry{"FIRST.bin", []byte("x")})
				expected = 409
			case "too-many":
				for i := 0; i < 16; i++ {
					entries = append(entries, uploadEntry{fmt.Sprint(i), nil})
				}
				expected = 413
			case "directory-traversal":
				dir = "../escape"
				expected = 422
			case "missing-parent":
				dir = "missing"
				expected = 403
			case "unknown-server":
				c.Server = userID
				expected = 404
			case "shutdown":
				a.Close()
				expected = 503
			case "restricted":
				entries[0].name = "file.secret"
				expected = 403
			case "wrong-field":
				expected = 422
			case "no-files":
				entries = nil
			case "headers":
				expected = 400
			case "trailing-body", "declared-large":
				expected = 413
			}
			b, ct := uploadMultipart(t, entries...)
			switch kind {
			case "truncated":
				b = b[:len(b)-12]
			case "wrong-field":
				b = bytes.ReplaceAll(b, []byte(`name="files"`), []byte(`name="other"`))
			case "bad-content-type":
				ct = "application/octet-stream"
			case "headers":
				b = bytes.ReplaceAll(b, []byte("Content-Disposition:"), []byte("X-Large: "+strings.Repeat("x", 8193)+"\r\nContent-Disposition:"))
			case "trailing-body":
				b = append(b, bytes.Repeat([]byte("x"), int(maxUploadBytes+uploadOverhead))...)
			}
			if kind == "declared-large" {
				r := httptest.NewRequest("POST", "/upload/file?token="+url.QueryEscape(signDownload(t, c)), bytes.NewReader(b))
				r.ContentLength = maxUploadBytes + uploadOverhead + 1
				r.Header.Set("Content-Type", ct)
				w := httptest.NewRecorder()
				a.ServeHTTP(w, r)
				if w.Code != expected {
					t.Fatal(w.Code, w.Body)
				}
			} else {
				w := uploadRequest(t, a, c, dir, b, ct, nil)
				if w.Code != expected {
					t.Fatal(w.Code, w.Body)
				}
			}
			uploadClean(t, a, "", want)
		})
	}
}

type interruptUpload struct {
	io.ReadCloser
	once  sync.Once
	after func()
	read  int
}

func (r *interruptUpload) Read(p []byte) (int, error) {
	if len(p) > 2048 {
		p = p[:2048]
	}
	n, e := r.ReadCloser.Read(p)
	r.read += n
	if r.read >= 64<<10 {
		r.once.Do(r.after)
	}
	return n, e
}
func TestUploadMidstreamGuards(t *testing.T) {
	for _, kind := range []string{"cancel", "revocation", "late-collision", "shutdown"} {
		t.Run(kind, func(t *testing.T) {
			a, _ := setup(t)
			c := freshUpload(a)
			b, ct := uploadMultipart(t, uploadEntry{"data.bin", bytes.Repeat([]byte("x"), 256<<10)})
			r := httptest.NewRequest("POST", "/upload/file?token="+url.QueryEscape(signDownload(t, c)), bytes.NewReader(b))
			r.Header.Set("Content-Type", ct)
			ctx, cancel := context.WithCancel(r.Context())
			defer cancel()
			r = r.WithContext(ctx)
			want := map[string][]byte{}
			status := 401
			r.Body = &interruptUpload{ReadCloser: r.Body, after: func() {
				entries, e := os.ReadDir(a.Manager.Get(serverID).Root)
				if e != nil || len(entries) != 1 || !strings.HasPrefix(entries[0].Name(), ".wings-upload-") {
					t.Fatal("interruption must occur after staging starts", e, entries)
				}
				switch kind {
				case "cancel":
					cancel()
				case "revocation":
					a.revoked.Store("jti:"+c.ID, time.Now())
				case "late-collision":
					putDownload(t, a, "data.bin", []byte("late"))
				case "shutdown":
					a.Close()
				}
			}}
			if kind == "cancel" {
				status = 408
			}
			if kind == "shutdown" {
				status = 503
			}
			if kind == "late-collision" {
				status = 409
				want["data.bin"] = []byte("late")
			}
			w := httptest.NewRecorder()
			a.ServeHTTP(w, r)
			if w.Code != status {
				t.Fatal(w.Code, w.Body)
			}
			uploadClean(t, a, "", want)
		})
	}
}

func TestUploadReparseParentRefused(t *testing.T) {
	a, _ := setup(t)
	outside := t.TempDir()
	if e := os.WriteFile(filepath.Join(outside, "retained"), []byte("old"), 0600); e != nil {
		t.Fatal(e)
	}
	if e := createTestLink(outside, filepath.Join(a.Manager.Get(serverID).Root, "linked")); e != nil {
		t.Fatal(e)
	}
	b, ct := uploadMultipart(t, uploadEntry{"new.bin", []byte("new")})
	w := uploadRequest(t, a, freshUpload(a), "linked", b, ct, nil)
	if w.Code != 403 {
		t.Fatal(w.Code, w.Body)
	}
	entries, e := os.ReadDir(outside)
	if e != nil || len(entries) != 1 || entries[0].Name() != "retained" {
		t.Fatal("outside changed", e, entries)
	}
}
func TestUploadConcurrentAndSharedReceipts(t *testing.T) {
	a, _ := setup(t)
	b, ct := uploadMultipart(t, uploadEntry{"data.bin", []byte("x")})
	c := freshUpload(a)
	start := make(chan struct{})
	results := make(chan int, 2)
	for i := 0; i < 2; i++ {
		go func() { <-start; results <- uploadRequest(t, a, c, "/", b, ct, nil).Code }()
	}
	close(start)
	counts := map[int]int{}
	counts[<-results]++
	counts[<-results]++
	if counts[200] != 1 || counts[404] != 1 {
		t.Fatal(counts)
	}
	c.Scope = "file-download"
	c.FilePath = "/data.bin"
	if w := downloadRequest(t, a, c); w.Code != 404 {
		t.Fatal("cross-scope receipt", w.Code)
	}
	for i := 0; i < cap(a.uploadSlots); i++ {
		a.uploadSlots <- struct{}{}
	}
	c = freshUpload(a)
	if w := uploadRequest(t, a, c, "/", b, ct, nil); w.Code != 503 {
		t.Fatal(w.Code)
	}
	for i := 0; i < cap(a.uploadSlots); i++ {
		<-a.uploadSlots
	}
	b, ct = uploadMultipart(t, uploadEntry{"second.bin", nil})
	if w := uploadRequest(t, a, c, "/", b, ct, nil); w.Code != 200 {
		t.Fatal("capacity burned token", w.Code, w.Body)
	}
}
