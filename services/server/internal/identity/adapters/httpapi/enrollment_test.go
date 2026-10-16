// SPDX-License-Identifier: Apache-2.0

package httpapi_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/adapters/identitytest"
	"github.com/OWNER/bothy/services/server/internal/identity/adapters/locale"
	"github.com/OWNER/bothy/services/server/internal/identity/adapters/postgres"
	"github.com/OWNER/bothy/services/server/internal/identity/app"
	"github.com/OWNER/bothy/services/server/internal/identity/domain"
	instancepg "github.com/OWNER/bothy/services/server/internal/instance/adapters/postgres"
	instanceapp "github.com/OWNER/bothy/services/server/internal/instance/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/jose"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
)

// redeemPath is the operation redeemEnrollmentCode.
const redeemPath = "/api/v1/auth/enroll/redeem"

// displayForm is the display form of an enrollment code: en_ plus eight groups of four Crockford
// Base32 characters.
var displayForm = regexp.MustCompile(`^en_[0-9A-HJKMNP-TV-Z]{4}(-[0-9A-HJKMNP-TV-Z]{4}){7}$`)

// instanceRow creates the instance row the way bothy migrate up does (the pgtest templates of this
// package run only the goose migrations).
func (w *sworld) instanceRow() {
	w.t.Helper()
	w.sql(`INSERT INTO instance (id, instance_id, schema_version, sync_epoch) VALUES (1, $1, 14, $2) ON CONFLICT (id) DO NOTHING`,
		w.instID.String(), ids.New().String())
}

// bootstrap runs bothy setup bootstrap-owner for handle through the use case and returns the
// result.
func (w *sworld) bootstrap(handle string) app.BootstrapResult {
	w.t.Helper()
	w.instanceRow()
	res, err := w.id.Logins.BootstrapOwner(context.Background(), app.BootstrapOwnerCmd{Handle: handle, DisplayName: "Alex"})
	if err != nil {
		w.t.Fatal(err)
	}
	if !displayForm.MatchString(res.Code.Code) {
		w.t.Fatalf("code %q is not in display form", res.Code.Code)
	}
	return res
}

// newPhone is the key and registration of a new device that has not logged in yet.
type newPhone struct {
	key *identitytest.Key
	pub identity.PublicKey
	id  ids.ID
	jkt string
}

// phoneKeys returns a new device with an ES256 key.
func (w *sworld) phoneKeys() *newPhone {
	w.t.Helper()
	key, pub := identitytest.NewDeviceKey(jose.AlgES256)
	jwk, err := jose.ParseJWK(pub.JWK)
	if err != nil {
		w.t.Fatal(err)
	}
	return &newPhone{key: key, pub: pub, id: ids.New(), jkt: jwk.Thumbprint()}
}

// body is the EnrollRedeemRequest of p for code.
func (p *newPhone) body(code string) string {
	b, _ := json.Marshal(map[string]any{
		"enrollment_code": code,
		"device":          map[string]any{"device_id": p.id.String(), "name": "Alex Phone", "platform": "ios", "public_jwk": json.RawMessage(p.pub.JWK), "alg": "ES256"},
	})
	return string(b)
}

// redeem sends redeemEnrollmentCode with the registration of p and a proof signed by signer
// (p.key when nil).
func (w *sworld) redeem(p *newPhone, code string, signer *identitytest.Key) *httpservertest.Response {
	w.t.Helper()
	if signer == nil {
		signer = p.key
	}
	proof := identitytest.SignDPoPProof(signer, identitytest.ProofClaims{
		HTM: http.MethodPost, HTU: httpservertest.PublicURL + redeemPath, IAT: w.clk.Now(), Nonce: w.id.DPoP.CurrentNonce(),
	})
	return w.h.Do(w.h.NewRequest(http.MethodPost, redeemPath, p.body(code), "DPoP", proof))
}

// attempts returns enrollment_codes.attempts of the code id.
func (w *sworld) attempts(id ids.ID) int {
	w.t.Helper()
	return w.count(`SELECT attempts FROM enrollment_codes WHERE id = $1`, id.String())
}

// withoutCorrelation returns the problem body without correlation_id.
func withoutCorrelation(t *testing.T, r *httpservertest.Response) string {
	t.Helper()
	p := r.Problem(t)
	delete(p, "correlation_id")
	b, _ := json.Marshal(p)
	return string(b)
}

// rejectedBody is the body of a rejected code: the answer to a code that never existed.
func (w *sworld) rejectedBody() string {
	w.t.Helper()
	r := w.redeem(w.phoneKeys(), "en_0000-0000-0000-0000-0000-0000-0000-0000", nil)
	w.want(r, http.StatusUnauthorized, "unauthenticated")
	return withoutCorrelation(w.t, r)
}

func TestRedeemActivatesFirstDevice(t *testing.T) {
	t.Parallel()
	w := newSessionWorld(t)
	res := w.bootstrap("alex")
	p := w.phoneKeys()
	// The code is accepted in any case, with spaces and without its prefix (08 section 3.2).
	typed := strings.ToLower(strings.ReplaceAll(strings.TrimPrefix(res.Code.Code, "en_"), "-", " "))
	m := w.want(w.redeem(p, typed, nil), http.StatusCreated, "")
	dev := m["device"].(map[string]any)
	sess := m["session"].(map[string]any)
	if dev["state"] != "active" || dev["id"] != p.id.String() || sess["auth_method"] != "enrollment_code" || sess["user_id"] != res.OwnerID.String() {
		t.Fatalf("device %v session %v", dev, sess)
	}
	var state, jkt, method string
	w.scalar(&state, `SELECT state FROM devices WHERE id = $1`, p.id.String())
	w.scalar(&jkt, `SELECT f.dpop_jkt FROM refresh_token_families f JOIN auth_sessions s ON s.id = f.session_id WHERE s.device_id = $1`, p.id.String())
	w.scalar(&method, `SELECT auth_method FROM auth_sessions WHERE device_id = $1`, p.id.String())
	if state != "active" || jkt != p.jkt || method != "enrollment_code" {
		t.Fatalf("device %s, family key %s (want %s), method %s", state, jkt, p.jkt, method)
	}
	if n := w.count(`SELECT count(*) FROM enrollment_codes WHERE id = $1 AND used_at IS NOT NULL AND used_by_device_id = $2`, res.Code.ID.String(), p.id.String()); n != 1 {
		t.Fatal("code not marked used by the device")
	}
	// The session carries the local identity, the client address and the identity its last login.
	if n := w.count(`SELECT count(*) FROM auth_sessions s JOIN user_identities i ON i.id = s.identity_id
		WHERE s.device_id = $1 AND s.last_ip IS NOT NULL AND i.last_login_at IS NOT NULL AND i.subject = 'alex'`, p.id.String()); n != 1 {
		t.Fatal("session without local identity, last_ip or last_login_at")
	}
	entries := w.audits("auth.login.complete")
	if len(entries) != 1 || entries[0].Decision != "allow" {
		t.Fatalf("audit %+v, want one allow entry", entries)
	}
	d := entries[0].Details
	if d["auth_method"] != "enrollment_code" || d["device_id"] != p.id.String() || d["new_device"] != true || d["device_state"] != "active" ||
		d["session_id"] != sess["id"] {
		t.Fatalf("details %v", d)
	}
	var actorID string
	w.scalar(&actorID, `SELECT coalesce(actor_principal_id::text, '') FROM audit_events WHERE action = 'auth.login.complete'`)
	if actorID != res.OwnerID.String() {
		t.Fatalf("actor %q, want the person who logged in", actorID)
	}
	// The tokens work: the device is active, so getMe answers.
	c := &client{w: w, key: p.key, user: res.OwnerID, device: p.id, access: m["access_token"].(string)}
	w.want(c.call(http.MethodGet, "/api/v1/me", ""), http.StatusOK, "")
}

func TestIDT11_CodeUsedTwiceRejected(t *testing.T) {
	t.Parallel()
	w := newSessionWorld(t)
	res := w.bootstrap("alex")
	want := w.rejectedBody()
	w.want(w.redeem(w.phoneKeys(), res.Code.Code, nil), http.StatusCreated, "")
	r := w.redeem(w.phoneKeys(), res.Code.Code, nil)
	w.want(r, http.StatusUnauthorized, "unauthenticated")
	if got := withoutCorrelation(t, r); got != want {
		t.Fatalf("body %s, want %s", got, want)
	}
}

func TestIDT11_ExpiredCodeRejected(t *testing.T) {
	t.Parallel()
	w := newSessionWorld(t)
	res := w.bootstrap("alex")
	want := w.rejectedBody()
	w.clk.Advance(sessCfg.EnrollmentCodeTTL)
	r := w.redeem(w.phoneKeys(), res.Code.Code, nil)
	w.want(r, http.StatusUnauthorized, "unauthenticated")
	if got := withoutCorrelation(t, r); got != want {
		t.Fatalf("body %s, want %s", got, want)
	}
}

func TestIDT11_CodeRejectedAfterMaxAttempts(t *testing.T) {
	t.Parallel()
	w := newSessionWorld(t)
	res := w.bootstrap("alex")
	want := w.rejectedBody()
	for i := range sessCfg.EnrollmentCodeMaxAttempts {
		p := w.phoneKeys()
		w.want(w.redeem(p, res.Code.Code, w.phoneKeys().key), http.StatusUnauthorized, "invalid_dpop_proof")
		if n := w.attempts(res.Code.ID); n != i+1 {
			t.Fatalf("attempts %d after %d failures", n, i+1)
		}
	}
	r := w.redeem(w.phoneKeys(), res.Code.Code, nil)
	w.want(r, http.StatusUnauthorized, "unauthenticated")
	if got := withoutCorrelation(t, r); got != want {
		t.Fatalf("body %s, want %s", got, want)
	}
	// The denials are aggregated per action and reason (05 "2.4"); none carries the code.
	denials := 0
	for _, e := range w.audits("auth.login.complete") {
		if e.Decision == "deny" {
			denials++
			if strings.Contains(e.Raw, res.Code.Code) {
				t.Fatal("a denial carries the code")
			}
		}
	}
	if denials == 0 {
		t.Fatal("no denial audited")
	}
}

func TestRedeemFailedAttemptSurvivesRollback(t *testing.T) {
	t.Parallel()
	w := newSessionWorld(t)
	res := w.bootstrap("alex")
	p := w.phoneKeys()
	// The proof is signed by another key than device.public_jwk: the unit of work rolls back.
	w.want(w.redeem(p, res.Code.Code, w.phoneKeys().key), http.StatusUnauthorized, "invalid_dpop_proof")
	if n := w.attempts(res.Code.ID); n != 1 {
		t.Fatalf("attempts %d, want 1 after the rollback", n)
	}
	if n := w.count(`SELECT count(*) FROM devices WHERE id = $1`, p.id.String()) + w.count(`SELECT count(*) FROM auth_sessions`); n != 0 {
		t.Fatal("the failed redemption left a device or a session")
	}
	// An invalid registration counts as well.
	bad := w.phoneKeys()
	b := strings.Replace(bad.body(res.Code.Code), `"Alex Phone"`, `"Alex\u202ePhone"`, 1)
	proof := identitytest.SignDPoPProof(bad.key, identitytest.ProofClaims{HTM: http.MethodPost, HTU: httpservertest.PublicURL + redeemPath, IAT: w.clk.Now(), Nonce: w.id.DPoP.CurrentNonce()})
	w.want(w.h.Do(w.h.NewRequest(http.MethodPost, redeemPath, b, "DPoP", proof)), http.StatusBadRequest, "validation_failed")
	if n := w.attempts(res.Code.ID); n != 2 {
		t.Fatalf("attempts %d, want 2", n)
	}
	w.want(w.redeem(w.phoneKeys(), res.Code.Code, nil), http.StatusCreated, "")
}

func TestRedeemUnknownCodeDoesNotTouchAttempts(t *testing.T) {
	t.Parallel()
	w := newSessionWorld(t)
	res := w.bootstrap("alex")
	for _, code := range []string{"en_0000-0000-0000-0000-0000-0000-0000-0000", "not a code", "", strings.Repeat("U", 32)} {
		w.want(w.redeem(w.phoneKeys(), code, nil), http.StatusUnauthorized, "unauthenticated")
	}
	if n := w.attempts(res.Code.ID); n != 0 {
		t.Fatalf("attempts %d, want 0", n)
	}
}

func TestRedeemWithoutDPoPProof(t *testing.T) {
	t.Parallel()
	w := newSessionWorld(t)
	res := w.bootstrap("alex")
	p := w.phoneKeys()
	w.want(w.h.Do(w.h.NewRequest(http.MethodPost, redeemPath, p.body(res.Code.Code))), http.StatusUnauthorized, "invalid_dpop_proof")
	// A proof for another URL is no login proof either.
	proof := identitytest.SignDPoPProof(p.key, identitytest.ProofClaims{HTM: http.MethodPost, HTU: httpservertest.PublicURL + "/api/v1/auth/token/refresh", IAT: w.clk.Now(), Nonce: w.id.DPoP.CurrentNonce()})
	w.want(w.h.Do(w.h.NewRequest(http.MethodPost, redeemPath, p.body(res.Code.Code), "DPoP", proof)), http.StatusUnauthorized, "invalid_dpop_proof")
	if n := w.attempts(res.Code.ID); n != 0 {
		t.Fatalf("attempts %d, want 0: the code is not looked up", n)
	}
	for _, e := range w.audits("auth.login.complete") {
		if e.Decision == "deny" {
			t.Fatalf("a rejected proof audited a code: %+v", e)
		}
	}
	w.want(w.redeem(p, res.Code.Code, nil), http.StatusCreated, "")
}

func TestRedeemConcurrentSameCode(t *testing.T) {
	t.Parallel()
	w := newSessionWorld(t)
	res := w.bootstrap("alex")
	phones := []*newPhone{w.phoneKeys(), w.phoneKeys()}
	statuses := make([]int, 2)
	var wg sync.WaitGroup
	for i, p := range phones {
		wg.Go(func() {
			statuses[i] = w.redeem(p, res.Code.Code, nil).StatusCode
		})
	}
	wg.Wait()
	created, refused := 0, 0
	for _, s := range statuses {
		switch s {
		case http.StatusCreated:
			created++
		case http.StatusUnauthorized:
			refused++
		}
	}
	if created != 1 || refused != 1 {
		t.Fatalf("statuses %v, want one 201 and one 401", statuses)
	}
}

func TestRedeemRejectsDisabledPerson(t *testing.T) {
	t.Parallel()
	w := newSessionWorld(t)
	res := w.bootstrap("alex")
	want := w.rejectedBody()
	w.sql(`UPDATE principals SET status = 'disabled', disabled_at = now() WHERE id = $1`, res.OwnerID.String())
	r := w.redeem(w.phoneKeys(), res.Code.Code, nil)
	w.want(r, http.StatusUnauthorized, "unauthenticated")
	if got := withoutCorrelation(t, r); got != want {
		t.Fatalf("body %s, want %s", got, want)
	}
	if n := w.attempts(res.Code.ID); n != 0 {
		t.Fatal("a rejected person counted an attempt")
	}
}

func TestIssueExpiresOlderCodes(t *testing.T) {
	t.Parallel()
	w := newSessionWorld(t)
	res := w.bootstrap("alex")
	second, err := w.id.Logins.IssueRecoveryCode(context.Background(), "alex")
	if err != nil {
		t.Fatal(err)
	}
	if second.Purpose != app.PurposeRecovery || second.Code == res.Code.Code {
		t.Fatalf("second code %+v", second)
	}
	if n := w.count(`SELECT count(*) FROM enrollment_codes WHERE user_id = $1 AND used_at IS NULL AND expires_at > $2`, res.OwnerID.String(), w.clk.Now()); n != 1 {
		t.Fatalf("%d valid codes, want 1", n)
	}
	w.want(w.redeem(w.phoneKeys(), res.Code.Code, nil), http.StatusUnauthorized, "unauthenticated")
	w.want(w.redeem(w.phoneKeys(), second.Code, nil), http.StatusCreated, "")
}

// TestRecoveryCommand_Redeem: the recovery code of bothy setup issue-enrollment-code is redeemed
// like a first_login code, for an owner with and without a local identity; the audit entry is the
// system action owner.recover without actor (the exit codes are tested in cmd/bothy).
func TestRecoveryCommand_Redeem(t *testing.T) {
	t.Parallel()
	w := newSessionWorld(t)
	res := w.bootstrap("alex")
	w.want(w.redeem(w.phoneKeys(), res.Code.Code, nil), http.StatusCreated, "")
	rec, err := w.id.Logins.IssueRecoveryCode(context.Background(), "alex")
	if err != nil {
		t.Fatal(err)
	}
	m := w.want(w.redeem(w.phoneKeys(), rec.Code, nil), http.StatusCreated, "")
	if m["device"].(map[string]any)["state"] != "active" {
		t.Fatalf("recovered device %v", m["device"])
	}
	entries := w.audits("owner.recover")
	if len(entries) != 1 || entries[0].Details["purpose"] != "recovery" || entries[0].ResourceID != res.OwnerID.String() ||
		strings.Contains(entries[0].Raw, rec.Code) {
		t.Fatalf("owner.recover %+v", entries)
	}
	if n := w.count(`SELECT count(*) FROM audit_events WHERE action = 'owner.recover' AND actor_principal_id IS NULL`); n != 1 {
		t.Fatal("owner.recover with an actor")
	}
	// An owner without a local identity recovers without one.
	sam := w.person("sam", true)
	rec2, err := w.id.Logins.IssueRecoveryCode(context.Background(), "sam")
	if err != nil {
		t.Fatal(err)
	}
	w.want(w.redeem(w.phoneKeys(), rec2.Code, nil), http.StatusCreated, "")
	if n := w.count(`SELECT count(*) FROM auth_sessions WHERE user_id = $1 AND identity_id IS NULL`, sam.String()); n != 1 {
		t.Fatal("recovery of an owner without local identity")
	}
	// Not an owner, unknown, or setup pending: refused without a code.
	w.person("robin", false)
	for _, h := range []string{"robin", "nobody"} {
		if _, err := w.id.Logins.IssueRecoveryCode(context.Background(), h); !errors.Is(err, app.ErrNotActiveOwner) {
			t.Fatalf("%s: %v, want ErrNotActiveOwner", h, err)
		}
	}
	fresh := newSessionWorld(t)
	fresh.instanceRow()
	if _, err := fresh.id.Logins.IssueRecoveryCode(context.Background(), "alex"); !errors.Is(err, app.ErrSetupPending) {
		t.Fatalf("before setup: %v, want ErrSetupPending", err)
	}
}

func TestBootstrapAuditEntry(t *testing.T) {
	t.Parallel()
	w := newSessionWorld(t)
	res := w.bootstrap("alex")
	entries := w.audits("instance.bootstrap")
	if len(entries) != 1 || entries[0].Decision != "allow" {
		t.Fatalf("instance.bootstrap %+v", entries)
	}
	d := entries[0].Details
	if d["owner_principal_id"] != res.OwnerID.String() || d["enrollment_code_id"] != res.Code.ID.String() || d["local_provider_id"] != res.ProviderID.String() || len(d) != 3 {
		t.Fatalf("details %v", d)
	}
	canonical, _ := domain.NormalizeEnrollmentCode(res.Code.Code)
	if n := w.count(`SELECT count(*) FROM audit_events WHERE details::text LIKE '%' || $1 || '%' OR details::text LIKE '%' || $2 || '%'`, canonical, res.Code.Code); n != 0 {
		t.Fatal("the code is in the audit log")
	}
	if n := w.count(`SELECT count(*) FROM audit_events WHERE action = 'instance.bootstrap' AND actor_principal_id IS NULL`); n != 1 {
		t.Fatal("instance.bootstrap with an actor")
	}
	// Owner, provider and identity as 08 section 4 lists them; the setup is marked.
	if n := w.count(`SELECT count(*) FROM owners WHERE user_id = $1 AND granted_by IS NULL AND reason = 'bootstrap' AND revoked_at IS NULL`, res.OwnerID.String()); n != 1 {
		t.Fatal("owner row")
	}
	if n := w.count(`SELECT count(*) FROM identity_providers WHERE id = $1 AND kind = 'local' AND name = 'Local accounts' AND enabled AND NOT allow_provisioning AND config = '{}'::jsonb`, res.ProviderID.String()); n != 1 {
		t.Fatal("local provider")
	}
	if n := w.count(`SELECT count(*) FROM instance WHERE setup_completed_at IS NOT NULL`); n != 1 {
		t.Fatal("setup not marked")
	}
	if n := w.count(`SELECT count(*) FROM enrollment_codes WHERE id = $1 AND purpose = 'first_login' AND created_by IS NULL AND length(code_hash) = 32`, res.Code.ID.String()); n != 1 {
		t.Fatal("first_login code")
	}
	// Nothing outside the response carries the code: no event, no log line.
	if n := w.count(`SELECT count(*) FROM event_log WHERE payload::text LIKE '%' || $1 || '%'`, canonical); n != 0 {
		t.Fatal("the code is in the event log")
	}
	if text := w.logText(); strings.Contains(text, canonical) || strings.Contains(text, res.Code.Code) {
		t.Fatal("the code is in the logs")
	}
	if _, err := w.id.Logins.BootstrapOwner(context.Background(), app.BootstrapOwnerCmd{Handle: "sam", DisplayName: "Sam"}); !errors.Is(err, app.ErrSetupCompleted) {
		t.Fatalf("second bootstrap: %v", err)
	}
}

// logText returns every captured log record of the world as text.
func (w *sworld) logText() string {
	var all []string
	for _, r := range append(w.logs.Records(), w.h.Logs.Records()...) {
		all = append(all, fmt.Sprint(r))
	}
	return strings.Join(all, "\n")
}

// failingAudit fails Record for one action, the way a failing audit write in step 5 would.
type failingAudit struct {
	auditing.Recorder
	action string
}

func (f failingAudit) Record(ctx context.Context, e auditing.Entry) error {
	if e.Action == f.action {
		return errs.New(errs.CodeUnavailable, "audit down")
	}
	return f.Recorder.Record(ctx, e)
}

// noKeys is a SigningKeys whose keys exist.
type noKeys struct{}

func (noKeys) EnsureActive(context.Context) error { return nil }

func TestBootstrapIsAtomic(t *testing.T) {
	t.Parallel()
	w := newSessionWorld(t)
	w.instanceRow()
	repo := postgres.NewRepository(w.pool)
	svc, err := app.NewService(sessCfg, app.ServiceDeps{
		People: repo, Devices: repo, Providers: repo, Identities: repo, Enrollment: w.id.Enrollment, Auth: w.id.Sessions, DPoP: w.id.DPoP,
		Instance: instanceapp.NewState(instancepg.NewRepository(w.pool), w.uow), Keys: noKeys{}, Locales: locale.Parser{}, UoW: w.uow,
		Authorizer: w.az, Audit: failingAudit{Recorder: w.rec, action: "instance.bootstrap"}, Clock: w.clk,
		Sessions: repo, Pairings: repo, Pepper: testPepper,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.BootstrapOwner(context.Background(), app.BootstrapOwnerCmd{Handle: "alex", DisplayName: "Alex"}); err == nil {
		t.Fatal("bootstrap with a failing audit write succeeded")
	}
	n := w.count(`SELECT (SELECT count(*) FROM principals) + (SELECT count(*) FROM users) + (SELECT count(*) FROM owners) +
		(SELECT count(*) FROM user_identities) + (SELECT count(*) FROM identity_providers) + (SELECT count(*) FROM enrollment_codes) +
		(SELECT count(*) FROM audit_events WHERE action = 'instance.bootstrap') + (SELECT count(*) FROM instance WHERE setup_completed_at IS NOT NULL)`)
	if n != 0 {
		t.Fatalf("%d rows after the failed bootstrap, want none", n)
	}
}

func TestBootstrapRejectsInvalidInput(t *testing.T) {
	t.Parallel()
	w := newSessionWorld(t)
	w.instanceRow()
	for _, cmd := range []app.BootstrapOwnerCmd{
		{Handle: "Alex", DisplayName: "Alex"}, {Handle: "alex", DisplayName: " "}, {Handle: "alex", DisplayName: "Al\u202eex"},
		{Handle: "alex", DisplayName: "Alex", Locale: "not a tag"}, {Handle: "alex", DisplayName: "Alex", TimeZone: "Local"},
		{Handle: strings.Repeat("a", 33), DisplayName: "Alex"},
	} {
		_, err := w.id.Logins.BootstrapOwner(context.Background(), cmd)
		if e, ok := errs.As(err); !ok || e.Code != errs.CodeValidationFailed {
			t.Errorf("%+v: %v, want validation_failed", cmd, err)
		}
	}
	if n := w.count(`SELECT count(*) FROM principals`); n != 0 {
		t.Fatal("invalid input created a principal")
	}
}

func TestCompleteLoginDeviceRules(t *testing.T) {
	t.Parallel()
	w := newSessionWorld(t)
	alex := w.person("alex", false)
	sam := w.person("sam", false)
	ctx := context.Background()
	login := func(user ids.ID, p *newPhone, proofJKT string, trust app.DeviceTrust) (app.LoginResult, error) {
		return w.id.Logins.CompleteLogin(ctx, app.LoginCompletion{
			UserID: user, AuthMethod: identity.AuthPasskey, ProofJKT: proofJKT, Trust: trust,
			Device: identity.RegisterDevice{DeviceID: p.id, Name: "Alex Phone", Platform: identity.PlatformIOS, DPoPKey: p.pub},
		})
	}
	confirmed := w.phoneKeys()
	r, err := login(alex, confirmed, confirmed.jkt, app.TrustConfirmed)
	if err != nil || !r.NewDevice || r.Device.State != identity.DeviceStateActive {
		t.Fatalf("confirmed: %+v, %v", r, err)
	}
	pending := w.phoneKeys()
	r, err = login(alex, pending, pending.jkt, app.TrustUnconfirmed)
	if err != nil || !r.NewDevice || r.Device.State != identity.DeviceStatePending {
		t.Fatalf("unconfirmed: %+v, %v", r, err)
	}
	var approved int
	w.scalar(&approved, `SELECT count(*) FROM devices WHERE id = $1 AND approved_at IS NULL`, pending.id.String())
	if approved != 1 {
		t.Fatal("a pending device has approved_at")
	}
	// The known pending device with the same key is reused unchanged, even by a confirming path.
	r, err = login(alex, pending, pending.jkt, app.TrustConfirmed)
	if err != nil || r.NewDevice || r.Device.State != identity.DeviceStatePending {
		t.Fatalf("known device: %+v, %v", r, err)
	}
	other := w.phoneKeys()
	other.id = confirmed.id
	for name, c := range map[string]struct {
		user ids.ID
		p    *newPhone
		jkt  string
		code errs.Code
	}{
		"another person's device": {sam, confirmed, confirmed.jkt, errs.CodeConflict},
		"other key":               {alex, other, other.jkt, errs.CodeConflict},
		"proof key mismatch":      {alex, w.phoneKeys(), "not-the-key", errs.CodeInvalidDPoPProof},
	} {
		_, err := login(c.user, c.p, c.jkt, app.TrustConfirmed)
		if e, ok := errs.As(err); !ok || e.Code != c.code {
			t.Errorf("%s: %v, want %s", name, err, c.code)
		}
	}
	w.sql(`UPDATE devices SET state = 'revoked', revoked_at = now(), revoke_reason = 'revoked_by_owner' WHERE id = $1`, confirmed.id.String())
	if _, err := login(alex, confirmed, confirmed.jkt, app.TrustConfirmed); errs.KindOf(err) != errs.KindConflict {
		t.Errorf("revoked device: %v, want conflict", err)
	}
	w.sql(`UPDATE principals SET status = 'disabled', disabled_at = now() WHERE id = $1`, sam.String())
	if _, err := login(sam, w.phoneKeys(), "", app.TrustConfirmed); errs.KindOf(err) != errs.KindUnauthenticated {
		t.Errorf("disabled person: %v, want unauthenticated", err)
	}
	// Exactly one allow entry per completed login (three above).
	if n := len(w.audits("auth.login.complete")); n != 3 {
		t.Fatalf("%d audit entries, want 3", n)
	}
}

// codePath is the operation createEnrollmentCode for user.
func codePath(user ids.ID) string { return "/api/v1/users/" + user.String() + "/enrollment-codes" }

func TestCreateEnrollmentCodeRights(t *testing.T) {
	t.Parallel()
	w := newSessionWorld(t)
	w.bootstrap("owner")
	admin := w.admin()
	kim := w.person("kim", false)
	sam := w.person("sam", false)
	// A local account without a device: first_login; with a device: recovery; without a local
	// identity: identity_link.
	created := w.want(admin.postWith(usersPath, `{"handle":"robin","display_name":"Robin","local_account":true}`, "Idempotency-Key", ids.New().String()), http.StatusCreated, "")
	robin := ids.MustParse(created["user"].(map[string]any)["id"].(string))
	m := w.want(admin.postWith(codePath(robin), "", "Idempotency-Key", ids.New().String()), http.StatusCreated, "")
	if m["purpose"] != "first_login" || !displayForm.MatchString(m["code"].(string)) {
		t.Fatalf("code %v", m)
	}
	w.want(w.redeem(w.phoneKeys(), m["code"].(string), nil), http.StatusCreated, "")
	m = w.want(admin.postWith(codePath(robin), "", "Idempotency-Key", ids.New().String()), http.StatusCreated, "")
	if m["purpose"] != "recovery" {
		t.Fatalf("purpose %v, want recovery", m["purpose"])
	}
	m = w.want(admin.postWith(codePath(kim), "", "Idempotency-Key", ids.New().String()), http.StatusCreated, "")
	if m["purpose"] != "identity_link" {
		t.Fatalf("purpose %v, want identity_link", m["purpose"])
	}
	// An identity_link code is not redeemable with device keys only.
	w.want(w.redeem(w.phoneKeys(), m["code"].(string), nil), http.StatusUnauthorized, "unauthenticated")
	// Without fresh step-up, as a non-owner, as an agent, for a disabled person.
	w.clk.Advance(sessCfg.StepUpMaxAge + time.Second)
	w.want(admin.postWith(codePath(kim), "", "Idempotency-Key", ids.New().String()), http.StatusUnauthorized, "step_up_required")
	kimPhone := w.phone(kim)
	kimPhone.stepUp()
	w.want(kimPhone.postWith(codePath(sam), "", "Idempotency-Key", ids.New().String()), http.StatusForbidden, "")
	admin.stepUp()
	w.sql(`UPDATE principals SET status = 'disabled', disabled_at = now() WHERE id = $1`, sam.String())
	p := w.want(admin.postWith(codePath(sam), "", "Idempotency-Key", ids.New().String()), http.StatusConflict, "conflict")
	if p["reason"] != "user_not_active" {
		t.Fatalf("reason %v", p["reason"])
	}
	w.want(admin.postWith(codePath(ids.New()), "", "Idempotency-Key", ids.New().String()), http.StatusNotFound, "not_found")
	allows := 0
	for _, e := range w.audits("user.update") {
		if e.Decision == "allow" {
			allows++
			if e.Details["enrollment_code_id"] == nil || e.Details["purpose"] == nil {
				t.Fatalf("user.update entry %+v", e)
			}
		}
	}
	if allows != 3 {
		t.Fatalf("%d allow entries of user.update, want 3", allows)
	}
}

func TestCreateEnrollmentCodeAgentToken(t *testing.T) {
	t.Parallel()
	w := newSessionWorld(t)
	w.bootstrap("owner")
	kim := w.person("kim", false)
	// API tokens reach HTTP with M1-44; the use case decides the same for an agent principal.
	id := ids.New()
	w.sql(`INSERT INTO principals (id, kind, handle, display_name) VALUES ($1, 'agent', 'helper', 'Helper')`, id.String())
	agent := actor.Actor{ID: id, Kind: actor.KindAgent, TokenID: ids.New()}
	_, err := w.id.Logins.CreateEnrollmentCode(actor.WithActor(context.Background(), agent), agent, kim)
	// Until M1-44 builds API tokens no agent token is valid, so authorize answers 401 instead of
	// the 403 a valid agent token gets; either way nothing is issued.
	if k := errs.KindOf(err); k != errs.KindForbidden && k != errs.KindUnauthenticated {
		t.Fatalf("agent: %v, want a denial", err)
	}
	if n := w.count(`SELECT count(*) FROM enrollment_codes WHERE user_id = $1`, kim.String()); n != 0 {
		t.Fatal("an agent issued a code")
	}
}

func TestOneTimeSecretNotStoredForReplay(t *testing.T) {
	t.Parallel()
	w := newSessionWorld(t)
	w.bootstrap("owner")
	admin := w.admin()
	kim := w.person("kim", false)
	key := ids.New().String()
	m := w.want(admin.postWith(codePath(kim), "", "Idempotency-Key", key), http.StatusCreated, "")
	code := m["code"].(string)
	p := w.want(admin.postWith(codePath(kim), "", "Idempotency-Key", key), http.StatusConflict, "conflict")
	if p["reason"] != "one_time_secret" {
		t.Fatalf("replay %v", p)
	}
	userKey := ids.New().String()
	body := `{"handle":"robin","display_name":"Robin","local_account":true}`
	u := w.want(admin.postWith(usersPath, body, "Idempotency-Key", userKey), http.StatusCreated, "")
	enr := u["enrollment"].(map[string]any)
	if enr["purpose"] != "first_login" {
		t.Fatalf("enrollment %v", enr)
	}
	p = w.want(admin.postWith(usersPath, body, "Idempotency-Key", userKey), http.StatusConflict, "conflict")
	if p["reason"] != "one_time_secret" {
		t.Fatalf("replay %v", p)
	}
	for _, c := range []string{code, enr["code"].(string)} {
		canonical, _ := domain.NormalizeEnrollmentCode(c)
		if n := w.count(`SELECT count(*) FROM idempotency_keys WHERE response_body::text LIKE '%' || $1 || '%' OR
			response_body::text LIKE '%' || $2 || '%'`, c, canonical); n != 0 {
			t.Fatal("idempotency_keys holds a code")
		}
	}
}

func TestCreateUserLocalAccountIssuesCode(t *testing.T) {
	t.Parallel()
	w := newSessionWorld(t)
	w.bootstrap("owner")
	admin := w.admin()
	u := w.want(admin.postWith(usersPath, `{"handle":"robin","display_name":"Robin","local_account":true}`, "Idempotency-Key", ids.New().String()), http.StatusCreated, "")
	user := u["user"].(map[string]any)
	enr := u["enrollment"].(map[string]any)
	if !displayForm.MatchString(enr["code"].(string)) || enr["purpose"] != "first_login" {
		t.Fatalf("enrollment %v", enr)
	}
	if n := w.count(`SELECT count(*) FROM user_identities i JOIN identity_providers p ON p.id = i.provider_id
		WHERE i.user_id = $1 AND i.subject = 'robin' AND p.kind = 'local'`, user["id"].(string)); n != 1 {
		t.Fatal("no local identity")
	}
	m := w.want(w.redeem(w.phoneKeys(), enr["code"].(string), nil), http.StatusCreated, "")
	if m["session"].(map[string]any)["user_id"] != user["id"] {
		t.Fatalf("session %v", m["session"])
	}
	// A disabled local provider refuses local accounts.
	w.sql(`UPDATE identity_providers SET enabled = false WHERE kind = 'local'`)
	p := w.want(admin.postWith(usersPath, `{"handle":"kim","display_name":"Kim","local_account":true}`, "Idempotency-Key", ids.New().String()), http.StatusConflict, "conflict")
	if p["reason"] != "local_provider_disabled" {
		t.Fatalf("reason %v", p["reason"])
	}
}

// TestRedeemWithDisabledLocalProvider: a disabled local provider closes the code path of a person
// with a local identity (08 section 3.3, R-ID-12): the redemption gets the same 401 as every
// rejected code and starts no session, and createEnrollmentCode refuses to issue a code that
// could not be redeemed. A recovery code issued on the host still lets an owner in, without the
// closed identity, so the host stays the root of trust (red team M1-37).
func TestRedeemWithDisabledLocalProvider(t *testing.T) {
	t.Parallel()
	w := newSessionWorld(t)
	res := w.bootstrap("owner")
	admin := w.admin()
	u := w.want(admin.postWith(usersPath, `{"handle":"robin","display_name":"Robin","local_account":true}`, "Idempotency-Key", ids.New().String()), http.StatusCreated, "")
	robin := u["user"].(map[string]any)["id"].(string)
	code := u["enrollment"].(map[string]any)["code"].(string)
	want := w.rejectedBody()
	w.sql(`UPDATE identity_providers SET enabled = false WHERE kind = 'local'`)
	r := w.redeem(w.phoneKeys(), code, nil)
	w.want(r, http.StatusUnauthorized, "unauthenticated")
	if got := withoutCorrelation(t, r); got != want {
		t.Fatalf("body %s, want %s", got, want)
	}
	if n := w.count(`SELECT count(*) FROM auth_sessions WHERE user_id = $1`, robin); n != 0 {
		t.Fatal("a session through the disabled local provider")
	}
	p := w.want(admin.postWith(codePath(ids.MustParse(robin)), "", "Idempotency-Key", ids.New().String()), http.StatusConflict, "conflict")
	if p["reason"] != "local_provider_disabled" {
		t.Fatalf("reason %v, want local_provider_disabled", p["reason"])
	}
	// The host recovery of the owner works and binds no identity of the closed provider.
	rec, err := w.id.Logins.IssueRecoveryCode(context.Background(), "owner")
	if err != nil {
		t.Fatal(err)
	}
	m := w.want(w.redeem(w.phoneKeys(), rec.Code, nil), http.StatusCreated, "")
	sid := m["session"].(map[string]any)["id"].(string)
	if n := w.count(`SELECT count(*) FROM auth_sessions WHERE id = $1 AND user_id = $2 AND identity_id IS NULL`, sid, res.OwnerID.String()); n != 1 {
		t.Fatal("host recovery bound the closed identity or failed")
	}
}
