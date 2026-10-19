package node

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
	"wings-windows/internal/config"
	"wings-windows/internal/identity"
	"wings-windows/internal/native"
	"wings-windows/internal/provision"
)

type Event struct {
	Event string   `json:"event"`
	Args  []string `json:"args"`
}
type Server struct {
	mu                sync.Mutex
	configurationSync sync.Mutex
	power             sync.Mutex
	powerCancel       context.CancelFunc
	powerEpoch        chan struct{}
	pendingPower      int
	filePolicy        *FilePolicy
	processPolicy     *processPolicy
	appliedLimits     native.Limits
	write             sync.Mutex
	settings          Settings
	Profile           config.Profile
	Root              string
	panel             *Panel
	process           *native.Process
	state             string
	started           time.Time
	previousSample    time.Time
	previousCPU       uint64
	cpu               float64
	logs              []string
	logBytes          int
	subscribers       map[chan Event]struct{}
	closing           bool
}
type Manager struct {
	Config  config.Config
	Panel   *Panel
	mu      sync.RWMutex
	servers map[string]*Server
}

func NewManager(c config.Config) *Manager {
	return &Manager{Config: c, Panel: NewPanel(c), servers: make(map[string]*Server)}
}
func (m *Manager) Sync(ctx context.Context) error {
	remote, err := m.Panel.Servers(ctx)
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, r := range remote {
		id := strings.ToLower(r.Settings.UUID)
		if !config.UUID.MatchString(id) || (r.UUID != "" && !strings.EqualFold(r.UUID, id)) {
			return errors.New("invalid panel server UUID")
		}
		profile, ok := m.Config.Profiles[id]
		if !ok {
			continue
		}
		if _, ok = m.servers[id]; ok {
			continue
		}
		root := filepath.Join(m.Config.DataDirectory, id)
		policy, err := NewFilePolicy(r.Settings.Egg.FileDenylist)
		if err != nil {
			return err
		}
		processPolicy, err := newProcessPolicy(profile, r.ProcessConfiguration)
		if err != nil {
			return fmt.Errorf("server %s: %w", id, err)
		}
		if err = os.MkdirAll(root, 0700); err != nil {
			return err
		}
		r.Settings.UUID = id
		m.servers[id] = &Server{settings: r.Settings, filePolicy: policy, processPolicy: processPolicy, powerEpoch: make(chan struct{}), Profile: profile, Root: root, panel: m.Panel, state: "offline", subscribers: make(map[chan Event]struct{})}
	}
	return nil
}
func (m *Manager) Get(id string) *Server {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.servers[strings.ToLower(id)]
}
func (m *Manager) All() []*Server {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*Server, 0, len(m.servers))
	for _, s := range m.servers {
		result = append(result, s)
	}
	return result
}
func (m *Manager) Close() {
	for _, s := range m.All() {
		s.mu.Lock()
		s.closing = true
		if s.powerEpoch != nil {
			close(s.powerEpoch)
			s.powerEpoch = nil
		}
		if s.powerCancel != nil {
			s.powerCancel()
		}
		p := s.process
		s.mu.Unlock()
		if p != nil {
			p.Kill()
			<-p.Done
		}
	}
}
func (s *Server) ID() string    { s.mu.Lock(); defer s.mu.Unlock(); return s.settings.UUID }
func (s *Server) State() string { s.mu.Lock(); defer s.mu.Unlock(); return s.state }
func (s *Server) Subscribe() (chan Event, func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ch := make(chan Event, 128)
	s.subscribers[ch] = struct{}{}
	return ch, func() { s.mu.Lock(); delete(s.subscribers, ch); s.mu.Unlock() }
}
func (s *Server) emitLocked(event string, args ...string) {
	for ch := range s.subscribers {
		select {
		case ch <- Event{event, args}:
		default: // Disconnect lagging consumers instead of blocking the game process.
			delete(s.subscribers, ch)
			close(ch)
		}
	}
}
func (s *Server) setStateLocked(state string) { s.state = state; s.emitLocked("status", state) }
func (s *Server) log(line string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.logs = append(s.logs, line)
	s.logBytes += len(line)
	for len(s.logs) > 1000 || s.logBytes > 1<<20 {
		s.logBytes -= len(s.logs[0])
		s.logs[0] = ""
		s.logs = s.logs[1:]
	}
	s.emitLocked("console output", line)
}
func (s *Server) processLine(p *native.Process, line string) {
	s.log(line)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.process == p && s.state == "starting" && s.processPolicy.ready(line) {
		s.setStateLocked("running")
	}
}
func (s *Server) Logs(count int) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if count < 1 {
		count = 100
	}
	if count > len(s.logs) {
		count = len(s.logs)
	}
	return append([]string{}, s.logs[len(s.logs)-count:]...)
}
func (s *Server) Sync(ctx context.Context) error {
	s.configurationSync.Lock()
	defer s.configurationSync.Unlock()
	details, err := s.panel.Details(ctx, s.ID())
	if err != nil {
		return err
	}
	settings := details.Settings
	// Suspension must stop the process even if unrelated new egg settings fail validation.
	if settings.Suspended {
		s.mu.Lock()
		s.settings.Suspended = true
		s.mu.Unlock()
		if err := s.Power("kill"); err != nil {
			return err
		}
	}
	policy, err := NewFilePolicy(settings.Egg.FileDenylist)
	if err != nil {
		return err
	}
	processPolicy, err := newProcessPolicy(s.Profile, details.ProcessConfiguration)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.process != nil && !settings.Suspended {
		limits := effectiveLimits(settings, s.Profile)
		if err := s.process.UpdateLimits(limits); err != nil && !errors.Is(err, native.ErrExited) {
			return fmt.Errorf("update running job limits: %w", err)
		} else if err == nil {
			s.appliedLimits = limits
		}
	}
	s.settings = settings
	s.filePolicy = policy
	s.processPolicy = processPolicy
	return nil
}
func effectiveLimits(settings Settings, profile config.Profile) native.Limits {
	memory := settings.Build.Memory
	if memory <= 0 || (profile.MemoryMB > 0 && profile.MemoryMB < memory) {
		memory = profile.MemoryMB
	}
	return native.Limits{MemoryMB: memory, CPUPercent: settings.Build.CPU}
}

var variable = regexp.MustCompile(`\{\{([A-Z_][A-Z0-9_]*)\}\}`)

func (s *Server) spec() (native.Spec, error) {
	s.mu.Lock()
	settings := s.settings
	s.mu.Unlock()
	env := map[string]string{}
	for _, key := range s.Profile.AllowedEnvironment {
		if value, ok := settings.Environment[key]; ok {
			env[key] = fmt.Sprint(value)
		}
	}
	// Wings-derived values take precedence over egg-provided environment values.
	var allocations struct {
		Default struct {
			IP   string `json:"ip"`
			Port int    `json:"port"`
		} `json:"default"`
	}
	if len(settings.Allocations) > 0 {
		if err := json.Unmarshal(settings.Allocations, &allocations); err != nil {
			return native.Spec{}, errors.New("invalid panel allocations")
		}
	}
	env["SERVER_MEMORY"] = fmt.Sprint(settings.Build.Memory)
	env["STARTUP"] = settings.Invocation
	env["SERVER_IP"] = allocations.Default.IP
	env["SERVER_PORT"] = fmt.Sprint(allocations.Default.Port)
	if timezone := s.panel.Config.Timezone; timezone != "" {
		env["TZ"] = timezone
	} else {
		env["TZ"] = "UTC"
	}
	args := make([]string, len(s.Profile.Arguments))
	var missing string
	for i, arg := range s.Profile.Arguments {
		args[i] = variable.ReplaceAllStringFunc(arg, func(token string) string {
			key := variable.FindStringSubmatch(token)[1]
			value, ok := env[key]
			if !ok {
				missing = key
			}
			return value
		})
	}
	if missing != "" {
		return native.Spec{}, fmt.Errorf("missing allowed environment value %s", missing)
	}
	temp := filepath.Join(s.Root, ".tmp")
	if err := os.MkdirAll(temp, 0700); err != nil {
		return native.Spec{}, err
	}
	systemRoot := os.Getenv("SystemRoot")
	if systemRoot == "" {
		return native.Spec{}, errors.New("SystemRoot is not set")
	}
	clean := []string{"SystemRoot=" + systemRoot, "WINDIR=" + systemRoot, "TEMP=" + temp, "TMP=" + temp, "PATH=" + filepath.Dir(s.Profile.Executable) + ";" + filepath.Join(systemRoot, "System32")}
	for k, v := range env {
		if strings.ContainsRune(v, 0) {
			return native.Spec{}, errors.New("NUL in environment")
		}
		clean = append(clean, k+"="+v)
	}
	limits := effectiveLimits(settings, s.Profile)
	var accessRules []identity.PathRule
	if s.Profile.RunAs != nil {
		binary, err := os.Executable()
		if err != nil {
			return native.Spec{}, err
		}
		accessRules = append(accessRules, identity.PathRule{Path: binary, Forbidden: identity.WriteAccess}, identity.PathRule{Path: filepath.Dir(binary), Forbidden: identity.ReplaceChildAccess})
		for _, secret := range []string{s.panel.Config.SourcePath, s.panel.Config.TLS.Key} {
			if secret != "" {
				accessRules = append(accessRules, identity.PathRule{Path: secret, Forbidden: identity.ReadWriteAccess}, identity.PathRule{Path: filepath.Dir(secret), Forbidden: identity.ReplaceChildAccess})
			}
		}
		for id := range s.panel.Config.Profiles {
			if !strings.EqualFold(id, s.ID()) {
				accessRules = append(accessRules, identity.PathRule{Path: filepath.Join(s.panel.Config.DataDirectory, id), Forbidden: identity.ReadWriteAccess, Recursive: true})
			}
		}
	}
	return native.Spec{RunAs: s.Profile.RunAs, AccessRules: accessRules, Executable: s.Profile.Executable, Arguments: args, Environment: clean, Directory: s.Root, MemoryMB: limits.MemoryMB, CPUPercent: limits.CPUPercent}, nil
}

var ErrSuspended = errors.New("cannot start or restart a suspended server")
var ErrPowerBusy = errors.New("another power action is in progress")
var ErrPowerCancelled = errors.New("power action cancelled")
var ErrAlreadyRunning = errors.New("server is already started")
var ErrPowerQueueFull = errors.New("power waiting queue is full")

func (s *Server) Power(action string) error {
	return s.powerWithEpoch(action, nil)
}
func (s *Server) powerWithEpoch(action string, epoch chan struct{}) error {
	switch action {
	case "start", "stop", "restart", "kill":
	default:
		return errors.New("unknown power action")
	}
	s.mu.Lock()
	if s.closing {
		s.mu.Unlock()
		return errors.New("node is shutting down")
	}
	if epoch != nil && epoch != s.powerEpoch {
		s.mu.Unlock()
		return ErrPowerCancelled
	}
	if action == "kill" {
		if s.powerEpoch != nil {
			close(s.powerEpoch)
		}
		s.powerEpoch = make(chan struct{})
		if s.powerCancel != nil {
			s.powerCancel()
		}
		p := s.process
		if p != nil {
			s.setStateLocked("stopping")
		}
		s.mu.Unlock()
		if p != nil {
			return p.Kill()
		}
		return nil
	}
	if (action == "start" || action == "restart") && s.settings.Suspended {
		s.mu.Unlock()
		return ErrSuspended
	}
	if !s.power.TryLock() {
		s.mu.Unlock()
		return ErrPowerBusy
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.powerCancel = cancel
	s.mu.Unlock()
	go func() {
		defer func() { cancel(); s.mu.Lock(); s.powerCancel = nil; s.mu.Unlock(); s.power.Unlock() }()
		var err error
		switch action {
		case "start":
			err = s.startWithContext(ctx)
		case "stop":
			err = s.stopWithContext(ctx, false)
		case "restart":
			if err = s.stopWithContext(ctx, false); err == nil && ctx.Err() == nil {
				err = s.startWithContext(ctx)
			}
		}
		if err != nil && !errors.Is(err, ErrAlreadyRunning) && !errors.Is(err, context.Canceled) {
			s.log("[Windows node] " + err.Error())
		}
	}()
	return nil
}
func (s *Server) start() error { return s.startWithContext(context.Background()) }
func (s *Server) startWithContext(parent context.Context) error {
	s.configurationSync.Lock()
	defer s.configurationSync.Unlock()
	s.mu.Lock()
	if s.process != nil {
		s.mu.Unlock()
		return ErrAlreadyRunning
	}
	s.mu.Unlock()
	ctx, cancel := context.WithTimeout(parent, 20*time.Second)
	defer cancel()
	details, err := s.panel.Details(ctx, s.ID())
	if err != nil {
		return err
	}
	settings := details.Settings
	policy, err := NewFilePolicy(settings.Egg.FileDenylist)
	if err != nil {
		return err
	}
	processPolicy, err := newProcessPolicy(s.Profile, details.ProcessConfiguration)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.settings = settings
	s.filePolicy = policy
	s.processPolicy = processPolicy
	closing := s.closing
	s.mu.Unlock()
	if closing {
		return errors.New("node is shutting down")
	}
	if settings.Suspended {
		return errors.New("server is suspended")
	}
	if err := provision.Check(ctx, s.Profile); err != nil {
		return fmt.Errorf("profile preflight: %w", err)
	}
	spec, err := s.spec()
	if err != nil {
		return err
	}
	s.mu.Lock()
	if s.closing {
		s.mu.Unlock()
		return errors.New("node is shutting down")
	}
	if err := ctx.Err(); err != nil {
		s.mu.Unlock()
		return err
	}
	s.mu.Unlock()
	// Credential lookup/logon and ACL queries may block. Keep kill/shutdown able
	// to cancel the launch; native creation rechecks cancellation before resume.
	spec.Context = ctx
	p, err := native.Start(spec)
	if err != nil {
		return fmt.Errorf("start native process: %w", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closing || ctx.Err() != nil {
		p.Kill()
		p.Stdout.Close()
		if err := ctx.Err(); err != nil {
			return err
		}
		return errors.New("node is shutting down")
	}
	s.process = p
	s.appliedLimits = native.Limits{MemoryMB: spec.MemoryMB, CPUPercent: spec.CPUPercent}
	s.started = time.Now()
	s.previousSample = s.started
	s.previousCPU = 0
	s.cpu = 0
	s.setStateLocked("starting")
	go func() {
		defer p.Stdout.Close()
		reader := bufio.NewReaderSize(p.Stdout, 64*1024)
		for {
			line, _, err := reader.ReadLine()
			if len(line) > 0 {
				s.processLine(p, string(line))
			}
			if err != nil {
				return
			}
		}
	}()
	go func() {
		<-p.Done
		s.mu.Lock()
		if s.process == p {
			s.process = nil
			s.setStateLocked("offline")
			s.emitLocked("console output", fmt.Sprintf("[Windows node] process exited (%d)", p.ExitCode))
		}
		s.mu.Unlock()
	}()
	return nil
}
func (s *Server) stop(kill bool) error { return s.stopWithContext(context.Background(), kill) }
func (s *Server) stopWithContext(ctx context.Context, kill bool) error {
	s.mu.Lock()
	p := s.process
	if p == nil {
		s.mu.Unlock()
		return nil
	}
	s.setStateLocked("stopping")
	policy := s.processPolicy
	s.mu.Unlock()
	if kill || policy.forceStop {
		p.Kill()
	} else if policy.stopCommand != "" {
		s.Command(policy.stopCommand)
	} else {
		p.Kill()
	}
	timer := time.NewTimer(time.Duration(s.Profile.StopTimeoutSeconds) * time.Second)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		if err := p.Kill(); err != nil {
			return err
		}
		<-p.Done
	case <-p.Done:
	case <-timer.C:
		if err := p.Kill(); err != nil {
			return err
		}
		<-p.Done
	}
	s.mu.Lock()
	if s.process == p {
		s.process = nil
		s.setStateLocked("offline")
	}
	s.mu.Unlock()
	return nil
}
func (s *Server) Command(command string) error {
	if len(command) > 4096 || strings.ContainsAny(command, "\r\n\x00") {
		return errors.New("command must be a single line of at most 4096 bytes")
	}
	s.write.Lock()
	defer s.write.Unlock()
	s.mu.Lock()
	p := s.process
	s.mu.Unlock()
	if p == nil {
		return errors.New("server is offline")
	}
	return p.Command(command)
}
func (s *Server) HasProcess() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.process == nil {
		return false
	}
	select {
	case <-s.process.Done:
		return false
	default:
		return true
	}
}
func (s *Server) Response() map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	uptime := int64(0)
	var memory uint64
	metricsAvailable := false
	if s.process != nil {
		uptime = time.Since(s.started).Milliseconds()
		if sample, err := s.process.Stats(); err == nil {
			metricsAvailable = true
			memory = sample.MemoryBytes
			now := time.Now()
			elapsed := now.Sub(s.previousSample).Seconds()
			if elapsed >= 0.25 {
				if sample.CPUTime100ns >= s.previousCPU {
					s.cpu = float64(sample.CPUTime100ns-s.previousCPU) / 1e7 / elapsed * 100
				}
				s.previousCPU = sample.CPUTime100ns
				s.previousSample = now
			}
		}
	}
	if s.process == nil {
		s.cpu = 0
	}
	limit := effectiveLimits(s.settings, s.Profile).MemoryMB
	if s.process != nil {
		limit = s.appliedLimits.MemoryMB
	}
	stats := map[string]any{"state": s.state, "uptime": uptime, "memory_bytes": memory, "memory_limit_bytes": int64(limit) * 1024 * 1024, "cpu_absolute": s.cpu, "disk_bytes": 0, "network": map[string]int{"rx_bytes": 0, "tx_bytes": 0}, "native_metrics": map[string]any{"process_metrics_available": metricsAvailable, "scope": "root_process", "network_available": false, "disk_available": false}}
	return map[string]any{"state": s.state, "is_suspended": s.settings.Suspended, "configuration": s.settings, "utilization": stats}
}
func (s *Server) StatsJSON() string {
	b, _ := json.Marshal(s.Response()["utilization"])
	return string(b)
}
