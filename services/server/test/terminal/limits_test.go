// SPDX-License-Identifier: Apache-2.0

package terminal_test

import (
	"encoding/json"
	"net/http"
	gosync "sync"
	"testing"

	"github.com/OWNER/bothy/services/server/test/harness"
)

// limCfg sets terminal.max_sessions_per_person and terminal.max_sessions_per_host.
func limCfg(perPerson, perHost string) worldOptions {
	return worldOptions{cfg: map[string]string{"terminal.max_sessions_per_person": perPerson, "terminal.max_sessions_per_host": perHost}}
}

// limProblem returns code and reason of a problem body.
func limProblem(r harness.Response) (code, reason string) {
	var p struct{ Code, Reason string }
	_ = json.Unmarshal(r.Body, &p)
	return p.Code, p.Reason
}

// limDenied fails unless r is 403 terminal_denied with reason session_limit.
func limDenied(t *testing.T, what string, r harness.Response) {
	t.Helper()
	if c, rs := limProblem(r); r.Status != http.StatusForbidden || c != "terminal_denied" || rs != "session_limit" {
		t.Fatalf("%s: %d %s %s, want 403 terminal_denied session_limit: %s", what, r.Status, c, rs, r.Body)
	}
}

// limCreated fails unless r is 201 and returns the session id of the grant.
func limCreated(t *testing.T, what string, r harness.Response) string {
	t.Helper()
	if r.Status != http.StatusCreated {
		t.Fatalf("%s: %d %s, want 201", what, r.Status, r.Body)
	}
	var g grantBody
	r.JSON(t, &g)
	return g.Session.ID
}

// limDenials waits for the aggregated deny entries of terminal.open with terminal_session_limit
// (audit.deny_aggregate_window) and checks there are want of them.
func limDenials(w *world, want int) {
	w.t.Helper()
	q := `SELECT count(*) FROM audit_events WHERE action = 'terminal.open' AND decision = 'deny' AND deny_reason = 'terminal_session_limit'`
	w.eventually("the deny entry terminal_session_limit", func() bool { return w.count(q) >= want })
	if n := w.count(q); n != want {
		w.t.Fatalf("%d deny entries terminal_session_limit, want %d", n, want)
	}
}

// limOpenOf returns the number of sessions of the person that are not closed.
func limOpenOf(w *world, person string) int {
	return w.count(`SELECT count(*) FROM terminal_sessions WHERE user_id = $1 AND state <> 'closed'`, person)
}

// limOpen sends openTerminalSession as c on host with the step-up c holds (no new one).
func limOpen(w *world, c *harness.Client, host string) harness.Response {
	w.t.Helper()
	return c.Do(w.t.Context(), harness.Request{Method: http.MethodPost, Path: "/hosts/" + host + "/terminal/sessions", Body: openBody()})
}

// limClose ends the session through closeTerminalSession as c.
func limClose(w *world, c *harness.Client, session string) {
	w.t.Helper()
	w.must(c, http.MethodPost, "/terminal/sessions/"+session+"/close", map[string]any{"reason": "terminal test"}, http.StatusOK)
}

// TestSessionLimit_PerPerson (24 section 6.3, Design note 9, 07 4.2): with
// terminal.max_sessions_per_person set low, Robin opens up to the limit on r1; the next open on
// another host (r2) gives 403 terminal_denied with reason session_limit, one deny entry with
// terminal_session_limit and no session; another person is not limited by Robin's sessions.
func TestSessionLimit_PerPerson(t *testing.T) {
	t.Parallel()
	w := newWorld(t, limCfg("2", "10"))
	limCreated(t, "first", w.openRaw(w.robin, w.r1))
	limCreated(t, "second", w.openRaw(w.robin, w.r1))
	limDenied(t, "third on r2", w.openRaw(w.robin, w.r2))
	limDenials(w, 1)
	if n := limOpenOf(w, w.robin.PrincipalID); n != 2 {
		t.Fatalf("%d open sessions of Robin, want 2", n)
	}
	limCreated(t, "Alex on r1", w.openRaw(w.alex, w.r1))
}

// TestSessionLimit_PerHost (24 section 6.3, Design note 9): with terminal.max_sessions_per_host set
// low, Alex and Robin together reach the limit on r1; the next open on r1 gives session_limit; r2
// is unaffected (Sam and Robin open there).
func TestSessionLimit_PerHost(t *testing.T) {
	t.Parallel()
	w := newWorld(t, limCfg("10", "2"))
	limCreated(t, "Alex on r1", w.openRaw(w.alex, w.r1))
	limCreated(t, "Robin on r1", w.openRaw(w.robin, w.r1))
	limDenied(t, "Alex again on r1", w.openRaw(w.alex, w.r1))
	limDenials(w, 1)
	limCreated(t, "Sam on r2", w.openRaw(w.sam, w.r2))
	limCreated(t, "Robin on r2", w.openRaw(w.robin, w.r2))
	if n := w.count(`SELECT count(*) FROM terminal_sessions WHERE host_id = $1 AND state <> 'closed'`, w.r1); n != 2 {
		t.Fatalf("%d open sessions on r1, want 2", n)
	}
}

// TestSessionLimit_ClosedNotCounted (Design note 9: only state <> 'closed' counts): at the limit a
// session closed through closeTerminalSession frees a place for the next open.
func TestSessionLimit_ClosedNotCounted(t *testing.T) {
	t.Parallel()
	w := newWorld(t, limCfg("2", "2"))
	first := limCreated(t, "first", w.openRaw(w.robin, w.r1))
	limCreated(t, "second", w.openRaw(w.robin, w.r2))
	limDenied(t, "third", w.openRaw(w.robin, w.r1))
	limClose(w, w.robin, first)
	w.waitState(first, "closed:client_closed")
	limCreated(t, "after the close", w.openRaw(w.robin, w.r1))
}

// TestSessionLimit_AttachNotCounted (Design note 9: attaching creates no session): at the limit
// Robin attaches to an own open session; a new open is still denied and the count stays.
func TestSessionLimit_AttachNotCounted(t *testing.T) {
	t.Parallel()
	w := newWorld(t, limCfg("2", "10"))
	first := limCreated(t, "first", w.openRaw(w.robin, w.r1))
	limCreated(t, "second", w.openRaw(w.robin, w.r2))
	if r := w.attach(w.robin, first, true); r.Status != http.StatusOK {
		t.Fatalf("attach at the limit: %d %s", r.Status, r.Body)
	}
	limDenied(t, "open after the attach", w.openRaw(w.robin, w.r1))
	if n := limOpenOf(w, w.robin.PrincipalID); n != 2 {
		t.Fatalf("%d open sessions of Robin, want 2", n)
	}
}

// TestSessionLimit_DenialKeepsStepUp (Design note 9: a denial rolls back, the step-up stays
// unconsumed): Robin's second device holds the limit; Robin's first device steps up once and is
// denied; after the second device closes one of its sessions the SAME step-up (no new one) opens
// within terminal.step_up_max_age, and only that open consumes it.
func TestSessionLimit_DenialKeepsStepUp(t *testing.T) {
	t.Parallel()
	w := newWorld(t, limCfg("2", "10"))
	other, err := w.s.UserSession(w.t.Context(), w.robin.PrincipalID)
	if err != nil {
		t.Fatal(err)
	}
	held := limCreated(t, "first of the second device", w.openRaw(other, w.r1))
	limCreated(t, "second of the second device", w.openRaw(other, w.r2))

	w.syncClock()
	if err := w.robin.StepUp(w.t.Context()); err != nil {
		t.Fatal(err)
	}
	consumed := func() string {
		return w.text(`SELECT coalesce(step_up_consumed_at::text, '') FROM auth_sessions WHERE id = $1`, w.robin.SessionID)
	}
	before := consumed()
	limDenied(t, "open at the limit", limOpen(w, w.robin, w.r1.String()))
	if got := consumed(); got != before {
		t.Fatalf("the denial consumed the step-up: %q, was %q", got, before)
	}
	limClose(w, other, held)
	w.waitState(held, "closed:client_closed")
	limCreated(t, "open with the same step-up", limOpen(w, w.robin, w.r1.String()))
	if got := consumed(); got == before {
		t.Fatal("the open did not consume the step-up")
	}
}

// TestSessionLimit_Concurrent (Design notes 9 and 15: the person lock): Robin opens in parallel
// from more devices than terminal.max_sessions_per_person, each with its own fresh step-up, on r1
// and r2: exactly the limit succeeds, every other answer is 403 session_limit (or 503 unavailable
// when the retries of a moved count ran out), and the database never holds more than the limit.
func TestSessionLimit_Concurrent(t *testing.T) {
	t.Parallel()
	const limit, devices = 2, 6
	w := newWorld(t, limCfg("2", "10"))
	clients := make([]*harness.Client, devices)
	for i := range clients {
		c, err := w.s.UserSession(w.t.Context(), w.robin.PrincipalID)
		if err != nil {
			t.Fatal(err)
		}
		clients[i] = c
	}
	w.syncClock()
	for _, c := range clients {
		if err := c.StepUp(w.t.Context()); err != nil {
			t.Fatal(err)
		}
	}
	hosts := []string{w.r1.String(), w.r2.String()}
	results := make([]harness.Response, devices)
	var wg gosync.WaitGroup
	start := make(chan struct{})
	for i, c := range clients {
		wg.Go(func() {
			<-start
			results[i] = c.Do(w.t.Context(), harness.Request{Method: http.MethodPost, Path: "/hosts/" + hosts[i%2] + "/terminal/sessions",
				Body: openBody()})
		})
	}
	close(start)
	wg.Wait()
	created := 0
	for i, r := range results {
		code, reason := limProblem(r)
		switch {
		case r.Status == http.StatusCreated:
			created++
		case r.Status == http.StatusForbidden && code == "terminal_denied" && reason == "session_limit":
		case r.Status == http.StatusServiceUnavailable && code == "unavailable":
		default:
			t.Errorf("device %d: %d %s", i, r.Status, r.Body)
		}
	}
	if created != limit {
		t.Fatalf("%d opens succeeded, want %d", created, limit)
	}
	if n := limOpenOf(w, w.robin.PrincipalID); n != limit {
		t.Fatalf("%d open sessions of Robin in the database, want %d", n, limit)
	}
}
