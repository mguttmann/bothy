// SPDX-License-Identifier: Apache-2.0

package runs_test

import (
	"net/http"
	"sync"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
)

// withRun is the member run_id of an answer to run.
func withRun(run string) string { return `"run_id":"` + run + `"` }

// answer posts text as Lotse into thread with run_id run under the client_msg_id id.
func (a *agentWorld) answer(thread ids.ID, run string, id ids.ID, text string) *httpservertest.Response {
	return a.send("lotse", thread, messageBodyWith(id, text, withRun(run)))
}

// unchangedRun fails when the run's state or version differ from r.
func (a *agentWorld) unchangedRun(name string, r runRow) {
	a.t.Helper()
	if got := a.runsWhere(`id = $1`, r.ID)[0]; got.State != r.State || got.Version != r.Version {
		a.t.Fatalf("%s: the run changed: %+v, was %+v", name, got, r)
	}
}

// lastSeq returns threads.last_message_seq of thread.
func (a *agentWorld) lastSeq(thread ids.ID) int64 {
	return a.count(`SELECT last_message_seq FROM threads WHERE id = $1`, thread.String())
}

// TestAnswer_RunIDActiveRun (M2-19 design notes 4 and 6): an answer that names the own active run
// of the thread in running, waiting_input and queued is 201; the Message has no run_id, the run
// and the triggering message are unchanged. Red before the change: 400 validation_failed with
// not_enabled.
func TestAnswer_RunIDActiveRun(t *testing.T) {
	t.Parallel()
	a := newAgentWorld(t)
	r, th, trig := a.startedRun()
	r = a.runsWhere(`id = $1`, r.ID)[0]
	_, _, trigVersion := a.triggerOf(trig)
	check := func(name string, r runRow, thread ids.ID) {
		t.Helper()
		m := a.want(a.answer(thread, r.ID, ids.New(), "Answer while "+r.State), http.StatusCreated, "")
		if _, ok := m["run_id"]; ok || m["origin"] != "agent" {
			t.Fatalf("%s: answer %v", name, m)
		}
		a.unchangedRun(name, r)
	}
	check("running", r, th)
	a.wantOK(a.await("lotse", r.ID, "Waiting for approval"))
	r = a.runsWhere(`id = $1`, r.ID)[0]
	check("waiting_input", r, th)
	if _, _, v := a.triggerOf(trig); v != trigVersion {
		t.Fatalf("the triggering message changed: version %d, was %d", v, trigVersion)
	}
	q, qth, _ := a.queuedRun("alex", a.allgemein, "lotse")
	if q.State != "queued" {
		t.Fatalf("run %+v", q)
	}
	check("queued", q, qth)
}

// TestAnswer_RunIDMismatch (05 rule 1.2, T-AI-04): an unknown run, Lotse's own active run of
// another thread and Texter's active guest run of this thread, named by Lotse, and Texter's run
// named by Alex all answer 400 validation_failed on /run_id with run_mismatch and equal bodies;
// nothing is stored.
func TestAnswer_RunIDMismatch(t *testing.T) {
	t.Parallel()
	a := newAgentWorld(t)
	own, _, _ := a.startedRun()
	guest, th, _ := a.queuedRun("owner", a.allgemein, "texter")
	if guest.Agent != a.texter.String() {
		t.Fatalf("the guest run belongs to %s", guest.Agent)
	}
	seq, last := a.maxSeq(), a.lastSeq(th)
	messages := a.count(`SELECT count(*) FROM messages`)
	var bodies []string
	for _, c := range []struct{ name, run string }{
		{"lotse", ids.New().String()}, {"lotse", own.ID}, {"lotse", guest.ID}, {"alex", guest.ID},
	} {
		r := a.send(c.name, th, messageBody("Answer", withRun(c.run)))
		p := a.want(r, http.StatusBadRequest, "validation_failed")
		errs, _ := p["errors"].([]any)
		f, _ := errs[0].(map[string]any)
		if len(errs) != 1 || f["pointer"] != "/run_id" || f["code"] != "run_mismatch" {
			t.Fatalf("%s %s: %s", c.name, c.run, r.Body)
		}
		bodies = append(bodies, normalized(r.Body))
	}
	for _, b := range bodies[1:] {
		if b != bodies[0] {
			t.Fatalf("bodies differ:\n%s\n%s", bodies[0], b)
		}
	}
	if a.count(`SELECT count(*) FROM messages`) != messages || a.lastSeq(th) != last {
		t.Fatal("a refused answer stored something")
	}
	if n := countType(a.eventsAfter(seq), "message.created"); n != 0 {
		t.Fatalf("%d message.created after refused answers", n)
	}
}

// TestAnswer_RunIDEndedRun (design note 4 c): after finishRun with completed and failed and after
// cancelRun with the adapter's confirmation, an answer with run_id is 409 conflict run_not_active
// and stores nothing; the same client_msg_id without run_id is then a normal message (201).
func TestAnswer_RunIDEndedRun(t *testing.T) {
	t.Parallel()
	a := newAgentWorld(t)
	for _, end := range []string{"completed", "failed", "cancelled"} {
		r, th, _ := a.startedRun()
		switch end {
		case "completed":
			a.wantOK(a.finish("lotse", r.ID, `"state":"completed"`))
		case "failed":
			a.wantOK(a.finish("lotse", r.ID, `"state":"failed","reason":"runtime_error"`))
		case "cancelled":
			a.want(a.cancel("alex", r.ID), http.StatusAccepted, "")
			a.wantOK(a.finish("lotse", r.ID, `"state":"cancelled","reason":"user_cancelled"`))
		}
		if got := a.lifecycle(r.ID); got.State != end {
			t.Fatalf("run %+v, want %s", got, end)
		}
		id, last := ids.New(), a.lastSeq(th)
		a.wantReason(a.answer(th, r.ID, id, "Late answer"), http.StatusConflict, "conflict", "run_not_active")
		if a.lastSeq(th) != last || a.count(`SELECT count(*) FROM messages WHERE client_msg_id = $1`, id.String()) != 0 {
			t.Fatalf("%s: a refused answer stored something", end)
		}
		m := a.want(a.send("lotse", th, messageBodyWith(id, "Late answer")), http.StatusCreated, "")
		if m["origin"] != "agent" {
			t.Fatalf("%s: %v", end, m)
		}
	}
}

// TestAnswer_ReplayAfterRunEnded (09 15 "Answer lost while sending"): an answer stored with run_id
// while the run was running is replayed with 200 and Idempotent-Replay after the run ended, and
// no second event is written.
func TestAnswer_ReplayAfterRunEnded(t *testing.T) {
	t.Parallel()
	a := newAgentWorld(t)
	r, th, _ := a.startedRun()
	id := ids.New()
	first := a.want(a.answer(th, r.ID, id, "Result"), http.StatusCreated, "")
	a.wantOK(a.finish("lotse", r.ID, `"state":"completed"`))
	seq := a.maxSeq()
	resp := a.answer(th, r.ID, id, "Result")
	again := a.wantOK(resp)
	if resp.Header.Get("Idempotent-Replay") != "true" || again["id"] != first["id"] {
		t.Fatalf("replay %v %v", resp.Header, again)
	}
	if n := len(a.eventsAfter(seq)); n != 0 {
		t.Fatalf("the replay wrote %d events", n)
	}
}

// TestAnswer_FirstMessageRunIDRejected (design note 5): createThread with first_message.run_id is
// 400 validation_failed on /first_message/run_id with run_mismatch, also for Lotse's active run;
// no thread is created.
func TestAnswer_FirstMessageRunIDRejected(t *testing.T) {
	t.Parallel()
	a := newAgentWorld(t)
	r, _, _ := a.startedRun()
	threads := a.count(`SELECT count(*) FROM threads`)
	p := a.want(a.do("lotse", http.MethodPost, "/api/v1/channels/"+a.reports.String()+"/threads",
		`{"title":"Report","first_message":`+messageBody("Report", withRun(r.ID))+`}`, key()...), http.StatusBadRequest, "validation_failed")
	errs, _ := p["errors"].([]any)
	f, _ := errs[0].(map[string]any)
	if f["pointer"] != "/first_message/run_id" || f["code"] != "run_mismatch" {
		t.Fatalf("problem %v", p)
	}
	if a.count(`SELECT count(*) FROM threads`) != threads {
		t.Fatal("a thread was created")
	}
}

// TestAnswer_ConcurrentWithFinish (design note 7): an answer with run_id and finishRun(completed)
// released together either store the answer before the end event (its message.created has the
// lower seq) or refuse it with 409 run_not_active without storing it; no deadlock or other error
// reaches the client.
func TestAnswer_ConcurrentWithFinish(t *testing.T) {
	t.Parallel()
	a := newAgentWorld(t)
	const rounds = 8
	stored, refused := 0, 0
	for range rounds {
		r, th, _ := a.startedRun()
		id := ids.New()
		var ans, fin *httpservertest.Response
		var wg sync.WaitGroup
		gate := make(chan struct{})
		wg.Add(2)
		go func() { defer wg.Done(); <-gate; ans = a.answer(th, r.ID, id, "Racing answer") }()
		go func() { defer wg.Done(); <-gate; fin = a.finish("lotse", r.ID, `"state":"completed"`) }()
		close(gate)
		wg.Wait()
		if fin.StatusCode != http.StatusOK {
			t.Fatalf("finish %d %s", fin.StatusCode, fin.Body)
		}
		end := a.count(`SELECT max(seq) FROM event_log WHERE type = 'run.state_changed' AND resource_id = $1`, r.ID)
		switch ans.StatusCode {
		case http.StatusCreated:
			stored++
			msg := a.count(`SELECT seq FROM event_log WHERE type = 'message.created' AND client_msg_id = $1`, id.String())
			if msg >= end {
				t.Fatalf("the answer (seq %d) committed after the end event (seq %d)", msg, end)
			}
		case http.StatusConflict:
			refused++
			a.wantReason(ans, http.StatusConflict, "conflict", "run_not_active")
			if a.count(`SELECT count(*) FROM messages WHERE client_msg_id = $1`, id.String()) != 0 {
				t.Fatal("a refused answer was stored")
			}
		default:
			t.Fatalf("answer %d %s", ans.StatusCode, ans.Body)
		}
	}
	t.Logf("%d answers stored before the end, %d refused after it", stored, refused)
}
