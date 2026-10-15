// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"net/http"
	"slices"
	"strconv"
	"testing"

	"github.com/OWNER/bothy/services/server/api/routes"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/test/terminaltest"
	"github.com/OWNER/bothy/services/server/test/terminalworld"
)

// TestHostRead_Visibility (M2-39, 07 section 2.12 point 5, PLAN row host.read): Robin lists every
// host in the order lower(name), id, Alex only R1, Sam R2 and R3, and after the expiry of the
// grant on R2 only R3; getHost the same; every other combination 404; ETag equals the version;
// a cursor round trip with limit 1 and a foreign cursor 400 cursor_invalid.
func TestHostRead_Visibility(t *testing.T) {
	t.Parallel()
	w := terminalworld.New(t)
	extra := w.Host(terminaltest.Host{Name: "a lab rack", State: "pairing"})
	retired := w.Host(terminaltest.Host{Name: "Zz old", State: "retired"})
	// lower(name): "a lab rack" < "build server" < "lab machine" < "spare machine" < "zz old".
	if got, want := w.ListIDs("robin", 1), []ids.ID{extra, w.R1, w.R2, w.R3, retired}; !slices.Equal(got, want) {
		t.Fatalf("Robin lists %v, want %v", got, want)
	}
	if got := w.ListIDs("alex", 2); !slices.Equal(got, []ids.ID{w.R1}) {
		t.Fatalf("Alex lists %v", got)
	}
	if got := w.ListIDs("sam", 3); !slices.Equal(got, []ids.ID{w.R2, w.R3}) {
		t.Fatalf("Sam lists %v", got)
	}
	for _, c := range []struct {
		name string
		host ids.ID
		ok   bool
	}{{"robin", w.R1, true}, {"robin", retired, true}, {"alex", w.R1, true}, {"alex", w.R2, false}, {"alex", w.R3, false},
		{"sam", w.R1, false}, {"sam", w.R2, true}, {"sam", w.R3, true}, {"sam", extra, false}} {
		r := w.Do(c.name, http.MethodGet, terminalworld.HostPath(c.host), "")
		if !c.ok {
			w.Want(r, http.StatusNotFound, "not_found")
			continue
		}
		m := w.Want(r, http.StatusOK, "")
		if r.Header.Get("ETag") != `"`+strconv.FormatInt(int64(m["version"].(float64)), 10)+`"` {
			t.Errorf("%s on %s: ETag %q, version %v", c.name, c.host, r.Header.Get("ETag"), m["version"])
		}
	}
	m := w.Want(w.Do("robin", http.MethodGet, terminalworld.HostPath(w.R1), ""), http.StatusOK, "")
	if m["noise_public_key"] == nil || m["key_fingerprint"] != terminaltest.KeyFingerprint || m["assigned_user_id"] != w.Alex.String() {
		t.Errorf("R1 as Robin %v", m)
	}
	if _, ok := m["reported_session_users"]; !ok {
		t.Error("a paired host lists its reported session users")
	}
	if m := w.Want(w.Do("robin", http.MethodGet, terminalworld.HostPath(extra), ""), http.StatusOK, ""); m["reported_session_users"] != nil {
		t.Error("a host in pairing shows reported session users")
	}
	// A cursor of Robin is not a cursor of Sam.
	first := w.Want(w.Do("robin", http.MethodGet, terminalworld.HostsPath+"?limit=1", ""), http.StatusOK, "")
	w.Want(w.Do("sam", http.MethodGet, terminalworld.HostsPath+"?cursor="+first["next_cursor"].(string), ""), http.StatusBadRequest, "cursor_invalid")
	w.Clk.Set(w.SamR2Expiry)
	if got := w.ListIDs("sam", 3); !slices.Equal(got, []ids.ID{w.R3}) {
		t.Fatalf("Sam lists %v after the expiry of the grant on R2", got)
	}
	w.Want(w.Do("sam", http.MethodGet, terminalworld.HostPath(w.R2), ""), http.StatusNotFound, "not_found")
}

// TestAZM60_InstanceGrantGivesNoHost (AZ-M60, R-AZ-24): Robin grants Alex trigger with every
// capability on the instance; getHost(R3) as Alex is 404 identical to a random id, listHosts lacks
// R3, R1 stays visible as Alex's own host.
func TestAZM60_InstanceGrantGivesNoHost(t *testing.T) {
	t.Parallel()
	w := terminalworld.New(t)
	w.Want(w.Do("robin", http.MethodPost, "/api/v1/grants", `{"subject_kind":"user","subject_id":"`+w.Alex.String()+
		`","target_kind":"instance","level":"trigger","capabilities":["approve_actions","upload","export","manage"],"reason":"Everything"}`, terminalworld.Key()...),
		http.StatusCreated, "")
	a := w.Do("alex", http.MethodGet, terminalworld.HostPath(w.R3), "")
	b := w.Do("alex", http.MethodGet, terminalworld.HostPath(ids.New()), "")
	terminalworld.SameAnswer(t, a, b)
	if got := w.ListIDs("alex", 3); !slices.Equal(got, []ids.ID{w.R1}) {
		t.Fatalf("Alex lists %v", got)
	}
}

// TestAZP4_HostNotFoundEquality (AZ-P4): for getHost, updateHost and setHostState, Sam on R1 and
// Sam on a random id get the same status, body and headers.
func TestAZP4_HostNotFoundEquality(t *testing.T) {
	t.Parallel()
	w := terminalworld.New(t)
	random := ids.New()
	for _, c := range []struct{ method, suffix, body string }{
		{http.MethodGet, "", ""},
		{http.MethodPatch, "", `{"name":"Mine","reason":"x"}`},
		{http.MethodPost, "/state", `{"state":"disabled","reason":"x"}`},
	} {
		a := w.Do("sam", c.method, terminalworld.HostPath(w.R1)+c.suffix, c.body, append(terminalworld.IfMatch(1), terminalworld.Key()...)...)
		b := w.Do("sam", c.method, terminalworld.HostPath(random)+c.suffix, c.body, append(terminalworld.IfMatch(1), terminalworld.Key()...)...)
		if a.StatusCode != http.StatusNotFound {
			t.Fatalf("%s %s: %d %s", c.method, c.suffix, a.StatusCode, a.Body)
		}
		terminalworld.SameAnswer(t, a, b)
	}
}

// TestHostRequirement_HTTP (M2-39, 05 section 4.14, TM-T17 server part): an agent token, an app
// token and Alex's personal token get 403 forbidden with reason subject_kind_not_allowed on all
// five operations, for R1 and a random id alike; the denials are audited.
func TestHostRequirement_HTTP(t *testing.T) {
	t.Parallel()
	w := terminalworld.New(t)
	w.AgentToken("lotse")
	w.AppToken("tracker")
	w.PersonalToken("alex-pat", w.Alex)
	for _, name := range []string{"lotse", "tracker", "alex-pat"} {
		for _, target := range []ids.ID{w.R1, ids.New()} {
			for _, c := range []struct{ method, path, body string }{
				{http.MethodGet, terminalworld.HostsPath, ""},
				{http.MethodPost, terminalworld.HostsPath, `{"name":"Mine","reason":"x"}`},
				{http.MethodGet, terminalworld.HostPath(target), ""},
				{http.MethodPatch, terminalworld.HostPath(target), `{"name":"Mine","reason":"x"}`},
				{http.MethodPost, terminalworld.HostPath(target) + "/state", `{"state":"disabled","reason":"x"}`},
			} {
				p := w.Want(w.Do(name, c.method, c.path, c.body, append(terminalworld.IfMatch(1), terminalworld.Key()...)...), http.StatusForbidden, "forbidden")
				if p["reason"] != "subject_kind_not_allowed" {
					t.Errorf("%s %s %s: %v", name, c.method, c.path, p)
				}
			}
		}
	}
	var denies int
	for _, act := range []string{"host.read", "host.manage"} {
		for _, r := range w.Audits(act) {
			if r.Decision == "deny" && r.Reason == "subject_kind_not_allowed" {
				denies++
			}
		}
	}
	if denies == 0 {
		t.Fatal("the refusals were not audited")
	}
}

// TestAZP1_HostRoutes (M2-39, AZ-P1) and TestPairingRoutes_AZP1 (M2-41): the host operations are
// registered on the API router with host.read, host.manage and host.pair and their rate-limit
// classes; the server opens no other route of the module (24 section 1: no inbound path; pairHost
// is called outbound by the host service on the normal API listener).
func TestAZP1_HostRoutes(t *testing.T) {
	t.Parallel()
	want := map[string][3]string{
		"listHosts":            {"GET /api/v1/hosts", "host.read", "rl.read"},
		"createHost":           {"POST /api/v1/hosts", "host.manage", "rl.admin"},
		"getHost":              {"GET /api/v1/hosts/{host_id}", "host.read", "rl.read"},
		"updateHost":           {"PATCH /api/v1/hosts/{host_id}", "host.manage", "rl.admin"},
		"setHostState":         {"POST /api/v1/hosts/{host_id}/state", "host.manage", "rl.admin"},
		"issueHostPairingCode": {"POST /api/v1/hosts/{host_id}/pairing-code", "host.manage", "rl.admin"},
		"pairHost":             {"POST /api/v1/hosts/pair", "host.pair", "rl.public_auth"},
		"confirmHost":          {"POST /api/v1/hosts/{host_id}/confirm", "host.manage", "rl.admin"},
	}
	var hostRoutes int
	for _, op := range routes.Enabled() {
		if w, ok := want[op.ID]; ok {
			hostRoutes++
			if op.Pattern != w[0] || op.Action != w[1] || op.RateLimitClass != w[2] {
				t.Errorf("%s: %s %s %s", op.ID, op.Pattern, op.Action, op.RateLimitClass)
			}
			continue
		}
		if op.Action == "host.pair" || op.Action == "host.connect" || len(op.Action) > 9 && op.Action[:9] == "terminal." {
			// pairHost is in want; any other route with these actions belongs to a later ticket.
			t.Errorf("operation %s of a later ticket is enabled", op.ID)
		}
	}
	if hostRoutes != len(want) {
		t.Fatalf("%d host routes, want %d", hostRoutes, len(want))
	}
}
