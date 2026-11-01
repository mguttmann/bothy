// SPDX-License-Identifier: Apache-2.0

package terminalsessions

import (
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
	"github.com/OWNER/bothy/services/server/test/harness"
	"github.com/OWNER/bothy/services/server/test/terminal/fakehost"
)

// closePath is the path of closeTerminalSession.
func closePath(session string) string { return "/terminal/sessions/" + session + "/close" }

// TestClose_EndsConnectionsAndTellsHost (T-TM-11) with TestAudit_TerminalEntries and
// TestEvents_TerminalSessionChanged: Alex closes her active session with a reason: 200 closed
// client_closed, closed_by Alex, device 4100 client_closed, H1 data 4100, H1 gets session.close;
// audit terminal.close with the reason; Robin closes another session of Alex: closed_by_owner and
// foreign_assignee true in his opening; every event goes only to Alex and holds no token or name.
func TestClose_EndsConnectionsAndTellsHost(t *testing.T) {
	t.Parallel()
	w := newWorld(t, nil)
	ctx := w.ctx()
	g, d := w.started(w.alex)
	sid := ids.MustParse(g.Session.ID)
	h := w.h1.Data(sid)
	r := w.must(w.alex, http.MethodPost, closePath(g.Session.ID), map[string]any{"reason": "done for today"}, http.StatusOK)
	var body map[string]any
	r.JSON(t, &body)
	if body["state"] != "closed" || body["close_reason"] != "client_closed" {
		t.Fatalf("answer %v", body)
	}
	w.closeCode("device", func() (int, string) { return d.CloseCode(ctx) }, 4100, "client_closed")
	w.closeCode("host", func() (int, string) { return h.CloseCode(ctx) }, 4100, "client_closed")
	if reason, err := w.h1.WaitClose(ctx, sid); err != nil || reason != "client_closed" {
		t.Fatalf("session.close %s %v", reason, err)
	}
	if got := w.text(`SELECT closed_by::text FROM terminal_sessions WHERE id = $1`, g.Session.ID); got != w.alex.PrincipalID {
		t.Fatalf("closed_by %s", got)
	}
	entries := w.audits("terminal.close")
	if len(entries) != 1 || entries[0]["reason"] != "done for today" || entries[0]["session_id"] != g.Session.ID {
		t.Fatalf("terminal.close entries %v", entries)
	}
	// Robin, an owner, opens on R1 (assigned to Alex) and closes a session of Alex.
	w.opened(w.robin, w.r1, openBody("shell"))
	opens := w.audits("terminal.open")
	foreign := map[bool]int{}
	for _, e := range opens {
		foreign[e["foreign_assignee"] == true]++
		if _, ok := e["pin_changed"]; !ok {
			t.Fatalf("terminal.open without pin_changed: %v", e)
		}
	}
	if foreign[true] != 1 {
		t.Fatalf("foreign_assignee %v", foreign)
	}
	g2 := w.opened(w.alex, w.r1, openBody("shell"))
	w.must(w.robin, http.MethodPost, closePath(g2.Session.ID), map[string]any{"reason": "owner check"}, http.StatusOK)
	if got := w.state(g2.Session.ID); got != "closed:closed_by_owner" {
		t.Fatalf("state %s", got)
	}
	rows, err := w.s.AdminDB().Query(ctx, `SELECT coalesce(audience_principal_id::text, ''), payload::text FROM event_log WHERE type = 'terminal_session.changed'`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var aud, payload string
		if err := rows.Scan(&aud, &payload); err != nil {
			t.Fatal(err)
		}
		if testkit.HasJSONNull([]byte(payload)) || strings.Contains(payload, "bothy-s-") || strings.Contains(payload, g.SessionToken) {
			t.Fatalf("payload %s", payload)
		}
		var p struct {
			UserID string `json:"user_id"`
		}
		_ = json.Unmarshal([]byte(payload), &p)
		if aud != p.UserID {
			t.Fatalf("event of %s to %s", p.UserID, aud)
		}
	}
}

// TestClose_RightsAndRepeats (AZ-P4, TM-T17): Sam on Alex's session gets the 404 of an unknown id;
// Lotse 403 forbidden; an already closed session 200 without a new version and with no_change; a
// reason above audit.reason_max_chars 400 too_long.
func TestClose_RightsAndRepeats(t *testing.T) {
	t.Parallel()
	w := newWorld(t, nil)
	w.h1.Default(fakehost.Script{Kind: fakehost.Ignore})
	g := w.opened(w.alex, w.r1, openBody("shell"))
	sam := w.sam.Do(t.Context(), harness.Request{Method: http.MethodPost, Path: closePath(g.Session.ID), Body: map[string]any{"reason": "x"}})
	unknown := w.sam.Do(t.Context(), harness.Request{Method: http.MethodPost, Path: closePath(ids.New().String()), Body: map[string]any{"reason": "x"}})
	expect(t, "Sam", sam, http.StatusNotFound, "not_found", "")
	expect(t, "unknown", unknown, http.StatusNotFound, "not_found", "")
	for _, op := range []harness.Request{
		{Method: http.MethodPost, Path: closePath(g.Session.ID), Body: map[string]any{"reason": "x"}},
		{Method: http.MethodPost, Path: "/terminal/sessions/" + g.Session.ID + "/attach", Body: map[string]any{"cols": 1, "rows": 1, "client_ephemeral_key_hash": cek}},
		{Method: http.MethodGet, Path: "/terminal/sessions/" + g.Session.ID},
		{Method: http.MethodGet, Path: "/terminal/sessions"},
	} {
		expect(t, "Lotse "+op.Path, w.lotse.Do(t.Context(), op), http.StatusForbidden, "forbidden", "subject_kind_not_allowed")
	}
	long := w.alex.Do(t.Context(), harness.Request{Method: http.MethodPost, Path: closePath(g.Session.ID),
		Body: map[string]any{"reason": strings.Repeat("x", 10001)}})
	expect(t, "too long", long, http.StatusBadRequest, "validation_failed", "")
	w.must(w.alex, http.MethodPost, closePath(g.Session.ID), map[string]any{"reason": "done"}, http.StatusOK)
	version := w.count(`SELECT version::int FROM terminal_sessions WHERE id = $1`, g.Session.ID)
	w.must(w.alex, http.MethodPost, closePath(g.Session.ID), map[string]any{"reason": "again"}, http.StatusOK)
	if w.count(`SELECT version::int FROM terminal_sessions WHERE id = $1`, g.Session.ID) != version {
		t.Fatal("a close without effect raised the version")
	}
	noChange := 0
	for _, e := range w.audits("terminal.close") {
		if e["no_change"] == true {
			noChange++
		}
	}
	if noChange != 1 {
		t.Fatalf("%d no_change entries", noChange)
	}
}

// TestListAndGet (Design note 17): Alex lists only her sessions, Robin all; the filters host_id
// and state; pages without gap or duplicate; Sam's get of Alex's session equals an unknown id.
func TestListAndGet(t *testing.T) {
	t.Parallel()
	w := newWorld(t, nil)
	w.h1.Default(fakehost.Script{Kind: fakehost.Ignore})
	var alex []string
	for range 3 {
		alex = append(alex, w.opened(w.alex, w.r1, openBody("shell")).Session.ID)
	}
	robin := w.opened(w.robin, w.r1, openBody("shell")).Session.ID
	w.must(w.alex, http.MethodPost, closePath(alex[0]), map[string]any{"reason": "done"}, http.StatusOK)
	list := func(c *harness.Client, query string) []string {
		var out []string
		path := "/terminal/sessions?limit=2" + query
		for {
			var p struct {
				Items []struct {
					ID string `json:"id"`
				} `json:"items"`
				NextCursor string `json:"next_cursor"`
			}
			w.must(c, http.MethodGet, path, nil, http.StatusOK).JSON(t, &p)
			for _, it := range p.Items {
				out = append(out, it.ID)
			}
			if p.NextCursor == "" {
				return out
			}
			path = "/terminal/sessions?limit=2" + query + "&cursor=" + p.NextCursor
		}
	}
	got := list(w.alex, "")
	want := slices.Clone(alex)
	slices.Sort(want)
	slices.Reverse(want)
	if !slices.Equal(got, want) {
		t.Fatalf("Alex lists %v, want %v", got, want)
	}
	all := list(w.robin, "&host_id="+w.r1.String())
	if len(all) != 4 || !slices.Contains(all, robin) {
		t.Fatalf("Robin lists %v", all)
	}
	if closed := list(w.robin, "&state=closed"); len(closed) != 1 || closed[0] != alex[0] {
		t.Fatalf("closed %v", closed)
	}
	sam := w.sam.Do(t.Context(), harness.Request{Method: http.MethodGet, Path: "/terminal/sessions/" + alex[1]})
	unknown := w.sam.Do(t.Context(), harness.Request{Method: http.MethodGet, Path: "/terminal/sessions/" + ids.New().String()})
	expect(t, "Sam get", sam, http.StatusNotFound, "not_found", "")
	expect(t, "unknown get", unknown, http.StatusNotFound, "not_found", "")
	w.must(w.alex, http.MethodGet, "/terminal/sessions/"+alex[1], nil, http.StatusOK)
}
