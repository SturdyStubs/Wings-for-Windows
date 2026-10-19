package networkpolicy

import (
	"context"
	"fmt"
	"github.com/tailscale/wf"
	"testing"
)

type inspectionState struct {
	rules        []*wf.Rule
	subs         []*wf.Sublayer
	leaveObjects bool
	alter        bool
	readError    bool
	closeError   bool
	cancel       context.CancelFunc
}
type fakeInspection struct {
	state     *inspectionState
	owned     bool
	closed    int
	failClose bool
}

func (f *fakeInspection) AddSublayer(s *wf.Sublayer) error {
	f.state.subs = append(f.state.subs, s)
	return nil
}
func (f *fakeInspection) AddRule(r *wf.Rule) error {
	copy := *r
	f.state.rules = append(f.state.rules, &copy)
	return nil
}
func (f *fakeInspection) Rules() ([]*wf.Rule, error) {
	if f.state.cancel != nil {
		f.state.cancel()
	}
	if f.state.readError {
		return nil, fmt.Errorf("inspection denied")
	}
	if f.owned && f.state.alter && len(f.state.rules) > 0 {
		f.state.rules[len(f.state.rules)-1].Action = wf.ActionPermit
	}
	return f.state.rules, nil
}
func (f *fakeInspection) Sublayers(...wf.ProviderID) ([]*wf.Sublayer, error) {
	return f.state.subs, nil
}
func (f *fakeInspection) Close() error {
	f.closed++
	if f.owned && !f.state.leaveObjects {
		// Preserve the unrelated sentinel; only this session's objects vanish.
		f.state.rules = f.state.rules[:1]
		f.state.subs = nil
	}
	if (f.owned && f.state.closeError) || f.failClose {
		return fmt.Errorf("session close failed")
	}
	return nil
}

func TestNetworkVerificationReadbackAndCleanupFailures(t *testing.T) {
	for _, name := range []string{"success", "altered", "stale", "read denied", "close failed", "observer denied", "observer close failed", "cancelled"} {
		t.Run(name, func(t *testing.T) {
			state := &inspectionState{rules: []*wf.Rule{{Name: "unrelated sentinel"}}}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			switch name {
			case "altered":
				state.alter = true
			case "stale":
				state.leaveObjects = true
			case "read denied":
				state.readError = true
			case "close failed":
				state.closeError = true
			case "cancelled":
				state.cancel = cancel
			}
			owner := &fakeInspection{state: state, owned: true}
			observer := &fakeInspection{state: state, failClose: name == "observer close failed"}
			calls := 0
			result := verify(ctx, Policy{Mode: "block_connections"}, reference(), func(*wf.Options) (engine, error) {
				calls++
				if calls == 1 {
					return owner, nil
				}
				if name == "observer denied" {
					return nil, fmt.Errorf("observer access denied")
				}
				return observer, nil
			})
			if result.Passed != (name == "success") {
				t.Fatal("unexpected verification result", result)
			}
			if result.TrafficTested || result.GameLaunched {
				t.Fatal("verification overclaimed", result)
			}
			if owner.closed != 1 {
				t.Fatal("owner close not attempted exactly once", owner.closed)
			}
			if name != "observer denied" && observer.closed != 1 {
				t.Fatal("observer close not attempted exactly once", observer.closed)
			}
			if len(result.RuleIDs) != 4 || result.SublayerID == "" {
				t.Fatal("missing owned IDs", result)
			}
			if state.rules[0].Name != "unrelated sentinel" {
				t.Fatal("unrelated filter changed")
			}
			if name == "success" && !result.CleanupVerified {
				t.Fatal("cleanup not independently verified")
			}
			if (name == "stale" || name == "read denied" || name == "close failed" || name == "observer denied" || name == "observer close failed") && result.CleanupVerified {
				t.Fatal("cleanup falsely verified", result)
			}
			if name == "altered" && !result.CleanupVerified {
				t.Fatal("readback error prevented cleanup", result)
			}
		})
	}
}

func TestNetworkVerificationEngineOpenFailureIsNotSuccess(t *testing.T) {
	result := verify(context.Background(), Policy{Mode: "block_connections"}, reference(), func(*wf.Options) (engine, error) { return nil, fmt.Errorf("WFP access denied") })
	if result.Passed || result.CleanupVerified || result.Checks[0].Status != "failed" {
		t.Fatal(result)
	}
	for _, check := range result.Checks[1:] {
		if check.Status != "not_run" {
			t.Fatal("inspection ran without an engine", result)
		}
	}
}

func TestNetworkVerificationRejectsUnmanagedWithoutOpeningEngine(t *testing.T) {
	result := verify(context.Background(), Policy{}, nil, func(*wf.Options) (engine, error) { t.Fatal("unmanaged verification touched WFP"); return nil, nil })
	if result.Passed || result.Checks[0].Status != "failed" {
		t.Fatal(result)
	}
}
