// Package sftpserver exposes a bounded SSH subsystem. Filesystem and
// panel authorization are delegated to the existing node backend.
package sftpserver

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

type AuthRequest struct {
	Type          string `json:"type"`
	User          string `json:"username"`
	Pass          string `json:"password"`
	IP            string `json:"ip"`
	SessionID     []byte `json:"session_id"`
	ClientVersion []byte `json:"client_version"`
}
type Access struct {
	Server, User string
	Permissions  []string
	Issued       time.Time
}

func (a Access) Can(permission string) bool {
	for _, p := range a.Permissions {
		if p == permission || p == "*" {
			return true
		}
	}
	return false
}

type ReadHandle interface {
	io.ReaderAt
	io.Closer
}
type WriteHandle interface {
	io.WriterAt
	io.Closer
	TransferError(error)
}

// UploadBackend is optional; implementations must enforce their own opt-in.
type UploadBackend interface {
	Upload(context.Context, Access, string) (WriteHandle, error)
}

type CommandBackend interface {
	Command(context.Context, Access, string, string, string) error
}

type Backend interface {
	Authenticate(context.Context, AuthRequest) (Access, error)
	Allowed(Access) bool
	Read(context.Context, Access, string) (ReadHandle, error)
	List(context.Context, Access, string) ([]os.FileInfo, error)
	Stat(context.Context, Access, string) (os.FileInfo, error)
}
type Server struct {
	done        chan struct{}
	ctx         context.Context
	cancel      context.CancelFunc
	backend     Backend
	listener    net.Listener
	signer      ssh.Signer
	mu          sync.Mutex
	closed      bool
	connections map[net.Conn]struct{}
	users       map[string]int
	ip          map[string]int
	wg          sync.WaitGroup
}

func Listen(address, keyFile string, backend Backend) (*Server, error) {
	signer, e := loadSigner(keyFile)
	if e != nil {
		return nil, errors.New("SFTP host key unavailable or invalid")
	}
	l, e := net.Listen("tcp", address)
	if e != nil {
		return nil, errors.New("SFTP listener unavailable")
	}
	return New(l, signer, backend), nil
}
func New(l net.Listener, signer ssh.Signer, backend Backend) *Server {
	ctx, cancel := context.WithCancel(context.Background())
	s := &Server{done: make(chan struct{}), ctx: ctx, cancel: cancel, listener: l, signer: signer, backend: backend, connections: make(map[net.Conn]struct{}), users: make(map[string]int), ip: make(map[string]int)}
	s.wg.Add(1)
	go s.accept()
	return s
}
func (s *Server) Addr() net.Addr      { return s.listener.Addr() }
func (s *Server) Fingerprint() string { return ssh.FingerprintSHA256(s.signer.PublicKey()) }
func (s *Server) accept() {
	defer s.wg.Done()
	defer close(s.done)
	for {
		conn, e := s.listener.Accept()
		if e != nil {
			return
		}
		ip, _, e := net.SplitHostPort(conn.RemoteAddr().String())
		if e != nil {
			conn.Close()
			continue
		}
		s.mu.Lock()
		if s.closed || len(s.connections) >= 16 || s.ip[ip] >= 4 {
			s.mu.Unlock()
			conn.Close()
			continue
		}
		s.connections[conn] = struct{}{}
		s.ip[ip]++
		s.wg.Add(1)
		s.mu.Unlock()
		go s.serve(conn, ip)
	}
}
func (s *Server) serve(conn net.Conn, ip string) {
	defer s.wg.Done()
	defer conn.Close()
	defer func() {
		s.mu.Lock()
		delete(s.connections, conn)
		s.ip[ip]--
		if s.ip[ip] == 0 {
			delete(s.ip, ip)
		}
		s.mu.Unlock()
	}()
	conn.SetDeadline(time.Now().Add(15 * time.Second))
	var access Access
	auth := func(meta ssh.ConnMetadata, kind, value string) (*ssh.Permissions, error) {
		if len(meta.User()) > 200 || len(value) > 8192 || len(meta.SessionID()) > 128 || len(meta.ClientVersion()) > 256 {
			return nil, os.ErrPermission
		}
		ctx, cancel := context.WithTimeout(s.ctx, 5*time.Second)
		defer cancel()
		a, e := s.backend.Authenticate(ctx, AuthRequest{Type: kind, User: meta.User(), Pass: value, IP: ip, SessionID: meta.SessionID(), ClientVersion: meta.ClientVersion()})
		if e != nil || !a.Can("file.sftp") || !s.backend.Allowed(a) {
			return nil, os.ErrPermission
		}
		blob, e := json.Marshal(a)
		if e != nil {
			return nil, os.ErrPermission
		}
		return &ssh.Permissions{Extensions: map[string]string{"wings_access": string(blob)}}, nil
	}
	config := &ssh.ServerConfig{MaxAuthTries: 3, ServerVersion: "SSH-2.0-WingsWindows", PasswordCallback: func(c ssh.ConnMetadata, p []byte) (*ssh.Permissions, error) { return auth(c, "password", string(p)) }, PublicKeyCallback: func(c ssh.ConnMetadata, p ssh.PublicKey) (*ssh.Permissions, error) {
		return auth(c, "public_key", strings.TrimSpace(string(ssh.MarshalAuthorizedKey(p))))
	}}
	config.AddHostKey(s.signer)
	sshConn, channels, requests, e := ssh.NewServerConn(conn, config)
	if e != nil {
		return
	}
	defer sshConn.Close()
	if sshConn.Permissions == nil || json.Unmarshal([]byte(sshConn.Permissions.Extensions["wings_access"]), &access) != nil {
		return
	}
	if !s.backend.Allowed(access) {
		return
	}
	s.mu.Lock()
	if s.closed || s.users[access.User] >= 4 {
		s.mu.Unlock()
		return
	}
	s.users[access.User]++
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		s.users[access.User]--
		if s.users[access.User] == 0 {
			delete(s.users, access.User)
		}
		s.mu.Unlock()
	}()
	conn.SetDeadline(time.Now().Add(60 * time.Second))
	ctx, cancel := context.WithTimeout(s.ctx, 10*time.Minute)
	defer cancel()
	done := make(chan struct{})
	defer close(done)
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				conn.Close()
				return
			case <-done:
				return
			case <-ticker.C:
				if !s.backend.Allowed(access) {
					conn.Close()
					return
				}
			}
		}
	}()
	go ssh.DiscardRequests(requests)
	// Exactly one session channel; shell/exec/forwarding and extra channels fail.
	for channel := range channels {
		if channel.ChannelType() != "session" {
			channel.Reject(ssh.UnknownChannelType, "SFTP only")
			continue
		}
		ch, reqs, e := channel.Accept()
		if e != nil {
			return
		}
		handlers := &handler{backend: s.backend, access: access, ctx: ctx, conn: conn, handles: make(chan struct{}, 16)}
		finished := make(chan struct{})
		go func() {
			defer close(finished)
			defer ch.Close()
			for req := range reqs {
				var payload struct{ Name string }
				ok := req.Type == "subsystem" && ssh.Unmarshal(req.Payload, &payload) == nil && payload.Name == "sftp"
				if req.WantReply {
					req.Reply(ok, nil)
				}
				if !ok {
					continue
				}
				rs := sftp.NewRequestServer(ch, sftp.Handlers{FileGet: handlers, FilePut: handlers, FileCmd: handlers, FileList: handlers}, sftp.WithRSMaxTxPacket(32<<10))
				_ = rs.Serve()
				rs.Close()
				return
			}
		}()
		// Reject any later channel without waiting for the active subsystem to end.
		go func() {
			for extra := range channels {
				extra.Reject(ssh.Prohibited, "One SFTP session only")
			}
		}()
		select {
		case <-finished:
			return
		case <-ctx.Done():
			return
		}
	}
}
func (s *Server) Close(ctx context.Context) error {
	s.mu.Lock()
	s.closed = true
	s.cancel()
	s.listener.Close()
	for conn := range s.connections {
		conn.Close()
	}
	s.mu.Unlock()
	done := make(chan struct{})
	go func() { s.wg.Wait(); close(done) }()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

type handler struct {
	backend Backend
	access  Access
	ctx     context.Context
	conn    net.Conn
	handles chan struct{}
}

func (h *handler) check() error {
	if h.ctx.Err() != nil || !h.backend.Allowed(h.access) {
		return os.ErrPermission
	}
	if h.conn != nil {
		h.conn.SetDeadline(time.Now().Add(60 * time.Second))
	}
	return nil
}
func (h *handler) Filewrite(r *sftp.Request) (io.WriterAt, error) {
	if e := h.check(); e != nil {
		return nil, e
	}
	flags := r.Pflags()
	backend, ok := h.backend.(UploadBackend)
	if !ok || !h.access.Can("file.create") || !flags.Write || !flags.Creat || flags.Append || r.Flags & ^uint32(63) != 0 || len(r.Attrs) > 8192 {
		return nil, os.ErrPermission
	}
	select {
	case h.handles <- struct{}{}:
	default:
		return nil, os.ErrPermission
	}
	f, e := backend.Upload(h.ctx, h.access, r.Filepath)
	if e != nil {
		<-h.handles
		return nil, os.ErrPermission
	}
	return &writer{WriteHandle: f, check: h.check, release: func() { <-h.handles }}, nil
}

// Clients commonly use a read/write OPEN for creating a file. Only its write
// side is supported; unpublished uploads cannot be read through that handle.
func (h *handler) OpenFile(r *sftp.Request) (sftp.WriterAtReaderAt, error) {
	f, e := h.Filewrite(r)
	if e != nil {
		return nil, e
	}
	return f.(*writer), nil
}

type writer struct {
	WriteHandle
	check   func() error
	release func()
	once    sync.Once
	err     error
}

func (w *writer) ReadAt([]byte, int64) (int, error) { return 0, os.ErrPermission }
func (w *writer) WriteAt(p []byte, off int64) (int, error) {
	e := w.check()
	if e == nil && (len(p) > 64<<10 || off < 0) {
		e = os.ErrPermission
	}
	if e != nil {
		w.TransferError(e)
		return 0, e
	}
	return w.WriteHandle.WriteAt(p, off)
}
func (w *writer) Close() error {
	w.once.Do(func() {
		if e := w.check(); e != nil {
			w.TransferError(e)
		}
		w.err = w.WriteHandle.Close()
		w.release()
	})
	return w.err
}
func (h *handler) Filecmd(r *sftp.Request) error {
	if e := h.check(); e != nil {
		return e
	}
	permission := CommandPermission(r.Method)
	backend, ok := h.backend.(CommandBackend)
	if !ok || permission == "" || !h.access.Can(permission) {
		return os.ErrPermission
	}
	if e := backend.Command(h.ctx, h.access, r.Method, r.Filepath, r.Target); e != nil {
		return os.ErrPermission
	}
	return nil
}
func CommandPermission(method string) string {
	switch method {
	case "Mkdir":
		return "file.create"
	case "Rename":
		return "file.update"
	case "Remove", "Rmdir":
		return "file.delete"
	}
	return ""
}
func (h *handler) Fileread(r *sftp.Request) (io.ReaderAt, error) {
	if e := h.check(); e != nil {
		return nil, e
	}
	if !h.access.Can("file.read-content") {
		return nil, os.ErrPermission
	}
	select {
	case h.handles <- struct{}{}:
	default:
		return nil, os.ErrPermission
	}
	f, e := h.backend.Read(h.ctx, h.access, r.Filepath)
	if e != nil {
		<-h.handles
		return nil, os.ErrPermission
	}
	return &reader{ReadHandle: f, check: h.check, release: func() { <-h.handles }}, nil
}

type reader struct {
	ReadHandle
	check   func() error
	once    sync.Once
	release func()
}

func (r *reader) ReadAt(p []byte, offset int64) (int, error) {
	if e := r.check(); e != nil {
		return 0, e
	}
	if len(p) > 64<<10 || offset < 0 {
		return 0, os.ErrPermission
	}
	return r.ReadHandle.ReadAt(p, offset)
}
func (r *reader) Close() error {
	var e error
	r.once.Do(func() { e = r.ReadHandle.Close(); r.release() })
	return e
}

type listing struct {
	once    sync.Once
	release func()
	items   []os.FileInfo
	check   func() error
}

func (l *listing) ListAt(items []os.FileInfo, offset int64) (int, error) {
	if e := l.check(); e != nil {
		return 0, e
	}
	if offset < 0 || offset >= int64(len(l.items)) {
		return 0, io.EOF
	}
	for _, entry := range l.items {
		if dynamic, ok := entry.(interface{ Allowed() bool }); ok && !dynamic.Allowed() {
			return 0, os.ErrPermission
		}
	}
	n := copy(items, l.items[int(offset):])
	if n < len(items) {
		return n, io.EOF
	}
	return n, nil
}
func (h *handler) Filelist(r *sftp.Request) (sftp.ListerAt, error) {
	if e := h.check(); e != nil {
		return nil, e
	}
	if !h.access.Can("file.read") {
		return nil, os.ErrPermission
	}
	var release func()
	if r.Method == "List" {
		select {
		case h.handles <- struct{}{}:
			release = func() { <-h.handles }
		default:
			return nil, os.ErrPermission
		}
	}
	var items []os.FileInfo
	var e error
	switch r.Method {
	case "List":
		items, e = h.backend.List(h.ctx, h.access, r.Filepath)
	case "Stat", "Lstat":
		var f os.FileInfo
		f, e = h.backend.Stat(h.ctx, h.access, r.Filepath)
		if e == nil {
			items = []os.FileInfo{f}
		}
	default:
		e = os.ErrPermission
	}
	if e != nil {
		if release != nil {
			release()
		}
		return nil, os.ErrPermission
	}
	return &listing{items: items, check: h.check, release: release}, nil
}
func (h *handler) RealPath(name string) (string, error) {
	if e := h.check(); e != nil {
		return "", e
	}
	if !h.access.Can("file.read") {
		return "", os.ErrPermission
	}
	if _, e := h.backend.Stat(h.ctx, h.access, name); e != nil {
		return "", os.ErrPermission
	}
	if name == "." || name == "" {
		return "/", nil
	}
	return "/" + strings.Trim(name, "/"), nil
}

func (l *listing) Close() error {
	l.once.Do(func() {
		if l.release != nil {
			l.release()
		}
	})
	return nil
}

func (s *Server) Done() <-chan struct{} { return s.done }
