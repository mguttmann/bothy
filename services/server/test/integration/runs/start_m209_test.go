// SPDX-License-Identifier: Apache-2.0

package runs_test

import (
	"context"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// TestStartRun_HappyPath: a delivered run starts with its token: 200 Run in running, the start
// columns stored, renewed_from_jti NULL, version + 1, trigger_state working, one
// run.state_changed and bothy.run.start.latency with started (M2-09 design note 3 f).
func TestStartRun_HappyPath(t *testing.T) {
	t.Parallel()
	w := runsWorld(t)
	r, m := w.openRun()
	t1 := w.deliver(r.ID)
	before := w.lifecycle(r.ID)
	seq := w.maxSeq()
	w.clk.Advance(5 * time.Second)
	body := w.wantOK(w.start("lotse", r.ID, t1, `"model":"model-large"`, `"billing_evidence":{"provider":"example","key_type":"api_key","method":"env"}`))
	if body["state"] != "running" || int64(body["version"].(float64)) != before.Version+1 {
		t.Fatalf("answer %v", body)
	}
	got := w.lifecycle(r.ID)
	if got.State != "running" || got.Started != jtiOf(t, t1) || got.Renewed != "" || got.Used != "organization" || got.Model != "model-large" ||
		got.Invocation != before.Invocation || got.StartedAt.IsZero() || got.Version != before.Version+1 {
		t.Fatalf("run %+v", got)
	}
	if got.Evidence != `{"method": "env", "key_type": "api_key", "provider": "example"}` {
		t.Fatalf("evidence %s", got.Evidence)
	}
	if st, _, _ := w.triggerOf(m); st != "working" {
		t.Fatalf("trigger_state %s", st)
	}
	if c := countType(w.eventsAfter(seq), "run.state_changed"); c != 1 {
		t.Fatalf("run.state_changed %d", c)
	}
	last := w.metrics.list[len(w.metrics.list)-1]
	if last.From != domain.StateQueued || last.To != domain.StateRunning || last.QueuedFor <= 0 {
		t.Fatalf("metrics %+v", last)
	}
	w.noTokenAnywhere(t1)
}

// TestStartRun_Replay: the same token again answers 200 with Idempotent-Replay: true, also after
// its exp, and writes no event; once the run waits for input, the start token is consumed
// (superseded) for startRun, resumeRun and renewInvocationToken (R-SY-19, D-W-44, AG-31).
func TestStartRun_Replay(t *testing.T) {
	t.Parallel()
	w := runsWorld(t)
	r, _ := w.openRun()
	t1 := w.deliver(r.ID)
	w.wantOK(w.start("lotse", r.ID, t1))
	seq := w.maxSeq()
	w.clk.Advance(tokenConfig.TTL + tokenConfig.ClockLeeway + time.Minute)
	resp := w.start("lotse", r.ID, t1)
	if w.wantOK(resp)["state"] != "running" || resp.Header.Get("Idempotent-Replay") != "true" {
		t.Fatalf("replay %d %s %v", resp.StatusCode, resp.Body, resp.Header)
	}
	if n := len(w.eventsAfter(seq)); n != 0 {
		t.Fatalf("the replay wrote %d events", n)
	}
	w.wantOK(w.await("lotse", r.ID, "Waiting for approval"))
	w.wantReason(w.start("lotse", r.ID, t1), http.StatusForbidden, "invocation_token_invalid", "superseded")
	w.wantReason(w.resume("lotse", r.ID, t1), http.StatusForbidden, "invocation_token_invalid", "superseded")
	w.wantReason(w.renew("lotse", r.ID, t1), http.StatusForbidden, "invocation_token_invalid", "superseded")
	if got := w.lifecycle(r.ID); got.State != "waiting_input" {
		t.Fatalf("run %+v", got)
	}
}

// TestStartRun_TokenInvalid_AG08: every forged or foreign token gives 403
// invocation_token_invalid with reason invalid at startRun and renewInvocationToken; the run
// stays queued and unchanged, and the denial is audited with invocation_token_invalid (AG-08
// server part, AZ-P6, T-TK-05).
func TestStartRun_TokenInvalid_AG08(t *testing.T) {
	t.Parallel()
	w := runsWorld(t)
	r, _ := w.openRun()
	other, _ := w.openRun()
	t1 := w.deliver(r.ID)
	c := claimsOf(t, t1)
	with := func(k string, v any) map[string]any {
		out := map[string]any{}
		for kk, vv := range c {
			out[kk] = vv
		}
		out[k] = v
		return out
	}
	h := w.headerOf()
	header := func(k string, v any) map[string]any {
		out := map[string]any{}
		for kk, vv := range h {
			out[kk] = vv
		}
		out[k] = v
		return out
	}
	cases := map[string]string{
		"empty":           "",
		"garbled":         "a.b.c",
		"wrong signature": w.forge(h, c, true),
		"alg none":        w.forge(header("alg", "none"), c, false),
		"alg HS256":       w.forge(header("alg", "HS256"), c, false),
		"typ access":      w.forge(header("typ", "at+jwt"), c, false),
		"unknown kid":     w.forge(header("kid", "unknown-key"), c, false),
		"aud of Texter":   w.forge(h, with("aud", w.texter.String()), false),
		"another run":     w.forge(h, with("run_id", other.ID), false),
		"other iss":       w.forge(h, with("iss", ids.New().String()), false),
	}
	before := w.lifecycle(r.ID)
	for name, tok := range cases {
		t.Run(name, func(t *testing.T) {
			w.wantReason(w.start("lotse", r.ID, tok), http.StatusForbidden, "invocation_token_invalid", "invalid")
			w.wantReason(w.renew("lotse", r.ID, tok), http.StatusForbidden, "invocation_token_invalid", "invalid")
		})
	}
	if got := w.lifecycle(r.ID); got != before {
		t.Fatalf("the run changed: %+v, was %+v", got, before)
	}
	if len(w.denies("run.start", "invocation_token_invalid")) == 0 || len(w.denies("run.token.renew", "invocation_token_invalid")) == 0 {
		t.Fatal("no aggregated deny entry")
	}
	// An absent token is a schema error, not a token verdict.
	w.want(w.do("lotse", http.MethodPost, "/api/v1/runs/"+r.ID+"/start", `{"billing_mode_used":"organization","tool_class_used":"chat"}`),
		http.StatusBadRequest, "validation_failed")
	w.noTokenAnywhere(t1)
}

// TestStartRun_Expired: after agent.invocation_token_ttl plus auth.clock_leeway the token gives
// 403 invocation_token_invalid with reason expired; exactly at exp + leeway it still starts.
func TestStartRun_Expired(t *testing.T) {
	t.Parallel()
	w := runsWorld(t)
	r, _ := w.openRun()
	t1 := w.deliver(r.ID)
	exp := time.Unix(int64(claimsOf(t, t1)["exp"].(float64)), 0)
	w.clk.Set(exp.Add(tokenConfig.ClockLeeway).Add(time.Second))
	w.wantReason(w.start("lotse", r.ID, t1), http.StatusForbidden, "invocation_token_invalid", "expired")
	r2, _ := w.openRun()
	t2 := w.deliver(r2.ID)
	w.clk.Set(time.Unix(int64(claimsOf(t, t2)["exp"].(float64)), 0).Add(tokenConfig.ClockLeeway))
	w.wantOK(w.start("lotse", r2.ID, t2))
}

// TestStartRun_BillingModeMismatch_BR06: a reported mode other than the configured one, or
// key_type unknown with a matching mode, gives 409 billing_mode_mismatch; the run is failed with
// that reason, its trigger_state failed, the deny entry carries the modes and the evidence codes
// (AG-11 server part, BR-06, 10 section 14).
func TestStartRun_BillingModeMismatch_BR06(t *testing.T) {
	t.Parallel()
	w := runsWorld(t)
	r, m := w.openRun()
	t1 := w.deliver(r.ID)
	w.want(w.startWith("lotse", r.ID, t1, "personal", "chat"), http.StatusConflict, "billing_mode_mismatch")
	if got := w.lifecycle(r.ID); got.State != "failed" || got.Reason != "billing_mode_mismatch" {
		t.Fatalf("run %+v", got)
	}
	if st, reason, _ := w.triggerOf(m); st != "failed" || reason != "billing_mode_mismatch" {
		t.Fatalf("trigger_state %s %s", st, reason)
	}
	r2, _ := w.openRun()
	t2 := w.deliver(r2.ID)
	w.want(w.startWith("lotse", r2.ID, t2, "organization", "chat", `"billing_evidence":{"provider":"example","key_type":"unknown"}`),
		http.StatusConflict, "billing_mode_mismatch")
	if got := w.lifecycle(r2.ID); got.State != "failed" || got.Reason != "billing_mode_mismatch" {
		t.Fatalf("run %+v", got)
	}
	d := w.denies("run.start", "billing_mode_mismatch")
	if len(d) == 0 || d[0].Details["billing_mode_expected"] != "organization" || d[0].Details["billing_mode_used"] != "personal" {
		t.Fatalf("deny entries %+v", d)
	}
	w.noTokenAnywhere(t1, t2)
}

// TestStartRun_ToolClassMismatch_AG30: tool_class_used host against a token with scoped gives
// 409 tool_class_mismatch and a failed run; chat against scoped starts (AG-30 server part).
func TestStartRun_ToolClassMismatch_AG30(t *testing.T) {
	t.Parallel()
	w := runsWorld(t)
	w.sql(`UPDATE agents SET tool_class = 'scoped' WHERE principal_id = $1`, w.lotse.String())
	r, _ := w.openRun()
	t1 := w.deliver(r.ID)
	if claimsOf(t, t1)["tool_class"] != "scoped" {
		t.Fatalf("claims %v", claimsOf(t, t1))
	}
	w.want(w.startWith("lotse", r.ID, t1, "organization", "host"), http.StatusConflict, "tool_class_mismatch")
	if got := w.lifecycle(r.ID); got.State != "failed" || got.Reason != "tool_class_mismatch" {
		t.Fatalf("run %+v", got)
	}
	r2, _ := w.openRun()
	w.wantOK(w.startWith("lotse", r2.ID, w.deliver(r2.ID), "organization", "chat"))
}

// TestStartRun_OtherDeliveryAlreadyStarted: after a start with T2 the token T1 of an earlier
// delivery gives 409 already_started (R-SY-19).
func TestStartRun_OtherDeliveryAlreadyStarted(t *testing.T) {
	t.Parallel()
	w := runsWorld(t)
	r, _ := w.openRun()
	t1 := w.deliver(r.ID)
	t2 := w.deliver(r.ID)
	w.wantReason(w.start("lotse", r.ID, t1), http.StatusForbidden, "invocation_token_invalid", "superseded")
	w.wantOK(w.start("lotse", r.ID, t2))
	w.want(w.start("lotse", r.ID, t1), http.StatusConflict, "already_started")
}

// TestResumeAndStartEndpoints: resumeRun on queued and startRun on waiting_input with a current
// token give 409 invalid_state_transition (ruling D-M2-09-5); an agent that cannot read the run gets
// 404 and a person 403 subject_kind_not_allowed.
func TestResumeAndStartEndpoints(t *testing.T) {
	t.Parallel()
	w := runsWorld(t)
	r, _ := w.openRun()
	t1 := w.deliver(r.ID)
	w.want(w.resume("lotse", r.ID, t1), http.StatusConflict, "invalid_state_transition")
	// Texter reads nothing of Support/Allgemein: the run is invisible to it (not_self is
	// asserted by the authz requirement tests for a visible run).
	w.want(w.start("texter", r.ID, t1), http.StatusNotFound, "not_found")
	w.wantReason(w.start("alex", r.ID, t1), http.StatusForbidden, "forbidden", "subject_kind_not_allowed")
	w.want(w.start("lotse", ids.New().String(), t1), http.StatusNotFound, "not_found")
	w.wantOK(w.start("lotse", r.ID, t1))
	w.wantOK(w.await("lotse", r.ID, "Waiting"))
	a := w.followupToken(r)
	w.want(w.start("lotse", r.ID, a), http.StatusConflict, "invalid_state_transition")
	w.wantOK(w.resume("lotse", r.ID, a))
}

// followupToken posts a follow-up of Alex into the thread of the waiting run r and delivers
// agent.followup through runs.Delivery; the allowed delivery records its jti (M2-08 matrix).
func (w *world) followupToken(r runRow) string {
	w.t.Helper()
	msg := w.post("alex", ids.MustParse(r.Thread), "Here is the answer")
	p, err := domain.FollowupPayload(ids.MustParse(r.ID), ids.MustParse(r.Thread), ids.MustParse(msg["id"].(string)))
	must(w.t, err)
	out, err := w.runs.Prepare(context.Background(), deliveryInput(w.lotse, r.ID, "agent.followup", p))
	must(w.t, err)
	if !out.Send {
		w.t.Fatal("the follow-up was not delivered")
	}
	return out.InvocationToken
}

// TestAG28_SlowStartRenewAndRestart: an expired token is renewed (T2), the old one is then
// superseded, a redelivery after a restart brings T3, T2 is superseded, T3 starts and replays;
// exactly one transition to running (AG-28).
func TestAG28_SlowStartRenewAndRestart(t *testing.T) {
	t.Parallel()
	w := runsWorld(t)
	r, _ := w.openRun()
	t1 := w.deliver(r.ID)
	w.clk.Advance(tokenConfig.TTL + tokenConfig.ClockLeeway + time.Second)
	w.wantReason(w.start("lotse", r.ID, t1), http.StatusForbidden, "invocation_token_invalid", "expired")
	g := w.wantOK(w.renew("lotse", r.ID, t1))
	t2 := g["invocation_token"].(string)
	if t2 == "" || jtiOf(t, t2) == jtiOf(t, t1) || g["expires_at"] == nil {
		t.Fatalf("grant %v", g)
	}
	if got := w.lifecycle(r.ID); got.Invocation != jtiOf(t, t2) || got.Renewed != jtiOf(t, t1) {
		t.Fatalf("run %+v", got)
	}
	w.wantReason(w.start("lotse", r.ID, t1), http.StatusForbidden, "invocation_token_invalid", "superseded")
	// The restarted instance redelivers from the cursor before the event (M2-07): a new token.
	t3 := w.deliver(r.ID)
	w.wantReason(w.start("lotse", r.ID, t2), http.StatusForbidden, "invocation_token_invalid", "superseded")
	w.wantOK(w.start("lotse", r.ID, t3))
	resp := w.start("lotse", r.ID, t3)
	if resp.StatusCode != http.StatusOK || resp.Header.Get("Idempotent-Replay") != "true" {
		t.Fatalf("replay %d %s", resp.StatusCode, resp.Body)
	}
	if n := countTransitions(w, r.ID, domain.StateRunning); n != 1 {
		t.Fatalf("%d transitions to running", n)
	}
	w.noTokenAnywhere(t1, t2, t3)
}

// TestAG29_RenewRefusals: a token replaced by a redelivery gives superseded, a renewal of a
// running run 409 already_started, and after the revocation of Alex's grant the renewal gives 403
// agent_invocation_denied with the run failed access_revoked and its deny entry (AG-29, AZ-M51).
func TestAG29_RenewRefusals(t *testing.T) {
	t.Parallel()
	w := runsWorld(t)
	r, _ := w.openRun()
	t1 := w.deliver(r.ID)
	t2 := w.deliver(r.ID)
	w.wantReason(w.renew("lotse", r.ID, t1), http.StatusForbidden, "invocation_token_invalid", "superseded")
	r2, _ := w.openRun()
	s := w.deliver(r2.ID)
	w.wantOK(w.start("lotse", r2.ID, s))
	w.want(w.renew("lotse", r2.ID, s), http.StatusConflict, "already_started")
	w.wantOK(w.do("owner", http.MethodPost, "/api/v1/grants/"+w.g1.String()+"/revoke", `{"reason":"Left the team"}`, key()...))
	w.wantReason(w.renew("lotse", r.ID, t2), http.StatusForbidden, "agent_invocation_denied", "access_revoked")
	if got := w.lifecycle(r.ID); got.State != "failed" || got.Reason != "access_revoked" {
		t.Fatalf("run %+v", got)
	}
	found := false
	for _, a := range w.audits("agent.invoke") {
		found = found || (a.Decision == "deny" && a.Details["stage"] == "renewal" && a.Details["run_id"] == r.ID)
	}
	if !found {
		t.Fatal("no deny entry of the renewal")
	}
}

// TestAG31_ConsumedStartToken: the consumed start token neither renews nor resumes; the token of
// a follow-up delivery resumes the run with started_at unchanged (AG-31).
func TestAG31_ConsumedStartToken(t *testing.T) {
	t.Parallel()
	w := runsWorld(t)
	r, _ := w.openRun()
	s := w.deliver(r.ID)
	w.wantOK(w.start("lotse", r.ID, s))
	started := w.lifecycle(r.ID).StartedAt
	w.wantOK(w.await("lotse", r.ID, "Waiting"))
	w.wantReason(w.renew("lotse", r.ID, s), http.StatusForbidden, "invocation_token_invalid", "superseded")
	w.wantReason(w.resume("lotse", r.ID, s), http.StatusForbidden, "invocation_token_invalid", "superseded")
	if got := w.lifecycle(r.ID); got.State != "waiting_input" {
		t.Fatalf("run %+v", got)
	}
	a := w.followupToken(r)
	w.clk.Advance(time.Minute)
	if w.wantOK(w.resume("lotse", r.ID, a))["state"] != "running" {
		t.Fatal("not running")
	}
	if got := w.lifecycle(r.ID); !got.StartedAt.Equal(started) || got.Started != jtiOf(t, a) {
		t.Fatalf("run %+v, started_at was %v", got, started)
	}
}

// TestAG32_LostRenewAnswer: a lost renewal answer is recovered with the old token
// (renewed_from_jti stays), the lost token is superseded, the run starts once; afterwards the old
// token gives already_started, and once the run waits, superseded (AG-32).
func TestAG32_LostRenewAnswer(t *testing.T) {
	t.Parallel()
	w := runsWorld(t)
	r, _ := w.openRun()
	t1 := w.deliver(r.ID)
	w.clk.Advance(tokenConfig.TTL + tokenConfig.ClockLeeway + time.Second)
	t2 := w.wantOK(w.renew("lotse", r.ID, t1))["invocation_token"].(string)
	t3 := w.wantOK(w.renew("lotse", r.ID, t1))["invocation_token"].(string)
	if got := w.lifecycle(r.ID); got.Renewed != jtiOf(t, t1) || got.Invocation != jtiOf(t, t3) {
		t.Fatalf("run %+v", got)
	}
	w.wantReason(w.start("lotse", r.ID, t2), http.StatusForbidden, "invocation_token_invalid", "superseded")
	w.wantOK(w.start("lotse", r.ID, t3))
	if got := w.lifecycle(r.ID); got.Renewed != "" {
		t.Fatalf("renewed_from_jti after the start: %+v", got)
	}
	if n := countTransitions(w, r.ID, domain.StateRunning); n != 1 {
		t.Fatalf("%d starts", n)
	}
	w.want(w.renew("lotse", r.ID, t1), http.StatusConflict, "already_started")
	w.wantOK(w.await("lotse", r.ID, "Waiting"))
	w.wantReason(w.renew("lotse", r.ID, t1), http.StatusForbidden, "invocation_token_invalid", "superseded")
}

// TestConcurrency_ParallelStarts: parallel starts with the same token give one transition and
// replays; parallel cancelRun and startRun on a queued run never end both running and cancelled.
func TestConcurrency_ParallelStarts(t *testing.T) {
	t.Parallel()
	w := runsWorld(t)
	r, _ := w.openRun()
	t1 := w.deliver(r.ID)
	var wg sync.WaitGroup
	codes := make([]int, 6)
	for i := range codes {
		wg.Add(1)
		go func() {
			defer wg.Done()
			codes[i] = w.start("lotse", r.ID, t1).StatusCode
		}()
	}
	wg.Wait()
	for _, c := range codes {
		if c != http.StatusOK {
			t.Fatalf("codes %v", codes)
		}
	}
	if n := countTransitions(w, r.ID, domain.StateRunning); n != 1 {
		t.Fatalf("%d transitions", n)
	}
	r2, _ := w.openRun()
	t2 := w.deliver(r2.ID)
	var start, cancel int
	wg.Add(2)
	go func() { defer wg.Done(); start = w.start("lotse", r2.ID, t2).StatusCode }()
	go func() { defer wg.Done(); cancel = w.cancel("alex", r2.ID).StatusCode }()
	wg.Wait()
	got := w.lifecycle(r2.ID)
	switch got.State {
	case "running":
		if start != http.StatusOK || cancel != http.StatusAccepted || len(w.cancels.of(r2.ID)) != 1 {
			t.Fatalf("running with start %d cancel %d", start, cancel)
		}
	case "cancelled":
		if start != http.StatusConflict || cancel != http.StatusAccepted {
			t.Fatalf("cancelled with start %d cancel %d", start, cancel)
		}
	default:
		t.Fatalf("state %s", got.State)
	}
}

// countType counts the events of typ in list.
func countType(list []logEvent, typ string) int {
	n := 0
	for _, e := range list {
		if e.Type == typ {
			n++
		}
	}
	return n
}

// countTransitions counts the recorded transitions of run to state.
func countTransitions(w *world, run string, to domain.State) int {
	w.metrics.mu.Lock()
	defer w.metrics.mu.Unlock()
	n := 0
	for _, x := range w.metrics.list {
		if x.Run.ID.String() == run && x.To == to {
			n++
		}
	}
	return n
}
