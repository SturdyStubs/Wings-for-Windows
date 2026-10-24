// Opt-in real-panel Client API probe. The key is read from stdin, never argv.
package main

import (
	"archive/tar"
	"bufio"
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
)

const demo = "58e8c7f9-4923-43b3-b431-90b2bf4edf4f"

type event struct {
	Event string   `json:"event"`
	Args  []string `json:"args"`
}
type entry struct {
	Attributes struct {
		Name string `json:"name"`
		File bool   `json:"is_file"`
	} `json:"attributes"`
}

func need(ok bool) {
	if !ok {
		panic("client probe failed; sensitive details omitted")
	}
}
func random() string {
	var b [16]byte
	_, e := rand.Read(b[:])
	need(e == nil)
	return hex.EncodeToString(b[:])
}
func main() {
	if len(os.Args) != 4 || os.Args[3] != "--allow-demo-fixture-and-power" {
		fmt.Fprintln(os.Stderr, "arguments: panel-HTTPS-base node-HTTPS-base --allow-demo-fixture-and-power; Client key on stdin")
		os.Exit(2)
	}
	key, e := bufio.NewReader(io.LimitReader(os.Stdin, 1025)).ReadString('\n')
	if e != nil && e != io.EOF {
		os.Exit(2)
	}
	key = strings.TrimSpace(key)
	needKey := strings.HasPrefix(key, "ptlc_") && len(key) < 1024
	checks := map[string]bool{}
	report := map[string]any{"observed_utc": time.Now().UTC().Format(time.RFC3339), "identity_source": "real Client API key and panel-issued signed URLs/WebSocket JWT", "browser_ui_tested": false, "panel_subuser_removal_tested": false, "checks": checks, "passed": false}
	failed := false
	owned := false
	powerTouched := false
	fixture := ""
	archive := ""
	var ws *websocket.Conn
	panel, node := os.Args[1], os.Args[2]
	client := &http.Client{Timeout: 30 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	send := func(method, address, token, contentType string, body io.Reader) (int, []byte) {
		r, e := http.NewRequest(method, address, body)
		need(e == nil)
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		if contentType != "" {
			r.Header.Set("Content-Type", contentType)
		}
		r.Header.Set("Accept", "application/json")
		resp, e := client.Do(r)
		need(e == nil)
		defer resp.Body.Close()
		b, e := io.ReadAll(io.LimitReader(resp.Body, 12<<20))
		need(e == nil)
		return resp.StatusCode, b
	}
	api := func(method, suffix string, body any) (int, []byte) {
		var input io.Reader
		if body != nil {
			b, e := json.Marshal(body)
			need(e == nil)
			input = bytes.NewReader(b)
		}
		return send(method, panel+"/api/client/servers/58e8c7f9"+suffix, key, "application/json", input)
	}
	list := func(dir string) []entry {
		status, b := api("GET", "/files/list?directory="+url.QueryEscape(dir), nil)
		need(status == 200)
		var v struct {
			Data []entry `json:"data"`
		}
		need(json.Unmarshal(b, &v) == nil)
		return v.Data
	}
	jsonPost := func(method, suffix string, v any, want int) []byte {
		status, b := api(method, suffix, v)
		if status != want {
			report["unexpected_status"] = status
			report["expected_status"] = want
		}
		need(status == want)
		return b
	}
	defer func() {
		if recover() != nil {
			failed = true
		}
		// Never recursively delete the fixture root. Remove only known filenames;
		// the operator removes the three empty directories on-node afterward.
		if owned {
			func() {
				defer func() {
					if recover() != nil {
						failed = true
						report["file_cleanup_failed"] = true
					}
				}()
				for dir, names := range map[string][]string{"source": {"note.txt", "renamed.txt", "renamed copy.txt", "payload.bin", "replay.bin"}, "extracted": {"renamed.txt", "plain.tar", "plain.txt"}} {
					if archive != "" {
						names = append(names, archive)
					}
					for _, name := range names {
						jsonPost("POST", "/files/delete", map[string]any{"root": "/" + fixture + "/" + dir, "files": []string{name}}, 204)
					}
					need(len(list("/"+fixture+"/"+dir)) == 0)
				}
				items := list("/" + fixture)
				need(len(items) == 2)
				for _, item := range items {
					need(!item.Attributes.File && (item.Attributes.Name == "source" || item.Attributes.Name == "extracted"))
				}
				checks["known_files_deleted_and_only_empty_owned_directories_remain"] = true
			}()
		}
		if powerTouched {
			func() {
				defer func() {
					if recover() != nil {
						failed = true
						report["power_restoration_failed"] = true
					}
				}()
				jsonPost("POST", "/power", map[string]string{"signal": "start"}, 204)
				checks["final_start_request_accepted"] = true
			}()
		}
		if ws != nil {
			ws.Close()
		}
		report["fixture_name"] = fixture
		report["passed"] = !failed
		report["finished_utc"] = time.Now().UTC().Format(time.RFC3339)
		json.NewEncoder(os.Stdout).Encode(report)
		if failed {
			os.Exit(1)
		}
	}()
	need(needKey)
	for _, base := range []string{panel, node} {
		u, e := url.Parse(base)
		need(e == nil && u.Scheme == "https" && u.Host != "" && u.User == nil && u.Path == "" && u.RawQuery == "" && u.Fragment == "" && (u.Port() == "" || u.Port() == "443"))
	}
	status, b := api("GET", "", nil)
	need(status == 200)
	var server struct {
		Attributes struct {
			UUID string `json:"uuid"`
		} `json:"attributes"`
	}
	need(json.Unmarshal(b, &server) == nil && server.Attributes.UUID == demo)
	checks["client_access_matches_approved_demo"] = true
	status, b = api("GET", "/resources", nil)
	need(status == 200)
	var resources struct {
		Attributes struct {
			State     string         `json:"current_state"`
			Resources map[string]any `json:"resources"`
		} `json:"attributes"`
	}
	need(json.Unmarshal(b, &resources) == nil && resources.Attributes.State == "running")
	_, ok := resources.Attributes.Resources["disk_bytes"]
	need(ok)
	checks["real_panel_resources_contains_disk_bytes"] = true
	b = jsonPost("GET", "/websocket", nil, 200)
	var creds struct {
		Data struct {
			Token  string `json:"token"`
			Socket string `json:"socket"`
		} `json:"data"`
	}
	need(json.Unmarshal(b, &creds) == nil)
	need(trustedNodeURL(creds.Data.Socket, node, "wss", "/api/servers/"+demo+"/ws", false) && creds.Data.Token != "")
	dialer := websocket.Dialer{HandshakeTimeout: 10 * time.Second}
	ws, _, e = dialer.Dial(creds.Data.Socket, http.Header{"Origin": []string{panel}})
	need(e == nil)
	ws.SetReadLimit(1 << 20)
	need(ws.SetWriteDeadline(time.Now().Add(5*time.Second)) == nil)
	need(ws.WriteJSON(event{"auth", []string{creds.Data.Token}}) == nil)
	creds.Data.Token = ""
	readUntil := func(match func(event) bool) event {
		need(ws.SetReadDeadline(time.Now().Add(15*time.Second)) == nil)
		for i := 0; i < 200; i++ {
			var out event
			need(ws.ReadJSON(&out) == nil)
			need(out.Event != "jwt error" && out.Event != "daemon error")
			if match(out) {
				return out
			}
		}
		panic("expected event unavailable")
	}
	readUntil(func(v event) bool { return v.Event == "auth success" })
	checks["actual_panel_websocket_token_authenticates_over_wss"] = true
	writeWS := func(v event) {
		need(ws.SetWriteDeadline(time.Now().Add(5*time.Second)) == nil)
		need(ws.WriteJSON(v) == nil)
	}
	writeWS(event{"send stats", []string{}})
	v := readUntil(func(v event) bool { return v.Event == "stats" })
	need(len(v.Args) == 1)
	var stats struct {
		State string `json:"state"`
		Disk  int64  `json:"disk_bytes"`
	}
	need(json.Unmarshal([]byte(v.Args[0]), &stats) == nil && stats.State == "running" && stats.Disk >= 0)
	checks["panel_authenticated_console_stats_received"] = true
	fixture = "client-probe-" + random()
	for _, v := range list("/") {
		need(v.Attributes.Name != fixture)
	}
	jsonPost("POST", "/files/create-folder", map[string]string{"root": "/", "name": fixture}, 204)
	owned = true
	for _, dir := range []string{"source", "extracted"} {
		jsonPost("POST", "/files/create-folder", map[string]string{"root": "/" + fixture, "name": dir}, 204)
	}
	checks["panel_creates_owned_directories"] = true
	file := "/" + fixture + "/source/note.txt"
	text := "panel client edited fixture\n"
	for _, body := range []string{"initial fixture\n", text} {
		status, _ = send("POST", panel+"/api/client/servers/58e8c7f9/files/write?file="+url.QueryEscape(file), key, "text/plain", strings.NewReader(body))
		need(status == 204)
	}
	status, b = api("GET", "/files/contents?file="+url.QueryEscape(file), nil)
	need(status == 200 && string(b) == text)
	checks["panel_file_create_edit_and_read_round_trip"] = true
	jsonPost("PUT", "/files/rename", map[string]any{"root": "/" + fixture + "/source", "files": []map[string]string{{"from": "note.txt", "to": "renamed.txt"}}}, 204)
	jsonPost("POST", "/files/copy", map[string]string{"location": "/" + fixture + "/source/renamed.txt"}, 204)
	for _, name := range []string{"renamed.txt", "renamed copy.txt"} {
		status, b = api("GET", "/files/contents?file="+url.QueryEscape("/"+fixture+"/source/"+name), nil)
		need(status == 200 && string(b) == text)
	}
	checks["panel_rename_and_copy_preserve_content"] = true
	parseSigned := func(b []byte, want string) string {
		var value struct {
			Attributes struct {
				URL string `json:"url"`
			} `json:"attributes"`
		}
		need(json.Unmarshal(b, &value) == nil)
		need(trustedNodeURL(value.Attributes.URL, node, "https", want, true))
		return value.Attributes.URL
	}
	b = jsonPost("GET", "/files/upload", nil, 200)
	signed := parseSigned(b, "/upload/file")
	u, e := url.Parse(signed)
	need(e == nil)
	q := u.Query()
	q.Set("directory", "/"+fixture+"/source")
	u.RawQuery = q.Encode()
	payload := make([]byte, 64<<10)
	_, e = rand.Read(payload)
	need(e == nil)
	upload := func(name string) int {
		var body bytes.Buffer
		mw := multipart.NewWriter(&body)
		part, e := mw.CreateFormFile("files", name)
		need(e == nil)
		_, e = part.Write(payload)
		need(e == nil && mw.Close() == nil)
		status, _ := send("POST", u.String(), "", mw.FormDataContentType(), &body)
		return status
	}
	need(upload("payload.bin") == 200)
	checks["actual_panel_signed_upload_accepts_binary"] = true
	status = upload("replay.bin")
	need(status >= 400 && status < 500)
	report["upload_replay_status"] = status
	checks["actual_panel_upload_token_replay_refused"] = true
	b = jsonPost("GET", "/files/download?file="+url.QueryEscape("/"+fixture+"/source/payload.bin"), nil, 200)
	signed = parseSigned(b, "/download/file")
	status, b = send("GET", signed, "", "", nil)
	need(status == 200 && sha256.Sum256(b) == sha256.Sum256(payload))
	checks["actual_panel_signed_download_binary_integrity"] = true
	status, _ = send("GET", signed, "", "", nil)
	need(status >= 400 && status < 500)
	report["download_replay_status"] = status
	checks["actual_panel_download_token_replay_refused"] = true
	b = jsonPost("POST", "/files/compress", map[string]any{"root": "/" + fixture + "/source", "files": []string{"renamed.txt"}}, 200)
	var metadata entry
	need(json.Unmarshal(b, &metadata) == nil)
	need(regexp.MustCompile(`^archive-[0-9TZ-]+-[0-9a-f]{12}\.tar\.gz$`).MatchString(metadata.Attributes.Name))
	archive = metadata.Attributes.Name
	jsonPost("PUT", "/files/rename", map[string]any{"root": "/" + fixture, "files": []map[string]string{{"from": "source/" + archive, "to": "extracted/" + archive}}}, 204)
	jsonPost("POST", "/files/decompress", map[string]string{"root": "/" + fixture + "/extracted", "file": archive}, 204)
	status, b = api("GET", "/files/contents?file="+url.QueryEscape("/"+fixture+"/extracted/renamed.txt"), nil)
	need(status == 200 && string(b) == text)
	checks["panel_compress_move_extract_round_trip"] = true
	// Exercise the plain-TAR format through the real panel-issued upload link.
	var tarBody bytes.Buffer
	tw := tar.NewWriter(&tarBody)
	need(tw.WriteHeader(&tar.Header{Name: "plain.txt", Size: int64(len(text)), Mode: 0600}) == nil)
	_, e = tw.Write([]byte(text))
	need(e == nil && tw.Close() == nil)
	b = jsonPost("GET", "/files/upload", nil, 200)
	u, e = url.Parse(parseSigned(b, "/upload/file"))
	need(e == nil)
	q = u.Query()
	q.Set("directory", "/"+fixture+"/extracted")
	u.RawQuery = q.Encode()
	payload = tarBody.Bytes()
	need(upload("plain.tar") == 200)
	jsonPost("POST", "/files/decompress", map[string]string{"root": "/" + fixture + "/extracted", "file": "plain.tar"}, 204)
	status, b = api("GET", "/files/contents?file="+url.QueryEscape("/"+fixture+"/extracted/plain.txt"), nil)
	need(status == 200 && string(b) == text)
	checks["panel_signed_upload_and_plain_tar_extract_round_trip"] = true
	powerTouched = true
	jsonPost("POST", "/power", map[string]string{"signal": "stop"}, 204)
	readUntil(func(v event) bool { return v.Event == "status" && len(v.Args) == 1 && v.Args[0] == "offline" })
	checks["client_power_stop_observed_over_wss"] = true
	jsonPost("POST", "/power", map[string]string{"signal": "start"}, 204)
	readUntil(func(v event) bool { return v.Event == "status" && len(v.Args) == 1 && v.Args[0] == "running" })
	checks["client_power_start_observed_over_wss"] = true
	for _, mode := range []string{"client", "websocket"} {
		command := "real-panel-" + random()
		if mode == "client" {
			jsonPost("POST", "/command", map[string]string{"command": command}, 204)
		} else {
			writeWS(event{"send command", []string{command}})
		}
		readUntil(func(v event) bool {
			return v.Event == "console output" && strings.Contains(strings.Join(v.Args, "\n"), "ECHO:"+command)
		})
		checks[mode+"_command_echo_over_authenticated_wss"] = true
	}
	report["signed_binary_bytes"] = 64 << 10
}

// The panel includes :443 explicitly; both forms identify the same TLS origin.
func trustedNodeURL(address, node, scheme, expectedPath string, signed bool) bool {
	u, e := url.Parse(address)
	if e != nil {
		return false
	}
	n, e := url.Parse(node)
	if e != nil {
		return false
	}
	if u.Scheme != scheme || !strings.EqualFold(u.Hostname(), n.Hostname()) || u.Hostname() == "" || u.User != nil || u.Fragment != "" || u.Path != expectedPath || u.RawPath != "" || (u.Port() != "" && u.Port() != "443") {
		return false
	}
	if signed {
		q, e := url.ParseQuery(u.RawQuery)
		return e == nil && len(q) == 1 && len(q["token"]) == 1 && q.Get("token") != ""
	}
	return u.RawQuery == ""
}
