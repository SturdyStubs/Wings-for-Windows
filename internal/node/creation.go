package node

import (
	"context"
	"errors"
	"strings"
	"wings-windows/internal/config"
)

const maxPendingCreation = 1024

var (
	ErrCreationDisabled   = errors.New("pending native creation is disabled; enable allow_pending_creation for manual local approval")
	ErrCreationStart      = errors.New("start_on_completion is unsupported for manual approval; create with automatic start disabled")
	ErrCreationConflict   = errors.New("this UUID already has an approved local profile")
	ErrCreationUnassigned = errors.New("the server is not assigned to this node")
	ErrCreationCapacity   = errors.New("pending native creation capacity reached")
	ErrManagerClosed      = errors.New("node is shutting down")
	ErrCreationUUID       = errors.New("a full server UUID is required")
)

// CreatePending acknowledges the panel's persisted record, not installation.
// Assignment is read back using node-scoped credentials. Request-supplied
// settings, scripts, paths and identities are never execution approval.
func (m *Manager) CreatePending(ctx context.Context, id string, start bool) error {
	if !config.UUID.MatchString(id) || id[14] != '4' || !strings.ContainsRune("89ab", rune(strings.ToLower(id)[19])) {
		return ErrCreationUUID
	}
	id = strings.ToLower(id)
	if !m.Config.AllowPendingCreation {
		return ErrCreationDisabled
	}
	if start {
		return ErrCreationStart
	}
	m.mu.RLock()
	closed := m.closed
	_, registered := m.servers[id]
	_, approved := m.Config.Profiles[id]
	m.mu.RUnlock()
	if closed {
		return ErrManagerClosed
	}
	if registered || approved {
		return ErrCreationConflict
	}
	// Do not trust a POST body as proof of assignment. A replay after removal
	// must fail even when an old in-memory pending entry exists.
	assigned, err := m.Panel.Servers(ctx)
	if err != nil {
		return err
	}
	var found *RemoteServer
	for _, remote := range assigned {
		if strings.EqualFold(remote.Settings.UUID, id) {
			if remote.UUID != "" && !strings.EqualFold(remote.UUID, id) {
				return ErrCreationUUID
			}
			copy := remote
			copy.Settings.UUID = id
			found = &copy
			break
		}
	}
	if found == nil {
		return ErrCreationUnassigned
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return ErrManagerClosed
	}
	if _, ok := m.servers[id]; ok {
		return ErrCreationConflict
	}
	if _, ok := m.Config.Profiles[id]; ok {
		return ErrCreationConflict
	}
	if m.pending == nil {
		m.pending = make(map[string]RemoteServer)
	}
	if _, exists := m.pending[id]; !exists && len(m.pending) >= maxPendingCreation {
		return ErrCreationCapacity
	}
	m.pending[id] = *found
	return nil
}

func pendingResponse(remote RemoteServer) map[string]any {
	return map[string]any{
		"state": "offline", "is_suspended": remote.Settings.Suspended,
		"configuration":   remote.Settings,
		"utilization":     map[string]any{"state": "offline", "uptime": 0, "memory_bytes": 0, "memory_limit_bytes": 0, "cpu_absolute": 0, "disk_bytes": 0, "network": map[string]int{"rx_bytes": 0, "tx_bytes": 0}},
		"native_creation": map[string]any{"stage": "awaiting_profile", "execution_approved": false, "installed": false},
	}
}
func (m *Manager) Pending(id string) (map[string]any, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	remote, ok := m.pending[strings.ToLower(id)]
	if !ok {
		return nil, false
	}
	return pendingResponse(remote), true
}
func (m *Manager) PendingResponses() []any {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]any, 0, len(m.pending))
	for _, remote := range m.pending {
		out = append(out, pendingResponse(remote))
	}
	return out
}
