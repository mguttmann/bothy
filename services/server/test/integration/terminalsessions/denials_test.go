// SPDX-License-Identifier: Apache-2.0

package terminalsessions

import (
	"net/http"
	"slices"
	"sync"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/test/harness"
)

// nothingOpened checks that no session, no session.open and no consumed step-up of c exist.
func (w *world) nothingOpened(c *harness.Client) {
	w.t.Helper()
	if n := w.count(`SELECT count(*) FROM terminal_sessions WHERE user_id = $1`, c.PrincipalID); n != 0 {
		w.t.Fatalf("%d sessions of the caller", n)
	}
	if n := len(w.h1.Opens()); n != 0 {
		w.t.Fatalf("%d session.open", n)
	}
	if c.SessionID != "" && w.count(`SELECT count(*) FROM auth_sessions WHERE id = $1 AND step_up_consumed_at IS NOT NULL`, c.SessionID) != 0 {
		w.t.Fatal("the step-up was consumed")
	}
}

// expect checks status, code and reason of r.
func expect(t *testing.T, what string, r harness.Response, status int, code, reason string) {
	t.Helper()
	c, rs := problem(r)
	if r.Status != status || c != code || rs != reason {
		t.Fatalf("%s: %d %s %s, want %d %s %s: %s", what, r.Status, c, rs, status, code, reason, r.Body)
	}
}

// TestOpen_TMT29g_WorkdirIndexOutsideOwnList (TM-T29 (g) server part, TestOpen_ClaudeWdi): an
// index outside the own list, an index valid only in Robin's list, claude without index, shell with
// an index and an unreported profile each give 403 terminal_denied profile_unknown with a deny
// entry and nothing opened; valid indexes stand as wdi.
func TestOpen_TMT29g_WorkdirIndexOutsideOwnList(t *testing.T) {
	w := newWorld(t, nil)
	for name, body := range map[string]map[string]any{
		"index 2":        openBody("claude", 2),
		"no index":       openBody("claude"),
		"shell index 0":  openBody("shell", 0),
		"unknown":        openBody("missing-profile"),
		"negative index": openBody("claude", -1),
	} {
		r := w.open(w.alex, w.r1, body)
		if name == "negative index" {
			expect(t, name, r, http.StatusBadRequest, "validation_failed", "")
			continue
		}
		expect(t, name, r, http.StatusForbidden, "terminal_denied", "profile_unknown")
	}
	w.nothingOpened(w.alex)
	w.waitDenial("terminal.open", "terminal_profile_unknown")
	// Alex's own list empty, Robin's keeps Project C: index 0 is no directory of Alex.
	if err := w.h1.Send("report", report(reportedUsers([]string{}...), false)); err != nil {
		t.Fatal(err)
	}
	w.eventually("the report", func() bool {
		return w.count(`SELECT count(*) FROM hosts h, jsonb_array_elements(h.reported_session_users) e
			WHERE h.id = $1 AND e->>'os_user' = $2 AND e->'workdirs' = '[]'::jsonb`, w.r1.String(), "bothy-s-alex") == 1
	})
	expect(t, "index of Robin's list", w.open(w.alex, w.r1, openBody("claude", 0)), http.StatusForbidden, "terminal_denied", "profile_unknown")
	w.nothingOpened(w.alex)
}

// TestOpen_ClaudeWdi: Alex opens claude with index 0 and 1; the tokens carry wdi 0 and 1.
func TestOpen_ClaudeWdi(t *testing.T) {
	w := newWorld(t, nil)
	for _, i := range []int{0, 1} {
		g := w.opened(w.alex, w.r1, openBody("claude", i))
		_, c, order := tokenClaims(t, g.SessionToken)
		if c["wdi"] != float64(i) || !slices.Contains(order, "wdi") {
			t.Fatalf("index %d: claims %v", i, c)
		}
	}
}

// TestOpen_NoSessionUserIsDeniedNot4104 (TM-T28 (e) server part): a report without bothy-s-alex
// makes the opening 403 terminal_denied no_session_user; no row, no token, no session.open.
func TestOpen_NoSessionUserIsDeniedNot4104(t *testing.T) {
	w := newWorld(t, nil)
	users := []any{
		map[string]any{"os_user": "bothy-s-robin", "workdirs": []string{"Project C"}},
		map[string]any{"os_user": "bothy-s-sam", "workdirs": []string{}},
	}
	if err := w.h1.Send("report", report(users, false)); err != nil {
		t.Fatal(err)
	}
	w.eventually("the report", func() bool {
		return w.count(`SELECT count(*) FROM hosts WHERE id = $1 AND NOT reported_session_users @> jsonb_build_array(jsonb_build_object('os_user', $2::text))`,
			w.r1.String(), "bothy-s-alex") == 1
	})
	expect(t, "no session user", w.open(w.alex, w.r1, openBody("shell")), http.StatusForbidden, "terminal_denied", "no_session_user")
	w.nothingOpened(w.alex)
}

// TestOpen_TMT07_StepUp (TM-T07 without the console part): no step-up, a step-up older than
// terminal.step_up_max_age and a step-up consumed by a successful opening each give 401
// step_up_required with a deny entry; of two parallel openings with one step-up exactly one wins.
func TestOpen_TMT07_StepUp(t *testing.T) {
	w := newWorld(t, nil)
	expect(t, "no step-up", w.openRaw(w.alex, w.r1, openBody("shell")), http.StatusUnauthorized, "step_up_required", "")
	if err := w.alex.StepUp(t.Context()); err != nil {
		t.Fatal(err)
	}
	w.exec(`UPDATE auth_sessions SET step_up_at = now() - interval '2 minutes' WHERE id = $1`, w.alex.SessionID)
	expect(t, "old step-up", w.openRaw(w.alex, w.r1, openBody("shell")), http.StatusUnauthorized, "step_up_required", "")
	w.opened(w.alex, w.r1, openBody("shell"))
	expect(t, "consumed step-up", w.openRaw(w.alex, w.r1, openBody("shell")), http.StatusUnauthorized, "step_up_required", "")
	w.waitDenial("terminal.open", "terminal_step_up_not_fresh")
	// Two parallel openings with one fresh step-up.
	if err := w.alex.StepUp(t.Context()); err != nil {
		t.Fatal(err)
	}
	before := w.count(`SELECT count(*) FROM terminal_sessions`)
	var wg sync.WaitGroup
	statuses := make([]int, 2)
	for i := range statuses {
		wg.Add(1)
		go func() {
			defer wg.Done()
			statuses[i] = w.openRaw(w.alex, w.r1, openBody("shell")).Status
		}()
	}
	wg.Wait()
	slices.Sort(statuses)
	if statuses[0] != http.StatusCreated || statuses[1] != http.StatusUnauthorized {
		t.Fatalf("parallel openings %v", statuses)
	}
	if n := w.count(`SELECT count(*) FROM terminal_sessions`) - before; n != 1 {
		t.Fatalf("%d new sessions", n)
	}
}

// TestOpen_DeniedOpensNothing (AZ-M60 over HTTP, TM-T17 for terminal.open): Sam is not the
// assignee; an agent token gets the gate; Sam on R2 with only the instance grant gets the same 404
// as an unknown host; H1 disconnected gives 409 host_unavailable; nothing is opened.
func TestOpen_DeniedOpensNothing(t *testing.T) {
	w := newWorld(t, nil)
	expect(t, "Sam on R1", w.open(w.sam, w.r1, openBody("shell")), http.StatusForbidden, "terminal_denied", "not_host_assignee")
	w.nothingOpened(w.sam)
	agent := w.openRaw(w.lotse, w.r1, openBody("shell"))
	expect(t, "agent", agent, http.StatusForbidden, "forbidden", "subject_kind_not_allowed")
	expect(t, "agent, random host", w.openRaw(w.lotse, ids.New(), openBody("shell")), http.StatusForbidden, "forbidden", "subject_kind_not_allowed")
	r2 := w.open(w.sam, w.r2, openBody("shell"))
	unknown := w.open(w.sam, ids.New(), openBody("shell"))
	expect(t, "Sam on R2", r2, http.StatusNotFound, "not_found", "")
	expect(t, "unknown host", unknown, http.StatusNotFound, "not_found", "")
	w.h1.Close()
	w.eventually("H1 disconnected", func() bool {
		r := w.must(w.robin, http.MethodGet, "/hosts/"+w.r1.String(), nil, http.StatusOK)
		var h struct{ Connected bool }
		r.JSON(t, &h)
		return !h.Connected
	})
	expect(t, "H1 disconnected", w.open(w.alex, w.r1, openBody("shell")), http.StatusConflict, "host_unavailable", "")
	w.nothingOpened(w.alex)
}

// TestToken_OsuFromAssignment: a body with an extra member osu is 400 validation_failed.
func TestToken_OsuFromAssignment(t *testing.T) {
	w := newWorld(t, nil)
	b := openBody("shell")
	b["osu"] = "bothy-s-robin"
	expect(t, "osu member", w.open(w.alex, w.r1, b), http.StatusBadRequest, "validation_failed", "")
	b = openBody("shell")
	b["client_ephemeral_key_hash"] = "not-a-hash"
	expect(t, "cek pattern", w.open(w.alex, w.r1, b), http.StatusBadRequest, "validation_failed", "")
	w.nothingOpened(w.alex)
}
