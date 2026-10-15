// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/adapters/identitytest"
	"github.com/OWNER/bothy/services/server/internal/identity/adapters/oidc/oidctest"
	"github.com/OWNER/bothy/services/server/internal/identity/app"
	"github.com/OWNER/bothy/services/server/internal/identity/domain/oidcflow"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/platform/egress"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

// completeURL is the htu of completeOidcLogin.
const completeURL = loginURL + "/api/v1/auth/oidc/complete"

// oidcSecretRef is the secret reference of the test provider.
const oidcSecretRef = "env:BOTHY_IDP_APP" //nolint:gosec // G101: a reference, no secret

// secretMap is a secret source over a map.
type secretMap map[string][]byte

// Resolve implements oidc.Secrets.
func (s secretMap) Resolve(_ context.Context, ref string) ([]byte, error) {
	if v, ok := s[ref]; ok {
		return v, nil
	}
	return nil, errors.New("unknown reference")
}

// scriptedClaimSync is the claim sync of the OIDC app world: fn answers each call (nil fn: a call
// is a test failure, for providers without groups_claim); the commands are kept. The claim sync
// itself is tested in authz and through the login in adapters/httpapi (M1-42).
type scriptedClaimSync struct {
	t     *testing.T
	mu    sync.Mutex
	fn    func(ctx context.Context) error
	calls []authz.SyncClaimGroupsCmd
}

// SyncClaimGroups implements authz.ClaimSync.
func (s *scriptedClaimSync) SyncClaimGroups(ctx context.Context, cmd authz.SyncClaimGroupsCmd) (authz.SyncClaimGroupsResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, cmd)
	if s.fn == nil {
		s.t.Error("claim sync called for a provider without groups_claim")
		return authz.SyncClaimGroupsResult{}, nil
	}
	return authz.SyncClaimGroupsResult{}, s.fn(ctx)
}

// script sets fn and clears the recorded calls.
func (s *scriptedClaimSync) script(fn func(ctx context.Context) error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.fn, s.calls = fn, nil
}

// recorded returns the commands seen since the last script.
func (s *scriptedClaimSync) recorded() []authz.SyncClaimGroupsCmd {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.calls)
}

// oidcApp is the login world with the OIDC use cases bound to a fake provider.
type oidcApp struct {
	w    *peopleWorld
	id   *wire.Identity
	o    *app.OIDC
	idp  *oidctest.IDP
	prov ids.ID
	// claims is the claim sync behind the completion (M1-42).
	claims *scriptedClaimSync
}

// newOIDCApp builds the world; provisioning sets allow_provisioning of the provider.
func newOIDCApp(t *testing.T, provisioning bool) *oidcApp {
	t.Helper()
	w := newPeopleWorld(t)
	id := w.identityOf()
	secret := randomB64(t)
	idp := oidctest.New(t, oidctest.Options{ClientID: "chat", Secret: secret, Now: w.clk.Now})
	port, err := strconv.ParseUint(idp.Port(), 10, 16)
	if err != nil {
		t.Fatal(err)
	}
	claims := &scriptedClaimSync{t: t}
	eg := egress.Config{Timeout: 10 * time.Second, MaxResponseBytes: 1 << 20,
		AllowedHosts: []egress.Rule{{Host: "127.0.0.1", Port: uint16(port), AllowPrivate: true}}}
	if err := id.BindOIDC(wire.OIDCDeps{Egress: eg, Secrets: secretMap{oidcSecretRef: []byte(secret)}, RootCAs: idp.RootCAs(),
		ClaimGroups: claims}); err != nil {
		t.Fatal(err)
	}
	a := &oidcApp{w: w, id: id, o: id.OIDC, idp: idp, prov: ids.New(), claims: claims}
	conf, _ := json.Marshal(map[string]any{"issuer": idp.Issuer(), "client_id": "chat", "client_auth": "client_secret_basic",
		"client_secret_ref": oidcSecretRef})
	w.sql(`INSERT INTO identity_providers (id, kind, name, config, allow_provisioning) VALUES ($1, 'oidc', 'Company', $2, $3)`,
		a.prov.String(), string(conf), provisioning)
	return a
}

// randomB64 is 32 random bytes as base64url.
func randomB64(t *testing.T) string {
	t.Helper()
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// login runs begin and the callback for the provider account sub and returns the verifier and the
// redirect the callback answered with.
func (a *oidcApp) login(t *testing.T, ctx context.Context, link *actor.Actor, sub string) (string, *url.URL) {
	t.Helper()
	verifier := randomB64(t)
	in := identity.OIDCBegin{ProviderID: a.prov, AppRedirectURI: "http://127.0.0.1:4711/callback", ClientChallenge: oidcflow.S256(verifier)}
	var (
		res identity.OIDCBeginResult
		err error
	)
	if link != nil {
		res, err = a.o.BeginIdentityLink(ctx, *link, in)
	} else {
		res, err = a.o.BeginOIDCLogin(ctx, in)
	}
	if err != nil {
		t.Fatal(err)
	}
	a.idp.NextLogin(oidctest.Login{Subject: sub, Name: "Robin Example", PreferredUsername: sub + "@example.com", Email: sub + "@example.com"})
	loc := a.idp.Authorize(t, res.AuthorizationURL)
	q := loc.Query()
	redirect, err := a.o.HandleOIDCCallback(context.Background(), identity.OIDCCallback{State: q.Get("state"), Code: q.Get("code"),
		Error: q.Get("error"), Iss: q.Get("iss"), HasIss: q.Has("iss")})
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(redirect)
	if err != nil {
		t.Fatal(err)
	}
	return verifier, u
}

// complete redeems code with verifier from a new device.
func (a *oidcApp) complete(code, verifier string) (identity.RedeemResult, error) {
	k := freshKey(a.w.t)
	proof := identitytest.SignDPoPProof(k.key, identitytest.ProofClaims{HTM: http.MethodPost, HTU: completeURL, IAT: a.w.clk.Now(), Nonce: a.id.DPoP.CurrentNonce()})
	return a.o.CompleteOIDCLogin(context.Background(), identity.OIDCComplete{
		LoginCode: code, ClientVerifier: verifier,
		Device: identity.RegisterDevice{DeviceID: ids.New(), Name: "Robin Phone", Platform: identity.PlatformIOS, DPoPKey: k.pub},
		Proof:  identity.ProofRequest{Proofs: []string{proof}, Method: http.MethodPost, URL: completeURL},
	})
}

// TestOIDCUseCases runs the OIDC use cases of app.OIDC directly against the fake provider: the
// checks of begin, the callback with its failures, the completion with a known identity, without a
// mapping, with provisioning and through an enrollment code, and link mode (the HTTP tests in
// adapters/httpapi cover the same through the chain).
func TestOIDCUseCases(t *testing.T) {
	t.Parallel()
	a := newOIDCApp(t, false)
	ctx := context.Background()
	good := identity.OIDCBegin{ProviderID: a.prov, AppRedirectURI: "http://127.0.0.1:4711/callback", ClientChallenge: oidcflow.S256(randomB64(t))}

	// begin: unknown provider, bad redirect, bad challenge, bad enrollment code.
	bad := good
	bad.ProviderID = ids.New()
	_, err := a.o.BeginOIDCLogin(ctx, bad)
	wantKind(t, err, errs.KindNotFound)
	bad = good
	bad.AppRedirectURI = "http://localhost:4711/callback"
	_, err = a.o.BeginOIDCLogin(ctx, bad)
	wantKind(t, err, errs.KindBadRequest)
	bad = good
	bad.ClientChallenge = "short"
	_, err = a.o.BeginOIDCLogin(ctx, bad)
	wantKind(t, err, errs.KindBadRequest)
	bad = good
	bad.EnrollmentCode = "en_0000-0000-0000-0000-0000-0000-0000-0000"
	_, err = a.o.BeginOIDCLogin(ctx, bad)
	wantKind(t, err, errs.KindUnauthenticated)

	// The callback with an unknown state: 400.
	_, err = a.o.HandleOIDCCallback(ctx, identity.OIDCCallback{State: "unknown", Code: "x"})
	wantKind(t, err, errs.KindBadRequest)
	// A provider error and a wrong nonce: redirects with the error code.
	a.idp.FailAuthorize("access_denied")
	_, u := a.login(t, ctx, nil, "nobody")
	if u.Query().Get("error") != oidcflow.ErrorAccessDenied {
		t.Fatalf("provider error redirect %s", u)
	}
	a.idp.Reset()
	a.idp.SetClaim("nonce", "other")
	_, u = a.login(t, ctx, nil, "nobody")
	if u.Query().Get("error") != oidcflow.ErrorLoginFailed {
		t.Fatalf("claims redirect %s", u)
	}
	a.idp.Reset()

	// No mapping: 401; a wrong verifier: 401.
	verifier, u := a.login(t, ctx, nil, "unknown-sub")
	_, err = a.complete(u.Query().Get("login_code"), verifier)
	wantKind(t, err, errs.KindUnauthenticated)
	robin := a.w.person("robin", "active", false)
	a.w.sql(`INSERT INTO user_identities (id, user_id, provider_id, subject, issuer) VALUES ($1, $2, $3, 'robin-sub', $4)`,
		ids.New().String(), robin.ID.String(), a.prov.String(), a.idp.Issuer())
	_, u = a.login(t, ctx, nil, "robin-sub")
	_, err = a.complete(u.Query().Get("login_code"), randomB64(t))
	wantKind(t, err, errs.KindUnauthenticated)

	// A known identity logs in with a pending device.
	verifier, u = a.login(t, ctx, nil, "robin-sub")
	out, err := a.complete(u.Query().Get("login_code"), verifier)
	if err != nil || out.Tokens.Session.UserID != robin.ID || out.Tokens.Device.State != identity.DeviceStatePending ||
		out.Tokens.Session.AuthMethod != identity.AuthOIDC {
		t.Fatalf("login = %+v, %v", out.Tokens.Session, err)
	}
	// The same code a second time: 401.
	_, err = a.complete(u.Query().Get("login_code"), verifier)
	wantKind(t, err, errs.KindUnauthenticated)

	// An enrollment code links the identity and activates the device.
	kim := a.w.person("kim", "active", false)
	var code app.IssuedEnrollmentCode
	if err := a.w.uow.Do(ctx, func(ctx context.Context) error {
		var err error
		code, err = a.id.Enrollment.Issue(ctx, kim.ID, app.PurposeIdentityLink, nil)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	verifier = randomB64(t)
	res, err := a.o.BeginOIDCLogin(ctx, identity.OIDCBegin{ProviderID: a.prov, AppRedirectURI: "http://127.0.0.1:4711/callback",
		ClientChallenge: oidcflow.S256(verifier), EnrollmentCode: code.Code})
	if err != nil {
		t.Fatal(err)
	}
	a.idp.NextLogin(oidctest.Login{Subject: "kim-sub"})
	q := a.idp.Authorize(t, res.AuthorizationURL).Query()
	redirect, err := a.o.HandleOIDCCallback(ctx, identity.OIDCCallback{State: q.Get("state"), Code: q.Get("code")})
	if err != nil {
		t.Fatal(err)
	}
	ru, _ := url.Parse(redirect)
	out, err = a.complete(ru.Query().Get("login_code"), verifier)
	if err != nil || out.Tokens.Session.UserID != kim.ID || out.Tokens.Device.State != identity.DeviceStateActive {
		t.Fatalf("link by code = %+v, %v", out.Tokens.Session, err)
	}

	// Link mode: step-up, another person's code, success, a linked subject.
	alex := a.w.person("alex", "active", false)
	alex.StepUpAt = a.w.clk.Now()
	sam := a.w.person("sam", "active", false)
	sam.StepUpAt = a.w.clk.Now()
	verifier, u = a.login(t, ctxOf(alex), &alex, "alex-second")
	_, err = a.o.CompleteIdentityLink(ctxOf(sam), sam, u.Query().Get("login_code"), verifier)
	wantKind(t, err, errs.KindNotFound)
	linked, err := a.o.CompleteIdentityLink(ctxOf(alex), alex, u.Query().Get("login_code"), verifier)
	if err != nil || linked.Subject != "alex-second" || linked.UserID != alex.ID || linked.Issuer != a.idp.Issuer() {
		t.Fatalf("link = %+v, %v", linked, err)
	}
	verifier, u = a.login(t, ctxOf(sam), &sam, "alex-second")
	_, err = a.o.CompleteIdentityLink(ctxOf(sam), sam, u.Query().Get("login_code"), verifier)
	wantKind(t, err, errs.KindConflict)
	_, u = a.login(t, ctxOf(sam), &sam, "sam-second")
	_, err = a.o.CompleteIdentityLink(ctxOf(sam), sam, u.Query().Get("login_code"), randomB64(t))
	wantKind(t, err, errs.KindUnauthenticated)
}

// TestOIDCProvisioning: an unknown subject at a provider with allow_provisioning becomes a pending
// person without e-mail, with a handle from preferred_username and a suffix on a collision.
func TestOIDCProvisioning(t *testing.T) {
	t.Parallel()
	a := newOIDCApp(t, true)
	ctx := context.Background()
	a.w.person("newbie", "active", false)
	verifier, u := a.login(t, ctx, nil, "newbie")
	out, err := a.complete(u.Query().Get("login_code"), verifier)
	if err != nil || out.Tokens.Device.State != identity.DeviceStatePending {
		t.Fatalf("provisioning = %+v, %v", out.Tokens, err)
	}
	var handle, status string
	var email *string
	if err := a.w.owner.QueryRow(ctx, `SELECT p.handle, p.status, u.email FROM principals p JOIN users u ON u.principal_id = p.id WHERE p.id = $1`,
		out.Tokens.Session.UserID.String()).Scan(&handle, &status, &email); err != nil {
		t.Fatal(err)
	}
	if status != "pending" || email != nil || !strings.HasPrefix(handle, "newbie-") {
		t.Fatalf("person %s %s %v", handle, status, email)
	}
}

// TestOIDCClaimSyncRetry (M1-42): a login whose claim sync reports a change runs its unit of work
// again as a rights change of group.member.sync with the claim values, the time of the request and
// auth.group_claim_max_age; an I1 violation of the sync fails the login, ends the flow as failed
// and leaves no session; an absent claim never calls the sync.
func TestOIDCClaimSyncRetry(t *testing.T) {
	t.Parallel()
	a := newOIDCApp(t, true)
	ctx := context.Background()
	a.w.person("owner1", "active", true)
	a.w.sql(`UPDATE identity_providers SET config = config || '{"groups_claim":"groups"}'::jsonb WHERE id = $1`, a.prov.String())
	var declared []string
	a.claims.script(func(ctx context.Context) error {
		spec, ok := txn.RightsChangeOf(ctx)
		if !ok {
			return authz.ErrClaimSyncNeedsRightsChange
		}
		declared = append(declared, spec.Action)
		return nil
	})
	a.idp.SetGroups("groups", "support", "research")
	verifier, u := a.login(t, ctx, nil, "claimed")
	out, err := a.complete(u.Query().Get("login_code"), verifier)
	if err != nil {
		t.Fatal(err)
	}
	calls := a.claims.recorded()
	if len(calls) != 2 || len(declared) != 1 || declared[0] != "group.member.sync" || calls[1].UserID != out.Tokens.Session.UserID ||
		calls[1].ProviderID != a.prov || strings.Join(calls[1].ClaimValues, ",") != "research,support" || calls[1].MaxAge != 30*24*time.Hour ||
		!calls[1].Now.Equal(a.w.clk.Now()) {
		t.Fatalf("calls %+v declared %v", calls, declared)
	}

	// An I1 violation of the sync fails the login and ends the flow as failed.
	a.claims.script(func(ctx context.Context) error {
		if !txn.InRightsChange(ctx) {
			return authz.ErrClaimSyncNeedsRightsChange
		}
		return authz.NewI1Violation([]authz.I1Conflict{{AgentID: ids.New()}}).WithHint(authz.HintI1ClaimSync)
	})
	verifier, u = a.login(t, ctx, nil, "blocked")
	_, err = a.complete(u.Query().Get("login_code"), verifier)
	if e, ok := errs.As(err); !ok || e.Code != errs.CodeI1Violation || e.Hint != authz.HintI1ClaimSync {
		t.Fatalf("error %v", err)
	}
	attempt := u.Query().Get("login_attempt_id")
	if n := a.w.count(`SELECT count(*) FROM auth_login_flows WHERE id = $1 AND status = 'failed'`, attempt); n != 1 {
		t.Fatal("the flow did not fail")
	}
	if n := a.w.count(`SELECT count(*) FROM principals WHERE handle LIKE 'blocked%'`); n != 0 {
		t.Fatal("the refused login provisioned a person")
	}

	// An absent claim never calls the sync.
	a.claims.script(nil)
	a.idp.Reset()
	verifier, u = a.login(t, ctx, nil, "absent")
	if _, err := a.complete(u.Query().Get("login_code"), verifier); err != nil {
		t.Fatal(err)
	}
}
