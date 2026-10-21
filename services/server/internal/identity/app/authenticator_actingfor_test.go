// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/identity/adapters/identitytest"
	"github.com/OWNER/bothy/services/server/internal/identity/app"
	"github.com/OWNER/bothy/services/server/internal/identity/domain"
	"github.com/OWNER/bothy/services/server/internal/identity/domain/apitoken"
	"github.com/OWNER/bothy/services/server/internal/identity/domain/dpop"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/jose"
)

// actingForCall is one call of fakeActingFor.
type actingForCall struct {
	agentID ids.ID
	token   string
}

// fakeActingFor is an identity.ActingForSource that records its calls and answers with result or
// err.
type fakeActingFor struct {
	mu     sync.Mutex
	calls  []actingForCall
	result actor.ActingFor
	err    error
}

// ResolveActingFor records the call and returns the configured answer.
func (f *fakeActingFor) ResolveActingFor(_ context.Context, agentID ids.ID, token string) (actor.ActingFor, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, actingForCall{agentID: agentID, token: token})
	return f.result, f.err
}

// count returns the number of calls so far.
func (f *fakeActingFor) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

// TestAuthenticator_InvocationHeader (M2-14 design note 2): no header leaves ActingFor zero; an
// agent with exactly one value calls the source with its id and the token and copies the result;
// a person with a session, a personal token and an app token with the header are invalid without
// a source call, as are two values and an empty value; the header without Authorization is 401
// unauthenticated; a failing source is 503 unavailable without the token in the error.
func TestAuthenticator_InvocationHeader(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := &countingTokens{}
	e := newTokenEnv(t)
	d := newDPoPEnv(t, "pepper")
	alex, run, thread := ids.New(), ids.New(), ids.New()
	src := &fakeActingFor{result: actor.ActingFor{Status: actor.ActingForBound, UserID: alex, RunID: run, ThreadID: thread, JTI: "jti-1"}}
	a, err := app.NewAuthenticator(app.Config{LastActiveGranularity: 5 * time.Minute, DevicePendingTTL: 24 * time.Hour}, app.AuthenticatorDeps{
		AccessTokens: e.at, DPoP: d.v, Sessions: unknownSessions{}, APITokens: store, Pepper: tokenPepper, Clock: e.clk,
		Log: slog.New(slog.DiscardHandler), ActingFor: src,
	})
	if err != nil {
		t.Fatal(err)
	}
	const invocation = "opaque-invocation-token-of-the-run" // the fake source never parses it
	agentToken, agentRec := store.store(t, apitoken.KindAgent, domain.KindAgent, nil)
	agent := "Bearer " + agentToken

	got, err := a.Authenticate(ctx, actor.Credentials{Authorization: agent})
	if err != nil || got.ActingFor.Present() || src.count() != 0 {
		t.Fatalf("no header: ActingFor %+v, err %v, %d source calls", got.ActingFor, err, src.count())
	}

	got, err = a.Authenticate(ctx, actor.Credentials{Authorization: agent, InvocationTokens: []string{invocation}})
	if err != nil {
		t.Fatal(err)
	}
	if got.ActingFor != src.result || got.ID != agentRec.PrincipalID || got.Kind != actor.KindAgent {
		t.Fatalf("agent with one value: actor %+v", got)
	}
	if src.count() != 1 || src.calls[0].agentID != agentRec.PrincipalID || src.calls[0].token != invocation {
		t.Fatalf("agent with one value: %d source calls, first with the agent %t and the token %t",
			src.count(), src.count() > 0 && src.calls[0].agentID == agentRec.PrincipalID, src.count() > 0 && src.calls[0].token == invocation)
	}

	key, _ := identitytest.NewDeviceKey(jose.AlgEdDSA)
	raw, _, err := e.at.Issue(ctx, e.record(), key.Thumbprint)
	if err != nil {
		t.Fatal(err)
	}
	htu := testPublicURL + "/api/v1/me"
	proof := identitytest.SignDPoPProof(key, identitytest.ProofClaims{HTM: "GET", HTU: htu, IAT: d.clk.Now(), Nonce: d.v.CurrentNonce(), ATH: dpop.ATH(raw)})
	personal, _ := store.store(t, apitoken.KindPersonal, domain.KindUser, nil)
	appToken, _ := store.store(t, apitoken.KindApp, domain.KindApp, nil)
	invalid := []struct {
		name  string
		creds actor.Credentials
	}{
		{"person with a session", actor.Credentials{Method: "GET", HTU: htu, Authorization: "DPoP " + raw, DPoP: proof, InvocationTokens: []string{invocation}}},
		{"personal token", actor.Credentials{Authorization: "Bearer " + personal, InvocationTokens: []string{invocation}}},
		{"app token", actor.Credentials{Authorization: "Bearer " + appToken, InvocationTokens: []string{invocation}}},
		{"agent with two values", actor.Credentials{Authorization: agent, InvocationTokens: []string{invocation, invocation}}},
		{"agent with an empty value", actor.Credentials{Authorization: agent, InvocationTokens: []string{""}}},
	}
	for _, c := range invalid {
		before := src.count()
		got, err := a.Authenticate(ctx, c.creds)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if got.ActingFor != (actor.ActingFor{Status: actor.ActingForInvalid}) || src.count() != before {
			t.Errorf("%s: ActingFor %+v with %d source calls, want invalid without a call", c.name, got.ActingFor, src.count()-before)
		}
	}

	before := src.count()
	for name, creds := range map[string]actor.Credentials{
		"header alone":           {InvocationTokens: []string{invocation}},
		"header with the cookie": {InvocationTokens: []string{invocation}, ConsoleCookie: actor.NewCookieValue("session-value")},
	} {
		_, err = a.Authenticate(ctx, creds)
		wantPublic(t, err, "unauthenticated")
		if e, ok := errs.As(err); !ok || e.Headers.Get("WWW-Authenticate") == "" {
			t.Errorf("%s: 401 without WWW-Authenticate", name)
		}
	}
	if src.count() != before {
		t.Errorf("the header without Authorization called the source")
	}

	src.err = errors.New("runs: database not reachable")
	_, err = a.Authenticate(ctx, actor.Credentials{Authorization: agent, InvocationTokens: []string{invocation}})
	wantPublic(t, err, "unavailable")
	if err != nil && strings.Contains(err.Error(), invocation) {
		t.Errorf("the source error carries the invocation token")
	}
}
