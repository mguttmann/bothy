// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"encoding/json"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/messaging"
	"github.com/OWNER/bothy/services/server/internal/runs"
	"github.com/OWNER/bothy/services/server/internal/runs/app"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// The team channels (docs/spec/09-agents.md 3.9, FEAT-TEAM-CHANNEL-SERVER) over the fakes of the
// app tests: the owner operations with their authz refusals, the acceptance hook, the job
// runs.team_input with the copies, the runs and follow-ups it starts, the brake, the stop and the
// checks before every delivery.

// teamRepo is the in-memory TeamRepository.
type teamRepo struct {
	mu      sync.Mutex
	teams   map[ids.ID]app.TeamRow
	members []app.TeamMemberRow
	inputs  map[ids.ID]app.TeamInputRow
	mirrors map[ids.ID]ids.ID
}

func newTeamRepo() *teamRepo {
	return &teamRepo{teams: map[ids.ID]app.TeamRow{}, inputs: map[ids.ID]app.TeamInputRow{}, mirrors: map[ids.ID]ids.ID{}}
}

func (r *teamRepo) UpsertTeam(_ context.Context, channel, sponsor ids.ID, at time.Time) (app.TeamRow, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	t, ok := r.teams[channel]
	if !ok {
		t = app.TeamRow{ChannelID: channel, CreatedAt: at}
	}
	t.SponsorID, t.Version = sponsor, t.Version+1
	r.teams[channel] = t
	return t, nil
}

func (r *teamRepo) Team(_ context.Context, channel ids.ID) (app.TeamRow, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	t, ok := r.teams[channel]
	return t, ok, nil
}

func (r *teamRepo) LockTeam(ctx context.Context, channel ids.ID) (app.TeamRow, bool, error) {
	return r.Team(ctx, channel)
}

func (r *teamRepo) DeleteTeam(_ context.Context, channel ids.ID) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, ok := r.teams[channel]
	delete(r.teams, channel)
	r.members = slices.DeleteFunc(r.members, func(m app.TeamMemberRow) bool { return m.ChannelID == channel })
	return ok, nil
}

func (r *teamRepo) SaveTeamBrake(_ context.Context, channel ids.ID, b domain.TeamBrake, _ time.Time) (app.TeamRow, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	t := r.teams[channel]
	t.Brake, t.Version = b, t.Version+1
	r.teams[channel] = t
	return t, nil
}

func (r *teamRepo) InsertTeamMember(_ context.Context, m app.TeamMemberRow) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.members = append(r.members, m)
	return nil
}

func (r *teamRepo) TeamMembers(_ context.Context, channel ids.ID) ([]app.TeamMemberRow, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []app.TeamMemberRow
	for _, m := range r.members {
		if m.ChannelID == channel {
			out = append(out, m)
		}
	}
	return out, nil
}

func (r *teamRepo) TeamMemberByWorkThread(_ context.Context, thread ids.ID) (app.TeamMemberRow, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, m := range r.members {
		if m.WorkThreadID == thread {
			return m, true, nil
		}
	}
	return app.TeamMemberRow{}, false, nil
}

func (r *teamRepo) DeleteTeamMember(_ context.Context, channel, member ids.ID) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := len(r.members)
	r.members = slices.DeleteFunc(r.members, func(m app.TeamMemberRow) bool { return m.ChannelID == channel && m.ID == member })
	return len(r.members) < n, nil
}

func (r *teamRepo) InsertTeamInput(_ context.Context, in app.TeamInputRow) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, x := range r.inputs {
		if x.SourceMessageID == in.SourceMessageID && x.MemberID == in.MemberID {
			return false, nil
		}
	}
	r.inputs[in.MessageID] = in
	return true, nil
}

func (r *teamRepo) TeamInput(_ context.Context, message ids.ID) (app.TeamInputRow, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	in, ok := r.inputs[message]
	return in, ok, nil
}

func (r *teamRepo) TeamInputDelivered(_ context.Context, source, member ids.ID) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, x := range r.inputs {
		if x.SourceMessageID == source && x.MemberID == member {
			return true, nil
		}
	}
	return false, nil
}

func (r *teamRepo) InsertTeamMirror(_ context.Context, source, mirror, _ ids.ID, _ time.Time) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.mirrors[source]; ok {
		return false, nil
	}
	r.mirrors[source] = mirror
	return true, nil
}

func (r *teamRepo) TeamMirrored(_ context.Context, source ids.ID) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, ok := r.mirrors[source]
	return ok, nil
}

// inputs returns the inputs of a member.
func (r *teamRepo) inputsOf(member ids.ID) []app.TeamInputRow {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []app.TeamInputRow
	for _, in := range r.inputs {
		if in.MemberID == member {
			out = append(out, in)
		}
	}
	return out
}

// teamCopy is one copy the fake messages wrote.
type teamCopy struct {
	cmd messaging.CopyCmd
	id  ids.ID
}

// teamMsgs is the fake TeamMessages: a copy becomes a trigger message of the world with the
// source's content; the notes are recorded.
type teamMsgs struct {
	w       *teamWorld
	mu      sync.Mutex
	content map[ids.ID]string
	copies  []teamCopy
	notes   []string
}

func (m *teamMsgs) CopyMessage(_ context.Context, cmd messaging.CopyCmd) (messaging.Copied, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	text, ok := m.content[cmd.SourceMessageID]
	if !ok {
		return messaging.Copied{}, errs.New(errs.CodeInternalError, "copy of an unknown message")
	}
	id := ids.New()
	info := m.w.threads.infos[cmd.ThreadID]
	origin, kind := "agent", "agent"
	if cmd.AuthorID == m.w.alex || cmd.AuthorID == m.w.sam {
		origin, kind = "human", "user"
	}
	m.w.msgFacts.mu.Lock()
	m.w.msgFacts.m[id] = messaging.TriggerMessage{
		ID: id, ThreadID: cmd.ThreadID, ChannelID: info.ChannelID, Origin: origin, Kind: "text",
		Author: messaging.PrincipalRef{ID: cmd.AuthorID, Kind: kind, Handle: "h", DisplayName: "H"},
	}
	m.w.msgFacts.mu.Unlock()
	m.content[id] = text
	m.copies = append(m.copies, teamCopy{cmd: cmd, id: id})
	return messaging.Copied{ID: id, ThreadSeq: 2, ContentMD: text}, nil
}

func (m *teamMsgs) AppendSystemNote(_ context.Context, thread, _ ids.ID, text string) (ids.ID, int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.notes = append(m.notes, text)
	return ids.New(), 1, nil
}

func (m *teamMsgs) copiesInto(thread ids.ID) []teamCopy {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []teamCopy
	for _, c := range m.copies {
		if c.cmd.ThreadID == thread {
			out = append(out, c)
		}
	}
	return out
}

// teamReaders is the fake Readers: per channel the readers, Alex by default.
type teamReaders struct {
	mu  sync.Mutex
	set map[ids.ID][]ids.ID
	dft []ids.ID
}

func (r *teamReaders) Readers(_ context.Context, channel ids.ID) ([]ids.ID, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if l, ok := r.set[channel]; ok {
		return slices.Clone(l), nil
	}
	return slices.Clone(r.dft), nil
}

// teamOwners is the fake owner role.
type teamOwners struct {
	mu  sync.Mutex
	set map[ids.ID]bool
}

func (o *teamOwners) IsActiveOwner(_ context.Context, p ids.ID) (bool, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.set[p], nil
}

// teamSched records the scheduled runs.team_input jobs.
type teamSched struct {
	mu   sync.Mutex
	list []ids.ID
}

func (s *teamSched) ScheduleTeamInput(_ context.Context, message, _ ids.ID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.list = append(s.list, message)
	return nil
}

// teamWorld is the app world with a team channel and two home channels: Lotse operates homeA,
// Texter homeB; Alex is the owner, Sam a person who is no owner.
type teamWorld struct {
	*world
	sam                    ids.ID
	team, teamMain         ids.ID
	homeA, mainA           ids.ID
	homeB, mainB           ids.ID
	trepo                  *teamRepo
	tmsgs                  *teamMsgs
	readers                *teamReaders
	owners                 *teamOwners
	sched                  *teamSched
	cfg                    app.TeamConfig
	coding, review         app.TeamMemberRow
	codingWork, reviewWork ids.ID
	// runOf is the run each reply of reply answers.
	runOf map[ids.ID]ids.ID
}

func newTeamWorld(t *testing.T, cfg app.TeamConfig) *teamWorld {
	t.Helper()
	w := &teamWorld{world: newWorld(t), sam: ids.New(), team: ids.New(), teamMain: ids.New(), homeA: ids.New(), mainA: ids.New(),
		homeB: ids.New(), mainB: ids.New(), trepo: newTeamRepo(), runOf: map[ids.ID]ids.ID{}, owners: &teamOwners{set: map[ids.ID]bool{}}, sched: &teamSched{}, cfg: cfg}
	w.owners.set[w.alex] = true
	w.readers = &teamReaders{set: map[ids.ID][]ids.ID{}, dft: []ids.ID{w.alex}}
	w.tmsgs = &teamMsgs{w: w, content: map[ids.ID]string{}}
	for _, c := range []struct{ ch, main, op ids.ID }{{w.team, w.teamMain, ids.ID{}}, {w.homeA, w.mainA, w.lotse}, {w.homeB, w.mainB, w.texter}} {
		w.channels[c.ch] = chat.TriggerChannel{ChannelID: c.ch, SpaceID: w.space, Kind: "text", EffectiveOperatorID: c.op, MainThreadID: c.main,
			RequireMention: true, AutoThread: true}
		w.threads.infos[c.main] = chat.ThreadInfo{ID: c.main, ChannelID: c.ch, SpaceID: w.space, ChannelKind: "text", Kind: "main", State: "open"}
	}
	w.bindTeams()
	return w
}

// bindTeams binds the team channels to the current use cases of the world.
func (w *teamWorld) bindTeams() {
	w.t.Helper()
	if err := w.svc.BindTeams(w.cfg, app.TeamDeps{Repo: w.trepo, Messages: w.tmsgs, Readers: w.readers, Owners: w.owners, Schedule: w.sched}); err != nil {
		w.t.Fatal(err)
	}
}

// defaultTeamConfig has the defaults of 09 21.
func defaultTeamConfig() app.TeamConfig {
	return app.TeamConfig{TurnBudget: 200, PingpongMax: 40, MaxMembers: 20, HandleMaxChars: 32}
}

// asUser returns a context with the person p.
func asUser(p ids.ID) context.Context {
	return actor.WithActor(context.Background(), actor.Actor{ID: p, Kind: actor.KindUser})
}

// setUp switches the team channel on as Alex and adds @coding (Lotse in homeA) and @review (Texter
// in homeB).
func (w *teamWorld) setUp() {
	w.t.Helper()
	teams := w.svc.Teams()
	if _, err := teams.Enable(asUser(w.alex), w.team); err != nil {
		w.t.Fatal(err)
	}
	c, err := teams.AddMember(asUser(w.alex), runs.AddTeamMember{ChannelID: w.team, AgentID: w.lotse, HomeChannelID: w.homeA, Handle: "coding"})
	if err != nil {
		w.t.Fatal(err)
	}
	r, err := teams.AddMember(asUser(w.alex), runs.AddTeamMember{ChannelID: w.team, AgentID: w.texter, HomeChannelID: w.homeB, Handle: "review"})
	if err != nil {
		w.t.Fatal(err)
	}
	w.codingWork, w.reviewWork = c.WorkThreadID, r.WorkThreadID
	members, _ := w.trepo.TeamMembers(context.Background(), w.team)
	w.coding, w.review = members[0], members[1]
}

// ownerMessage stores a message of author with text in the team channel's main thread, posts it
// through the acceptance and returns it.
func (w *teamWorld) ownerMessage(author ids.ID, text string) ids.ID {
	w.t.Helper()
	ev := w.posted(w.teamMain)
	ev.AuthorID = author
	w.msgFacts.m[ev.MessageID] = messaging.TriggerMessage{
		ID: ev.MessageID, ThreadID: w.teamMain, ChannelID: w.team, Origin: "human", Kind: "text",
		Author: messaging.PrincipalRef{ID: author, Kind: "user", Handle: "alex", DisplayName: "Alex"},
	}
	w.tmsgs.content[ev.MessageID] = text
	w.mustPost(ev)
	return ev.MessageID
}

// reply stores a final reply of the member's agent in its work thread with text, answering a new
// running run of the agent there that Alex triggered, and returns it; it is not posted through the
// acceptance.
func (w *teamWorld) reply(m app.TeamMemberRow, text string) ids.ID {
	return w.replyBy(m, text, w.alex)
}

// replyBy is reply for a run that person triggered.
func (w *teamWorld) replyBy(m app.TeamMemberRow, text string, person ids.ID) ids.ID {
	run := w.putRun(m.WorkThreadID, m.AgentID, domain.StateRunning)
	w.repo.mu.Lock()
	w.repo.runs[len(w.repo.runs)-1].TriggeredBy = person
	w.repo.mu.Unlock()
	id := w.replyTo(m, text)
	w.runOf[id] = run
	return id
}

// replyTo stores a final reply of the member's agent in its work thread with text.
func (w *teamWorld) replyTo(m app.TeamMemberRow, text string) ids.ID {
	id := ids.New()
	w.msgFacts.m[id] = messaging.TriggerMessage{
		ID: id, ThreadID: m.WorkThreadID, ChannelID: m.HomeChannelID, Origin: "agent", Kind: "text",
		Author: messaging.PrincipalRef{ID: m.AgentID, Kind: "agent", Handle: m.Handle, DisplayName: m.Handle},
	}
	w.tmsgs.content[id] = text
	return id
}

// input runs the job runs.team_input for the message without a principal.
func (w *teamWorld) input(message ids.ID) {
	w.t.Helper()
	if err := w.svc.TeamInput(context.Background(), message, w.runOf[message]); err != nil {
		w.t.Fatal(err)
	}
}

// runsIn returns the runs of the thread.
func (w *teamWorld) runsIn(thread ids.ID) []app.RunRow {
	var out []app.RunRow
	for _, r := range w.repo.runs {
		if r.ThreadID == thread {
			out = append(out, r)
		}
	}
	return out
}

// invocations returns the payloads of the agent.invocation events of the thread.
func (w *teamWorld) invocations(thread ids.ID) []map[string]any {
	var out []map[string]any
	for _, c := range w.j.list() {
		ev, ok := c.arg.(eventlog.Event)
		if c.what != "event" || !ok || ev.Type != string(authorize.EventAgentInvocation) || ev.ThreadID != thread {
			continue
		}
		var p map[string]any
		if err := json.Unmarshal(ev.Payload, &p); err != nil {
			w.t.Fatal(err)
		}
		out = append(out, p)
	}
	return out
}

// teamAudits returns the audit entries of agent.invoke with stage team and their actors.
func (w *teamWorld) teamAudits() ([]auditing.Entry, []actor.Actor) {
	es, as := w.audit.list()
	var oe []auditing.Entry
	var oa []actor.Actor
	for i, e := range es {
		if e.Action == string(authorize.ActionAgentInvoke) && e.Attrs["stage"].Data() == "team" {
			oe, oa = append(oe, e), append(oa, as[i])
		}
	}
	return oe, oa
}

// TestTeam_OwnerOperationsNeedTheOwnerAction: a person whom team.update or team.stop refuses can
// neither create a team channel, nor add a member, nor stop it; nothing is stored.
func TestTeam_OwnerOperationsNeedTheOwnerAction(t *testing.T) {
	t.Parallel()
	w := newTeamWorld(t, defaultTeamConfig())
	w.authz.decide = func(act authorize.Action, _ bool, _ authorize.Resource) decision {
		if act == authorize.ActionTeamUpdate || act == authorize.ActionTeamStop || act == authorize.ActionTeamRead {
			return decision{reason: "owner_only"}
		}
		return decision{allowed: true}
	}
	teams := w.svc.Teams()
	if _, err := teams.Enable(asUser(w.sam), w.team); errs.KindOf(err) != errs.KindForbidden {
		t.Fatalf("Enable by a non-owner: %v", err)
	}
	if len(w.trepo.teams) != 0 {
		t.Fatalf("a refused Enable stored a team: %+v", w.trepo.teams)
	}
	w.trepo.teams[w.team] = app.TeamRow{ChannelID: w.team, SponsorID: w.alex}
	_, err := teams.AddMember(asUser(w.sam), runs.AddTeamMember{ChannelID: w.team, AgentID: w.lotse, HomeChannelID: w.homeA, Handle: "coding"})
	if errs.KindOf(err) != errs.KindForbidden || len(w.trepo.members) != 0 {
		t.Fatalf("AddMember by a non-owner: %v, members %+v", err, w.trepo.members)
	}
	if _, err := teams.Stop(asUser(w.sam), w.team); errs.KindOf(err) != errs.KindForbidden {
		t.Fatalf("Stop by a non-owner: %v", err)
	}
	if _, err := teams.Get(asUser(w.sam), w.team); errs.KindOf(err) != errs.KindForbidden {
		t.Fatalf("Get by a non-owner: %v", err)
	}
}

// TestTeam_EnableRefusesChannelsThatCannotBeTeams: a DM, a forum, an archived and a read-only
// channel answer 409 with their reason.
func TestTeam_EnableRefusesChannelsThatCannotBeTeams(t *testing.T) {
	t.Parallel()
	w := newTeamWorld(t, defaultTeamConfig())
	forum, archived, ro := ids.New(), ids.New(), ids.New()
	w.channels[forum] = chat.TriggerChannel{ChannelID: forum, Kind: "forum"}
	w.channels[archived] = chat.TriggerChannel{ChannelID: archived, Kind: "text", Archived: true}
	w.channels[ro] = chat.TriggerChannel{ChannelID: ro, Kind: "text", ReadOnly: true}
	for ch, reason := range map[ids.ID]string{w.dmChannel: "dm_channel", forum: "channel_kind", archived: "channel_archived", ro: "channel_read_only"} {
		_, err := w.svc.Teams().Enable(asUser(w.alex), ch)
		if e, ok := errs.As(err); !ok || e.Code != errs.CodeConflict || e.Reason != reason {
			t.Errorf("Enable of %s: %v", reason, err)
		}
	}
}

// TestTeam_AddMemberRules: the handle rule, the team mode, the operator of the home channel, the
// readers rule and the uniqueness of handle and pair.
func TestTeam_AddMemberRules(t *testing.T) {
	t.Parallel()
	w := newTeamWorld(t, defaultTeamConfig())
	teams := w.svc.Teams()
	add := func(agent, home ids.ID, handle string) error {
		_, err := teams.AddMember(asUser(w.alex), runs.AddTeamMember{ChannelID: w.team, AgentID: agent, HomeChannelID: home, Handle: handle})
		return err
	}
	reasonOf := func(err error) string {
		if e, ok := errs.As(err); ok {
			return e.Reason
		}
		return ""
	}
	if err := add(w.lotse, w.homeA, "Coding"); errs.KindOf(err) != errs.KindBadRequest {
		t.Fatalf("an upper-case handle: %v", err)
	}
	if err := add(w.lotse, w.homeA, "coding"); reasonOf(err) != "team_not_enabled" {
		t.Fatalf("a channel without team mode: %v", err)
	}
	if _, err := teams.Enable(asUser(w.alex), w.team); err != nil {
		t.Fatal(err)
	}
	if err := add(w.texter, w.homeA, "coding"); reasonOf(err) != "team_home_not_operated" {
		t.Fatalf("an agent that does not operate the home channel: %v", err)
	}
	w.readers.set[w.homeA] = []ids.ID{w.alex, w.sam}
	if err := add(w.lotse, w.homeA, "coding"); reasonOf(err) != domain.TeamRefusalReaders {
		t.Fatalf("a home channel Sam can read and the team channel not: %v", err)
	}
	delete(w.readers.set, w.homeA)
	if err := add(w.lotse, w.team, "coding"); reasonOf(err) == "" {
		t.Fatal("the team channel itself is no home channel")
	}
	if err := add(w.lotse, w.homeA, "coding"); err != nil {
		t.Fatal(err)
	}
	if err := add(w.texter, w.homeB, "coding"); reasonOf(err) != "team_handle_taken" {
		t.Fatalf("a taken handle: %v", err)
	}
	if err := add(w.lotse, w.homeA, "coding2"); reasonOf(err) != "team_member_exists" {
		t.Fatalf("the same agent and home twice: %v", err)
	}
	if len(w.trepo.members) != 1 || w.trepo.members[0].WorkThreadID.IsZero() {
		t.Fatalf("members %+v", w.trepo.members)
	}
	info := w.threads.infos[w.trepo.members[0].WorkThreadID]
	if info.ChannelID != w.homeA {
		t.Fatalf("the work thread lies in %v, want the home channel", info.ChannelID)
	}
}

// TestTeam_OwnerMessageReachesEveryMember: an owner's message in the team channel triggers no
// ordinary run, schedules the job, and the job copies it into every work thread as the owner's
// message and starts a team run there for the owner; the mentioned member is addressed.
func TestTeam_OwnerMessageReachesEveryMember(t *testing.T) {
	t.Parallel()
	w := newTeamWorld(t, defaultTeamConfig())
	w.setUp()
	msg := w.ownerMessage(w.alex, "@coding build feature X")
	if len(w.repo.runs) != 0 || w.j.count("authz:agent.invoke:request") != 0 {
		t.Fatalf("the acceptance decided a team channel message: runs %d", len(w.repo.runs))
	}
	if !slices.Equal(w.sched.list, []ids.ID{msg}) {
		t.Fatalf("scheduled %v, want the message", w.sched.list)
	}
	w.input(msg)
	for _, m := range []app.TeamMemberRow{w.coding, w.review} {
		copies := w.tmsgs.copiesInto(m.WorkThreadID)
		if len(copies) != 1 || copies[0].cmd.AuthorID != w.alex {
			t.Fatalf("%s: copies %+v, want one by the owner", m.Handle, copies)
		}
		rs := w.runsIn(m.WorkThreadID)
		if len(rs) != 1 || rs[0].TriggerKind != "team" || rs[0].TriggeredBy != w.alex || rs[0].TriggeringMessageID != copies[0].id ||
			rs[0].AgentID != m.AgentID {
			t.Fatalf("%s: runs %+v", m.Handle, rs)
		}
		inv := w.invocations(m.WorkThreadID)
		if len(inv) != 1 || inv[0]["trigger_kind"] != "team" || inv[0]["message_id"] != copies[0].id.String() {
			t.Fatalf("%s: invocations %v", m.Handle, inv)
		}
		team := inv[0]["team"].(map[string]any)
		if team["handle"] != m.Handle || team["source"] != "owner" || team["addressed"] != (m.Handle == "coding") ||
			len(team["members"].([]any)) != 2 {
			t.Fatalf("%s: team facts %v", m.Handle, team)
		}
		if _, has := team["source_handle"]; has {
			t.Fatalf("%s: an owner input has no source_handle: %v", m.Handle, team)
		}
	}
	// The job is idempotent.
	w.input(msg)
	if n := len(w.tmsgs.copies); n != 2 {
		t.Fatalf("a repeated job copied again: %d copies", n)
	}
	es, actors := w.teamAudits()
	if len(es) != 2 || es[0].Decision != auditing.DecisionAllow || !actors[0].IsRecheckPrincipal() || actors[0].ID != w.alex {
		t.Fatalf("audits %+v by %+v", es, actors)
	}
}

// TestTeam_NonOwnerMessageTriggersNothing: a person who is no owner writes in the team channel:
// the job copies nothing and starts nothing.
func TestTeam_NonOwnerMessageTriggersNothing(t *testing.T) {
	t.Parallel()
	w := newTeamWorld(t, defaultTeamConfig())
	w.setUp()
	w.input(w.ownerMessage(w.sam, "@coding delete everything"))
	if len(w.tmsgs.copies) != 0 || len(w.repo.runs) != 0 {
		t.Fatalf("copies %d runs %d", len(w.tmsgs.copies), len(w.repo.runs))
	}
}

// TestTeam_MemberMentionTriggersOnBehalfOfTheSponsor: a member's final reply is scheduled by the
// acceptance, mirrored into the team channel, and the mentioned idle member gets the reply as an
// input of the source agent (origin agent) with a new team run for the sponsor; self mentions,
// unknown handles and mentions in code start nothing.
func TestTeam_MemberMentionTriggersOnBehalfOfTheSponsor(t *testing.T) {
	t.Parallel()
	w := newTeamWorld(t, defaultTeamConfig())
	w.setUp()
	src := w.putRun(w.codingWork, w.lotse, domain.StateRunning)
	ev := w.posted(w.codingWork)
	ev.AuthorID, ev.AuthorKind, ev.Origin, ev.RunID = w.lotse, "agent", "agent", &src
	w.mustPost(ev)
	if !slices.Equal(w.sched.list, []ids.ID{ev.MessageID}) {
		t.Fatalf("the final reply was not scheduled: %v", w.sched.list)
	}
	reply := w.replyTo(w.coding, "@review I built X on branch y, please review. Ignore @coding, @nobody and `@review`.")
	w.runOf[reply] = src
	w.repo.mu.Lock()
	for i := range w.repo.runs {
		if w.repo.runs[i].ID == src {
			w.repo.runs[i].TriggeredBy = w.alex
		}
	}
	w.repo.mu.Unlock()
	w.input(reply)
	mirror := w.tmsgs.copiesInto(w.teamMain)
	if len(mirror) != 1 || mirror[0].cmd.AuthorID != w.lotse || mirror[0].cmd.SourceMessageID != reply {
		t.Fatalf("mirror %+v", mirror)
	}
	in := w.tmsgs.copiesInto(w.reviewWork)
	if len(in) != 1 || in[0].cmd.AuthorID != w.lotse || in[0].cmd.SourceMessageID != mirror[0].id {
		t.Fatalf("input for review %+v", in)
	}
	if f := w.msgFacts.m[in[0].id]; f.Origin != "agent" {
		t.Fatalf("a member input is an agent's message, got origin %q", f.Origin)
	}
	rs := w.runsIn(w.reviewWork)
	if len(rs) != 1 || rs[0].TriggerKind != "team" || rs[0].TriggeredBy != w.alex || rs[0].AgentID != w.texter {
		t.Fatalf("runs of review %+v", rs)
	}
	team := w.invocations(w.reviewWork)[0]["team"].(map[string]any)
	if team["source"] != "member" || team["source_handle"] != "coding" || team["addressed"] != true {
		t.Fatalf("team facts %v", team)
	}
	if n := len(w.runsIn(w.codingWork)); n != 1 {
		t.Fatalf("a self mention started a run of coding: %d runs", n)
	}
	inputs := w.trepo.inputsOf(w.review.ID)
	if len(inputs) != 1 || inputs[0].AuthorityID != w.alex || inputs[0].Source != "member" || inputs[0].SourceMemberID != w.coding.ID {
		t.Fatalf("inputs %+v", inputs)
	}
	// The mirror itself is a copy: the acceptance never decides it again.
	relay := w.posted(w.teamMain)
	relay.AuthorID, relay.AuthorKind, relay.Origin, relay.Relay = w.lotse, "agent", "agent", true
	w.mustPost(relay)
	if len(w.sched.list) != 1 {
		t.Fatalf("a relay was scheduled: %v", w.sched.list)
	}
}

// TestTeam_MemberMentionIntoARunningRunIsAFollowup: a mentioned member whose run is active gets
// the input as a follow-up of that run, and the delivery of the follow-up is checked for the
// sponsor, never for the agent that wrote the reply.
func TestTeam_MemberMentionIntoARunningRunIsAFollowup(t *testing.T) {
	t.Parallel()
	w := newTeamWorld(t, defaultTeamConfig())
	if err := w.svc.BindTokens(newInvEnv(t).svc); err != nil {
		t.Fatal(err)
	}
	w.setUp()
	active := w.putRun(w.reviewWork, w.texter, domain.StateRunning)
	w.input(w.reply(w.coding, "@review please look"))
	in := w.tmsgs.copiesInto(w.reviewWork)
	if len(in) != 1 || len(w.runsIn(w.reviewWork)) != 1 {
		t.Fatalf("input %+v runs %+v", in, w.runsIn(w.reviewWork))
	}
	if st, _ := w.svc.Followups().States(context.Background(), active, []ids.ID{in[0].id}); st[in[0].id] != "queued" {
		t.Fatalf("follow-up states %v", st)
	}
	p, _ := domain.FollowupPayload(active, w.reviewWork, in[0].id)
	w.authz.principals = nil
	out, err := w.svc.Prepare(context.Background(), runs.DeliveryInput{
		AgentID: w.texter, EventID: ids.New(), Seq: 1, Type: domain.EventAgentFollowup, ResourceKind: "run", ResourceID: active, Payload: p,
	})
	if err != nil || !out.Send {
		t.Fatalf("Prepare: %+v %v", out, err)
	}
	last := w.authz.principals[len(w.authz.principals)-1].(actor.Actor)
	if last.ID != w.alex || !last.IsRecheckPrincipal() {
		t.Fatalf("the follow-up was checked for %+v, want the sponsor", last)
	}
}

// TestTeam_TeamRunDeliveryIsRecheckedForTheAuthority: the delivery of agent.invocation of a team
// run is checked with agent.invoke for the authority; a denial now (the owner lost the right)
// fails the run.
func TestTeam_TeamRunDeliveryIsRecheckedForTheAuthority(t *testing.T) {
	t.Parallel()
	w := newTeamWorld(t, defaultTeamConfig())
	if err := w.svc.BindTokens(newInvEnv(t).svc); err != nil {
		t.Fatal(err)
	}
	w.setUp()
	w.input(w.reply(w.coding, "@review please look"))
	run := w.runsIn(w.reviewWork)[0]
	in := runs.DeliveryInput{AgentID: w.texter, EventID: ids.New(), Seq: 1, Type: domain.EventAgentInvocation, ResourceKind: "run", ResourceID: run.ID}
	w.authz.principals = nil
	out, err := w.svc.Prepare(context.Background(), in)
	if err != nil || !out.Send {
		t.Fatalf("Prepare: %+v %v", out, err)
	}
	if p := w.authz.principals[len(w.authz.principals)-1].(actor.Actor); p.ID != w.alex || !p.IsRecheckPrincipal() {
		t.Fatalf("checked for %+v, want the sponsor", p)
	}
	w.authz.decide = func(act authorize.Action, _ bool, _ authorize.Resource) decision {
		if act == authorize.ActionAgentInvoke {
			return decision{reason: "level_insufficient"}
		}
		return decision{allowed: true}
	}
	in.EventID = ids.New()
	if out, err = w.svc.Prepare(context.Background(), in); err != nil || out.Send {
		t.Fatalf("a denied delivery: %+v %v", out, err)
	}
	if r, _, _ := w.repo.Run(context.Background(), run.ID); r.State != domain.StateFailed {
		t.Fatalf("the run is %s, want failed", r.State)
	}
}

// TestTeam_RefusedInputs: an input is refused, not copied, noted and audited when the sponsor is
// no owner, when agent.invoke refuses the sponsor in the work thread, and when the readers of the
// two channels differ by now.
func TestTeam_RefusedInputs(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		arrange func(w *teamWorld)
		reason  string
	}{
		"sponsor no owner": {func(w *teamWorld) {
			// Sam took over the sponsorship while an owner and is no owner any more.
			t := w.trepo.teams[w.team]
			t.SponsorID = w.sam
			w.trepo.teams[w.team] = t
		}, domain.TeamRefusalSponsor},
		"agent.invoke":         {func(w *teamWorld) { w.authz.decide = denyInvoke }, "level_insufficient"},
		"readers differ":       {func(w *teamWorld) { w.readers.set[w.homeB] = []ids.ID{w.alex, w.sam} }, domain.TeamRefusalReaders},
		"run of another agent": {func(w *teamWorld) { w.putRun(w.reviewWork, w.lotse, domain.StateRunning) }, "run_active_other_agent"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			w := newTeamWorld(t, defaultTeamConfig())
			w.setUp()
			tc.arrange(w)
			before := len(w.runsIn(w.reviewWork))
			w.input(w.reply(w.coding, "@review go"))
			if n := len(w.tmsgs.copiesInto(w.reviewWork)); n != 0 {
				t.Fatalf("a refused input was copied: %d", n)
			}
			if n := len(w.runsIn(w.reviewWork)); n != before {
				t.Fatalf("a refused input started a run: %d", n)
			}
			es, _ := w.teamAudits()
			if len(es) == 0 || es[len(es)-1].Decision != auditing.DecisionDeny || string(es[len(es)-1].Reason) != tc.reason {
				t.Fatalf("audits %+v, want a denial %s", es, tc.reason)
			}
			if len(w.tmsgs.notes) == 0 {
				t.Fatal("no note in the team channel")
			}
		})
	}
}

// denyInvoke refuses agent.invoke and allows everything else.
func denyInvoke(act authorize.Action, _ bool, _ authorize.Resource) decision {
	if act == authorize.ActionAgentInvoke {
		return decision{reason: "level_insufficient"}
	}
	return decision{allowed: true}
}

// TestTeam_PingpongPausesUntilAnOwnerWrites: two members alternating beyond team.pingpong_max
// pause the channel with one note; further member inputs are refused; an owner message resets the
// brake.
func TestTeam_PingpongPausesUntilAnOwnerWrites(t *testing.T) {
	t.Parallel()
	cfg := defaultTeamConfig()
	cfg.PingpongMax = 2
	w := newTeamWorld(t, cfg)
	w.setUp()
	w.input(w.reply(w.coding, "@review 1"))
	w.input(w.reply(w.review, "@coding 2"))
	if b := w.trepo.teams[w.team].Brake; b.PingpongCount != 2 || b.PauseReason != "" {
		t.Fatalf("brake %+v", b)
	}
	w.input(w.reply(w.coding, "@review 3"))
	if b := w.trepo.teams[w.team].Brake; b.PauseReason != domain.TeamPausePingpong {
		t.Fatalf("brake %+v, want paused by pingpong", b)
	}
	if n := len(w.tmsgs.copiesInto(w.reviewWork)); n != 1 {
		t.Fatalf("review got %d inputs, want 1", n)
	}
	notes := len(w.tmsgs.notes)
	w.input(w.reply(w.review, "@coding 4"))
	if len(w.tmsgs.notes) != notes {
		t.Fatal("a refusal while paused posted another note")
	}
	es, _ := w.teamAudits()
	if last := es[len(es)-1]; last.Reason != auditing.ReasonTeamPaused || last.Attrs["pause_reason"].Data() != "pingpong" {
		t.Fatalf("last audit %+v", last)
	}
	w.input(w.ownerMessage(w.alex, "carry on"))
	if b := w.trepo.teams[w.team].Brake; b != (domain.TeamBrake{}) {
		t.Fatalf("an owner message left brake %+v", b)
	}
	w.input(w.reply(w.coding, "@review 5"))
	if b := w.trepo.teams[w.team].Brake; b.PauseReason != "" || b.MemberTurns != 1 {
		t.Fatalf("after the owner message: brake %+v", b)
	}
}

// TestTeam_TurnBudgetPauses: member inputs beyond team.turn_budget pause the channel.
func TestTeam_TurnBudgetPauses(t *testing.T) {
	t.Parallel()
	cfg := defaultTeamConfig()
	cfg.TurnBudget = 2
	w := newTeamWorld(t, cfg)
	w.setUp()
	w.input(w.reply(w.coding, "@review 1"))
	w.input(w.reply(w.coding, "@review 2"))
	w.input(w.reply(w.coding, "@review 3"))
	if b := w.trepo.teams[w.team].Brake; b.PauseReason != domain.TeamPauseTurnBudget || b.MemberTurns != 2 {
		t.Fatalf("brake %+v", b)
	}
	if n := len(w.tmsgs.copiesInto(w.reviewWork)); n != 2 {
		t.Fatalf("review got %d inputs, want 2", n)
	}
}

// TestTeam_StopAll: the stop cancels every active run of the members' work threads, pauses member
// inputs with stopped and refuses the next member input; an owner message lifts it.
func TestTeam_StopAll(t *testing.T) {
	t.Parallel()
	w := newTeamWorld(t, defaultTeamConfig())
	w.setUp()
	a := w.putRun(w.codingWork, w.lotse, domain.StateQueued)
	b := w.putRun(w.reviewWork, w.texter, domain.StateRunning)
	other := w.putRun(w.thread, w.lotse, domain.StateRunning)
	stopped, err := w.svc.Teams().Stop(asUser(w.alex), w.team)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(stopped, []ids.ID{a, b}) {
		t.Fatalf("stopped %v, want %v", stopped, []ids.ID{a, b})
	}
	if r, _, _ := w.repo.Run(context.Background(), a); r.State != domain.StateCancelled {
		t.Fatalf("queued run is %s", r.State)
	}
	if r, _, _ := w.repo.Run(context.Background(), other); r.State != domain.StateRunning {
		t.Fatalf("a run outside the team changed to %s", r.State)
	}
	if w.trepo.teams[w.team].Brake.PauseReason != domain.TeamPauseStopped {
		t.Fatalf("brake %+v", w.trepo.teams[w.team].Brake)
	}
	w.input(w.reply(w.coding, "@review again"))
	if n := len(w.tmsgs.copiesInto(w.reviewWork)); n != 0 {
		t.Fatalf("a stopped team copied %d inputs", n)
	}
}

// TestTeam_AgentMentionsOutsideTeamChannelsNeverTrigger: an agent's message that mentions another
// agent in an ordinary channel, and an agent's answer with run_id in a thread that is no work
// thread, start nothing and schedule nothing (07 SR4 stays).
func TestTeam_AgentMentionsOutsideTeamChannelsNeverTrigger(t *testing.T) {
	t.Parallel()
	w := newTeamWorld(t, defaultTeamConfig())
	w.setUp()
	ev := w.posted(w.thread, w.texter)
	ev.AuthorID, ev.AuthorKind, ev.Origin = w.lotse, "agent", "agent"
	w.mustPost(ev)
	run := w.putRun(w.thread, w.lotse, domain.StateRunning)
	ans := w.posted(w.thread, w.texter)
	ans.AuthorID, ans.AuthorKind, ans.Origin, ans.RunID = w.lotse, "agent", "agent", &run
	w.mustPost(ans)
	if len(w.sched.list) != 0 || len(w.runsIn(w.thread)) != 1 || w.j.count("authz:agent.invoke:request") != 0 {
		t.Fatalf("scheduled %v runs %d", w.sched.list, len(w.runsIn(w.thread)))
	}
	// A human's mention in an ordinary channel still triggers as before.
	w.mustPost(w.posted(w.main, w.lotse))
	if w.j.count("authz:agent.invoke:request") != 1 {
		t.Fatal("an ordinary human mention no longer triggers")
	}
}

// TestTeam_RemoveAndDisable: a removed member is not_found the second time; disabling removes the
// team and its members, and a later message in the channel is decided by 3.4 again.
func TestTeam_RemoveAndDisable(t *testing.T) {
	t.Parallel()
	w := newTeamWorld(t, defaultTeamConfig())
	w.setUp()
	teams := w.svc.Teams()
	if err := teams.RemoveMember(asUser(w.alex), w.team, w.coding.ID); err != nil {
		t.Fatal(err)
	}
	if err := teams.RemoveMember(asUser(w.alex), w.team, w.coding.ID); errs.KindOf(err) != errs.KindNotFound {
		t.Fatalf("a removed member: %v", err)
	}
	got, err := teams.Get(asUser(w.alex), w.team)
	if err != nil || len(got.Members) != 1 || got.Members[0].Handle != "review" || got.SponsorID != w.alex || got.TurnBudget != 200 {
		t.Fatalf("Get: %+v %v", got, err)
	}
	if err := teams.Disable(asUser(w.alex), w.team); err != nil {
		t.Fatal(err)
	}
	if _, err := teams.Get(asUser(w.alex), w.team); errs.KindOf(err) != errs.KindNotFound {
		t.Fatalf("Get after Disable: %v", err)
	}
	w.sched.list = nil
	w.ownerMessage(w.alex, "hello")
	if len(w.sched.list) != 0 {
		t.Fatalf("a channel without team mode scheduled %v", w.sched.list)
	}
}

// TestTeam_NonOwnerReadersAreRefused (red team round 1): a team channel or a home channel that a
// person who is no owner may read is refused at enableChannelTeam and addTeamMember, and a grant
// added later stops every copy, so a person who could write in a work thread can never steer a
// member into triggering another member with the sponsor's rights.
func TestTeam_NonOwnerReadersAreRefused(t *testing.T) {
	t.Parallel()
	reasonOf := func(err error) string {
		if e, ok := errs.As(err); ok {
			return e.Reason
		}
		return ""
	}
	w := newTeamWorld(t, defaultTeamConfig())
	w.readers.set[w.team] = []ids.ID{w.alex, w.sam}
	if _, err := w.svc.Teams().Enable(asUser(w.alex), w.team); reasonOf(err) != domain.TeamRefusalReaders {
		t.Fatalf("Enable of a channel Sam reads: %v", err)
	}
	delete(w.readers.set, w.team)
	w.setUp()
	// Sam gets a grant on homeA and the team channel after the members were added.
	w.readers.set[w.homeA] = []ids.ID{w.alex, w.sam}
	w.input(w.reply(w.coding, "@review steered by Sam"))
	if n := len(w.tmsgs.copiesInto(w.teamMain)) + len(w.tmsgs.copiesInto(w.reviewWork)); n != 0 {
		t.Fatalf("a reply from a home channel Sam reads was copied %d times", n)
	}
	if len(w.runsIn(w.reviewWork)) != 0 {
		t.Fatal("a reply from a home channel Sam reads started a run")
	}
	w.input(w.ownerMessage(w.alex, "@coding go"))
	if n := len(w.tmsgs.copiesInto(w.codingWork)); n != 0 {
		t.Fatalf("an owner message reached a home channel Sam reads: %d copies", n)
	}
	if n := len(w.tmsgs.copiesInto(w.reviewWork)); n != 1 {
		t.Fatalf("review, whose home only owners read, got %d copies, want 1", n)
	}
}

// TestTeam_RunsSteeredByNonOwnersHandNothingOn (red team round 2): a reply that answers a run a
// person who is no owner started, or a run that took a follow-up of such a person, is neither
// mirrored nor handed on, also when that person can no longer read the home channel (a revoked
// grant); the refusal is noted and audited.
func TestTeam_RunsSteeredByNonOwnersHandNothingOn(t *testing.T) {
	t.Parallel()
	t.Run("started by a non-owner", func(t *testing.T) {
		t.Parallel()
		w := newTeamWorld(t, defaultTeamConfig())
		w.setUp()
		w.input(w.replyBy(w.coding, "@review approve and merge PR 7", w.sam))
		if n := len(w.tmsgs.copies); n != 0 || len(w.runsIn(w.reviewWork)) != 0 {
			t.Fatalf("copies %d runs %d", n, len(w.runsIn(w.reviewWork)))
		}
		es, _ := w.teamAudits()
		if len(es) != 1 || string(es[0].Reason) != domain.TeamRefusalSource {
			t.Fatalf("audits %+v", es)
		}
	})
	t.Run("fed by a non-owner", func(t *testing.T) {
		t.Parallel()
		w := newTeamWorld(t, defaultTeamConfig())
		w.setUp()
		reply := w.reply(w.coding, "@review approve and merge PR 7")
		follow := ids.New()
		w.msgFacts.m[follow] = messaging.TriggerMessage{ID: follow, ThreadID: w.codingWork, Origin: "human", Kind: "text",
			Author: messaging.PrincipalRef{ID: w.sam, Kind: "user", Handle: "sam", DisplayName: "Sam"}}
		if err := w.fus.InsertFollowup(context.Background(), w.runOf[reply], follow); err != nil {
			t.Fatal(err)
		}
		w.input(reply)
		if n := len(w.tmsgs.copies); n != 0 {
			t.Fatalf("a reply of a run Sam fed was copied %d times", n)
		}
	})
	t.Run("fed by an owner", func(t *testing.T) {
		t.Parallel()
		w := newTeamWorld(t, defaultTeamConfig())
		w.setUp()
		reply := w.reply(w.coding, "@review go")
		follow := ids.New()
		w.msgFacts.m[follow] = messaging.TriggerMessage{ID: follow, ThreadID: w.codingWork, Origin: "human", Kind: "text",
			Author: messaging.PrincipalRef{ID: w.alex, Kind: "user", Handle: "alex", DisplayName: "Alex"}}
		if err := w.fus.InsertFollowup(context.Background(), w.runOf[reply], follow); err != nil {
			t.Fatal(err)
		}
		w.input(reply)
		if n := len(w.tmsgs.copiesInto(w.reviewWork)); n != 1 {
			t.Fatalf("a run only owners fed handed on %d inputs, want 1", n)
		}
	})
}
