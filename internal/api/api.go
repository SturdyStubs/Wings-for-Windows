package api

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
	"wings-windows/internal/config"
	"wings-windows/internal/node"
)

type API struct {
	Manager     *node.Manager
	boot        time.Time
	revoked     sync.Map
	renameLocks sync.Map
	slots       chan struct{}
	done        chan struct{}
	closeOnce   sync.Once
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
	respond(w, status, map[string]string{"error": message, "request_id": w.Header().Get("X-Request-Id")})
}
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)
	d := json.NewDecoder(r.Body)
	if err := d.Decode(v); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			failure(w, 413, "Request body exceeds 64 KiB")
			return false
		}
		failure(w, 400, "Invalid request body")
		return false
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			failure(w, 413, "Request body exceeds 64 KiB")
			return false
		}
		failure(w, 400, "Expected one JSON object")
		return false
	}
	return true
}
func (a *API) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	a.headers(w, r)
	if r.Method == http.MethodOptions {
		respond(w, 204, nil)
		return
	}
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
		w.Header().Set("WWW-Authenticate", "Bearer")
		failure(w, 401, "Unauthorized")
		return
	}
	if r.URL.Path == "/api/system" && r.Method == "GET" {
		a.system(w, r)
		return
	}
	if r.URL.Path == "/api/servers" && r.Method == "GET" {
		result := []any{}
		for _, s := range a.Manager.All() {
			result = append(result, s.Response())
		}
		result = append(result, a.Manager.PendingResponses()...)
		respond(w, 200, result)
		return
	}
	if r.URL.Path == "/api/servers" && r.Method == "POST" {
		var request struct {
			UUID              string `json:"uuid"`
			StartOnCompletion bool   `json:"start_on_completion"`
		}
		if !decode(w, r, &request) {
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		if err := a.Manager.CreatePending(ctx, request.UUID, request.StartOnCompletion); err != nil {
			status, message := 502, "Cannot verify server assignment with panel"
			switch {
			case errors.Is(err, node.ErrCreationUUID), errors.Is(err, node.ErrCreationStart):
				status, message = 422, err.Error()
			case errors.Is(err, node.ErrCreationDisabled):
				status, message = 501, err.Error()
			case errors.Is(err, node.ErrCreationConflict), errors.Is(err, node.ErrManagerClosed):
				status, message = 409, err.Error()
			case errors.Is(err, node.ErrCreationUnassigned):
				status, message = 404, err.Error()
			case errors.Is(err, node.ErrCreationCapacity):
				status, message = 429, err.Error()
			}
			failure(w, status, message)
			return
		}
		respond(w, 202, nil)
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
		if pending, ok := a.Manager.Pending(parts[2]); ok {
			if len(parts) == 3 && r.Method == "GET" {
				respond(w, 200, pending)
			} else {
				failure(w, 409, "Local Windows profile approval required before server operations")
			}
			return
		}
		failure(w, 404, "Unknown local server profile")
		return
	}
	action := strings.Join(parts[3:], "/")
	switch {
	case action == "" && r.Method == "GET":
		respond(w, 200, s.Response())
	case action == "install/reconcile" && r.Method == "POST":
		var request struct {
			AttemptID string `json:"attempt_id"`
			Action    string `json:"action"`
		}
		if !decode(w, r, &request) {
			return
		}
		if !config.UUID.MatchString(request.AttemptID) || (request.Action != "notify" && request.Action != "report_failed") {
			failure(w, 422, "Provide a full attempt_id and action notify or report_failed")
			return
		}
		if err := s.ReconcileInstall(r.Context(), request.AttemptID, request.Action == "report_failed"); err != nil {
			status := 409
			if errors.Is(err, node.ErrInstallPersistDisabled) {
				status = 501
			}
			if errors.Is(err, node.ErrInstallNotification) {
				status = 502
			}
			if errors.Is(err, node.ErrInstallStorage) {
				status = 503
			}
			failure(w, status, err.Error())
			return
		}
		respond(w, 204, nil)
	case (action == "install" || action == "reinstall") && r.Method == "POST":
		if err := s.EnqueueInstall(action == "reinstall"); err != nil {
			status := 409
			if errors.Is(err, node.ErrInstallStorage) {
				status = 503
			} else if errors.Is(err, node.ErrInstallDisabled) {
				status = 501
			} else if errors.Is(err, node.ErrSuspended) {
				status = 400
			}
			failure(w, status, err.Error())
			return
		}
		respond(w, 202, nil)
	case action == "power" && r.Method == "POST":
		var v struct {
			Action      string `json:"action"`
			WaitSeconds int    `json:"wait_seconds"`
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
		if err := s.EnqueuePower(v.Action, v.WaitSeconds); err != nil {
			if errors.Is(err, node.ErrSuspended) {
				failure(w, 400, err.Error())
				return
			}
			if errors.Is(err, node.ErrPowerQueueFull) {
				failure(w, 429, err.Error())
				return
			}
			failure(w, 409, err.Error())
			return
		}
		respond(w, 202, nil)
	case action == "commands" && r.Method == "POST":
		if !s.HasProcess() {
			failure(w, 502, "Cannot send commands to a stopped server instance")
			return
		}
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
		size, _ := strconv.Atoi(r.URL.Query().Get("size"))
		if size <= 0 {
			size = 100
		}
		if size > 100 {
			size = 100
		}
		respond(w, 200, map[string]any{"data": s.Logs(size)})
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
		failure(w, 501, "Unsupported on Windows Native alpha: this operation requires additional implementation")
	}
}
