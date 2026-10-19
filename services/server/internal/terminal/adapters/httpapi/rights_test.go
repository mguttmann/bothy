// SPDX-License-Identifier: Apache-2.0

package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	auditdomain "github.com/OWNER/bothy/services/server/internal/audit/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
	"github.com/OWNER/bothy/services/server/test/terminaltest"
	"github.com/OWNER/bothy/services/server/test/terminalworld"
)

// TestHostManage_StepUpReasonAudit (M2-39, PLAN row): for createHost, updateHost and setHostState
// a step-up older than auth.step_up_max_age is 401 step_up_required and changes nothing; a
// missing, blank, bidi or too long reason is 400; every successful write has its audit entries
// with the reason, valid against audit_details.v1 and without the host name; refusals by
// authorize are audited.
func TestHostManage_StepUpReasonAudit(t *testing.T) {
	t.Parallel()
	w := terminalworld.New(t)
	w.Session("robin-stale", w.Robin, w.Clk.Now().Add(-terminalworld.IDConfig.StepUpMaxAge-time.Second))
	ops := []struct{ method, path, body string }{
		{http.MethodPost, terminalworld.HostsPath, `{"name":"Stale","reason":"R"}`},
		{http.MethodPatch, terminalworld.HostPath(w.R3), `{"name":"Stale","reason":"R"}`},
		{http.MethodPost, terminalworld.HostPath(w.R3) + "/state", `{"state":"disabled","reason":"R"}`},
	}
	hosts, v := w.Count(`SELECT count(*) FROM hosts`), w.HostVersion(w.R3)
	for _, o := range ops {
		w.Want(w.Do("robin-stale", o.method, o.path, o.body, w.Keyed(w.R3)...), http.StatusUnauthorized, "step_up_required")
	}
	if w.Count(`SELECT count(*) FROM hosts`) != hosts || w.HostVersion(w.R3) != v {
		t.Fatal("a stale step-up changed something")
	}
	for _, reason := range []string{`"   "`, `"a\u202eb"`, `"` + strings.Repeat("r", terminalworld.HostConfig.ReasonMaxChars+1) + `"`} {
		for _, o := range ops {
			body := strings.Replace(o.body, `"R"`, reason, 1)
			p := w.Want(w.Do("robin", o.method, o.path, body, w.Keyed(w.R3)...), http.StatusBadRequest, "validation_failed")
			if _, ptr := terminalworld.Field(p); ptr != "/reason" {
				t.Errorf("%s %s with reason %s: %v", o.method, o.path, reason, p)
			}
		}
	}
	for _, o := range ops {
		body := strings.Replace(o.body, `"R"`, `"Planned change"`, 1)
		body = strings.Replace(body, `"Stale"`, `"Fresh"`, 1)
		if o.method == http.MethodPatch {
			body = strings.Replace(body, `"Fresh"`, `"Fresh two"`, 1)
		}
		status := http.StatusOK
		if o.method == http.MethodPost && o.path == terminalworld.HostsPath {
			status = http.StatusCreated
		}
		w.Want(w.Do("robin", o.method, o.path, body, w.Keyed(w.R3)...), status, "")
	}
	for _, r := range w.Allows("host.manage") {
		if err := auditdomain.DetailsV1(r.Details).Validate(); err != nil {
			t.Errorf("details %v: %v", r.Details, err)
		}
		raw, _ := json.Marshal(r.Details)
		for _, name := range []string{"Fresh", "Spare machine", "Build server"} {
			if strings.Contains(string(raw), name) {
				t.Errorf("the host name %q is in the audit details %s", name, raw)
			}
		}
		if r.Details["operation"] != "session_user" && r.Details["reason"] != "Planned change" && r.Details["no_change"] == nil {
			t.Errorf("audit entry without the reason: %v", r.Details)
		}
	}
	w.Want(w.Do("alex", http.MethodPatch, terminalworld.HostPath(w.R1), `{"name":"Mine","reason":"x"}`, w.Keyed(w.R1)...), http.StatusForbidden, "forbidden")
	var denied bool
	for _, r := range w.Audits("host.manage") {
		denied = denied || r.Decision == "deny" && r.Reason == "owner_only"
	}
	if !denied {
		t.Fatal("the refusal of Alex is not audited")
	}
}

// terminalworld.RequiredHostMembers are the required members of the schema Host.
var requiredHostMembers = []string{"id", "name", "state", "llm_credentials", "declared_server_host", "key_generation", "profiles", "doctor",
	"connected", "created_at", "version"}

// TestHostUpdated_Event (M2-39, 05 section 6, 24 section 10.2, T-TM-01): every host.updated row has
// the required members of Host under host, empty scope, receive action host.read and the new
// version; R1's payload has key_fingerprint but no noise_public_key and no pairing code, and
// key_changed is absent; the receive decision host.read in filter mode lets Robin and Alex receive
// R1's terminalworld.event and Sam only once he holds a grant.
func TestHostUpdated_Event(t *testing.T) {
	t.Parallel()
	w := terminalworld.New(t)
	m := w.Want(w.Do("robin", http.MethodPost, terminalworld.HostsPath, `{"name":"Fresh","reason":"New"}`, terminalworld.Key()...), http.StatusCreated, "")
	code := m["pairing_code"].(string)
	w.Want(w.Do("robin", http.MethodPatch, terminalworld.HostPath(w.R1), `{"name":"Build server two","reason":"Rename"}`, w.Keyed(w.R1)...), http.StatusOK, "")
	evs := w.Events("host.updated")
	if len(evs) != 2 {
		t.Fatalf("%d events", len(evs))
	}
	for _, e := range evs {
		host, _ := e.Payload["host"].(map[string]any)
		for _, k := range requiredHostMembers {
			if _, ok := host[k]; !ok {
				t.Errorf("terminalworld.Event of %s lacks %s", e.ResourceID, k)
			}
		}
		if len(e.Payload) != 1 || e.HasScope || e.ReceiveAction != "host.read" || e.Audience != "" || int64(host["version"].(float64)) != e.Version {
			t.Errorf("envelope %+v", e)
		}
		if _, ok := host["noise_public_key"]; ok || strings.Contains(e.Raw, code) {
			t.Errorf("terminalworld.Event %s carries a terminalworld.Key or the code", e.ResourceID)
		}
	}
	if r1 := evs[1].Payload["host"].(map[string]any); r1["key_fingerprint"] != terminaltest.KeyFingerprint || evs[1].ResourceID != w.R1.String() {
		t.Fatalf("R1 terminalworld.Event %v", r1)
	}
	receives := func(name string) bool {
		d, err := w.Az.Authorize(authorize.Filter(context.Background()), w.Actors.Get(name), authorize.ActionHostRead,
			authorize.Resource{Kind: authorize.ResourceHost, ID: w.R1})
		if err != nil {
			t.Fatal(err)
		}
		return d.Allowed()
	}
	if !receives("robin") || !receives("alex") || receives("sam") {
		t.Fatal("receive decision of R1's terminalworld.Event")
	}
	w.Want(w.Do("robin", http.MethodPost, "/api/v1/grants", `{"subject_kind":"user","subject_id":"`+w.Sam.String()+
		`","target_kind":"host","target_id":"`+w.R1.String()+`","capabilities":["terminal"],"reason":"Sam too"}`, terminalworld.Key()...), http.StatusCreated, "")
	if !receives("sam") {
		t.Fatal("Sam with a grant does not receive R1's terminalworld.Event")
	}
}

// TestCreateGrant_HostTarget (M2-39, 07 section 2.12 points 1 to 3, 04 section 8.2): Robin grants
// terminal on R1 to Sam: 201, Sam gets access.changed with an added host entry and grant.changed
// has an empty scope; revoking gives removed; a group grant gives the members the entry; Alex as
// holder gets 403 owner_only; a random host is 404; a retired host is 409 conflict.
func TestCreateGrant_HostTarget(t *testing.T) {
	t.Parallel()
	w := terminalworld.New(t)
	grant := func(name, subjectKind string, subject, host ids.ID) *httpservertest.Response {
		return w.Do(name, http.MethodPost, "/api/v1/grants", `{"subject_kind":"`+subjectKind+`","subject_id":"`+subject.String()+
			`","target_kind":"host","target_id":"`+host.String()+`","capabilities":["terminal"],"reason":"Terminal"}`, terminalworld.Key()...)
	}
	seq := w.LastSeq()
	g := w.Want(grant("robin", "user", w.Sam, w.R1), http.StatusCreated, "")
	if got := w.AccessChanges(seq)[w.Sam]; len(got) != 1 || !terminalworld.HasHostEntry(t, got[0], "added", w.R1, true) {
		t.Fatalf("Sam's access.changed %v", got)
	}
	for _, e := range w.Events("grant.changed") {
		if e.HasScope {
			t.Errorf("grant.changed with a scope: %s", e.Raw)
		}
	}
	seq = w.LastSeq()
	w.Want(w.Do("robin", http.MethodPost, "/api/v1/grants/"+g["id"].(string)+"/revoke", `{"reason":"Done"}`, terminalworld.Key()...), http.StatusOK, "")
	if got := w.AccessChanges(seq)[w.Sam]; len(got) != 1 || !terminalworld.HasHostEntry(t, got[0], "removed", w.R1, false) {
		t.Fatalf("Sam's removal %v", got)
	}
	r4 := w.Host(terminaltest.Host{Name: "Group host", State: "active"})
	seq = w.LastSeq()
	w.Want(grant("robin", "group", w.Leser, r4), http.StatusCreated, "")
	if got := w.AccessChanges(seq)[w.Sam]; len(got) != 1 || !terminalworld.HasHostEntry(t, got[0], "added", r4, true) {
		t.Fatalf("group member's access.changed %v", got)
	}
	if p := w.Want(grant("alex", "user", w.Sam, w.R1), http.StatusForbidden, "forbidden"); p["reason"] != "owner_only" {
		t.Fatalf("holder: %v", p)
	}
	w.Want(grant("robin", "user", w.Sam, ids.New()), http.StatusNotFound, "not_found")
	retired := w.Host(terminaltest.Host{Name: "Old", State: "retired"})
	w.Want(grant("robin", "user", w.Sam, retired), http.StatusConflict, "conflict")
}

// TestAccessChanged_HostEntries (M2-39, 04 section 8.2): createHost gives Robin an added host
// entry and nobody else an entry; addOwner for Alex gives Alex added entries for every host;
// setHostState and a holder change give no access.changed.
func TestAccessChanged_HostEntries(t *testing.T) {
	t.Parallel()
	w := terminalworld.New(t)
	seq := w.LastSeq()
	w.Want(w.Do("robin", http.MethodPost, "/api/v1/owners", `{"user_id":"`+w.Alex.String()+`","reason":"Second owner"}`, terminalworld.Key()...), http.StatusCreated, "")
	got := w.AccessChanges(seq)[w.Alex]
	if len(got) != 1 {
		t.Fatalf("Alex's access.changed %v", got)
	}
	for _, h := range []ids.ID{w.R2, w.R3} {
		if !terminalworld.HasHostEntry(t, got[0], "added", h, true) {
			t.Errorf("addOwner: no added entry for %s in %s", h, got[0])
		}
	}
	seq = w.LastSeq()
	w.Want(w.Do("robin", http.MethodPost, terminalworld.HostPath(w.R3)+"/state", `{"state":"disabled","reason":"Off"}`, w.Keyed(w.R3)...), http.StatusOK, "")
	w.Want(w.Do("robin", http.MethodPatch, terminalworld.HostPath(w.R2), `{"assigned_user_id":"`+w.Sam.String()+`","reason":"Sam"}`, w.Keyed(w.R2)...), http.StatusOK, "")
	if c := w.AccessChanges(seq); len(c) != 0 {
		t.Fatalf("state and holder changes wrote access.changed: %v", c)
	}
	for _, e := range w.Events("access.changed") {
		var p struct {
			PermissionVersion int64             `json:"permission_version"`
			Added             []json.RawMessage `json:"added"`
		}
		if err := json.Unmarshal([]byte(e.Raw), &p); err != nil || p.PermissionVersion == 0 {
			t.Errorf("payload %s: %v", e.Raw, err)
		}
	}
}

// TestConcurrency_Hosts (M2-39, Design note 15): two parallel createHost with one name give one
// 201 and one 409; two parallel updateHost that give one reported name to Alex and to Sam give
// one 200 and one 409 session_user_taken; two writes with the same If-Match give one success and
// one 412 version_conflict. Each race runs several times, so both orders occur.
func TestConcurrency_Hosts(t *testing.T) {
	t.Parallel()
	w := terminalworld.New(t)
	race := func(a, b func() *httpservertest.Response) [2]*httpservertest.Response {
		var out [2]*httpservertest.Response
		var wg sync.WaitGroup
		wg.Add(2)
		go func() { defer wg.Done(); out[0] = a() }()
		go func() { defer wg.Done(); out[1] = b() }()
		wg.Wait()
		return out
	}
	statuses := func(rs [2]*httpservertest.Response) map[int]int {
		m := map[int]int{}
		for _, r := range rs {
			m[r.StatusCode]++
		}
		return m
	}
	for i := range 3 {
		name := `{"name":"Twin ` + strconv.Itoa(i) + `","reason":"Race"}`
		rs := race(func() *httpservertest.Response {
			return w.Do("robin", http.MethodPost, terminalworld.HostsPath, name, terminalworld.Key()...)
		},
			func() *httpservertest.Response {
				return w.Do("robin", http.MethodPost, terminalworld.HostsPath, name, terminalworld.Key()...)
			})
		if s := statuses(rs); s[http.StatusCreated] != 1 || s[http.StatusConflict] != 1 {
			t.Fatalf("createHost race %d: %v", i, s)
		}
	}
	for i := range 3 {
		h := w.Host(terminaltest.Host{Name: "Shared " + strconv.Itoa(i), State: "active", ReportedSessionUsers: []string{"bothy-s-shared"}})
		body := func(u ids.ID) string {
			return `{"add_session_users":[{"user_id":"` + u.String() + `","os_user":"bothy-s-shared"}],"reason":"Race"}`
		}
		hdr := append(terminalworld.IfMatch(1), "Idempotency-Key", ids.New().String())
		hdr2 := append(terminalworld.IfMatch(1), "Idempotency-Key", ids.New().String())
		rs := race(func() *httpservertest.Response {
			return w.Do("robin", http.MethodPatch, terminalworld.HostPath(h), body(w.Alex), hdr...)
		},
			func() *httpservertest.Response {
				return w.Do("robin", http.MethodPatch, terminalworld.HostPath(h), body(w.Sam), hdr2...)
			})
		s := statuses(rs)
		if s[http.StatusOK] != 1 || s[http.StatusConflict]+s[http.StatusPreconditionFailed] != 1 {
			t.Fatalf("session user race %d: %v", i, s)
		}
		for _, r := range rs {
			if r.StatusCode == http.StatusConflict && r.Problem(t)["reason"] != "session_user_taken" {
				t.Fatalf("race %d: %s", i, r.Body)
			}
		}
		if n := w.Count(`SELECT count(*) FROM host_session_users WHERE host_id = $1`, h.String()); n != 1 {
			t.Fatalf("race %d stored %d rows", i, n)
		}
	}
	for i := range 3 {
		h := w.Host(terminaltest.Host{Name: "Version " + strconv.Itoa(i), State: "active"})
		rs := race(func() *httpservertest.Response {
			return w.Do("robin", http.MethodPatch, terminalworld.HostPath(h), `{"name":"V one `+strconv.Itoa(i)+`","reason":"Race"}`, append(terminalworld.IfMatch(1), terminalworld.Key()...)...)
		}, func() *httpservertest.Response {
			return w.Do("robin", http.MethodPatch, terminalworld.HostPath(h), `{"name":"V two `+strconv.Itoa(i)+`","reason":"Race"}`, append(terminalworld.IfMatch(1), terminalworld.Key()...)...)
		})
		if s := statuses(rs); s[http.StatusOK] != 1 || s[http.StatusPreconditionFailed] != 1 {
			t.Fatalf("If-Match race %d: %v", i, s)
		}
	}
}

// TestReadGrant_HostTarget (M2-39 red team, D-M2-39-6, 05 grant.read): an owner reads a host grant
// with getGrant and lists the grants of a host with listGrants; a non-owner gets 404 for the host
// grant and for the host filter, like for an unknown id.
func TestReadGrant_HostTarget(t *testing.T) {
	t.Parallel()
	w := terminalworld.New(t)
	g := w.Want(w.Do("robin", http.MethodGet, "/api/v1/grants/"+w.SamR2Grant.String(), ""), http.StatusOK, "")
	if g["id"] != w.SamR2Grant.String() || g["target_kind"] != "host" {
		t.Fatalf("owner getGrant %v", g)
	}
	l := w.Do("robin", http.MethodGet, "/api/v1/grants?target_kind=host&target_id="+w.R2.String(), "")
	if l.StatusCode != http.StatusOK || !strings.Contains(string(l.Body), w.SamR2Grant.String()) {
		t.Fatalf("owner listGrants on R2: %d %s", l.StatusCode, l.Body)
	}
	unknown := w.Do("alex", http.MethodGet, "/api/v1/grants/"+ids.New().String(), "")
	known := w.Do("alex", http.MethodGet, "/api/v1/grants/"+w.SamR2Grant.String(), "")
	if known.StatusCode != http.StatusNotFound || unknown.StatusCode != http.StatusNotFound {
		t.Fatalf("holder getGrant: %d, unknown %d", known.StatusCode, unknown.StatusCode)
	}
	if p := w.Want(w.Do("alex", http.MethodGet, "/api/v1/grants?target_kind=host&target_id="+w.R1.String(), ""), http.StatusNotFound, "not_found"); p["reason"] != nil {
		t.Fatalf("holder listGrants on R1: %v", p)
	}
}

// TestListHosts_ForgedCursor (M2-39 red team): the cursor carries no signature, so a client can
// build one with NUL in the name; it answers 400 cursor_invalid, never 500.
func TestListHosts_ForgedCursor(t *testing.T) {
	t.Parallel()
	w := terminalworld.New(t)
	key := struct {
		Name string `json:"n"`
		ID   string `json:"id"`
	}{Name: "a\x00b", ID: ids.New().String()}
	cur, err := httpserver.EncodeCursor("terminal.hosts", []byte(w.Sam.String()+"\x00hosts"), key)
	if err != nil {
		t.Fatal(err)
	}
	w.Want(w.Do("sam", http.MethodGet, terminalworld.HostsPath+"?cursor="+cur, ""), http.StatusBadRequest, "cursor_invalid")
}
