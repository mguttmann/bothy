// SPDX-License-Identifier: Apache-2.0

package runcontext_test

import (
	"context"
	"net/http"
	"slices"
	"testing"
)

// feedersOf returns run_feeders of run as sorted "principal/source" lines ("-" for NULL).
func (c *ctxWorld) feedersOf(run string) []string {
	c.t.Helper()
	rows, err := c.db.Query(context.Background(), `SELECT coalesce(principal_id::text, '-') || '/' || source FROM run_feeders WHERE run_id = $1`, run)
	if err != nil {
		c.t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			c.t.Fatal(err)
		}
		out = append(out, s)
	}
	slices.Sort(out)
	return out
}

// TestRunContext_SessionCarriesTheRecordOfItsRuns (09 3.9.4, red team round 6 of
// FEAT-TEAM-CHANNEL-SERVER): a run that resumes a session takes over what the session already
// holds, so its record gets every principal recorded for the earlier runs of that session in the
// thread (here Sam, who fed the first run), although resume leaves those inputs and the agent's
// own messages out of the context. A bound session holds history from outside the thread, which
// no record names, so a run in mode bind records an input that speaks for no person.
func TestRunContext_SessionCarriesTheRecordOfItsRuns(t *testing.T) {
	t.Parallel()
	c := newCtxWorld(t)
	th := c.newThread(c.allgemein)
	run1, _ := c.triggerWith("alex", th, "@lotse first")
	c.startWithSession(run1, "sess-1", th)
	c.sql(`INSERT INTO run_feeders (run_id, principal_id, source) VALUES ($1, $2, 'followup')`, run1, c.sam.String())
	c.finishOK(run1)
	run2, _ := c.triggerWith("alex", th, "@lotse second")
	if p := c.invocation(run2); p["session_mode"] != "resume" {
		t.Fatalf("session_mode %v", p["session_mode"])
	}
	c.contextOf("lotse", run2)
	alex, sam := c.alex.String(), c.sam.String()
	if got, want := c.feedersOf(run2), []string{alex + "/context", alex + "/trigger", sam + "/context"}; !slices.Equal(got, want) {
		t.Fatalf("resume: run_feeders %v, want %v", got, want)
	}
	c.wantOK(c.start("lotse", run2, c.deliver(run2)))
	c.finishOK(run2)
	c.want(c.reset("alex", th), http.StatusCreated, "")
	fresh, _ := c.triggerWith("alex", th, "@lotse after the reset")
	if p := c.invocation(fresh); p["session_mode"] != "new" {
		t.Fatalf("session_mode %v", p["session_mode"])
	}
	c.contextOf("lotse", fresh)
	// A new session takes nothing over; the live boxes of the earlier runs in its window still
	// carry their runs' record (run_answers), Sam included.
	if got := c.feedersOf(fresh); slices.Contains(got, "-/context") || !slices.Contains(got, sam+"/context") {
		t.Fatalf("new: run_feeders %v", got)
	}
	c.wantOK(c.start("lotse", fresh, c.deliver(fresh)))
	c.finishOK(fresh)
	c.want(c.bind("alex", th, `{"external_session_ref":"sess-2"}`), http.StatusCreated, "")
	run3, _ := c.triggerWith("alex", th, "@lotse after the bind")
	if p := c.invocation(run3); p["session_mode"] != "bind" {
		t.Fatalf("session_mode %v", p["session_mode"])
	}
	c.contextOf("lotse", run3)
	if got := c.feedersOf(run3); !slices.Contains(got, "-/context") {
		t.Fatalf("bind: run_feeders %v lack the input that speaks for no person", got)
	}
}
