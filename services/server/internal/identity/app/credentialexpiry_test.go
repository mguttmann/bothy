// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/identity/adapters/identitytest"
	"github.com/OWNER/bothy/services/server/internal/identity/app"
	"github.com/OWNER/bothy/services/server/internal/identity/domain"
	"github.com/OWNER/bothy/services/server/internal/identity/domain/apitoken"
	"github.com/OWNER/bothy/services/server/internal/identity/domain/dpop"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/jose"
)

// unknownSessions is a SessionStore that knows no session: the person's actor is then marked
// CredentialInactive, which this test does not look at.
type unknownSessions struct{ app.SessionStore }

// AuthState reports every session as unknown.
func (unknownSessions) AuthState(context.Context, ids.ID) (app.AuthState, bool, error) {
	return app.AuthState{}, false, nil
}

// TestAuthenticate_CredentialExpiresAt (M1-64): the actor carries the expiry of the credential it
// was authenticated with, the exp of a person's access token and the expires_at of an API token,
// so the gateway schedules reauth_required and closes with 4011 at that instant without parsing
// the token itself.
func TestAuthenticate_CredentialExpiresAt(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := &countingTokens{}
	e := newTokenEnv(t)
	d := newDPoPEnv(t, "pepper")
	a, err := app.NewAuthenticator(app.Config{LastActiveGranularity: 5 * time.Minute, DevicePendingTTL: 24 * time.Hour}, app.AuthenticatorDeps{
		AccessTokens: e.at, DPoP: d.v, Sessions: unknownSessions{}, APITokens: store, Pepper: tokenPepper, Clock: e.clk,
		Log: slog.New(slog.DiscardHandler),
	})
	if err != nil {
		t.Fatal(err)
	}
	key, _ := identitytest.NewDeviceKey(jose.AlgEdDSA)
	raw, claims, err := e.at.Issue(ctx, e.record(), key.Thumbprint)
	if err != nil {
		t.Fatal(err)
	}
	htu := testPublicURL + "/api/v1/gateway"
	proof := identitytest.SignDPoPProof(key, identitytest.ProofClaims{HTM: "GET", HTU: htu, IAT: d.clk.Now(), Nonce: d.v.CurrentNonce(), ATH: dpop.ATH(raw)})
	got, err := a.Authenticate(ctx, actor.Credentials{Method: "GET", HTU: htu, Authorization: "DPoP " + raw, DPoP: proof})
	if err != nil {
		t.Fatal(err)
	}
	if want := time.Unix(claims.Expiry, 0).UTC(); !got.CredentialExpiresAt.Equal(want) {
		t.Errorf("person: CredentialExpiresAt %v, want the exp %v", got.CredentialExpiresAt, want)
	}

	exp := time.Date(2026, 11, 2, 8, 0, 0, 0, time.UTC)
	agt, _ := store.store(t, apitoken.KindAgent, domain.KindAgent, func(r *app.APITokenRecord) { r.ExpiresAt = exp })
	got, err = a.Authenticate(ctx, actor.Credentials{Authorization: "Bearer " + agt})
	if err != nil {
		t.Fatal(err)
	}
	if !got.CredentialExpiresAt.Equal(exp) {
		t.Errorf("API token: CredentialExpiresAt %v, want expires_at %v", got.CredentialExpiresAt, exp)
	}
}
