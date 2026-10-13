// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/app"
	"github.com/OWNER/bothy/services/server/internal/identity/domain"
	"github.com/OWNER/bothy/services/server/internal/identity/domain/apitoken"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// tokenPepper is the pepper of the token unit tests (never a secret).
var tokenPepper = []byte("0123456789abcdef0123456789abcdef")

// countingTokens is an APITokenStore that counts the lookups by hash and serves the records of
// byHash; every other method is unused by the authenticator and panics through the nil interface.
type countingTokens struct {
	app.APITokenStore
	lookups int
	byHash  map[string]app.APITokenAuthRecord
	byID    map[ids.ID]app.APITokenRecord
	touched int
}

func (c *countingTokens) APITokenByHash(_ context.Context, hash []byte) (app.APITokenAuthRecord, bool, error) {
	c.lookups++
	r, ok := c.byHash[string(hash)]
	return r, ok, nil
}

func (c *countingTokens) APIToken(_ context.Context, id ids.ID) (app.APITokenRecord, bool, error) {
	r, ok := c.byID[id]
	return r, ok, nil
}

func (c *countingTokens) TouchAPIToken(context.Context, ids.ID, time.Time, time.Time) error {
	c.touched++
	return nil
}

// noSessions is a SessionStore the API token path never calls.
type noSessions struct{ app.SessionStore }

// tokenAuth is an Authenticator over the counting store, at the clock of a token env.
func tokenAuth(t *testing.T, store *countingTokens) (*app.Authenticator, *tokenEnv) {
	t.Helper()
	e := newTokenEnv(t)
	d := newDPoPEnv(t, "pepper")
	a, err := app.NewAuthenticator(app.Config{LastActiveGranularity: 5 * time.Minute}, app.AuthenticatorDeps{
		AccessTokens: e.at, DPoP: d.v, Sessions: noSessions{}, APITokens: store, Pepper: tokenPepper, Clock: e.clk,
		Log: slog.New(slog.DiscardHandler),
	})
	if err != nil {
		t.Fatal(err)
	}
	return a, e
}

// store puts a token of principal kind kind into c and returns its secret and record.
func (c *countingTokens) store(t *testing.T, k apitoken.Kind, kind domain.Kind, mod func(*app.APITokenRecord)) (string, app.APITokenRecord) {
	t.Helper()
	secret, err := apitoken.Generate(nil, k)
	if err != nil {
		t.Fatal(err)
	}
	r := app.APITokenRecord{ID: ids.New(), PrincipalID: ids.New(), ExpiresAt: time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC)}
	if mod != nil {
		mod(&r)
	}
	if c.byHash == nil {
		c.byHash, c.byID = map[string]app.APITokenAuthRecord{}, map[ids.ID]app.APITokenRecord{}
	}
	c.byHash[string(apitoken.Hash(tokenPepper, secret))] = app.APITokenAuthRecord{APITokenRecord: r, PrincipalKind: kind}
	c.byID[r.ID] = r
	return secret, r
}

// wantPublic checks the public code of err.
func wantPublic(t *testing.T, err error, code string) {
	t.Helper()
	if got := codeOfErr(err); got != code {
		t.Fatalf("error %v (code %s), want %s", err, got, code)
	}
}

// TestIDT23_WrongChecksumRejectedWithoutLookup (ID-T23): a value with the prefix that fails the
// format or the checksum, and an incoming webhook token, are 401 unauthenticated without a
// database call; a well-formed unknown token costs exactly one lookup.
func TestIDT23_WrongChecksumRejectedWithoutLookup(t *testing.T) {
	t.Parallel()
	store := &countingTokens{}
	a, _ := tokenAuth(t, store)
	good, err := apitoken.Generate(nil, apitoken.KindAgent)
	if err != nil {
		t.Fatal(err)
	}
	last := good[len(good)-1]
	other := byte('A')
	if last == 'A' {
		other = 'B'
	}
	whi, err := apitoken.Generate(nil, apitoken.KindIncomingWebhook)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{good[:len(good)-1] + string(other), good[:len(good)-1], "bothy_xyz" + good[9:], whi, "bothy_"} {
		for _, scheme := range []string{"Bearer ", "DPoP "} {
			_, err := a.Authenticate(context.Background(), actor.Credentials{Authorization: scheme + s, Method: "GET", HTU: testPublicURL + "/api/v1/me"})
			wantPublic(t, err, "unauthenticated")
		}
	}
	if store.lookups != 0 {
		t.Fatalf("%d lookups for malformed tokens, want 0", store.lookups)
	}
	_, err = a.Authenticate(context.Background(), actor.Credentials{Authorization: "Bearer " + good})
	wantPublic(t, err, "unauthenticated")
	if store.lookups != 1 {
		t.Fatalf("%d lookups for an unknown token, want 1", store.lookups)
	}
}

// TestAuthenticateAPIToken_Actor: a valid unbound token yields the actor of its principal with the
// token id and scopes, no session, no device and no step-up; a revoked or expired token yields the
// same actor without touching last_used_at (authorize refuses it with token_revoked); a kind that
// does not fit the principal is 401; a bound token as Bearer is invalid_dpop_proof.
func TestAuthenticateAPIToken_Actor(t *testing.T) {
	t.Parallel()
	store := &countingTokens{}
	a, e := tokenAuth(t, store)
	ctx := context.Background()
	pat, rec := store.store(t, apitoken.KindPersonal, domain.KindUser, nil)
	got, err := a.Authenticate(ctx, actor.Credentials{Authorization: "Bearer " + pat})
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != rec.PrincipalID || got.Kind != actor.KindUser || got.TokenID != rec.ID || !got.SessionID.IsZero() || !got.DeviceID.IsZero() ||
		!got.StepUpAt.IsZero() {
		t.Fatalf("actor %+v", got)
	}
	if store.touched != 1 {
		t.Errorf("touched %d, want 1", store.touched)
	}
	revoked, rrec := store.store(t, apitoken.KindAgent, domain.KindAgent, func(r *app.APITokenRecord) { r.RevokedAt = e.clk.Now() })
	expired, erec := store.store(t, apitoken.KindAgent, domain.KindAgent, func(r *app.APITokenRecord) { r.ExpiresAt = e.clk.Now() })
	for s, r := range map[string]app.APITokenRecord{revoked: rrec, expired: erec} {
		got, err := a.Authenticate(ctx, actor.Credentials{Authorization: "Bearer " + s})
		if err != nil || got.TokenID != r.ID || got.Kind != actor.KindAgent {
			t.Errorf("inactive token: %+v, %v", got, err)
		}
		if err := a.Revalidate(ctx, got); codeOfErr(err) != "unauthenticated" {
			t.Errorf("Revalidate of an inactive token: %v", err)
		}
	}
	if store.touched != 1 {
		t.Errorf("an inactive token touched last_used_at")
	}
	mismatch, _ := store.store(t, apitoken.KindAgent, domain.KindUser, nil)
	_, err = a.Authenticate(ctx, actor.Credentials{Authorization: "Bearer " + mismatch})
	wantPublic(t, err, "unauthenticated")
	bound, _ := store.store(t, apitoken.KindAgent, domain.KindAgent, func(r *app.APITokenRecord) { r.DPoPJKT = strings.Repeat("A", 43) })
	_, err = a.Authenticate(ctx, actor.Credentials{Authorization: "Bearer " + bound})
	var de *identity.DPoPError
	if !errors.As(err, &de) || de.Code != identity.DPoPInvalidProof {
		t.Fatalf("bound token as Bearer: %v", err)
	}
	if err := a.Revalidate(ctx, got); err != nil {
		t.Errorf("Revalidate of the active personal token: %v", err)
	}
	foreign := got
	foreign.ID = ids.New()
	if err := a.Revalidate(ctx, foreign); codeOfErr(err) != "unauthenticated" {
		t.Errorf("Revalidate of a token of another principal: %v", err)
	}
}

// TestIssuedToken_Redacted (SEC-LOG-1): the secret of an IssuedToken prints [redacted] through
// fmt, slog and JSON; only Reveal hands it out.
func TestIssuedToken_Redacted(t *testing.T) {
	t.Parallel()
	secret, err := apitoken.Generate(nil, apitoken.KindAgent)
	if err != nil {
		t.Fatal(err)
	}
	it := identity.IssuedToken{Secret: identity.NewToken(secret)}
	var sb strings.Builder
	slog.New(slog.NewTextHandler(&sb, nil)).Info("issued", slog.Any("token", it), slog.Any("secret", it.Secret))
	b, err := json.Marshal(it)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{sb.String(), it.Secret.String(), fmt.Sprintf("%v %+v %#v %s", it, it, it, it.Secret), string(b)} {
		if strings.Contains(s, secret) {
			t.Fatalf("secret printed: %s", s)
		}
	}
	if it.Secret.Reveal() != secret {
		t.Fatal("Reveal lost the secret")
	}
}

// TestCredentialSource_Tokens (brief M1-44, PostgreSQL 18): a token is active when it exists,
// belongs to the principal and is not revoked; its expiry is reported for authz to compare, so an
// expired token is active with an end in the past; a token of another principal and an unknown one
// are not active.
func TestCredentialSource_Tokens(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	src := app.NewCredentialSource(f.repo, f.repo, f.repo)
	ctx := context.Background()
	lotse := f.add(domain.KindAgent, "lotse")
	alex := f.add(domain.KindUser, "alex")
	end := time.Now().Add(time.Hour).UTC().Truncate(time.Microsecond)
	tok := func(expires time.Time, revoked bool) ids.ID {
		id := ids.New()
		secret, err := apitoken.Generate(nil, apitoken.KindAgent)
		if err != nil {
			t.Fatal(err)
		}
		p, _ := apitoken.Parse(secret)
		f.do(func(ctx context.Context) error {
			return f.repo.InsertAPIToken(ctx, app.APITokenRecord{ID: id, PrincipalID: lotse.ID, Name: "Adapter", Prefix: p.Prefix(),
				Hash: apitoken.Hash(tokenPepper, secret), ExpiresAt: expires, CreatedBy: alex.ID, CreatedAt: time.Now()})
		})
		if revoked {
			f.sql(`UPDATE api_tokens SET revoked_at = now(), revoked_by = $2 WHERE id = $1`, id.String(), alex.ID.String())
		}
		return id
	}
	past := time.Now().Add(-time.Minute).UTC().Truncate(time.Microsecond)
	active, revoked, expired := tok(end, false), tok(end, true), tok(past, false)
	for _, tc := range []struct {
		name      string
		principal ids.ID
		token     ids.ID
		active    bool
		expires   time.Time
	}{
		{"active", lotse.ID, active, true, end},
		{"revoked", lotse.ID, revoked, false, time.Time{}},
		{"expired", lotse.ID, expired, true, past},
		{"foreign principal", alex.ID, active, false, time.Time{}},
		{"unknown token", lotse.ID, ids.New(), false, time.Time{}},
	} {
		got, err := src.Credentials(ctx, authz.CredentialRef{PrincipalID: tc.principal, TokenID: tc.token})
		if err != nil || got.TokenActive != tc.active || (tc.active && !got.TokenExpiresAt.Equal(tc.expires)) || got.SessionActive {
			t.Errorf("%s: %+v, %v; want active %v until %v", tc.name, got, err, tc.active, tc.expires)
		}
	}
	// A request without a token reports no token facts.
	if got, err := src.Credentials(ctx, authz.CredentialRef{PrincipalID: lotse.ID}); err != nil || got.TokenActive {
		t.Errorf("no token id: %+v, %v", got, err)
	}
}

// TestNewTokens_Config: the token use cases refuse a missing pepper, a maximum lifetime of zero
// and a value of auth.dpop_required_for outside agent, app and personal.
func TestNewTokens_Config(t *testing.T) {
	t.Parallel()
	if err := app.CheckDPoPRequiredFor([]string{"agent", "app", "personal"}); err != nil {
		t.Fatal(err)
	}
	err := app.CheckDPoPRequiredFor([]string{"agents"})
	if err == nil || strings.Contains(err.Error(), "agents") {
		t.Fatalf("CheckDPoPRequiredFor(agents) = %v", err)
	}
	if _, err := app.NewTokens(app.Config{}, tokenPepper, app.TokensDeps{}); err == nil {
		t.Fatal("NewTokens without collaborators passed")
	}
}
