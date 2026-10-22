// SPDX-License-Identifier: Apache-2.0

package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
)

// The guided flow resolve_i1 of M2-21 (05 rule 1.6, R-AZ-17, 10 section 11) on the world of
// M1-54 with every real module.

// resolveJSON is the member resolve_i1 with reason and the agents in this order.
func resolveJSON(reason string, agents ...ids.ID) string {
	list := make([]string, 0, len(agents))
	for _, a := range agents {
		list = append(list, `"`+a.String()+`"`)
	}
	return `"resolve_i1":{"switch_to_organization":[` + strings.Join(list, ",") + `],"reason":"` + reason + `"}`
}

// with adds the members extra to the JSON object body.
func with(body, extra string) string {
	return strings.TrimSuffix(body, "}") + "," + extra + "}"
}

// samNotizen is the body of createGrant Sam trigger on Research/Notizen (AZ-M21 shape).
func (w *world) samNotizen() string {
	return `{"subject_kind":"user","subject_id":"` + w.sam.String() + `","target_kind":"channel","target_id":"` + w.notizen.String() +
		`","level":"trigger","reason":"Notes"}`
}

// createGrant sends createGrant as name with body and a fresh Idempotency-Key.
func (w *world) createGrant(name, body string) *httpservertest.Response {
	return w.do(name, http.MethodPost, "/api/v1/grants", body, key()...)
}

// policy returns billing_mode, credential holder ("-" for none) and version of the agent id.
func (w *world) policy(id ids.ID) string {
	return w.text(`SELECT billing_mode || ':' || coalesce(credential_holder_id::text, '-') || ':' || version FROM agents WHERE principal_id = $1`,
		id.String())
}

// corrEvent is one event of a request.
type corrEvent struct {
	Type, ResourceID, Audience string
	PermissionVersion          int64
}

// eventsOf returns the events with correlation_id corr in seq order.
func (w *world) eventsOf(corr string) []corrEvent {
	w.t.Helper()
	rs, err := w.db.Query(context.Background(), `SELECT type, coalesce(resource_id::text, ''), coalesce(audience_principal_id::text, ''),
		permission_version FROM event_log WHERE correlation_id = $1::uuid ORDER BY seq`, corr)
	must(w.t, err)
	defer rs.Close()
	var out []corrEvent
	for rs.Next() {
		var e corrEvent
		must(w.t, rs.Scan(&e.Type, &e.ResourceID, &e.Audience, &e.PermissionVersion))
		out = append(out, e)
	}
	return out
}

// corrAudit is one audit row of a request.
type corrAudit struct {
	Action, Decision, ResourceID, Details string
}

// auditsOf returns the audit rows with correlation_id corr in seq order.
func (w *world) auditsOf(corr string) []corrAudit {
	w.t.Helper()
	must(w.t, w.rec.Aggregator().Flush(context.Background()))
	rs, err := w.db.Query(context.Background(), `SELECT action, decision, coalesce(resource_id::text, ''), details::text
		FROM audit_events WHERE correlation_id = $1::uuid ORDER BY seq`, corr)
	must(w.t, err)
	defer rs.Close()
	var out []corrAudit
	for rs.Next() {
		var a corrAudit
		must(w.t, rs.Scan(&a.Action, &a.Decision, &a.ResourceID, &a.Details))
		out = append(out, a)
	}
	return out
}

// corr returns the Correlation-Id of r; it fails the test without one.
func corr(t *testing.T, r *httpservertest.Response) string {
	t.Helper()
	c := r.Header.Get("Correlation-Id")
	if c == "" {
		t.Fatal("no Correlation-Id")
	}
	return c
}

// wantSwitchedFirst checks a successful guided flow of action whose own event is ownEvent: the
// Assistent is organization without a holder, agent.updated for it precedes the own event, every
// event of the request has one permission_version, the allow entries of agent.billing_mode.set and
// of action share the correlation of the response, and I1 holds.
func (w *world) wantSwitchedFirst(r *httpservertest.Response, action, ownEvent string) {
	w.t.Helper()
	if p := w.policy(w.assistent); !strings.HasPrefix(p, "organization:-:") {
		w.t.Fatalf("Assistent after the guided flow: %s", p)
	}
	c := corr(w.t, r)
	evs := w.eventsOf(c)
	switched, own := -1, -1
	for i, e := range evs {
		if e.Type == "agent.updated" && e.ResourceID == w.assistent.String() && switched < 0 {
			switched = i
		}
		if e.Type == ownEvent && own < 0 {
			own = i
		}
		if e.PermissionVersion != evs[0].PermissionVersion {
			w.t.Fatalf("events with two rights versions: %+v", evs)
		}
	}
	if switched < 0 || own < 0 || switched > own {
		w.t.Fatalf("events %+v: want agent.updated of Assistent before %s", evs, ownEvent)
	}
	var actions []string
	for _, a := range w.auditsOf(c) {
		if a.Decision == "allow" {
			actions = append(actions, a.Action)
		}
	}
	if !slices.Equal(actions, []string{"agent.billing_mode.set", action}) {
		w.t.Fatalf("allow entries of the request %v", actions)
	}
	v, err := w.hook.VerifyInvariants(context.Background(), w.clk.Now())
	if err != nil || !v.Empty() {
		w.t.Fatalf("I1 after the guided flow: %+v %v", v, err)
	}
}

// guidedActions reads the actions the bullet "Guided flow" of 05 rule 1.6 names (the German
// specification of this branch or the English one of main).
func guidedActions(t *testing.T) []string {
	t.Helper()
	text, err := os.ReadFile(filepath.Clean(filepath.Join("..", "..", "..", "..", "docs", "spec", "05-actions.md")))
	if err != nil {
		t.Fatal(err)
	}
	start := bytes.Index(text, []byte("Guided flow:"))
	end := bytes.Index(text[max(start, 0):], []byte("resolve_i1"))
	if start < 0 || end < 0 {
		t.Fatal("05 rule 1.6 has no bullet on the guided flow")
	}
	var out []string
	for _, m := range regexp.MustCompile("`([a-z_.]+)`").FindAllSubmatch(text[start:start+end], -1) {
		out = append(out, string(m[1]))
	}
	return out
}

// TestAZ_P7_ResolveI1MakesChangeAdmissible (05 section 9 test 5, AZ-P7, M2-21): every action of
// the guided flow is rejected with 409 i1_violation without resolve_i1 and admitted with
// resolve_i1 [Assistent], which switches Assistent first in the same transaction.
func TestAZ_P7_ResolveI1MakesChangeAdmissible(t *testing.T) {
	t.Parallel()
	type tcase struct {
		ownEvent string
		// send sends the change with the extra members (empty for none); extra names the principal
		// of the conflict without resolve_i1; check checks the change after the success.
		send  func(w *world, extra string) *httpservertest.Response
		extra func(w *world) ids.ID
		check func(w *world, r *httpservertest.Response)
	}
	cases := map[string]tcase{
		"grant.create": {ownEvent: "grant.changed",
			send: func(w *world, extra string) *httpservertest.Response {
				body := w.samNotizen()
				if extra != "" {
					body = with(body, extra)
				}
				return w.createGrant("owner", body)
			},
			extra: func(w *world) ids.ID { return w.sam },
			check: func(w *world, r *httpservertest.Response) { w.want(r, http.StatusCreated, "") }},
		"group.member.add": {ownEvent: "group.changed",
			send: func(w *world, extra string) *httpservertest.Response {
				g := w.text(`SELECT coalesce((SELECT id::text FROM groups WHERE name = 'Forschung'), '')`)
				if g == "" {
					m := w.want(w.do("owner", http.MethodPost, "/api/v1/groups", `{"name":"Forschung"}`, key()...), http.StatusCreated, "")
					g = m["id"].(string)
					w.grant(`"subject_kind":"group","subject_id":"` + g + `","target_kind":"space","target_id":"` + w.research.String() +
						`","level":"trigger","reason":"Research team"`)
				}
				body := `{"user_id":"` + w.sam.String() + `"}`
				if extra != "" {
					body = with(body, extra)
				}
				return w.do("owner", http.MethodPost, "/api/v1/groups/"+g+"/members", body, key()...)
			},
			extra: func(w *world) ids.ID { return w.sam },
			check: func(w *world, r *httpservertest.Response) { w.want(r, http.StatusCreated, "") }},
		"owner.add": {ownEvent: "user.updated",
			send: func(w *world, extra string) *httpservertest.Response {
				body := `{"user_id":"` + w.alex.String() + `","reason":"Deputy"}`
				if extra != "" {
					body = with(body, extra)
				}
				return w.do("owner", http.MethodPost, "/api/v1/owners", body, key()...)
			},
			extra: func(w *world) ids.ID { return w.alex },
			check: func(w *world, r *httpservertest.Response) {
				w.want(r, http.StatusCreated, "")
				if n := w.count(`SELECT count(*) FROM owners WHERE user_id = $1 AND revoked_at IS NULL`, w.alex.String()); n != 1 {
					w.t.Fatalf("Alex owners rows %d", n)
				}
			}},
		"channel.operator.set": {ownEvent: "channel.updated",
			send: func(w *world, extra string) *httpservertest.Response {
				body := agentBody(w.assistent)
				if extra != "" {
					body = with(body, extra)
				}
				return w.setChannelOperator(w.allgemein, body)
			},
			extra: func(w *world) ids.ID { return w.alex },
			check: func(w *world, r *httpservertest.Response) {
				if op := w.wantOK(r)["operator"].(map[string]any); op["effective_agent_id"] != w.assistent.String() {
					w.t.Fatalf("operator %v", op)
				}
			}},
		"space.operator.set": {ownEvent: "space.updated",
			send: func(w *world, extra string) *httpservertest.Response {
				body := agentBody(w.assistent)
				if extra != "" {
					body = with(body, extra)
				}
				return w.setSpaceOperator(w.support, body)
			},
			extra: func(w *world) ids.ID { return w.alex },
			check: func(w *world, r *httpservertest.Response) {
				if m := w.wantOK(r); m["default_operator_agent_id"] != w.assistent.String() {
					w.t.Fatalf("space %v", m)
				}
			}},
	}
	for _, action := range guidedActions(t) {
		if _, ok := cases[action]; !ok {
			t.Errorf("05 rule 1.6 names %s, which has no case", action)
		}
	}
	for action, c := range cases {
		t.Run(action, func(t *testing.T) {
			t.Parallel()
			w := newWorld(t)
			w.wantI1(c.send(w, ""), w.assistent, w.owner, c.extra(w))
			r := c.send(w, resolveJSON("Second person triggers", w.assistent))
			c.check(w, r)
			w.wantSwitchedFirst(r, action, c.ownEvent)
		})
	}
}

// TestBR07_GuidedFlowCommitsBoth (BR-07): createGrant with resolve_i1 [Assistent] writes two allow
// entries in order with the correlation of the response, and agent.updated, grant.changed and
// access.changed for Sam with that correlation and one new permission_version.
func TestBR07_GuidedFlowCommitsBoth(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	pv := w.permissionVersion()
	r := w.createGrant("owner", with(w.samNotizen(), resolveJSON("Sam triggers the assistant", w.assistent)))
	g := w.want(r, http.StatusCreated, "")
	c := corr(t, r)
	audits := w.auditsOf(c)
	if len(audits) != 2 || audits[0].Action != "agent.billing_mode.set" || audits[1].Action != "grant.create" ||
		audits[0].Decision != "allow" || audits[1].Decision != "allow" {
		t.Fatalf("audit rows %+v", audits)
	}
	for _, want := range []string{`"from_billing_mode": "personal"`, `"to_billing_mode": "organization"`,
		`"from_credential_holder_id": "` + w.owner.String() + `"`, `"reason": "Sam triggers the assistant"`} {
		if !strings.Contains(audits[0].Details, want) {
			t.Fatalf("switch details %s lack %s", audits[0].Details, want)
		}
	}
	if audits[0].ResourceID != w.assistent.String() || audits[1].ResourceID != g["id"] || !strings.Contains(audits[1].Details, w.sam.String()) {
		t.Fatalf("audit resources %+v", audits)
	}
	evs := w.eventsOf(c)
	var types []string
	for _, e := range evs {
		types = append(types, e.Type)
		if e.PermissionVersion != pv+1 {
			t.Fatalf("event %+v, want permission_version %d", e, pv+1)
		}
	}
	if len(evs) < 3 || evs[0].Type != "agent.updated" || evs[1].Type != "grant.changed" || !slices.Contains(types[2:], "access.changed") {
		t.Fatalf("events %v", types)
	}
	sam := false
	for _, e := range evs[2:] {
		sam = sam || (e.Type == "access.changed" && e.Audience == w.sam.String())
	}
	if !sam || w.permissionVersion() != pv+1 {
		t.Fatalf("access.changed for Sam %v, permission_version %d", sam, w.permissionVersion())
	}
	updated := w.events("agent.updated")
	a := updated[len(updated)-1].Payload["agent"].(map[string]any)
	if a["billing_mode"] != "organization" || a["credential_holder_id"] != nil {
		t.Fatalf("agent.updated payload %v", a)
	}
}

// TestBR07_AbortInChangePartUndoesSwitch (BR-07): without G-2 and with Texter personal, a trigger
// grant on the instance with resolve_i1 [Assistent] still violates I1 for Texter; the switch of
// Assistent is rolled back with it and only the deny entry of grant.create remains.
func TestBR07_AbortInChangePartUndoesSwitch(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	w.wantOK(w.do("owner", http.MethodPost, "/api/v1/grants/"+w.g2.String()+"/revoke", `{"reason":"Moved"}`, key()...))
	w.wantOK(w.setPersonal(w.texter, w.owner))
	assistent, pv := w.policy(w.assistent), w.permissionVersion()
	updates, grants := len(w.events("agent.updated")), w.count(`SELECT count(*) FROM grants`)
	r := w.createGrant("owner", `{"subject_kind":"user","subject_id":"`+w.sam.String()+`","target_kind":"instance","level":"trigger","reason":"All",`+
		resolveJSON("Instance wide", w.assistent)+`}`)
	w.wantI1(r, w.texter, w.owner, w.sam)
	if w.policy(w.assistent) != assistent || w.permissionVersion() != pv || len(w.events("agent.updated")) != updates ||
		w.count(`SELECT count(*) FROM grants`) != grants {
		t.Fatalf("state changed: Assistent %s, permission_version %d", w.policy(w.assistent), w.permissionVersion())
	}
	for _, a := range w.auditsOf(corr(t, r)) {
		if a.Decision == "allow" {
			t.Fatalf("allow entry after the rollback: %+v", a)
		}
	}
	d := w.denies("grant.create", "i1_violation")
	if len(d) != 1 || !strings.Contains(stringify(d[0].Details["violations"]), w.texter.String()) ||
		strings.Contains(stringify(d[0].Details["violations"]), w.assistent.String()) {
		t.Fatalf("deny rows %+v", d)
	}
}

// stringify renders a decoded JSON value for a containment check.
func stringify(v any) string {
	var b strings.Builder
	var walk func(any)
	walk = func(v any) {
		switch x := v.(type) {
		case map[string]any:
			for k, e := range x {
				b.WriteString(k + "=")
				walk(e)
				b.WriteString(";")
			}
		case []any:
			for _, e := range x {
				walk(e)
				b.WriteString(",")
			}
		case string:
			b.WriteString(x)
		}
	}
	walk(v)
	return b.String()
}

// TestBR07_AbortInSwitchPartLeavesChangeUndone (BR-07): Alex with trigger and manage on Support may
// grant Sam read on Support/Allgemein; with resolve_i1 [Lotse] the switch is refused with 403
// owner_only, with resolve_i1 [Assistent] (invisible for Alex) with the 404 of an unknown agent;
// nothing is written and the denial of agent.billing_mode.set stands in the audit log.
func TestBR07_AbortInSwitchPartLeavesChangeUndone(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	w.grant(`"subject_kind":"user","subject_id":"` + w.alex.String() + `","target_kind":"space","target_id":"` + w.support.String() +
		`","level":"trigger","capabilities":["manage"],"reason":"Support lead manages"`)
	body := `{"subject_kind":"user","subject_id":"` + w.sam.String() + `","target_kind":"channel","target_id":"` + w.allgemein.String() +
		`","level":"read","reason":"Reads support"}`
	grants, pv, updates := w.count(`SELECT count(*) FROM grants`), w.permissionVersion(), len(w.events("agent.updated"))
	p := w.want(w.createGrant("alex", with(body, resolveJSON("x", w.lotse))), http.StatusForbidden, "forbidden")
	if p["reason"] != "owner_only" {
		t.Fatalf("reason %v", p)
	}
	invisible := w.createGrant("alex", with(body, resolveJSON("x", w.assistent)))
	unknown := w.createGrant("alex", with(body, resolveJSON("x", ids.New())))
	w.want(invisible, http.StatusNotFound, "not_found")
	w.want(unknown, http.StatusNotFound, "not_found")
	if withoutCorrelation(t, invisible) != withoutCorrelation(t, unknown) || !sameHeaders(invisible, unknown) {
		t.Fatalf("invisible and unknown agent differ:\n%s %v\n%s %v", invisible.Body, invisible.Header, unknown.Body, unknown.Header)
	}
	if w.count(`SELECT count(*) FROM grants`) != grants || w.permissionVersion() != pv || len(w.events("agent.updated")) != updates ||
		len(w.allows("grant.create")) != 6 {
		t.Fatalf("a refused guided flow wrote something")
	}
	if d := w.audits("agent.billing_mode.set"); len(d) == 0 || d[len(d)-1].Decision != "deny" {
		t.Fatalf("no denial of agent.billing_mode.set: %+v", d)
	}
	w.want(w.createGrant("alex", body), http.StatusCreated, "")
}

// withoutCorrelation returns the problem body of r without its correlation_id, the one member
// that differs per request.
func withoutCorrelation(t *testing.T, r *httpservertest.Response) string {
	t.Helper()
	p := r.Problem(t)
	delete(p, "correlation_id")
	b, err := json.Marshal(p)
	must(t, err)
	return string(b)
}

// sameHeaders compares the headers of two answers apart from Correlation-Id and Date.
func sameHeaders(a, b *httpservertest.Response) bool {
	x, y := a.Header.Clone(), b.Header.Clone()
	for _, h := range []string{"Correlation-Id", "Date"} {
		x.Del(h)
		y.Del(h)
	}
	if len(x) != len(y) {
		return false
	}
	for k, v := range x {
		if !slices.Equal(v, y[k]) {
			return false
		}
	}
	return true
}

// TestResolveI1_OuterChecksComeFirst (M2-21 design note 3 b and c): with a valid resolve_i1 the
// refusals of the outer action come first and switch nothing.
func TestResolveI1_OuterChecksComeFirst(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	before := w.policy(w.assistent)
	res := resolveJSON("Guided", w.assistent)
	// Sam cannot see a group he is not a member of.
	g := w.want(w.do("owner", http.MethodPost, "/api/v1/groups", `{"name":"Intern"}`, key()...), http.StatusCreated, "")
	w.want(w.do("sam", http.MethodPost, "/api/v1/groups/"+g["id"].(string)+"/members", `{"user_id":"`+w.robin.String()+`",`+res+`}`, key()...),
		http.StatusNotFound, "not_found")
	body := with(agentBody(w.assistent), res)
	w.want(w.setChannelOperatorAs("owner", w.allgemein, body, key()...), http.StatusPreconditionRequired, "precondition_required")
	w.want(w.setChannelOperatorAs("owner", w.allgemein, body, ifMatch(w.channelVersion(w.allgemein)+3)...), http.StatusPreconditionFailed, "version_conflict")
	w.want(w.do("owner", http.MethodPost, "/api/v1/groups/"+w.leser.String()+"/members", `{"user_id":"`+w.sam.String()+`",`+res+`}`, key()...),
		http.StatusConflict, "conflict")
	w.session("stale", w.owner, w.clk.Now().Add(-time.Hour))
	owner := `{"user_id":"` + w.alex.String() + `","reason":"Deputy",` + res + `}`
	w.want(w.do("stale", http.MethodPost, "/api/v1/owners", owner, key()...), http.StatusUnauthorized, "step_up_required")
	w.personalToken("pat", w.owner)
	// A personal token acts without the owner role (SR5), so owner.add refuses it with owner_only
	// before the step-up rule is reached (the order of M1-31).
	if p := w.want(w.do("pat", http.MethodPost, "/api/v1/owners", owner, key()...), http.StatusForbidden, "forbidden"); p["reason"] != "owner_only" {
		t.Fatalf("personal token %v", p)
	}
	if w.policy(w.assistent) != before {
		t.Fatalf("Assistent switched: %s", w.policy(w.assistent))
	}
}

// TestResolveI1_OperatorNoOpSwitchesNothing (M2-21 design note 3 d): the current assignment with
// resolve_i1 answers 200 without a switch and with one no-change entry of the operator action.
func TestResolveI1_OperatorNoOpSwitchesNothing(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	before, version := w.policy(w.assistent), w.channelVersion(w.kampagnen)
	r := w.setChannelOperator(w.kampagnen, `{"mode":"inherit","reason":"Same",`+resolveJSON("Guided", w.assistent)+`}`)
	w.wantOK(r)
	if w.policy(w.assistent) != before || w.channelVersion(w.kampagnen) != version {
		t.Fatalf("no-op changed state: %s %d", w.policy(w.assistent), w.channelVersion(w.kampagnen))
	}
	audits := w.auditsOf(corr(t, r))
	if len(audits) != 1 || audits[0].Action != "channel.operator.set" || !strings.Contains(audits[0].Details, `"no_change": true`) {
		t.Fatalf("audit rows %+v", audits)
	}
}

// TestResolveI1_AlreadyOrganizationIsNoChange (M2-21 design note 4): Lotse is on organization
// already; it keeps its version and gets a no-change entry, Assistent is switched, and the three
// allow entries share the correlation.
func TestResolveI1_AlreadyOrganizationIsNoChange(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	lotse := w.policy(w.lotse)
	r := w.createGrant("owner", with(w.samNotizen(), resolveJSON("Guided", w.assistent, w.lotse)))
	w.want(r, http.StatusCreated, "")
	if w.policy(w.lotse) != lotse || !strings.HasPrefix(w.policy(w.assistent), "organization:-:") {
		t.Fatalf("Lotse %s, Assistent %s", w.policy(w.lotse), w.policy(w.assistent))
	}
	c := corr(t, r)
	for _, e := range w.eventsOf(c) {
		if e.Type == "agent.updated" && e.ResourceID == w.lotse.String() {
			t.Fatal("agent.updated for Lotse")
		}
	}
	audits := w.auditsOf(c)
	if len(audits) != 3 {
		t.Fatalf("audit rows %+v", audits)
	}
	for _, a := range audits {
		if a.Decision != "allow" {
			t.Fatalf("audit %+v", a)
		}
		if a.ResourceID == w.lotse.String() && !strings.Contains(a.Details, `"no_change": true`) {
			t.Fatalf("Lotse entry %+v", a)
		}
	}
}

// TestResolveI1_ReasonRules (SEC-T07 for the reason of resolve_i1, M2-21 design note 2): a blank,
// a bidi and a too long reason give 400 at /resolve_i1/reason; the same table on BillingModeSet,
// BillingModeSetAll and CredentialHolderSet at /reason.
func TestResolveI1_ReasonRules(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	reasons := map[string]string{"blank": "   ", "bidi": `bad\u202e`, "too long": strings.Repeat("a", reasonMaxChars+1)}
	wantField := func(r *httpservertest.Response, pointer string) {
		t.Helper()
		p := w.want(r, http.StatusBadRequest, "validation_failed")
		for _, f := range p["errors"].([]any) {
			if f.(map[string]any)["pointer"] == pointer {
				return
			}
		}
		t.Fatalf("no field error at %s: %v", pointer, p)
	}
	for name, reason := range reasons {
		t.Run(name, func(t *testing.T) {
			wantField(w.createGrant("owner", with(w.samNotizen(), resolveJSON(reason, w.assistent))), "/resolve_i1/reason")
			wantField(w.do("owner", http.MethodPut, "/api/v1/agents/"+w.lotse.String()+"/billing-mode",
				`{"billing_mode":"organization","reason":"`+reason+`"}`, ifMatch(w.agentVersion(w.lotse))...), "/reason")
			wantField(w.do("owner", http.MethodPut, "/api/v1/instance/agent-billing-mode",
				`{"billing_mode":"organization","reason":"`+reason+`"}`), "/reason")
			wantField(w.do("owner", http.MethodPut, "/api/v1/agents/"+w.assistent.String()+"/credential-holder",
				`{"credential_holder_id":"`+w.owner.String()+`","reason":"`+reason+`"}`, ifMatch(w.agentVersion(w.assistent))...), "/reason")
		})
	}
	// Field errors of the outer body and of resolve_i1 come together (06 section 5 point 3).
	p := w.want(w.createGrant("owner", `{"subject_kind":"user","subject_id":"`+w.sam.String()+`","target_kind":"channel","target_id":"`+
		w.notizen.String()+`","level":"trigger","reason":"  ",`+resolveJSON("  ", w.assistent)+`}`), http.StatusBadRequest, "validation_failed")
	var pointers []string
	for _, f := range p["errors"].([]any) {
		pointers = append(pointers, f.(map[string]any)["pointer"].(string))
	}
	if !slices.Contains(pointers, "/reason") || !slices.Contains(pointers, "/resolve_i1/reason") {
		t.Fatalf("pointers %v", pointers)
	}
}

// TestResolveI1_KnownOnlyInFourSchemas (SEC-IN-1, SEC-T06 shape): resolve_i1 is unknown in the
// bodies of setAgentBillingMode, revokeGrant and createGroup and known in OwnerAdd,
// GroupMemberAdd, GrantCreate and OperatorSet; exactly these four schemas reference ResolveI1.
func TestResolveI1_KnownOnlyInFourSchemas(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	res := resolveJSON("Guided", w.assistent)
	unknown := func(r *httpservertest.Response) {
		t.Helper()
		p := w.want(r, http.StatusBadRequest, "validation_failed")
		if f := fieldOf(p); f["code"] != "unknown_member" {
			t.Fatalf("field %v", f)
		}
	}
	unknown(w.do("owner", http.MethodPut, "/api/v1/agents/"+w.lotse.String()+"/billing-mode", `{"billing_mode":"organization","reason":"x",`+res+`}`,
		ifMatch(w.agentVersion(w.lotse))...))
	unknown(w.do("owner", http.MethodPost, "/api/v1/grants/"+w.g1.String()+"/revoke", `{"reason":"x",`+res+`}`, key()...))
	unknown(w.do("owner", http.MethodPost, "/api/v1/groups", `{"name":"Neu",`+res+`}`, key()...))
	// Accepted: the four bodies (the cases of TestAZ_P7_ResolveI1MakesChangeAdmissible); here
	// each with a body the server takes.
	w.want(w.do("owner", http.MethodPost, "/api/v1/groups/"+w.leser.String()+"/members", `{"user_id":"`+w.robin.String()+`",`+res+`}`, key()...),
		http.StatusCreated, "")
	b, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "docs", "api", "openapi.yaml"))
	must(t, err)
	var refs []string
	schema := ""
	for _, line := range strings.Split(string(b), "\n") {
		if m := regexp.MustCompile(`^    ([A-Za-z0-9]+):$`).FindStringSubmatch(line); m != nil {
			schema = m[1]
		}
		if strings.Contains(line, "#/components/schemas/ResolveI1'") {
			refs = append(refs, schema)
		}
	}
	slices.Sort(refs)
	if want := []string{"GrantCreate", "GroupMemberAdd", "OperatorSet", "OwnerAdd"}; !slices.Equal(refs, want) {
		t.Fatalf("schemas referencing ResolveI1 %v, want %v", refs, want)
	}
}

// TestResolveI1_IdempotencyReplay (M2-21 design note 10): the replay answers the stored 201 and
// switches nothing again; the same key with another list is 422 idempotency_key_reuse.
func TestResolveI1_IdempotencyReplay(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	k := []string{"Idempotency-Key", ids.New().String()}
	body := with(w.samNotizen(), resolveJSON("Guided", w.assistent))
	first := w.do("owner", http.MethodPost, "/api/v1/grants", body, k...)
	w.want(first, http.StatusCreated, "")
	updates, switches := len(w.events("agent.updated")), len(w.audits("agent.billing_mode.set"))
	replay := w.do("owner", http.MethodPost, "/api/v1/grants", body, k...)
	w.want(replay, http.StatusCreated, "")
	var a, b map[string]any
	must(t, json.Unmarshal(first.Body, &a))
	must(t, json.Unmarshal(replay.Body, &b))
	if replay.Header.Get("Idempotent-Replay") != "true" || a["id"] != b["id"] || len(a) != len(b) {
		t.Fatalf("replay %v %s", replay.Header, replay.Body)
	}
	if len(w.events("agent.updated")) != updates || len(w.audits("agent.billing_mode.set")) != switches {
		t.Fatal("the replay switched again")
	}
	w.want(w.do("owner", http.MethodPost, "/api/v1/grants", with(w.samNotizen(), resolveJSON("Guided", w.assistent, w.lotse)), k...),
		http.StatusUnprocessableEntity, "idempotency_key_reuse")
}

// TestResolveI1_ConcurrentSwitchBackSerializes (BR-03 shape, M2-21 design note 10): Assistent on
// organization; a switch back to personal and a guided createGrant for Sam race in both orders;
// never do both end with Assistent personal and Sam's grant, no client sees a deadlock, and I1
// holds afterwards.
func TestResolveI1_ConcurrentSwitchBackSerializes(t *testing.T) {
	t.Parallel()
	for _, switchFirst := range []bool{true, false} {
		t.Run(map[bool]string{true: "switch first", false: "guided first"}[switchFirst], func(t *testing.T) {
			t.Parallel()
			w := newWorld(t)
			w.wantOK(w.do("owner", http.MethodPut, "/api/v1/agents/"+w.assistent.String()+"/billing-mode",
				`{"billing_mode":"organization","reason":"Start"}`, ifMatch(w.agentVersion(w.assistent))...))
			version := w.agentVersion(w.assistent)
			body := with(w.samNotizen(), resolveJSON("Guided", w.assistent))
			reqs := []func() *httpservertest.Response{
				func() *httpservertest.Response {
					return w.h.Do(w.h.NewRequest(http.MethodPut, "/api/v1/agents/"+w.assistent.String()+"/billing-mode",
						`{"billing_mode":"personal","credential_holder_id":"`+w.owner.String()+`","reason":"Back"}`,
						append(ifMatch(version), "Authorization", "Bearer owner")...))
				},
				func() *httpservertest.Response {
					return w.h.Do(w.h.NewRequest(http.MethodPost, "/api/v1/grants", body, append(key(), "Authorization", "Bearer owner")...))
				},
			}
			if !switchFirst {
				slices.Reverse(reqs)
			}
			out := make([]*httpservertest.Response, len(reqs))
			var barrier, done sync.WaitGroup
			barrier.Add(1)
			for i, req := range reqs {
				done.Add(1)
				go func() {
					defer done.Done()
					barrier.Wait()
					out[i] = req()
				}()
				time.Sleep(5 * time.Millisecond)
			}
			barrier.Done()
			done.Wait()
			for _, r := range out {
				if r.StatusCode >= 500 {
					t.Fatalf("server error: %s", r.Body)
				}
			}
			granted := w.count(`SELECT count(*) FROM grants WHERE subject_principal_id = $1 AND target_channel_id = $2 AND revoked_at IS NULL`,
				w.sam.String(), w.notizen.String()) == 1
			personal := strings.HasPrefix(w.policy(w.assistent), "personal:")
			if !granted || personal {
				t.Fatalf("granted %v, Assistent %s: %s | %s", granted, w.policy(w.assistent), out[0].Body, out[1].Body)
			}
			if v, err := w.hook.VerifyInvariants(context.Background(), w.clk.Now()); err != nil || !v.Empty() {
				t.Fatalf("I1 after the race: %+v %v", v, err)
			}
		})
	}
}

// TestGuidedFlowPreviewOrganizationReady (10 section 11 point 1): the preview without resolve_i1
// names Assistent with organization_ready false before any heartbeat and true after a heartbeat
// that supports organization.
func TestGuidedFlowPreviewOrganizationReady(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	ready := func() any {
		m := w.wantOK(w.do("owner", http.MethodPost, "/api/v1/grants/preview", w.samNotizen()))
		list, _ := m["invariant_violations"].([]any)
		if m["allowed"] != false || len(list) != 1 {
			t.Fatalf("preview %v", m)
		}
		v := list[0].(map[string]any)
		if v["agent_id"] != w.assistent.String() {
			t.Fatalf("violation %v", v)
		}
		return v["organization_ready"]
	}
	if r := ready(); r != false {
		t.Fatalf("organization_ready before a heartbeat %v", r)
	}
	w.beatOK("assistent", `{"state":"online","reported_billing_mode":"personal","billing_modes_supported":["personal","organization"]}`)
	must(t, w.presence.Flush(context.Background()))
	if r := ready(); r != true {
		t.Fatalf("organization_ready after the heartbeat %v", r)
	}
}

// TestPreviewGrantWithResolveI1 (M2-21 design note 9): the owner without fresh step-up previews the
// guided grant: allowed, no violation, an effect for Sam and the switch line, and nothing stays
// behind; Alex previewing with Lotse gets 403 owner_only, with an unknown agent 404.
func TestPreviewGrantWithResolveI1(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	w.session("stale", w.owner, w.clk.Now().Add(-time.Hour))
	events, audits, pv, before := w.eventCount(), w.count(`SELECT count(*) FROM audit_events`), w.permissionVersion(), w.policy(w.assistent)
	m := w.wantOK(w.do("stale", http.MethodPost, "/api/v1/grants/preview", with(w.samNotizen(), resolveJSON("Guided", w.assistent))))
	violations, _ := m["invariant_violations"].([]any)
	effects, _ := m["effects"].([]any)
	if m["allowed"] != true || len(violations) != 0 || len(effects) == 0 {
		t.Fatalf("preview %v", m)
	}
	sam := false
	for _, e := range effects {
		sam = sam || e.(map[string]any)["principal"].(map[string]any)["id"] == w.sam.String()
	}
	text, _ := m["text"].(string)
	if !sam || !strings.Contains(text, "assistent") || !strings.Contains(text, "Organisation") {
		t.Fatalf("effects %v, text %q", effects, text)
	}
	must(t, w.rec.Aggregator().Flush(context.Background()))
	if w.eventCount() != events || w.count(`SELECT count(*) FROM audit_events`) != audits || w.permissionVersion() != pv || w.policy(w.assistent) != before {
		t.Fatal("the preview left something behind")
	}
	w.grant(`"subject_kind":"user","subject_id":"` + w.alex.String() + `","target_kind":"space","target_id":"` + w.support.String() +
		`","level":"trigger","capabilities":["manage"],"reason":"Support lead manages"`)
	body := `{"subject_kind":"user","subject_id":"` + w.sam.String() + `","target_kind":"channel","target_id":"` + w.allgemein.String() +
		`","level":"read","reason":"Reads support"}`
	if p := w.want(w.do("alex", http.MethodPost, "/api/v1/grants/preview", with(body, resolveJSON("x", w.lotse))), http.StatusForbidden, "forbidden"); p["reason"] != "owner_only" {
		t.Fatalf("Alex with Lotse %v", p)
	}
	w.want(w.do("alex", http.MethodPost, "/api/v1/grants/preview", with(body, resolveJSON("x", ids.New()))), http.StatusNotFound, "not_found")
}

// TestReverseSwitchNamesBlockingSources (10 section 11 last paragraph): after the guided flow the
// switch back to personal names Sam, explainChannelAccess names the grant as Sam's source, and
// after revokeGrant the switch back succeeds.
func TestReverseSwitchNamesBlockingSources(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	g := w.want(w.createGrant("owner", with(w.samNotizen(), resolveJSON("Guided", w.assistent))), http.StatusCreated, "")
	w.wantI1(w.setPersonal(w.assistent, w.owner), w.assistent, w.owner, w.sam)
	m := w.wantOK(w.do("owner", http.MethodGet, "/api/v1/channels/"+w.notizen.String()+"/access", ""))
	_, kinds, _ := sourcesOf(t, m, w.sam)
	if !slices.Contains(kinds, "direct") {
		t.Fatalf("sources of Sam %v", kinds)
	}
	w.wantOK(w.do("owner", http.MethodPost, "/api/v1/grants/"+g["id"].(string)+"/revoke", `{"reason":"Back to personal"}`, key()...))
	w.wantOK(w.setPersonal(w.assistent, w.owner))
}

// TestGuidedFlowAuditDetails (SEC-AUD-5): the audit details of the guided flow hold IDs, codes and
// the reason, never a display name.
func TestGuidedFlowAuditDetails(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	r := w.createGrant("owner", with(w.samNotizen(), resolveJSON("Guided flow", w.assistent, w.lotse)))
	w.want(r, http.StatusCreated, "")
	o := w.do("owner", http.MethodPost, "/api/v1/owners", `{"user_id":"`+w.alex.String()+`","reason":"Deputy",`+resolveJSON("Owner too", w.texter)+`}`, key()...)
	w.want(o, http.StatusCreated, "")
	allowed := []string{"from_billing_mode", "to_billing_mode", "from_credential_holder_id", "reason", "no_change", "ip_hmac", "ip_key_id"}
	for _, resp := range []*httpservertest.Response{r, o} {
		for _, a := range w.auditsOf(corr(t, resp)) {
			for _, name := range []string{"owner", "alex", "sam", "robin", "assistent", "lotse", "texter", "runner", "Research", "Notizen"} {
				if strings.Contains(a.Details, `"`+name+`"`) {
					t.Fatalf("details of %s name %s: %s", a.Action, name, a.Details)
				}
			}
			if a.Action != "agent.billing_mode.set" {
				continue
			}
			var d map[string]any
			must(t, json.Unmarshal([]byte(a.Details), &d))
			for k := range d {
				if !slices.Contains(allowed, k) {
					t.Fatalf("unexpected detail %s in %v", k, d)
				}
			}
		}
	}
}
