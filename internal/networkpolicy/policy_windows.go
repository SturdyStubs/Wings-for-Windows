package networkpolicy

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"github.com/tailscale/wf"
	"golang.org/x/sys/windows"
	"io"
	"sync"
	"time"
	"wings-windows/internal/identity"
)

type engine interface {
	AddSublayer(*wf.Sublayer) error
	AddRule(*wf.Rule) error
	Close() error
}
type engineFactory func(*wf.Options) (engine, error)
type lease struct {
	once     sync.Once
	engine   engine
	err      error
	rules    []*wf.Rule
	sublayer *wf.Sublayer
}

func (l *lease) Close() error { l.once.Do(func() { l.err = l.engine.Close() }); return l.err }

func Install(ctx context.Context, p Policy, ref *identity.Reference) (io.Closer, error) {
	return install(ctx, p, ref, func(opts *wf.Options) (engine, error) { return wf.New(opts) })
}

func install(ctx context.Context, p Policy, ref *identity.Reference, open engineFactory) (io.Closer, error) {
	if err := p.Validate(ref); err != nil {
		return nil, err
	}
	if p.Mode != "block_connections" {
		return nil, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return nil, err
	}
	if user.User.Sid.String() == ref.SID {
		return nil, fmt.Errorf("network policy must not target the daemon account")
	}
	rules, sub, err := blockRules(ref.SID)
	if err != nil {
		return nil, err
	}
	session, err := open(&wf.Options{Name: "Wings Windows identity network policy", Dynamic: true, TransactionStartTimeout: 2 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("open WFP dynamic session: %w", err)
	}
	l := &lease{engine: session, rules: rules, sublayer: sub}
	rollback := func(err error) (io.Closer, error) {
		if closeErr := l.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("WFP rollback: %w", closeErr))
		}
		return nil, err
	}
	if err := session.AddSublayer(sub); err != nil {
		return rollback(fmt.Errorf("add WFP sublayer: %w", err))
	}
	for _, rule := range rules {
		if err := ctx.Err(); err != nil {
			return rollback(err)
		}
		if err := session.AddRule(rule); err != nil {
			return rollback(fmt.Errorf("add WFP %s block: %w", rule.Layer, err))
		}
	}
	if err := ctx.Err(); err != nil {
		return rollback(err)
	}
	return l, nil
}

func randomGUID() (windows.GUID, error) {
	var b [16]byte
	rand.Read(b[:])
	b[6] = (b[6] & 15) | 64
	b[8] = (b[8] & 63) | 128
	return windows.GUIDFromString(fmt.Sprintf("{%x-%x-%x-%x-%x}", b[:4], b[4:6], b[6:8], b[8:10], b[10:]))
}

func blockRules(sid string) ([]*wf.Rule, *wf.Sublayer, error) {
	sd, err := windows.SecurityDescriptorFromString("D:(A;;CC;;;" + sid + ")")
	if err != nil {
		return nil, nil, err
	}
	id, err := randomGUID()
	if err != nil {
		return nil, nil, err
	}
	sub := &wf.Sublayer{ID: wf.SublayerID(id), Name: "Wings Windows account block", Weight: 0xffff}
	var rules []*wf.Rule
	for _, layer := range []wf.LayerID{wf.LayerALEAuthConnectV4, wf.LayerALEAuthConnectV6, wf.LayerALEAuthRecvAcceptV4, wf.LayerALEAuthRecvAcceptV6} {
		id, err := randomGUID()
		if err != nil {
			return nil, nil, err
		}
		rules = append(rules, &wf.Rule{ID: wf.RuleID(id), Name: "Wings Windows block connections", Layer: layer, Sublayer: sub.ID, Weight: 1, Action: wf.ActionBlock, HardAction: true, Conditions: []*wf.Match{{Field: wf.FieldALEUserID, Op: wf.MatchTypeEqual, Value: sd}}})
	}
	return rules, sub, nil
}
