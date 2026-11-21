// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/messaging"
	"github.com/OWNER/bothy/services/server/internal/runs"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// teamHook is the part of the acceptance for team channels (09 3.9.3, 3.9.4), inside the unit of
// work of the message: an owner's text message in a team channel and a member's final reply in its
// work thread schedule the job runs.team_input, which does the copying after the commit; it reports
// whether the message lies in a team channel, where 3.4 decides nothing. Without bound team
// channels nothing is a team message.
func (s *Service) teamHook(ctx context.Context, ev messaging.MessagePosted) (bool, error) {
	b := s.teams.Load()
	if b == nil || ev.Kind != kindText {
		team, err := s.isTeamChannel(ctx, b, ev.ChannelID)
		return team, err
	}
	if ev.Origin == originAgent && ev.RunID != nil {
		m, found, err := b.d.Repo.TeamMemberByWorkThread(ctx, ev.ThreadID)
		if err != nil || !found || m.AgentID != ev.AuthorID {
			return false, err
		}
		return false, b.d.Schedule.ScheduleTeamInput(ctx, ev.MessageID, *ev.RunID)
	}
	team, err := s.isTeamChannel(ctx, b, ev.ChannelID)
	if err != nil || !team {
		return false, err
	}
	if ev.Origin == originHuman {
		return true, b.d.Schedule.ScheduleTeamInput(ctx, ev.MessageID, ids.ID{})
	}
	return true, nil
}

// isTeamChannel reports whether the channel is a team channel; false without bound team channels.
func (s *Service) isTeamChannel(ctx context.Context, b *teamBox, channelID ids.ID) (bool, error) {
	if b == nil {
		return false, nil
	}
	_, found, err := b.d.Repo.Team(ctx, channelID)
	return found, err
}

// TeamInput is the use case of the job runs.team_input (09 3.9.3, 3.9.4) in one unit of work, for
// a member's reply with the run it answers (runID): the message is read again; an active owner's message in a team channel goes to every member, a
// member's reply in its work thread is mirrored and goes to the members it mentions. A deleted
// message, a team that is gone and a message that is neither are nothing; a repeated job changes
// nothing (team_mirrors, team_inputs). Business refusals post notes and audit entries; only
// infrastructure errors return an error, which makes the job retry.
func (s *Service) TeamInput(ctx context.Context, messageID, runID ids.ID) error {
	b, err := s.teamBox()
	if err != nil {
		return err
	}
	return s.d.UoW.Do(ctx, func(ctx context.Context) error {
		m, err := s.d.Messages.TriggerMessageOf(ctx, messageID)
		if errs.KindOf(err) == errs.KindNotFound {
			return nil
		}
		if err != nil || m.Deleted || m.Kind != kindText {
			return err
		}
		if m.Origin == originHuman {
			return s.ownerInput(ctx, b, m)
		}
		member, found, err := b.d.Repo.TeamMemberByWorkThread(ctx, m.ThreadID)
		if err != nil || !found || m.Origin != originAgent || member.AgentID != m.Author.ID {
			return err
		}
		return s.memberReply(ctx, b, m, member, runID)
	})
}

// teamSource is the source of one input: the message that is copied, its author (the copy's
// author), the authority of the input and, for a member reply, the source member.
type teamSource struct {
	message, author, authority ids.ID
	kind                       string
	member                     *TeamMemberRow
	// mentioned are the handles the source mentions, for the flag addressed; parsed reports that
	// they were read.
	mentioned []string
	parsed    bool
}

// ownerInput copies an owner's message in the team channel to every member (09 3.9.3): the brake
// resets, every member gets a follow-up or a new team run. A message of a person who is not an
// active owner is nothing.
func (s *Service) ownerInput(ctx context.Context, b *teamBox, m messaging.TriggerMessage) error {
	team, found, err := b.d.Repo.LockTeam(ctx, m.ChannelID)
	if err != nil || !found {
		return err
	}
	owner, err := b.d.Owners.IsActiveOwner(ctx, m.Author.ID)
	if err != nil || !owner {
		return err
	}
	if team.Brake != domain.ResetBrake() {
		if team, err = b.d.Repo.SaveTeamBrake(ctx, team.ChannelID, domain.ResetBrake(), s.now()); err != nil {
			return err
		}
	}
	members, err := b.d.Repo.TeamMembers(ctx, team.ChannelID)
	if err != nil {
		return err
	}
	src := teamSource{message: m.ID, author: m.Author.ID, authority: m.Author.ID, kind: domain.TeamSourceOwner}
	for _, member := range members {
		if _, err := s.teamDeliver(ctx, b, team, member, members, &src); err != nil {
			return err
		}
	}
	return nil
}

// memberReply mirrors a member's final reply into the main thread of the team channel and hands
// it to every member it mentions (09 3.9.4), each through the brake and the checks of 3.9.5, with
// the sponsor as authority.
func (s *Service) memberReply(ctx context.Context, b *teamBox, m messaging.TriggerMessage, member TeamMemberRow, runID ids.ID) error {
	mirrored, err := b.d.Repo.TeamMirrored(ctx, m.ID)
	if err != nil || mirrored {
		return err
	}
	team, found, err := b.d.Repo.LockTeam(ctx, member.ChannelID)
	if err != nil || !found {
		return err
	}
	owned, err := s.ownerRun(ctx, b, runID, member)
	if err != nil {
		return err
	}
	if !owned {
		return s.refuseInput(ctx, b, team, member,
			&teamSource{authority: team.SponsorID, kind: domain.TeamSourceMember, member: &member}, domain.TeamRefusalSource, "")
	}
	copied, ok, err := s.mirrorReply(ctx, b, team, m, member)
	if err != nil || !ok {
		return err
	}
	return s.handOn(ctx, b, team, member, copied)
}

// ownerRun reports whether only active owners steered the run runID a member's reply answers (09
// 3.9.4): it is a run of the member's agent in its work thread, its triggering person (for a team
// run the authority of its input) is an active owner, every follow-up it took is an active owner's
// message or a team input whose authority is an active owner, and every interaction with its
// components is an active owner's. A run that a person who is no owner started or fed, also one
// whose grant was revoked since, never hands work on with the sponsor's rights.
func (s *Service) ownerRun(ctx context.Context, b *teamBox, runID ids.ID, member TeamMemberRow) (bool, error) {
	run, found, err := s.d.Repo.Run(ctx, runID)
	if err != nil || !found || run.AgentID != member.AgentID || run.ThreadID != member.WorkThreadID {
		return false, err
	}
	persons, err := s.runInputPersons(ctx, b, run)
	if err != nil {
		return false, err
	}
	for _, p := range persons {
		if ok, err := b.d.Owners.IsActiveOwner(ctx, p); err != nil || p.IsZero() || !ok {
			return false, err
		}
	}
	return true, nil
}

// runInputPersons returns every person whose input the run took: its triggering person, the
// person of each follow-up (zero for a follow-up that speaks for nobody) and the principal of each
// interaction with its components. A wake input always speaks for the run's own person (5.8.3).
func (s *Service) runInputPersons(ctx context.Context, b *teamBox, run RunRow) ([]ids.ID, error) {
	persons := []ids.ID{run.TriggeredBy}
	if s.fu != nil {
		rows, err := s.fu.rows.RunFollowups(ctx, run.ID)
		if err != nil {
			return nil, err
		}
		for _, r := range rows {
			p, err := s.followupPerson(ctx, r.MessageID)
			if err != nil {
				return nil, err
			}
			persons = append(persons, p)
		}
	}
	ix, err := b.d.Repo.RunInteractionPersons(ctx, run.ID)
	if err != nil {
		return nil, err
	}
	return append(persons, ix...), nil
}

// followupPerson returns the person a follow-up message speaks for: the authority of a team input,
// the author of a person's message; zero for every other message.
func (s *Service) followupPerson(ctx context.Context, messageID ids.ID) (ids.ID, error) {
	in, team, err := s.teamInputOf(ctx, messageID)
	if err != nil || team {
		return in.AuthorityID, err
	}
	m, err := s.d.Messages.TriggerMessageOf(ctx, messageID)
	if err != nil || m.Origin != originHuman {
		return ids.ID{}, err
	}
	return m.Author.ID, nil
}

// mirrorReply copies the reply into the main thread of the team channel as the member's agent and
// records the mirror; ok is false for an archived team channel or one whose readers are not only
// owners, which is refused with a note and an audit entry.
func (s *Service) mirrorReply(ctx context.Context, b *teamBox, team TeamRow, m messaging.TriggerMessage, member TeamMemberRow) (messaging.Copied, bool, error) {
	ch, err := s.d.Channels.TriggerChannel(ctx, team.ChannelID)
	if err != nil || ch.MainThreadID.IsZero() || ch.Archived {
		return messaging.Copied{}, false, err
	}
	same, err := s.sameReaders(ctx, b, team.ChannelID, member.HomeChannelID)
	if err != nil {
		return messaging.Copied{}, false, err
	}
	if !same {
		return messaging.Copied{}, false, s.refuseInput(ctx, b, team, member,
			&teamSource{authority: team.SponsorID, kind: domain.TeamSourceMember, member: &member}, domain.TeamRefusalReaders, "")
	}
	copied, err := b.d.Messages.CopyMessage(ctx, messaging.CopyCmd{SourceMessageID: m.ID, ThreadID: ch.MainThreadID, AuthorID: member.AgentID})
	if err != nil {
		return messaging.Copied{}, false, err
	}
	if _, err := b.d.Repo.InsertTeamMirror(ctx, m.ID, copied.ID, member.ID, s.now()); err != nil {
		return messaging.Copied{}, false, err
	}
	return copied, true, nil
}

// handOn gives the mirrored reply to every other member it mentions, in the order of the first
// mention.
func (s *Service) handOn(ctx context.Context, b *teamBox, team TeamRow, member TeamMemberRow, copied messaging.Copied) error {
	members, err := b.d.Repo.TeamMembers(ctx, team.ChannelID)
	if err != nil {
		return err
	}
	byHandle := make(map[string]TeamMemberRow, len(members))
	for _, x := range members {
		byHandle[x.Handle] = x
	}
	src := teamSource{
		message: copied.ID, author: member.AgentID, authority: team.SponsorID, kind: domain.TeamSourceMember, member: &member,
		mentioned: domain.MentionedHandles(copied.ContentMD), parsed: true,
	}
	for _, h := range src.mentioned {
		target, ok := byHandle[h]
		if !ok || target.ID == member.ID {
			continue
		}
		if team, err = s.memberInput(ctx, b, team, target, members, &src); err != nil {
			return err
		}
	}
	return nil
}

// memberInput passes one member input through the brake (09 3.9.6) and delivers it; the brake
// state is stored when the input was delivered or the brake paused the channel. It returns the team
// row after the input.
func (s *Service) memberInput(ctx context.Context, b *teamBox, team TeamRow, target TeamMemberRow, members []TeamMemberRow,
	src *teamSource) (TeamRow, error) {
	next, reason := domain.NextBrake(team.Brake, src.member.ID, target.ID, b.cfg.limits())
	if reason != "" {
		return s.brakeRefusal(ctx, b, team, target, src, next, reason)
	}
	delivered, err := s.teamDeliver(ctx, b, team, target, members, src)
	if err != nil || !delivered {
		return team, err
	}
	return b.d.Repo.SaveTeamBrake(ctx, team.ChannelID, next, s.now())
}

// brakeRefusal refuses a member input the brake stopped: a channel that pauses now stores the
// pause, posts one note naming the pair and audits the pause; every refused input is audited with
// team_paused and the pause reason.
func (s *Service) brakeRefusal(ctx context.Context, b *teamBox, team TeamRow, target TeamMemberRow, src *teamSource, next domain.TeamBrake,
	reason string) (TeamRow, error) {
	if team.Brake.PauseReason == "" {
		var err error
		if team, err = b.d.Repo.SaveTeamBrake(ctx, team.ChannelID, next, s.now()); err != nil {
			return TeamRow{}, err
		}
		if err := s.teamNote(ctx, b, team.ChannelID, team.SponsorID, pauseNote(reason, src.member.Handle, target.Handle)); err != nil {
			return TeamRow{}, err
		}
		if err := s.auditTeam(ctx, actor.Recheck(team.SponsorID), authorize.ActionTeamUpdate, team.ChannelID, auditing.Attrs{
			attrOperation: auditing.Code(opPause), attrPauseReason: auditing.Code(reason),
		}); err != nil {
			return TeamRow{}, err
		}
	}
	return team, s.auditTeamInput(ctx, team, target, src, string(auditing.ReasonTeamPaused), reason)
}

// pauseNote is the text of the note of a pause of the brake (09 3.9.6).
func pauseNote(reason, from, to string) string {
	switch reason {
	case domain.TeamPausePingpong:
		return "Team paused: @" + from + " and @" + to + " kept handing work back and forth (team.pingpong_max). An owner message resumes the team."
	case domain.TeamPauseTurnBudget:
		return "Team paused: the members triggered each other team.turn_budget times since an owner last wrote. An owner message resumes the team."
	}
	return "Team paused (" + reason + "). An owner message resumes the team."
}

// teamDeliver checks one input for the member (09 3.9.5) and delivers it: the authority is an
// active owner, the readers rule holds, agent.invoke for the work thread allows for the authority;
// then the copy goes into the work thread, the input is recorded and becomes a follow-up of the
// member's active run or a new team run. A source that already reached the member is nothing. It
// reports whether the input was delivered; a refusal posts a note and is audited.
func (s *Service) teamDeliver(ctx context.Context, b *teamBox, team TeamRow, member TeamMemberRow, members []TeamMemberRow,
	src *teamSource) (bool, error) {
	done, err := b.d.Repo.TeamInputDelivered(ctx, src.message, member.ID)
	if err != nil || done {
		return false, err
	}
	reason, active, err := s.teamRefusal(ctx, b, team, member, src)
	if err != nil {
		return false, err
	}
	if reason != "" {
		return false, s.refuseInput(ctx, b, team, member, src, reason, "")
	}
	copied, err := b.d.Messages.CopyMessage(ctx, messaging.CopyCmd{SourceMessageID: src.message, ThreadID: member.WorkThreadID, AuthorID: src.author})
	if err != nil {
		return false, err
	}
	in := TeamInputRow{
		MessageID: copied.ID, MemberID: member.ID, SourceMessageID: src.message, Source: src.kind, AuthorityID: src.authority, CreatedAt: s.now(),
	}
	if src.member != nil {
		in.SourceMemberID = src.member.ID
	}
	if _, err := b.d.Repo.InsertTeamInput(ctx, in); err != nil {
		return false, err
	}
	if !src.parsed {
		// An owner's message is read once, at its first copy, for the members it addresses.
		src.mentioned, src.parsed = domain.MentionedHandles(copied.ContentMD), true
	}
	if active != nil {
		if s.fu == nil {
			return false, errs.New(errs.CodeInternalError, "runs: a team input into a running run needs the follow-ups")
		}
		if err := s.fu.store(ctx, copied.ID, *active); err != nil {
			return false, err
		}
	} else if err := s.openTeamRun(ctx, member, members, in, src); err != nil {
		return false, err
	}
	return true, s.auditTeamInput(ctx, team, member, src, "", "")
}

// teamRefusal applies the checks of 09 3.9.5 to an input for member and returns the reason of a
// refusal ("" to deliver) and the member's active run in the work thread, locked (nil for none).
func (s *Service) teamRefusal(ctx context.Context, b *teamBox, team TeamRow, member TeamMemberRow, src *teamSource) (string, *RunRow, error) {
	owner, err := b.d.Owners.IsActiveOwner(ctx, src.authority)
	if err != nil {
		return "", nil, err
	}
	if !owner {
		return domain.TeamRefusalSponsor, nil, nil
	}
	same, err := s.sameReaders(ctx, b, team.ChannelID, member.HomeChannelID)
	if err != nil {
		return "", nil, err
	}
	if !same {
		return domain.TeamRefusalReaders, nil, nil
	}
	row, found, err := s.d.Repo.LockActiveRunOfThread(ctx, member.WorkThreadID)
	if err != nil {
		return "", nil, err
	}
	var activeAgent ids.ID
	var active *RunRow
	if found {
		activeAgent, active = row.AgentID, &row
	}
	denial, err := s.wakeInvokeDenial(ctx, member.AgentID, member.WorkThreadID, src.authority, activeAgent)
	if err != nil || denial != "" {
		return denial, nil, err
	}
	if active != nil && active.AgentID != member.AgentID {
		return string(auditing.ReasonRunActiveOtherAgent), nil, nil
	}
	return "", active, nil
}

// refuseInput writes the refusal of an input: a note in the team channel naming the member and the
// reason, and the audit entry.
func (s *Service) refuseInput(ctx context.Context, b *teamBox, team TeamRow, member TeamMemberRow, src *teamSource, reason, detail string) error {
	if err := s.teamNote(ctx, b, team.ChannelID, member.AgentID, "Team input for @"+member.Handle+" refused: "+reason+"."); err != nil {
		return err
	}
	return s.auditTeamInput(ctx, team, member, src, reason, detail)
}

// auditTeamInput writes the audit entry of a team input (09 3.9.5): agent.invoke with stage team,
// decision allow or deny with the reason, the check principal of the authority as the actor, the
// work thread as the resource.
func (s *Service) auditTeamInput(ctx context.Context, team TeamRow, member TeamMemberRow, src *teamSource, reason, pause string) error {
	if s.d.Audit == nil {
		return errs.New(errs.CodeInternalError, "runs: the team channels have no audit recorder")
	}
	attrs := auditing.Attrs{
		attrStage: auditing.Code(stageTeam), attrTeamChannelID: auditing.ID(team.ChannelID), attrMemberID: auditing.ID(member.ID),
		attrAgentID: auditing.ID(member.AgentID), attrSource: auditing.Code(src.kind),
	}
	if src.member != nil {
		attrs[attrSourceMemberID] = auditing.ID(src.member.ID)
	}
	if pause != "" {
		attrs[attrPauseReason] = auditing.Code(pause)
	}
	e := auditing.Entry{
		Action: string(authorize.ActionAgentInvoke), Decision: auditing.DecisionAllow, ResourceKind: resourceThread, ResourceID: member.WorkThreadID,
		Attrs: attrs,
	}
	if reason != "" {
		e.Decision, e.Reason = auditing.DecisionDeny, auditing.Reason(reason)
	}
	return s.d.Audit.Record(actor.WithActor(ctx, actor.Recheck(src.authority)), e)
}

// openTeamRun opens the team run of an input in the member's work thread (09 3.9.3, 3.9.4):
// trigger_kind team, the authority as triggered_by, the input as triggering message, and
// agent.invocation with the team facts. Another run that won the thread meanwhile is a conflict;
// the job retries.
func (s *Service) openTeamRun(ctx context.Context, member TeamMemberRow, members []TeamMemberRow, in TeamInputRow, src *teamSource) error {
	facts, err := s.d.Agents.RunFacts(ctx, []ids.ID{member.AgentID})
	if err != nil {
		return err
	}
	mode := facts[member.AgentID].BillingMode
	// A job ends and opens runs without a principal; the anchor of the run is then written by the
	// member's agent (chat refuses a caller other than the principal of the unit).
	if a, ok := actor.FromContext(ctx); !ok || a.ID.IsZero() {
		ctx = actor.WithActor(ctx, actor.Actor{ID: member.AgentID, Kind: actor.KindAgent})
	}
	sum, err := s.Open(ctx, OpenCmd{
		ThreadID: member.WorkThreadID, AgentID: member.AgentID, TriggeredBy: in.AuthorityID, TriggeringMessageID: in.MessageID,
		TriggerKind: triggerKindTeam, BillingModeExpected: mode,
	})
	var exists *ActiveRunExistsError
	if errors.As(err, &exists) {
		return errs.New(errs.CodeConflict, "runs: another run opened in the work thread; the team input is tried again")
	}
	if err != nil {
		return err
	}
	return s.emitTeamInvocation(ctx, sum, in.MessageID, mode, teamFacts(member, members, src))
}

// teamFacts returns the team facts of the invocation of an input for member.
func teamFacts(member TeamMemberRow, members []TeamMemberRow, src *teamSource) domain.TeamFacts {
	f := domain.TeamFacts{ChannelID: member.ChannelID, MemberID: member.ID, Handle: member.Handle, Source: src.kind}
	if src.member != nil {
		f.SourceHandle = src.member.Handle
	}
	for _, h := range src.mentioned {
		if h == member.Handle {
			f.Addressed = true
		}
	}
	for _, m := range members {
		f.Members = append(f.Members, domain.TeamMemberFacts{MemberID: m.ID, AgentID: m.AgentID, HomeChannelID: m.HomeChannelID, Handle: m.Handle})
	}
	return f
}

// emitTeamInvocation appends agent.invocation of the team run sum (09 3.9.9): audience the agent,
// receive action agent.events.consume, resource the run, the payload with trigger_kind team, the
// input as message_id, the authority as triggered_by, the team facts and the session of the run.
func (s *Service) emitTeamInvocation(ctx context.Context, sum runs.RunSummary, message ids.ID, mode string, team domain.TeamFacts) error {
	info, err := s.d.Threads.Info(ctx, sum.ThreadID)
	if err != nil {
		return err
	}
	ch, err := s.d.Channels.TriggerChannel(ctx, info.ChannelID)
	if err != nil {
		return err
	}
	refs, err := s.d.Messages.PrincipalRefs(ctx, []ids.ID{sum.TriggeredBy})
	if err != nil {
		return err
	}
	ref, ok := refs[sum.TriggeredBy]
	if !ok {
		return errs.New(errs.CodeInternalError, "runs: the authority of a team run is unknown")
	}
	session, err := s.invocationSession(ctx, sum, operatorOf(info, ch))
	if err != nil {
		return err
	}
	p, err := domain.InvocationPayload(domain.InvocationPayloadInput{
		RunID: sum.ID, ThreadID: sum.ThreadID, ChannelID: info.ChannelID, SpaceID: info.SpaceID, MessageID: message, TriggerKind: triggerKindTeam,
		Team: &team,
		TriggeredBy: domain.PrincipalRefPayload{
			ID: ref.ID.String(), Kind: ref.Kind, Handle: ref.Handle, DisplayName: ref.DisplayName, AvatarFileID: optionalID(ref.AvatarFileID),
		},
		BillingMode: mode, SettingsVersion: ch.SettingsVersion,
	}, session)
	if err != nil {
		return errs.Wrap(errs.CodeInternalError, err, "runs: the payload of agent.invocation of a team run")
	}
	kind, actorID := eventActor(ctx)
	return s.d.Events.Append(ctx, eventlog.Event{
		Type: string(authorize.EventAgentInvocation), SchemaVersion: 1, ActorKind: kind, ActorPrincipalID: actorID,
		SpaceID: info.SpaceID, ChannelID: info.ChannelID, ThreadID: sum.ThreadID, ResourceKind: resourceRun, ResourceID: sum.ID,
		AudiencePrincipalID: sum.AgentID, ReceiveAction: string(authorize.ActionAgentEventsConsume), Payload: p,
	})
}

// teamInputOf returns the input row of a message; found is false without bound team channels or
// for a message that is no input.
func (s *Service) teamInputOf(ctx context.Context, messageID ids.ID) (TeamInputRow, bool, error) {
	b := s.teams.Load()
	if b == nil || messageID.IsZero() {
		return TeamInputRow{}, false, nil
	}
	return b.d.Repo.TeamInput(ctx, messageID)
}

// resolveInvocationTeam is agent.invocation with trigger_kind team (09 3.9.5): the trigger is the
// authority of the input (runs.triggered_by), the triggering object the input, origin human and the
// agent counted as addressed, as for a wake run; never the author of the input, which may be
// another member's agent. A run whose input was deleted cannot carry a valid token and is gone.
func resolveInvocationTeam(s *Service, ctx context.Context, run RunRow, _ runs.DeliveryInput) (trigger, error) {
	if run.TriggeringMessageID.IsZero() {
		return trigger{gone: true}, nil
	}
	m, err := s.d.Messages.TriggerMessageOf(ctx, run.TriggeringMessageID)
	if err != nil {
		return trigger{}, err
	}
	return trigger{
		person: run.TriggeredBy, object: m.ID, message: m.ID, human: m.Origin == originHuman, origin: originHuman, mentions: true,
		state: m.TriggerState,
	}, nil
}

// teamFollowupTrigger returns the trigger of a follow-up whose message is a team input: the
// authority recorded for the input, origin human, the agent counted as addressed (09 3.9.5).
func teamFollowupTrigger(in TeamInputRow, m messaging.TriggerMessage) trigger {
	return trigger{
		person: in.AuthorityID, object: m.ID, message: m.ID, human: m.Origin == originHuman, origin: originHuman, mentions: true,
		state: m.TriggerState,
	}
}

// followOnTeamRun opens the follow-on run of an ended run whose first allowed candidate is the
// team input in (09 3.9.5, section 7): a team run with the input's authority and the team facts of
// the member now. A work thread that is no longer a member's (the member was removed) starts the
// team run without facts of other members.
func (s *Service) followOnTeamRun(ctx context.Context, ended runs.RunSummary, in TeamInputRow, mode string) (runs.RunSummary, error) {
	b, err := s.teamBox()
	if err != nil {
		return runs.RunSummary{}, err
	}
	member, found, err := b.d.Repo.TeamMemberByWorkThread(ctx, ended.ThreadID)
	if err != nil {
		return runs.RunSummary{}, err
	}
	if !found {
		member = TeamMemberRow{ID: in.MemberID, AgentID: ended.AgentID, WorkThreadID: ended.ThreadID}
	}
	var members []TeamMemberRow
	if found {
		if members, err = b.d.Repo.TeamMembers(ctx, member.ChannelID); err != nil {
			return runs.RunSummary{}, err
		}
	}
	src := &teamSource{message: in.SourceMessageID, authority: in.AuthorityID, kind: in.Source, parsed: true}
	for i := range members {
		if members[i].ID == in.SourceMemberID {
			src.member = &members[i]
		}
	}
	sum, err := s.Open(ctx, OpenCmd{
		ThreadID: ended.ThreadID, AgentID: ended.AgentID, TriggeredBy: in.AuthorityID, TriggeringMessageID: in.MessageID,
		TriggerKind: triggerKindTeam, BillingModeExpected: mode,
	})
	if err != nil {
		return runs.RunSummary{}, err
	}
	return sum, s.emitTeamInvocation(ctx, sum, in.MessageID, mode, teamFacts(member, members, src))
}
