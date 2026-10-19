// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/api/routes"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
	"github.com/OWNER/bothy/services/server/internal/terminal/domain"
	"github.com/OWNER/bothy/services/server/test/terminaltest"
	"github.com/OWNER/bothy/services/server/test/terminalworld"
)

// tworld is the world of the brief M2-40 over PostgreSQL and the HTTP chain: the Owner (owner),
// Alex, Sam and Robin; the group Leser with Sam; R1 (active, assigned to Alex, organization,
// reporting bothy-s-alex and bothy-s-sam), R2 (active, personal with holder Robin, reporting
// bothy-s-robin and bothy-s-owner), R3 (active, assigned to Sam, declared server host, reporting
// bothy-s-sam and bothy-s-owner); the assignments Alex and Sam on R1, Robin and the Owner on R2,
// Sam and the Owner on R3; grants terminal to Alex and Sam on R1 and to Sam on R3.
type tworld struct {
	*terminalworld.World
	owner, alex, sam, robin, leser ids.ID
	r1, r2, r3                     ids.ID
	// su are the session user rows by host and person.
	su map[[2]ids.ID]ids.ID
	// grants are the grants of the world by subject and host.
	grants map[[2]ids.ID]ids.ID
}

// newTWorld builds the world.
func newTWorld(t *testing.T) *tworld {
	t.Helper()
	w := &tworld{World: terminalworld.NewEmpty(t), su: map[[2]ids.ID]ids.ID{}, grants: map[[2]ids.ID]ids.ID{}}
	w.owner = w.Person("owner", true)
	w.Robin = w.owner
	w.alex, w.sam, w.robin = w.Person("alex", false), w.Person("sam", false), w.Person("robin", false)
	for id, name := range map[ids.ID]string{w.owner: "Owner", w.alex: "Alex", w.sam: "Sam", w.robin: "Robin"} {
		w.SQL(`UPDATE principals SET display_name = $2 WHERE id = $1`, id.String(), name)
	}
	g := w.Want(w.Do("owner", http.MethodPost, "/api/v1/groups", `{"name":"Leser"}`, terminalworld.Key()...), http.StatusCreated, "")
	w.leser = ids.MustParse(g["id"].(string))
	w.Want(w.Do("owner", http.MethodPost, "/api/v1/groups/"+w.leser.String()+"/members", `{"user_id":"`+w.sam.String()+`"}`, terminalworld.Key()...),
		http.StatusCreated, "")
	w.r1 = w.Host(terminaltest.Host{Name: "R1", AssignedUserID: w.alex, LLMCredentials: "organization", CreatedBy: w.owner,
		ReportedSessionUsers: []string{"bothy-s-alex", "bothy-s-sam"}})
	w.r2 = w.Host(terminaltest.Host{Name: "R2", LLMCredentials: "personal", CredentialHolderID: w.robin, CreatedBy: w.owner,
		ReportedSessionUsers: []string{"bothy-s-robin", "bothy-s-owner"}})
	w.r3 = w.Host(terminaltest.Host{Name: "R3", AssignedUserID: w.sam, LLMCredentials: "organization", DeclaredServerHost: true, CreatedBy: w.owner,
		ReportedSessionUsers: []string{"bothy-s-sam", "bothy-s-owner"}})
	for _, a := range []struct {
		host, who ids.ID
		name      string
	}{{w.r1, w.alex, "bothy-s-alex"}, {w.r1, w.sam, "bothy-s-sam"}, {w.r2, w.robin, "bothy-s-robin"}, {w.r2, w.owner, "bothy-s-owner"},
		{w.r3, w.sam, "bothy-s-sam"}, {w.r3, w.owner, "bothy-s-owner"}} {
		w.su[[2]ids.ID{a.host, a.who}] = w.SessionUser(a.host, a.who, a.name)
	}
	for _, g := range [][2]ids.ID{{w.alex, w.r1}, {w.sam, w.r1}, {w.sam, w.r3}} {
		w.grants[g] = w.grant(t, "user", g[0], g[1], http.StatusCreated, "")
	}
	return w
}

// grant creates a grant terminal to the subject on host as the Owner and expects status and code.
func (w *tworld) grant(t *testing.T, kind string, subject, host ids.ID, status int, code string) ids.ID {
	t.Helper()
	m := w.Want(w.Do("owner", http.MethodPost, "/api/v1/grants", `{"subject_kind":"`+kind+`","subject_id":"`+subject.String()+
		`","target_kind":"host","target_id":"`+host.String()+`","capabilities":["terminal"],"reason":"Vertretung"}`, terminalworld.Key()...), status, code)
	if status != http.StatusCreated {
		return ids.ID{}
	}
	return ids.MustParse(m["id"].(string))
}

// revoke revokes the grant id as the Owner.
func (w *tworld) revoke(t *testing.T, id ids.ID) {
	t.Helper()
	w.Want(w.Do("owner", http.MethodPost, "/api/v1/grants/"+id.String()+"/revoke", `{"reason":"Ende"}`, terminalworld.Key()...), http.StatusOK, "")
}

// declare sets the LLM login of host as the Owner.
func (w *tworld) declare(host ids.ID, body string) map[string]any {
	return w.Want(w.Do("owner", http.MethodPut, terminalworld.HostPath(host)+"/llm-credentials", body, terminalworld.IfMatch(w.HostVersion(host))...),
		http.StatusOK, "")
}

// open authorizes terminal.open on host for the token name.
func (w *tworld) open(t *testing.T, name string, host ids.ID) authorize.Decision {
	t.Helper()
	d, err := w.Az.Authorize(context.Background(), w.Actor(name), authorize.ActionTerminalOpen, authorize.Resource{Kind: authorize.ResourceHost, ID: host})
	if err != nil {
		t.Fatal(err)
	}
	return d
}

// session inserts an open session of who on host with its session user.
func (w *tworld) session(host, who ids.ID) ids.ID {
	return w.TerminalSession(host, who, w.su[[2]ids.ID{host, who}], "active")
}

// closeReason returns the state and close reason of the session id.
func (w *tworld) closeReason(id ids.ID) (string, string) {
	return w.Text(`SELECT state FROM terminal_sessions WHERE id = $1`, id.String()),
		w.Text(`SELECT coalesce(close_reason, '') FROM terminal_sessions WHERE id = $1`, id.String())
}

// counts returns the rows of the tables a refused change must not touch.
func (w *tworld) counts() [6]int {
	return [6]int{
		w.Count(`SELECT count(*) FROM grants`), w.Count(`SELECT coalesce(sum(version), 0)::int FROM hosts`),
		w.Count(`SELECT count(*) FROM terminal_sessions WHERE state = 'closed'`), w.PermissionVersion(),
		w.Count(`SELECT count(*) FROM event_log`), w.Count(`SELECT count(*) FROM audit_events WHERE decision = 'allow'`),
	}
}

// i6Denials returns the deny entries with i6_violation.
func (w *tworld) i6Denials() []terminalworld.AuditRow {
	w.Audits("grant.create")
	var out []terminalworld.AuditRow
	for _, a := range []string{"grant.create", "host.manage", "user.enable"} {
		for _, r := range w.Audits(a) {
			if r.Reason == "i6_violation" {
				out = append(out, r)
			}
		}
	}
	return out
}

// TestTMT09_I6 (TM-T09, AZ-M57 over HTTP): grants on the personal host R2 to Sam and to Leser and
// the switch of R1 to personal with a grant to Sam roll back with 409 i6_violation, the hint and
// the conflicts; each refusal leaves every table unchanged and writes one deny entry; the grant to
// the holder Robin is created.
func TestTMT09_I6(t *testing.T) {
	t.Parallel()
	w := newTWorld(t)
	refusals := []func() map[string]any{
		func() map[string]any {
			return w.Want(w.Do("owner", http.MethodPost, "/api/v1/grants", `{"subject_kind":"user","subject_id":"`+w.sam.String()+
				`","target_kind":"host","target_id":"`+w.r2.String()+`","capabilities":["terminal"],"reason":"x"}`, terminalworld.Key()...),
				http.StatusConflict, "i6_violation")
		},
		func() map[string]any {
			return w.Want(w.Do("owner", http.MethodPost, "/api/v1/grants", `{"subject_kind":"group","subject_id":"`+w.leser.String()+
				`","target_kind":"host","target_id":"`+w.r2.String()+`","capabilities":["terminal"],"reason":"x"}`, terminalworld.Key()...),
				http.StatusConflict, "i6_violation")
		},
		func() map[string]any {
			return w.Want(w.Do("owner", http.MethodPut, terminalworld.HostPath(w.r1)+"/llm-credentials",
				`{"llm_credentials":"personal","credential_holder_id":"`+w.alex.String()+`","reason":"Abo von Alex"}`, terminalworld.IfMatch(w.HostVersion(w.r1))...),
				http.StatusConflict, "i6_violation")
		},
	}
	wantHosts := []ids.ID{w.r2, w.r2, w.r1}
	wantHolders := []ids.ID{w.robin, w.robin, w.alex}
	for i, refuse := range refusals {
		before, denials := w.counts(), len(w.i6Denials())
		p := refuse()
		if p["hint"] != "Revoke the other grants on this host or declare organization first." {
			t.Fatalf("refusal %d: hint %v", i, p["hint"])
		}
		list, _ := p["conflicts"].([]any)
		if len(list) != 1 {
			t.Fatalf("refusal %d: conflicts %v", i, p["conflicts"])
		}
		c := list[0].(map[string]any)
		if c["code"] != "i6_violation" || c["host_id"] != wantHosts[i].String() || c["credential_holder_id"] != wantHolders[i].String() {
			t.Fatalf("refusal %d: conflict %v", i, c)
		}
		if ps, _ := c["principal_ids"].([]any); len(ps) != 1 || ps[0] != w.sam.String() {
			t.Fatalf("refusal %d: principal_ids %v", i, c["principal_ids"])
		}
		if after := w.counts(); after != before {
			t.Fatalf("refusal %d changed something: %v -> %v", i, before, after)
		}
		if n := len(w.i6Denials()) - denials; n != 1 {
			t.Fatalf("refusal %d: %d deny entries with i6_violation, want 1", i, n)
		}
	}
	w.grant(t, "user", w.robin, w.r2, http.StatusCreated, "")
	assertNoContent(t, w)
}

// TestI6_UserEnableRevivesGrant (Design note 9): Sam is disabled, so the grant to Sam on R1 is
// dormant and R1 may switch to personal with holder Alex; enabling Sam would make the grant valid
// again and rolls back with 409 i6_violation; Sam stays disabled.
func TestI6_UserEnableRevivesGrant(t *testing.T) {
	t.Parallel()
	w := newTWorld(t)
	w.SQL(`UPDATE principals SET status = 'disabled' WHERE id = $1`, w.sam.String())
	w.declare(w.r1, `{"llm_credentials":"personal","credential_holder_id":"`+w.alex.String()+`","reason":"Abo von Alex"}`)
	p := w.Want(w.Do("owner", http.MethodPost, "/api/v1/users/"+w.sam.String()+"/enable", `{"reason":"Back again"}`, terminalworld.Key()...),
		http.StatusConflict, "i6_violation")
	if list, _ := p["conflicts"].([]any); len(list) != 1 {
		t.Fatalf("conflicts %v", p["conflicts"])
	}
	if s := w.Text(`SELECT status FROM principals WHERE id = $1`, w.sam.String()); s != "disabled" {
		t.Fatalf("Sam is %s", s)
	}
}

// TestI6_SwitchEndsForeignSessions (24 section 7.3 point 3): the switch of R1 to personal with
// holder Alex ends the Owner's session with i6_violation in the same commit as host.updated and
// the host.manage entry, keeps Alex's session and removes R1 from the Owner's view.
func TestI6_SwitchEndsForeignSessions(t *testing.T) {
	t.Parallel()
	w := newTWorld(t)
	w.SQL(`UPDATE hosts SET reported_session_users = reported_session_users || '[{"os_user":"bothy-s-owner","workdirs":[]}]'::jsonb WHERE id = $1`, w.r1.String())
	w.su[[2]ids.ID{w.r1, w.owner}] = w.SessionUser(w.r1, w.owner, "bothy-s-owner")
	alexSes, ownerSes := w.session(w.r1, w.alex), w.session(w.r1, w.owner)
	w.revoke(t, w.grants[[2]ids.ID{w.sam, w.r1}])
	seq := w.LastSeq()
	m := w.declare(w.r1, `{"llm_credentials":"personal","credential_holder_id":"`+w.alex.String()+`","reason":"Abo von Alex"}`)
	if m["llm_credentials"] != "personal" || m["credential_holder_id"] != w.alex.String() {
		t.Fatalf("host %v", m)
	}
	if st, r := w.closeReason(ownerSes); st != "closed" || r != "i6_violation" {
		t.Fatalf("the Owner's session: %s %s", st, r)
	}
	if st, _ := w.closeReason(alexSes); st != "active" {
		t.Fatalf("Alex's session: %s", st)
	}
	corr := w.Text(`SELECT correlation_id::text FROM event_log WHERE type = 'host.updated' AND resource_id = $1 ORDER BY seq DESC LIMIT 1`, w.r1.String())
	for _, action := range []string{"host.manage", "terminal.terminate"} {
		if n := w.Count(`SELECT count(*) FROM audit_events WHERE action = $1 AND correlation_id::text = $2`, action, corr); n != 1 {
			t.Fatalf("%d %s entries with the correlation of host.updated", n, action)
		}
	}
	got := w.AccessChanges(seq)[w.owner]
	if len(got) != 1 || !terminalworld.HasHostEntry(t, got[0], "removed", w.r1, false) {
		t.Fatalf("access.changed of the Owner: %v", got)
	}
	if d := w.open(t, "owner", w.r1); d.Reason != "i6_holder_only" {
		t.Fatalf("the Owner opens R1: %+v", d)
	}
	assertNoContent(t, w)
}

// TestI6_HolderChangeEndsForeignSessions: R4 is personal with holder Robin, a second owner; the
// change of the holder to the Owner ends Robin's session with i6_violation. A change that fails
// in phase 3.1 (a grant to Sam written past the use cases violates I6) leaves the session open and
// the declaration unchanged.
func TestI6_HolderChangeEndsForeignSessions(t *testing.T) {
	t.Parallel()
	w := newTWorld(t)
	w.SQL(`INSERT INTO owners (id, user_id, reason) VALUES ($1, $2, 'second owner')`, ids.New().String(), w.robin.String())
	r4 := w.Host(terminaltest.Host{Name: "R4", LLMCredentials: "personal", CredentialHolderID: w.robin, CreatedBy: w.owner,
		ReportedSessionUsers: []string{"bothy-s-robin", "bothy-s-owner"}})
	w.su[[2]ids.ID{r4, w.robin}] = w.SessionUser(r4, w.robin, "bothy-s-robin")
	w.su[[2]ids.ID{r4, w.owner}] = w.SessionUser(r4, w.owner, "bothy-s-owner")
	robinSes := w.session(r4, w.robin)
	w.SQL(`INSERT INTO grants (id, subject_kind, subject_principal_id, target_kind, target_host_id, capabilities, reason, granted_by)
		VALUES ($1, 'user', $2, 'host', $3, '{terminal}', 'fixture', $4)`, ids.New().String(), w.sam.String(), r4.String(), w.owner.String())
	body := `{"llm_credentials":"personal","credential_holder_id":"` + w.owner.String() + `","reason":"Owner takes over"}`
	w.Want(w.Do("owner", http.MethodPut, terminalworld.HostPath(r4)+"/llm-credentials", body, terminalworld.IfMatch(w.HostVersion(r4))...),
		http.StatusConflict, "i6_violation")
	if st, _ := w.closeReason(robinSes); st != "active" {
		t.Fatalf("a rolled back change ended Robin's session: %s", st)
	}
	if h := w.Text(`SELECT credential_holder_id::text FROM hosts WHERE id = $1`, r4.String()); h != w.robin.String() {
		t.Fatalf("holder %s after a rollback", h)
	}
	w.SQL(`DELETE FROM grants WHERE target_host_id = $1`, r4.String())
	w.declare(r4, body)
	if st, r := w.closeReason(robinSes); st != "closed" || r != "i6_violation" {
		t.Fatalf("Robin's session: %s %s", st, r)
	}
}

// TestSR10_AssigneeChangeEndsSessions (07 SR10, Design note 11): a new assignee ends the sessions
// of the old one with access_revoked, owners keep theirs; clearing ends the sessions of the
// assignee; an old assignee who is an owner keeps the session.
func TestSR10_AssigneeChangeEndsSessions(t *testing.T) {
	t.Parallel()
	w := newTWorld(t)
	w.SQL(`UPDATE hosts SET reported_session_users = reported_session_users || '[{"os_user":"bothy-s-owner","workdirs":[]}]'::jsonb WHERE id = $1`, w.r1.String())
	w.su[[2]ids.ID{w.r1, w.owner}] = w.SessionUser(w.r1, w.owner, "bothy-s-owner")
	alexSes, ownerSes := w.session(w.r1, w.alex), w.session(w.r1, w.owner)
	update := func(body string) {
		w.Want(w.Do("owner", http.MethodPatch, terminalworld.HostPath(w.r1), body, w.Keyed(w.r1)...), http.StatusOK, "")
	}
	update(`{"assigned_user_id":"` + w.sam.String() + `","reason":"Sam takes over"}`)
	if st, r := w.closeReason(alexSes); st != "closed" || r != "access_revoked" {
		t.Fatalf("Alex's session: %s %s", st, r)
	}
	if st, _ := w.closeReason(ownerSes); st != "active" {
		t.Fatalf("the Owner's session: %s", st)
	}
	samSes := w.session(w.r1, w.sam)
	update(`{"clear_assigned_user":true,"reason":"Niemand"}`)
	if st, r := w.closeReason(samSes); st != "closed" || r != "access_revoked" {
		t.Fatalf("Sam's session: %s %s", st, r)
	}
	update(`{"assigned_user_id":"` + w.alex.String() + `","reason":"Alex wieder"}`)
	w.SQL(`INSERT INTO owners (id, user_id, reason) VALUES ($1, $2, 'second owner')`, ids.New().String(), w.alex.String())
	alexSes2 := w.session(w.r1, w.alex)
	update(`{"assigned_user_id":"` + w.sam.String() + `","reason":"Sam wieder"}`)
	if st, _ := w.closeReason(alexSes2); st != "active" {
		t.Fatalf("the session of Alex as an owner: %s", st)
	}
}

// TestSetHostLlmCredentials_HTTP (Design note 10): the happy path, the call without effect, the
// field errors, the preconditions, the refusals of authorize and a retired host.
func TestSetHostLlmCredentials_HTTP(t *testing.T) {
	t.Parallel()
	w := newTWorld(t)
	path := terminalworld.HostPath(w.r3) + "/llm-credentials"
	put := func(name, body string, headers ...string) *httpservertest.Response {
		return w.Do(name, http.MethodPut, path, body, headers...)
	}
	v, pv, events := w.HostVersion(w.r3), w.PermissionVersion(), len(w.Events("host.updated"))
	r := put("owner", `{"llm_credentials":"personal","credential_holder_id":"`+w.sam.String()+`","reason":"Abo von Sam"}`, terminalworld.IfMatch(v)...)
	m := w.Want(r, http.StatusOK, "")
	if int64(m["version"].(float64)) != v+1 || r.Header.Get("ETag") != `"`+strconv.FormatInt(v+1, 10)+`"` || w.PermissionVersion() != pv+1 {
		t.Fatalf("version %v, ETag %s, permission_version %d after %d", m["version"], r.Header.Get("ETag"), w.PermissionVersion(), pv)
	}
	if n := len(w.Events("host.updated")); n != events+1 {
		t.Fatalf("%d host.updated", n-events)
	}
	allows := w.Allows("host.manage")
	last := allows[len(allows)-1].Details
	if last["operation"] != "llm_credentials" || last["from_llm_credentials"] != "organization" || last["to_llm_credentials"] != "personal" ||
		last["to_credential_holder_id"] != w.sam.String() || last["reason"] != "Abo von Sam" || last["from_credential_holder_id"] != nil {
		t.Fatalf("audit details %v", last)
	}
	// The same declaration again: 200 without a new version, event or rights version, audited.
	v, pv, events = w.HostVersion(w.r3), w.PermissionVersion(), len(w.Events("host.updated"))
	w.Want(put("owner", `{"llm_credentials":"personal","credential_holder_id":"`+w.sam.String()+`","reason":"Nochmal"}`, terminalworld.IfMatch(v)...),
		http.StatusOK, "")
	if w.HostVersion(w.r3) != v || w.PermissionVersion() != pv || len(w.Events("host.updated")) != events {
		t.Fatal("a call without effect changed something")
	}
	allows = w.Allows("host.manage")
	if d := allows[len(allows)-1].Details; d["no_change"] != true || d["operation"] != "llm_credentials" {
		t.Fatalf("no_change entry %v", d)
	}
	v = w.HostVersion(w.r3)
	for _, c := range []struct{ body, code, pointer string }{
		{`{"llm_credentials":"personal","reason":"x"}`, "required", "/credential_holder_id"},
		{`{"llm_credentials":"organization","credential_holder_id":"` + w.sam.String() + `","reason":"x"}`, "not_allowed", "/credential_holder_id"},
		{`{"llm_credentials":"personal","credential_holder_id":"` + ids.New().String() + `","reason":"x"}`, "unknown_user", "/credential_holder_id"},
		{`{"llm_credentials":"organization","reason":"   "}`, "required", "/reason"},
		{`{"llm_credentials":"organization","reason":"` + strings.Repeat("r", terminalworld.HostConfig.ReasonMaxChars+1) + `"}`, "too_long", "/reason"},
	} {
		p := w.Want(put("owner", c.body, terminalworld.IfMatch(v)...), http.StatusBadRequest, "validation_failed")
		if code, ptr := terminalworld.Field(p); code != c.code || ptr != c.pointer {
			t.Errorf("%s: %s at %s", c.body, code, ptr)
		}
	}
	agent := w.AgentToken("lotse")
	w.Want(w.Do("owner", http.MethodPut, terminalworld.HostPath(w.r3)+"/llm-credentials", `{"llm_credentials":"personal","credential_holder_id":"`+agent.String()+
		`","reason":"x"}`, terminalworld.IfMatch(v)...), http.StatusBadRequest, "validation_failed")
	body := `{"llm_credentials":"organization","reason":"Back again"}`
	w.Want(put("owner", body), http.StatusPreconditionRequired, "precondition_required")
	w.Want(put("owner", body, terminalworld.IfMatch(v+7)...), http.StatusPreconditionFailed, "version_conflict")
	unknown := w.Do("owner", http.MethodPut, terminalworld.HostPath(ids.New())+"/llm-credentials", body, terminalworld.IfMatch(1)...)
	w.Want(unknown, http.StatusNotFound, "not_found")
	invisible := w.Do("robin", http.MethodPut, terminalworld.HostPath(w.r1)+"/llm-credentials", body, terminalworld.IfMatch(1)...)
	terminalworld.SameAnswer(t, unknown, invisible)
	p := w.Want(w.Do("alex", http.MethodPut, terminalworld.HostPath(w.r1)+"/llm-credentials", body, terminalworld.IfMatch(1)...), http.StatusForbidden, "forbidden")
	if p["reason"] != "owner_only" {
		t.Fatalf("Alex: %v", p)
	}
	w.Session("owner-stale", w.owner, w.Clk.Now().Add(-terminalworld.IDConfig.StepUpMaxAge-time.Second))
	w.Want(put("owner-stale", body, terminalworld.IfMatch(v)...), http.StatusUnauthorized, "step_up_required")
	w.PersonalToken("alex-token", w.alex)
	p = w.Want(w.Do("alex-token", http.MethodPut, terminalworld.HostPath(w.r1)+"/llm-credentials", body, terminalworld.IfMatch(1)...),
		http.StatusForbidden, "forbidden")
	if p["reason"] != "subject_kind_not_allowed" {
		t.Fatalf("personal token: %v", p)
	}
	retired := w.Host(terminaltest.Host{Name: "Alt", State: "retired", CreatedBy: w.owner})
	w.Want(w.Do("owner", http.MethodPut, terminalworld.HostPath(retired)+"/llm-credentials", body, terminalworld.IfMatch(w.HostVersion(retired))...),
		http.StatusConflict, "conflict")
	assertNoContent(t, w)
}

// TestSR12_ReportDropsName (07 SR12, Design note 8): a report of R1 without bothy-s-alex raises the
// rights version, ends Alex's session with access_revoked and denies Alex; the same report again
// changes nothing; reported again, Alex may open; an invalid report writes nothing.
func TestSR12_ReportDropsName(t *testing.T) {
	t.Parallel()
	w := newTWorld(t)
	ctx := context.Background()
	alexSes := w.session(w.r1, w.alex)
	svc := w.Terminal.Service
	if d := w.open(t, "alex", w.r1); !d.Allowed() {
		t.Fatalf("Alex before the report: %+v", d)
	}
	pv, events := w.PermissionVersion(), len(w.Events("host.updated"))
	without := []domain.ReportedSessionUser{{OSUser: "bothy-s-sam", Workdirs: []string{}}}
	out, err := svc.RecordSessionUserReport(ctx, authorize.ActionHostConnect, w.r1, without)
	if err != nil || !out.Changed || out.EndedSessions != 1 {
		t.Fatalf("report: %+v %v", out, err)
	}
	if w.PermissionVersion() != pv+1 || len(w.Events("host.updated")) != events+1 {
		t.Fatal("the report raised no rights version or wrote no host.updated")
	}
	if st, r := w.closeReason(alexSes); st != "closed" || r != "access_revoked" {
		t.Fatalf("Alex's session: %s %s", st, r)
	}
	if d := w.open(t, "alex", w.r1); d.Reason != "terminal_no_session_user" {
		t.Fatalf("Alex after the report: %+v", d)
	}
	pv = w.PermissionVersion()
	if out, err := svc.RecordSessionUserReport(ctx, authorize.ActionHostConnect, w.r1, without); err != nil || out.Changed {
		t.Fatalf("the same report: %+v %v", out, err)
	}
	if w.PermissionVersion() != pv {
		t.Fatal("the same report raised the rights version")
	}
	again := append(slices.Clone(without), domain.ReportedSessionUser{OSUser: "bothy-s-alex"})
	if out, err := svc.RecordSessionUserReport(ctx, authorize.ActionHostConnect, w.r1, again); err != nil || !out.Changed {
		t.Fatalf("reported again: %+v %v", out, err)
	}
	if d := w.open(t, "alex", w.r1); !d.Allowed() {
		t.Fatalf("Alex reported again: %+v", d)
	}
	pv = w.PermissionVersion()
	for _, bad := range [][]domain.ReportedSessionUser{{{OSUser: "../etc"}}, {{OSUser: strings.Repeat("a", terminalworld.HostConfig.OSUserMaxChars+1)}},
		{{OSUser: "bothy-s-x", Workdirs: []string{"a\u202eb"}}}} {
		if _, err := svc.RecordSessionUserReport(ctx, authorize.ActionHostConnect, w.r1, bad); errs.KindOf(err) != errs.KindBadRequest {
			t.Fatalf("invalid report %v: %v", bad, err)
		}
	}
	if w.PermissionVersion() != pv || !strings.Contains(w.Text(`SELECT reported_session_users::text FROM hosts WHERE id = $1`, w.r1.String()), "bothy-s-alex") {
		t.Fatal("an invalid report wrote something")
	}
	assertNoContent(t, w)
}

// TestPreviewHostGrant (07 section 9.3, Design note 13): the hints, effects, violations and text of
// previews of grants on hosts, without any trace.
func TestPreviewHostGrant(t *testing.T) {
	t.Parallel()
	w := newTWorld(t)
	preview := func(kind string, subject, host ids.ID) map[string]any {
		t.Helper()
		before := [4]int{w.Count(`SELECT count(*) FROM grants`), w.Count(`SELECT count(*) FROM event_log`),
			w.Count(`SELECT count(*) FROM audit_events`), w.PermissionVersion()}
		m := w.Want(w.Do("owner", http.MethodPost, "/api/v1/grants/preview", `{"subject_kind":"`+kind+`","subject_id":"`+subject.String()+
			`","target_kind":"host","target_id":"`+host.String()+`","capabilities":["terminal"],"reason":"Vertretung"}`), http.StatusOK, "")
		after := [4]int{w.Count(`SELECT count(*) FROM grants`), w.Count(`SELECT count(*) FROM event_log`),
			w.Count(`SELECT count(*) FROM audit_events`), w.PermissionVersion()}
		if before != after {
			t.Fatalf("a preview left a trace: %v -> %v", before, after)
		}
		return m
	}
	hint := func(m map[string]any) map[string]any {
		for _, h := range m["hints"].([]any) {
			if hm := h.(map[string]any); hm["code"] == "terminal_not_effective" {
				return hm
			}
		}
		return nil
	}
	// language-check: off
	wantText := "Sam erhält das Zusatzrecht \"Terminal\" auf dem Rechner R1.\n" +
		"Hinweis: Die Freigabe wirkt für Sam nicht. Auf R1 öffnen nur Owner und die Person, der der Rechner zugewiesen ist, ein Terminal.\n" +
		"Hinweis: Die Freigabe für Sam hat kein Ablaufdatum."
	// language-check: on
	// With Sam's grant in place: the hint and the text, no effect.
	m := preview("user", w.sam, w.r1)
	if h := hint(m); h == nil || h["principal_id"] != w.sam.String() || m["allowed"] != true || m["text"] != wantText {
		t.Fatalf("preview with the grant: %v", m)
	}
	if len(m["effects"].([]any)) != 0 {
		t.Fatalf("effects with the grant in place: %v", m["effects"])
	}
	w.revoke(t, w.grants[[2]ids.ID{w.sam, w.r1}])
	m = preview("user", w.sam, w.r1)
	effects := m["effects"].([]any)
	if len(effects) != 1 || m["text"] != wantText || hint(m) == nil {
		t.Fatalf("preview after the revocation: %v", m)
	}
	e := effects[0].(map[string]any)
	if e["target_kind"] != "host" || e["target_id"] != w.r1.String() || e["before_level"] != "none" || e["after_level"] != "none" ||
		!slices.Equal(e["capabilities_added"].([]any), []any{"terminal"}) || e["principal"].(map[string]any)["id"] != w.sam.String() {
		t.Fatalf("effect %v", e)
	}
	if h := hint(preview("user", w.alex, w.r1)); h != nil {
		t.Fatalf("a hint for the assignee: %v", h)
	}
	w.Want(w.Do("owner", http.MethodPatch, terminalworld.HostPath(w.r1), `{"revoke_session_users":["`+w.alex.String()+`"],"reason":"Ende"}`, w.Keyed(w.r1)...),
		http.StatusOK, "")
	m = preview("user", w.alex, w.r1)
	// language-check: off
	if hint(m) == nil || !strings.Contains(m["text"].(string), "erst, wenn Alex auf R1 einen eigenen Sitzungsbenutzer hat") {
		t.Fatalf("preview without a session user: %v", m)
	}
	// language-check: on
	if h := hint(preview("group", w.leser, w.r1)); h == nil || h["group_id"] != w.leser.String() || h["count"] != float64(1) {
		t.Fatalf("group hint %v", h)
	}
	m = preview("user", w.sam, w.r2)
	vs := m["invariant_violations"].([]any)
	if m["allowed"] != false || len(vs) != 1 || vs[0].(map[string]any)["code"] != "i6_violation" || vs[0].(map[string]any)["host_id"] != w.r2.String() || hint(m) != nil {
		t.Fatalf("preview on the personal host: %v", m)
	}
	// language-check: off
	if !strings.Contains(m["text"].(string), "Abgelehnt: Auf R2 gilt die persönliche LLM-Anmeldung von Robin.") {
		t.Fatalf("text %q", m["text"])
	}
	// language-check: on
}

// TestTerminalAuthzAudit_NoContent (SEC-LOG-2, SEC-AUD-5): the audit rows of the flows of M2-40
// hold no host name, no session user name outside os_user of a session user change, no token or
// signature text and no JSON null.
func TestTerminalAuthzAudit_NoContent(t *testing.T) {
	t.Parallel()
	w := newTWorld(t)
	w.session(w.r2, w.owner)
	w.open(t, "owner", w.r1)
	w.open(t, "sam", w.r1)
	w.grant(t, "user", w.sam, w.r2, http.StatusConflict, "i6_violation")
	w.declare(w.r2, `{"llm_credentials":"personal","credential_holder_id":"`+w.owner.String()+`","reason":"Owner takes over"}`)
	if _, err := w.Terminal.Service.RecordSessionUserReport(context.Background(), authorize.ActionHostConnect, w.r1,
		[]domain.ReportedSessionUser{{OSUser: "bothy-s-sam"}}); err != nil {
		t.Fatal(err)
	}
	assertNoContent(t, w)
}

// assertNoContent checks every audit row of the world (the recorder validated each against
// audit_details.v1 when it wrote it): no host name, no session user name except os_user of
// host.manage, no token and no JSON null.
func assertNoContent(t *testing.T, w *tworld) {
	t.Helper()
	if err := w.Rec.Aggregator().Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
	rows, err := w.Owner.Query(context.Background(), `SELECT action, details::text FROM audit_events`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var action, raw string
		if err := rows.Scan(&action, &raw); err != nil {
			t.Fatal(err)
		}
		var details map[string]any
		if err := json.Unmarshal([]byte(raw), &details); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(raw, "null") || strings.Contains(raw, `"R1"`) || strings.Contains(raw, `"R2"`) || strings.Contains(raw, `"R3"`) {
			t.Errorf("%s: content in %s", action, raw)
		}
		delete(details, "os_user")
		cleaned, _ := json.Marshal(details)
		if strings.Contains(string(cleaned), "bothy-s-") || strings.Contains(string(cleaned), "Bearer") {
			t.Errorf("%s: a session user or token in %s", action, raw)
		}
	}
}

// TestAZP1_TerminalAuthzRoutes (AZ-P1, network point of D-W-101): setHostLlmCredentials is enabled
// with host.manage and rl.admin, and no route with a terminal action, host.pair or host.connect is
// enabled by this ticket.
func TestAZP1_TerminalAuthzRoutes(t *testing.T) {
	t.Parallel()
	found := false
	for _, op := range routes.Enabled() {
		if op.ID == "setHostLlmCredentials" {
			found = op.Action == "host.manage" && op.RateLimitClass == "rl.admin" && op.Pattern == "PUT /api/v1/hosts/{host_id}/llm-credentials"
		}
		if slices.Contains([]authorize.Action{authorize.ActionTerminalOpen, authorize.ActionTerminalAttach, authorize.ActionTerminalStream,
			authorize.ActionTerminalReadOwn, authorize.ActionTerminalClose, authorize.ActionHostPair, authorize.ActionHostConnect},
			authorize.Action(op.Action)) {
			t.Errorf("route %s with %s is enabled", op.ID, op.Action)
		}
	}
	if !found {
		t.Fatal("setHostLlmCredentials is not enabled with host.manage and rl.admin")
	}
}
