package node

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"wings-windows/internal/config"
)

type EggSettings struct {
	ID           string   `json:"id"`
	FileDenylist []string `json:"file_denylist"`
}
type Settings struct {
	Meta           map[string]any  `json:"meta,omitempty"`
	Invocation     string          `json:"invocation,omitempty"`
	Allocations    json.RawMessage `json:"allocations,omitempty"`
	Egg            EggSettings     `json:"egg"`
	Container      json.RawMessage `json:"container,omitempty"`
	SkipEggScripts bool            `json:"skip_egg_scripts"`
	UUID           string          `json:"uuid"`
	Suspended      bool            `json:"suspended"`
	Environment    map[string]any  `json:"environment"`
	Build          struct {
		Memory int `json:"memory_limit"`
		CPU    int `json:"cpu_limit"`
	} `json:"build"`
}
type RemoteServer struct {
	UUID                 string                `json:"uuid"`
	Settings             Settings              `json:"settings"`
	ProcessConfiguration *ProcessConfiguration `json:"process_configuration"`
}
type Panel struct {
	Config config.Config
	HTTP   *http.Client
}

func NewPanel(c config.Config) *Panel {
	return &Panel{Config: c, HTTP: &http.Client{Timeout: 20 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}}
}
func (p *Panel) request(ctx context.Context, method, path string, body io.Reader, out any) error {
	req, err := http.NewRequestWithContext(ctx, method, p.Config.PanelURL+"/api/remote"+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+p.Config.TokenID+"."+p.Config.Token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Wings-Windows-Native/"+config.Version)
	resp, err := p.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("panel request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("panel returned HTTP %d", resp.StatusCode)
	}
	if out != nil {
		limit := int64(8 << 20)
		if path == "/sftp/auth" {
			limit = 16 << 10
		}
		return json.NewDecoder(io.LimitReader(resp.Body, limit)).Decode(out)
	}
	return nil
}
func (p *Panel) Servers(ctx context.Context) ([]RemoteServer, error) {
	var all []RemoteServer
	for page := 1; page <= 100; page++ {
		var result struct {
			Data []RemoteServer `json:"data"`
			Meta struct {
				LastPage int `json:"last_page"`
			} `json:"meta"`
		}
		if err := p.request(ctx, "GET", fmt.Sprintf("/servers?page=%d&per_page=100", page), nil, &result); err != nil {
			return nil, err
		}
		all = append(all, result.Data...)
		if page >= result.Meta.LastPage {
			return all, nil
		}
	}
	return nil, fmt.Errorf("panel server pagination exceeded 100 pages")
}
func (p *Panel) Server(ctx context.Context, id string) (Settings, error) {
	details, err := p.Details(ctx, id)
	return details.Settings, err
}
func (p *Panel) Details(ctx context.Context, id string) (RemoteServer, error) {
	var result RemoteServer
	err := p.request(ctx, "GET", "/servers/"+id, nil, &result)
	if err == nil && !strings.EqualFold(result.Settings.UUID, id) {
		err = fmt.Errorf("panel returned a different server UUID")
	}
	if err == nil {
		result.Settings.UUID = strings.ToLower(id)
	}
	return result, err
}
func (p *Panel) Installed(ctx context.Context, id string) error {
	return p.InstallResult(ctx, id, true, false)
}
func (p *Panel) InstallResult(ctx context.Context, id string, successful, reinstall bool) error {
	body, _ := json.Marshal(struct {
		Successful bool `json:"successful"`
		Reinstall  bool `json:"reinstall"`
	}{successful, reinstall})
	return p.request(ctx, "POST", "/servers/"+id+"/install", strings.NewReader(string(body)), nil)
}

// SFTPAuth uses the existing node-authenticated remote contract. The caller
// validates server/user/permissions before allowing a local session.
func (p *Panel) SFTPAuth(ctx context.Context, input, out any) error {
	b, e := json.Marshal(input)
	if e != nil {
		return e
	}
	if len(b) > 16384 {
		return fmt.Errorf("SFTP auth input exceeds limit")
	}
	return p.request(ctx, "POST", "/sftp/auth", bytes.NewReader(b), out)
}
