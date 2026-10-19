package api

import (
	"errors"
	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
	"net/http"
	"net/url"
	"slices"
	"time"
	"wings-windows/internal/node"
)

type Claims struct {
	jwt.RegisteredClaims
	Server      string   `json:"server_uuid"`
	User        string   `json:"user_uuid"`
	Scope       string   `json:"scope"`
	Permissions []string `json:"permissions"`
}

func (c *Claims) can(permission string) bool {
	return slices.Contains(c.Permissions, permission) || slices.Contains(c.Permissions, "*")
}
func (a *API) valid(c *Claims, id string) bool {
	if c == nil || c.Server != id || c.User == "" || c.Scope != "websocket" || c.IssuedAt == nil || c.ExpiresAt == nil || !c.ExpiresAt.After(time.Now()) || c.IssuedAt.Before(a.boot) || !c.can("websocket.connect") {
		return false
	}
	for _, key := range []string{"jti:" + c.ID, "user:" + id + ":" + c.User} {
		if t, ok := a.revoked.Load(key); ok && !c.IssuedAt.After(t.(time.Time)) {
			return false
		}
	}
	return true
}
func (a *API) parse(token, id string) (*Claims, error) {
	c := &Claims{}
	_, err := jwt.ParseWithClaims(token, c, func(t *jwt.Token) (any, error) { return []byte(a.Manager.Config.Token), nil }, jwt.WithValidMethods([]string{"HS256"}), jwt.WithExpirationRequired(), jwt.WithIssuedAt(), jwt.WithIssuer(a.Manager.Config.PanelURL))
	if err != nil || !a.valid(c, id) {
		return nil, errors.New("invalid or revoked websocket token")
	}
	return c, nil
}
func (a *API) socket(w http.ResponseWriter, r *http.Request, s *node.Server) {
	select {
	case a.slots <- struct{}{}:
		defer func() { <-a.slots }()
	default:
		failure(w, 503, "Websocket capacity reached")
		return
	}
	u, _ := url.Parse(a.Manager.Config.PanelURL)
	panelOrigin := u.Scheme + "://" + u.Host
	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool {
		o := r.Header.Get("Origin")
		return o == panelOrigin || slices.Contains(a.Manager.Config.Origins, o)
	}}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()
	conn.SetReadLimit(8192)
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	// One reader and one writer; all output is serialized through this event loop.
	incoming := make(chan node.Event, 16)
	done := make(chan struct{})
	defer close(done)
	go func() {
		defer close(incoming)
		for {
			var e node.Event
			if conn.ReadJSON(&e) != nil {
				return
			}
			if e.Event == "auth" {
				conn.SetReadDeadline(time.Time{})
			}
			select {
			case incoming <- e:
			case <-done:
				return
			}
		}
	}()
	events, unsubscribe := s.Subscribe()
	defer unsubscribe()
	timer := time.NewTicker(time.Second)
	defer timer.Stop()
	send := func(event string, args ...string) bool {
		conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
		return conn.WriteJSON(node.Event{Event: event, Args: args}) == nil
	}
	var claims *Claims
	warned := false
	for {
		select {
		case <-a.done:
			return
		case e, ok := <-incoming:
			if !ok {
				return
			}
			if e.Event == "auth" {
				if len(e.Args) != 1 {
					send("jwt error", "Token required")
					return
				}
				claims, err = a.parse(e.Args[0], s.ID())
				if err != nil {
					send("jwt error", err.Error())
					return
				}
				warned = false
				if !send("auth success") || !send("status", s.State()) {
					return
				}
				continue
			}
			if !a.valid(claims, s.ID()) {
				send("jwt error", "Authentication required or token expired")
				return
			}
			switch e.Event {
			case "send logs":
				for _, line := range s.Logs(100) {
					if !send("console output", line) {
						return
					}
				}
			case "send stats":
				if !send("stats", s.StatsJSON()) {
					return
				}
			case "send command":
				if len(e.Args) != 1 || !claims.can("control.console") {
					send("daemon error", "Console permission required")
					continue
				}
				if err = s.Command(e.Args[0]); err != nil {
					send("daemon error", err.Error())
				}
			case "set state":
				if len(e.Args) != 1 {
					continue
				}
				action := e.Args[0]
				perm := ""
				switch action {
				case "start":
					perm = "control.start"
				case "stop", "kill":
					perm = "control.stop"
				case "restart":
					perm = "control.restart"
				}
				if perm == "" || !claims.can(perm) {
					send("daemon error", "Power permission required")
					continue
				}
				if err = s.Power(action); err != nil {
					send("daemon error", err.Error())
				}
			}
		case e, ok := <-events:
			if !ok {
				return
			}
			if a.valid(claims, s.ID()) {
				if !send(e.Event, e.Args...) {
					return
				}
			}
		case <-timer.C:
			if claims == nil {
				continue
			}
			if !a.valid(claims, s.ID()) {
				send("token expired")
				return
			}
			if !warned && time.Until(claims.ExpiresAt.Time) < time.Minute {
				warned = true
				if !send("token expiring") {
					return
				}
			}
			if !send("stats", s.StatsJSON()) {
				return
			}
		}
	}
}
