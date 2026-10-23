// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/instance"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/runs/app"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// bindings is app.RunBindings over a map, with a counter of reads and an optional error.
type bindings struct {
	mu    sync.Mutex
	rows  map[ids.ID]domain.RunBinding
	reads int
	err   error
}

func (b *bindings) RunBinding(_ context.Context, id ids.ID) (domain.RunBinding, bool, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.reads++
	if b.err != nil {
		return domain.RunBinding{}, false, b.err
	}
	row, ok := b.rows[id]
	return row, ok, nil
}

// spyVerifier records the expectation of every Verify call and passes it to next, or answers
// with claims or err when set.
type spyVerifier struct {
	mu     sync.Mutex
	next   app.TokenVerifier
	calls  []app.VerifyInvocation
	claims *domain.InvocationClaims
	err    error
}

func (s *spyVerifier) Verify(ctx context.Context, token string, v app.VerifyInvocation) (domain.InvocationClaims, error) {
	s.mu.Lock()
	s.calls = append(s.calls, v)
	claims, err := s.claims, s.err
	s.mu.Unlock()
	if err != nil {
		return domain.InvocationClaims{}, err
	}
	if claims != nil {
		return *claims, nil
	}
	return s.next.Verify(ctx, token, v)
}

// actingForEnv is the use case over the real InvocationTokens of newInvEnv, a spy around them
// and a run of the grant g, running.
type actingForEnv struct {
	*invEnv
	uc   *app.ActingFor
	spy  *spyVerifier
	runs *bindings
	g    domain.InvocationGrant
}

func newActingForEnv(t *testing.T) *actingForEnv {
	t.Helper()
	e := &actingForEnv{invEnv: newInvEnv(t), g: invGrant()}
	e.spy = &spyVerifier{next: e.svc}
	e.runs = &bindings{rows: map[ids.ID]domain.RunBinding{e.g.RunID: {ID: e.g.RunID, AgentID: e.g.AgentID,
		TriggeredBy: e.g.TriggeredBy, ThreadID: e.g.ThreadID, State: string(domain.StateRunning)}}}
	uc, err := app.NewActingFor(e.spy, e.runs)
	if err != nil {
		t.Fatal(err)
	}
	e.uc = uc
	return e
}

// issue signs a token for g and returns the compact JWS and its jti.
func (e *actingForEnv) issue(t *testing.T, g domain.InvocationGrant) (string, ids.ID) {
	t.Helper()
	got, err := e.svc.Issue(context.Background(), g)
	if err != nil {
		t.Fatal(err)
	}
	return got.Token.Reveal(), got.JTI
}

func TestNewActingFor_RequiresDeps(t *testing.T) {
	t.Parallel()
	e := newActingForEnv(t)
	if _, err := app.NewActingFor(nil, e.runs); err == nil {
		t.Error("NewActingFor without a verifier")
	}
	if _, err := app.NewActingFor(e.spy, nil); err == nil {
		t.Error("NewActingFor without the run bindings")
	}
}

// TestResolveActingFor covers Design note 3 of M2-14 through the real verifier: Verify gets the
// agent as audience and IgnoreExpiry and no run; every verdict of Verify gives invalid without a
// read of the run; an expired token and an older token of an active run are bound.
func TestResolveActingFor(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	e := newActingForEnv(t)
	agent := e.g.AgentID
	tok, jti := e.issue(t, e.g)
	bound := domain.ActingForResult{Status: domain.ActingForBound, UserID: e.g.TriggeredBy, RunID: e.g.RunID,
		ThreadID: e.g.ThreadID, JTI: jti.String()}

	got, err := e.uc.ResolveActingFor(ctx, agent, tok)
	if err != nil || got != bound {
		t.Fatalf("fresh token of a running run: %+v, %v; want %+v", got, err, bound)
	}
	if len(e.spy.calls) != 1 || e.spy.calls[0] != (app.VerifyInvocation{Audience: agent, IgnoreExpiry: true}) {
		t.Errorf("Verify calls %+v, want one with the agent as audience and IgnoreExpiry", e.spy.calls)
	}

	// Tokens Verify refuses: a bad signature, the token of another agent, an access-token-typed
	// and a terminal-typed JWS over the same payload, an empty value, and an autocomplete token
	// without run_id. None of them reads the run.
	h, payload, parts := segments(t, tok)
	raw, _ := json.Marshal(payload)
	sig := []byte(parts[2])
	sig[3] ^= 1
	other := e.g
	other.AgentID = ids.New()
	otherTok, _ := e.issue(t, other)
	auto := e.g
	auto.TriggerKind, auto.RunID, auto.MessageID, auto.CommandID = domain.TriggerAutocomplete, ids.ID{}, ids.ID{}, ids.New()
	autoTok, _ := e.issue(t, auto)
	refused := map[string]string{
		"bad signature": parts[0] + "." + parts[1] + "." + string(sig),
		"other agent":   otherTok,
		"typ at+jwt": forge(t, e.keys.signerFor(instance.PurposeAccessToken),
			map[string]any{"alg": "EdDSA", "kid": e.keys.active[instance.PurposeAccessToken].kid, "typ": "at+jwt"}, raw),
		"typ terminal": forge(t, e.keys.signerFor(instance.PurposeTerminal),
			map[string]any{"alg": "EdDSA", "kid": e.keys.active[instance.PurposeTerminal].kid, "typ": "terminal-session+jwt"}, raw),
		"invocation typ, terminal key": forge(t, e.keys.signerFor(instance.PurposeTerminal),
			map[string]any{"alg": "EdDSA", "kid": e.keys.active[instance.PurposeTerminal].kid, "typ": h["typ"]}, raw),
		"empty":        "",
		"autocomplete": autoTok,
	}
	reads := e.runs.reads
	for name, tk := range refused {
		got, err := e.uc.ResolveActingFor(ctx, agent, tk)
		if err != nil || got != (domain.ActingForResult{Status: domain.ActingForInvalid}) {
			t.Errorf("%s: %+v, %v; want invalid without an error", name, got, err)
		}
	}
	// The autocomplete token passes Verify (it names the agent) and is refused for its missing
	// run_id before the read; every other one fails Verify.
	if e.runs.reads != reads {
		t.Errorf("refused tokens read the run %d times", e.runs.reads-reads)
	}

	// An older token of the same run stays bound after a renewal issued a newer one, also after
	// both expired (Design note 3e).
	newer, newerJTI := e.issue(t, e.g)
	got, err = e.uc.ResolveActingFor(ctx, agent, tok)
	if err != nil || got != bound {
		t.Errorf("older token of the run: %+v, %v; want %+v", got, err, bound)
	}
	e.clk.Set(e.clk.Now().Add(invCfg.TTL + invCfg.ClockLeeway + time.Hour))
	if _, err := e.svc.Verify(ctx, tok, app.VerifyInvocation{Audience: agent}); err == nil {
		t.Fatal("the token did not expire")
	}
	want := bound
	want.JTI = newerJTI.String()
	if got, err = e.uc.ResolveActingFor(ctx, agent, newer); err != nil || got != want {
		t.Errorf("expired token of an active run: %+v, %v; want %+v", got, err, want)
	}
	if got, err = e.uc.ResolveActingFor(ctx, agent, tok); err != nil || got != bound {
		t.Errorf("expired older token of an active run: %+v, %v; want %+v", got, err, bound)
	}

	// The run ended: run_inactive with the same IDs. An unknown run: invalid.
	e.runs.mu.Lock()
	row := e.runs.rows[e.g.RunID]
	row.State = string(domain.StateCompleted)
	e.runs.rows[e.g.RunID] = row
	e.runs.mu.Unlock()
	inactive := bound
	inactive.Status = domain.ActingForRunInactive
	if got, err = e.uc.ResolveActingFor(ctx, agent, tok); err != nil || got != inactive {
		t.Errorf("ended run: %+v, %v; want %+v", got, err, inactive)
	}
	e.runs.mu.Lock()
	delete(e.runs.rows, e.g.RunID)
	e.runs.mu.Unlock()
	if got, err = e.uc.ResolveActingFor(ctx, agent, tok); err != nil || got != (domain.ActingForResult{Status: domain.ActingForInvalid}) {
		t.Errorf("unknown run: %+v, %v; want invalid", got, err)
	}
}

// TestResolveActingFor_Errors: a failure of the key store (no verdict about the token) and of
// the run read come back as errors with a zero result, never as a status, so the authenticator
// answers 503 (Design note 2d); a run_id that does not parse is invalid without a read.
func TestResolveActingFor_Errors(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	e := newActingForEnv(t)
	tok, _ := e.issue(t, e.g)

	e.spy.err = errs.New(errs.CodeUnavailable, "instance: key store unavailable")
	got, err := e.uc.ResolveActingFor(ctx, e.g.AgentID, tok)
	if errs.KindOf(err) != errs.KindUnavailable || got != (domain.ActingForResult{}) {
		t.Errorf("key store failure: %+v, %v; want the unavailable error and a zero result", got, err)
	}
	e.spy.err = nil

	e.runs.err = errs.New(errs.CodeInternalError, "runs: reading the binding of a run failed")
	got, err = e.uc.ResolveActingFor(ctx, e.g.AgentID, tok)
	if errs.KindOf(err) != errs.KindInternal || got != (domain.ActingForResult{}) {
		t.Errorf("run read failure: %+v, %v; want the internal error and a zero result", got, err)
	}
	if err != nil && strings.Contains(err.Error(), strings.Split(tok, ".")[1]) {
		t.Error("the error names the token")
	}
	e.runs.err = nil

	reads := e.runs.reads
	e.spy.claims = &domain.InvocationClaims{Aud: e.g.AgentID.String(), Sub: e.g.TriggeredBy.String(), RunID: "not-a-run",
		ThreadID: e.g.ThreadID.String(), Jti: ids.New().String()}
	got, err = e.uc.ResolveActingFor(ctx, e.g.AgentID, tok)
	if err != nil || got != (domain.ActingForResult{Status: domain.ActingForInvalid}) || e.runs.reads != reads {
		t.Errorf("run_id that does not parse: %+v, %v, %d reads; want invalid without a read", got, err, e.runs.reads-reads)
	}
}
