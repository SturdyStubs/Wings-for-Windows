package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/sys/windows"
	"mime"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"wings-windows/internal/node"
)

var downloadSequence atomic.Uint64

func freshDownload(a *API) downloadClaims {
	u := fmt.Sprint(downloadSequence.Add(1))
	return downloadClaims{RegisteredClaims: jwt.RegisteredClaims{Issuer: a.Manager.Config.PanelURL, ID: "jti-" + u, IssuedAt: jwt.NewNumericDate(time.Now()), ExpiresAt: jwt.NewNumericDate(time.Now().Add(10 * time.Minute))}, Scope: "file-download", FilePath: "/data.bin", Server: serverID, User: userID, Unique: "unique-" + u}
}
func signDownload(t *testing.T, c downloadClaims) string {
	t.Helper()
	v, e := jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString([]byte(secret))
	if e != nil {
		t.Fatal(e)
	}
	return v
}
func downloadRequest(t *testing.T, a *API, c downloadClaims) *httptest.ResponseRecorder {
	t.Helper()
	return request(t, a, "GET", "/download/file?token="+url.QueryEscape(signDownload(t, c)), "", "")
}
func putDownload(t *testing.T, a *API, name string, b []byte) {
	t.Helper()
	file := filepath.Join(a.Manager.Get(serverID).Root, filepath.FromSlash(name))
	if e := os.MkdirAll(filepath.Dir(file), 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(file, b, 0600); e != nil {
		t.Fatal(e)
	}
}

func TestDownloadPanelContract(t *testing.T) {
	a, _ := setup(t)
	raw, e := os.ReadFile("testdata/upstream-download-claims.json")
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
	putDownload(t, a, "config/Café.bin", []byte{0, 255, 1, 2, 0, 3})
	w := downloadRequest(t, a, c)
	if w.Code != 200 || !bytes.Equal(w.Body.Bytes(), []byte{0, 255, 1, 2, 0, 3}) {
		t.Fatal(w.Code, w.Body)
	}
	_, params, e := mime.ParseMediaType(w.Header().Get("Content-Disposition"))
	if e != nil || params["filename"] != "Café.bin" {
		t.Fatal("filename", e)
	}
	if w.Header().Get("Content-Type") != "application/octet-stream" || w.Header().Get("Content-Length") != "6" || w.Header().Get("Cache-Control") != "no-store" || w.Header().Get("Referrer-Policy") != "no-referrer" {
		t.Fatal(w.Header())
	}
	if downloadRequest(t, a, c).Code != 404 {
		t.Fatal("replay")
	}
	f, e := os.OpenFile(filepath.Join(a.Manager.Get(serverID).Root, "config", "Café.bin"), os.O_WRONLY, 0)
	if e != nil {
		t.Fatal("source handle leaked", e)
	}
	f.Close()
}

func TestDownloadLargeAndEmpty(t *testing.T) {
	for _, size := range []int{0, 9 << 20} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			a, _ := setup(t)
			b := bytes.Repeat([]byte{0, 255, 7}, size/3)
			b = append(b, make([]byte, size-len(b))...)
			putDownload(t, a, "data.bin", b)
			w := downloadRequest(t, a, freshDownload(a))
			if w.Code != 200 || !bytes.Equal(w.Body.Bytes(), b) {
				t.Fatal(w.Code, len(w.Body.Bytes()))
			}
		})
	}
}

func TestDownloadJWTGuards(t *testing.T) {
	a, _ := setup(t)
	putDownload(t, a, "data.bin", []byte("not-for-invalid-tokens"))
	tests := []struct {
		name   string
		change func(*downloadClaims)
	}{
		{"issuer", func(c *downloadClaims) { c.Issuer = "https://other.invalid" }},
		{"scope", func(c *downloadClaims) { c.Scope = "websocket" }},
		{"missing issued", func(c *downloadClaims) { c.IssuedAt = nil }},
		{"before boot", func(c *downloadClaims) { c.IssuedAt = jwt.NewNumericDate(a.boot.Add(-time.Second)) }},
		{"future issued", func(c *downloadClaims) { c.IssuedAt = jwt.NewNumericDate(time.Now().Add(time.Minute)) }},
		{"missing expiry", func(c *downloadClaims) { c.ExpiresAt = nil }},
		{"expired", func(c *downloadClaims) { c.ExpiresAt = jwt.NewNumericDate(time.Now().Add(-time.Second)) }},
		{"too long lifetime", func(c *downloadClaims) { c.ExpiresAt = jwt.NewNumericDate(time.Now().Add(time.Hour)) }},
		{"not before", func(c *downloadClaims) { c.NotBefore = jwt.NewNumericDate(time.Now().Add(time.Minute)) }},
		{"missing user", func(c *downloadClaims) { c.User = "" }},
		{"invalid server", func(c *downloadClaims) { c.Server = "not-a-uuid" }},
		{"missing unique", func(c *downloadClaims) { c.Unique = "" }},
		{"missing jti", func(c *downloadClaims) { c.ID = "" }},
		{"large unique", func(c *downloadClaims) { c.Unique = strings.Repeat("x", 257) }},
		{"large path", func(c *downloadClaims) { c.FilePath = strings.Repeat("x", 4097) }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := freshDownload(a)
			tc.change(&c)
			w := downloadRequest(t, a, c)
			if w.Code != 401 || strings.Contains(w.Body.String(), "not-for-invalid-tokens") {
				t.Fatal(w.Code)
			}
		})
	}
	c := freshDownload(a)
	bad, e := jwt.NewWithClaims(jwt.SigningMethodHS512, c).SignedString([]byte(secret))
	if e != nil {
		t.Fatal(e)
	}
	if request(t, a, "GET", "/download/file?token="+bad, "", "").Code != 401 {
		t.Fatal("algorithm")
	}
	bad, e = jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString([]byte("wrong-key"))
	if e != nil {
		t.Fatal(e)
	}
	if request(t, a, "GET", "/download/file?token="+bad, "", "").Code != 401 {
		t.Fatal("signature")
	}
}

func TestDownloadRequestGuards(t *testing.T) {
	a, _ := setup(t)
	putDownload(t, a, "data.bin", []byte("data"))
	for _, query := range []string{"", "token=bad", "token=a&token=b", "token=%zz", "token=" + strings.Repeat("x", 8193)} {
		if request(t, a, "GET", "/download/file?"+query, "", secret).Code != 401 {
			t.Fatal("query guard")
		}
	}
	c := freshDownload(a)
	q := "/download/file?token=" + url.QueryEscape(signDownload(t, c))
	for _, method := range []string{"HEAD", "POST"} {
		if request(t, a, method, q, "", secret).Code != 405 {
			t.Fatal(method)
		}
	}
	r := httptest.NewRequest("GET", q, nil)
	r.Header.Set("Range", "bytes=0-1")
	w := httptest.NewRecorder()
	a.ServeHTTP(w, r)
	if w.Code != 416 {
		t.Fatal(w.Code)
	}
	if downloadRequest(t, a, c).Code != 200 {
		t.Fatal("unsupported range consumed token")
	}
	for _, name := range []string{"/", "/../outside", "/a/../data.bin", "/NUL", "/data.bin:stream", "/folder\\data.bin"} {
		c = freshDownload(a)
		c.FilePath = name
		if got := downloadRequest(t, a, c).Code; got != 422 {
			t.Fatal(name, got)
		}
	}
	c = freshDownload(a)
	c.Server = "33333333-3333-4333-8333-333333333333"
	if downloadRequest(t, a, c).Code != 404 {
		t.Fatal("unknown server")
	}
}

func TestDownloadFileScopeAndUnavailable(t *testing.T) {
	a, _ := setup(t)
	putDownload(t, a, "data.bin", []byte("signed"))
	putDownload(t, a, "other.bin", []byte("other"))
	c := freshDownload(a)
	q := "/download/file?token=" + url.QueryEscape(signDownload(t, c)) + "&file=/other.bin"
	w := request(t, a, "GET", q, "", "")
	if w.Code != 200 || w.Body.String() != "signed" {
		t.Fatal("claim path overridden", w.Code)
	}
	c = freshDownload(a)
	c.FilePath = "/missing"
	if downloadRequest(t, a, c).Code != 404 {
		t.Fatal("missing")
	}
	putDownload(t, a, "missing", []byte("now exists"))
	if downloadRequest(t, a, c).Code != 200 {
		t.Fatal("failed preflight consumed link")
	}
	c = freshDownload(a)
	c.FilePath = "/folder"
	if e := os.Mkdir(filepath.Join(a.Manager.Get(serverID).Root, "folder"), 0700); e != nil {
		t.Fatal(e)
	}
	if downloadRequest(t, a, c).Code != 404 {
		t.Fatal("directory")
	}
}

func TestDownloadDenylistAndLinks(t *testing.T) {
	for _, kind := range []string{"ancestor denylist", "file denylist", "source junction", "parent junction", "hardlink"} {
		t.Run(kind, func(t *testing.T) {
			a, _ := setupWithSettings(t, func(s *node.Settings) { s.Egg.FileDenylist = []string{"private/", "*.secret"} })
			root := a.Manager.Get(serverID).Root
			outside := t.TempDir()
			if e := os.WriteFile(filepath.Join(outside, "secret"), []byte("outside-secret-marker"), 0600); e != nil {
				t.Fatal(e)
			}
			c := freshDownload(a)
			switch kind {
			case "ancestor denylist":
				putDownload(t, a, "private/data", []byte("private"))
				c.FilePath = "/private/data"
			case "file denylist":
				putDownload(t, a, "key.secret", []byte("private"))
				c.FilePath = "/key.secret"
			case "source junction":
				if e := createTestLink(outside, filepath.Join(root, "alias")); e != nil {
					t.Fatal(e)
				}
				c.FilePath = "/alias"
			case "parent junction":
				if e := createTestLink(outside, filepath.Join(root, "alias")); e != nil {
					t.Fatal(e)
				}
				c.FilePath = "/alias/secret"
			case "hardlink":
				if e := os.Link(filepath.Join(outside, "secret"), filepath.Join(root, "alias")); e != nil {
					t.Fatal(e)
				}
				c.FilePath = "/alias"
			}
			w := downloadRequest(t, a, c)
			if w.Code != 403 || strings.Contains(w.Body.String(), "outside-secret-marker") {
				t.Fatal(kind, w.Code, w.Body)
			}
		})
	}
}

func TestDownloadLockedAndOversize(t *testing.T) {
	a, _ := setup(t)
	putDownload(t, a, "data.bin", []byte("keep"))
	file := filepath.Join(a.Manager.Get(serverID).Root, "data.bin")
	f, e := os.OpenFile(file, os.O_WRONLY, 0)
	if e != nil {
		t.Fatal(e)
	}
	c := freshDownload(a)
	if downloadRequest(t, a, c).Code != 423 {
		f.Close()
		t.Fatal("writer not excluded")
	}
	f.Close()
	if downloadRequest(t, a, c).Code != 200 {
		t.Fatal("locked token burned")
	}
	f, e = os.Create(filepath.Join(a.Manager.Get(serverID).Root, "large"))
	if e != nil {
		t.Fatal(e)
	}
	var n uint32
	if e = windows.DeviceIoControl(windows.Handle(f.Fd()), windows.FSCTL_SET_SPARSE, nil, 0, nil, 0, &n, nil); e != nil {
		f.Close()
		t.Fatal(e)
	}
	if e = f.Truncate(maxDownloadBytes + 1); e != nil {
		f.Close()
		t.Fatal(e)
	}
	f.Close()
	c = freshDownload(a)
	c.FilePath = "/large"
	if downloadRequest(t, a, c).Code != 413 {
		t.Fatal("source size bound")
	}
}

func TestDownloadConcurrentOneUseAndCapacity(t *testing.T) {
	a, _ := setup(t)
	putDownload(t, a, "data.bin", bytes.Repeat([]byte("x"), 1<<20))
	c := freshDownload(a)
	start := make(chan struct{})
	results := make(chan int, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); <-start; results <- downloadRequest(t, a, c).Code }()
	}
	close(start)
	wg.Wait()
	close(results)
	counts := map[int]int{}
	for status := range results {
		counts[status]++
	}
	if counts[200] != 1 || counts[404] != 1 {
		t.Fatal(counts)
	}
	for i := 0; i < cap(a.downloadSlots); i++ {
		a.downloadSlots <- struct{}{}
	}
	c = freshDownload(a)
	if downloadRequest(t, a, c).Code != 503 {
		t.Fatal("slot bound")
	}
	for i := 0; i < cap(a.downloadSlots); i++ {
		<-a.downloadSlots
	}
	if downloadRequest(t, a, c).Code != 200 {
		t.Fatal("capacity burned token")
	}
}

func TestDownloadReceiptBoundAndExpiryCleanup(t *testing.T) {
	a, _ := setup(t)
	for i := 0; i < maxDownloadReceipts; i++ {
		a.downloadUsed[fmt.Sprint(i)] = time.Now().Add(time.Minute)
	}
	c := freshDownload(a)
	if a.consumeDownload(&c) != 503 {
		t.Fatal("unbounded receipts")
	}
	a.downloadUsed["0"] = time.Now().Add(-time.Second)
	if a.consumeDownload(&c) != 0 || len(a.downloadUsed) != maxDownloadReceipts {
		t.Fatal("expired receipt not reclaimed")
	}
	if a.consumeDownload(&c) != 404 {
		t.Fatal("receipt replay")
	}
}

type interruptDownload struct {
	*httptest.ResponseRecorder
	once      sync.Once
	interrupt func()
}

func (w *interruptDownload) Write(p []byte) (int, error) {
	n, e := w.ResponseRecorder.Write(p)
	w.once.Do(w.interrupt)
	return n, e
}
func TestDownloadMidstreamChecks(t *testing.T) {
	for _, kind := range []string{"user revocation", "jti revocation", "context cancellation", "policy refresh", "shutdown", "token expiry"} {
		t.Run(kind, func(t *testing.T) {
			var denied atomic.Bool
			a, _ := setupWithSettings(t, func(s *node.Settings) {
				if denied.Load() {
					s.Egg.FileDenylist = []string{"data.bin"}
				}
			})
			putDownload(t, a, "data.bin", bytes.Repeat([]byte("s"), 256<<10))
			c := freshDownload(a)
			if kind == "token expiry" {
				c.ExpiresAt = jwt.NewNumericDate(time.Now().Add(2 * time.Second))
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			w := &interruptDownload{ResponseRecorder: httptest.NewRecorder()}
			w.interrupt = func() {
				switch kind {
				case "user revocation":
					a.revoked.Store("user:"+serverID+":"+userID, time.Now())
				case "jti revocation":
					a.revoked.Store("jti:"+c.ID, time.Now())
				case "context cancellation":
					cancel()
				case "policy refresh":
					denied.Store(true)
					if e := a.Manager.Get(serverID).Sync(context.Background()); e != nil {
						t.Fatal(e)
					}
				case "shutdown":
					a.Close()
				case "token expiry":
					time.Sleep(2200 * time.Millisecond)
				}
			}
			r := httptest.NewRequest("GET", "/download/file?token="+url.QueryEscape(signDownload(t, c)), nil).WithContext(ctx)
			var aborted any
			func() { defer func() { aborted = recover() }(); a.ServeHTTP(w, r) }()
			if aborted != http.ErrAbortHandler || w.Code != 200 || w.Body.Len() != 64<<10 {
				t.Fatal("transfer was not truncated safely", aborted, w.Code, w.Body.Len())
			}
			if len(a.downloadSlots) != 0 {
				t.Fatal("slot leaked")
			}
		})
	}
}

func TestDownloadRevocationNormalization(t *testing.T) {
	a, _ := setup(t)
	putDownload(t, a, "data.bin", []byte("keep"))
	user := "abcdefab-abcd-4abc-8abc-abcdefabcdef"
	c := freshDownload(a)
	c.User = user
	ws := Claims{RegisteredClaims: jwt.RegisteredClaims{Issuer: a.Manager.Config.PanelURL, IssuedAt: c.IssuedAt, ExpiresAt: c.ExpiresAt}, Server: serverID, User: user, Scope: "websocket", Permissions: []string{"websocket.connect"}}
	raw, e := jwt.NewWithClaims(jwt.SigningMethodHS256, ws).SignedString([]byte(secret))
	if e != nil {
		t.Fatal(e)
	}
	if _, e = a.parse(raw, serverID); e != nil {
		t.Fatal(e)
	}
	body, _ := json.Marshal(map[string]any{"user": strings.ToUpper(user), "servers": []string{serverID}})
	if request(t, a, "POST", "/api/deauthorize-user", string(body), secret).Code != 204 {
		t.Fatal("revoke")
	}
	if downloadRequest(t, a, c).Code != 401 {
		t.Fatal("mixed-case user revocation bypass")
	}
	c.User = strings.ToUpper(user)
	c.Server = strings.ToUpper(serverID)
	if downloadRequest(t, a, c).Code != 401 {
		t.Fatal("uppercase token revocation bypass")
	}
	if _, e = a.parse(raw, serverID); e == nil {
		t.Fatal("websocket mixed-case revocation bypass")
	}
}

func TestDownloadPinsSourceWhileStreaming(t *testing.T) {
	a, _ := setup(t)
	putDownload(t, a, "data.bin", bytes.Repeat([]byte("z"), 128<<10))
	c := freshDownload(a)
	root := a.Manager.Get(serverID).Root
	w := &interruptDownload{ResponseRecorder: httptest.NewRecorder()}
	w.interrupt = func() {
		f, e := os.OpenFile(filepath.Join(root, "data.bin"), os.O_WRONLY, 0)
		if e == nil {
			f.Close()
			t.Fatal("new source writer accepted")
		}
		if e = os.Rename(filepath.Join(root, "data.bin"), filepath.Join(root, "moved")); e == nil {
			t.Fatal("pinned source moved")
		}
	}
	a.ServeHTTP(w, httptest.NewRequest("GET", "/download/file?token="+url.QueryEscape(signDownload(t, c)), nil))
	if w.Code != 200 || w.Body.Len() != 128<<10 {
		t.Fatal(w.Code, w.Body.Len())
	}
}

type deadlineDownload struct {
	*httptest.ResponseRecorder
	deadlines []time.Time
}

func (w *deadlineDownload) SetWriteDeadline(at time.Time) error {
	w.deadlines = append(w.deadlines, at)
	return nil
}
func TestDownloadWriteDeadlineCappedByToken(t *testing.T) {
	a, _ := setup(t)
	putDownload(t, a, "data.bin", bytes.Repeat([]byte("x"), 128<<10))
	c := freshDownload(a)
	c.ExpiresAt = jwt.NewNumericDate(time.Now().Add(10 * time.Second))
	w := &deadlineDownload{ResponseRecorder: httptest.NewRecorder()}
	a.ServeHTTP(w, httptest.NewRequest("GET", "/download/file?token="+url.QueryEscape(signDownload(t, c)), nil))
	if w.Code != 200 || len(w.deadlines) < 3 {
		t.Fatal(w.Code, len(w.deadlines))
	}
	for _, deadline := range w.deadlines {
		if !deadline.Equal(c.ExpiresAt.Time) {
			t.Fatal("deadline outlives signed expiry")
		}
	}
}

func TestDownloadUniquePerLinkNotJti(t *testing.T) {
	a, _ := setup(t)
	putDownload(t, a, "data.bin", []byte("signed"))
	c := freshDownload(a)
	used := c.Unique
	if downloadRequest(t, a, c).Code != 200 {
		t.Fatal("first link")
	}
	c.Unique += "-second"
	if downloadRequest(t, a, c).Code != 200 {
		t.Fatal("new panel link with same identified-by jti must work")
	}
	c.Unique = used
	c.ID += "-changed"
	if downloadRequest(t, a, c).Code != 404 {
		t.Fatal("used unique id accepted with a different jti")
	}
}
