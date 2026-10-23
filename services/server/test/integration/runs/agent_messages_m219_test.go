// SPDX-License-Identifier: Apache-2.0

package runs_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
)

// agentWorld is the world of the brief M2-19 ("Integration"): the world of 07 section 10 without
// the grant G-5, so Lotse writes in Support only through the operator rule unless a test grants
// more; Support/Allgemein with free_response and allow_mentioned_agents, Support/Eskalation with
// auto_thread off; the forum Support/Reports with operator mode none and a grant write on it for
// Lotse; the forum Support/Bulletin that inherits Lotse; the forum Marketing/Ideas; Texter with a
// grant write on Support/Allgemein for the guest cases.
type agentWorld struct {
	*world
	reports, bulletin, ideas ids.ID
	// texterGrant is Texter's grant write on Support/Allgemein.
	texterGrant ids.ID
}

// newAgentWorld builds the agentWorld through the API.
func newAgentWorld(t *testing.T) *agentWorld {
	t.Helper()
	w := newWorld(t)
	// The instance row of bothy migrate up (catch-up reads head and epoch).
	w.sql(`INSERT INTO instance (id, instance_id, schema_version, sync_epoch) VALUES (1, $1, 1, $2) ON CONFLICT (id) DO NOTHING`,
		ids.New().String(), ids.New().String())
	w.wantOK(w.do("owner", http.MethodPost, "/api/v1/grants/"+w.g5.String()+"/revoke", `{"reason":"Operator rule only"}`, key()...))
	w.behavior(w.allgemein, `"free_response":true,"allow_mentioned_agents":true`)
	w.behavior(w.eskalation, `"auto_thread":false`)
	a := &agentWorld{world: w}
	a.reports = w.channel(w.support, "forum", "Reports")
	w.wantOK(w.setChannelOperator(a.reports, `{"mode":"none","reason":"Report channel"}`))
	w.grant(`"subject_kind":"agent","subject_id":"` + w.lotse.String() + `","target_kind":"channel","target_id":"` + a.reports.String() +
		`","level":"write","reason":"Reports of Lotse"`)
	a.bulletin = w.channel(w.support, "forum", "Bulletin")
	a.ideas = w.channel(w.marketing, "forum", "Ideas")
	a.texterGrant = w.grant(`"subject_kind":"agent","subject_id":"` + w.texter.String() + `","target_kind":"channel","target_id":"` +
		w.allgemein.String() + `","level":"write","reason":"Guest in Allgemein"`)
	return a
}

// messageBody is a MessageCreate body with a fresh client_msg_id, text and the extra members.
func messageBody(text string, extra ...string) string {
	return messageBodyWith(ids.New(), text, extra...)
}

// messageBodyWith is messageBody with the client_msg_id id.
func messageBodyWith(id ids.ID, text string, extra ...string) string {
	c, _ := json.Marshal(text)
	b := `{"client_msg_id":"` + id.String() + `","content_md":` + string(c)
	for _, e := range extra {
		b += "," + e
	}
	return b + "}"
}

// send posts body as name into thread.
func (a *agentWorld) send(name string, thread ids.ID, body string) *httpservertest.Response {
	return a.do(name, http.MethodPost, "/api/v1/threads/"+thread.String()+"/messages", body, key()...)
}

// say posts text as name into thread with the extra members and expects 201.
func (a *agentWorld) say(name string, thread ids.ID, text string, extra ...string) map[string]any {
	a.t.Helper()
	return a.want(a.send(name, thread, messageBody(text, extra...)), http.StatusCreated, "")
}

// thread creates a thread of kind thread in the text channel: Lotse writes its parent into the
// main thread (an agent message never triggers, so free_response opens no run), then the owner
// creates the thread on it.
func (a *agentWorld) thread(channel ids.ID) ids.ID {
	a.t.Helper()
	parent := a.say("lotse", a.mainThread(channel), "Topic")
	m := a.want(a.do("owner", http.MethodPost, "/api/v1/channels/"+channel.String()+"/threads",
		`{"parent_message_id":"`+parent["id"].(string)+`"}`, key()...), http.StatusCreated, "")
	return ids.MustParse(m["thread"].(map[string]any)["id"].(string))
}

// report creates a report thread as Lotse in the forum with title and the first message; it
// returns the 201 body.
func (a *agentWorld) report(forum ids.ID, title, text string) map[string]any {
	a.t.Helper()
	return a.want(a.do("lotse", http.MethodPost, "/api/v1/channels/"+forum.String()+"/threads",
		`{"title":"`+title+`","first_message":`+messageBody(text)+`}`, key()...), http.StatusCreated, "")
}

// threadOf returns the thread id of a ThreadCreated body.
func threadOf(m map[string]any) ids.ID {
	return ids.MustParse(m["thread"].(map[string]any)["id"].(string))
}

// startedRun opens a run of Lotse for Alex in a new thread of Allgemein, delivers and starts it;
// it returns the run, its thread and the triggering message.
func (a *agentWorld) startedRun() (runRow, ids.ID, string) {
	a.t.Helper()
	r, th, m := a.queuedRun("alex", a.allgemein, "lotse")
	a.wantOK(a.start("lotse", r.ID, a.deliver(r.ID)))
	return r, th, m
}

// queuedRun opens a run of the agent handle for the person name in a new thread of the text
// channel: the person mentions the agent there (Lotse as fixed operator answers without a
// mention too).
func (a *agentWorld) queuedRun(name string, channel ids.ID, handle string) (runRow, ids.ID, string) {
	a.t.Helper()
	th := a.thread(channel)
	m := a.say(name, th, "@"+handle+" please look at the export")
	r, ok := a.runOf(m["id"].(string))
	if !ok {
		a.t.Fatal("the message opened no run")
	}
	return r, th, m["id"].(string)
}

// runCount returns the number of runs.
func (a *agentWorld) runCount() int64 { return a.count(`SELECT count(*) FROM runs`) }

// followups returns the rows of run_followups, 0 while the table does not exist (M2-11).
func (a *agentWorld) followups() int64 {
	if a.text(`SELECT coalesce(to_regclass('run_followups')::text, '')`) == "" {
		return 0
	}
	return a.count(`SELECT count(*) FROM run_followups`)
}

// noTriggerWork fails when anything of a trigger happened after seq: a new run (runsBefore is
// the count), agent.invocation, message.updated, a follow-up, a change of the run active (state,
// version) or a trigger_state on the message m; with newThread false also thread.created (an
// auto thread).
func (a *agentWorld) noTriggerWork(name string, seq, runsBefore int64, active *runRow, m map[string]any, newThread bool) {
	a.t.Helper()
	if m["origin"] != "agent" {
		a.t.Fatalf("%s: origin %v", name, m["origin"])
	}
	if _, ok := m["trigger_state"]; ok {
		a.t.Fatalf("%s: trigger_state in %v", name, m)
	}
	if n := a.runCount(); n != runsBefore {
		a.t.Fatalf("%s: %d runs, want %d", name, n, runsBefore)
	}
	for _, e := range a.eventsAfter(seq) {
		if e.Type == "agent.invocation" || e.Type == "message.updated" || (e.Type == "thread.created" && !newThread) {
			a.t.Fatalf("%s: event %s after an agent message", name, e.Type)
		}
	}
	if n := a.count(`SELECT count(*) FROM messages WHERE id = $1 AND trigger_state IS NULL`, m["id"].(string)); n != 1 {
		a.t.Fatalf("%s: the stored row has a trigger_state", name)
	}
	if a.followups() != 0 {
		a.t.Fatalf("%s: a follow-up", name)
	}
	if active != nil {
		if got := a.runsWhere(`id = $1`, active.ID)[0]; got.State != active.State || got.Version != active.Version {
			a.t.Fatalf("%s: the active run changed: %+v, was %+v", name, got, *active)
		}
	}
}

// TestAG01_AgentMessagesNeverTrigger (AG-01 for origin agent, 07 SR4, T-AI-07): no agent
// message creates a run, a follow-up, a trigger_state or an agent.invocation, with or without
// mentions, in a channel with free_response and in a thread with an active run.
func TestAG01_AgentMessagesNeverTrigger(t *testing.T) {
	t.Parallel()
	a := newAgentWorld(t)
	main := a.mainThread(a.allgemein)
	active, th, _ := a.startedRun()
	active = a.runsWhere(`id = $1`, active.ID)[0]
	cases := []struct {
		name, handle string
		thread       ids.ID
		text         string
	}{
		{"Lotse without mention in a free channel", "lotse", main, "Status report"},
		{"Lotse mentions itself", "lotse", main, "@lotse look at this"},
		{"Lotse mentions Texter", "lotse", main, "@texter please draft a text"},
		{"Texter as guest mentions Lotse", "texter", main, "@lotse please check"},
		{"Lotse in the thread of its running run", "lotse", th, "Working on it"},
		{"Texter in the thread of a run of Lotse", "texter", th, "@texter @lotse what about this"},
		{"Lotse in the main thread with auto_thread", "lotse", main, "@lotse new topic"},
	}
	for _, c := range cases {
		seq, before := a.maxSeq(), a.runCount()
		m := a.say(c.handle, c.thread, c.text)
		a.noTriggerWork(c.name, seq, before, &active, m, false)
	}
}

// TestAG26_AgentWithoutWrite (AG-26, AZ-M28, T-AI-04): an agent without read right gets 404
// identical to an unknown id for messages and report threads, with read right only 403
// forbidden level_insufficient; nothing is stored; the denials are audited aggregated; in an
// archived channel even the operator gets 403 channel_read_only channel_archived.
func TestAG26_AgentWithoutWrite(t *testing.T) {
	t.Parallel()
	a := newAgentWorld(t)
	kampagnen := a.mainThread(a.kampagnen)
	messages := a.count(`SELECT count(*) FROM messages`)
	threads := a.count(`SELECT count(*) FROM threads`)
	createIn := func(channel ids.ID) *httpservertest.Response {
		return a.do("lotse", http.MethodPost, "/api/v1/channels/"+channel.String()+"/threads",
			`{"title":"Report","first_message":`+messageBody("Weekly report")+`}`, key()...)
	}
	same := func(name string, x, y *httpservertest.Response) {
		t.Helper()
		if x.StatusCode != http.StatusNotFound || y.StatusCode != http.StatusNotFound {
			t.Fatalf("%s: %d and %d, want 404", name, x.StatusCode, y.StatusCode)
		}
		if normalized(x.Body) != normalized(y.Body) {
			t.Fatalf("%s: bodies differ:\n%s\n%s", name, x.Body, y.Body)
		}
		for _, h := range []string{"Content-Type", "Cache-Control", "Etag", "Location", "Retry-After"} {
			if x.Header.Get(h) != y.Header.Get(h) {
				t.Fatalf("%s: header %s differs: %q and %q", name, h, x.Header.Get(h), y.Header.Get(h))
			}
		}
		if len(x.Header) != len(y.Header) {
			t.Fatalf("%s: header sets differ: %v and %v", name, x.Header, y.Header)
		}
	}
	same("createMessage", a.send("lotse", kampagnen, messageBody("Hello")), a.send("lotse", ids.New(), messageBody("Hello")))
	same("createThread", createIn(a.ideas), createIn(ids.New()))
	a.grant(`"subject_kind":"agent","subject_id":"` + a.lotse.String() + `","target_kind":"space","target_id":"` + a.marketing.String() +
		`","level":"read","reason":"Lotse reads Marketing"`)
	a.wantReason(a.send("lotse", kampagnen, messageBody("Hello")), http.StatusForbidden, "forbidden", "level_insufficient")
	a.wantReason(createIn(a.ideas), http.StatusForbidden, "forbidden", "level_insufficient")
	if a.count(`SELECT count(*) FROM messages`) != messages || a.count(`SELECT count(*) FROM threads`) != threads {
		t.Fatal("a refused request stored something")
	}
	must(t, a.rec.Aggregator().Flush(context.Background()))
	for _, action := range []string{"message.create", "thread.create"} {
		for _, reason := range []string{"no_grant", "level_insufficient"} {
			if n := a.count(`SELECT count(*) FROM audit_events WHERE action = $1 AND decision = 'deny' AND deny_reason = $2
				AND actor_principal_id = $3`, action, reason, a.lotse.String()); n != 1 {
				t.Errorf("%s %s: %d deny entries of Lotse, want 1", action, reason, n)
			}
		}
	}
	a.wantOK(a.do("owner", http.MethodPost, "/api/v1/channels/"+a.allgemein.String()+"/archive", `{"reason":"Done"}`, key()...))
	a.wantReason(a.send("lotse", a.mainThread(a.allgemein), messageBody("Hello")), http.StatusForbidden, "channel_read_only", "channel_archived")
}

// TestAgentWritesWithGrantOrAsOperator (AZ-M27, AZ-M29, 05 2.3 point 4): Lotse writes through the
// operator rule and through its grant; a guest run of Texter lets Texter answer with run_id until
// its grant is revoked; after the operator of Eskalation changes, Lotse's answer in the main
// thread there is 404, because a main thread has no fixed operator (R-DM-21).
func TestAgentWritesWithGrantOrAsOperator(t *testing.T) {
	t.Parallel()
	a := newAgentWorld(t)
	a.say("lotse", a.mainThread(a.allgemein), "Through the operator rule")
	a.report(a.reports, "Daily report", "Through the grant")

	// The owner mentions Texter: a mention resolves only for an author who may read the agent
	// (agent.read, 07 4.6), and Alex reads no channel Texter operates.
	guest, th, _ := a.queuedRun("owner", a.allgemein, "texter")
	if guest.Agent != a.texter.String() {
		t.Fatalf("the guest run belongs to %s", guest.Agent)
	}
	a.wantOK(a.start("texter", guest.ID, a.deliver(guest.ID)))
	a.say("texter", th, "Guest answer", `"run_id":"`+guest.ID+`"`)
	a.wantOK(a.do("owner", http.MethodPost, "/api/v1/grants/"+a.texterGrant.String()+"/revoke", `{"reason":"Guest done"}`, key()...))
	a.want(a.send("texter", th, messageBody("Late guest answer", `"run_id":"`+guest.ID+`"`)), http.StatusNotFound, "not_found")

	main := a.mainThread(a.eskalation)
	m := a.say("alex", main, "@lotse the escalation is open")
	r, ok := a.runOf(m["id"].(string))
	if !ok || r.Thread != main.String() {
		t.Fatalf("no run in the main thread of Eskalation: %+v", r)
	}
	a.wantOK(a.start("lotse", r.ID, a.deliver(r.ID)))
	a.wantOK(a.setChannelOperator(a.eskalation, `{"mode":"agent","agent_id":"`+a.texter.String()+`","reason":"Texter takes over"}`))
	a.want(a.send("lotse", main, messageBody("Answer", `"run_id":"`+r.ID+`"`)), http.StatusNotFound, "not_found")
}

// TestReport_NewThreadPerReport (09 14 point 1, 09 12 point 1): a report as a new thread of the
// forum Reports: title_source auto, no operator, created by Lotse, the first message of origin
// agent at thread_seq 1, thread.created then message.created, no run, and a replay of the same
// first_message.client_msg_id answers 200 with Idempotent-Replay.
func TestReport_NewThreadPerReport(t *testing.T) {
	t.Parallel()
	a := newAgentWorld(t)
	seq, runs := a.maxSeq(), a.runCount()
	id := ids.New()
	body := `{"title":"Nightly build","first_message":` + messageBodyWith(id, "Nightly build passed") + `}`
	m := a.want(a.do("lotse", http.MethodPost, "/api/v1/channels/"+a.reports.String()+"/threads", body, key()...), http.StatusCreated, "")
	th := m["thread"].(map[string]any)
	first := m["first_message"].(map[string]any)
	if th["title_source"] != "auto" || th["created_by"] != a.lotse.String() || th["title"] != "Nightly build" {
		t.Fatalf("thread %v", th)
	}
	if _, ok := th["operator_agent_id"]; ok {
		t.Fatalf("operator in a channel without operator: %v", th)
	}
	if first["origin"] != "agent" || first["thread_seq"] != float64(1) {
		t.Fatalf("first message %v", first)
	}
	if got := types(a.eventsAfter(seq)); strings.Join(got, ",") != "thread.created,message.created" {
		t.Fatalf("events %v", got)
	}
	a.noTriggerWork("report", seq, runs, nil, first, true)
	replay := a.do("lotse", http.MethodPost, "/api/v1/channels/"+a.reports.String()+"/threads", body, key()...)
	got := a.wantOK(replay)
	if replay.Header.Get("Idempotent-Replay") != "true" || got["thread"].(map[string]any)["id"] != th["id"] {
		t.Fatalf("replay %v %v", replay.Header, got)
	}
}

// TestReport_FixedThread (09 14 point 2): three reports into one thread of the forum Bulletin are
// three messages with consecutive thread_seq and no run.
func TestReport_FixedThread(t *testing.T) {
	t.Parallel()
	a := newAgentWorld(t)
	runs := a.runCount()
	created := a.report(a.bulletin, "Status", "Report 1")
	th := threadOf(created)
	if created["thread"].(map[string]any)["operator_agent_id"] != a.lotse.String() {
		t.Fatalf("thread %v", created["thread"])
	}
	seqs := []any{created["first_message"].(map[string]any)["thread_seq"]}
	for _, text := range []string{"Report 2", "Report 3"} {
		seqs = append(seqs, a.say("lotse", th, text)["thread_seq"])
	}
	if seqs[0] != float64(1) || seqs[1] != float64(2) || seqs[2] != float64(3) {
		t.Fatalf("thread_seq %v", seqs)
	}
	if a.runCount() != runs {
		t.Fatal("a report opened a run")
	}
}

// TestReport_HumanReplyTriggersNormally (09 14 point 5): a human answer in a report thread is
// decided like any message: in Bulletin Lotse is the fixed operator and runs; in Reports, a
// channel without operator, nothing triggers, and a mention of Lotse fails with
// agent_not_allowed_here because guests are not allowed there.
func TestReport_HumanReplyTriggersNormally(t *testing.T) {
	t.Parallel()
	a := newAgentWorld(t)
	bulletin := threadOf(a.report(a.bulletin, "Status", "All systems normal"))
	m := a.say("alex", bulletin, "Why was the backup late")
	if r, ok := a.runOf(m["id"].(string)); !ok || r.Agent != a.lotse.String() || r.Thread != bulletin.String() {
		t.Fatalf("no run of Lotse in the report thread: %+v", r)
	}
	reports := threadOf(a.report(a.reports, "Nightly", "Nightly build passed"))
	plain := a.say("alex", reports, "Thanks")
	if _, ok := a.runOf(plain["id"].(string)); ok {
		t.Fatal("a message in a channel without operator opened a run")
	}
	if st, _, _ := a.triggerOf(plain["id"].(string)); st != "" {
		t.Fatalf("trigger_state %s", st)
	}
	mention := a.say("alex", reports, "@lotse what failed")
	if st, reason, _ := a.triggerOf(mention["id"].(string)); st != "failed" || reason != "agent_not_allowed_here" {
		t.Fatalf("trigger_state %s %s", st, reason)
	}
}

// TestAgentMessage_CannotForgeOriginOrKind (19 SEC-OUT-3, SEC-IN-1): origin, system_code, kind
// system and kind run cannot be sent; nothing is stored.
func TestAgentMessage_CannotForgeOriginOrKind(t *testing.T) {
	t.Parallel()
	a := newAgentWorld(t)
	main := a.mainThread(a.allgemein)
	before := a.count(`SELECT count(*) FROM messages`)
	for _, extra := range []string{`"origin":"human"`, `"system_code":"thread.reopened"`, `"kind":"system"`, `"kind":"run"`} {
		a.want(a.send("lotse", main, messageBody("Forged", extra)), http.StatusBadRequest, "validation_failed")
	}
	if a.count(`SELECT count(*) FROM messages`) != before {
		t.Fatal("a forged message was stored")
	}
}

// TestAgentMessage_ReopensDoneThread (M1-56 D6): an answer of Lotse in a closed thread reopens it:
// thread.reopened, the system line of Lotse with origin system, then Lotse's message; no run.
func TestAgentMessage_ReopensDoneThread(t *testing.T) {
	t.Parallel()
	a := newAgentWorld(t)
	th := a.thread(a.allgemein)
	a.wantOK(a.do("owner", http.MethodPost, "/api/v1/threads/"+th.String()+"/close", "", key()...))
	seq, runs := a.maxSeq(), a.runCount()
	m := a.say("lotse", th, "Reopening with news")
	evs := a.eventsAfter(seq)
	if got := strings.Join(types(evs), ","); got != "thread.reopened,message.created,message.created" {
		t.Fatalf("events %s", got)
	}
	var origin, code, author string
	must(t, a.db.QueryRow(context.Background(), `SELECT origin, coalesce(system_code, ''), author_principal_id::text FROM messages
		WHERE thread_id = $1 AND kind = 'system' ORDER BY thread_seq DESC LIMIT 1`, th.String()).Scan(&origin, &code, &author))
	if origin != "system" || code != "thread.reopened" || author != a.lotse.String() {
		t.Fatalf("system line %s %s %s", origin, code, author)
	}
	if evs[2].Resource != m["id"] {
		t.Fatalf("the last event is not Lotse's message: %+v", evs[2])
	}
	a.noTriggerWork("reopen", seq, runs, nil, m, false)
}
