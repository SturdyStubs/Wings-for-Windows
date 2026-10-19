package api

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"runtime"
	"strings"
	"sync"
	"time"
	"wings-windows/internal/config"
	"wings-windows/internal/node"
)

type API struct {
	Manager   *node.Manager
	boot      time.Time
	revoked   sync.Map
	slots     chan struct{}
	done      chan struct{}
	closeOnce sync.Once
}

func New(m *node.Manager) *API {
	return &API{Manager: m, boot: time.Now().Truncate(time.Second), slots: make(chan struct{}, 256), done: make(chan struct{})}
}
func (a *API) Close() { a.closeOnce.Do(func() { close(a.done) }) }
func respond(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if v != nil {
		json.NewEncoder(w).Encode(v)
	}
}
func failure(w http.ResponseWriter, status int, message string) {
	respond(w, status, map[string]string{"error": message})
}
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)
	d := json.NewDecoder(r.Body)
	if d.Decode(v) != nil {
		failure(w, 422, "Invalid request body")
		return false
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		failure(w, 422, "Expected one JSON object")
		return false
	}
	return true
}
func (a *API) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "no-store")
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) == 4 && parts[0] == "api" && parts[1] == "servers" && parts[3] == "ws" && r.Method == "GET" {
		s := a.Manager.Get(parts[2])
		if s == nil {
			failure(w, 404, "Unknown local server profile")
			return
		}
		a.socket(w, r, s)
		return
	}
	expected := "Bearer " + a.Manager.Config.Token
	if subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte(expected)) != 1 {
		failure(w, 401, "Unauthorized")
		return
	}
	if r.URL.Path == "/api/system" && r.Method == "GET" {
		system := map[string]any{"architecture": runtime.GOARCH, "cpu_threads": runtime.NumCPU(), "kernel_version": "Windows NT", "os": "windows", "type": "windows"}
		v := map[string]any{"architecture": runtime.GOARCH, "cpu_count": runtime.NumCPU(), "kernel_version": "Windows NT", "os": "windows", "version": config.Version, "capabilities": []string{"native_executable", "job_objects", "console", "power", "files_read_write"}}
		if r.URL.Query().Get("v") == "2" {
			v["system"] = system
			v["docker"] = map[string]any{"version": "unavailable: native Windows runtime", "containers": map[string]int{"total": len(a.Manager.All()), "running": 0, "paused": 0, "stopped": 0}, "cgroups": map[string]string{"driver": "none", "version": "none"}, "storage": map[string]string{"driver": "native", "filesystem": "NTFS"}, "runc": map[string]string{"version": "none"}}
		}
		respond(w, 200, v)
		return
	}
	if r.URL.Path == "/api/servers" && r.Method == "GET" {
		result := []any{}
		for _, s := range a.Manager.All() {
			result = append(result, s.Response())
		}
		respond(w, 200, result)
		return
	}
	if r.URL.Path == "/api/deauthorize-user" && r.Method == "POST" {
		var v struct {
			User    string   `json:"user"`
			Servers []string `json:"servers"`
		}
		if !decode(w, r, &v) {
			return
		}
		if !config.UUID.MatchString(v.User) {
			failure(w, 422, "Invalid user UUID")
			return
		}
		if len(v.Servers) == 0 {
			for _, s := range a.Manager.All() {
				v.Servers = append(v.Servers, s.ID())
			}
		}
		for _, id := range v.Servers {
			a.revoked.Store("user:"+strings.ToLower(id)+":"+v.User, time.Now())
		}
		respond(w, 204, nil)
		return
	}
	if len(parts) < 3 || parts[0] != "api" || parts[1] != "servers" {
		failure(w, 501, "This endpoint is not supported by Windows Native alpha")
		return
	}
	s := a.Manager.Get(parts[2])
	if s == nil {
		failure(w, 404, "Unknown local server profile")
		return
	}
	action := strings.Join(parts[3:], "/")
	switch {
	case action == "" && r.Method == "GET":
		respond(w, 200, s.Response())
	case action == "power" && r.Method == "POST":
		var v struct {
			Action string `json:"action"`
		}
		if !decode(w, r, &v) {
			return
		}
		switch v.Action {
		case "start", "stop", "restart", "kill":
		default:
			failure(w, 422, "Invalid power action")
			return
		}
		if err := s.Power(v.Action); err != nil {
			if errors.Is(err, node.ErrSuspended) {
				failure(w, 400, err.Error())
				return
			}
			failure(w, 409, err.Error())
			return
		}
		respond(w, 202, nil)
	case action == "commands" && r.Method == "POST":
		var v struct {
			Commands []string `json:"commands"`
		}
		if !decode(w, r, &v) {
			return
		}
		if len(v.Commands) > 32 {
			failure(w, 422, "Too many commands")
			return
		}
		for _, cmd := range v.Commands {
			if err := s.Command(cmd); err != nil {
				failure(w, 502, err.Error())
				return
			}
		}
		respond(w, 204, nil)
	case action == "logs" && r.Method == "GET":
		respond(w, 200, map[string]any{"data": s.Logs(100)})
	case action == "sync" && r.Method == "POST":
		if err := s.Sync(r.Context()); err != nil {
			failure(w, 502, "Panel synchronization failed")
			return
		}
		respond(w, 204, nil)
	case action == "ws/deny" && r.Method == "POST":
		var v struct {
			JTIs []string `json:"jtis"`
		}
		if !decode(w, r, &v) {
			return
		}
		for _, id := range v.JTIs {
			a.revoked.Store("jti:"+id, time.Now())
		}
		respond(w, 204, nil)
	case strings.HasPrefix(action, "files/"):
		a.files(w, r, s, strings.TrimPrefix(action, "files/"))
	default:
		failure(w, 501, "Unsupported on Windows Native alpha: installers, reinstall, SFTP, downloads, backups, transfers and deletion require additional implementation")
	}
}
