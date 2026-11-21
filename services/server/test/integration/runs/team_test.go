// SPDX-License-Identifier: Apache-2.0

package runs_test

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	runspg "github.com/OWNER/bothy/services/server/internal/runs/adapters/postgres"
	runsapp "github.com/OWNER/bothy/services/server/internal/runs/app"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// The team channels against the real server and database (FEAT-TEAM-CHANNEL-SERVER,
// docs/spec/09-agents.md 3.9): the owner operations over the REST API with their refusals, the
// copies messaging writes, the team runs and follow-ups, the delivery check for the sponsor and the
// stop. The world has no job runtime, so the tests run the use case of runs.team_input directly.

// teamJobs records the scheduled jobs runs.team_input (runsapp.TeamScheduler).
type teamJobs struct {
	mu   sync.Mutex
	list []ids.ID
}

// ScheduleTeamInput implements runsapp.TeamScheduler.
func (j *teamJobs) ScheduleTeamInput(_ context.Context, id ids.ID) error {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.list = append(j.list, id)
	return nil
}

// has reports whether the message was scheduled.
func (j *teamJobs) has(id string) bool {
	j.mu.Lock()
	defer j.mu.Unlock()
	for _, x := range j.list {
		if x.String() == id {
			return true
		}
	}
	return false
}

// teamWorld is the runs world with the team channels bound and a space Labs that only owners read:
// the team channel Team, the channel Lab-A that Lotse operates and Lab-B that Texter operates.
type teamWorld struct {
	*world
	jobs             *teamJobs
	labs             ids.ID
	team, labA, labB ids.ID
	teamMain         ids.ID
	codingID, review string
	codingW, reviewW ids.ID
}

func newTeamWorld(t *testing.T) *teamWorld {
	t.Helper()
	w := runsWorld(t)
	tw := &teamWorld{world: w, jobs: &teamJobs{}}
	must(t, w.runs.BindTeams(runsapp.TeamConfig{TurnBudget: 200, PingpongMax: 40, MaxMembers: 20, HandleMaxChars: 32},
		runsapp.TeamDeps{Repo: runspg.NewRepository(w.pool), Messages: w.msgs, Readers: w.az, Owners: w.az, Schedule: tw.jobs}))
	w.rn.Handlers.BindTeams(w.runs.Teams())
	tw.labs = w.space("Labs")
	tw.team, tw.labA, tw.labB = w.channel(tw.labs, "text", "Team"), w.channel(tw.labs, "text", "Lab-A"), w.channel(tw.labs, "text", "Lab-B")
	w.wantOK(w.setChannelOperator(tw.labA, `{"mode":"agent","agent_id":"`+w.lotse.String()+`","reason":"Lab A agent"}`))
	w.wantOK(w.setChannelOperator(tw.labB, `{"mode":"agent","agent_id":"`+w.texter.String()+`","reason":"Lab B agent"}`))
	tw.teamMain = w.mainThread(tw.team)
	return tw
}

// teamPath is the path of the team of the team channel plus suffix.
func (w *teamWorld) teamPath(suffix string) string {
	return "/api/v1/channels/" + w.team.String() + "/team" + suffix
}

// addMember adds agent with home under handle as the owner and returns the response.
func (w *teamWorld) addMember(agent, home ids.ID, handle string) map[string]any {
	w.t.Helper()
	return w.want(w.do("owner", http.MethodPost, w.teamPath("/members"),
		`{"agent_id":"`+agent.String()+`","home_channel_id":"`+home.String()+`","handle":"`+handle+`"}`), http.StatusCreated, "")
}

// setUp switches the team on as the owner and adds @coding (Lotse in Lab-A) and @review (Texter in
// Lab-B).
func (w *teamWorld) setUp() {
	w.t.Helper()
	w.wantOK(w.do("owner", http.MethodPut, w.teamPath(""), ""))
	c := w.addMember(w.lotse, w.labA, "coding")
	r := w.addMember(w.texter, w.labB, "review")
	w.codingID, w.review = c["id"].(string), r["id"].(string)
	w.codingW, w.reviewW = ids.MustParse(c["work_thread_id"].(string)), ids.MustParse(r["work_thread_id"].(string))
}

// runInput runs the use case of runs.team_input for the message.
func (w *teamWorld) runInput(message string) {
	w.t.Helper()
	must(w.t, w.runs.TeamInput(context.Background(), ids.MustParse(message)))
}

// messageIn is one message of a thread as the tests read it.
type messageIn struct {
	ID, Author, Origin, Content string
}

// messagesIn reads the text messages of thread that are no system notes, oldest first.
func (w *teamWorld) messagesIn(thread ids.ID) []messageIn {
	w.t.Helper()
	rs, err := w.db.Query(context.Background(), `SELECT id::text, author_principal_id::text, origin, content_md FROM messages
		WHERE thread_id = $1 AND kind = 'text' AND origin <> 'system' ORDER BY thread_seq`, thread)
	must(w.t, err)
	defer rs.Close()
	var out []messageIn
	for rs.Next() {
		var m messageIn
		must(w.t, rs.Scan(&m.ID, &m.Author, &m.Origin, &m.Content))
		out = append(out, m)
	}
	return out
}

// teamRuns reads the runs of thread with trigger kind team.
func (w *teamWorld) teamRuns(thread ids.ID) []runRow {
	return w.runsWhere(`thread_id = $1 AND trigger_kind = 'team'`, thread)
}

// TestTeam_OwnerOperations: only owners switch team mode on and add members (a person who cannot
// read the channel gets 404); the readers rule refuses a home channel others can read; the work
// thread lies in the home channel and the team lists its members.
func TestTeam_OwnerOperations(t *testing.T) {
	w := newTeamWorld(t)
	w.want(w.do("alex", http.MethodPut, w.teamPath(""), ""), http.StatusNotFound, "not_found")
	w.want(w.do("owner", http.MethodPut, "/api/v1/channels/"+w.projektA.String()+"/team", ""), http.StatusConflict, "conflict")
	w.setUp()
	// Allgemein is read by Alex and the group Leser, the team channel only by owners.
	m := w.want(w.do("owner", http.MethodPost, w.teamPath("/members"),
		`{"agent_id":"`+w.lotse.String()+`","home_channel_id":"`+w.allgemein.String()+`","handle":"support"}`), http.StatusConflict, "conflict")
	if m["reason"] != "team_readers_not_owners" {
		t.Fatalf("a home channel with other readers: %v", m)
	}
	m = w.want(w.do("owner", http.MethodPost, w.teamPath("/members"),
		`{"agent_id":"`+w.texter.String()+`","home_channel_id":"`+w.labA.String()+`","handle":"other"}`), http.StatusConflict, "conflict")
	if m["reason"] != "team_home_not_operated" {
		t.Fatalf("an agent that does not operate the home channel: %v", m)
	}
	if got := w.text(`SELECT channel_id::text FROM threads WHERE id = $1`, w.codingW); got != w.labA.String() {
		t.Fatalf("the work thread lies in %s", got)
	}
	team := w.wantOK(w.do("owner", http.MethodGet, w.teamPath(""), ""))
	if members := team["members"].([]any); len(members) != 2 || team["sponsor_id"] != w.owner.String() {
		t.Fatalf("team %v", team)
	}
	allowed := 0
	for _, a := range w.audits("team.update") {
		if a.Decision == "allow" {
			allowed++
		}
	}
	if allowed != 3 {
		t.Fatalf("%d allowed audit entries of team.update, want 3", allowed)
	}
}

// TestTeam_Scenario: the owner writes in the team channel, every member gets the message in its
// work thread as the owner's own message and a team run for the owner; a member's final reply
// that mentions the other member is mirrored into the team channel and reaches the other member,
// whose run is active, as a follow-up of the member's agent (origin agent) checked for the
// sponsor; the stop ends every run of the team; an agent's mention outside the team starts nothing.
func TestTeam_Scenario(t *testing.T) {
	w := newTeamWorld(t)
	w.setUp()
	owner := w.post("owner", w.teamMain, "@coding build feature X")
	if !w.jobs.has(owner["id"].(string)) {
		t.Fatal("the owner message was not scheduled")
	}
	if n := len(w.runsWhere(`triggering_message_id = $1`, owner["id"])); n != 0 {
		t.Fatalf("the acceptance started %d ordinary runs in the team channel", n)
	}
	w.runInput(owner["id"].(string))
	for _, th := range []ids.ID{w.codingW, w.reviewW} {
		msgs := w.messagesIn(th)
		if len(msgs) != 1 || msgs[0].Author != w.owner.String() || msgs[0].Origin != "human" || msgs[0].Content != "@coding build feature X" {
			t.Fatalf("inputs of %s: %+v", th, msgs)
		}
		rs := w.teamRuns(th)
		if len(rs) != 1 || rs[0].TriggeredBy != w.owner.String() || rs[0].Triggering != msgs[0].ID {
			t.Fatalf("team runs of %s: %+v", th, rs)
		}
	}
	inv := w.payloadOf(`SELECT payload::text FROM event_log WHERE type = 'agent.invocation' AND thread_id = $1`, w.codingW)
	team := inv["team"].(map[string]any)
	if inv["trigger_kind"] != "team" || team["handle"] != "coding" || team["addressed"] != true || team["source"] != "owner" {
		t.Fatalf("agent.invocation %v", inv)
	}
	// The repeated job copies nothing again.
	w.runInput(owner["id"].(string))
	if n := len(w.messagesIn(w.reviewW)); n != 1 {
		t.Fatalf("a repeated job copied again: %d", n)
	}

	coding, review := w.teamRuns(w.codingW)[0], w.teamRuns(w.reviewW)[0]
	w.wantOK(w.start("lotse", coding.ID, w.deliver(coding.ID)))
	w.wantOK(w.start("texter", review.ID, w.deliver(review.ID)))
	reply := w.want(w.do("lotse", http.MethodPost, "/api/v1/threads/"+w.codingW.String()+"/messages",
		`{"client_msg_id":"`+ids.New().String()+`","content_md":"@review built X on branch y, please review","run_id":"`+coding.ID+`"}`, key()...),
		http.StatusCreated, "")
	if !w.jobs.has(reply["id"].(string)) {
		t.Fatal("the final reply was not scheduled")
	}
	w.runInput(reply["id"].(string))
	mirror := w.messagesIn(w.teamMain)
	if last := mirror[len(mirror)-1]; last.Author != w.lotse.String() || last.Origin != "agent" || last.Content != "@review built X on branch y, please review" {
		t.Fatalf("mirror %+v", mirror)
	}
	in := w.messagesIn(w.reviewW)
	if len(in) != 2 || in[1].Author != w.lotse.String() || in[1].Origin != "agent" {
		t.Fatalf("inputs of review %+v", in)
	}
	if rs := w.teamRuns(w.reviewW); len(rs) != 1 {
		t.Fatalf("review got a second run instead of a follow-up: %+v", rs)
	}
	if state := w.text(`SELECT state FROM run_followups WHERE run_id = $1 AND message_id = $2`, review.ID, in[1].ID); state != "queued" {
		t.Fatalf("follow-up state %q", state)
	}
	payload, err := domain.FollowupPayload(ids.MustParse(review.ID), w.reviewW, ids.MustParse(in[1].ID))
	must(t, err)
	out, err := w.runs.Prepare(context.Background(), deliveryInput(w.texter, review.ID, "agent.followup", payload))
	if err != nil || !out.Send {
		t.Fatalf("the follow-up of a member input was not delivered: %+v %v", out, err)
	}
	if n := w.count(`SELECT count(*) FROM audit_events WHERE action = 'agent.invoke' AND decision = 'allow' AND actor_principal_id = $1`, w.owner); n < 3 {
		t.Fatalf("%d allowed team inputs audited for the sponsor, want 3", n)
	}

	stopped := w.wantOK(w.do("owner", http.MethodPost, w.teamPath("/stop"), ""))
	if ids := stopped["stopped_run_ids"].([]any); len(ids) != 2 {
		t.Fatalf("stopped %v", stopped)
	}
	if got := w.wantOK(w.do("owner", http.MethodGet, w.teamPath(""), "")); got["pause_reason"] != "stopped" {
		t.Fatalf("team after the stop %v", got)
	}

	// Outside a team channel an agent's mention of another agent never triggers (07 SR4).
	th := w.newThread(w.allgemein)
	before := w.count(`SELECT count(*) FROM runs`)
	w.want(w.do("lotse", http.MethodPost, "/api/v1/threads/"+th.String()+"/messages",
		`{"client_msg_id":"`+ids.New().String()+`","content_md":"@texter please take over"}`, key()...), http.StatusCreated, "")
	if after := w.count(`SELECT count(*) FROM runs`); after != before {
		t.Fatalf("an agent message started %d runs", after-before)
	}
}

// payloadOf returns the decoded JSON object of the first row of query.
func (w *teamWorld) payloadOf(query string, args ...any) map[string]any {
	w.t.Helper()
	var m map[string]any
	must(w.t, json.Unmarshal([]byte(w.text(query+` ORDER BY seq LIMIT 1`, args...)), &m))
	return m
}
