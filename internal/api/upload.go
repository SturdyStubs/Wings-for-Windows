package api

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"time"
	"wings-windows/internal/node"
)

const maxUploadBytes int64 = 100 << 20
const maxUploadFiles = 16
const uploadOverhead int64 = 256 << 10

func (a *API) uploadFile(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Referrer-Policy", "no-referrer")
	q, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil || len(q["token"]) != 1 || len(q["directory"]) > 1 {
		failure(w, 401, "Invalid upload request")
		return
	}
	c, err := a.parseFileToken(q.Get("token"), "file-upload")
	if err != nil {
		failure(w, 401, "Invalid or unavailable upload token")
		return
	}
	s := a.Manager.Get(c.Server)
	if s == nil {
		failure(w, 404, "Upload resource unavailable")
		return
	}
	dir := strings.Trim(q.Get("directory"), "/")
	if dir != "" {
		parts, e := directoryParts("", dir)
		if e != nil {
			failure(w, 422, "Invalid upload directory")
			return
		}
		dir = strings.Join(parts, "/")
	} else {
		dir = "."
	}
	select {
	case a.uploadSlots <- struct{}{}:
		defer func() { <-a.uploadSlots }()
	default:
		failure(w, 503, "Upload capacity reached; request a fresh link")
		return
	}
	controller := http.NewResponseController(w)
	// The global write timeout is too short for a streamed upload. Keep a hard
	// token-expiry ceiling, then restore a short deadline for the response.
	if e := controller.SetWriteDeadline(c.ExpiresAt.Time); e != nil && !errors.Is(e, http.ErrNotSupported) {
		failure(w, 500, "Cannot prepare upload transport")
		return
	}
	check := func() error {
		if r.Context().Err() != nil {
			return &directoryFailure{408, "Upload cancelled"}
		}
		select {
		case <-a.done:
			return &directoryFailure{503, "Node is stopping"}
		default:
		}
		if !a.validFileToken(c, "file-upload") {
			return &directoryFailure{401, "Upload authorization expired or revoked"}
		}
		if e := renamePolicy(s, dir, true); e != nil {
			return &directoryFailure{403, "Upload directory is restricted"}
		}
		return nil
	}
	err = a.receiveUpload(w, r, s, c, dir, controller, check)
	_ = (downloadWriter{expires: c.ExpiresAt.Time, controller: controller}).deadline()
	if err != nil {
		status, message := 400, "Upload could not be completed; reload the file listing"
		var p *directoryFailure
		var large *http.MaxBytesError
		if errors.As(err, &p) {
			status, message = p.status, p.message
		} else if errors.As(err, &large) {
			status, message = 413, "Upload exceeds the 100 MiB request limit"
		} else if copyDiskFull(err) {
			status, message = 507, "Insufficient storage for upload"
		}
		failure(w, status, message)
		return
	}
	respond(w, 200, nil)
}

type uploadStage struct {
	f         *os.File
	name      string
	published bool
}

func (a *API) receiveUpload(w http.ResponseWriter, r *http.Request, s *node.Server, c *downloadClaims, dir string, controller *http.ResponseController, check func() error) (err error) {
	if err = check(); err != nil {
		return err
	}
	root, err := os.OpenRoot(s.Root)
	if err != nil {
		return &directoryFailure{500, "Cannot open upload data"}
	}
	defer root.Close()
	parent, err := renameParent(r.Context(), root, s, path.Join(dir, ".upload-parent-check"))
	if err != nil {
		return &directoryFailure{403, "Upload parent unavailable or restricted"}
	}
	defer parent.Close()
	canonical, err := resolvedFilePath(root, parent)
	if err != nil {
		return &directoryFailure{403, "Upload parent unavailable"}
	}
	current := func(name string) error {
		if e := check(); e != nil {
			return e
		}
		actual, e := resolvedFilePath(root, parent)
		if e != nil || !strings.EqualFold(actual, canonical) {
			return &directoryFailure{409, "Upload parent changed"}
		}
		if renamePolicy(s, path.Join(dir, name), false) != nil || renamePolicy(s, path.Join(actual, name), false) != nil {
			return &directoryFailure{403, "Upload path is restricted"}
		}
		return nil
	}
	if r.ContentLength > maxUploadBytes+uploadOverhead {
		return &directoryFailure{413, "Upload exceeds the 100 MiB request limit"}
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes+uploadOverhead)
	r.Body = &uploadBody{ReadCloser: r.Body, check: check, controller: controller, expires: c.ExpiresAt.Time}
	reader, err := r.MultipartReader()
	if err != nil {
		return &directoryFailure{400, "Expected multipart/form-data"}
	}
	if status := a.consumeDownload(c); status != 0 {
		return &directoryFailure{status, "Upload link already used or capacity reached; request a new link"}
	}
	var stages []*uploadStage
	defer func() {
		for _, stage := range stages {
			if !stage.published {
				if discardCopyStage(stage.f) != nil {
					err = &directoryFailure{500, "Upload failed; an owned temporary upload may remain"}
				}
			}
			stage.f.Close()
		}
	}()
	seen := map[string]bool{}
	var total int64
	buf := make([]byte, 64<<10)
	for {
		if err = check(); err != nil {
			return err
		}
		part, e := reader.NextRawPart()
		if e == io.EOF {
			break
		}
		if e != nil {
			return e
		}
		if len(stages) >= maxUploadFiles {
			return &directoryFailure{413, "At most 16 files may be uploaded per request"}
		}
		headerBytes, headerCount := 0, 0
		for k, values := range part.Header {
			for _, v := range values {
				headerBytes += len(k) + len(v)
				headerCount++
			}
		}
		if headerBytes > 8192 || headerCount > 16 || len(part.Header.Values("Content-Disposition")) != 1 || part.Header.Get("Content-Transfer-Encoding") != "" {
			return &directoryFailure{400, "Unsupported multipart headers"}
		}
		kind, params, e := mime.ParseMediaType(part.Header.Get("Content-Disposition"))
		name := params["filename"] // Do not use Part.FileName: it silently strips path components.
		if e != nil || kind != "form-data" || params["name"] != "files" || !validRenameLeaf(name) {
			return &directoryFailure{422, "Expected files fields with plain Windows filenames"}
		}
		if _, e = directoryParts("", path.Join(dir, name)); e != nil {
			return &directoryFailure{422, "Upload destination exceeds Windows path bounds"}
		}
		if seen[strings.ToLower(name)] {
			return &directoryFailure{409, "Duplicate upload filename"}
		}
		seen[strings.ToLower(name)] = true
		if err = current(name); err != nil {
			return err
		}
		if _, e = root.Lstat(path.Join(canonical, name)); e == nil {
			return &directoryFailure{409, "Upload destination already exists"}
		} else if !os.IsNotExist(e) {
			return e
		}
		var random [16]byte
		if _, err = rand.Read(random[:]); err != nil {
			return err
		}
		stageName := ".wings-upload-" + hex.EncodeToString(random[:]) + ".tmp"
		if err = current(stageName); err != nil {
			return err
		}
		f, e := openCopyHandle(parent, stageName, true)
		if e != nil {
			return e
		}
		stage := &uploadStage{f: f, name: name}
		stages = append(stages, stage)
		// Both wrappers prevent io.Copy fast paths from skipping authorization checks.
		n, e := io.CopyBuffer(copyWriter{ctx: r.Context(), w: f}, downloadReader{r: io.LimitReader(part, maxUploadBytes-total+1), check: func() error { return current(name) }}, buf)
		total += n
		if e != nil {
			return e
		}
		if total > maxUploadBytes {
			return &directoryFailure{413, "Upload exceeds the 100 MiB aggregate file limit"}
		}
		if err = f.Sync(); err != nil {
			return err
		}
	}
	if len(stages) == 0 {
		return &directoryFailure{400, "No files were uploaded"}
	}
	// Drain bounded epilogue as well: a truncated body, cancellation or excessive
	// trailing data must not publish files which were only partially validated.
	if _, err = io.Copy(io.Discard, r.Body); err != nil {
		return err
	}
	for _, stage := range stages {
		if err = current(stage.name); err != nil {
			return err
		}
		if err = renameNoReplace(stage.f, parent, stage.name); err != nil {
			if os.IsExist(err) {
				return &directoryFailure{409, "Upload destination appeared during transfer; reload the file listing"}
			}
			return err
		}
		stage.published = true
	}
	return nil
}

type uploadBody struct {
	io.ReadCloser
	check      func() error
	controller *http.ResponseController
	expires    time.Time
}

func (b *uploadBody) Read(p []byte) (int, error) {
	if e := b.check(); e != nil {
		return 0, e
	}
	deadline := time.Now().Add(30 * time.Second)
	if b.expires.Before(deadline) {
		deadline = b.expires
	}
	if e := b.controller.SetReadDeadline(deadline); e != nil && !errors.Is(e, http.ErrNotSupported) {
		return 0, e
	}
	n, e := b.ReadCloser.Read(p)
	if checkErr := b.check(); checkErr != nil {
		return 0, checkErr
	}
	return n, e
}
