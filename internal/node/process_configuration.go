package node

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"wings-windows/internal/config"
)

type ProcessConfiguration struct {
	Startup struct {
		Done            []string `json:"done"`
		StripANSI       bool     `json:"strip_ansi"`
		UserInteraction []string `json:"user_interaction"`
	} `json:"startup"`
	Stop struct {
		Type  string `json:"type"`
		Value string `json:"value"`
	} `json:"stop"`
	Configs []json.RawMessage `json:"configs"`
}
type outputMatcher struct {
	literal string
	regex   *regexp.Regexp
}
type processPolicy struct {
	matchers    []outputMatcher
	stripANSI   bool
	stopCommand string
	forceStop   bool
}

var ansiEscape = regexp.MustCompile("[\u001B\u009B][[\\]()#;?]*(?:(?:(?:[a-zA-Z\\d]*(?:;[a-zA-Z\\d]*)*)?\u0007)|(?:(?:\\d{1,4}(?:;\\d{0,4})*)?[\\dA-PRZcf-ntqry=><~]))")

func newProcessPolicy(profile config.Profile, remote *ProcessConfiguration) (*processPolicy, error) {
	p := &processPolicy{stopCommand: profile.StopCommand, forceStop: profile.StopCommand == ""}
	if !profile.UseEggProcessConfiguration {
		r, err := regexp.Compile(profile.ReadyPattern)
		if err != nil {
			return nil, err
		}
		p.matchers = []outputMatcher{{regex: r}}
		return p, nil
	}
	if remote == nil {
		return nil, fmt.Errorf("panel process_configuration is required by this profile")
	}
	if len(remote.Configs) > 0 {
		return nil, fmt.Errorf("egg config-file rewriting is not supported yet; use a local execution profile")
	}
	if len(remote.Startup.Done) == 0 || len(remote.Startup.Done) > 64 {
		return nil, fmt.Errorf("egg startup.done must contain 1..64 matchers")
	}
	p.stripANSI = remote.Startup.StripANSI
	for _, line := range remote.Startup.Done {
		if len(line) > 8192 {
			return nil, fmt.Errorf("egg startup matcher exceeds 8192 bytes")
		}
		m := outputMatcher{literal: line}
		if strings.HasPrefix(line, "regex:") && len(line) > 6 {
			m.literal = strings.TrimPrefix(line, "regex:")
			m.regex, _ = regexp.Compile(m.literal)
		}
		p.matchers = append(p.matchers, m)
	}
	switch remote.Stop.Type {
	case "command":
		if remote.Stop.Value == "" || len(remote.Stop.Value) > 4096 || strings.ContainsAny(remote.Stop.Value, "\r\n\x00") {
			return nil, fmt.Errorf("egg stop command must be a nonempty single console line")
		}
		p.stopCommand = remote.Stop.Value
		p.forceStop = false
	case "signal":
		if strings.ToUpper(remote.Stop.Value) != "SIGKILL" {
			return nil, fmt.Errorf("egg stop signal %q requires a Windows control-event adapter; use a local profile or command stop", remote.Stop.Value)
		}
		p.stopCommand = ""
		p.forceStop = true
	default:
		return nil, fmt.Errorf("egg stop type %q has no native Windows mapping; use a local profile", remote.Stop.Type)
	}
	return p, nil
}
func (p *processPolicy) ready(line string) bool {
	if p == nil {
		return false
	}
	if p.stripANSI {
		line = ansiEscape.ReplaceAllString(line, "")
	}
	for _, m := range p.matchers {
		if m.regex != nil {
			if m.regex.MatchString(line) {
				return true
			}
		} else if strings.Contains(line, m.literal) {
			return true
		}
	}
	return false
}
