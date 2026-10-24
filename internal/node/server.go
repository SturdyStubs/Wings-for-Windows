package node

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
	"wings-windows/internal/config"
	"wings-windows/internal/consolelog"
	"wings-windows/internal/diskusage"
	"wings-windows/internal/eggconfig"
	"wings-windows/internal/logtail"
	"wings-windows/internal/native"
	"wings-windows/internal/provision"
)

type Event struct {
	Event string   `json:"event"`
	Args  []string `json:"args"`
}
type recoveryBudget interface {
	Save([]time.Time) error
	Close(context.Context) error
}
type desiredPersistence interface {
	Save(bool) error
	Close(context.Context) error
}
type installPersistence interface {
	Save(consolelog.InstallRecord) error
	Close(context.Context) error
}
type Server struct {
	disk                 *diskusage.Meter
	installStore         installPersistence
	installRecord        consolelog.InstallRecord
	installStorageError  string
	installing           bool
	installerProcess     *native.Process
	installDone          chan struct{}
	installStage         string
	installSuccessful    bool
	installCallbackError string
	logFileActive        bool
	logFileError         string
	desiredWrite         sync.Mutex
	desiredStore         desiredPersistence
	desiredRunning       bool
	desiredRestored      bool
	desiredGeneration    uint64
	desiredStorageError  string
	startupSuppressed    bool
	mu                   sync.Mutex
	configurationSync    sync.Mutex
	power                sync.Mutex
	powerCancel          context.CancelFunc
	powerEpoch           chan struct{}
	pendingPower         int
	filePolicy           *FilePolicy
	processPolicy        *processPolicy
	appliedLimits        native.Limits
	write                sync.Mutex
	settings             Settings
	Profile              config.Profile
	Root                 string
	panel                *Panel
	process              *native.Process
	state                string
	started              time.Time
	previousSample       time.Time
	previousCPU          uint64
	cpu                  float64
	logs                 []string
	logBytes             int
	consoleStore         *consolelog.Store
	outputWG             sync.WaitGroup
	subscribers          map[chan Event]struct{}
	closing              bool
	recoveryStopped      bool
	recovery             *recoveryAttempt
	recoveryHistory      []time.Time
	recoveryStore        recoveryBudget
	recoveryRestored     int
	recoveryStorageError string
}
type Manager struct {
	diskSlots   chan struct{}
	closed      bool
	pending     map[string]RemoteServer
	restoreOnce sync.Once
	restoreGate chan struct{}
	Config      config.Config
	Panel       *Panel
	mu          sync.RWMutex
	servers     map[string]*Server
}

func NewManager(c config.Config) *Manager {
	return &Manager{diskSlots: make(chan struct{}, 2), pending: make(map[string]RemoteServer), restoreGate: make(chan struct{}, 2), Config: c, Panel: NewPanel(c), servers: make(map[string]*Server)}
}
func (m *Manager) Sync(ctx context.Context) error {
	remote, err := m.Panel.Servers(ctx)
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return ErrManagerClosed
	}
	pending := make(map[string]RemoteServer)
	for _, r := range remote {
		id := strings.ToLower(r.Settings.UUID)
		if !config.UUID.MatchString(id) || (r.UUID != "" && !strings.EqualFold(r.UUID, id)) {
			return errors.New("invalid panel server UUID")
		}
		profile, ok := m.Config.Profiles[id]
		if !ok {
			if m.Config.AllowPendingCreation {
				if len(pending) >= maxPendingCreation {
					return ErrCreationCapacity
				}
				pending[id] = r
			}
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
		s := &Server{settings: r.Settings, filePolicy: policy, processPolicy: processPolicy, powerEpoch: make(chan struct{}), Profile: profile, Root: root, panel: m.Panel, state: "offline", subscribers: make(map[chan Event]struct{})}
		if options := m.Config.ConsoleLogs; options != nil {
			store, history, err := consolelog.Open(*options, id)
			if err != nil {
				return fmt.Errorf("server %s console log storage: %w", id, err)
			}
			s.consoleStore = store
			s.logs = history
			for _, line := range history {
				s.logBytes += len(line)
			}
			if profile.CrashRecovery != nil && profile.CrashRecovery.PersistHistory {
				budget, attempts, err := consolelog.OpenRecovery(*options, id)
				if err != nil {
					closeCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
					store.Close(closeCtx)
					cancel()
					return fmt.Errorf("server %s recovery storage: %w", id, err)
				}
				s.recoveryStore = budget
				s.recoveryHistory = attempts
				s.recoveryRestored = len(attempts)
				s.pruneRecoveryLocked(time.Now())
			}
		}

		if profile.RestoreRunning {
			if m.Config.ConsoleLogs == nil {
				return errors.New("restore_running requires private storage")
			}
			intent, running, err := consolelog.OpenIntent(*m.Config.ConsoleLogs, id)
			if err != nil {
				closeCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				if s.recoveryStore != nil {
					s.recoveryStore.Close(closeCtx)
				}
				if s.consoleStore != nil {
					s.consoleStore.Close(closeCtx)
				}
				cancel()
				return fmt.Errorf("server %s desired state storage could not be opened", id)
			}
			s.desiredStore = intent
			s.desiredRunning = running
			s.desiredRestored = running
		}
		if profile.Installer != nil && profile.Installer.PersistState {
			if m.Config.ConsoleLogs == nil {
				return errors.New("persistent installer requires private storage")
			}
			receipt, record, err := consolelog.OpenInstallState(*m.Config.ConsoleLogs, id)
			if err != nil {
				closeCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				if s.desiredStore != nil {
					s.desiredStore.Close(closeCtx)
				}
				if s.recoveryStore != nil {
					s.recoveryStore.Close(closeCtx)
				}
				if s.consoleStore != nil {
					s.consoleStore.Close(closeCtx)
				}
				cancel()
				return fmt.Errorf("server %s installation state could not be opened", id)
			}
			s.installStore = receipt
			s.installRecord = record
			s.restoreInstallRecord()
		}
		s.disk = diskusage.New(root, m.diskSlots)
		m.servers[id] = s
	}
	m.pending = pending
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
	m.mu.Lock()
	m.closed = true
	m.mu.Unlock()
	for _, s := range m.All() {
		s.mu.Lock()
		s.closing = true
		s.cancelRecoveryLocked(true)
		if s.powerEpoch != nil {
			close(s.powerEpoch)
			s.powerEpoch = nil
		}
		if s.powerCancel != nil {
			s.powerCancel()
		}
		p := s.process
		installer := s.installerProcess
		installDone := s.installDone
		s.mu.Unlock()
		if s.disk != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			if err := s.disk.Close(ctx); err != nil {
				log.Printf("disk scan shutdown did not drain")
			}
			cancel()
		}
		if installer != nil {
			installer.Kill()
			<-installer.Done
		}
		if installDone != nil {
			select {
			case <-installDone:
			case <-time.After(6 * time.Second):
				log.Printf("server installer shutdown did not drain")
			}
		}
		if p != nil {
			p.Kill()
			<-p.Done
		}
		if s.installStore != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			if err := s.installStore.Close(ctx); err != nil {
				log.Printf("server installation state close did not complete")
			}
			cancel()
		}
		if s.desiredStore != nil {
			drained := make(chan struct{})
			go func() { s.outputWG.Wait(); close(drained) }()
			select {
			case <-drained:
			case <-time.After(2 * time.Second):
				log.Printf("server %s desired-state exit drain timed out", s.ID())
			}
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			if err := s.desiredStore.Close(ctx); err != nil {
				log.Printf("server %s desired state close did not complete successfully", s.ID())
			}
			cancel()
		}
		if s.recoveryStore != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			if err := s.recoveryStore.Close(ctx); err != nil {
				log.Printf("server %s recovery storage close did not complete successfully", s.ID())
			}
			cancel()
		}
		if s.consoleStore != nil {
			drained := make(chan struct{})
			go func() { s.outputWG.Wait(); close(drained) }()
			select {
			case <-drained:
			case <-time.After(2 * time.Second):
				log.Printf("server %s console drain timed out", s.ID())
			}
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			if err := s.consoleStore.Close(ctx); err != nil {
				log.Printf("server %s console flush: %v", s.ID(), err)
			}
			cancel()
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
	s.logLocked(line)
}
func (s *Server) logLocked(line string) {
	if s.consoleStore != nil {
		s.consoleStore.Enqueue(line)
	}
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
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.process != p || s.closing {
		return
	}
	s.logLocked(line)
	if (s.Profile.LogFile == nil || !s.Profile.LogFile.Readiness) && s.state == "starting" && s.processPolicy.ready(line) {
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
	return s.specForProfile(s.Profile)
}
func (s *Server) specForProfile(profile config.Profile) (native.Spec, error) {
	s.mu.Lock()
	settings := s.settings
	s.mu.Unlock()
	env := map[string]string{}
	for _, key := range profile.AllowedEnvironment {
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
	args := make([]string, len(profile.Arguments))
	var missing string
	for i, arg := range profile.Arguments {
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
	clean := []string{"SystemRoot=" + systemRoot, "WINDIR=" + systemRoot, "TEMP=" + temp, "TMP=" + temp, "PATH=" + filepath.Dir(profile.Executable) + ";" + filepath.Join(systemRoot, "System32")}
	for k, v := range env {
		if strings.ContainsRune(v, 0) {
			return native.Spec{}, errors.New("NUL in environment")
		}
		clean = append(clean, k+"="+v)
	}
	limits := effectiveLimits(settings, profile)
	accessRules, err := s.identityAccessRules()
	if err != nil {
		return native.Spec{}, err
	}
	return native.Spec{NetworkPolicy: profile.NetworkPolicy, RunAs: profile.RunAs, AccessRules: accessRules, Executable: profile.Executable, Arguments: args, Environment: clean, Directory: s.Root, TempDirectory: temp, RequiredFiles: append([]string(nil), profile.Requirements.RequiredFiles...), MemoryMB: limits.MemoryMB, CPUPercent: limits.CPUPercent}, nil
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
	if (action == "start" || action == "restart") && (s.installStorageError != "" || (s.installStore != nil && s.installRecord.Phase == "running")) {
		s.mu.Unlock()
		return ErrInstallUnresolved
	}
	if epoch != nil && epoch != s.powerEpoch {
		s.mu.Unlock()
		return ErrPowerCancelled
	}
	if (action == "start" || action == "restart") && s.settings.Suspended {
		s.mu.Unlock()
		return ErrSuspended
	}
	s.startupSuppressed = true
	if epoch == nil {
		s.cancelRecoveryLocked(action == "stop" || action == "kill")
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
		installer := s.installerProcess
		if p != nil {
			s.setStateLocked("stopping")
		}
		s.mu.Unlock()
		var killErr error
		if installer != nil {
			killErr = installer.Kill()
		}
		if p != nil {
			killErr = p.Kill()
		}
		return errors.Join(killErr, s.setDesired(context.Background(), false, nil))
	}
	if s.installing {
		s.mu.Unlock()
		return ErrInstallBusy
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
		desiredErr := s.setDesired(ctx, action != "stop", nil)
		if desiredErr != nil && action != "stop" {
			if !errors.Is(desiredErr, context.Canceled) {
				s.log("[Windows node] start blocked: desired intent could not be persisted")
			}
			return
		}
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
		err = errors.Join(err, desiredErr)
		if err != nil && !errors.Is(err, ErrAlreadyRunning) && !errors.Is(err, context.Canceled) {
			s.log("[Windows node] " + err.Error())
		}
	}()
	return nil
}
func (s *Server) start() error {
	s.mu.Lock()
	s.cancelRecoveryLocked(false)
	s.mu.Unlock()
	if err := s.setDesired(context.Background(), true, nil); err != nil {
		return err
	}
	return s.startWithContext(context.Background())
}
func (s *Server) startWithContext(parent context.Context) error {
	s.mu.Lock()
	if s.installStorageError != "" || (s.installStore != nil && s.installRecord.Phase == "running") {
		s.mu.Unlock()
		return ErrInstallUnresolved
	}
	installing := s.installing
	s.mu.Unlock()
	if installing {
		return ErrInstallBusy
	}
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
	if len(processPolicy.configs) > 0 {
		protected := []string{s.panel.Config.SourcePath, s.panel.Config.TLS.Cert, s.panel.Config.TLS.Key}
		for _, profile := range s.panel.Config.Profiles {
			protected = append(protected, profile.Executable)
			protected = append(protected, profile.Requirements.RequiredFiles...)
			if profile.Installer != nil {
				protected = append(protected, profile.Installer.Executable)
				protected = append(protected, profile.Installer.Requirements.RequiredFiles...)
			}
		}
		if err := eggconfig.Apply(ctx, s.Root, processPolicy.configs, protected); err != nil {
			return err
		}
		s.log("[Windows node] approved egg configuration files updated")
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
	var logReader *logtail.Reader
	if s.Profile.LogFile != nil {
		if policy.Denied(s.Profile.LogFile.Path, false) {
			return errors.New("approved log file is denied by the egg file policy")
		}
		protected := []string{s.panel.Config.SourcePath, s.panel.Config.TLS.Cert, s.panel.Config.TLS.Key}
		for _, profile := range s.panel.Config.Profiles {
			protected = append(protected, profile.Executable)
			protected = append(protected, profile.Requirements.RequiredFiles...)
			if profile.Installer != nil {
				protected = append(protected, profile.Installer.Executable)
				protected = append(protected, profile.Installer.Requirements.RequiredFiles...)
			}
		}
		logReader, err = logtail.Open(ctx, s.Root, s.Profile.LogFile.Path, protected)
		if err != nil {
			return errors.New("log-file preflight failed; inspect approved path and permissions")
		}
		defer func() {
			if logReader != nil {
				logReader.Close()
			}
		}()
	}
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
	s.logFileError = ""
	s.logFileActive = logReader != nil
	s.appliedLimits = native.Limits{MemoryMB: spec.MemoryMB, CPUPercent: spec.CPUPercent}
	s.started = time.Now()
	s.previousSample = s.started
	s.previousCPU = 0
	s.cpu = 0
	s.setStateLocked("starting")
	s.outputWG.Add(2)
	if logReader != nil {
		s.outputWG.Add(1)
		go s.readLogFile(p, logReader)
		logReader = nil // ownership transferred to the process-scoped reader
	}
	go func() {
		defer s.outputWG.Done()
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
		defer s.outputWG.Done()
		<-p.Done
		var cleanGeneration *uint64
		s.mu.Lock()
		if p.CleanupError != nil {
			s.logLocked("[Windows node] cleanup warning: " + p.CleanupError.Error())
		}
		if s.process == p {
			cleanIntent := !s.closing && (s.state == "starting" || s.state == "running") && (p.ExitCode == 0 || p.CleanupError != nil)
			generation := s.desiredGeneration
			unexpected := (s.state == "starting" || s.state == "running") && p.ExitCode != 0 && p.CleanupError == nil
			s.process = nil
			s.setStateLocked("offline")
			s.logLocked(fmt.Sprintf("[Windows node] process exited (%d)", p.ExitCode))
			if cleanIntent {
				cleanGeneration = &generation
			}
			if unexpected {
				s.scheduleRecoveryLocked()
			}
		}
		s.mu.Unlock()
		if cleanGeneration != nil {
			s.clearDesiredAfterExit(*cleanGeneration)
		}
	}()
	return nil
}
func (s *Server) stop(kill bool) error {
	s.mu.Lock()
	s.cancelRecoveryLocked(true)
	s.mu.Unlock()
	s.mu.Lock()
	s.startupSuppressed = true
	s.mu.Unlock()
	desiredErr := s.setDesired(context.Background(), false, nil)
	return errors.Join(s.stopWithContext(context.Background(), kill), desiredErr)
}
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
		if err := s.deliverCommand(ctx, policy.stopCommand); err != nil {
			s.log("[Windows node] stop command delivery failed; waiting for exit before forced stop")
		}
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
	return s.commandWithContext(context.Background(), command)
}
func (s *Server) commandWithContext(ctx context.Context, command string) error {
	s.mu.Lock()
	installing := s.installing
	s.mu.Unlock()
	if installing {
		return ErrInstallBusy
	}
	return s.deliverCommand(ctx, command)
}

// Lifecycle-owned shutdown may deliver its approved stop command while the
// installer holds the power lock; public commands remain blocked above.
func (s *Server) deliverCommand(ctx context.Context, command string) error {
	if len(command) > 4096 || strings.ContainsAny(command, "\r\n\x00") {
		return errors.New("command must be a single line of at most 4096 bytes")
	}
	if s.Profile.SourceRCON != nil {
		if !s.write.TryLock() {
			return errors.New("RCON command is busy; no command was sent")
		}
	} else {
		s.write.Lock()
	}
	defer s.write.Unlock()
	s.mu.Lock()
	p := s.process
	s.mu.Unlock()
	if p == nil {
		return errors.New("server is offline")
	}
	if s.Profile.SourceRCON != nil {
		return s.rconCommand(ctx, p, command)
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
	disk := diskusage.Status{Scope: "data_root_logical_files", Stale: true}
	if s.disk != nil {
		disk = s.disk.Snapshot()
	}
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
	stats := map[string]any{"state": s.state, "uptime": uptime, "memory_bytes": memory, "memory_limit_bytes": int64(limit) * 1024 * 1024, "cpu_absolute": s.cpu, "disk_bytes": disk.Bytes, "network": map[string]int{"rx_bytes": 0, "tx_bytes": 0}, "native_metrics": map[string]any{"process_metrics_available": metricsAvailable, "scope": "root_process", "network_available": false, "disk_available": disk.Available, "disk": disk}}
	s.pruneRecoveryLocked(time.Now())
	recovery := map[string]any{"enabled": s.Profile.CrashRecovery != nil && s.Profile.CrashRecovery.Enabled, "pending": s.recovery != nil, "attempts_in_window": len(s.recoveryHistory)}
	recovery["persistent_history"] = s.recoveryStore != nil
	recovery["restored_attempts"] = s.recoveryRestored
	if s.recoveryStorageError != "" {
		recovery["storage_error"] = s.recoveryStorageError
	}
	if policy := s.Profile.CrashRecovery; policy != nil {
		recovery["max_restarts"] = policy.MaxRestarts
		recovery["window_seconds"] = policy.WindowSeconds
	}
	if s.recovery != nil {
		recovery["next_attempt_at"] = s.recovery.next.UTC().Format(time.RFC3339Nano)
	}
	consoleStatus := consolelog.Status{}
	if s.consoleStore != nil {
		consoleStatus = s.consoleStore.Status()
	}
	return map[string]any{"state": s.state, "is_suspended": s.settings.Suspended, "configuration": s.settings, "utilization": stats, "native_recovery": recovery, "native_restore": s.restoreStatusLocked(), "native_console_logs": consoleStatus, "native_install": s.installStatusLocked(), "native_log_file": map[string]any{"enabled": s.Profile.LogFile != nil, "active": s.process != nil && s.logFileActive, "error": s.logFileError}}
}
func (s *Server) StatsJSON() string {
	b, _ := json.Marshal(s.Response()["utilization"])
	return string(b)
}
