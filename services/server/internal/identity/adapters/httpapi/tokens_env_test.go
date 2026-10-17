// SPDX-License-Identifier: Apache-2.0

package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/identity/adapters/identitytest"
	"github.com/OWNER/bothy/services/server/internal/identity/domain/apitoken"
	"github.com/OWNER/bothy/services/server/internal/identity/domain/dpop"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/jose"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
)

// tokensPath is the path of listTokens and createToken for the principal id.
func tokensPath(id ids.ID) string { return "/api/v1/principals/" + id.String() + "/tokens" }

// tokenPath is the path of revokeToken.
func tokenPath(id string) string { return "/api/v1/tokens/" + id }

// agentPrincipal inserts an active agent principal.
func (w *sworld) agentPrincipal(handle string) ids.ID {
	w.t.Helper()
	id := ids.New()
	w.sql(`INSERT INTO principals (id, kind, status, handle, display_name) VALUES ($1, 'agent', 'active', $2, $2)`, id.String(), w.handle(handle))
	return id
}

// appPrincipal inserts an active app principal.
func (w *sworld) appPrincipal(handle string) ids.ID {
	w.t.Helper()
	id := ids.New()
	w.sql(`INSERT INTO principals (id, kind, status, handle, display_name) VALUES ($1, 'app', 'active', $2, $2)`, id.String(), w.handle(handle))
	return id
}

// callH is call with further headers (name, value pairs).
func (c *client) callH(method, path, body string, headers ...string) *httpservertest.Response {
	c.w.t.Helper()
	h := append([]string{"Authorization", "DPoP " + c.access, "DPoP", c.proof(c.key, method, path, c.access)}, headers...)
	return c.w.h.Do(c.w.h.NewRequest(method, path, body, h...))
}

// tokenBody is the JSON of ApiTokenCreate; zero values are left out.
func tokenBody(name string, expires time.Time, jkt string, scopes ...string) string {
	m := map[string]any{"name": name}
	if !expires.IsZero() {
		m["expires_at"] = expires.UTC().Format(time.RFC3339Nano)
	}
	if jkt != "" {
		m["dpop_jkt"] = jkt
	}
	if scopes != nil {
		m["scopes"] = scopes
	}
	b, _ := json.Marshal(m)
	return string(b)
}

// create sends createToken for principal with a fresh Idempotency-Key.
func (c *client) create(principal ids.ID, body string) *httpservertest.Response {
	c.w.t.Helper()
	return c.callH(http.MethodPost, tokensPath(principal), body, "Idempotency-Key", ids.New().String())
}

// issued is a created token: its id, its secret and the key it is bound to (nil for none).
type issued struct {
	id, secret string
	key        *identitytest.Key
}

// agentToken creates a DPoP-bound agent token for agent through the API (owner c with a fresh
// step-up) and returns it.
func (c *stepClient) agentToken(agent ids.ID) issued {
	c.w.t.Helper()
	key, _ := identitytest.NewDeviceKey(jose.AlgES256)
	m := c.w.want(c.create(agent, tokenBody("Adapter", c.w.clk.Now().Add(24*time.Hour), key.Thumbprint)), http.StatusCreated, "")
	return issued{id: m["token"].(map[string]any)["id"].(string), secret: m["secret"].(string), key: key}
}

// personalToken creates a personal token of c's person (switch on) and returns it.
func (c *stepClient) personalToken() issued {
	c.w.t.Helper()
	c.w.setPersonal(true)
	m := c.w.want(c.create(c.user, tokenBody("Laptop", c.w.clk.Now().Add(24*time.Hour), "")), http.StatusCreated, "")
	return issued{id: m["token"].(map[string]any)["id"].(string), secret: m["secret"].(string)}
}

// withToken sends a request with the API token: DPoP with a proof of tk.key when it is bound,
// else Bearer.
func (w *sworld) withToken(tk issued, method, path, body string, headers ...string) *httpservertest.Response {
	w.t.Helper()
	if tk.key == nil {
		return w.h.Do(w.h.NewRequest(method, path, body, append([]string{"Authorization", "Bearer " + tk.secret}, headers...)...))
	}
	return w.withProof(tk, tk.key, method, path, body, headers...)
}

// withProof sends the token under the scheme DPoP with a proof of key.
func (w *sworld) withProof(tk issued, key *identitytest.Key, method, path, body string, headers ...string) *httpservertest.Response {
	w.t.Helper()
	proof := identitytest.SignDPoPProof(key, identitytest.ProofClaims{
		HTM: method, HTU: httpservertest.PublicURL + path, IAT: w.clk.Now(), Nonce: w.id.DPoP.CurrentNonce(), ATH: dpop.ATH(tk.secret),
	})
	return w.h.Do(w.h.NewRequest(method, path, body, append([]string{"Authorization", "DPoP " + tk.secret, "DPoP", proof}, headers...)...))
}

// wantReason checks status, code and reason of a problem.
func (w *sworld) wantReason(r *httpservertest.Response, status int, code, reason string) {
	w.t.Helper()
	m := w.want(r, status, code)
	if m["reason"] != reason {
		w.t.Fatalf("reason %v, want %s: %s", m["reason"], reason, r.Body)
	}
}

// ownerWithStepUp is an owner on an active phone after a fresh step-up through the M1-35 flow.
func (w *sworld) ownerWithStepUp() *stepClient {
	w.t.Helper()
	c := w.phone(w.person(w.handle("owner"), true))
	c.stepUp()
	return c
}

// caseIDT23OnlyHashStored (ID-T23): after createToken the full secret appears in no row of
// api_tokens, idempotency_keys and audit_events; token_hash is the HMAC of the secret with the
// pepper, token_prefix bothy_<kind>_ plus 6 random characters; the answer is no-store.
func caseIDT23OnlyHashStored(t *testing.T, w *sworld) {
	o := w.ownerWithStepUp()
	agent := w.agentPrincipal("lotse")
	key, _ := identitytest.NewDeviceKey(jose.AlgES256)
	r := o.create(agent, tokenBody("Adapter production", w.clk.Now().Add(time.Hour), key.Thumbprint))
	m := w.want(r, http.StatusCreated, "")
	if cc := r.Header.Get("Cache-Control"); cc != "no-store" {
		t.Errorf("Cache-Control %q, want no-store", cc)
	}
	secret := m["secret"].(string)
	p, err := apitoken.Parse(secret)
	if err != nil || p.Kind != apitoken.KindAgent {
		t.Fatalf("secret is no agent token: %v", err)
	}
	tok := m["token"].(map[string]any)
	if tok["token_prefix"] != p.Prefix() || tok["dpop_jkt"] != key.Thumbprint || tok["principal_id"] != agent.String() {
		t.Errorf("token %v", tok)
	}
	var hash []byte
	var prefix string
	w.scalar(&hash, `SELECT token_hash FROM api_tokens WHERE id = $1`, tok["id"])
	w.scalar(&prefix, `SELECT token_prefix FROM api_tokens WHERE id = $1`, tok["id"])
	if !bytes.Equal(hash, apitoken.Hash(testPepper, secret)) || prefix != p.Prefix() {
		t.Error("token_hash is not the HMAC of the secret with the pepper, or token_prefix differs")
	}
	for _, q := range []string{`SELECT coalesce(string_agg(t::text, ' '), '') FROM api_tokens t`,
		`SELECT coalesce(string_agg(k::text, ' '), '') FROM idempotency_keys k`,
		`SELECT coalesce(string_agg(a::text, ' '), '') FROM audit_events a`} {
		var dump string
		w.scalar(&dump, q)
		if strings.Contains(dump, secret) || strings.Contains(dump, p.Random) {
			t.Fatalf("the secret or its random part is stored: %s", q)
		}
	}
	if w.count(`SELECT count(*) FROM idempotency_keys`) == 0 {
		t.Error("no idempotency row: the one-time secret path is untested")
	}
	var got []sauditRow
	for _, r := range w.audits("token.create") {
		if r.Details["token_id"] == tok["id"] {
			got = append(got, r)
		}
	}
	if len(got) != 1 || got[0].Decision != "allow" || got[0].Details["token_id"] != tok["id"] || got[0].Details["kind"] != "agt" ||
		got[0].Details["dpop_bound"] != true || got[0].Details["principal_id"] != agent.String() || got[0].Details["expires_at"] == nil {
		t.Errorf("audit %+v", got)
	}
}

// caseIDT23ShownOnce (ID-T23, R-API-26): listTokens never returns the secret; a replay with the
// same Idempotency-Key and body answers 409 conflict one_time_secret with Idempotent-Replay.
func caseIDT23ShownOnce(t *testing.T, w *sworld) {
	o := w.ownerWithStepUp()
	agent := w.agentPrincipal("lotse")
	key, _ := identitytest.NewDeviceKey(jose.AlgES256)
	body, idem := tokenBody("Adapter", w.clk.Now().Add(time.Hour), key.Thumbprint), ids.New().String()
	m := w.want(o.callH(http.MethodPost, tokensPath(agent), body, "Idempotency-Key", idem), http.StatusCreated, "")
	secret := m["secret"].(string)
	replay := o.callH(http.MethodPost, tokensPath(agent), body, "Idempotency-Key", idem)
	w.wantReason(replay, http.StatusConflict, "conflict", "one_time_secret")
	if replay.Header.Get("Idempotent-Replay") != "true" || bytes.Contains(replay.Body, []byte(secret)) {
		t.Errorf("replay %v %s", replay.Header, replay.Body)
	}
	if n := w.count(`SELECT count(*) FROM api_tokens WHERE principal_id = $1`, agent.String()); n != 1 {
		t.Errorf("%d tokens after the replay, want 1", n)
	}
	list := o.call(http.MethodGet, tokensPath(agent), "")
	lm := w.want(list, http.StatusOK, "")
	if bytes.Contains(list.Body, []byte(secret)) || bytes.Contains(list.Body, []byte("token_hash")) || bytes.Contains(list.Body, []byte("secret")) {
		t.Fatalf("listTokens shows the secret or the hash: %s", list.Body)
	}
	if items := lm["items"].([]any); len(items) != 1 {
		t.Fatalf("items %v", items)
	}
}

// caseListTokensPagesNewestFirstAndKeepsExpired: listTokens lists the unrevoked tokens of a
// principal, expired ones included, newest first with cursor pages bound to the principal; a
// person sees its own tokens, not those of another principal (404).
func caseListTokensPagesNewestFirstAndKeepsExpired(t *testing.T, w *sworld) {
	o := w.ownerWithStepUp()
	agent := w.agentPrincipal("lotse")
	var made []string
	for range 4 {
		made = append(made, o.agentToken(agent).id)
		w.clk.Advance(time.Second)
	}
	w.want(o.call(http.MethodDelete, tokenPath(made[1]), ""), http.StatusNoContent, "")
	w.sql(`UPDATE api_tokens SET expires_at = now() - interval '1 hour' WHERE id = $1`, made[0])
	var got []string
	cursor := ""
	for page := 0; page < 3; page++ {
		path := tokensPath(agent)
		if cursor != "" {
			path += "?cursor=" + cursor
		}
		m := w.want(o.call(http.MethodGet, path, ""), http.StatusOK, "")
		for _, it := range m["items"].([]any) {
			got = append(got, it.(map[string]any)["id"].(string))
		}
		next, _ := m["next_cursor"].(string)
		if next == "" {
			break
		}
		cursor = next
	}
	if want := []string{made[3], made[2], made[0]}; fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("listed %v, want %v", got, want)
	}
	if cursor == "" {
		t.Fatal("no second page with page size 2")
	}
	// The cursor of the agent's list does not work for another principal.
	other := w.agentPrincipal("texter")
	w.want(o.call(http.MethodGet, tokensPath(other)+"?cursor="+cursor, ""), http.StatusBadRequest, "cursor_invalid")
	// A person lists its own tokens and gets 404 for another principal and an unknown one.
	sam := w.phone(w.person(w.handle("sam"), false))
	w.want(sam.call(http.MethodGet, tokensPath(sam.user), ""), http.StatusOK, "")
	w.want(sam.call(http.MethodGet, tokensPath(agent), ""), http.StatusNotFound, "not_found")
	w.want(o.call(http.MethodGet, tokensPath(ids.New()), ""), http.StatusNotFound, "not_found")
}

// caseIDT23ExpiredRejected (ID-T23): a token past expires_at is 401 unauthenticated, audited
// with token_revoked.
func caseIDT23ExpiredRejected(t *testing.T, w *sworld) {
	o := w.ownerWithStepUp()
	agent := w.agentPrincipal("lotse")
	tk := o.agentToken(agent)
	w.want(w.withToken(tk, http.MethodGet, tokensPath(agent), ""), http.StatusOK, "")
	before := w.revokedListDenials()
	w.clk.Advance(24 * time.Hour)
	w.want(w.withToken(tk, http.MethodGet, tokensPath(agent), ""), http.StatusUnauthorized, "unauthenticated")
	if w.revokedListDenials() <= before {
		t.Error("no audit deny with token_revoked")
	}
}

// revokedListDenials counts the deny entries of token.list with the reason token_revoked.
func (w *sworld) revokedListDenials() int {
	w.t.Helper()
	n := 0
	for _, r := range w.audits("token.list") {
		if r.Decision == "deny" && r.Reason == "token_revoked" {
			n++
		}
	}
	return n
}

// caseIDT23RevokedRejected (ID-T23): a revoked token is 401 unauthenticated at the next request,
// audited with token_revoked.
func caseIDT23RevokedRejected(t *testing.T, w *sworld) {
	o := w.ownerWithStepUp()
	agent := w.agentPrincipal("lotse")
	tk := o.agentToken(agent)
	w.want(w.withToken(tk, http.MethodGet, tokensPath(agent), ""), http.StatusOK, "")
	before := w.revokedListDenials()
	w.want(o.call(http.MethodDelete, tokenPath(tk.id), ""), http.StatusNoContent, "")
	w.want(w.withToken(tk, http.MethodGet, tokensPath(agent), ""), http.StatusUnauthorized, "unauthenticated")
	if w.revokedListDenials() <= before {
		t.Error("no audit deny with token_revoked")
	}
}

// caseIDT23DPoPBoundAgentTokenWithoutProof (ID-T23, RFC 9449 section 7.2): a bound agent token
// as Bearer is 401 invalid_dpop_proof with the DPoP challenge; a proof of another key is 401; a
// proof without ath of the token is 401; a valid proof succeeds on listTokens for the agent itself.
func caseIDT23DPoPBoundAgentTokenWithoutProof(t *testing.T, w *sworld) {
	o := w.ownerWithStepUp()
	agent := w.agentPrincipal("lotse")
	tk := o.agentToken(agent)
	path := tokensPath(agent)
	r := w.h.Do(w.h.NewRequest(http.MethodGet, path, "", "Authorization", "Bearer "+tk.secret))
	w.want(r, http.StatusUnauthorized, "invalid_dpop_proof")
	if h := r.Header.Get("WWW-Authenticate"); !strings.Contains(h, `error="invalid_dpop_proof"`) {
		t.Errorf("WWW-Authenticate %q", h)
	}
	other, _ := identitytest.NewDeviceKey(jose.AlgES256)
	w.want(w.withProof(tk, other, http.MethodGet, path, ""), http.StatusUnauthorized, "invalid_dpop_proof")
	noATH := identitytest.SignDPoPProof(tk.key, identitytest.ProofClaims{HTM: http.MethodGet, HTU: httpservertest.PublicURL + path,
		IAT: w.clk.Now(), Nonce: w.id.DPoP.CurrentNonce()})
	w.want(w.h.Do(w.h.NewRequest(http.MethodGet, path, "", "Authorization", "DPoP "+tk.secret, "DPoP", noATH)), http.StatusUnauthorized, "invalid_dpop_proof")
	m := w.want(w.withToken(tk, http.MethodGet, path, ""), http.StatusOK, "")
	if items := m["items"].([]any); len(items) != 1 {
		t.Fatalf("items %v", items)
	}
	// An unbound personal token must not come under the scheme DPoP.
	sam := w.phone(w.person(w.handle("sam"), false))
	sam.stepUp()
	pat := sam.personalToken()
	w.want(w.withProof(pat, other, http.MethodGet, tokensPath(sam.user), ""), http.StatusUnauthorized, "unauthenticated")
	w.want(w.withToken(pat, http.MethodGet, tokensPath(sam.user), ""), http.StatusOK, "")
}

// caseAPITokenMalformedAndForeignKinds: a value with the prefix that fails the format or the
// checksum, an incoming webhook token and a token whose kind does not fit its principal are 401
// unauthenticated.
func caseAPITokenMalformedAndForeignKinds(t *testing.T, w *sworld) {
	o := w.ownerWithStepUp()
	agent := w.agentPrincipal("lotse")
	tk := o.agentToken(agent)
	path := tokensPath(agent)
	bad := tk.secret[:len(tk.secret)-1] + flip(tk.secret[len(tk.secret)-1])
	whi, err := apitoken.Generate(nil, apitoken.KindIncomingWebhook)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{bad, "bothy_agt_short", whi} {
		w.want(w.h.Do(w.h.NewRequest(http.MethodGet, path, "", "Authorization", "Bearer "+s)), http.StatusUnauthorized, "unauthenticated")
	}
	// A pat-shaped token stored for an agent (kind mismatch) never authenticates.
	pat, err := apitoken.Generate(nil, apitoken.KindPersonal)
	if err != nil {
		t.Fatal(err)
	}
	pp, _ := apitoken.Parse(pat)
	w.sql(`INSERT INTO api_tokens (id, principal_id, name, token_prefix, token_hash, expires_at, created_by) VALUES ($1, $2, 'x', $3, $4, now() + interval '1 day', $2)`,
		ids.New().String(), agent.String(), pp.Prefix(), apitoken.Hash(testPepper, pat))
	w.want(w.h.Do(w.h.NewRequest(http.MethodGet, path, "", "Authorization", "Bearer "+pat)), http.StatusUnauthorized, "unauthenticated")
}

// flip returns another Base62 character than c.
func flip(c byte) string {
	if c == 'A' {
		return "B"
	}
	return "A"
}

// caseCreateTokenAgentWithoutJKT400: an agent token without dpop_jkt is 400 on /dpop_jkt while
// agent is in auth.dpop_required_for, and a malformed thumbprint too.
func caseCreateTokenAgentWithoutJKT400(t *testing.T, w *sworld) {
	o := w.ownerWithStepUp()
	agent := w.agentPrincipal("lotse")
	for _, jkt := range []string{"", "short", strings.Repeat("A", 42) + "="} {
		wantPointer(t, w.want(o.create(agent, tokenBody("Adapter", w.clk.Now().Add(time.Hour), jkt)), http.StatusBadRequest, "validation_failed"), "/dpop_jkt")
	}
	if n := w.count(`SELECT count(*) FROM api_tokens WHERE principal_id = $1`, agent.String()); n != 0 {
		t.Errorf("%d tokens stored", n)
	}
}

// wantPointer checks that the problem m names pointer in its errors.
func wantPointer(t *testing.T, m map[string]any, pointer string) {
	t.Helper()
	errs, _ := m["errors"].([]any)
	for _, e := range errs {
		if e.(map[string]any)["pointer"] == pointer {
			return
		}
	}
	t.Fatalf("no error on %s: %v", pointer, m)
}

// caseCreateTokenExpiryRequiredAndBounded: expires_at is required, later than now and at most
// auth.api_token_max_ttl (agent) or auth.personal_token_max_ttl (personal) ahead.
func caseCreateTokenExpiryRequiredAndBounded(t *testing.T, w *sworld) {
	o := w.ownerWithStepUp()
	w.setPersonal(true)
	agent := w.agentPrincipal("lotse")
	key, _ := identitytest.NewDeviceKey(jose.AlgES256)
	now := w.clk.Now()
	// Required: the contract refuses a body without expires_at before the handler.
	w.want(o.create(agent, tokenBody("Adapter", time.Time{}, key.Thumbprint)), http.StatusBadRequest, "validation_failed")
	// now plus 500ns is stored as now (microseconds), so it is refused as not after now.
	for _, at := range []time.Time{now, now.Add(500 * time.Nanosecond), now.Add(-time.Hour), now.Add(sessCfg.APITokenMaxTTL + time.Second)} {
		wantPointer(t, w.want(o.create(agent, tokenBody("Adapter", at, key.Thumbprint)), http.StatusBadRequest, "validation_failed"), "/expires_at")
	}
	w.want(o.create(agent, tokenBody("Adapter", now.Add(sessCfg.APITokenMaxTTL), key.Thumbprint)), http.StatusCreated, "")
	wantPointer(t, w.want(o.create(o.user, tokenBody("Laptop", now.Add(sessCfg.PersonalTokenMaxTTL+time.Second), "")), http.StatusBadRequest,
		"validation_failed"), "/expires_at")
	w.want(o.create(o.user, tokenBody("Laptop", now.Add(sessCfg.PersonalTokenMaxTTL), "")), http.StatusCreated, "")
}

// caseCreateTokenScopesRejectedForAgentAndPersonal: scopes must be empty for agent and personal
// tokens (400 on /scopes).
func caseCreateTokenScopesRejectedForAgentAndPersonal(t *testing.T, w *sworld) {
	o := w.ownerWithStepUp()
	w.setPersonal(true)
	agent := w.agentPrincipal("lotse")
	key, _ := identitytest.NewDeviceKey(jose.AlgES256)
	at := w.clk.Now().Add(time.Hour)
	wantPointer(t, w.want(o.create(agent, tokenBody("Adapter", at, key.Thumbprint, "messages:read")), http.StatusBadRequest, "validation_failed"), "/scopes")
	wantPointer(t, w.want(o.create(o.user, tokenBody("Laptop", at, "", "messages:read")), http.StatusBadRequest, "validation_failed"), "/scopes")
	w.want(o.create(agent, tokenBody("Adapter", at, key.Thumbprint, []string{}...)), http.StatusCreated, "")
}

// caseCreateTokenAppPrincipal409: a token for an app is 409 conflict app_not_installed until
// M2-24; an unknown principal is 404.
func caseCreateTokenAppPrincipal409(t *testing.T, w *sworld) {
	o := w.ownerWithStepUp()
	at := w.clk.Now().Add(time.Hour)
	w.wantReason(o.create(w.appPrincipal("kalender"), tokenBody("Sync", at, "")), http.StatusConflict, "conflict", "app_not_installed")
	w.want(o.create(ids.New(), tokenBody("Sync", at, "")), http.StatusNotFound, "not_found")
	if n := w.count(`SELECT count(*) FROM api_tokens WHERE created_by = $1`, o.user.String()); n != 0 {
		t.Errorf("%d tokens stored", n)
	}
}

// caseCreateTokenStepUpRequired: an owner session without a fresh step-up gets 401
// step_up_required.
func caseCreateTokenStepUpRequired(t *testing.T, w *sworld) {
	o := w.phone(w.person(w.handle("owner"), true))
	agent := w.agentPrincipal("lotse")
	key, _ := identitytest.NewDeviceKey(jose.AlgES256)
	body := tokenBody("Adapter", w.clk.Now().Add(time.Hour), key.Thumbprint)
	w.want(o.create(agent, body), http.StatusUnauthorized, "step_up_required")
	o.stepUp()
	w.want(o.create(agent, body), http.StatusCreated, "")
	w.clk.Advance(sessCfg.StepUpMaxAge + time.Second)
	w.want(o.create(agent, tokenBody("Adapter", w.clk.Now().Add(time.Hour), key.Thumbprint)), http.StatusUnauthorized, "step_up_required")
}

// caseCreateTokenPersonalDisabled403: a personal token while tokens.personal_enabled is off is
// 403 feature_disabled.
func caseCreateTokenPersonalDisabled403(t *testing.T, w *sworld) {
	w.setPersonal(false)
	sam := w.phone(w.person(w.handle("sam"), false))
	sam.stepUp()
	at := w.clk.Now().Add(time.Hour)
	w.want(sam.create(sam.user, tokenBody("Laptop", at, "")), http.StatusForbidden, "feature_disabled")
	w.setPersonal(true)
	m := w.want(sam.create(sam.user, tokenBody("Laptop", at, "")), http.StatusCreated, "")
	if p, err := apitoken.Parse(m["secret"].(string)); err != nil || p.Kind != apitoken.KindPersonal {
		t.Fatalf("no personal token: %v", err)
	}
}

// caseCreateTokenPersonalForOtherPerson403: nobody creates a personal token for another person
// (403 not_self, owners included); a person creates none for an agent (403 owner_only).
func caseCreateTokenPersonalForOtherPerson403(t *testing.T, w *sworld) {
	w.setPersonal(true)
	o := w.ownerWithStepUp()
	sam := w.phone(w.person(w.handle("sam"), false))
	sam.stepUp()
	at := w.clk.Now().Add(time.Hour)
	w.wantReason(o.create(sam.user, tokenBody("Laptop", at, "")), http.StatusForbidden, "forbidden", "not_self")
	w.wantReason(sam.create(o.user, tokenBody("Laptop", at, "")), http.StatusForbidden, "forbidden", "not_self")
	w.wantReason(sam.create(w.agentPrincipal("lotse"), tokenBody("Adapter", at, "")), http.StatusForbidden, "forbidden", "owner_only")
	if n := w.count(`SELECT count(*) FROM api_tokens WHERE created_by = $1 OR created_by = $2`, o.user.String(), sam.user.String()); n != 0 {
		t.Errorf("%d tokens stored", n)
	}
}

// caseAZM35PersonalTokenCannotStepUp (AZ-M35, SR5): Sam's personal token calling createToken for
// Sam gives 403 step_up_not_possible; it cannot start a step-up either.
func caseAZM35PersonalTokenCannotStepUp(t *testing.T, w *sworld) {
	sam := w.phone(w.person(w.handle("sam"), false))
	sam.stepUp()
	pat := sam.personalToken()
	w.wantReason(w.withToken(pat, http.MethodPost, tokensPath(sam.user), tokenBody("Again", w.clk.Now().Add(time.Hour), ""),
		"Idempotency-Key", ids.New().String()), http.StatusForbidden, "forbidden", "step_up_not_possible")
	w.wantReason(w.withToken(pat, http.MethodPost, challengePath, ""), http.StatusForbidden, "forbidden", "step_up_not_possible")
}

// casePersonalTokenOfOwnerHasNoOwnerRights (SR5, red team M1 item 4): the owner's personal token
// calling createToken for an agent gives 403 owner_only.
func casePersonalTokenOfOwnerHasNoOwnerRights(t *testing.T, w *sworld) {
	o := w.ownerWithStepUp()
	pat := o.personalToken()
	key, _ := identitytest.NewDeviceKey(jose.AlgES256)
	agent := w.agentPrincipal("lotse")
	w.wantReason(w.withToken(pat, http.MethodPost, tokensPath(agent), tokenBody("Adapter", w.clk.Now().Add(time.Hour), key.Thumbprint),
		"Idempotency-Key", ids.New().String()), http.StatusForbidden, "forbidden", "owner_only")
	// Nor may it list or revoke the agent's tokens.
	tk := o.agentToken(agent)
	w.want(w.withToken(pat, http.MethodGet, tokensPath(agent), ""), http.StatusNotFound, "not_found")
	w.want(w.withToken(pat, http.MethodDelete, tokenPath(tk.id), ""), http.StatusNotFound, "not_found")
}

// caseRevokeTokenImmediate: the next request with a revoked token is 401; a second DELETE is 204
// without a new audit entry; a token of another principal without rights is 404;
// permission_version is raised in the rights change and the audit entry written.
func caseRevokeTokenImmediate(t *testing.T, w *sworld) {
	o := w.ownerWithStepUp()
	agent := w.agentPrincipal("lotse")
	tk := o.agentToken(agent)
	w.want(w.withToken(tk, http.MethodGet, tokensPath(agent), ""), http.StatusOK, "")
	sam := w.phone(w.person(w.handle("sam"), false))
	w.want(sam.call(http.MethodDelete, tokenPath(tk.id), ""), http.StatusNotFound, "not_found")
	w.want(sam.call(http.MethodDelete, tokenPath(ids.New().String()), ""), http.StatusNotFound, "not_found")
	before := w.permissionVersion()
	changed := w.count(`SELECT count(*) FROM event_log WHERE type = 'access.changed'`)
	w.want(o.call(http.MethodDelete, tokenPath(tk.id), ""), http.StatusNoContent, "")
	if v := w.permissionVersion(); v <= before {
		t.Errorf("permission_version %d, before %d", v, before)
	}
	// The M1-29 path appends access.changed only for a principal whose visible rights differ; a
	// token changes no grant, so the difference is empty and nothing is appended.
	if n := w.count(`SELECT count(*) FROM event_log WHERE type = 'access.changed'`); n != changed {
		t.Errorf("%d access.changed after the revocation, want %d", n, changed)
	}
	w.want(w.withToken(tk, http.MethodGet, tokensPath(agent), ""), http.StatusUnauthorized, "unauthenticated")
	v := w.permissionVersion()
	w.want(o.call(http.MethodDelete, tokenPath(tk.id), ""), http.StatusNoContent, "")
	if w.permissionVersion() != v {
		t.Error("a second revocation raised permission_version")
	}
	var allows int
	for _, r := range w.audits("token.revoke") {
		if r.Decision == "allow" && r.ResourceID == tk.id {
			allows++
			if r.Details["principal_id"] != agent.String() || r.Details["kind"] != "agt" {
				t.Errorf("audit %+v", r)
			}
		}
	}
	if allows != 1 {
		t.Errorf("%d allow entries of token.revoke, want 1", allows)
	}
	var by string
	w.scalar(&by, `SELECT revoked_by::text FROM api_tokens WHERE id = $1`, tk.id)
	if by != o.user.String() {
		t.Errorf("revoked_by %s", by)
	}
	// A person revokes its own personal token.
	sam.stepUp()
	pat := sam.personalToken()
	w.want(sam.call(http.MethodDelete, tokenPath(pat.id), ""), http.StatusNoContent, "")
	w.want(w.withToken(pat, http.MethodGet, tokensPath(sam.user), ""), http.StatusUnauthorized, "unauthenticated")
}

// permissionVersion reads the rights version of the instance.
func (w *sworld) permissionVersion() int64 {
	w.t.Helper()
	var v int64
	w.scalar(&v, `SELECT value FROM sequence_counters WHERE name = 'permission_version'`)
	return v
}

// caseRevokeAllForPrincipal: RevokeAllForPrincipal revokes every active token of a principal in a
// rights change and returns the count; outside a rights change it fails; tokens of other
// principals stay active.
func caseRevokeAllForPrincipal(t *testing.T, w *sworld) {
	o := w.ownerWithStepUp()
	agent, other := w.agentPrincipal("lotse"), w.agentPrincipal("texter")
	a1, a2, keep := o.agentToken(agent), o.agentToken(agent), o.agentToken(other)
	w.want(o.call(http.MethodDelete, tokenPath(a1.id), ""), http.StatusNoContent, "")
	ctx := context.Background()
	if _, err := w.id.Tokens.RevokeAllForPrincipal(ctx, agent, o.user); err == nil {
		t.Fatal("RevokeAllForPrincipal outside a rights change passed")
	}
	var n int
	err := w.uow.Do(ctx, func(ctx context.Context) error {
		var err error
		n, err = w.id.Tokens.RevokeAllForPrincipal(ctx, agent, o.user)
		return err
	}, txn.DeclareRightsChange(txn.RightsChangeSpec{Action: "agent.disable", Resource: txn.Ref{Kind: "agent", ID: agent}, Principals: []ids.ID{agent}}))
	if err != nil || n != 1 {
		t.Fatalf("RevokeAllForPrincipal = %d, %v; want 1", n, err)
	}
	w.want(w.withToken(a2, http.MethodGet, tokensPath(agent), ""), http.StatusUnauthorized, "unauthenticated")
	w.want(w.withToken(keep, http.MethodGet, tokensPath(other), ""), http.StatusOK, "")
	if c := w.count(`SELECT count(*) FROM api_tokens WHERE principal_id = $1 AND revoked_at IS NULL`, agent.String()); c != 0 {
		t.Errorf("%d active tokens left", c)
	}
}

// caseLastUsedGranularity: last_used_at is written on the first use and then at most once per
// identity.last_active_granularity.
func caseLastUsedGranularity(t *testing.T, w *sworld) {
	o := w.ownerWithStepUp()
	agent := w.agentPrincipal("lotse")
	tk := o.agentToken(agent)
	lastUsed := func() *time.Time {
		var at *time.Time
		w.scalar(&at, `SELECT last_used_at FROM api_tokens WHERE id = $1`, tk.id)
		return at
	}
	if lastUsed() != nil {
		t.Fatal("last_used_at set before any use")
	}
	w.want(w.withToken(tk, http.MethodGet, tokensPath(agent), ""), http.StatusOK, "")
	first := lastUsed()
	if first == nil || !first.Equal(w.clk.Now()) {
		t.Fatalf("last_used_at %v, want %v", first, w.clk.Now())
	}
	w.clk.Advance(sessCfg.LastActiveGranularity - time.Second)
	w.want(w.withToken(tk, http.MethodGet, tokensPath(agent), ""), http.StatusOK, "")
	if got := lastUsed(); !got.Equal(*first) {
		t.Errorf("last_used_at moved inside the granularity: %v", got)
	}
	w.clk.Advance(2 * time.Second)
	w.want(w.withToken(tk, http.MethodGet, tokensPath(agent), ""), http.StatusOK, "")
	if got := lastUsed(); !got.Equal(w.clk.Now()) {
		t.Errorf("last_used_at %v, want %v", got, w.clk.Now())
	}
}

// caseAPITokenNeverLogged (SEC-LOG-1): the logs of the chain and of identity and the audit trail
// contain no secret after creating, using, failing with and revoking tokens.
func caseAPITokenNeverLogged(t *testing.T, w *sworld) {
	o := w.ownerWithStepUp()
	agent := w.agentPrincipal("lotse")
	tk := o.agentToken(agent)
	pat := o.personalToken()
	w.want(w.withToken(tk, http.MethodGet, tokensPath(agent), ""), http.StatusOK, "")
	w.want(w.h.Do(w.h.NewRequest(http.MethodGet, tokensPath(agent), "", "Authorization", "Bearer "+tk.secret)), http.StatusUnauthorized, "invalid_dpop_proof")
	w.want(w.withToken(pat, http.MethodPost, tokensPath(o.user), tokenBody("x", w.clk.Now().Add(time.Hour), ""), "Idempotency-Key", ids.New().String()),
		http.StatusForbidden, "forbidden")
	w.want(o.call(http.MethodDelete, tokenPath(tk.id), ""), http.StatusNoContent, "")
	w.want(w.withToken(tk, http.MethodGet, tokensPath(agent), ""), http.StatusUnauthorized, "unauthenticated")
	var all []string
	for _, r := range append(w.logs.Records(), w.h.Logs.Records()...) {
		all = append(all, fmt.Sprint(r))
	}
	for _, a := range []string{"token.create", "token.revoke", "token.list"} {
		for _, r := range w.audits(a) {
			all = append(all, r.Raw)
		}
	}
	if len(all) == 0 {
		t.Fatal("nothing captured")
	}
	text := strings.Join(all, "\n")
	for _, s := range []string{tk.secret, pat.secret, tk.secret[len(apitoken.Prefix)+4 : len(tk.secret)-7], pat.secret[len(apitoken.Prefix)+4 : len(pat.secret)-7]} {
		if strings.Contains(text, s) {
			t.Fatal("an API token leaked into logs or audit")
		}
	}
}

// handle returns a handle unique in the world: the token cases share one world per group.
func (w *sworld) handle(prefix string) string {
	return fmt.Sprintf("%s%d", prefix, w.seq.Add(1))
}

// tokenCase is one case of a token group; the cases of a group share one world and run in order.
type tokenCase struct {
	name string
	fn   func(t *testing.T, w *sworld)
}

// runTokenCases runs the cases on one new world, each as a subtest named after the test of the
// brief; w.t is the subtest while it runs. One world per group keeps the package inside the
// default test timeout on a loaded machine.
func runTokenCases(t *testing.T, cases []tokenCase) {
	t.Parallel()
	w := newSessionWorld(t)
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			prev := w.t
			w.t = t
			defer func() { w.t = prev }()
			c.fn(t, w)
		})
	}
}

// TestAPITokens_Issue: createToken and what it stores (ID-T23, the body rules, step-up, the
// switch, not_self).
func TestAPITokens_Issue(t *testing.T) {
	runTokenCases(t, []tokenCase{
		{"TestIDT23_OnlyHashStored", caseIDT23OnlyHashStored},
		{"TestIDT23_ShownOnce", caseIDT23ShownOnce},
		{"TestCreateToken_AgentWithoutJKT400", caseCreateTokenAgentWithoutJKT400},
		{"TestCreateToken_ExpiryRequiredAndBounded", caseCreateTokenExpiryRequiredAndBounded},
		{"TestCreateToken_ScopesRejectedForAgentAndPersonal", caseCreateTokenScopesRejectedForAgentAndPersonal},
		{"TestCreateToken_AppPrincipal409", caseCreateTokenAppPrincipal409},
		{"TestCreateToken_PersonalDisabled403", caseCreateTokenPersonalDisabled403},
		{"TestCreateToken_PersonalForOtherPerson403", caseCreateTokenPersonalForOtherPerson403},
		{"TestCreateToken_StepUpRequired", caseCreateTokenStepUpRequired},
	})
}

// TestAPITokens_Authenticate: the API token path of the Authenticator (ID-T23, AZ-M35, SR5,
// SEC-LOG-1).
func TestAPITokens_Authenticate(t *testing.T) {
	runTokenCases(t, []tokenCase{
		{"TestIDT23_DPoPBoundAgentTokenWithoutProof", caseIDT23DPoPBoundAgentTokenWithoutProof},
		{"TestAPIToken_MalformedAndForeignKinds", caseAPITokenMalformedAndForeignKinds},
		{"TestAZM35_PersonalTokenCannotStepUp", caseAZM35PersonalTokenCannotStepUp},
		{"TestPersonalTokenOfOwnerHasNoOwnerRights", casePersonalTokenOfOwnerHasNoOwnerRights},
		{"TestAPITokenNeverLogged", caseAPITokenNeverLogged},
		{"TestIDT23_RevokedRejected", caseIDT23RevokedRejected},
		{"TestIDT23_ExpiredRejected", caseIDT23ExpiredRejected},
	})
}

// TestAPITokens_Lifecycle: listing, revocation and last_used_at.
func TestAPITokens_Lifecycle(t *testing.T) {
	runTokenCases(t, []tokenCase{
		{"TestListTokens_PagesNewestFirstAndKeepsExpired", caseListTokensPagesNewestFirstAndKeepsExpired},
		{"TestRevokeToken_Immediate", caseRevokeTokenImmediate},
		{"TestRevokeAllForPrincipal", caseRevokeAllForPrincipal},
		{"TestLastUsedGranularity", caseLastUsedGranularity},
		{"TestAPIToken_NoReplayAfterRevocation", caseReplayAfterRevocation},
	})
}

// caseReplayAfterRevocation (red team round 1): a stored Idempotency-Key answer is never replayed
// to a revoked or expired API token, and a revoked DPoP-bound token without a proof gets nothing
// either: the request is 401 unauthenticated like any other request of the token.
func caseReplayAfterRevocation(t *testing.T, w *sworld) {
	sam := w.phone(w.person(w.handle("sam"), false))
	sam.stepUp()
	pat := sam.personalToken()
	key := ids.New().String()
	body := `{"display_name":"Sam Secret"}`
	w.want(w.withToken(pat, http.MethodPatch, "/api/v1/me/profile", body, "Idempotency-Key", key), http.StatusOK, "")
	w.want(sam.call(http.MethodDelete, tokenPath(pat.id), ""), http.StatusNoContent, "")
	r := w.withToken(pat, http.MethodPatch, "/api/v1/me/profile", body, "Idempotency-Key", key)
	w.want(r, http.StatusUnauthorized, "unauthenticated")
	if r.Header.Get("Idempotent-Replay") != "" {
		t.Errorf("a revoked token got a replay: %v", r.Header)
	}
	// Expired: the same with a token past expires_at.
	m := w.want(sam.create(sam.user, tokenBody("Short", w.clk.Now().Add(time.Minute), "")), http.StatusCreated, "")
	pat2 := issued{id: m["token"].(map[string]any)["id"].(string), secret: m["secret"].(string)}
	key2 := ids.New().String()
	w.want(w.withToken(pat2, http.MethodPatch, "/api/v1/me/profile", body, "Idempotency-Key", key2), http.StatusOK, "")
	w.clk.Advance(time.Minute)
	w.want(w.withToken(pat2, http.MethodPatch, "/api/v1/me/profile", body, "Idempotency-Key", key2), http.StatusUnauthorized, "unauthenticated")
	// A revoked bound agent token without its key: Bearer without a proof replays nothing.
	o := w.ownerWithStepUp()
	agent := w.agentPrincipal("lotse")
	tk := o.agentToken(agent)
	key3 := ids.New().String()
	create := tokenBody("Again", w.clk.Now().Add(time.Hour), tk.key.Thumbprint)
	w.want(w.withToken(tk, http.MethodPost, tokensPath(agent), create, "Idempotency-Key", key3), http.StatusForbidden, "forbidden")
	w.want(o.call(http.MethodDelete, tokenPath(tk.id), ""), http.StatusNoContent, "")
	r = w.h.Do(w.h.NewRequest(http.MethodPost, tokensPath(agent), create, "Authorization", "Bearer "+tk.secret, "Idempotency-Key", key3))
	w.want(r, http.StatusUnauthorized, "unauthenticated")
	if r.Header.Get("Idempotent-Replay") != "" {
		t.Errorf("a revoked bound token without proof got a replay: %v", r.Header)
	}
}
