// SPDX-License-Identifier: Apache-2.0

package runs_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/measure"
)

// countOf returns the calls of metric with dimension in channel (any channel when zero) and their
// good events.
func countOf(r *measure.Recording, metric measure.Metric, dimension string, channel ids.ID) (n, good int) {
	for _, c := range r.Calls(metric) {
		if c.Key.Dimension != dimension || !channel.IsZero() && c.Key.ChannelID != channel {
			continue
		}
		n++
		if c.Value.OK {
			good++
		}
	}
	return n, good
}

// TestMeasure_RunsPaths drives the measuring points of runs (M6-13, 25 3.2) through the real use
// cases on PostgreSQL 18, each recorded after its commit: a delivered and started run is
// runs_started with its billing mode and a good sli.run_start; a tool_call step the adapter
// finishes by patch is tool_calls completed; the completed run is runs_finished completed with
// its duration and sli.turn_duration; a queued run the adapter fails with billing_mode_mismatch is
// invocations_denied billing_mode_mismatch and a bad sli.run_start; a cancelled queued run is
// runs_finished cancelled without an sli.run_start. Every key carries the channel of the run and
// the agent.
func TestMeasure_RunsPaths(t *testing.T) {
	t.Parallel()
	w := runsWorld(t)
	r, _ := w.startedRun()
	step := ids.New().String()
	for i, o := range []string{
		`{"op":"add","path":"/steps/` + step + `","value":{"kind":"tool_call","name":"read_file"}}`,
		`{"op":"replace","path":"/steps/` + step + `/status","value":"completed"}`,
	} {
		body := `{"base_version":` + jsonInt(w.lifecycle(r.ID).Version) + `,"ops":[` + o + `]}`
		if resp := w.do("lotse", http.MethodPatch, "/api/v1/runs/"+r.ID, body); resp.StatusCode != http.StatusOK {
			t.Fatalf("patch %d = %d %s", i, resp.StatusCode, resp.Body)
		}
	}
	w.clk.Advance(2 * time.Second)
	w.wantOK(w.finish("lotse", r.ID, `"state":"completed"`))
	q, _ := w.openRun()
	w.wantOK(w.finish("lotse", q.ID, `"state":"failed","reason":"billing_mode_mismatch"`))
	c, _ := w.openRun()
	w.want(w.cancel("alex", c.ID), http.StatusAccepted, "")

	rec := w.measure
	if inv := rec.Invalid(); len(inv) != 0 {
		t.Fatalf("invalid measure calls %v", inv)
	}
	channel := w.threadChannel(r.Thread)
	for _, call := range rec.Calls("") {
		if call.Key.ChannelID != channel || call.Key.AgentID != w.lotse {
			t.Errorf("%s: key %+v, want the channel of the run and Lotse", call.Metric, call.Key)
		}
	}
	mode := w.lifecycle(r.ID).Used
	checks := []struct {
		metric          measure.Metric
		dimension       string
		wantN, wantGood int
	}{
		{measure.RunsStarted, mode, 1, 0},
		{measure.SLIRunStart, "", 2, 1},
		{measure.ToolCalls, "completed", 1, 0},
		{measure.RunsFinished, "completed", 1, 0},
		{measure.SLITurnDuration, "", 1, 1},
		{measure.RunsFinished, "failed", 1, 0},
		{measure.InvocationsDenied, "billing_mode_mismatch", 1, 0},
		{measure.RunsFinished, "cancelled", 1, 0},
	}
	for _, c := range checks {
		if n, good := countOf(rec, c.metric, c.dimension, channel); n != c.wantN || good != c.wantGood {
			t.Errorf("%s %q: %d calls, %d good; want %d and %d", c.metric, c.dimension, n, good, c.wantN, c.wantGood)
		}
	}
	for _, call := range rec.Calls(measure.RunsFinished) {
		if call.Key.Dimension == "completed" && call.Value.Elapsed < 2*time.Second {
			t.Errorf("runs_finished completed took %v, want the run duration", call.Value.Elapsed)
		}
	}
}

// TestMeasure_ApprovalCardsAndDenials drives approval_cards and the I2 denial of invocations_denied
// through the real paths (M6-13): an approval card of a run is requested once per card; a click on
// once approves; an expired card counts expired when runs.component_expiry expires it; a card
// without approve_actions counts nothing; after the grant of the clicking person is revoked, the
// delivery of a new click's interaction.created is denied before the delivery (I2) and counts
// i2_denied.
func TestMeasure_ApprovalCardsAndDenials(t *testing.T) {
	t.Parallel()
	w := runsWorld(t)
	r := w.waitingRun()
	thread := w.threadOf(r.ID)
	_, byID := w.card("lotse", thread, r.ID, approvalCard)
	w.want(w.click("alex", byID["once"]["id"].(string), ""), http.StatusCreated, "")
	w.card("lotse", thread, r.ID, twoButtons)
	exp := w.clk.Now().Add(10 * time.Minute).UTC().Format("2006-01-02T15:04:05.000Z")
	expiring := strings.ReplaceAll(approvalCard, `"required_capability":"approve_actions"}`, `"required_capability":"approve_actions","expires_at":"`+exp+`"}`)
	m, _ := w.card("lotse", thread, r.ID, expiring)
	_, again := w.card("lotse", thread, r.ID, approvalCard)
	ix := w.want(w.click("alex", again["deny"]["id"].(string), ""), http.StatusCreated, "")
	w.wantOK(w.do("owner", http.MethodPost, "/api/v1/grants/"+w.g1.String()+"/revoke", `{"reason":"Moved team"}`, key()...))
	if out := w.deliverInteraction(ix["id"].(string)); out.Send {
		t.Fatal("interaction.created was sent after the revocation")
	}
	w.clk.Advance(11 * time.Minute)
	must(t, w.runs.ExpireComponents(t.Context(), ids.MustParse(m["id"].(string))))

	rec := w.measure
	if inv := rec.Invalid(); len(inv) != 0 {
		t.Fatalf("invalid measure calls %v", inv)
	}
	channel := w.threadChannel(thread.String())
	for _, c := range []struct {
		dimension string
		want      int
	}{{"requested", 3}, {"approved", 1}, {"denied", 1}, {"expired", 1}} {
		if n, _ := countOf(rec, measure.ApprovalCards, c.dimension, channel); n != c.want {
			t.Errorf("approval_cards %s: %d, want %d (calls %+v)", c.dimension, n, c.want, rec.Calls(measure.ApprovalCards))
		}
	}
	if n, _ := countOf(rec, measure.InvocationsDenied, "i2_denied", channel); n != 1 {
		t.Errorf("invocations_denied i2_denied: %d, want 1 (calls %+v)", n, rec.Calls(measure.InvocationsDenied))
	}
	for _, call := range rec.Calls(measure.ApprovalCards) {
		if call.Key.AgentID != w.lotse {
			t.Errorf("approval_cards %s: agent %v, want Lotse", call.Key.Dimension, call.Key.AgentID)
		}
	}
}

// jsonInt formats n as a JSON number.
func jsonInt(n int64) string {
	b, _ := json.Marshal(n)
	return string(b)
}

// threadChannel returns the channel of thread.
func (w *world) threadChannel(thread string) ids.ID {
	w.t.Helper()
	return ids.MustParse(w.text(`SELECT channel_id::text FROM threads WHERE id = $1`, thread))
}
