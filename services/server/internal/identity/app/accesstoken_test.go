// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/identity/app"
	"github.com/OWNER/bothy/services/server/internal/identity/domain/session"
	"github.com/OWNER/bothy/services/server/internal/instance"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/jose"
)

// fakeKeys is instance.Keys over in-memory Ed25519 keys of purpose access_token. VerificationKey
// gives next, active and retiring keys like the real port; a retired or unknown kid is not_found.
type fakeKeys struct {
	mu     sync.Mutex
	priv   map[string]ed25519.PrivateKey
	state  map[string]instance.KeyState
	active string
}

func newFakeKeys() *fakeKeys {
	k := &fakeKeys{priv: map[string]ed25519.PrivateKey{}, state: map[string]instance.KeyState{}}
	k.active = k.add(instance.KeyActive)
	return k
}

// add creates a key in state s and returns its kid.
func (k *fakeKeys) add(s instance.KeyState) string {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		panic(err)
	}
	jwk, err := jose.JWKFromPublicKey(pub)
	if err != nil {
		panic(err)
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	kid := jwk.Thumbprint()
	k.priv[kid], k.state[kid] = priv, s
	return kid
}

// set moves kid to state s; an active key becomes the signing key.
func (k *fakeKeys) set(kid string, s instance.KeyState) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.state[kid] = s
	if s == instance.KeyActive {
		k.active = kid
	}
}

type fakeSigner struct {
	kid  string
	priv ed25519.PrivateKey
}

func (s fakeSigner) KeyID() string { return s.kid }
func (fakeSigner) Alg() string     { return instance.AlgEdDSA }
func (s fakeSigner) Sign(_ context.Context, in []byte) ([]byte, error) {
	return ed25519.Sign(s.priv, in), nil
}

func (k *fakeKeys) Signer(_ context.Context, p instance.Purpose) (instance.Signer, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if p != instance.PurposeAccessToken {
		return nil, errs.New(errs.CodeInternalError, "purpose")
	}
	return fakeSigner{kid: k.active, priv: k.priv[k.active]}, nil
}

func (k *fakeKeys) VerificationKey(_ context.Context, p instance.Purpose, kid string) (instance.PublicKey, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	priv, ok := k.priv[kid]
	if !ok || p != instance.PurposeAccessToken || k.state[kid] == instance.KeyRetired {
		return instance.PublicKey{}, errs.New(errs.CodeNotFound, "")
	}
	pub, _ := priv.Public().(ed25519.PublicKey)
	return instance.PublicKey{KID: kid, Purpose: p, Alg: instance.AlgEdDSA, State: k.state[kid], Key: pub}, nil
}

func (*fakeKeys) PublicKey(context.Context, string) (instance.PublicKey, error) {
	return instance.PublicKey{}, errs.New(errs.CodeNotFound, "")
}
func (*fakeKeys) JWKS(context.Context) (instance.JWKSet, error) { return instance.JWKSet{}, nil }
func (*fakeKeys) Rotate(context.Context, actor.Actor, instance.RotateCommand) (instance.Rotation, error) {
	return instance.Rotation{}, nil
}
func (*fakeKeys) EnsureActive(context.Context) error { return nil }

const testPublicURL = "https://chat.example.com"

// tokenEnv is an access token issuer over fake keys with a fake clock.
type tokenEnv struct {
	keys *fakeKeys
	clk  *clock.Fake
	at   *app.AccessTokens
	iss  ids.ID
}

func newTokenEnv(t *testing.T) *tokenEnv {
	t.Helper()
	e := &tokenEnv{keys: newFakeKeys(), clk: clock.NewFake(time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)), iss: ids.New()}
	at, err := app.NewAccessTokens(app.AccessTokenConfig{InstanceID: e.iss, PublicURL: testPublicURL, TTL: 10 * time.Minute,
		Leeway: 30 * time.Second}, e.keys, e.clk)
	if err != nil {
		t.Fatal(err)
	}
	e.at = at
	return e
}

func (e *tokenEnv) record() app.SessionRecord {
	now := e.clk.Now()
	return app.SessionRecord{ID: ids.New(), UserID: ids.New(), DeviceID: ids.New(), AuthMethod: "passkey", ExpiresAt: now.Add(30 * 24 * time.Hour)}
}

// codeOfErr returns the public code of a kernel/errs error.
func codeOfErr(err error) string {
	if e, ok := errs.As(err); ok {
		return string(e.PublicCode())
	}
	return ""
}

// TestAccessTokens_IssueVerify: the token has header typ at+jwt with the kid and alg of the active
// key and exactly the claims of 08 "7.2"; Verify returns them.
func TestAccessTokens_IssueVerify(t *testing.T) {
	t.Parallel()
	e := newTokenEnv(t)
	s := e.record()
	raw, c, err := e.at.Issue(context.Background(), s, "jkt-1")
	if err != nil {
		t.Fatal(err)
	}
	head, _ := base64.RawURLEncoding.DecodeString(strings.Split(raw, ".")[0])
	var h map[string]any
	if err := json.Unmarshal(head, &h); err != nil {
		t.Fatal(err)
	}
	if h["typ"] != "at+jwt" || h["kid"] != e.keys.active || h["alg"] != "EdDSA" || len(h) != 3 {
		t.Fatalf("header %v", h)
	}
	want := session.BuildClaims(session.ClaimsInput{InstanceID: e.iss, Audience: testPublicURL + "/api/v1", UserID: s.UserID,
		SessionID: s.ID, DeviceID: s.DeviceID, JTI: ids.MustParse(c.JTI), Now: e.clk.Now(), TTL: 10 * time.Minute,
		SessionEnd: s.ExpiresAt, JKT: "jkt-1", AuthMethod: "passkey"})
	got, err := e.at.Verify(context.Background(), raw)
	if err != nil {
		t.Fatal(err)
	}
	gj, _ := json.Marshal(got)
	wj, _ := json.Marshal(want)
	if string(gj) != string(wj) {
		t.Fatalf("claims\n got %s\nwant %s", gj, wj)
	}
	if !e.at.IsOwnAccessToken(context.Background(), raw) {
		t.Fatal("IsOwnAccessToken false for an own token")
	}
}

// TestAuthenticate_KeyStates (token part): a token of an active or retiring key verifies; next,
// retired and unknown keys are 401 unauthenticated.
func TestAuthenticate_KeyStates(t *testing.T) {
	t.Parallel()
	e := newTokenEnv(t)
	ctx := context.Background()
	old := e.keys.active
	raw, _, err := e.at.Issue(ctx, e.record(), "j")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.at.Verify(ctx, raw); err != nil {
		t.Fatalf("active: %v", err)
	}
	e.keys.set(e.keys.add(instance.KeyActive), instance.KeyActive)
	e.keys.set(old, instance.KeyRetiring)
	if _, err := e.at.Verify(ctx, raw); err != nil {
		t.Fatalf("retiring: %v", err)
	}
	e.keys.set(old, instance.KeyNext)
	if _, err := e.at.Verify(ctx, raw); codeOfErr(err) != "unauthenticated" {
		t.Fatalf("next: %v", err)
	}
	e.keys.set(old, instance.KeyRetired)
	if _, err := e.at.Verify(ctx, raw); codeOfErr(err) != "unauthenticated" {
		t.Fatalf("retired: %v", err)
	}
	if e.at.IsOwnAccessToken(ctx, raw) {
		t.Fatal("a token of a retired key counts as own")
	}
}

// TestAuthenticate_ExpiryLeeway (token part): exp within auth.clock_leeway is accepted, beyond it
// token_expired; nbf in the future beyond the leeway is rejected.
func TestAuthenticate_ExpiryLeeway(t *testing.T) {
	t.Parallel()
	e := newTokenEnv(t)
	ctx := context.Background()
	raw, _, err := e.at.Issue(ctx, e.record(), "j")
	if err != nil {
		t.Fatal(err)
	}
	e.clk.Advance(10*time.Minute + 29*time.Second)
	if _, err := e.at.Verify(ctx, raw); err != nil {
		t.Fatalf("within leeway: %v", err)
	}
	e.clk.Advance(time.Second)
	_, err = e.at.Verify(ctx, raw)
	if codeOfErr(err) != "token_expired" {
		t.Fatalf("beyond leeway: %v", err)
	}
	if ee, _ := errs.As(err); ee.Headers.Get("WWW-Authenticate") != `DPoP error="invalid_token", algs="ES256 EdDSA"` {
		t.Fatalf("challenge %q", ee.Headers.Get("WWW-Authenticate"))
	}
	f := newTokenEnv(t)
	raw, _, err = f.at.Issue(ctx, f.record(), "j")
	if err != nil {
		t.Fatal(err)
	}
	f.clk.Advance(-31 * time.Second)
	if _, err := f.at.Verify(ctx, raw); codeOfErr(err) != "unauthenticated" {
		t.Fatalf("nbf beyond leeway: %v", err)
	}
	f.clk.Advance(2 * time.Second)
	if _, err := f.at.Verify(ctx, raw); err != nil {
		t.Fatalf("nbf within leeway: %v", err)
	}
}

// TestAuthenticate_WrongIssOrAud (token part) and malformed tokens: 401 unauthenticated.
func TestAuthenticate_WrongIssOrAud(t *testing.T) {
	t.Parallel()
	e := newTokenEnv(t)
	ctx := context.Background()
	other, err := app.NewAccessTokens(app.AccessTokenConfig{InstanceID: ids.New(), PublicURL: testPublicURL, TTL: time.Minute}, e.keys, e.clk)
	if err != nil {
		t.Fatal(err)
	}
	wrongAud, err := app.NewAccessTokens(app.AccessTokenConfig{InstanceID: e.iss, PublicURL: "https://other.example.com", TTL: time.Minute}, e.keys, e.clk)
	if err != nil {
		t.Fatal(err)
	}
	for name, iss := range map[string]*app.AccessTokens{"iss": other, "aud": wrongAud} {
		raw, _, err := iss.Issue(ctx, e.record(), "j")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := e.at.Verify(ctx, raw); codeOfErr(err) != "unauthenticated" {
			t.Errorf("wrong %s: %v", name, err)
		}
	}
	raw, _, err := e.at.Issue(ctx, e.record(), "j")
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(raw, ".")
	payload, _ := base64.RawURLEncoding.DecodeString(parts[1])
	forged := parts[0] + "." + base64.RawURLEncoding.EncodeToString([]byte(strings.Replace(string(payload), `"amr":["passkey"]`, `"amr":["oidc"]`, 1))) + "." + parts[2]
	for name, tok := range map[string]string{"garbage": "abc", "forged": forged, "empty": "", "two parts": parts[0] + "." + parts[1]} {
		if _, err := e.at.Verify(ctx, tok); codeOfErr(err) != "unauthenticated" {
			t.Errorf("%s: %v", name, err)
		}
		if e.at.IsOwnAccessToken(ctx, tok) && name != "forged" {
			t.Errorf("%s counts as own", name)
		}
	}
}
