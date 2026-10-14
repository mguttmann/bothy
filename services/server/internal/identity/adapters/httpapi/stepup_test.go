// SPDX-License-Identifier: Apache-2.0

package httpapi_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/adapters/identitytest"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/jose"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
)

// The step-up paths of docs/api/openapi.yaml.
const (
	challengePath = "/api/v1/auth/step-up/challenge"
	deviceKeyPath = "/api/v1/auth/step-up/device-key"
)

// stepClient is a client whose device has a step_up key (nil for none).
type stepClient struct {
	*client
	step *identitytest.Key
}

// loginDevice registers a device of user on platform in state, with a step_up key when stepUp is
// true, and starts an app session on it.
func (w *sworld) loginDevice(user ids.ID, state identity.DeviceState, platform identity.Platform, stepUp bool) *stepClient {
	w.t.Helper()
	key, pub := identitytest.NewDeviceKey(jose.AlgES256)
	reg := identity.RegisterDevice{DeviceID: ids.New(), UserID: user, Name: "Alex Phone", Platform: platform, DPoPKey: pub, State: state}
	var step *identitytest.Key
	if stepUp {
		k, p := identitytest.NewDeviceKey(jose.AlgES256)
		step, reg.StepUpKey = k, &p
	}
	if state == identity.DeviceStateActive {
		reg.ApprovedBy = &user
	}
	dev, err := w.id.Devices.Register(context.Background(), reg)
	if err != nil {
		w.t.Fatal(err)
	}
	set, err := w.id.Sessions.StartSession(context.Background(), identity.StartSession{UserID: user, DeviceID: dev.ID, AuthMethod: identity.AuthPasskey})
	if err != nil {
		w.t.Fatal(err)
	}
	c := &client{w: w, key: key, user: user, device: dev.ID}
	c.take(set)
	return &stepClient{client: c, step: step}
}

// phone logs user in on an active iOS device with a step_up key.
func (w *sworld) phone(user ids.ID) *stepClient {
	w.t.Helper()
	return w.loginDevice(user, identity.DeviceStateActive, identity.PlatformIOS, true)
}

// challenge asks for a challenge and returns its id and value.
func (c *stepClient) challenge() (string, string) {
	c.w.t.Helper()
	m := c.w.want(c.call(http.MethodPost, challengePath, ""), http.StatusOK, "")
	return m["challenge_id"].(string), m["challenge"].(string)
}

// sign returns the confirmation of value for session sid with iat now, signed by key.
func (c *stepClient) sign(key *identitytest.Key, value string, sid ids.ID) string {
	body, err := json.Marshal(map[string]any{"challenge": value, "sid": sid.String(), "iat": json.Number(strconv.FormatInt(c.w.clk.Now().Unix(), 10))})
	if err != nil {
		c.w.t.Fatal(err)
	}
	return identitytest.SignJWS(key, jose.Header{Alg: jose.AlgES256}, body)
}

// confirm sends the confirmation sig for the challenge id.
func (c *stepClient) confirm(id, sig string) *httpservertest.Response {
	c.w.t.Helper()
	body, _ := json.Marshal(map[string]string{"challenge_id": id, "signature": sig})
	return c.call(http.MethodPost, deviceKeyPath, string(body))
}

// stepUp runs a full step-up and expects 200.
func (c *stepClient) stepUp() {
	c.w.t.Helper()
	id, value := c.challenge()
	c.w.want(c.confirm(id, c.sign(c.step, value, c.session)), http.StatusOK, "")
}

// actor authenticates the current access token of c the way the chain does.
func (c *client) actor() actor.Actor {
	c.w.t.Helper()
	a, err := c.w.id.Authenticator.Authenticate(context.Background(), actor.Credentials{
		Method: http.MethodGet, HTU: httpservertest.PublicURL + "/api/v1/me", Authorization: "DPoP " + c.access,
		DPoP: c.proof(c.key, http.MethodGet, "/api/v1/me", c.access),
	})
	if err != nil {
		c.w.t.Fatal(err)
	}
	return a
}

// failedCount returns the failed step-up challenges of session id.
func (w *sworld) failedCount(id ids.ID) int {
	w.t.Helper()
	return w.count(`SELECT count(*) FROM auth_challenges WHERE session_id = $1 AND status = 'failed'`, id.String())
}

// decide authorizes act on r for a with the real authorizer.
func (w *sworld) decide(a actor.Actor, act authorize.Action, r authorize.Resource) authorize.Decision {
	w.t.Helper()
	d, err := w.az.Authorize(context.Background(), a, act, r)
	if err != nil {
		w.t.Fatal(err)
	}
	return d
}

// instance is the resource of instance-wide owner actions.
var instance = authorize.Resource{Kind: authorize.ResourceInstance}

// parseMS parses a Timestamp of the API.
func parseMS(t *testing.T, v any) time.Time {
	t.Helper()
	s, _ := v.(string)
	at, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		t.Fatalf("timestamp %v: %v", v, err)
	}
	return at
}

// TestStepUp_DeviceKeySuccess: a confirmation by the step_up key sets step_up_at and step_up_method
// = device_key, answers valid_until = step_up_at + auth.step_up_max_age, audits allow, and the next
// request's actor carries the new step_up_at.
func TestStepUp_DeviceKeySuccess(t *testing.T) {
	t.Parallel()
	w := newSessionWorld(t)
	c := w.phone(w.person("alex", false))
	if !c.actor().StepUpAt.IsZero() {
		t.Fatal("step-up before confirming")
	}
	id, value := c.challenge()
	var exp time.Time
	w.scalar(&exp, `SELECT expires_at FROM auth_challenges WHERE id = $1`, id)
	if !exp.Equal(w.clk.Now().Add(sessCfg.StepUpChallengeTTL)) {
		t.Fatalf("expires_at %v, want now + auth.step_up_challenge_ttl", exp)
	}
	if n := w.count(`SELECT count(*) FROM auth_challenges WHERE id = $1 AND challenge_hash = $2`, id, []byte(value)); n != 0 {
		t.Fatal("challenge stored in clear")
	}
	w.clk.Advance(3 * time.Second)
	m := w.want(c.confirm(id, c.sign(c.step, value, c.session)), http.StatusOK, "")
	at, until := parseMS(t, m["step_up_at"]), parseMS(t, m["valid_until"])
	if !at.Equal(w.clk.Now().Truncate(time.Millisecond)) || !until.Equal(at.Add(sessCfg.StepUpMaxAge)) {
		t.Fatalf("step_up_at %v valid_until %v, want now and now + auth.step_up_max_age", at, until)
	}
	var method, status string
	var stored time.Time
	if err := w.owner.QueryRow(context.Background(), `SELECT step_up_at, step_up_method FROM auth_sessions WHERE id = $1`, c.session.String()).
		Scan(&stored, &method); err != nil {
		t.Fatal(err)
	}
	if method != identity.StepUpMethodDeviceKey || !stored.Equal(w.clk.Now()) {
		t.Fatalf("stored step-up %v %q", stored, method)
	}
	w.scalar(&status, `SELECT status FROM auth_challenges WHERE id = $1`, id)
	if status != "used" {
		t.Fatalf("challenge %s, want used", status)
	}
	if a := c.actor(); !a.StepUpAt.Equal(w.clk.Now()) || a.StepUpMethod != identity.StepUpMethodDeviceKey {
		t.Fatalf("actor step-up %v %q", a.StepUpAt, a.StepUpMethod)
	}
	me := w.want(c.call(http.MethodGet, "/api/v1/me", ""), http.StatusOK, "")
	if !parseMS(t, me["step_up_valid_until"]).Equal(until) {
		t.Fatalf("getMe step_up_valid_until %v, want %v", me["step_up_valid_until"], until)
	}
	var allows []sauditRow
	for _, r := range w.audits("auth.step_up") {
		if r.Decision == "allow" {
			allows = append(allows, r)
		}
	}
	if len(allows) != 2 || allows[0].ResourceID != c.session.String() || allows[1].Details["step"] != "confirm" ||
		allows[1].Details["method"] != identity.StepUpMethodDeviceKey || allows[1].Details["challenge_id"] != id {
		t.Fatalf("audit allows %+v", allows)
	}
}

// TestStepUp_IDT18_Missing: an owner session without step-up gets step_up_required for
// identity_provider.manage (instance, owner, step-up); after a step-up it is allowed.
func TestStepUp_IDT18_Missing(t *testing.T) {
	t.Parallel()
	w := newSessionWorld(t)
	c := w.phone(w.person("alex", true))
	if d := w.decide(c.actor(), authorize.ActionIdentityProviderManage, instance); d.Outcome != authorize.OutcomeStepUpRequired ||
		d.Reason != "step_up_required" {
		t.Fatalf("without step-up: %+v", d)
	}
	c.stepUp()
	if d := w.decide(c.actor(), authorize.ActionIdentityProviderManage, instance); !d.Allowed() {
		t.Fatalf("after step-up: %+v", d)
	}
}

// TestStepUp_AZM36_TooOld: AZ-M36, an owner whose step-up is older than auth.step_up_max_age gets
// step_up_required for grant.create on the Marketing channel (structure from the fake structure
// source, principal, session and step-up real).
func TestStepUp_AZM36_TooOld(t *testing.T) {
	t.Parallel()
	w := newSessionWorld(t)
	marketing, kampagnen := ids.New(), ids.New()
	w.structure.PutSpace(marketing)
	w.structure.PutChannel(authz.ChannelFacts{ID: kampagnen, SpaceID: marketing, Kind: authz.ChannelText})
	r := authorize.Resource{Kind: authorize.ResourceChannel, ID: kampagnen, Grant: &authorize.GrantFacts{
		SubjectKind: "user", Level: "read", TargetKind: "channel", TargetChannelID: kampagnen}}
	c := w.phone(w.person("alex", true))
	c.stepUp()
	w.clk.Advance(sessCfg.StepUpMaxAge)
	if d := w.decide(c.actor(), authorize.ActionGrantCreate, r); !d.Allowed() {
		t.Fatalf("step-up exactly auth.step_up_max_age old: %+v", d)
	}
	w.clk.Advance(time.Second)
	c.rotate()
	if d := w.decide(c.actor(), authorize.ActionGrantCreate, r); d.Outcome != authorize.OutcomeStepUpRequired || d.Reason != "step_up_required" {
		t.Fatalf("step-up too old: %+v", d)
	}
}

// wantForbiddenNotPossible checks the 403 forbidden with reason step_up_not_possible of err.
func wantForbiddenNotPossible(t *testing.T, err error) {
	t.Helper()
	e, ok := errs.As(err)
	if !ok || e.Code != errs.CodeForbidden || e.Reason != "step_up_not_possible" {
		t.Fatalf("error %v, want 403 forbidden with reason step_up_not_possible", err)
	}
}

// notPossibleAudits returns the deny entries of auth.step_up with reason step_up_not_possible.
func (w *sworld) notPossibleAudits() []sauditRow {
	w.t.Helper()
	var out []sauditRow
	for _, r := range w.audits("auth.step_up") {
		if r.Decision == "deny" && r.Reason == "step_up_not_possible" {
			out = append(out, r)
		}
	}
	return out
}

// TestStepUp_IDT18_AgentToken: an agent actor (fake, no session) gets 403 forbidden with audit
// reason step_up_not_possible on both operations.
func TestStepUp_IDT18_AgentToken(t *testing.T) {
	t.Parallel()
	w := newSessionWorld(t)
	agent := actor.Actor{ID: ids.New(), Kind: actor.KindAgent, TokenID: ids.New()}
	_, err := w.id.StepUps.CreateChallenge(context.Background(), agent)
	wantForbiddenNotPossible(t, err)
	_, err = w.id.StepUps.CompleteWithDeviceKey(context.Background(), agent, ids.New(), "x.y.z")
	wantForbiddenNotPossible(t, err)
	if got := w.notPossibleAudits(); len(got) == 0 {
		t.Fatal("no audit deny with step_up_not_possible")
	}
	// A person without a session is no different.
	_, err = w.id.StepUps.CreateChallenge(context.Background(), actor.Actor{ID: ids.New(), Kind: actor.KindUser})
	wantForbiddenNotPossible(t, err)
}

// TestStepUp_IDT29_CliDevice: a session of a cli device gets 403 forbidden (step_up_not_possible)
// for the challenge and the completion; a step-up action from it stays 401 step_up_required.
func TestStepUp_IDT29_CliDevice(t *testing.T) {
	t.Parallel()
	w := newSessionWorld(t)
	c := w.loginDevice(w.person("alex", true), identity.DeviceStateActive, identity.PlatformCLI, false)
	m := w.want(c.call(http.MethodPost, challengePath, ""), http.StatusForbidden, "forbidden")
	if m["reason"] != "step_up_not_possible" {
		t.Fatalf("reason %v", m["reason"])
	}
	w.want(c.confirm(ids.New().String(), "a.b.c"), http.StatusForbidden, "forbidden")
	if got := w.notPossibleAudits(); len(got) == 0 || got[0].ResourceID != c.session.String() {
		t.Fatalf("audit %+v", got)
	}
	if d := w.decide(c.actor(), authorize.ActionIdentityProviderManage, instance); d.Outcome != authorize.OutcomeStepUpRequired {
		t.Fatalf("step-up action from a cli device: %+v", d)
	}
}

// TestStepUp_IDT18_WrongKey: a confirmation signed by the dpop key is 401 step_up_required and
// counted; the session keeps no step-up.
func TestStepUp_IDT18_WrongKey(t *testing.T) {
	t.Parallel()
	w := newSessionWorld(t)
	c := w.phone(w.person("alex", false))
	id, value := c.challenge()
	w.want(c.confirm(id, c.sign(c.key, value, c.session)), http.StatusUnauthorized, "step_up_required")
	if n := w.failedCount(c.session); n != 1 {
		t.Fatalf("failed challenges %d, want 1", n)
	}
	if !c.actor().StepUpAt.IsZero() {
		t.Fatal("step-up set by a wrong key")
	}
	var deny []sauditRow
	for _, r := range w.audits("auth.step_up") {
		if r.Decision == "deny" {
			deny = append(deny, r)
		}
	}
	if len(deny) != 1 || deny[0].Reason != "step_up_required" || deny[0].Details["cause"] != "bad_signature" {
		t.Fatalf("audit deny %+v", deny)
	}
	// A wrong sid, signed by the right key, counts as well.
	id, value = c.challenge()
	w.want(c.confirm(id, c.sign(c.step, value, ids.New())), http.StatusUnauthorized, "step_up_required")
	if n := w.failedCount(c.session); n != 2 {
		t.Fatalf("failed challenges %d, want 2", n)
	}
}

// TestStepUp_ChallengeSingleUse: a second completion with the same challenge is 401 and not
// counted.
func TestStepUp_ChallengeSingleUse(t *testing.T) {
	t.Parallel()
	w := newSessionWorld(t)
	c := w.phone(w.person("alex", false))
	id, value := c.challenge()
	sig := c.sign(c.step, value, c.session)
	w.want(c.confirm(id, sig), http.StatusOK, "")
	w.want(c.confirm(id, sig), http.StatusUnauthorized, "step_up_required")
	if n := w.failedCount(c.session); n != 0 {
		t.Fatalf("failed challenges %d, want 0", n)
	}
}

// TestStepUp_ChallengeExpired: a challenge at expires_at is 401 and not counted.
func TestStepUp_ChallengeExpired(t *testing.T) {
	t.Parallel()
	w := newSessionWorld(t)
	c := w.phone(w.person("alex", false))
	id, value := c.challenge()
	w.clk.Advance(sessCfg.StepUpChallengeTTL)
	w.want(c.confirm(id, c.sign(c.step, value, c.session)), http.StatusUnauthorized, "step_up_required")
	if n := w.failedCount(c.session); n != 0 {
		t.Fatalf("failed challenges %d, want 0", n)
	}
	var status string
	w.scalar(&status, `SELECT status FROM auth_challenges WHERE id = $1`, id)
	if status != "issued" {
		t.Fatalf("expired challenge %s, want unchanged", status)
	}
}

// TestStepUp_ForeignChallenge: an unknown challenge, one of another session of the same person and
// one of another purpose are 401 and not counted.
func TestStepUp_ForeignChallenge(t *testing.T) {
	t.Parallel()
	w := newSessionWorld(t)
	alex := w.person("alex", false)
	c, other := w.phone(alex), w.phone(alex)
	id, value := other.challenge()
	w.want(c.confirm(id, c.sign(c.step, value, c.session)), http.StatusUnauthorized, "step_up_required")
	w.want(c.confirm(ids.New().String(), c.sign(c.step, value, c.session)), http.StatusUnauthorized, "step_up_required")
	login := ids.New()
	w.sql(`INSERT INTO auth_challenges (id, purpose, challenge_hash, expires_at) VALUES ($1, 'passkey_login', $2, $3)`,
		login.String(), []byte("hash of another ceremony"), w.clk.Now().Add(time.Hour))
	w.want(c.confirm(login.String(), c.sign(c.step, value, c.session)), http.StatusUnauthorized, "step_up_required")
	if n := w.failedCount(c.session) + w.failedCount(other.session); n != 0 {
		t.Fatalf("failed challenges %d, want 0", n)
	}
	var status string
	w.scalar(&status, `SELECT status FROM auth_challenges WHERE id = $1`, id)
	if status != "issued" {
		t.Fatalf("challenge of the other session %s, want issued", status)
	}
	// The challenge still works in its own session.
	w.want(other.confirm(id, other.sign(other.step, value, other.session)), http.StatusOK, "")
}

// TestStepUp_ConcurrentCompletion: two parallel completions of one challenge succeed exactly once.
func TestStepUp_ConcurrentCompletion(t *testing.T) {
	t.Parallel()
	w := newSessionWorld(t)
	c := w.phone(w.person("alex", false))
	a := c.actor()
	for round := range 5 {
		ch, err := w.id.StepUps.CreateChallenge(context.Background(), a)
		if err != nil {
			t.Fatal(err)
		}
		sig := c.sign(c.step, ch.Value, c.session)
		var wg sync.WaitGroup
		results := make([]error, 2)
		for i := range results {
			wg.Go(func() {
				_, results[i] = w.id.StepUps.CompleteWithDeviceKey(context.Background(), a, ch.ID, sig)
			})
		}
		wg.Wait()
		ok := 0
		for _, err := range results {
			if err == nil {
				ok++
			} else if e, isErr := errs.As(err); !isErr || e.Code != errs.CodeStepUpRequired {
				t.Fatalf("round %d: loser %v, want step_up_required", round, err)
			}
		}
		if ok != 1 {
			t.Fatalf("round %d: %d successes, want exactly 1", round, ok)
		}
	}
	if n := w.failedCount(c.session); n != 0 {
		t.Fatalf("failed challenges %d, want 0", n)
	}
}

// TestStepUp_FailLimitRevokesSession: failures older than auth.login_fail_window do not count;
// after auth.step_up_fail_limit counted failures the session is revoked with step_up_failed,
// session.revoked is appended and the answer is 401 session_revoked.
func TestStepUp_FailLimitRevokesSession(t *testing.T) {
	t.Parallel()
	w := newSessionWorld(t)
	c := w.phone(w.person("alex", false))
	fail := func(code string) {
		t.Helper()
		id, value := c.challenge()
		w.want(c.confirm(id, c.sign(c.key, value, c.session)), http.StatusUnauthorized, code)
	}
	for range sessCfg.StepUpFailLimit - 1 {
		fail("step_up_required")
	}
	w.clk.Advance(sessCfg.LoginFailWindow)
	c.rotate()
	for range sessCfg.StepUpFailLimit - 1 {
		fail("step_up_required")
	}
	if s := w.sessionState(c.session); s != "" {
		t.Fatalf("session revoked with %q although old failures left the window", s)
	}
	fail("session_revoked")
	if s := w.sessionState(c.session); s != "step_up_failed" {
		t.Fatalf("revoke_reason %q, want step_up_failed", s)
	}
	ev := w.revokedEvents()
	if len(ev) != 1 || ev[0]["resource"] != c.session.String() || !strings.Contains(ev[0]["payload"], `"step_up_failed"`) ||
		ev[0]["actor"] != c.user.String() {
		t.Fatalf("session.revoked %+v", ev)
	}
	if len(*w.ended) != 1 || (*w.ended)[0].Reason != identity.RevokeStepUpFailed {
		t.Fatalf("SessionEnded %+v", *w.ended)
	}
	w.want(c.call(http.MethodPost, challengePath, ""), http.StatusUnauthorized, "session_revoked")
	var revoked []sauditRow
	for _, r := range w.audits("auth.step_up") {
		if r.Decision == "deny" && r.Details["session_revoked"] == true {
			revoked = append(revoked, r)
		}
	}
	if len(revoked) != 1 {
		t.Fatalf("audit of the revoking failure %+v", revoked)
	}
}

// TestStepUp_BoundToSession: a step-up in one session satisfies neither another session of the same
// person nor its child session, and a step-up in the child does not count for the parent.
func TestStepUp_BoundToSession(t *testing.T) {
	t.Parallel()
	w := newSessionWorld(t)
	alex := w.person("alex", true)
	c, other := w.phone(alex), w.phone(alex)
	child := &stepClient{client: c.child(), step: c.step}
	c.stepUp()
	if !c.actor().StepUpAt.Equal(w.clk.Now()) {
		t.Fatal("no step-up in the confirming session")
	}
	for name, x := range map[string]*client{"other session": other.client, "child session": child.client} {
		if !x.actor().StepUpAt.IsZero() {
			t.Fatalf("%s carries the step-up", name)
		}
		if d := w.decide(x.actor(), authorize.ActionIdentityProviderManage, instance); d.Outcome != authorize.OutcomeStepUpRequired {
			t.Fatalf("%s: %+v", name, d)
		}
	}
	w.clk.Advance(time.Second)
	child.stepUp()
	if !child.actor().StepUpAt.Equal(w.clk.Now()) || c.actor().StepUpAt.Equal(w.clk.Now()) {
		t.Fatal("a child step-up must count only for the child")
	}
}

// TestStepUp_PendingDevice: a pending device gets 403 device_pending.
func TestStepUp_PendingDevice(t *testing.T) {
	t.Parallel()
	w := newSessionWorld(t)
	c := w.loginDevice(w.person("alex", false), identity.DeviceStatePending, identity.PlatformIOS, true)
	w.want(c.call(http.MethodPost, challengePath, ""), http.StatusForbidden, "device_pending")
	w.want(c.confirm(ids.New().String(), "a.b.c"), http.StatusForbidden, "device_pending")
}

// TestStepUp_NoStepUpKey: a device without step_up key gets 403 forbidden (step_up_not_possible,
// cause no_step_up_key) on completion.
func TestStepUp_NoStepUpKey(t *testing.T) {
	t.Parallel()
	w := newSessionWorld(t)
	c := w.loginDevice(w.person("alex", false), identity.DeviceStateActive, identity.PlatformAndroid, false)
	id, value := c.challenge()
	m := w.want(c.confirm(id, c.sign(c.key, value, c.session)), http.StatusForbidden, "forbidden")
	if m["reason"] != "step_up_not_possible" {
		t.Fatalf("reason %v", m["reason"])
	}
	got := w.notPossibleAudits()
	if len(got) != 1 || got[0].Details["cause"] != "no_step_up_key" {
		t.Fatalf("audit %+v", got)
	}
	if n := w.failedCount(c.session); n != 0 {
		t.Fatalf("failed challenges %d, want 0", n)
	}
}

// TestStepUp_NoSecretsInLogs: challenge values and signatures appear in no log and no audit detail.
func TestStepUp_NoSecretsInLogs(t *testing.T) {
	t.Parallel()
	w := newSessionWorld(t)
	c := w.phone(w.person("alex", false))
	var canaries []string
	id, value := c.challenge()
	sig := c.sign(c.step, value, c.session)
	canaries = append(canaries, value, sig)
	w.want(c.confirm(id, sig), http.StatusOK, "")
	id, value = c.challenge()
	bad := c.sign(c.key, value, c.session)
	canaries = append(canaries, value, bad)
	w.want(c.confirm(id, bad), http.StatusUnauthorized, "step_up_required")
	w.want(c.confirm(id, bad), http.StatusUnauthorized, "step_up_required")
	var all []string
	for _, r := range append(w.logs.Records(), w.h.Logs.Records()...) {
		all = append(all, fmt.Sprint(r))
	}
	for _, r := range w.audits("auth.step_up") {
		all = append(all, r.Raw)
	}
	text := strings.Join(all, "\n")
	for _, s := range canaries {
		if strings.Contains(text, s) {
			t.Fatal("challenge value or signature leaked into logs or audit")
		}
	}
	if len(all) == 0 {
		t.Fatal("nothing captured")
	}
}

// TestAuthCleanup_DeletesExpired: the job identity.auth_cleanup deletes challenges past expires_at,
// keeps open ones and keeps failed step-ups while they count inside auth.login_fail_window.
func TestAuthCleanup_DeletesExpired(t *testing.T) {
	t.Parallel()
	w := newSessionWorld(t)
	c := w.phone(w.person("alex", false))
	used, value := c.challenge()
	w.want(c.confirm(used, c.sign(c.step, value, c.session)), http.StatusOK, "")
	failed, value := c.challenge()
	w.want(c.confirm(failed, c.sign(c.key, value, c.session)), http.StatusUnauthorized, "step_up_required")
	open, _ := c.challenge()
	w.clk.Advance(sessCfg.StepUpChallengeTTL - time.Second)
	fresh, _ := c.challenge()
	w.clk.Advance(time.Second)
	n, err := w.id.AuthCleanup.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("deleted %d, want 2 (used and open)", n)
	}
	left := func() map[string]bool {
		out := map[string]bool{}
		rows, err := w.owner.Query(context.Background(), `SELECT id::text FROM auth_challenges`)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		for rows.Next() {
			var s string
			if err := rows.Scan(&s); err != nil {
				t.Fatal(err)
			}
			out[s] = true
		}
		return out
	}
	if l := left(); len(l) != 2 || !l[failed] || !l[fresh] || l[open] || l[used] {
		t.Fatalf("left %v", l)
	}
	w.clk.Advance(sessCfg.LoginFailWindow)
	if _, err := w.id.AuthCleanup.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if l := left(); len(l) != 0 {
		t.Fatalf("left after the window %v", l)
	}
}
