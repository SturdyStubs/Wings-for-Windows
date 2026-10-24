package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"sort"
	"strings"
	"sync/atomic"
	"time"
	"wings-windows/internal/node"
)

const maxPullBytes int64 = 100 << 20

type pullJob struct {
	id, server string
	ctx        context.Context
	cancel     context.CancelFunc
	bytes      atomic.Int64
	length     atomic.Int64
}
type pullInput struct {
	Root       string `json:"root"`
	Directory  string `json:"directory"`
	URL        string `json:"url"`
	FileName   string `json:"file_name"`
	UseHeader  bool   `json:"use_header"`
	Foreground bool   `json:"foreground"`
}
type pullProgress struct {
	Identifier string
	Progress   float64
}

func (j *pullJob) progress() pullProgress {
	p := float64(0)
	if size := j.length.Load(); size > 0 {
		p = float64(j.bytes.Load()) / float64(size)
		if p > 1 {
			p = 1
		}
	}
	return pullProgress{j.id, p}
}
func randomPullID() (string, error) {
	var b [16]byte
	if _, e := rand.Read(b[:]); e != nil {
		return "", e
	}
	b[6] = b[6]&15 | 64
	b[8] = b[8]&63 | 128
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[:4], b[4:6], b[6:8], b[8:10], b[10:]), nil
}

func (a *API) pullFiles(w http.ResponseWriter, r *http.Request, s *node.Server, action string) {
	if !a.Manager.Config.AllowRemoteDownloads {
		failure(w, 501, "Remote downloads require allow_remote_downloads in the native configuration")
		return
	}
	if action == "pull" && r.Method == http.MethodGet {
		items := []pullProgress{}
		a.pullMu.Lock()
		for _, job := range a.pulls {
			if job.server == s.ID() {
				items = append(items, job.progress())
			}
		}
		a.pullMu.Unlock()
		sort.Slice(items, func(i, j int) bool { return items[i].Identifier < items[j].Identifier })
		respond(w, 200, map[string]any{"downloads": items})
		return
	}
	if strings.HasPrefix(action, "pull/") && r.Method == http.MethodDelete && !strings.Contains(strings.TrimPrefix(action, "pull/"), "/") {
		a.pullMu.Lock()
		if job := a.pulls[strings.TrimPrefix(action, "pull/")]; job != nil && job.server == s.ID() {
			job.cancel()
		}
		a.pullMu.Unlock()
		respond(w, 204, nil)
		return
	}
	if action != "pull" || r.Method != http.MethodPost {
		failure(w, 501, "Unsupported remote download operation")
		return
	}
	var input pullInput
	if !decode(w, r, &input) {
		return
	}
	if input.Root == "" {
		input.Root = input.Directory
	}
	input.Root = strings.Trim(input.Root, "/")
	if input.Root != "" {
		parts, e := directoryParts("", input.Root)
		if e != nil {
			failure(w, 422, "Invalid remote download directory")
			return
		}
		input.Root = strings.Join(parts, "/")
	}
	u, e := pullURL(input.URL)
	if e != nil {
		failure(w, 422, "Only public HTTPS URLs on port 443 without credentials or fragments are supported")
		return
	}
	if input.FileName != "" && !validRenameLeaf(input.FileName) {
		failure(w, 422, "Invalid remote download filename")
		return
	}
	if !input.UseHeader && input.FileName == "" && !validRenameLeaf(path.Base(u.Path)) {
		failure(w, 422, "Specify a valid download filename")
		return
	}
	if input.Foreground {
		e := http.NewResponseController(w).SetWriteDeadline(time.Now().Add(125 * time.Second))
		if e != nil && !errors.Is(e, http.ErrNotSupported) {
			failure(w, 500, "Cannot prepare download response")
			return
		}
	}
	id, e := randomPullID()
	if e != nil {
		failure(w, 500, "Cannot identify download")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	job := &pullJob{id: id, server: s.ID(), ctx: ctx, cancel: cancel}
	job.length.Store(-1)
	a.pullMu.Lock()
	count := 0
	for _, old := range a.pulls {
		if old.server == s.ID() {
			count++
		}
	}
	select {
	case <-a.done:
		a.pullMu.Unlock()
		cancel()
		failure(w, 503, "Node is stopping")
		return
	default:
	}
	if len(a.pulls) >= 6 || count >= 3 {
		a.pullMu.Unlock()
		cancel()
		failure(w, 503, "Remote download capacity reached")
		return
	}
	a.pulls[id] = job
	a.pullWG.Add(1)
	a.pullMu.Unlock()
	run := func() (fileMetadata, error) {
		defer a.pullWG.Done()
		defer cancel()
		defer func() { a.pullMu.Lock(); delete(a.pulls, id); a.pullMu.Unlock() }()
		return a.executePull(s, job, u, input)
	}
	if !input.Foreground {
		go func() {
			_, e := run()
			state := "completed"
			if e != nil {
				state = "failed or cancelled"
			}
			log.Printf("remote file pull %s %s", id, state)
		}()
		respond(w, 202, map[string]string{"identifier": id})
		return
	}
	stop := context.AfterFunc(r.Context(), cancel)
	defer stop()
	metadata, e := run()
	_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(10 * time.Second))
	if e != nil {
		status, message := 400, "Remote download failed; request a new transfer"
		var p *directoryFailure
		if errors.As(e, &p) {
			status, message = p.status, p.message
		} else if os.IsExist(e) {
			status, message = 409, "Download destination already exists"
		} else if os.IsNotExist(e) {
			status, message = 404, "Download parent unavailable"
		} else if copyDiskFull(e) {
			status, message = 507, "Insufficient storage for remote download"
		}
		failure(w, status, message)
		return
	}
	respond(w, 200, metadata)
}

func (a *API) executePull(s *node.Server, job *pullJob, u *url.URL, input pullInput) (metadata fileMetadata, err error) {
	check := func() error {
		if job.ctx.Err() != nil {
			return &directoryFailure{408, "Remote download cancelled or time limit reached"}
		}
		select {
		case <-a.done:
			return &directoryFailure{503, "Node is stopping"}
		default:
		}
		return nil
	}
	if err = check(); err != nil {
		return
	}
	root, e := os.OpenRoot(s.Root)
	if e != nil {
		return metadata, &directoryFailure{500, "Cannot open download data"}
	}
	defer root.Close()
	parent, e := renameParent(job.ctx, root, s, path.Join(input.Root, ".pull-parent-check"))
	if e != nil {
		return metadata, &directoryFailure{403, "Download directory unavailable or restricted"}
	}
	defer parent.Close()
	canonical, e := resolvedFilePath(root, parent)
	if e != nil {
		return metadata, e
	}
	current := func(name string) error {
		if e := check(); e != nil {
			return e
		}
		actual, e := resolvedFilePath(root, parent)
		if e != nil || !strings.EqualFold(actual, canonical) {
			return &directoryFailure{409, "Download parent changed"}
		}
		if renamePolicy(s, path.Join(input.Root, name), false) != nil || renamePolicy(s, path.Join(actual, name), false) != nil {
			return &directoryFailure{403, "Download path is restricted"}
		}
		return nil
	}
	req, e := http.NewRequestWithContext(job.ctx, "GET", u.String(), nil)
	if e != nil {
		return metadata, &directoryFailure{400, "Invalid remote request"}
	}
	req.Header.Set("User-Agent", "Wings-Windows-Native")
	req.Header.Set("Accept-Encoding", "identity")
	resp, e := a.pullClient.Do(req)
	if e != nil {
		if e = check(); e != nil {
			return metadata, e
		}
		return metadata, &directoryFailure{400, "Remote HTTPS request failed or destination was refused"}
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return metadata, &directoryFailure{400, "Remote response was not HTTP 200; redirects are not followed"}
	}
	if resp.Header.Get("Content-Encoding") != "" && resp.Header.Get("Content-Encoding") != "identity" {
		return metadata, &directoryFailure{400, "Encoded remote responses are unsupported"}
	}
	if resp.ContentLength > maxPullBytes {
		return metadata, &directoryFailure{413, "Remote file exceeds 100 MiB"}
	}
	job.length.Store(resp.ContentLength)
	name := input.FileName
	if input.UseHeader && resp.Header.Get("Content-Disposition") != "" {
		if len(resp.Header.Values("Content-Disposition")) != 1 {
			return metadata, &directoryFailure{422, "Ambiguous remote filename"}
		}
		_, params, e := mime.ParseMediaType(resp.Header.Get("Content-Disposition"))
		if e != nil {
			return metadata, &directoryFailure{422, "Invalid remote filename header"}
		}
		if headerName := params["filename"]; headerName != "" {
			name = headerName
		}
	}
	if name == "" {
		name = path.Base(u.Path)
	}
	if !validRenameLeaf(name) {
		return metadata, &directoryFailure{422, "Remote filename must be a plain Windows filename"}
	}
	if _, e = directoryParts(input.Root, name); e != nil {
		return metadata, &directoryFailure{422, "Download destination exceeds Windows bounds"}
	}
	if e = current(name); e != nil {
		return metadata, e
	}
	if _, e = root.Lstat(path.Join(canonical, name)); e == nil {
		return metadata, &directoryFailure{409, "Download destination already exists"}
	} else if !os.IsNotExist(e) {
		return metadata, e
	}
	var random [16]byte
	if _, e = rand.Read(random[:]); e != nil {
		return metadata, e
	}
	stageName := ".wings-pull-" + hex.EncodeToString(random[:]) + ".tmp"
	if e = current(stageName); e != nil {
		return metadata, e
	}
	stage, e := openCopyHandle(parent, stageName, true)
	if e != nil {
		return metadata, e
	}
	published := false
	defer func() {
		if !published {
			if discardCopyStage(stage) != nil {
				err = &directoryFailure{500, "Download failed; an owned temporary file may remain"}
			}
		}
		stage.Close()
	}()
	guard := func() error {
		if e := current(stageName); e != nil {
			return e
		}
		return current(name)
	}
	written, e := io.CopyBuffer(pullCounter{writer: stage, job: job}, downloadReader{r: io.LimitReader(resp.Body, maxPullBytes+1), check: guard}, make([]byte, 64<<10))
	if e != nil {
		if cancelled := check(); cancelled != nil {
			return metadata, cancelled
		}
		return metadata, e
	}
	if written > maxPullBytes {
		return metadata, &directoryFailure{413, "Remote file exceeds 100 MiB"}
	}
	if resp.ContentLength >= 0 && written != resp.ContentLength {
		return metadata, &directoryFailure{400, "Remote file length did not match response"}
	}
	if e = stage.Sync(); e != nil {
		return metadata, e
	}
	if e = guard(); e != nil {
		return metadata, e
	}
	info, e := stage.Stat()
	if e != nil {
		return metadata, e
	}
	if e = renameNoReplace(stage, parent, name); e != nil {
		return metadata, e
	}
	published = true
	metadata = fileMetadata{Name: name, Mode: info.Mode().String(), ModeBits: fmt.Sprintf("%o", info.Mode().Perm()), Size: info.Size(), File: true, Mime: "application/octet-stream", Created: fileCreated(info).UTC().Format(time.RFC3339), Modified: info.ModTime().UTC().Format(time.RFC3339)}
	return
}

type pullCounter struct {
	writer io.Writer
	job    *pullJob
}

func (w pullCounter) Write(p []byte) (int, error) {
	n, e := w.writer.Write(p)
	w.job.bytes.Add(int64(n))
	return n, e
}
