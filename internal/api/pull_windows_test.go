package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"wings-windows/internal/node"
)

type pullRoundTrip func(*http.Request) (*http.Response, error)

func (f pullRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func enablePull(a *API, handler pullRoundTrip) {
	a.Manager.Config.AllowRemoteDownloads = true
	a.pullClient = &http.Client{Transport: handler, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}
func pullResponse(data []byte) *http.Response {
	return &http.Response{StatusCode: 200, Header: make(http.Header), ContentLength: int64(len(data)), Body: io.NopCloser(bytes.NewReader(data))}
}
func pullRequest(t *testing.T, a *API, in pullInput) *httptest.ResponseRecorder {
	t.Helper()
	b, e := json.Marshal(in)
	if e != nil {
		t.Fatal(e)
	}
	return request(t, a, "POST", "/api/servers/"+serverID+"/files/pull", string(b), secret)
}
func normalPull() pullInput {
	return pullInput{Root: "/", URL: "https://files.example.test/file.bin", Foreground: true}
}

func TestPullPublicAddressPolicy(t *testing.T) {
	for _, address := range []string{"0.0.0.0", "10.1.2.3", "100.64.1.1", "127.0.0.1", "169.254.169.254", "172.19.183.28", "192.168.1.1", "192.0.0.9", "192.0.2.2", "192.88.99.1", "198.18.0.1", "198.51.100.1", "203.0.113.1", "224.0.0.1", "240.0.0.1", "255.255.255.255", "::", "::1", "fc00::1", "fe80::1", "ff02::1", "::ffff:127.0.0.1", "64:ff9b::a00:1", "2001::1", "2001:db8::1", "2002:a00:1::1", "3fff::1"} {
		t.Run(address, func(t *testing.T) {
			if publicPullAddress(netip.MustParseAddr(address)) {
				t.Fatal("unsafe destination admitted")
			}
		})
	}
	for _, address := range []string{"1.1.1.1", "8.8.8.8", "158.69.220.93", "2606:4700:4700::1111", "2001:4860:4860::8888"} {
		t.Run(address, func(t *testing.T) {
			if !publicPullAddress(netip.MustParseAddr(address)) {
				t.Fatal("public destination refused")
			}
		})
	}
}
func TestPullURLGuards(t *testing.T) {
	for _, raw := range []string{"", "http://example.test/a", "file:///C:/secret", "ftp://example.test/a", "https://u:p@example.test/a", "https://example.test:8443/a", "https://example.test/a#fragment", "https://127.0.0.1/a", "https://[::1]/a", "https://[fe80::1%25eth0]/a", "https://169.254.169.254/metadata", strings.Repeat("x", 8193)} {
		t.Run(fmt.Sprintf("%.45s", raw), func(t *testing.T) {
			if _, e := pullURL(raw); e == nil {
				t.Fatal("URL admitted")
			}
		})
	}
	if _, e := pullURL("https://example.test:443/path?signature=private-value"); e != nil {
		t.Fatal(e)
	}
}
func TestPullDNSPinning(t *testing.T) {
	for _, kind := range []string{"private", "mixed", "empty", "failure", "too-many", "port", "public"} {
		t.Run(kind, func(t *testing.T) {
			called := false
			ips := []netip.Addr{netip.MustParseAddr("1.1.1.1")}
			switch kind {
			case "private":
				ips = []netip.Addr{netip.MustParseAddr("127.0.0.1")}
			case "mixed":
				ips = append(ips, netip.MustParseAddr("10.0.0.1"))
			case "empty":
				ips = nil
			case "too-many":
				for len(ips) < 17 {
					ips = append(ips, ips[0])
				}
			}
			dial := guardedPullDial(func(context.Context, string, string) ([]netip.Addr, error) {
				if kind == "failure" {
					return nil, errors.New("DNS failure")
				}
				return ips, nil
			}, func(_ context.Context, network, address string) (net.Conn, error) {
				called = true
				if address != "1.1.1.1:443" || network != "tcp" {
					t.Fatal("unvalidated address dialed", address)
				}
				a, b := net.Pipe()
				b.Close()
				return a, nil
			})
			address := "files.example.test:443"
			if kind == "port" {
				address = "files.example.test:80"
			}
			conn, e := dial(context.Background(), "tcp", address)
			if conn != nil {
				conn.Close()
			}
			if kind == "public" {
				if e != nil || !called {
					t.Fatal(e)
				}
			} else if e == nil || called {
				t.Fatal("unsafe dial", called, e)
			}
		})
	}
	c := newPullClient()
	tr := c.Transport.(*http.Transport)
	if tr.Proxy != nil || tr.TLSClientConfig.InsecureSkipVerify || !tr.DisableCompression || !tr.DisableKeepAlives || tr.MaxResponseHeaderBytes > 16<<10 {
		t.Fatal("unsafe production transport")
	}
	if c.CheckRedirect(nil, nil) != http.ErrUseLastResponse {
		t.Fatal("redirect followed")
	}
}
func TestPullPanelContractAndBinary(t *testing.T) {
	a, _ := setup(t)
	data := bytes.Repeat([]byte{0, 255, 1}, 3<<20)
	enablePull(a, func(r *http.Request) (*http.Response, error) {
		if r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" || r.Header.Get("Accept-Encoding") != "identity" {
			t.Fatal("credentials forwarded or decompression allowed")
		}
		return pullResponse(data), nil
	})
	putDownload(t, a, "target/retained", []byte("old"))
	raw, e := os.ReadFile("testdata/upstream-pull-request.json")
	if e != nil {
		t.Fatal(e)
	}
	w := request(t, a, "POST", "/api/servers/"+serverID+"/files/pull", string(raw), secret)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	var m fileMetadata
	if e = json.Unmarshal(w.Body.Bytes(), &m); e != nil {
		t.Fatal(e)
	}
	if m.Name != "Café.bin" || m.Size != int64(len(data)) || !m.File || m.Created == "" {
		t.Fatal(m)
	}
	uploadClean(t, a, "target", map[string][]byte{"retained": []byte("old"), "Café.bin": data})
	if w = request(t, a, "GET", "/api/servers/"+serverID+"/files/pull", "", secret); w.Code != 200 || w.Body.String() != "{\"downloads\":[]}\n" {
		t.Fatal(w.Code, w.Body)
	}
}
func TestPullResponseAndPathFailures(t *testing.T) {
	for _, kind := range []string{"redirect", "status", "encoding", "oversize", "short", "read-error", "existing", "header-traversal", "ambiguous-header", "denied", "denied-stage", "missing-parent", "root-traversal", "filename", "private-url"} {
		t.Run(kind, func(t *testing.T) {
			a, _ := setupWithSettings(t, func(s *node.Settings) {
				if kind == "denied" {
					s.Egg.FileDenylist = []string{"file.bin"}
				}
				if kind == "denied-stage" {
					s.Egg.FileDenylist = []string{".wings-pull-*"}
				}
			})
			input := normalPull()
			want := 400
			enablePull(a, func(*http.Request) (*http.Response, error) {
				r := pullResponse([]byte("new"))
				switch kind {
				case "redirect":
					r.StatusCode = 302
					r.Header.Set("Location", "https://127.0.0.1/metadata")
				case "status":
					r.StatusCode = 404
				case "encoding":
					r.Header.Set("Content-Encoding", "gzip")
				case "oversize":
					r.ContentLength = maxPullBytes + 1
				case "short":
					r.ContentLength = 100
				case "read-error":
					r.Body = io.NopCloser(errorPullReader{})
				case "header-traversal":
					r.Header.Set("Content-Disposition", `attachment; filename="../bad"`)
				case "ambiguous-header":
					r.Header["Content-Disposition"] = []string{`attachment; filename="a"`, `attachment; filename="b"`}
				}
				return r, nil
			})
			wantFiles := map[string][]byte{}
			switch kind {
			case "oversize":
				want = 413
			case "existing":
				want = 409
				putDownload(t, a, "file.bin", []byte("old"))
				wantFiles["file.bin"] = []byte("old")
			case "header-traversal", "ambiguous-header":
				want = 422
				input.UseHeader = true
			case "denied", "denied-stage":
				want = 403
			case "missing-parent":
				want = 403
				input.Root = "missing"
			case "root-traversal":
				want = 422
				input.Root = "../private"
			case "filename":
				want = 422
				input.FileName = "NUL.txt"
			case "private-url":
				want = 422
				input.URL = "https://127.0.0.1/private"
			}
			w := pullRequest(t, a, input)
			if w.Code != want {
				t.Fatal(w.Code, w.Body)
			}
			if strings.Contains(w.Body.String(), input.URL) {
				t.Fatal("URL leaked")
			}
			uploadClean(t, a, "", wantFiles)
		})
	}
}

type errorPullReader struct{}

func (errorPullReader) Read(p []byte) (int, error) { return 0, io.ErrUnexpectedEOF }
func TestPullNameEmptyAndUnknownLength(t *testing.T) {
	for _, kind := range []string{"header", "empty", "unknown", "directory-alias"} {
		t.Run(kind, func(t *testing.T) {
			a, _ := setup(t)
			input := normalPull()
			name := "file.bin"
			data := []byte("data")
			if kind == "empty" {
				data = nil
			}
			if kind == "header" {
				input.UseHeader = true
				input.FileName = "fallback"
				name = "Café.bin"
			}
			if kind == "directory-alias" {
				input.Root = ""
				input.Directory = "/"
			}
			enablePull(a, func(*http.Request) (*http.Response, error) {
				r := pullResponse(data)
				if kind == "unknown" {
					r.ContentLength = -1
				}
				if kind == "header" {
					r.Header.Set("Content-Disposition", `attachment; filename*=UTF-8''Caf%C3%A9.bin`)
				}
				return r, nil
			})
			w := pullRequest(t, a, input)
			if w.Code != 200 {
				t.Fatal(w.Code, w.Body)
			}
			uploadClean(t, a, "", map[string][]byte{name: data})
		})
	}
}

type blockingPullBody struct {
	ctx     context.Context
	sent    bool
	started chan struct{}
	once    sync.Once
}

func (b *blockingPullBody) Read(p []byte) (int, error) {
	if !b.sent {
		b.sent = true
		n := copy(p, []byte("partial"))
		return n, nil
	}
	b.once.Do(func() { close(b.started) })
	<-b.ctx.Done()
	return 0, b.ctx.Err()
}
func (*blockingPullBody) Close() error { return nil }
func waitPulls(t *testing.T, a *API) {
	t.Helper()
	done := make(chan struct{})
	go func() { a.pullWG.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("jobs did not stop")
	}
}
func TestPullAsyncProgressCancelAndShutdown(t *testing.T) {
	for _, kind := range []string{"cancel", "shutdown"} {
		t.Run(kind, func(t *testing.T) {
			a, _ := setup(t)
			started := make(chan struct{})
			enablePull(a, func(r *http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: 200, Header: make(http.Header), ContentLength: 100, Body: &blockingPullBody{ctx: r.Context(), started: started}}, nil
			})
			input := normalPull()
			input.Foreground = false
			w := pullRequest(t, a, input)
			if w.Code != 202 {
				t.Fatal(w.Code, w.Body)
			}
			var accepted struct {
				Identifier string `json:"identifier"`
			}
			json.Unmarshal(w.Body.Bytes(), &accepted)
			select {
			case <-started:
			case <-time.After(3 * time.Second):
				t.Fatal("no transfer")
			}
			w = request(t, a, "GET", "/api/servers/"+serverID+"/files/pull", "", secret)
			var list struct {
				Downloads []pullProgress `json:"downloads"`
			}
			if e := json.Unmarshal(w.Body.Bytes(), &list); e != nil || len(list.Downloads) != 1 || list.Downloads[0].Identifier != accepted.Identifier || list.Downloads[0].Progress <= 0 || list.Downloads[0].Progress >= 1 {
				t.Fatal(w.Body, e)
			}
			if kind == "shutdown" {
				a.Close()
			} else {
				w = request(t, a, "DELETE", "/api/servers/"+serverID+"/files/pull/"+accepted.Identifier, "", secret)
				if w.Code != 204 {
					t.Fatal(w.Code)
				}
			}
			waitPulls(t, a)
			uploadClean(t, a, "", nil)
		})
	}
}
func TestPullAdmissionAndScopedCancel(t *testing.T) {
	a, _ := setup(t)
	if w := pullRequest(t, a, normalPull()); w.Code != 501 {
		t.Fatal("default enabled")
	}
	enablePull(a, func(*http.Request) (*http.Response, error) { return pullResponse(nil), nil })
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	foreign := &pullJob{id: "foreign", server: userID, ctx: ctx, cancel: cancel}
	a.pulls[foreign.id] = foreign
	w := request(t, a, "DELETE", "/api/servers/"+serverID+"/files/pull/foreign", "", secret)
	if w.Code != 204 || ctx.Err() != nil {
		t.Fatal("cross-server cancel")
	}
	for i := 0; i < 3; i++ {
		a.pulls[fmt.Sprint(i)] = &pullJob{id: fmt.Sprint(i), server: serverID, cancel: func() {}}
	}
	if w := pullRequest(t, a, normalPull()); w.Code != 503 {
		t.Fatal(w.Code)
	}
	for i := 0; i < 3; i++ {
		delete(a.pulls, fmt.Sprint(i))
	}
	delete(a.pulls, "foreign")
	a.Close()
	if w := pullRequest(t, a, normalPull()); w.Code != 503 {
		t.Fatal(w.Code)
	}
}
func TestPullMidstreamPolicyAndCollision(t *testing.T) {
	for _, kind := range []string{"policy", "collision"} {
		t.Run(kind, func(t *testing.T) {
			var denied atomic.Bool
			a, _ := setupWithSettings(t, func(s *node.Settings) {
				if denied.Load() {
					s.Egg.FileDenylist = []string{"file.bin"}
				}
			})
			data := bytes.Repeat([]byte("x"), 256<<10)
			enablePull(a, func(*http.Request) (*http.Response, error) {
				r := pullResponse(data)
				r.Body = &interruptUpload{ReadCloser: r.Body, after: func() {
					if kind == "policy" {
						denied.Store(true)
						if e := a.Manager.Get(serverID).Sync(context.Background()); e != nil {
							t.Fatal(e)
						}
					} else {
						putDownload(t, a, "file.bin", []byte("late"))
					}
				}}
				return r, nil
			})
			w := pullRequest(t, a, normalPull())
			want := 409
			files := map[string][]byte{"file.bin": []byte("late")}
			if kind == "policy" {
				want = 403
				files = nil
			}
			if w.Code != want {
				t.Fatal(w.Code, w.Body)
			}
			uploadClean(t, a, "", files)
		})
	}
}

func TestPullStreamingBounds(t *testing.T) {
	for _, size := range []int64{maxPullBytes, maxPullBytes + 1} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			a, _ := setup(t)
			enablePull(a, func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: 200, Header: make(http.Header), ContentLength: -1, Body: io.NopCloser(io.LimitReader(zeroUploadReader{}, size))}, nil
			})
			w := pullRequest(t, a, normalPull())
			if size > maxPullBytes {
				if w.Code != 413 {
					t.Fatal(w.Code, w.Body)
				}
				uploadClean(t, a, "", nil)
			} else {
				if w.Code != 200 {
					t.Fatal(w.Code, w.Body)
				}
				f, e := os.Open(a.Manager.Get(serverID).Root + "/file.bin")
				if e != nil {
					t.Fatal(e)
				}
				info, e := f.Stat()
				f.Close()
				if e != nil || info.Size() != size {
					t.Fatal(info, e)
				}
			}
		})
	}
}

func TestPullRequestCancellation(t *testing.T) {
	a, _ := setup(t)
	started := make(chan struct{})
	enablePull(a, func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: make(http.Header), ContentLength: 100, Body: &blockingPullBody{ctx: r.Context(), started: started}}, nil
	})
	b, _ := json.Marshal(normalPull())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r := httptest.NewRequest("POST", "/api/servers/"+serverID+"/files/pull", bytes.NewReader(b)).WithContext(ctx)
	r.Header.Set("Authorization", "Bearer "+secret)
	result := make(chan *httptest.ResponseRecorder, 1)
	go func() { w := httptest.NewRecorder(); a.ServeHTTP(w, r); result <- w }()
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("no read")
	}
	cancel()
	select {
	case w := <-result:
		if w.Code != 408 {
			t.Fatal(w.Code, w.Body)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("request cancellation ignored")
	}
	waitPulls(t, a)
	uploadClean(t, a, "", nil)
}
