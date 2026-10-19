package api

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"net/url"
	"slices"
)

func newRequestID() string {
	var b [16]byte
	rand.Read(b[:])
	b[6] = (b[6] & 15) | 64
	b[8] = (b[8] & 63) | 128
	v := hex.EncodeToString(b[:])
	return v[:8] + "-" + v[8:12] + "-" + v[12:16] + "-" + v[16:20] + "-" + v[20:]
}
func (a *API) panelOrigin() string {
	u, _ := url.Parse(a.Manager.Config.PanelURL)
	return u.Scheme + "://" + u.Host
}
func (a *API) originAllowed(origin string) bool {
	return origin != "" && (origin == a.panelOrigin() || slices.Contains(a.Manager.Config.Origins, origin))
}
func (a *API) headers(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Request-Id", newRequestID())
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "no-store")
	origin := a.panelOrigin()
	if a.originAllowed(r.Header.Get("Origin")) {
		origin = r.Header.Get("Origin")
	}
	w.Header().Set("Access-Control-Allow-Origin", origin)
	w.Header().Set("Access-Control-Allow-Credentials", "true")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Accept, Accept-Encoding, Authorization, Cache-Control, Content-Type, Content-Length, Origin, X-Real-IP, X-CSRF-Token")
	w.Header().Set("Access-Control-Expose-Headers", "X-Request-Id, X-Mime-Type, Content-Length, Content-Disposition")
	w.Header().Set("Access-Control-Max-Age", "7200")
	w.Header().Add("Vary", "Origin")
	if a.Manager.Config.AllowCORSPrivateNetwork {
		w.Header().Set("Access-Control-Allow-Private-Network", "true")
	}
}
