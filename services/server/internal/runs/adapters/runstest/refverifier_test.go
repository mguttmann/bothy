// SPDX-License-Identifier: Apache-2.0

package runstest_test

import (
	"context"
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/instance"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/runs/adapters/runstest"
	"github.com/OWNER/bothy/services/server/internal/runs/app"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// The values of these tests: agent.invocation_token_ttl, adapter.invocation_token_leeway,
// adapter.jwks_refresh_min_interval and the max-age of the JWKS (agent.jwks_max_age).
const (
	ttl     = time.Minute
	leeway  = 30 * time.Second
	refresh = 5 * time.Minute
	maxAge  = 20 * time.Minute
)

// keyring is the server side of the tests: Ed25519 keys by kid with a purpose, the active key per
// purpose, a JWKS over HTTP on 127.0.0.1 port 0 and the methods of instance.Keys that
// app.InvocationTokens uses.
type keyring struct {
	mu        sync.Mutex
	keys      map[string]ed25519.PrivateKey
	purposes  map[string]instance.Purpose
	active    map[instance.Purpose]string
	published map[string]bool
	srv       *httptest.Server
}

func newKeyring(t *testing.T) *keyring {
	t.Helper()
	k := &keyring{keys: map[string]ed25519.PrivateKey{}, purposes: map[string]instance.Purpose{}, active: map[instance.Purpose]string{},
		published: map[string]bool{}}
	k.add(t, instance.PurposeInvocation, true)
	k.add(t, instance.PurposeAccessToken, true)
	k.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		k.mu.Lock()
		var set struct {
			Keys []map[string]string `json:"keys"`
		}
		for kid, priv := range k.keys {
			if !k.published[kid] {
				continue
			}
			set.Keys = append(set.Keys, map[string]string{"kty": "OKP", "crv": "Ed25519", "kid": kid, "alg": "EdDSA", "use": "sig",
				"x": base64.RawURLEncoding.EncodeToString(priv.Public().(ed25519.PublicKey))})
		}
		k.mu.Unlock()
		w.Header().Set("Content-Type", "application/jwk-set+json")
		w.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%d", int(maxAge/time.Second)))
		_ = json.NewEncoder(w).Encode(set)
	}))
	t.Cleanup(k.srv.Close)
	return k
}

// add creates a key of p; active makes it the active one; it is published.
func (k *keyring) add(t *testing.T, p instance.Purpose, active bool) string {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	kid := fmt.Sprintf("%x", sha256.Sum256(priv.Public().(ed25519.PublicKey)))
	k.mu.Lock()
	defer k.mu.Unlock()
	k.keys[kid], k.purposes[kid], k.published[kid] = priv, p, true
	if active {
		k.active[p] = kid
	}
	return kid
}

type signer struct {
	kid  string
	priv ed25519.PrivateKey
}

func (s signer) KeyID() string { return s.kid }
func (signer) Alg() string     { return instance.AlgEdDSA }
func (s signer) Sign(_ context.Context, in []byte) ([]byte, error) {
	return ed25519.Sign(s.priv, in), nil
}

func (k *keyring) Signer(_ context.Context, p instance.Purpose) (instance.Signer, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	kid := k.active[p]
	return signer{kid: kid, priv: k.keys[kid]}, nil
}

func (k *keyring) VerificationKey(_ context.Context, p instance.Purpose, kid string) (instance.PublicKey, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	priv, ok := k.keys[kid]
	if !ok || k.purposes[kid] != p || !k.published[kid] {
		return instance.PublicKey{}, errs.New(errs.CodeNotFound, "")
	}
	return instance.PublicKey{KID: kid, Purpose: p, Alg: instance.AlgEdDSA, State: instance.KeyActive, Key: priv.Public().(ed25519.PublicKey)}, nil
}

// priv returns the private key of kid.
func (k *keyring) priv(kid string) ed25519.PrivateKey {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.keys[kid]
}

// state is instance.State.Load with a fixed instance id.
type state struct{ id ids.ID }

func (s state) Load(context.Context) (instance.Record, error) {
	return instance.Record{InstanceID: s.id}, nil
}

// env is a keyring, the server's invocation service, a clock and a grant.
type env struct {
	ring  *keyring
	clk   *clock.Fake
	iss   ids.ID
	svc   *app.InvocationTokens
	grant domain.InvocationGrant
}

func newEnv(t *testing.T) *env {
	t.Helper()
	e := &env{ring: newKeyring(t), clk: clock.NewFake(time.Date(2026, 10, 20, 9, 0, 0, 0, time.UTC)), iss: ids.New()}
	svc, err := app.NewInvocationTokens(app.InvocationTokenConfig{TTL: ttl, ClockLeeway: leeway},
		app.InvocationTokenDeps{Keys: e.ring, State: state{id: e.iss}, Clock: e.clk, IDs: ids.NewGenerator(e.clk, rand.Reader)})
	if err != nil {
		t.Fatal(err)
	}
	e.svc = svc
	e.grant = domain.InvocationGrant{
		AgentID: ids.New(), TriggeredBy: ids.New(), TriggerKind: domain.TriggerMessage, RunID: ids.New(), MessageID: ids.New(),
		ThreadID: ids.New(), ChannelID: ids.New(), SpaceID: ids.New(), Level: "trigger", BillingMode: "organization",
		ToolClass: "scoped", PermissionVersion: 4,
	}
	return e
}

// verifier returns a reference verifier of the agent of the grant.
func (e *env) verifier(t *testing.T) *runstest.ReferenceVerifier {
	t.Helper()
	v, err := runstest.NewReferenceVerifier(runstest.ReferenceOptions{
		JWKSURL: e.ring.srv.URL + "/api/v1/instance/jwks", Client: e.ring.srv.Client(), Clock: e.clk,
		Issuer: e.iss, AgentID: e.grant.AgentID, Leeway: leeway, RefreshMinInterval: refresh,
	})
	if err != nil {
		t.Fatal(err)
	}
	return v
}

// issue issues a token for g through the server path.
func (e *env) issue(t *testing.T, g domain.InvocationGrant) string {
	t.Helper()
	got, err := e.svc.Issue(context.Background(), g)
	if err != nil {
		t.Fatal(err)
	}
	return got.Token.Reveal()
}

// full is the expectation with every member set for the grant.
func (e *env) full() runstest.ReferenceExpectation {
	return runstest.ReferenceExpectation{RunID: e.grant.RunID, ThreadID: e.grant.ThreadID, MessageID: e.grant.MessageID,
		ActualBillingMode: "organization", ActualToolClass: "scoped"}
}

// b64 is base64url without padding.
func b64(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

// payloadOf decodes the payload of tok.
func payloadOf(t *testing.T, tok string) map[string]any {
	t.Helper()
	raw, err := base64.RawURLEncoding.DecodeString(strings.Split(tok, ".")[1])
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

// signEd builds a token from header and payload with an Ed25519 signature of priv over the
// header as given.
func signEd(header string, payload []byte, priv ed25519.PrivateKey) string {
	in := b64([]byte(header)) + "." + b64(payload)
	return in + "." + b64(ed25519.Sign(priv, []byte(in)))
}

func TestReferenceVerifier_AcceptsValidToken(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	v := e.verifier(t)
	tok := e.issue(t, e.grant)
	c, err := v.Check(context.Background(), tok, e.full())
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if c.Iss != e.iss.String() || c.Aud != e.grant.AgentID.String() || c.RunID != e.grant.RunID.String() || c.TriggerKind != "message" ||
		c.PermissionVersion != 4 || c.Exp-c.Iat != int64(ttl/time.Second) {
		t.Fatalf("claims %+v", c)
	}
	// An autocomplete token without run passes with the command expectation.
	g := e.grant
	g.TriggerKind, g.RunID, g.MessageID, g.CommandID = domain.TriggerAutocomplete, ids.ID{}, ids.ID{}, ids.New()
	if _, err := v.Check(context.Background(), e.issue(t, g), runstest.ReferenceExpectation{CommandID: g.CommandID, ThreadID: g.ThreadID}); err != nil {
		t.Fatalf("autocomplete: %v", err)
	}
}

func TestNewReferenceVerifier_RequiresOptions(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	good := runstest.ReferenceOptions{JWKSURL: e.ring.srv.URL, Client: e.ring.srv.Client(), Clock: e.clk, Issuer: e.iss,
		AgentID: e.grant.AgentID, Leeway: leeway, RefreshMinInterval: refresh}
	for name, f := range map[string]func(*runstest.ReferenceOptions){
		"url":      func(o *runstest.ReferenceOptions) { o.JWKSURL = "" },
		"client":   func(o *runstest.ReferenceOptions) { o.Client = nil },
		"clock":    func(o *runstest.ReferenceOptions) { o.Clock = nil },
		"issuer":   func(o *runstest.ReferenceOptions) { o.Issuer = ids.ID{} },
		"agent":    func(o *runstest.ReferenceOptions) { o.AgentID = ids.ID{} },
		"leeway":   func(o *runstest.ReferenceOptions) { o.Leeway = -time.Second },
		"interval": func(o *runstest.ReferenceOptions) { o.RefreshMinInterval = 0 },
	} {
		o := good
		f(&o)
		if _, err := runstest.NewReferenceVerifier(o); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestReferenceVerifier_RejectsAlgAndTyp(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ctx := context.Background()
	v := e.verifier(t)
	tok := e.issue(t, e.grant)
	parts := strings.Split(tok, ".")
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatal(err)
	}
	kid := e.ring.active[instance.PurposeInvocation]
	priv := e.ring.priv(kid)
	hdr := func(alg, typ string) string {
		m := map[string]any{"kid": kid}
		if alg != "-" {
			m["alg"] = alg
		}
		if typ != "-" {
			m["typ"] = typ
		}
		b, _ := json.Marshal(m)
		return string(b)
	}
	cases := map[string]string{}
	for _, alg := range []string{"none", "HS256", "ES256", "RS256", "Ed25519", "eddsa", "-"} {
		cases["alg "+alg] = signEd(hdr(alg, "agent-invocation+jwt"), payload, priv)
	}
	for _, typ := range []string{"-", "JWT", "at+jwt", "terminal-session+jwt", "application/agent-invocation+jwt", "Agent-Invocation+JWT", "agent-invocation+jwt "} {
		cases["typ "+typ] = signEd(hdr("EdDSA", typ), payload, priv)
	}
	cases["empty kid"] = signEd(`{"alg":"EdDSA","kid":"","typ":"agent-invocation+jwt"}`, payload, priv)
	cases["crit"] = signEd(`{"alg":"EdDSA","kid":"`+kid+`","typ":"agent-invocation+jwt","crit":["exp"]}`, payload, priv)
	cases["duplicate alg"] = signEd(`{"alg":"none","alg":"EdDSA","kid":"`+kid+`","typ":"agent-invocation+jwt"}`, payload, priv)
	// Algorithm confusion: HS256 keyed with the bytes of the published public key.
	hs := b64([]byte(hdr("HS256", "agent-invocation+jwt"))) + "." + parts[1]
	mac := hmac.New(sha256.New, priv.Public().(ed25519.PublicKey))
	mac.Write([]byte(hs))
	cases["HS256 with the public key"] = hs + "." + b64(mac.Sum(nil))
	cases["none with an empty signature"] = b64([]byte(hdr("none", "agent-invocation+jwt"))) + "." + parts[1] + "."
	cases["padded signature"] = parts[0] + "." + parts[1] + "." + parts[2] + "="
	for name, bad := range cases {
		if _, err := v.Check(ctx, bad, e.full()); !errors.Is(err, runstest.ErrRefInvalid) {
			t.Errorf("%s: Check = %v, want ErrRefInvalid", name, err)
		}
		// The server verifier rejects the same token.
		if _, err := e.svc.Verify(ctx, bad, app.VerifyInvocation{Audience: e.grant.AgentID, RunID: e.grant.RunID}); err == nil {
			t.Errorf("%s: the server verifier accepted it", name)
		}
	}
	// The jti store is unchanged: the valid token passes once, then is a replay.
	if _, err := v.Check(ctx, tok, e.full()); err != nil {
		t.Fatalf("the valid token after the rejections: %v", err)
	}
	if _, err := v.Check(ctx, tok, e.full()); !errors.Is(err, runstest.ErrRefReplay) {
		t.Fatalf("second check: %v, want ErrRefReplay", err)
	}
}

// resign changes the payload of tok with f and signs it again with the active invocation key.
func (e *env) resign(t *testing.T, tok string, f func(map[string]any)) string {
	t.Helper()
	m := payloadOf(t, tok)
	f(m)
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	kid := e.ring.active[instance.PurposeInvocation]
	return signEd(`{"alg":"EdDSA","kid":"`+kid+`","typ":"agent-invocation+jwt"}`, b, e.ring.priv(kid))
}

func TestReferenceVerifier_Points(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ctx := context.Background()
	v := e.verifier(t)
	full := e.full()
	invalid := map[string]func() (string, runstest.ReferenceExpectation){
		"foreign iss": func() (string, runstest.ReferenceExpectation) {
			return e.resign(t, e.issue(t, e.grant), func(m map[string]any) { m["iss"] = ids.New().String() }), full
		},
		"foreign aud": func() (string, runstest.ReferenceExpectation) {
			return e.resign(t, e.issue(t, e.grant), func(m map[string]any) { m["aud"] = ids.New().String() }), full
		},
		"changed payload byte": func() (string, runstest.ReferenceExpectation) {
			p := strings.Split(e.issue(t, e.grant), ".")
			raw, _ := base64.RawURLEncoding.DecodeString(p[1])
			raw = []byte(strings.Replace(string(raw), `"level":"trigger"`, `"level":"triggex"`, 1))
			return p[0] + "." + b64(raw) + "." + p[2], full
		},
		"run_id": func() (string, runstest.ReferenceExpectation) {
			x := full
			x.RunID = ids.New()
			return e.issue(t, e.grant), x
		},
		"thread_id": func() (string, runstest.ReferenceExpectation) {
			x := full
			x.ThreadID = ids.New()
			return e.issue(t, e.grant), x
		},
		"triggering object": func() (string, runstest.ReferenceExpectation) {
			x := full
			x.MessageID = ids.New()
			return e.issue(t, e.grant), x
		},
		"iat in the future": func() (string, runstest.ReferenceExpectation) {
			return e.resign(t, e.issue(t, e.grant), func(m map[string]any) {
				m["iat"] = m["iat"].(float64) + float64(leeway/time.Second) + 1
				m["exp"] = m["iat"].(float64) + 60
			}), full
		},
	}
	for name, f := range invalid {
		tok, x := f()
		if _, err := v.Check(ctx, tok, x); !errors.Is(err, runstest.ErrRefInvalid) {
			t.Errorf("%s: %v, want ErrRefInvalid", name, err)
		}
	}
	for _, mode := range []string{"personal", "unknown"} {
		x := full
		x.ActualBillingMode = mode
		if _, err := v.Check(ctx, e.issue(t, e.grant), x); !errors.Is(err, runstest.ErrRefBillingModeMismatch) {
			t.Errorf("actual mode %s: %v, want ErrRefBillingModeMismatch", mode, err)
		}
	}
	x := full
	x.ActualToolClass = "host"
	if _, err := v.Check(ctx, e.issue(t, e.grant), x); !errors.Is(err, runstest.ErrRefToolClassMismatch) {
		t.Errorf("host with claim scoped: %v", err)
	}
	x.ActualToolClass = "chat"
	if _, err := v.Check(ctx, e.issue(t, e.grant), x); err != nil {
		t.Errorf("chat with claim scoped: %v", err)
	}
	// Expired with a billing mismatch gives the mismatch; expired alone is ErrRefExpired; the
	// last moment before exp plus the leeway passes.
	old := e.issue(t, e.grant)
	oldExp := time.Unix(int64(payloadOf(t, old)["exp"].(float64)), 0)
	last := e.issue(t, e.grant)
	e.clk.Set(oldExp.Add(leeway))
	x = full
	x.ActualBillingMode = "personal"
	if _, err := v.Check(ctx, old, x); !errors.Is(err, runstest.ErrRefBillingModeMismatch) {
		t.Errorf("expired with a billing mismatch: %v", err)
	}
	if _, err := v.Check(ctx, old, full); !errors.Is(err, runstest.ErrRefExpired) {
		t.Errorf("expired: %v", err)
	}
	e.clk.Set(oldExp.Add(leeway - time.Second))
	if _, err := v.Check(ctx, last, full); err != nil {
		t.Errorf("one second before exp plus the leeway: %v", err)
	}
	// The same valid token twice: a replay.
	tok := e.issue(t, e.grant)
	if _, err := v.Check(ctx, tok, full); err != nil {
		t.Fatal(err)
	}
	if _, err := v.Check(ctx, tok, full); !errors.Is(err, runstest.ErrRefReplay) {
		t.Errorf("second check: %v, want ErrRefReplay", err)
	}
}

func TestReferenceVerifier_JWKSCache(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ctx := context.Background()
	v := e.verifier(t)
	if _, err := v.Check(ctx, e.issue(t, e.grant), e.full()); err != nil {
		t.Fatal(err)
	}
	if v.Fetches() != 1 {
		t.Fatalf("fetches %d after the first check", v.Fetches())
	}
	e.clk.Advance(maxAge - time.Second)
	if _, err := v.Check(ctx, e.issue(t, e.grant), e.full()); err != nil || v.Fetches() != 1 {
		t.Fatalf("before max-age: %v, fetches %d", err, v.Fetches())
	}
	e.clk.Advance(time.Second)
	if _, err := v.Check(ctx, e.issue(t, e.grant), e.full()); err != nil || v.Fetches() != 2 || v.ForcedReloads() != 0 {
		t.Fatalf("after max-age: %v, fetches %d forced %d", err, v.Fetches(), v.ForcedReloads())
	}
	// An unknown kid: exactly one forced reload, then ErrRefUnknownKey.
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	payload, _ := base64.RawURLEncoding.DecodeString(strings.Split(e.issue(t, e.grant), ".")[1])
	unknown := signEd(`{"alg":"EdDSA","kid":"unknown-1","typ":"agent-invocation+jwt"}`, payload, priv)
	if _, err := v.Check(ctx, unknown, e.full()); !errors.Is(err, runstest.ErrRefUnknownKey) {
		t.Fatalf("unknown kid: %v", err)
	}
	if v.ForcedReloads() != 1 || v.Fetches() != 3 {
		t.Fatalf("forced %d fetches %d, want 1 and 3", v.ForcedReloads(), v.Fetches())
	}
	// A second unknown kid within adapter.jwks_refresh_min_interval fetches nothing.
	e.clk.Advance(refresh - time.Second)
	other := signEd(`{"alg":"EdDSA","kid":"unknown-2","typ":"agent-invocation+jwt"}`, payload, priv)
	if _, err := v.Check(ctx, other, e.full()); !errors.Is(err, runstest.ErrRefUnknownKey) || v.Fetches() != 3 || v.ForcedReloads() != 1 {
		t.Fatalf("second unknown kid: %v, fetches %d forced %d", err, v.Fetches(), v.ForcedReloads())
	}
	// After the interval a kid that was published meanwhile is found by one forced reload.
	e.clk.Advance(time.Second)
	kid := e.ring.add(t, instance.PurposeInvocation, true)
	if _, err := v.Check(ctx, e.issue(t, e.grant), e.full()); err != nil || v.ForcedReloads() != 2 {
		t.Fatalf("new key %s: %v, forced %d", kid, err, v.ForcedReloads())
	}
}

func TestRunstest_NotInBinary(t *testing.T) {
	t.Parallel()
	out, err := exec.CommandContext(t.Context(), "go", "list", "-deps", "../../../../cmd/bothy").CombinedOutput()
	if err != nil {
		t.Fatalf("go list: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "/internal/runs/app") {
		t.Fatalf("go list -deps did not list the runs module; output %.200s", out)
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasSuffix(line, "/internal/runs/adapters/runstest") {
			t.Fatal("cmd/bothy depends on internal/runs/adapters/runstest")
		}
	}
}
