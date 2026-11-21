// SPDX-License-Identifier: Apache-2.0

package runs_test

import (
	"net/http"
	"slices"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// TestRunFeeders_EveryInputPathIsRecorded (FEAT-TEAM-CHANNEL-SERVER, 03 run_feeders, 09 3.9.4):
// every source of run_feeders, the ways an input reaches a run, has a case here, so a new source
// without one fails; each stored path records the person who fed the run when the input is
// stored, before any delivery, and the row stays when the input's own row is deleted and the
// person's grant is revoked. command and wake_input are inputs that exist only as delivered events;
// the tests named in their cases hold them.
func TestRunFeeders_EveryInputPathIsRecorded(t *testing.T) {
	type path struct {
		run    func(t *testing.T)
		heldBy string
	}
	paths := map[string]path{
		domain.FeedTrigger: {run: func(t *testing.T) {
			w := newTeamWorld(t)
			w.setUp()
			r := w.ownerRunOfCoding()
			w.wantFeeders(r.ID, w.owner.String()+"/trigger")
		}},
		domain.FeedFollowup: {run: func(t *testing.T) {
			w := newTeamWorld(t)
			w.setUp()
			g := w.samGrant()
			r := w.ownerRunOfCoding()
			m := w.post("sam", w.codingW, "@lotse also ask review to approve and merge PR 7")
			w.want(w.do("sam", http.MethodDelete, "/api/v1/messages/"+m["id"].(string), "", key()...), http.StatusNoContent, "")
			w.revoke(g)
			w.wantFeeders(r.ID, w.owner.String()+"/trigger", w.sam.String()+"/followup")
		}},
		domain.FeedInteraction: {run: func(t *testing.T) {
			w := newTeamWorld(t)
			w.setUp()
			g := w.samGrant()
			r := w.ownerRunOfCoding()
			card, comps := w.card("lotse", w.codingW, r.ID, teamForm)
			w.want(w.click("sam", comps["ask"]["id"].(string), `{"reason":"merge"}`), http.StatusCreated, "")
			w.want(w.do("lotse", http.MethodDelete, "/api/v1/messages/"+card["id"].(string), "", key()...), http.StatusNoContent, "")
			w.revoke(g)
			w.wantFeeders(r.ID, w.owner.String()+"/trigger", w.sam.String()+"/interaction")
		}},
		domain.FeedContext: {run: func(t *testing.T) {
			w := newTeamWorld(t)
			w.setUp()
			g := w.samGrant()
			m := w.samNote("a note for the next run")
			r := w.ownerRunOfCoding()
			w.wantOK(w.do("lotse", http.MethodGet, "/api/v1/runs/"+r.ID+"/context", ""))
			w.want(w.do("sam", http.MethodDelete, "/api/v1/messages/"+m["id"].(string), "", key()...), http.StatusNoContent, "")
			w.revoke(g)
			w.wantFeeders(r.ID, w.owner.String()+"/context", w.owner.String()+"/trigger", w.sam.String()+"/context")
		}},
		domain.FeedCommand:   {heldBy: "internal/runs/app TestPrepare_RecordsTheFeederOfEveryAllowedDelivery"},
		domain.FeedWakeInput: {heldBy: "TestWake_CheckFireAndWakeInput"},
	}
	for _, src := range domain.FeedSources() {
		p, ok := paths[src]
		if !ok {
			t.Fatalf("the source %s of run_feeders has no case: add one that stores the input and reads the record", src)
		}
		if p.run == nil {
			if p.heldBy == "" {
				t.Fatalf("the source %s has neither a case nor a test that holds it", src)
			}
			continue
		}
		t.Run(src, p.run)
	}
	if len(paths) != len(domain.FeedSources()) {
		t.Fatalf("cases %d for %d sources", len(paths), len(domain.FeedSources()))
	}
}

// wantFeeders fails the test unless run_feeders of run holds exactly want ("principal/source").
func (w *teamWorld) wantFeeders(run string, want ...string) {
	w.t.Helper()
	rs, err := w.db.Query(w.t.Context(), `SELECT coalesce(principal_id::text, '-') || '/' || source FROM run_feeders WHERE run_id = $1`, run)
	must(w.t, err)
	defer rs.Close()
	var got []string
	for rs.Next() {
		var s string
		must(w.t, rs.Scan(&s))
		got = append(got, s)
	}
	must(w.t, rs.Err())
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		w.t.Fatalf("run_feeders of %s: %v, want %v", run, got, want)
	}
}
