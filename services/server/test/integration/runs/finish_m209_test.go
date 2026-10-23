// SPDX-License-Identifier: Apache-2.0

package runs_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/pb33f/libopenapi"
	"github.com/pb33f/libopenapi-validator/schema_validation"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/agents/adapters/agentstest"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	runsapp "github.com/OWNER/bothy/services/server/internal/runs/app"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// started opens, delivers and starts a run of Lotse for Alex and returns it with its token.
func (w *world) started() (runRow, string, string) {
	w.t.Helper()
	r, m := w.openRun()
	tok := w.deliver(r.ID)
	w.wantOK(w.start("lotse", r.ID, tok))
	return r, m, tok
}

// TestAwaitInput: running -> waiting_input with the status line; again 200 without change; from
// queued 409 invalid_state_transition (design note 6 a).
func TestAwaitInput(t *testing.T) {
	t.Parallel()
	w := runsWorld(t)
	r, m, _ := w.started()
	if w.wantOK(w.await("lotse", r.ID, "Waiting for approval"))["state"] != "waiting_input" {
		t.Fatal("not waiting")
	}
	got := w.lifecycle(r.ID)
	if got.StatusLine != "Waiting for approval" {
		t.Fatalf("run %+v", got)
	}
	if st, _, _ := w.triggerOf(m); st != "working" {
		t.Fatalf("trigger_state %s", st)
	}
	seq := w.maxSeq()
	w.wantOK(w.await("lotse", r.ID, "Waiting for approval"))
	if again := w.lifecycle(r.ID); again.Version != got.Version || len(w.eventsAfter(seq)) != 0 {
		t.Fatal("the second wait changed the run")
	}
	q, _ := w.openRun()
	w.want(w.await("lotse", q.ID, "Waiting"), http.StatusConflict, "invalid_state_transition")
}

// TestFinishRun: the transitions and reason rules of design note 6 over HTTP.
func TestFinishRun(t *testing.T) {
	t.Parallel()
	w := runsWorld(t)
	r, m, _ := w.started()
	w.clk.Advance(2 * time.Second)
	body := w.wantOK(w.finish("lotse", r.ID, `"state":"completed","reason":"ignored","summary":"Found the cause","metrics":{"tokens_in":5,"tokens_out":7}`))
	if body["state"] != "completed" {
		t.Fatalf("answer %v", body)
	}
	got := w.lifecycle(r.ID)
	if got.State != "completed" || got.Reason != "" || got.Summary != "Found the cause" || got.FinishedAt.IsZero() || got.Duration <= 0 ||
		!strings.Contains(got.Metrics, `"tokens_in": 5`) {
		t.Fatalf("run %+v", got)
	}
	if st, _, _ := w.triggerOf(m); st != "done" {
		t.Fatalf("trigger_state %s", st)
	}
	last := w.metrics.list[len(w.metrics.list)-1]
	if last.To != domain.StateCompleted || last.Ran <= 0 {
		t.Fatalf("metrics %+v", last)
	}
	seq := w.maxSeq()
	w.wantOK(w.finish("lotse", r.ID, `"state":"completed"`))
	if again := w.lifecycle(r.ID); again.Version != got.Version || len(w.eventsAfter(seq)) != 0 {
		t.Fatal("the same end state again changed the run")
	}
	w.want(w.finish("lotse", r.ID, `"state":"failed","reason":"runtime_error"`), http.StatusConflict, "invalid_state_transition")

	q, _ := w.openRun()
	w.want(w.finish("lotse", q.ID, `"state":"completed"`), http.StatusConflict, "invalid_state_transition")
	p := w.want(w.finish("lotse", q.ID, `"state":"failed"`), http.StatusBadRequest, "validation_failed")
	if !strings.Contains(string(mustJSON(t, p)), `"/reason"`) {
		t.Fatalf("problem %v", p)
	}
	w.want(w.finish("lotse", q.ID, `"state":"cancelled","reason":"stale"`), http.StatusBadRequest, "validation_failed")
	// costs is booked from M2-22 on (test/integration/costs); its not_enabled answer of M2-09 is gone.
	w.wantOK(w.finish("lotse", q.ID, `"state":"failed","reason":"stale"`))
	if got := w.lifecycle(q.ID); got.State != "failed" || got.Reason != "adapter_error" {
		t.Fatalf("run %+v", got)
	}
	if n := w.count(`SELECT count(*) FROM run_steps WHERE run_id = $1 AND name = 'adapter_reason'`, q.ID); n != 1 {
		t.Fatal("no step of the replaced reason")
	}
	c, _ := w.openRun()
	w.wantOK(w.finish("lotse", c.ID, `"state":"cancelled","reason":"user_cancelled"`))
	if got := w.lifecycle(c.ID); got.State != "cancelled" || got.Reason != "user_cancelled" {
		t.Fatalf("run %+v", got)
	}
}

// TestFinishRun_AdapterReportsMismatch_AG11: failed with billing_mode_mismatch from queued ends
// the run and writes the audit entry of 10 section 14 (design note 6 e).
func TestFinishRun_AdapterReportsMismatch_AG11(t *testing.T) {
	t.Parallel()
	w := runsWorld(t)
	q, _ := w.openRun()
	w.wantOK(w.finish("lotse", q.ID, `"state":"failed","reason":"billing_mode_mismatch"`))
	if got := w.lifecycle(q.ID); got.State != "failed" || got.Reason != "billing_mode_mismatch" {
		t.Fatalf("run %+v", got)
	}
	d := w.denies("run.finish", "billing_mode_mismatch")
	if len(d) != 1 || d[0].Details["billing_mode_expected"] != "organization" || d[0].Details["tool_class"] == nil || d[0].ResourceID != q.ID {
		t.Fatalf("audit %+v", d)
	}
}

// TestCancelRun_Queued: a queued run ends at once with user_cancelled and run.cancel_requested
// names Alex; a later start gets 409 already_started; no timeout job (design note 7 b).
func TestCancelRun_Queued(t *testing.T) {
	t.Parallel()
	w := runsWorld(t)
	r, m := w.openRun()
	t1 := w.deliver(r.ID)
	resp := w.want(w.cancel("alex", r.ID), http.StatusAccepted, "")
	if resp["state"] != "cancelled" || resp["state_reason"] != "user_cancelled" {
		t.Fatalf("answer %v", resp)
	}
	if st, reason, _ := w.triggerOf(m); st != "cancelled" || reason != "user_cancelled" {
		t.Fatalf("trigger_state %s %s", st, reason)
	}
	ev := w.eventsOf("run.cancel_requested", r.ID)
	if len(ev) != 1 || ev[0]["reason"] != "user_cancelled" || ev[0]["requested_by"] != w.alex.String() || ev[0]["run_id"] != r.ID {
		t.Fatalf("events %v", ev)
	}
	if aud := w.text(`SELECT audience_principal_id::text FROM event_log WHERE type = 'run.cancel_requested' AND resource_id = $1`, r.ID); aud != w.lotse.String() {
		t.Fatalf("audience %s", aud)
	}
	w.want(w.start("lotse", r.ID, t1), http.StatusConflict, "already_started")
	if n := len(w.cancels.of(r.ID)); n != 0 {
		t.Fatalf("%d timeout jobs", n)
	}
}

// TestCancelRun_Running_AG14: a running run gets the request and one job at agent.cancel_grace;
// the job ends it cancelled with cancel_timeout; later changes of the adapter are refused, the
// same end state again answers without change (AG-14 server part).
func TestCancelRun_Running_AG14(t *testing.T) {
	t.Parallel()
	w := runsWorld(t)
	r, _, tok := w.started()
	now := w.clk.Now()
	if w.want(w.cancel("alex", r.ID), http.StatusAccepted, "")["state"] != "running" {
		t.Fatal("the state changed")
	}
	jobs := w.cancels.of(r.ID)
	if len(jobs) != 1 || !jobs[0].At.Equal(now.Add(30*time.Second).UTC().Truncate(time.Microsecond)) {
		t.Fatalf("jobs %+v, now %v", jobs, now)
	}
	if ev := w.eventsOf("run.cancel_requested", r.ID); len(ev) != 1 || ev[0]["reason"] != "user_cancelled" {
		t.Fatalf("events %v", ev)
	}
	w.clk.Advance(31 * time.Second)
	must(t, w.runs.CancelTimeout(context.Background(), jobs[0].Run))
	got := w.lifecycle(r.ID)
	if got.State != "cancelled" || got.Reason != "cancel_timeout" {
		t.Fatalf("run %+v", got)
	}
	w.want(w.finish("lotse", r.ID, `"state":"completed"`), http.StatusConflict, "invalid_state_transition")
	w.want(w.await("lotse", r.ID, "Waiting"), http.StatusConflict, "invalid_state_transition")
	w.want(w.resume("lotse", r.ID, tok), http.StatusConflict, "already_started")
	w.wantOK(w.finish("lotse", r.ID, `"state":"cancelled","reason":"user_cancelled"`))
	if again := w.lifecycle(r.ID); again.Version != got.Version || again.Reason != "cancel_timeout" {
		t.Fatalf("run %+v", again)
	}
	// A repeated job is harmless (02 12.4 rule 1).
	must(t, w.runs.CancelTimeout(context.Background(), jobs[0].Run))
}

// TestCancelRun_AdapterConfirms: the adapter's cancelled within the window wins; the later job
// changes nothing.
func TestCancelRun_AdapterConfirms(t *testing.T) {
	t.Parallel()
	w := runsWorld(t)
	r, _, _ := w.started()
	w.want(w.cancel("alex", r.ID), http.StatusAccepted, "")
	w.wantOK(w.finish("lotse", r.ID, `"state":"cancelled","reason":"user_cancelled"`))
	got := w.lifecycle(r.ID)
	w.clk.Advance(time.Minute)
	must(t, w.runs.CancelTimeout(context.Background(), ids.MustParse(r.ID)))
	if again := w.lifecycle(r.ID); again != got || again.Reason != "user_cancelled" {
		t.Fatalf("run %+v", again)
	}
}

// TestCancelRun_Rights: the starter and the owner may; Sam with trigger but neither starter nor
// manage gets 403, with manage he may; Robin without read gets 404 like an unknown id; Lotse's
// token gets 403; denials are audited, allows have one entry with the details of note 7 e.
func TestCancelRun_Rights(t *testing.T) {
	t.Parallel()
	w := runsWorld(t)
	r1, _, _ := w.started()
	w.want(w.cancel("alex", r1.ID), http.StatusAccepted, "")
	r2, _, _ := w.started()
	w.want(w.cancel("owner", r2.ID), http.StatusAccepted, "")
	r3, _, _ := w.started()
	w.grant(`"subject_kind":"user","subject_id":"` + w.sam.String() + `","target_kind":"channel","target_id":"` + w.allgemein.String() +
		`","level":"trigger","reason":"Helper"`)
	w.want(w.cancel("sam", r3.ID), http.StatusForbidden, "forbidden")
	w.want(w.cancel("robin", r3.ID), http.StatusNotFound, "not_found")
	w.want(w.cancel("robin", ids.New().String()), http.StatusNotFound, "not_found")
	w.want(w.cancel("lotse", r3.ID), http.StatusForbidden, "forbidden")
	w.grant(`"subject_kind":"user","subject_id":"` + w.sam.String() + `","target_kind":"channel","target_id":"` + w.allgemein.String() +
		`","level":"trigger","capabilities":["manage"],"reason":"Lead"`)
	w.want(w.cancel("sam", r3.ID), http.StatusAccepted, "")
	var allows int
	for _, a := range w.audits("run.cancel") {
		if a.Decision == "allow" {
			allows++
			if a.Details["state_before"] != "running" || a.Details["immediate"] != false || a.Details["cancel_reason"] != "user_cancelled" {
				t.Fatalf("allow entry %+v", a)
			}
		}
	}
	if allows != 3 || len(w.denies("run.cancel", "not_self")) == 0 {
		t.Fatalf("%d allows, denies %+v", allows, w.audits("run.cancel"))
	}
}

// TestCancelRun_TerminalAndIdempotency: a cancelled run answers 202 without change with audit
// no_change; a completed run 409; a replay with the same Idempotency-Key answers the stored
// response without a second event; an invalid reason gives 400.
func TestCancelRun_TerminalAndIdempotency(t *testing.T) {
	t.Parallel()
	w := runsWorld(t)
	q, _ := w.openRun()
	k := ids.New().String()
	first := w.do("alex", http.MethodPost, "/api/v1/runs/"+q.ID+"/cancel", `{"reason":"Wrong ticket"}`, "Idempotency-Key", k)
	w.want(first, http.StatusAccepted, "")
	seq := w.maxSeq()
	again := w.do("alex", http.MethodPost, "/api/v1/runs/"+q.ID+"/cancel", `{"reason":"Wrong ticket"}`, "Idempotency-Key", k)
	var a, b map[string]any
	must(t, json.Unmarshal(first.Body, &a))
	must(t, json.Unmarshal(again.Body, &b))
	if again.StatusCode != http.StatusAccepted || string(mustJSON(t, a)) != string(mustJSON(t, b)) || len(w.eventsAfter(seq)) != 0 {
		t.Fatalf("replay %d %s", again.StatusCode, again.Body)
	}
	w.want(w.cancel("alex", q.ID), http.StatusAccepted, "")
	found := false
	for _, a := range w.audits("run.cancel") {
		found = found || a.Details["no_change"] == true
	}
	if !found {
		t.Fatal("no audit entry with no_change")
	}
	if len(w.eventsAfter(seq)) != 0 {
		t.Fatal("the cancel of a cancelled run wrote an event")
	}
	r, _, _ := w.started()
	w.wantOK(w.finish("lotse", r.ID, `"state":"completed"`))
	w.want(w.cancel("alex", r.ID), http.StatusConflict, "invalid_state_transition")
	w.want(w.do("alex", http.MethodPost, "/api/v1/runs/"+r.ID+"/cancel", `{"reason":"bad\u0007text"}`, key()...), http.StatusBadRequest, "validation_failed")
}

// TestRevocation_RSY11: revoking Alex's Support grant writes run.cancel_requested with
// access_revoked for her two Support runs in the revoking transaction and schedules two jobs; the
// states and her Marketing run stay; the adapter's cancelled ends the first, the job the second.
func TestRevocation_RSY11(t *testing.T) {
	t.Parallel()
	w := runsWorld(t)
	running, _, _ := w.started()
	queued, _ := w.openRun()
	w.deliver(queued.ID)
	w.grant(`"subject_kind":"user","subject_id":"` + w.alex.String() + `","target_kind":"channel","target_id":"` + w.kampagnen.String() +
		`","level":"trigger","reason":"Campaigns"`)
	mth := w.newThread(w.kampagnen)
	mm := w.post("alex", mth, "Please draft the text")
	market, ok := w.runOf(mm["id"].(string))
	if !ok {
		t.Fatal("no Marketing run")
	}
	w.wantOK(w.do("owner", http.MethodPost, "/api/v1/grants/"+w.g1.String()+"/revoke", `{"reason":"Left the team"}`, key()...))
	for _, r := range []runRow{running, queued} {
		ev := w.eventsOf("run.cancel_requested", r.ID)
		if len(ev) != 1 || ev[0]["reason"] != "access_revoked" || ev[0]["requested_by"] != nil {
			t.Fatalf("events of %s: %v", r.ID, ev)
		}
		if len(w.cancels.of(r.ID)) != 1 {
			t.Fatalf("jobs of %s", r.ID)
		}
		same := w.count(`SELECT count(*) FROM event_log c JOIN event_log a ON a.correlation_id = c.correlation_id AND a.type = 'access.changed'
			AND a.audience_principal_id = $2 WHERE c.type = 'run.cancel_requested' AND c.resource_id = $1`, r.ID, w.alex.String())
		if same == 0 {
			t.Fatalf("run.cancel_requested of %s is not in the transaction of access.changed", r.ID)
		}
	}
	if got := w.lifecycle(running.ID); got.State != "running" {
		t.Fatalf("running run %+v", got)
	}
	if got := w.lifecycle(queued.ID); got.State != "queued" {
		t.Fatalf("queued run %+v", got)
	}
	if len(w.eventsOf("run.cancel_requested", market.ID)) != 0 || w.lifecycle(market.ID).State != "queued" {
		t.Fatal("the Marketing run was touched")
	}
	w.wantOK(w.finish("lotse", running.ID, `"state":"cancelled","reason":"access_revoked"`))
	if got := w.lifecycle(running.ID); got.State != "cancelled" || got.Reason != "access_revoked" {
		t.Fatalf("run %+v", got)
	}
	w.clk.Advance(31 * time.Second)
	must(t, w.runs.CancelTimeout(context.Background(), ids.MustParse(queued.ID)))
	if got := w.lifecycle(queued.ID); got.State != "cancelled" || got.Reason != "cancel_timeout" {
		t.Fatalf("run %+v", got)
	}
	w.validateEvents()
}

// TestRevocation_Variants: user.disable gives user_disabled, archiveChannel channel_read_only, and
// a change that keeps trigger (a capability only) writes nothing (design note 8).
func TestRevocation_Variants(t *testing.T) {
	t.Parallel()
	t.Run("disableUser", func(t *testing.T) {
		t.Parallel()
		w := runsWorld(t)
		r, _, _ := w.started()
		q, _ := w.openRun()
		// The world has no lifecycle handlers of identity (no sessions bound), so the access change
		// of user.disable reaches the subscriber of runs directly, inside a unit of work; every
		// active run of the person is affected (08 11 point 7).
		acc := runsapp.NewAcceptor()
		must(t, acc.Bind(w.runs))
		must(t, w.inUnit(func(ctx context.Context) error {
			return acc.OnAccessChanged(ctx, authorize.AccessChanged{Action: authorize.ActionUserDisable,
				Changes: []authorize.PrincipalAccessChange{{PrincipalID: w.alex}}})
		}))
		for _, x := range []runRow{r, q} {
			if ev := w.eventsOf("run.cancel_requested", x.ID); len(ev) != 1 || ev[0]["reason"] != "user_disabled" || len(w.cancels.of(x.ID)) != 1 {
				t.Fatalf("events %v", ev)
			}
		}
	})
	t.Run("archiveChannel", func(t *testing.T) {
		t.Parallel()
		w := runsWorld(t)
		r, _, _ := w.started()
		w.wantOK(w.do("owner", http.MethodPost, "/api/v1/channels/"+w.allgemein.String()+"/archive", `{"reason":"Closed"}`, key()...))
		if ev := w.eventsOf("run.cancel_requested", r.ID); len(ev) != 1 || ev[0]["reason"] != "channel_read_only" {
			t.Fatalf("events %v", ev)
		}
		w.validateEvents()
	})
	t.Run("capabilityOnly", func(t *testing.T) {
		t.Parallel()
		w := runsWorld(t)
		r, _, _ := w.started()
		w.grant(`"subject_kind":"user","subject_id":"` + w.alex.String() + `","target_kind":"channel","target_id":"` + w.allgemein.String() +
			`","level":"trigger","capabilities":["manage"],"reason":"Lead"`)
		if ev := w.eventsOf("run.cancel_requested", r.ID); len(ev) != 0 || len(w.cancels.of(r.ID)) != 0 {
			t.Fatalf("events %v", ev)
		}
	})
}

// TestAgentDisabled_CancelsRuns: disableAgent ends Lotse's active runs cancelled agent_disabled
// in the same transaction and writes run.cancel_requested; no timeout job (design note 9).
func TestAgentDisabled_CancelsRuns(t *testing.T) {
	t.Parallel()
	w := runsWorld(t)
	running, _, _ := w.started()
	queued, _ := w.openRun()
	w.wantOK(w.do("owner", http.MethodPost, "/api/v1/agents/"+w.lotse.String()+"/disable", `{"reason":"Maintenance"}`, key()...))
	for _, r := range []runRow{running, queued} {
		if got := w.lifecycle(r.ID); got.State != "cancelled" || got.Reason != "agent_disabled" {
			t.Fatalf("run %+v", got)
		}
		if ev := w.eventsOf("run.cancel_requested", r.ID); len(ev) != 1 || ev[0]["reason"] != "agent_disabled" {
			t.Fatalf("events %v", ev)
		}
		if len(w.cancels.of(r.ID)) != 0 {
			t.Fatal("a timeout job for a disabled agent")
		}
	}
}

// TestStallCheck_Interrupted_AG10: a persisted heartbeat without the run more than
// agent.presence_timeout after its start fails it interrupted; the thread keeps one run and no new
// invocation follows (R-AG-17).
func TestStallCheck_Interrupted_AG10(t *testing.T) {
	t.Parallel()
	w := runsWorld(t)
	r, m, _ := w.started()
	startedAt := w.lifecycle(r.ID).StartedAt
	invocations := w.count(`SELECT count(*) FROM event_log WHERE type = 'agent.invocation'`)
	agentstest.PersistHeartbeat(t, w.db, w.lotse, startedAt.Add(90*time.Second), []ids.ID{})
	w.clk.Set(startedAt.Add(90 * time.Second))
	must(t, w.runs.StallCheck(context.Background()))
	if got := w.lifecycle(r.ID); got.State != "running" {
		t.Fatalf("exactly agent.presence_timeout after the start: %+v", got)
	}
	agentstest.PersistHeartbeat(t, w.db, w.lotse, startedAt.Add(91*time.Second), []ids.ID{})
	w.clk.Set(startedAt.Add(92 * time.Second))
	must(t, w.runs.StallCheck(context.Background()))
	if got := w.lifecycle(r.ID); got.State != "failed" || got.Reason != "interrupted" {
		t.Fatalf("run %+v", got)
	}
	if st, reason, _ := w.triggerOf(m); st != "failed" || reason != "interrupted" {
		t.Fatalf("trigger_state %s %s", st, reason)
	}
	if n := w.count(`SELECT count(*) FROM runs WHERE thread_id = $1`, r.Thread); n != 1 {
		t.Fatalf("%d runs in the thread", n)
	}
	if n := w.count(`SELECT count(*) FROM event_log WHERE type = 'agent.invocation'`); n != invocations {
		t.Fatal("a new invocation followed")
	}
}

// TestStallCheck_HeartbeatAfterReadWins: a heartbeat stored after the stall check read the
// heartbeats but before it locked the run is the one the verdict uses, so a run whose adapter
// came back in between keeps running (09 5.5; CodeRabbit finding on endOrphan).
func TestStallCheck_HeartbeatAfterReadWins(t *testing.T) {
	t.Parallel()
	w := runsWorld(t)
	a, _, _ := w.started()
	w.clk.Advance(15*time.Minute + time.Second)
	w.heartbeats.onNextRead(func() {
		agentstest.PersistHeartbeat(t, w.db, w.lotse, w.clk.Now(), []ids.ID{ids.MustParse(a.ID)})
	})
	must(t, w.runs.StallCheck(context.Background()))
	if got := w.lifecycle(a.ID); got.State != "running" {
		t.Fatalf("run %+v", got)
	}
}

// TestStallCheck_Stale: without a heartbeat for longer than run.orphan_timeout running and
// waiting runs fail stale; a recent heartbeat listing the run changes nothing.
func TestStallCheck_Stale(t *testing.T) {
	t.Parallel()
	w := runsWorld(t)
	a, _, _ := w.started()
	b, _, _ := w.started()
	w.wantOK(w.await("lotse", b.ID, "Waiting"))
	w.clk.Advance(15*time.Minute + time.Second)
	must(t, w.runs.StallCheck(context.Background()))
	for _, r := range []runRow{a, b} {
		if got := w.lifecycle(r.ID); got.State != "failed" || got.Reason != "stale" {
			t.Fatalf("run %+v", got)
		}
	}
	c, _, _ := w.started()
	w.clk.Advance(15*time.Minute + time.Second)
	agentstest.PersistHeartbeat(t, w.db, w.lotse, w.clk.Now().Add(-time.Second), []ids.ID{ids.MustParse(c.ID)})
	must(t, w.runs.StallCheck(context.Background()))
	if got := w.lifecycle(c.ID); got.State != "running" {
		t.Fatalf("run %+v", got)
	}
}

// validateEvents checks every run.state_changed and run.cancel_requested payload of the world
// against its schema; none carries JSON null.
func (w *world) validateEvents() {
	w.t.Helper()
	doc, err := libopenapi.NewDocument(rest.Spec)
	must(w.t, err)
	model, err := doc.BuildV3Model()
	must(w.t, err)
	for typ, schema := range map[string]string{"run.state_changed": "RunStateChangedPayload", "run.cancel_requested": "RunCancelRequestedPayload"} {
		proxy, ok := model.Model.Components.Schemas.Get(schema)
		if !ok {
			w.t.Fatalf("no schema %s", schema)
		}
		rs, err := w.db.Query(context.Background(), `SELECT payload::text FROM event_log WHERE type = $1`, typ)
		must(w.t, err)
		var payloads []string
		for rs.Next() {
			var p string
			must(w.t, rs.Scan(&p))
			payloads = append(payloads, p)
		}
		rs.Close()
		if len(payloads) == 0 {
			w.t.Fatalf("no %s", typ)
		}
		for _, p := range payloads {
			if strings.Contains(p, "null") {
				w.t.Fatalf("null in %s", p)
			}
			if valid, problems := schema_validation.NewSchemaValidator().ValidateSchemaString(proxy.Schema(), p); !valid {
				w.t.Fatalf("%s does not match %s: %v", p, schema, problems)
			}
		}
	}
}

// mustJSON encodes v.
func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	must(t, err)
	return b
}
