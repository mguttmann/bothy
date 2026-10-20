// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/instance"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/runs/app"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// invCfg stands for agent.invocation_token_ttl and auth.clock_leeway; the TTL has a fraction of a
// second so the truncation to whole seconds shows.
var invCfg = app.InvocationTokenConfig{TTL: 90*time.Second + 400*time.Millisecond, ClockLeeway: 30 * time.Second}

// fakeKey is one Ed25519 key of fakeKeys.
type fakeKey struct {
	kid     string
	purpose instance.Purpose
	priv    ed25519.PrivateKey
}

// fakeKeys is instance.Keys for the invocation service: one active key per purpose, a set of
// published keys and a record of every requested purpose.
type fakeKeys struct {
	mu        sync.Mutex
	active    map[instance.Purpose]*fakeKey
	published map[string]*fakeKey
	requested []instance.Purpose
	signs     int
	alg       string
	signerErr error
}

func newFakeKeys(t *testing.T) *fakeKeys {
	t.Helper()
	k := &fakeKeys{active: map[instance.Purpose]*fakeKey{}, published: map[string]*fakeKey{}, alg: instance.AlgEdDSA}
	for _, p := range []instance.Purpose{instance.PurposeInvocation, instance.PurposeAccessToken, instance.PurposeTerminal} {
		k.rotate(t, p)
	}
	return k
}

// rotate makes a new active key of p; the old one stays published.
func (k *fakeKeys) rotate(t *testing.T, p instance.Purpose) *fakeKey {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	fk := &fakeKey{kid: fmt.Sprintf("%s-%x", p, priv.Public().(ed25519.PublicKey)[:6]), purpose: p, priv: priv}
	k.mu.Lock()
	k.active[p], k.published[fk.kid] = fk, fk
	k.mu.Unlock()
	return fk
}

// fakeSigner is the instance.Signer of a fakeKey.
type fakeSigner struct {
	k    *fakeKeys
	key  *fakeKey
	algo string
}

func (s fakeSigner) KeyID() string { return s.key.kid }
func (s fakeSigner) Alg() string   { return s.algo }
func (s fakeSigner) Sign(_ context.Context, in []byte) ([]byte, error) {
	s.k.mu.Lock()
	s.k.signs++
	s.k.mu.Unlock()
	return ed25519.Sign(s.key.priv, in), nil
}

func (k *fakeKeys) Signer(_ context.Context, p instance.Purpose) (instance.Signer, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.requested = append(k.requested, p)
	if k.signerErr != nil {
		return nil, k.signerErr
	}
	return fakeSigner{k: k, key: k.active[p], algo: k.alg}, nil
}

// signerFor returns a signer of purpose p for tests that forge tokens.
func (k *fakeKeys) signerFor(p instance.Purpose) fakeSigner {
	k.mu.Lock()
	defer k.mu.Unlock()
	return fakeSigner{k: k, key: k.active[p], algo: instance.AlgEdDSA}
}

func (k *fakeKeys) VerificationKey(_ context.Context, p instance.Purpose, kid string) (instance.PublicKey, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	fk, ok := k.published[kid]
	if !ok || fk.purpose != p {
		return instance.PublicKey{}, errs.New(errs.CodeNotFound, "")
	}
	return instance.PublicKey{KID: kid, Purpose: fk.purpose, Alg: instance.AlgEdDSA, State: instance.KeyActive,
		Key: fk.priv.Public().(ed25519.PublicKey)}, nil
}

// fakeState is instance.State.Load with a counter and an optional first failure.
type fakeState struct {
	mu    sync.Mutex
	id    ids.ID
	loads int
	fail  int
}

func (s *fakeState) Load(context.Context) (instance.Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.loads++
	if s.fail > 0 {
		s.fail--
		return instance.Record{}, errs.New(errs.CodeUnavailable, "")
	}
	return instance.Record{InstanceID: s.id}, nil
}

// invEnv is the invocation service over the fakes, with a captured log.
type invEnv struct {
	svc   *app.InvocationTokens
	keys  *fakeKeys
	state *fakeState
	clk   *clock.Fake
	logs  *bytes.Buffer
}

func newInvEnv(t *testing.T) *invEnv {
	t.Helper()
	e := &invEnv{keys: newFakeKeys(t), state: &fakeState{id: ids.New()}, clk: clock.NewFake(time.Date(2026, 10, 20, 9, 0, 0, 700_000_000, time.UTC)),
		logs: &bytes.Buffer{}}
	svc, err := app.NewInvocationTokens(invCfg, app.InvocationTokenDeps{Keys: e.keys, State: e.state, Clock: e.clk,
		IDs: ids.NewGenerator(e.clk, rand.Reader)})
	if err != nil {
		t.Fatal(err)
	}
	e.svc = svc
	return e
}

// grant is a complete message grant.
func invGrant() domain.InvocationGrant {
	return domain.InvocationGrant{
		AgentID: ids.New(), TriggeredBy: ids.New(), TriggerKind: domain.TriggerMessage, RunID: ids.New(), MessageID: ids.New(),
		ThreadID: ids.New(), ChannelID: ids.New(), SpaceID: ids.New(), Level: "trigger", BillingMode: "organization",
		ToolClass: "chat", PermissionVersion: 3,
	}
}

// segments splits a compact JWS and decodes header and payload.
func segments(t *testing.T, tok string) (map[string]any, map[string]any, []string) {
	t.Helper()
	parts := strings.Split(tok, ".")
	if len(parts) != 3 {
		t.Fatalf("token has %d segments", len(parts))
	}
	var h, p map[string]any
	for i, dst := range []*map[string]any{&h, &p} {
		raw, err := base64.RawURLEncoding.DecodeString(parts[i])
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(raw, dst); err != nil {
			t.Fatal(err)
		}
	}
	return h, p, parts
}

func TestNewInvocationTokens_RequiresDeps(t *testing.T) {
	t.Parallel()
	e := newInvEnv(t)
	good := app.InvocationTokenDeps{Keys: e.keys, State: e.state, Clock: e.clk, IDs: ids.NewGenerator(e.clk, rand.Reader)}
	for name, f := range map[string]func(*app.InvocationTokenConfig, *app.InvocationTokenDeps){
		"no keys":     func(_ *app.InvocationTokenConfig, d *app.InvocationTokenDeps) { d.Keys = nil },
		"no state":    func(_ *app.InvocationTokenConfig, d *app.InvocationTokenDeps) { d.State = nil },
		"no ids":      func(_ *app.InvocationTokenConfig, d *app.InvocationTokenDeps) { d.IDs = nil },
		"ttl zero":    func(c *app.InvocationTokenConfig, _ *app.InvocationTokenDeps) { c.TTL = 0 },
		"leeway < 0":  func(c *app.InvocationTokenConfig, _ *app.InvocationTokenDeps) { c.ClockLeeway = -time.Second },
		"ttl < 1 sec": func(c *app.InvocationTokenConfig, _ *app.InvocationTokenDeps) { c.TTL = 500 * time.Millisecond },
	} {
		cfg, d := invCfg, good
		f(&cfg, &d)
		if _, err := app.NewInvocationTokens(cfg, d); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestIssue_SignsWithInvocationKey(t *testing.T) {
	t.Parallel()
	e := newInvEnv(t)
	ctx := context.Background()
	g := invGrant()
	var jtis []ids.ID
	for i := range 5 {
		got, err := e.svc.Issue(ctx, g)
		if err != nil {
			t.Fatalf("Issue %d: %v", i, err)
		}
		h, p, _ := segments(t, got.Token.Reveal())
		if len(h) != 3 || h["alg"] != "EdDSA" || h["typ"] != "agent-invocation+jwt" || h["kid"] != e.keys.active[instance.PurposeInvocation].kid {
			t.Errorf("header %v", h)
		}
		if got.KeyID != h["kid"] || p["iss"] != e.state.id.String() || p["jti"] != got.JTI.String() || p["aud"] != g.AgentID.String() {
			t.Errorf("issued %+v payload %v", got, p)
		}
		if got.JTI.Time().IsZero() || got.JTI.String()[14] != '7' {
			t.Errorf("jti %s is not a UUIDv7", got.JTI)
		}
		if got.ExpiresAt.Sub(got.IssuedAt) != 90*time.Second || !got.IssuedAt.Equal(e.clk.Now().Truncate(time.Second)) {
			t.Errorf("issued at %v expires at %v", got.IssuedAt, got.ExpiresAt)
		}
		if int64(p["exp"].(float64)) != got.ExpiresAt.Unix() || int64(p["iat"].(float64)) != got.IssuedAt.Unix() {
			t.Errorf("iat/exp %v %v", p["iat"], p["exp"])
		}
		jtis = append(jtis, got.JTI)
		e.clk.Advance(time.Millisecond)
	}
	for i := range jtis {
		for j := range i {
			if jtis[i] == jtis[j] {
				t.Errorf("jti %s issued twice", jtis[i])
			}
		}
	}
	for _, p := range e.keys.requested {
		if p != instance.PurposeInvocation {
			t.Errorf("signer of purpose %s requested", p)
		}
	}
	if len(e.keys.requested) != 5 {
		t.Errorf("%d signer requests, want one per token", len(e.keys.requested))
	}
	if e.state.loads != 1 {
		t.Errorf("instance.State.Load ran %d times, want once", e.state.loads)
	}
	// A promotion (new active key) takes effect on the next token.
	next := e.keys.rotate(t, instance.PurposeInvocation)
	got, err := e.svc.Issue(ctx, g)
	if err != nil || got.KeyID != next.kid {
		t.Fatalf("after a promotion: %+v, %v; want kid %s", got, err, next.kid)
	}
}

func TestIssue_RetriesFailedInstanceLoad(t *testing.T) {
	t.Parallel()
	e := newInvEnv(t)
	e.state.fail = 1
	if _, err := e.svc.Issue(context.Background(), invGrant()); errs.KindOf(err) != errs.KindUnavailable {
		t.Fatalf("first Issue: %v, want the error of Load", err)
	}
	if _, err := e.svc.Issue(context.Background(), invGrant()); err != nil {
		t.Fatalf("second Issue: %v", err)
	}
	if e.state.loads != 2 || e.keys.signs != 1 {
		t.Errorf("loads %d signs %d", e.state.loads, e.keys.signs)
	}
}

func TestIssue_RefusesIncompleteGrant(t *testing.T) {
	t.Parallel()
	e := newInvEnv(t)
	broken := map[string]func(*domain.InvocationGrant){
		"no run":        func(g *domain.InvocationGrant) { g.RunID = ids.ID{} },
		"no object":     func(g *domain.InvocationGrant) { g.MessageID = ids.ID{} },
		"two objects":   func(g *domain.InvocationGrant) { g.CommandID = ids.New() },
		"zero thread":   func(g *domain.InvocationGrant) { g.ThreadID = ids.ID{} },
		"unknown level": func(g *domain.InvocationGrant) { g.Level = "owner" },
		"negative pv":   func(g *domain.InvocationGrant) { g.PermissionVersion = -1 },
		"autocomplete":  func(g *domain.InvocationGrant) { g.TriggerKind = domain.TriggerAutocomplete },
		"command no run": func(g *domain.InvocationGrant) {
			g.TriggerKind, g.MessageID, g.CommandID, g.RunID = domain.TriggerCommand, ids.ID{}, ids.New(), ids.ID{}
		},
	}
	for name, f := range broken {
		g := invGrant()
		f(&g)
		if _, err := e.svc.Issue(context.Background(), g); errs.KindOf(err) != errs.KindInternal {
			t.Errorf("%s: Issue = %v, want an internal error", name, err)
		}
	}
	if e.keys.signs != 0 {
		t.Errorf("the signer signed %d times for broken grants", e.keys.signs)
	}
}

func TestIssue_SignerAlgMustBeEdDSA(t *testing.T) {
	t.Parallel()
	e := newInvEnv(t)
	e.keys.alg = "ES256"
	if _, err := e.svc.Issue(context.Background(), invGrant()); errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("Issue with an ES256 signer: %v, want internal", err)
	}
	if e.keys.signs != 0 {
		t.Errorf("signed %d times", e.keys.signs)
	}
	// Errors of the key store pass through with their kind.
	e.keys.alg, e.keys.signerErr = instance.AlgEdDSA, errs.New(errs.CodeUnavailable, "")
	if _, err := e.svc.Issue(context.Background(), invGrant()); errs.KindOf(err) != errs.KindUnavailable {
		t.Fatalf("Issue without a usable key store: %v, want unavailable", err)
	}
}

// reasonOf returns the code and reason of err.
func reasonOf(err error) (errs.Code, string, errs.Kind) {
	e, ok := errs.As(err)
	if !ok {
		return "", "", errs.KindNone
	}
	return e.Code, e.Reason, errs.KindOf(err)
}

// wantReason fails the test when err is not 403 invocation_token_invalid with reason.
func wantReason(t *testing.T, name string, err error, reason string) {
	t.Helper()
	code, r, kind := reasonOf(err)
	if code != errs.CodeInvocationTokenInvalid || r != reason || kind != errs.KindForbidden {
		t.Errorf("%s: %v (code %s reason %q kind %v), want invocation_token_invalid %s", name, err, code, r, kind, reason)
	}
}

// forge signs payload with the header members under signer.
func forge(t *testing.T, s fakeSigner, header map[string]any, payload []byte) string {
	t.Helper()
	h, err := json.Marshal(header)
	if err != nil {
		t.Fatal(err)
	}
	in := base64.RawURLEncoding.EncodeToString(h) + "." + base64.RawURLEncoding.EncodeToString(payload)
	sig, _ := s.Sign(context.Background(), []byte(in))
	return in + "." + base64.RawURLEncoding.EncodeToString(sig)
}

func TestVerify_Reasons(t *testing.T) {
	t.Parallel()
	e := newInvEnv(t)
	ctx := context.Background()
	g := invGrant()
	got, err := e.svc.Issue(ctx, g)
	if err != nil {
		t.Fatal(err)
	}
	tok := got.Token.Reveal()
	v := app.VerifyInvocation{Audience: g.AgentID, RunID: g.RunID}
	c, err := e.svc.Verify(ctx, tok, v)
	if err != nil || c.Jti != got.JTI.String() || c.MessageID != g.MessageID.String() {
		t.Fatalf("Verify of a fresh token: %+v, %v", c, err)
	}
	if _, err := e.svc.Verify(ctx, tok, app.VerifyInvocation{Audience: g.AgentID}); err != nil {
		t.Errorf("Verify without a run: %v", err)
	}
	_, payload, parts := segments(t, tok)
	raw, _ := json.Marshal(payload)
	sig := []byte(parts[2])
	sig[3] ^= 1
	badSig := parts[0] + "." + parts[1] + "." + string(sig)
	header := map[string]any{"alg": "EdDSA", "kid": got.KeyID, "typ": "agent-invocation+jwt"}
	foreignIss := map[string]any{}
	for k, val := range payload {
		foreignIss[k] = val
	}
	foreignIss["iss"] = ids.New().String()
	fi, _ := json.Marshal(foreignIss)
	unknownKid := map[string]any{"alg": "EdDSA", "kid": "nope", "typ": "agent-invocation+jwt"}
	accessHeader := map[string]any{"alg": "EdDSA", "kid": e.keys.active[instance.PurposeAccessToken].kid, "typ": "agent-invocation+jwt"}
	cases := map[string]struct {
		tok string
		v   app.VerifyInvocation
	}{
		"bad signature":     {badSig, v},
		"unknown kid":       {forge(t, e.keys.signerFor(instance.PurposeInvocation), unknownKid, raw), v},
		"foreign aud":       {tok, app.VerifyInvocation{Audience: ids.New(), RunID: g.RunID}},
		"foreign iss":       {forge(t, e.keys.signerFor(instance.PurposeInvocation), header, fi), v},
		"wrong run":         {tok, app.VerifyInvocation{Audience: g.AgentID, RunID: ids.New()}},
		"access token key":  {forge(t, e.keys.signerFor(instance.PurposeAccessToken), accessHeader, raw), v},
		"typ at+jwt":        {forge(t, e.keys.signerFor(instance.PurposeInvocation), map[string]any{"alg": "EdDSA", "kid": got.KeyID, "typ": "at+jwt"}, raw), v},
		"typ upper case":    {forge(t, e.keys.signerFor(instance.PurposeInvocation), map[string]any{"alg": "EdDSA", "kid": got.KeyID, "typ": "Agent-Invocation+JWT"}, raw), v},
		"not a JWS":         {"abc", v},
		"empty":             {"", v},
		"bad signature ign": {badSig, app.VerifyInvocation{Audience: g.AgentID, RunID: g.RunID, IgnoreExpiry: true}},
	}
	for name, c := range cases {
		_, err := e.svc.Verify(ctx, c.tok, c.v)
		wantReason(t, name, err, "invalid")
		if err != nil && (strings.Contains(err.Error(), parts[2]) || strings.Contains(err.Error(), parts[1])) {
			t.Errorf("%s: the error names the token", name)
		}
	}
	// Expired, otherwise valid: expired; with IgnoreExpiry the claims come back; expired and
	// another failure: invalid.
	e.clk.Set(got.ExpiresAt.Add(invCfg.ClockLeeway))
	_, err = e.svc.Verify(ctx, tok, v)
	wantReason(t, "expired", err, "expired")
	c, err = e.svc.Verify(ctx, tok, app.VerifyInvocation{Audience: g.AgentID, RunID: g.RunID, IgnoreExpiry: true})
	if err != nil || c.Jti != got.JTI.String() {
		t.Errorf("IgnoreExpiry: %+v, %v", c, err)
	}
	_, err = e.svc.Verify(ctx, tok, app.VerifyInvocation{Audience: ids.New(), RunID: g.RunID})
	wantReason(t, "expired and foreign aud", err, "invalid")
	e.clk.Set(got.ExpiresAt.Add(invCfg.ClockLeeway - time.Second))
	if _, err := e.svc.Verify(ctx, tok, v); err != nil {
		t.Errorf("one second before exp plus auth.clock_leeway: %v", err)
	}
}

func TestVerify_UnknownClaimIgnored(t *testing.T) {
	t.Parallel()
	e := newInvEnv(t)
	ctx := context.Background()
	g := invGrant()
	got, err := e.svc.Issue(ctx, g)
	if err != nil {
		t.Fatal(err)
	}
	h, payload, _ := segments(t, got.Token.Reveal())
	payload["newer_claim"] = map[string]any{"x": 1}
	raw, _ := json.Marshal(payload)
	tok := forge(t, e.keys.signerFor(instance.PurposeInvocation), h, raw)
	c, err := e.svc.Verify(ctx, tok, app.VerifyInvocation{Audience: g.AgentID, RunID: g.RunID})
	if err != nil || c.Jti != got.JTI.String() || c.PermissionVersion != 3 {
		t.Fatalf("Verify with an extra member: %+v, %v", c, err)
	}
}

// captureHandler records every log record as JSON.
type captureHandler struct {
	mu  sync.Mutex
	buf *bytes.Buffer
	h   slog.Handler
}

func (c *captureHandler) Enabled(context.Context, slog.Level) bool { return true }
func (c *captureHandler) Handle(ctx context.Context, r slog.Record) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.h.Handle(ctx, r)
}
func (c *captureHandler) WithAttrs([]slog.Attr) slog.Handler { return c }
func (c *captureHandler) WithGroup(string) slog.Handler      { return c }

func TestInvocationToken_Redacted(t *testing.T) {
	t.Parallel()
	e := newInvEnv(t)
	ctx := context.Background()
	got, err := e.svc.Issue(ctx, invGrant())
	if err != nil {
		t.Fatal(err)
	}
	tok := got.Token.Reveal()
	_, _, parts := segments(t, tok)
	if strings.Count(tok, ".") != 2 {
		t.Fatalf("Reveal %q is not a compact JWS", tok)
	}
	holder := struct {
		Token app.InvocationToken `json:"token"`
	}{got.Token}
	var b bytes.Buffer
	fmt.Fprintf(&b, "%v %+v %#v %s %q|", got.Token, got, holder, got.Token, got.Token)
	j, err := json.Marshal(holder)
	if err != nil {
		t.Fatal(err)
	}
	b.Write(j)
	text, _ := got.Token.MarshalText()
	b.Write(text)
	h := &captureHandler{buf: &bytes.Buffer{}}
	h.h = slog.NewJSONHandler(h.buf, nil)
	slog.New(h).Info("issued", "token", got.Token, "issued", got)
	b.Write(h.buf.Bytes())
	out := b.String()
	if !strings.Contains(out, "[redacted]") || strings.Contains(out, parts[2]) || strings.Contains(out, parts[1]) || strings.Contains(out, parts[0]) {
		t.Fatalf("rendering leaks the token: %s", out)
	}
	// Errors of Verify never contain the token or its segments.
	for _, bad := range []string{tok + "x", parts[0] + "." + parts[1] + ".", tok[:len(tok)-2] + "AA"} {
		_, err := e.svc.Verify(ctx, bad, app.VerifyInvocation{Audience: ids.New()})
		if err == nil {
			t.Fatal("a broken token verified")
		}
		msg := fmt.Sprintf("%v %+v", err, err)
		lh := &captureHandler{buf: &bytes.Buffer{}}
		lh.h = slog.NewJSONHandler(lh.buf, nil)
		slog.New(lh).Error("verify", "err", err)
		msg += lh.buf.String()
		for _, p := range parts {
			if strings.Contains(msg, p) {
				t.Fatalf("an error of Verify contains a segment of the token: %s", msg)
			}
		}
	}
	var zero app.InvocationToken
	if zero.Reveal() != "" {
		t.Error("the zero token reveals a value")
	}
}
