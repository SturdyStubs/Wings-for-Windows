package api

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"wings-windows/internal/node"
)

// Fixture keys come from upstream server/filesystem/stat.go at 747bc43 and
// the Pterodactyl 2 FilesClient contract, independently of our DTO.
func TestPanelDirectoryListingContract(t *testing.T) {
	a, _ := setup(t)
	root := a.Manager.Get(serverID).Root
	os.WriteFile(filepath.Join(root, "server.cfg"), []byte("server setup"), 0600)
	os.Mkdir(filepath.Join(root, "logs"), 0700)
	fixture, err := os.ReadFile("testdata/upstream-file-list.json")
	if err != nil {
		t.Fatal(err)
	}
	var expected []map[string]any
	if err = json.Unmarshal(fixture, &expected); err != nil {
		t.Fatal(err)
	}
	w := request(t, a, "GET", "/api/servers/"+serverID+"/files/list-directory?directory=/", "", secret)
	if w.Code != 200 {
		t.Fatalf("panel listing path returned %d: %s", w.Code, w.Body)
	}
	var entries []map[string]any
	if err = json.Unmarshal(w.Body.Bytes(), &entries); err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatal(entries)
	}
	for _, entry := range entries {
		for key, value := range expected[0] {
			got, ok := entry[key]
			if !ok || reflect.TypeOf(got) != reflect.TypeOf(value) {
				t.Errorf("metadata %q has wrong/missing type: %#v", key, got)
			}
		}
		for _, key := range []string{"is_file", "is_symlink", "mimetype", "created_at", "modified_at"} {
			if _, ok := entry[key]; ok {
				t.Errorf("nonstandard metadata field %q", key)
			}
		}
		if entry["name"] == "logs" && (entry["directory"] != true || entry["file"] != false || entry["mime"] != "inode/directory") {
			t.Error("wrong directory metadata", entry)
		}
	}
	w = request(t, a, "GET", "/api/servers/"+serverID+"/files/contents?file=/server.cfg&download=1", "", secret)
	if w.Code != 200 || w.Header().Get("Content-Length") != "12" || w.Header().Get("X-Mime-Type") == "" || !strings.Contains(w.Header().Get("Content-Disposition"), "server.cfg") {
		t.Fatal("file content headers", w.Code, w.Header())
	}
}
func TestEggDenylistBlocksReadWriteAndListing(t *testing.T) {
	a, _ := setupWithSettings(t, func(settings *node.Settings) {
		settings.Egg.FileDenylist = []string{"*.secret", "private/", "!public.secret"}
	})
	root := a.Manager.Get(serverID).Root
	os.Mkdir(filepath.Join(root, "private"), 0700)
	for name, body := range map[string]string{"node.secret": "unchanged", "public.secret": "visible", "private/password.cfg": "private"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	base := "/api/servers/" + serverID + "/files/"
	for _, name := range []string{"node.secret", "NODE.SECRET", "private/password.cfg", "PRIVATE/password.cfg"} {
		for _, method := range []string{"GET", "POST"} {
			action := "contents"
			body := ""
			if method == "POST" {
				action = "write"
				body = "overwritten"
			}
			if w := request(t, a, method, base+action+"?file=/"+name, body, secret); w.Code != 403 {
				t.Fatalf("%s %s bypassed denylist: %d %s", method, name, w.Code, w.Body)
			}
		}
	}
	for name, expected := range map[string]string{"node.secret": "unchanged", "private/password.cfg": "private"} {
		if value, _ := os.ReadFile(filepath.Join(root, name)); string(value) != expected {
			t.Fatal("denied file was modified")
		}
	}
	w := request(t, a, "GET", base+"list-directory?directory=/", "", secret)
	if w.Code != 200 || strings.Contains(w.Body.String(), "node.secret") || strings.Contains(w.Body.String(), `"private"`) || !strings.Contains(w.Body.String(), "public.secret") {
		t.Fatal("denylist listing", w.Code, w.Body)
	}
	if w = request(t, a, "GET", base+"list-directory?directory=/private", "", secret); w.Code != 403 {
		t.Fatal("denied directory listed", w.Code)
	}
	if w = request(t, a, "GET", base+"contents?file=/public.secret", "", secret); w.Code != 200 {
		t.Fatal("negated pattern not honored", w.Code)
	}
}
func TestDenylistChecksResolvedJunctionTargets(t *testing.T) {
	a, _ := setupWithSettings(t, func(settings *node.Settings) { settings.Egg.FileDenylist = []string{"private/"} })
	root := a.Manager.Get(serverID).Root
	private := filepath.Join(root, "private")
	os.Mkdir(private, 0700)
	os.WriteFile(filepath.Join(private, "password.cfg"), []byte("unchanged"), 0600)
	if err := createTestLink(private, filepath.Join(root, "alias")); err != nil {
		t.Fatal(err)
	}
	base := "/api/servers/" + serverID + "/files/"
	for _, test := range []struct{ method, route string }{{"GET", "contents?file=/alias/password.cfg"}, {"POST", "write?file=/alias/password.cfg"}, {"POST", "write?file=/alias/new.cfg"}, {"GET", "list-directory?directory=/alias"}} {
		w := request(t, a, test.method, base+test.route, "modified", secret)
		if w.Code != 403 && w.Code != 404 && w.Code != 422 {
			t.Fatalf("resolved path bypass: %s %d %s", test.route, w.Code, w.Body)
		}
	}
	if value, _ := os.ReadFile(filepath.Join(private, "password.cfg")); string(value) != "unchanged" {
		t.Fatal("denied junction target was truncated")
	}
	if _, err := os.Stat(filepath.Join(private, "new.cfg")); !os.IsNotExist(err) {
		t.Fatal("created file through denied junction")
	}
}
func TestDenylistUpdatesOnPanelSync(t *testing.T) {
	enabled := &atomic.Bool{}
	a, _ := setupWithSettings(t, func(settings *node.Settings) {
		if enabled.Load() {
			settings.Egg.FileDenylist = []string{"config.cfg"}
		}
	})
	root := a.Manager.Get(serverID).Root
	os.WriteFile(filepath.Join(root, "config.cfg"), []byte("settings"), 0600)
	route := "/api/servers/" + serverID + "/files/contents?file=/config.cfg"
	if w := request(t, a, "GET", route, "", secret); w.Code != 200 {
		t.Fatal(w.Code)
	}
	enabled.Store(true)
	if w := request(t, a, "POST", "/api/servers/"+serverID+"/sync", "{}", secret); w.Code != 204 {
		t.Fatal(w.Code)
	}
	if w := request(t, a, "GET", route, "", secret); w.Code != 403 {
		t.Fatal("synced denylist not enforced", w.Code)
	}
}
func TestSuspendedPowerResponseMatchesPanelContract(t *testing.T) {
	a, _ := setupWithSettings(t, func(settings *node.Settings) { settings.Suspended = true })
	for _, action := range []string{"start", "restart"} {
		w := request(t, a, "POST", "/api/servers/"+serverID+"/power", `{"action":"`+action+`"}`, secret)
		if w.Code != 400 {
			t.Fatalf("suspended %s: %d", action, w.Code)
		}
	}
}
