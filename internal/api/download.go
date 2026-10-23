package api

import (
	"context"
	"errors"
	"github.com/golang-jwt/jwt/v5"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"slices"
	"strconv"
	"strings"
	"time"
	"wings-windows/internal/config"
)

const maxDownloadBytes int64 = 1 << 30
const maxDownloadReceipts = 4096

type downloadClaims struct {
	jwt.RegisteredClaims
	Scope    string `json:"scope"`
	FilePath string `json:"file_path"`
	Server   string `json:"server_uuid"`
	User     string `json:"user_uuid"`
	Unique   string `json:"unique_id"`
}

func (a *API) validDownload(c *downloadClaims) bool {
	return a.validFileToken(c, "file-download")
}

func (a *API) validFileToken(c *downloadClaims, scope string) bool {
	now := time.Now()
	if c == nil || c.IssuedAt == nil || c.ExpiresAt == nil || c.IssuedAt.Before(a.boot) || !c.ExpiresAt.After(now) || c.ExpiresAt.Sub(c.IssuedAt.Time) > 16*time.Minute || !config.UUID.MatchString(c.Server) || !config.UUID.MatchString(c.User) || c.ID == "" || len(c.ID) > 256 || c.Unique == "" || len(c.Unique) > 256 || len(c.FilePath) > 4096 || len(c.Scope) > 128 || !slices.Contains(strings.Fields(c.Scope), scope) {
		return false
	}
	for _, key := range []string{"jti:" + c.ID, "user:" + strings.ToLower(c.Server) + ":" + strings.ToLower(c.User)} {
		if t, ok := a.revoked.Load(key); ok && !c.IssuedAt.After(t.(time.Time)) {
			return false
		}
	}
	return true
}

func (a *API) parseDownload(token string) (*downloadClaims, error) {
	return a.parseFileToken(token, "file-download")
}

func (a *API) parseFileToken(token, scope string) (*downloadClaims, error) {
	if token == "" || len(token) > 8192 {
		return nil, errors.New("invalid download token")
	}
	c := &downloadClaims{}
	_, err := jwt.ParseWithClaims(token, c, func(*jwt.Token) (any, error) { return []byte(a.Manager.Config.Token), nil }, jwt.WithValidMethods([]string{"HS256"}), jwt.WithExpirationRequired(), jwt.WithIssuedAt(), jwt.WithIssuer(a.Manager.Config.PanelURL))
	if err != nil || !a.validFileToken(c, scope) {
		return nil, errors.New("invalid download token")
	}
	return c, nil
}

// Serialize check-and-consume. Never evict unexpired receipts to admit new ones.
func (a *API) consumeDownload(c *downloadClaims) int {
	a.downloadMu.Lock()
	defer a.downloadMu.Unlock()
	now := time.Now()
	for key, expires := range a.downloadUsed {
		if !expires.After(now) {
			delete(a.downloadUsed, key)
		}
	}
	if _, used := a.downloadUsed[c.Unique]; used {
		return http.StatusNotFound
	}
	if len(a.downloadUsed) >= maxDownloadReceipts {
		return http.StatusServiceUnavailable
	}
	a.downloadUsed[c.Unique] = c.ExpiresAt.Time
	return 0
}

func (a *API) downloadFile(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Referrer-Policy", "no-referrer")
	values, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil || len(values["token"]) != 1 {
		failure(w, 401, "Invalid or unavailable download token")
		return
	}
	claims, err := a.parseDownload(values.Get("token"))
	if err != nil {
		failure(w, 401, "Invalid or unavailable download token")
		return
	}
	s := a.Manager.Get(claims.Server)
	if s == nil {
		failure(w, 404, "Download resource unavailable")
		return
	}
	if r.Header.Get("Range") != "" {
		w.Header().Set("Accept-Ranges", "none")
		failure(w, 416, "Signed downloads require a new full-file request")
		return
	}
	parts, err := directoryParts("", strings.Trim(claims.FilePath, "/"))
	if err != nil {
		failure(w, 422, "Invalid signed file path")
		return
	}
	name := strings.Join(parts, "/")
	select {
	case a.downloadSlots <- struct{}{}:
		defer func() { <-a.downloadSlots }()
	default:
		failure(w, 503, "Download capacity reached; request a fresh link")
		return
	}
	if err = renamePolicy(s, name, false); err != nil {
		failure(w, 403, "Download path is restricted")
		return
	}
	root, err := os.OpenRoot(s.Root)
	if err != nil {
		failure(w, 500, "Cannot open download data")
		return
	}
	defer root.Close()
	parent, err := renameParent(r.Context(), root, s, name)
	if err != nil {
		status := 404
		if p, ok := err.(*directoryFailure); ok {
			status = p.status
		}
		failure(w, status, "Download parent unavailable or restricted")
		return
	}
	defer parent.Close()
	info, err := root.Lstat(name)
	if err != nil {
		failure(w, 404, "Download resource unavailable")
		return
	}
	if fileIsReparse(info) {
		failure(w, 403, "Downloads refuse links and reparse points")
		return
	}
	if !info.Mode().IsRegular() {
		failure(w, 404, "Download resource unavailable")
		return
	}
	f, err := openCopyHandle(parent, path.Base(name), false)
	if err != nil {
		status := 423
		if os.IsNotExist(err) {
			status = 404
		}
		failure(w, status, "Download file unavailable or locked")
		return
	}
	defer f.Close()
	if err = inspectRenameEntry(f); err != nil {
		failure(w, 403, "Downloads refuse links and multi-hardlink files")
		return
	}
	info, err = f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		failure(w, 404, "Download resource unavailable")
		return
	}
	if info.Size() < 0 || info.Size() > maxDownloadBytes {
		failure(w, 413, "Signed downloads support files up to 1 GiB")
		return
	}
	canonical, err := resolvedFilePath(root, f)
	if err != nil {
		failure(w, 403, "Download path unavailable")
		return
	}
	if err = renamePolicy(s, canonical, false); err != nil {
		failure(w, 403, "Download path is restricted")
		return
	}
	valid := func() error {
		if err := r.Context().Err(); err != nil {
			return err
		}
		select {
		case <-a.done:
			return context.Canceled
		default:
		}
		if !a.validDownload(claims) {
			return errors.New("download authorization expired")
		}
		return renamePolicy(s, canonical, false)
	}
	if err = valid(); err != nil {
		failure(w, 401, "Download expired or cancelled")
		return
	}
	if status := a.consumeDownload(claims); status != 0 {
		failure(w, status, "Download link already used or capacity reached; request a new link")
		return
	}
	// Reset the global server write timeout for each bounded chunk, capped by JWT
	// expiry. Slow/broken clients still have a 30-second write-idle deadline.
	writer := downloadWriter{w: w, check: valid, expires: claims.ExpiresAt.Time, controller: http.NewResponseController(w)}
	if err = writer.deadline(); err != nil {
		failure(w, 500, "Cannot prepare download transport")
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", strconv.FormatInt(info.Size(), 10))
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": path.Base(canonical)}))
	w.Header().Set("Accept-Ranges", "none")
	w.WriteHeader(http.StatusOK)
	n, err := io.CopyBuffer(writer, downloadReader{r: io.LimitReader(f, info.Size()), check: valid}, make([]byte, 64<<10))
	if err != nil || n != info.Size() {
		panic(http.ErrAbortHandler)
	}
}

type downloadReader struct {
	r     io.Reader
	check func() error
}

func (d downloadReader) Read(p []byte) (int, error) {
	if err := d.check(); err != nil {
		return 0, err
	}
	return d.r.Read(p)
}

type downloadWriter struct {
	w          io.Writer
	check      func() error
	expires    time.Time
	controller *http.ResponseController
}

func (d downloadWriter) deadline() error {
	deadline := time.Now().Add(30 * time.Second)
	if d.expires.Before(deadline) {
		deadline = d.expires
	}
	err := d.controller.SetWriteDeadline(deadline)
	if errors.Is(err, http.ErrNotSupported) {
		return nil
	}
	return err
}
func (d downloadWriter) Write(p []byte) (int, error) {
	if err := d.check(); err != nil {
		return 0, err
	}
	if err := d.deadline(); err != nil {
		return 0, err
	}
	return d.w.Write(p)
}
