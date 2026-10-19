package networkpolicy

import (
	"context"
	"fmt"
	"github.com/tailscale/wf"
	"golang.org/x/sys/windows"
	"unsafe"
	"wings-windows/internal/identity"
)

type inspectionEngine interface {
	engine
	Rules() ([]*wf.Rule, error)
	Sublayers(...wf.ProviderID) ([]*wf.Sublayer, error)
}

// Verify temporarily installs the selected account's block and verifies its
// own rules and cleanup. It performs no traffic probes and launches no game.
func Verify(ctx context.Context, p Policy, ref *identity.Reference) VerificationReport {
	return verify(ctx, p, ref, func(opts *wf.Options) (engine, error) { return wf.New(opts) })
}

func verify(ctx context.Context, p Policy, ref *identity.Reference, open engineFactory) VerificationReport {
	if ctx == nil {
		ctx = context.Background()
	}
	report := VerificationReport{RuleIDs: []string{}}
	for _, name := range []string{"install", "readback", "session_close", "independent_cleanup_readback"} {
		report.Checks = append(report.Checks, identity.DiagnosticCheck{Name: name, Status: "not_run"})
	}
	set := func(i int, err error) {
		if err != nil {
			report.Checks[i].Status = "failed"
			report.Checks[i].Error = err.Error()
		} else {
			report.Checks[i].Status = "passed"
		}
	}
	if p.Mode != "block_connections" {
		set(0, fmt.Errorf("network verification requires block_connections mode"))
		return report
	}
	owned, err := install(ctx, p, ref, open)
	if err != nil {
		set(0, err)
		return report
	}
	l := owned.(*lease)
	defer l.Close()
	set(0, nil)
	for _, rule := range l.rules {
		report.RuleIDs = append(report.RuleIDs, windows.GUID(rule.ID).String())
	}
	report.SublayerID = windows.GUID(l.sublayer.ID).String()
	reader, ok := l.engine.(inspectionEngine)
	if !ok {
		set(1, fmt.Errorf("WFP backend cannot inspect rules"))
	} else {
		err = ctx.Err()
		if err == nil {
			err = verifyInstalled(reader, l, ref.SID)
		}
		if err == nil {
			err = ctx.Err()
		}
		set(1, err)
	}
	// Cleanup and inspection must still run after cancellation/readback failure.
	set(2, l.Close())
	observer, err := open(&wf.Options{Name: "Wings Windows network cleanup observer", Dynamic: true})
	if err != nil {
		set(3, fmt.Errorf("open cleanup observer: %w", err))
		return report
	}
	observerLease := &lease{engine: observer}
	defer observerLease.Close()
	inspection, ok := observer.(inspectionEngine)
	if !ok {
		err = fmt.Errorf("cleanup observer cannot inspect rules")
	} else {
		err = verifyAbsent(inspection, l)
	}
	if closeErr := observerLease.Close(); err == nil {
		err = closeErr
	}
	set(3, err)
	report.CleanupVerified = err == nil && report.Checks[2].Status == "passed"
	report.Passed = true
	for _, check := range report.Checks {
		if check.Status != "passed" {
			report.Passed = false
		}
	}
	return report
}

func verifyInstalled(reader inspectionEngine, l *lease, sid string) error {
	rules, err := reader.Rules()
	if err != nil {
		return fmt.Errorf("read installed rules: %w", err)
	}
	byID := map[wf.RuleID]*wf.Rule{}
	ownedCount := 0
	for _, rule := range rules {
		byID[rule.ID] = rule
		if rule.Sublayer == l.sublayer.ID {
			ownedCount++
		}
	}
	if ownedCount != len(l.rules) {
		return fmt.Errorf("unexpected rule count in owned sublayer")
	}
	for _, expected := range l.rules {
		actual := byID[expected.ID]
		if actual == nil {
			return fmt.Errorf("installed rule missing: %s", expected.ID)
		}
		if actual.Layer != expected.Layer || actual.Sublayer != expected.Sublayer || actual.Action != wf.ActionBlock || !actual.HardAction || actual.Persistent || actual.Disabled || actual.Weight != expected.Weight || len(actual.Conditions) != 1 {
			return fmt.Errorf("installed rule differs from expected block: %s", expected.ID)
		}
		match := actual.Conditions[0]
		if match.Field != wf.FieldALEUserID || match.Op != wf.MatchTypeEqual {
			return fmt.Errorf("installed rule has incorrect account scope")
		}
		sd, ok := match.Value.(*windows.SECURITY_DESCRIPTOR)
		if !ok {
			return fmt.Errorf("installed rule account descriptor missing")
		}
		dacl, _, err := sd.DACL()
		if err != nil || dacl == nil || dacl.AceCount != 1 {
			return fmt.Errorf("installed rule account descriptor differs")
		}
		var ace *windows.ACCESS_ALLOWED_ACE
		if err := windows.GetAce(dacl, 0, &ace); err != nil {
			return err
		}
		if ace.Header.AceType != 0 || ace.Header.AceFlags != 0 || ace.Mask != 1 || (*windows.SID)(unsafe.Pointer(&ace.SidStart)).String() != sid {
			return fmt.Errorf("installed rule matches the wrong account")
		}
	}
	sublayers, err := reader.Sublayers()
	if err != nil {
		return fmt.Errorf("read installed sublayer: %w", err)
	}
	for _, sub := range sublayers {
		if sub.ID == l.sublayer.ID {
			if sub.Weight != l.sublayer.Weight || sub.Persistent {
				return fmt.Errorf("installed sublayer differs")
			}
			return nil
		}
	}
	return fmt.Errorf("installed sublayer missing")
}

func verifyAbsent(reader inspectionEngine, l *lease) error {
	rules, err := reader.Rules()
	if err != nil {
		return fmt.Errorf("read cleanup rules: %w", err)
	}
	for _, actual := range rules {
		if actual.Sublayer == l.sublayer.ID {
			return fmt.Errorf("rule remains in owned sublayer after session close")
		}
		for _, owned := range l.rules {
			if actual.ID == owned.ID {
				return fmt.Errorf("owned rule remains after session close: %s", owned.ID)
			}
		}
	}
	sublayers, err := reader.Sublayers()
	if err != nil {
		return fmt.Errorf("read cleanup sublayers: %w", err)
	}
	for _, actual := range sublayers {
		if actual.ID == l.sublayer.ID {
			return fmt.Errorf("owned sublayer remains after session close")
		}
	}
	return nil
}
