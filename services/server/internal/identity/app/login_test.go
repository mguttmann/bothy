// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/authz/adapters/fake"
	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/adapters/identitytest"
	"github.com/OWNER/bothy/services/server/internal/identity/app"
	instanceapp "github.com/OWNER/bothy/services/server/internal/instance/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/jose"
	"github.com/OWNER/bothy/services/server/internal/platform/secrets"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

// loginURL is the public URL of the login world; redeemURL the htu of redeemEnrollmentCode.
const (
	loginURL  = "https://chat.example.com"
	redeemURL = loginURL + "/api/v1/auth/enroll/redeem"
)

// loginCfg is the identity configuration of the login world.
var loginCfg = app.Config{
	HandleMaxChars: 32, DisplayNameMaxChars: 64, DPoPNonceTTL: 5 * time.Minute, DPoPProofMaxAge: time.Minute,
	ClockLeeway: 30 * time.Second, PublicURL: loginURL, PageSizeDefault: 2, PageSizeMax: 3, StepUpMaxAge: 5 * time.Minute,
	AccessTokenTTL: 10 * time.Minute, RefreshTokenTTL: 14 * 24 * time.Hour, SessionMaxAge: 30 * 24 * time.Hour, ConsoleSessionMaxAge: 12 * time.Hour,
	ConsoleSessionIdleTimeout: 30 * time.Minute, ViewAsMaxDuration: 15 * time.Minute,
	RefreshReuseGrace: 30 * time.Second, LastActiveGranularity: 5 * time.Minute, StepUpChallengeTTL: 2 * time.Minute,
	StepUpFailLimit: 3, LoginFailWindow: 15 * time.Minute, CleanupInterval: time.Hour,
	EnrollmentCodeTTL: 72 * time.Hour, EnrollmentCodeMaxAttempts: 2, LoginFlowTTL: 10 * time.Minute,
	DevicePendingTTL: 24 * time.Hour, DeviceVerificationCodeDigits: 6, PairingCodeTTL: 5 * time.Minute, PairingCodeChars: 8,
	APITokenMaxTTL: 365 * 24 * time.Hour, PersonalTokenMaxTTL: 90 * 24 * time.Hour, DPoPRequiredFor: []string{"agent"},
	ServerCodeTTL: time.Minute, OIDCJWKSCacheTTL: time.Hour, OIDCClientAssertionTTL: 2 * time.Minute,
	IDPRecheckInterval: 7 * 24 * time.Hour, DeviceInactiveRevokeAfter: 180 * 24 * time.Hour, GroupClaimMaxAge: 30 * 24 * time.Hour,
	LocalPasswordEnabled: true, PasswordMinLength: 12, LoginFailLimit: 5, LoginLockDuration: 15 * time.Minute,
}

// identityOf builds the identity module with its login service over the people world, with real
// signing keys in a key store of the test and the instance row of bothy migrate.
func (w *peopleWorld) identityOf() *wire.Identity {
	w.t.Helper()
	dir := filepath.Join(w.t.TempDir(), "signing-keys")
	if err := os.Mkdir(dir, 0o700); err != nil {
		w.t.Fatal(err)
	}
	sp, err := secrets.NewProvider(secrets.Options{Dir: filepath.Dir(dir), SigningKeyDir: dir, CacheTTL: time.Minute})
	if err != nil {
		w.t.Fatal(err)
	}
	log := slog.New(slog.DiscardHandler)
	keys, err := wire.NewSigningKeys(wire.SigningKeyConfig{Keys: instanceapp.KeysConfig{
		CacheTTL: time.Minute, InvocationRotation: 30 * 24 * time.Hour, AccessTokenRotation: 30 * 24 * time.Hour,
		JWKSMaxAge: time.Hour, AccessTokenTTL: loginCfg.AccessTokenTTL, ClockLeeway: loginCfg.ClockLeeway,
	}, Dir: dir}, w.pool, sp, w.uow, w.az, w.rec, w.clk, log)
	if err != nil {
		w.t.Fatal(err)
	}
	if err := keys.Keys.EnsureActive(context.Background()); err != nil {
		w.t.Fatal(err)
	}
	instID := ids.New()
	w.sql(`INSERT INTO instance (id, instance_id, schema_version, sync_epoch) VALUES (1, $1, 14, $2) ON CONFLICT (id) DO NOTHING`, instID.String(), ids.New().String())
	st, err := wire.NewInstanceSettings(wire.DefaultInstanceConfig(), true, w.pool, w.uow, w.az, wire.SettingsWrites{Audit: w.rec, Events: w.events}, w.clk)
	if err != nil {
		w.t.Fatal(err)
	}
	id, err := wire.NewIdentity(loginCfg, w.pool, w.uow, w.az, wire.IdentityWrites{Audit: w.rec, Events: w.events, Settings: st.Admin, Flags: st.Admin},
		[]byte("0123456789abcdef0123456789abcdef"), w.clk, log)
	if err != nil {
		w.t.Fatal(err)
	}
	if err := id.BindSessions(keys.Keys, instID); err != nil {
		w.t.Fatal(err)
	}
	return id
}

// newKey is a device key with its public part and thumbprint.
type newKey struct {
	key *identitytest.Key
	pub identity.PublicKey
	jkt string
}

func freshKey(t *testing.T) newKey {
	t.Helper()
	key, pub := identitytest.NewDeviceKey(jose.AlgES256)
	jwk, err := jose.ParseJWK(pub.JWK)
	if err != nil {
		t.Fatal(err)
	}
	return newKey{key: key, pub: pub, jkt: jwk.Thumbprint()}
}

// redeemIn is the input of a redemption of code by a new device with key k, the proof signed by
// signer.
func (w *peopleWorld) redeemIn(id *wire.Identity, code string, k newKey, signer *identitytest.Key) identity.RedeemEnrollment {
	proof := identitytest.SignDPoPProof(signer, identitytest.ProofClaims{HTM: http.MethodPost, HTU: redeemURL, IAT: w.clk.Now(), Nonce: id.DPoP.CurrentNonce()})
	return identity.RedeemEnrollment{
		Code:   code,
		Device: identity.RegisterDevice{DeviceID: ids.New(), Name: "Alex Phone", Platform: identity.PlatformIOS, DPoPKey: k.pub},
		Proof:  identity.ProofRequest{Proofs: []string{proof}, Method: http.MethodPost, URL: redeemURL},
	}
}

func wantKind(t *testing.T, err error, k errs.Kind) {
	t.Helper()
	if errs.KindOf(err) != k {
		t.Fatalf("err %v, want kind %v", err, k)
	}
}

// TestService_BootstrapRedeemAndRecover runs the M1-37 use cases of app.Service directly: the
// recovery before setup, the bootstrap with its validation and second run, the redemption with an
// unknown code, a failed attempt, success and reuse, the codes an owner issues and the host
// recovery (the HTTP and command tests cover the same through their surfaces).
func TestService_BootstrapRedeemAndRecover(t *testing.T) {
	t.Parallel()
	w := newPeopleWorld(t)
	id := w.identityOf()
	ctx := context.Background()
	svc := id.Logins

	if _, err := svc.IssueRecoveryCode(ctx, "alex"); !errors.Is(err, app.ErrSetupPending) {
		t.Fatalf("recovery before setup: %v", err)
	}
	for _, bad := range []app.BootstrapOwnerCmd{{Handle: "Alex", DisplayName: "Alex"}, {Handle: "alex", DisplayName: ""},
		{Handle: "alex", DisplayName: "Alex", Locale: "not a tag"}, {Handle: "alex", DisplayName: "Alex", TimeZone: "Local"}} {
		_, err := svc.BootstrapOwner(ctx, bad)
		wantKind(t, err, errs.KindBadRequest)
	}
	res, err := svc.BootstrapOwner(ctx, app.BootstrapOwnerCmd{Handle: "alex", DisplayName: "Alex", Locale: "en-us", TimeZone: "Europe/Berlin"})
	if err != nil || res.Code.Purpose != app.PurposeFirstLogin {
		t.Fatalf("bootstrap = %+v, %v", res, err)
	}
	if _, err := svc.BootstrapOwner(ctx, app.BootstrapOwnerCmd{Handle: "sam", DisplayName: "Sam"}); !errors.Is(err, app.ErrSetupCompleted) {
		t.Fatalf("second bootstrap: %v", err)
	}

	k := freshKey(t)
	_, err = svc.RedeemEnrollmentCode(ctx, w.redeemIn(id, "en_0000-0000-0000-0000-0000-0000-0000-0000", k, k.key))
	wantKind(t, err, errs.KindUnauthenticated)
	other := freshKey(t)
	_, err = svc.RedeemEnrollmentCode(ctx, w.redeemIn(id, res.Code.Code, k, other.key))
	wantKind(t, err, errs.KindUnauthenticated)
	out, err := svc.RedeemEnrollmentCode(ctx, w.redeemIn(id, res.Code.Code, k, k.key))
	if err != nil || out.Tokens.Device.State != identity.DeviceStateActive || out.Tokens.Session.AuthMethod != identity.AuthEnrollmentCode {
		t.Fatalf("redeem = %+v, %v", out, err)
	}
	_, err = svc.RedeemEnrollmentCode(ctx, w.redeemIn(id, res.Code.Code, freshKey(t), freshKey(t).key))
	wantKind(t, err, errs.KindUnauthenticated)

	owner := actor.Actor{ID: res.OwnerID, Kind: actor.KindUser, SessionID: ids.New(), DeviceID: out.Tokens.Device.ID, StepUpAt: w.clk.Now()}
	w.creds.PutDevice(owner.DeviceID, fakeDevice(owner.ID))
	w.creds.PutSession(owner.SessionID, fakeSession(owner.ID, owner.DeviceID))
	octx := actor.WithActor(ctx, owner)
	robin, err := w.p.CreateUser(octx, owner, identity.CreateUserCmd{Handle: "robin", DisplayName: "Robin", LocalAccount: true})
	if err != nil || robin.Enrollment == nil || robin.Enrollment.Purpose != string(app.PurposeFirstLogin) {
		t.Fatalf("local account = %+v, %v", robin, err)
	}
	kim, err := w.p.CreateUser(octx, owner, identity.CreateUserCmd{Handle: "kim", DisplayName: "Kim"})
	if err != nil {
		t.Fatal(err)
	}
	for user, purpose := range map[ids.ID]string{robin.User.ID: "first_login", kim.User.ID: "identity_link", res.OwnerID: "recovery"} {
		c, err := svc.CreateEnrollmentCode(octx, owner, user)
		if err != nil || c.Purpose != purpose || c.Code.Reveal() == "" {
			t.Fatalf("code for %s = %+v, %v; want %s", user, c, err, purpose)
		}
	}
	_, err = svc.CreateEnrollmentCode(octx, owner, ids.New())
	wantKind(t, err, errs.KindNotFound)
	w.sql(`UPDATE principals SET status = 'disabled', disabled_at = now() WHERE id = $1`, kim.User.ID.String())
	_, err = svc.CreateEnrollmentCode(octx, owner, kim.User.ID)
	wantKind(t, err, errs.KindConflict)

	// The local provider off: the local codes are refused, the host recovery still works.
	w.sql(`UPDATE identity_providers SET enabled = false WHERE kind = 'local'`)
	_, err = svc.CreateEnrollmentCode(octx, owner, robin.User.ID)
	wantKind(t, err, errs.KindConflict)
	rec, err := svc.IssueRecoveryCode(ctx, "alex")
	if err != nil {
		t.Fatal(err)
	}
	r := freshKey(t)
	if got, err := svc.RedeemEnrollmentCode(ctx, w.redeemIn(id, rec.Code, r, r.key)); err != nil || got.Tokens.Session.ID.IsZero() {
		t.Fatalf("host recovery with the local provider off: %+v, %v", got, err)
	}
	if _, err := svc.IssueRecoveryCode(ctx, "robin"); !errors.Is(err, app.ErrNotActiveOwner) {
		t.Fatalf("recovery for a non-owner: %v", err)
	}
}

// TestService_CompleteLoginRules covers the device rules of CompleteLogin directly.
func TestService_CompleteLoginRules(t *testing.T) {
	t.Parallel()
	w := newPeopleWorld(t)
	id := w.identityOf()
	ctx := context.Background()
	alex := w.person("alex", "active", false)
	sam := w.person("sam", "active", false)
	k := freshKey(t)
	dev := ids.New()
	login := func(user ids.ID, key newKey, proofJKT string, trust app.DeviceTrust) (app.LoginResult, error) {
		return id.Logins.CompleteLogin(ctx, app.LoginCompletion{
			UserID: user, AuthMethod: identity.AuthPasskey, ProofJKT: proofJKT, Trust: trust,
			Device: identity.RegisterDevice{DeviceID: dev, Name: "Alex Phone", Platform: identity.PlatformIOS, DPoPKey: key.pub},
		})
	}
	if _, err := id.Logins.CompleteLogin(ctx, app.LoginCompletion{UserID: alex.ID}); errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("no trust: %v", err)
	}
	r, err := login(alex.ID, k, k.jkt, app.TrustUnconfirmed)
	if err != nil || !r.NewDevice || r.Device.State != identity.DeviceStatePending {
		t.Fatalf("unconfirmed = %+v, %v", r, err)
	}
	if r, err = login(alex.ID, k, k.jkt, app.TrustConfirmed); err != nil || r.NewDevice {
		t.Fatalf("known device = %+v, %v", r, err)
	}
	_, err = login(sam.ID, k, k.jkt, app.TrustConfirmed)
	wantKind(t, err, errs.KindConflict)
	o := freshKey(t)
	_, err = login(alex.ID, o, o.jkt, app.TrustConfirmed)
	wantKind(t, err, errs.KindConflict)
	_, err = login(alex.ID, k, "another", app.TrustConfirmed)
	wantKind(t, err, errs.KindUnauthenticated)
	dev = ids.New()
	_, err = id.Logins.CompleteLogin(ctx, app.LoginCompletion{UserID: alex.ID, AuthMethod: identity.AuthPasskey, ProofJKT: k.jkt, Trust: app.TrustConfirmed,
		Device: identity.RegisterDevice{DeviceID: dev, Name: "", Platform: identity.PlatformIOS, DPoPKey: k.pub}})
	wantKind(t, err, errs.KindBadRequest)
	w.sql(`UPDATE principals SET status = 'disabled', disabled_at = now() WHERE id = $1`, sam.ID.String())
	_, err = login(sam.ID, freshKey(t), "", app.TrustConfirmed)
	wantKind(t, err, errs.KindUnauthenticated)
}

func TestNewService_RefusesMissingParts(t *testing.T) {
	t.Parallel()
	if _, err := app.NewService(loginCfg, app.ServiceDeps{}); err == nil {
		t.Fatal("NewService without collaborators")
	}
	if _, err := app.NewEnrollmentCodes(loginCfg, nil, app.EnrollmentDeps{}); err == nil {
		t.Fatal("NewEnrollmentCodes without collaborators")
	}
}

// TestEnrollmentCodes_UnitRules: Issue, Match and MarkUsed need the ambient unit of work and
// RecordFailedAttempt refuses to run inside one.
func TestEnrollmentCodes_UnitRules(t *testing.T) {
	t.Parallel()
	w := newPeopleWorld(t)
	id := w.identityOf()
	ctx := context.Background()
	if _, err := id.Enrollment.Issue(ctx, ids.New(), app.PurposeFirstLogin, nil); errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("Issue outside a unit: %v", err)
	}
	if _, err := id.Enrollment.Match(ctx, "x"); errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("Match outside a unit: %v", err)
	}
	if err := id.Enrollment.MarkUsed(ctx, ids.New(), ids.New()); errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("MarkUsed outside a unit: %v", err)
	}
	err := w.uow.Do(ctx, func(ctx context.Context) error {
		if err := id.Enrollment.RecordFailedAttempt(ctx, ids.New()); errs.KindOf(err) != errs.KindInternal {
			t.Errorf("RecordFailedAttempt inside a unit: %v", err)
		}
		if _, err := id.Enrollment.Issue(ctx, ids.New(), "bogus", nil); errs.KindOf(err) != errs.KindInternal {
			t.Errorf("unknown purpose: %v", err)
		}
		if _, err := id.Enrollment.Issue(ctx, ids.New(), app.PurposeFirstLogin, nil); errs.KindOf(err) != errs.KindNotFound {
			t.Errorf("unknown person: %v", err)
		}
		var rej *app.EnrollmentRejectedError
		if err := id.Enrollment.MarkUsed(ctx, ids.New(), ids.New()); !errors.As(err, &rej) || rej.Error() == "" {
			t.Errorf("MarkUsed of an unknown code: %v", err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// fakeDevice and fakeSession register an active device and a live session of a person in the fake
// credential source of authz.
func fakeDevice(id ids.ID) fake.Device {
	return fake.Device{PrincipalID: id, State: authz.DeviceActive}
}

func fakeSession(id, dev ids.ID) fake.Session {
	return fake.Session{PrincipalID: id, DeviceID: dev, ExpiresAt: peopleStart.Add(time.Hour)}
}
