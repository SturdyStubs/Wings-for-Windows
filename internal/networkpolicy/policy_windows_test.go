package networkpolicy

import (
	"context"
	"errors"
	"fmt"
	"github.com/tailscale/wf"
	"golang.org/x/sys/windows"
	"testing"
	"unsafe"
	"wings-windows/internal/identity"
)

type fakeEngine struct {
	rules      []*wf.Rule
	sub        *wf.Sublayer
	failAt     int
	closeError error
	closed     int
	afterAdd   func()
}

func (f *fakeEngine) AddSublayer(s *wf.Sublayer) error {
	if f.failAt == -1 {
		return fmt.Errorf("sublayer denied")
	}
	f.sub = s
	return nil
}
func (f *fakeEngine) AddRule(r *wf.Rule) error {
	if f.failAt == len(f.rules)+1 {
		return fmt.Errorf("filter denied")
	}
	f.rules = append(f.rules, r)
	if f.afterAdd != nil {
		f.afterAdd()
	}
	return nil
}
func (f *fakeEngine) Close() error { f.closed++; return f.closeError }
func reference() *identity.Reference {
	return &identity.Reference{SID: "S-1-5-21-1-2-3-1001", CredentialTarget: "WingsWindows/11111111-1111-4111-8111-111111111111"}
}

func TestBlockInstallRollbackAndLifetime(t *testing.T) {
	for _, fail := range []int{-1, 1, 2, 3, 4, 0} {
		t.Run(fmt.Sprint(fail), func(t *testing.T) {
			fake := &fakeEngine{failAt: fail}
			lease, err := install(context.Background(), Policy{Mode: "block_connections"}, reference(), func(opts *wf.Options) (engine, error) {
				if !opts.Dynamic {
					t.Fatal("non-dynamic session")
				}
				return fake, nil
			})
			if fail != 0 {
				if err == nil || lease != nil || fake.closed != 1 {
					t.Fatal("partial install not rolled back", lease, err, fake.closed)
				}
				return
			}
			if err != nil || lease == nil || len(fake.rules) != 4 || fake.closed != 0 {
				t.Fatal("incomplete block install", err)
			}
			if err := lease.Close(); err != nil {
				t.Fatal(err)
			}
			lease.Close()
			if fake.closed != 1 {
				t.Fatal("session not closed exactly once")
			}
		})
	}
}

func TestCancellationAndRollbackFailure(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := install(ctx, Policy{Mode: "block_connections"}, reference(), func(*wf.Options) (engine, error) { t.Fatal("opened WFP after cancellation"); return nil, nil }); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	ctx, cancel = context.WithCancel(context.Background())
	fake := &fakeEngine{afterAdd: cancel, closeError: fmt.Errorf("cleanup failed")}
	lease, err := install(ctx, Policy{Mode: "block_connections"}, reference(), func(*wf.Options) (engine, error) { return fake, nil })
	if lease != nil || err == nil || !errors.Is(err, context.Canceled) || fake.closed != 1 {
		t.Fatal("cancelled install not rolled back", err)
	}
}

func TestRuleContractIsSIDScopedBlockForBothIPFamiliesAndDirections(t *testing.T) {
	rules, sub, err := blockRules(reference().SID)
	if err != nil {
		t.Fatal(err)
	}
	// Independently pinned public WFP layer GUIDs, not this builder's output.
	layers := map[string]bool{"{C38D57D1-05A7-4C33-904F-7FBCEEE60E82}": false, "{4A72393B-319F-44BC-84C3-BA54DCB3B6B4}": false, "{E1CD9FE7-F4B5-4273-96C0-592E487B8650}": false, "{A3B42C97-9F04-4672-B87E-CEE9C483257F}": false}
	for _, rule := range rules {
		key := windows.GUID(rule.Layer).String()
		if seen, ok := layers[key]; !ok || seen {
			t.Fatal("wrong or duplicate authorization layer", key)
		}
		layers[key] = true
		if rule.Action != wf.ActionBlock || !rule.HardAction || rule.Persistent || rule.Sublayer != sub.ID || len(rule.Conditions) != 1 {
			t.Fatal("unsafe block rule")
		}
		condition := rule.Conditions[0]
		if windows.GUID(condition.Field).String() != "{AF043A0A-B34D-4F86-979C-C90371AF6E66}" || condition.Op != wf.MatchTypeEqual {
			t.Fatal("not scoped to ALE user ID")
		}
		sd, ok := condition.Value.(*windows.SECURITY_DESCRIPTOR)
		if !ok {
			t.Fatal("missing user descriptor")
		}
		dacl, _, err := sd.DACL()
		if err != nil || dacl.AceCount != 1 {
			t.Fatal("bad account matcher")
		}
		var ace *windows.ACCESS_ALLOWED_ACE
		if err := windows.GetAce(dacl, 0, &ace); err != nil {
			t.Fatal(err)
		}
		if ace.Mask != 1 {
			t.Fatal("missing FWP_ACTRL_MATCH_FILTER")
		}
		if (*windows.SID)(unsafe.Pointer(&ace.SidStart)).String() != reference().SID {
			t.Fatal("block matcher targets the wrong account")
		}
	}
	for key, seen := range layers {
		if !seen {
			t.Fatal("missing authorization layer", key)
		}
	}
}

func TestUnmanagedNeverOpensEngineAndInvalidPolicyFails(t *testing.T) {
	for _, mode := range []string{"", "unmanaged"} {
		lease, err := install(context.Background(), Policy{Mode: mode}, nil, func(*wf.Options) (engine, error) { t.Fatal("unmanaged touched WFP"); return nil, nil })
		if lease != nil || err != nil {
			t.Fatal(err)
		}
	}
	if err := (Policy{Mode: "block_connections"}).Validate(nil); err == nil {
		t.Fatal("shared-account block accepted")
	}
	if err := (Policy{Mode: "allow_everything_else"}).Validate(reference()); err == nil {
		t.Fatal("unknown policy accepted")
	}
}
