// SPDX-License-Identifier: Apache-2.0

package runs_test

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
	"github.com/OWNER/bothy/services/server/internal/runs"
	runspg "github.com/OWNER/bothy/services/server/internal/runs/adapters/postgres"
	runsapp "github.com/OWNER/bothy/services/server/internal/runs/app"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// The helpers of the wake-up tests (AR-04, docs/spec/09-agents.md 5.8): the world of the runs
// tests with the wake-ups bound over the real repository and messaging, a recording scheduler of
// runs.wake_due (the world has no job runtime, so the tests run the job's use case directly) and
// recording counters.

// wakeCfg is the configuration of the wake-ups of the world: the defaults of 09 21, with small caps
// so the tests reach them.
var wakeCfg = runsapp.WakeConfig{
	MaxPending: 5, MaxWait: 7 * 24 * time.Hour, MinDelay: time.Minute, UsageWakeMax: 3, LabelMaxChars: 120, ChainMax: 2, IdleMax: 2,
}

// wakeJob is one scheduled runs.wake_due.
type wakeJob struct {
	Wake ids.ID
	At   time.Time
}

// wakeJobs records the scheduled jobs runs.wake_due (runsapp.WakeScheduler).
type wakeJobs struct {
	mu   sync.Mutex
	list []wakeJob
}

// ScheduleWakeDue implements runsapp.WakeScheduler.
func (j *wakeJobs) ScheduleWakeDue(_ context.Context, id ids.ID, at time.Time) error {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.list = append(j.list, wakeJob{Wake: id, At: at})
	return nil
}

// of returns the times the job of wake was scheduled at.
func (j *wakeJobs) of(wake string) []time.Time {
	j.mu.Lock()
	defer j.mu.Unlock()
	var out []time.Time
	for _, x := range j.list {
		if x.Wake.String() == wake {
			out = append(out, x.At)
		}
	}
	return out
}

// wakeCounts records the counters of the wake-ups (runsapp.WakeMetrics).
type wakeCounts struct {
	mu    sync.Mutex
	ended []string
	caps  []string
}

// WakeEnded implements runsapp.WakeMetrics.
func (c *wakeCounts) WakeEnded(_ context.Context, state domain.WakeState, reason domain.WakeReason) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ended = append(c.ended, string(state)+"/"+string(reason))
}

// AutonomyCapReached implements runsapp.WakeMetrics.
func (c *wakeCounts) AutonomyCapReached(_ context.Context, code string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.caps = append(c.caps, code)
}

// wakeWorld is the world of the runs tests with the wake-ups bound.
type wakeWorld struct {
	*world
	jobs   *wakeJobs
	counts *wakeCounts
}

// newWakeWorld returns the runs world with the wake-ups bound with cfg.
func newWakeWorld(t *testing.T, cfg runsapp.WakeConfig) *wakeWorld {
	t.Helper()
	w := runsWorld(t)
	ww := &wakeWorld{world: w, jobs: &wakeJobs{}, counts: &wakeCounts{}}
	must(t, w.runs.BindWakes(cfg, runsapp.WakeDeps{Repo: runspg.NewRepository(w.pool), Schedule: ww.jobs, Notes: w.msgs, Owners: w.az, Metrics: ww.counts}))
	w.rn.Handlers.BindWakes(w.runs.Wakes())
	return ww
}

// runOfPerson opens a run of Lotse for person in a new thread of Support/Allgemein, delivers and
// starts it; it returns the run.
func (w *wakeWorld) runOfPerson(person string) runRow {
	w.t.Helper()
	th := w.newThread(w.allgemein)
	m := w.post(person, th, "Please watch the build")
	r, ok := w.runOf(m["id"].(string))
	if !ok {
		w.t.Fatalf("the message of %s opened no run", person)
	}
	w.wantOK(w.start("lotse", r.ID, w.deliver(r.ID)))
	return r
}

// createWake sends createRunWake as the agent handle with body and a fresh Idempotency-Key.
func (w *wakeWorld) createWake(handle, run, body string) *httpservertest.Response {
	return w.do(handle, http.MethodPost, "/api/v1/runs/"+run+"/wakes", body, key()...)
}

// atWake creates an at wake-up of run as Lotse and returns its id (201).
func (w *wakeWorld) atWake(run, label string) string {
	w.t.Helper()
	m := w.want(w.createWake("lotse", run, `{"kind":"at","reason":"agent","label":"`+label+`"}`), http.StatusCreated, "")
	return m["id"].(string)
}

// prWake creates a pr_checks wake-up of run as Lotse and returns its id (201).
func (w *wakeWorld) prWake(run string) string {
	w.t.Helper()
	m := w.want(w.createWake("lotse", run, `{"kind":"pr_checks","reason":"agent","label":"CI of #12","condition":{"repository":"example/app","number":12,"head_commit":"abc1234"}}`),
		http.StatusCreated, "")
	return m["id"].(string)
}

// wakeRow is the part of a run_wakes row the tests read.
type wakeRow struct {
	State, Reason, Kind, WakeReason, Label, Result, FiredRun, FiredInto, Person string
	NotBefore, Deadline, Created                                                time.Time
	Ended                                                                       bool
	Version                                                                     int64
}

// wake reads the wake-up id.
func (w *wakeWorld) wake(id string) wakeRow {
	w.t.Helper()
	var r wakeRow
	must(w.t, w.db.QueryRow(context.Background(), `SELECT state, coalesce(state_reason, ''), kind, reason, label, result_text,
		coalesce(fired_run_id::text, ''), coalesce(fired_into_run_id::text, ''), triggered_by::text, not_before, deadline, created_at,
		ended_at IS NOT NULL, version FROM run_wakes WHERE id = $1`, id).
		Scan(&r.State, &r.Reason, &r.Kind, &r.WakeReason, &r.Label, &r.Result, &r.FiredRun, &r.FiredInto, &r.Person, &r.NotBefore, &r.Deadline,
			&r.Created, &r.Ended, &r.Version))
	return r
}

// notes returns the contents of the system notes (origin system, kind text) of thread, oldest first.
func (w *wakeWorld) notes(thread string) []string {
	w.t.Helper()
	rs, err := w.db.Query(context.Background(), `SELECT content_md FROM messages WHERE thread_id = $1 AND origin = 'system' AND kind = 'text'
		ORDER BY thread_seq`, thread)
	must(w.t, err)
	defer rs.Close()
	var out []string
	for rs.Next() {
		var s string
		must(w.t, rs.Scan(&s))
		out = append(out, s)
	}
	return out
}

// wakeEvents returns the payloads of run.wake_changed of thread, oldest first.
func (w *wakeWorld) wakeEvents(thread string) []map[string]any {
	w.t.Helper()
	return w.payloads(`SELECT payload::text FROM event_log WHERE type = 'run.wake_changed' AND thread_id = $1 ORDER BY seq`, thread)
}

// payloads returns the decoded JSON objects of query.
func (w *wakeWorld) payloads(query string, args ...any) []map[string]any {
	w.t.Helper()
	rs, err := w.db.Query(context.Background(), query, args...)
	must(w.t, err)
	defer rs.Close()
	var out []map[string]any
	for rs.Next() {
		var raw string
		must(w.t, rs.Scan(&raw))
		var m map[string]any
		must(w.t, json.Unmarshal([]byte(raw), &m))
		out = append(out, m)
	}
	return out
}

// audits counts the audit entries of action with decision and reason ("" for any reason).
func (w *wakeWorld) audits(action, decision, reason string) int64 {
	w.t.Helper()
	if reason == "" {
		return w.count(`SELECT count(*) FROM audit_events WHERE action = $1 AND decision = $2`, action, decision)
	}
	return w.count(`SELECT count(*) FROM audit_events WHERE action = $1 AND decision = $2 AND deny_reason = $3`, action, decision, reason)
}

// due runs the job runs.wake_due of wake now.
func (w *wakeWorld) due(wake string) {
	w.t.Helper()
	must(w.t, w.runs.WakeDue(context.Background(), ids.MustParse(wake)))
}

// deliverWakeInput prepares the delivery of the newest agent.wake_input to the run's agent and
// returns the outcome.
func (w *wakeWorld) deliverWakeInput(run string) runs.DeliveryOutcome {
	w.t.Helper()
	var payload string
	must(w.t, w.db.QueryRow(context.Background(), `SELECT payload::text FROM event_log WHERE type = 'agent.wake_input' AND resource_id = $1
		ORDER BY seq DESC LIMIT 1`, run).Scan(&payload))
	agent := w.text(`SELECT agent_id::text FROM runs WHERE id = $1`, run)
	out, err := w.runs.Prepare(context.Background(), runs.DeliveryInput{AgentID: ids.MustParse(agent), EventID: ids.New(), Type: "agent.wake_input",
		ResourceKind: "run", ResourceID: ids.MustParse(run), Payload: []byte(payload)})
	must(w.t, err)
	return out
}

// setAutonomous switches allow_autonomous_runs of channel as the owner.
func (w *wakeWorld) setAutonomous(channel ids.ID, allow bool) map[string]any {
	w.t.Helper()
	v := w.count(`SELECT version FROM channels WHERE id = $1`, channel.String())
	body := `{"allow_autonomous_runs":false}`
	if allow {
		body = `{"allow_autonomous_runs":true}`
	}
	return w.wantOK(w.do("owner", http.MethodPut, "/api/v1/channels/"+channel.String()+"/autonomous-runs", body, ifMatch(v)...))
}
