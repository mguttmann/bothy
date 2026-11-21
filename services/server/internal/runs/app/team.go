// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"slices"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/runs"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// Team channels (docs/spec/09-agents.md 3.9, FEAT-TEAM-CHANNEL-SERVER): the operations of the
// owners (Teams), the hook of the acceptance that schedules runs.team_input, and the job that
// copies an owner's message to every member or mirrors a member's reply and hands it to the
// members it mentions.

// triggerKindTeam is runs.trigger_kind of a run of a team channel member (03 runs, R-DM-62).
const triggerKindTeam = "team"

// The channel kinds of chat (03 channels.kind).
const (
	channelText = "text"
)

// The reasons of the 409 answers of the team operations (09 3.9.1).
const (
	reasonTeamNotEnabled      = "team_not_enabled"
	reasonTeamHandleTaken     = "team_handle_taken"
	reasonTeamMemberExists    = "team_member_exists"
	reasonTeamMaxMembers      = "team_max_members"
	reasonTeamHomeNotOperated = "team_home_not_operated"
	reasonTeamHomeIsTeam      = "team_home_is_team"
	reasonChannelKind         = "channel_kind"
	reasonDMChannel           = "dm_channel"
	reasonChannelArchived     = "channel_archived"
	reasonChannelReadOnly     = "channel_read_only"
	reasonAgentDisabled       = "agent_disabled"
)

// The audit attributes of the team channels.
const (
	attrTeamChannelID  = "team_channel_id"
	attrMemberID       = "member_id"
	attrSourceMemberID = "source_member_id"
	attrSource         = "source"
	attrPauseReason    = "pause_reason"
	attrOperation      = "operation"
	attrStopped        = "stopped_runs"
	stageTeam          = "team"
	resourceChannel    = "channel"
)

// The operations of team.update in the audit attribute operation.
const (
	opEnable  = "enable"
	opDisable = "disable"
	opAdd     = "member_add"
	opRemove  = "member_remove"
	opPause   = "pause"
)

// teamBox holds the bound configuration and collaborators of the team channels.
type teamBox struct {
	cfg TeamConfig
	d   TeamDeps
}

// BindTeams binds the team channels (FEAT-TEAM-CHANNEL-SERVER); internal/wire calls it once after
// the runs use cases exist. Errors: a missing collaborator or a limit below 1.
func (s *Service) BindTeams(cfg TeamConfig, d TeamDeps) error {
	if d.Repo == nil || d.Messages == nil || d.Readers == nil || d.Owners == nil || d.Schedule == nil {
		return errors.New("runs: BindTeams needs the repository, messages, readers, owners and scheduler")
	}
	if err := cfg.check(); err != nil {
		return err
	}
	s.teams.Store(&teamBox{cfg: cfg, d: d})
	return nil
}

// teamBox returns the bound team channels or internal_error.
func (s *Service) teamBox() (*teamBox, error) {
	b := s.teams.Load()
	if b == nil {
		return nil, errs.New(errs.CodeInternalError, "runs: the team channels are not bound")
	}
	return b, nil
}

// Teams returns the operations of the team channels (runs.Teams).
func (s *Service) Teams() *Teams { return &Teams{s: s} }

// Teams implements runs.Teams over the use cases.
type Teams struct{ s *Service }

// Compile-time check: Teams is the port of the team operations.
var _ runs.Teams = (*Teams)(nil)

// teamConflict is the 409 of a team operation with its reason (06 R-API-30).
func teamConflict(reason string) error {
	return errs.New(errs.CodeConflict, "the team operation was refused: "+reason).WithReason(reason)
}

// authorizeChannel decides act on the channel for a in request mode: not_found for a channel the
// caller cannot read, the error of the decision for every other denial.
func (s *Service) authorizeChannel(ctx context.Context, a actor.Actor, act authorize.Action, channelID ids.ID) error {
	d, err := s.d.Authorizer.Authorize(ctx, a, act, authorize.Resource{Kind: authorize.ResourceChannel, ID: channelID})
	if err != nil {
		return err
	}
	if d.Outcome == authorize.OutcomeNotFound {
		return invisible()
	}
	if !d.Allowed() {
		return d.Err()
	}
	return nil
}

// auditTeam writes the audit entry of a team operation or decision: allow without reason, deny
// with reason, resource the team channel, as the actor a.
func (s *Service) auditTeam(ctx context.Context, a actor.Actor, act authorize.Action, reason string, channelID ids.ID, attrs auditing.Attrs) error {
	if s.d.Audit == nil {
		return errs.New(errs.CodeInternalError, "runs: the team channels have no audit recorder")
	}
	e := auditing.Entry{Action: string(act), Decision: auditing.DecisionAllow, ResourceKind: resourceChannel, ResourceID: channelID, Attrs: attrs}
	if reason != "" {
		e.Decision, e.Reason = auditing.DecisionDeny, auditing.Reason(reason)
	}
	return s.d.Audit.Record(actor.WithActor(ctx, a), e)
}

// Get implements runs.Teams (getChannelTeam, team.read).
func (t *Teams) Get(ctx context.Context, channelID ids.ID) (runs.Team, error) {
	s := t.s
	b, err := s.teamBox()
	if err != nil {
		return runs.Team{}, err
	}
	a := caller(ctx)
	var out runs.Team
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		if err := s.authorizeChannel(ctx, a, authorize.ActionTeamRead, channelID); err != nil {
			return err
		}
		row, found, err := b.d.Repo.Team(ctx, channelID)
		if err != nil {
			return err
		}
		if !found {
			return invisible()
		}
		out, err = s.teamView(ctx, b, row)
		return err
	})
	return out, err
}

// teamView returns the API form of the team row with its members.
func (s *Service) teamView(ctx context.Context, b *teamBox, row TeamRow) (runs.Team, error) {
	members, err := b.d.Repo.TeamMembers(ctx, row.ChannelID)
	if err != nil {
		return runs.Team{}, err
	}
	out := runs.Team{
		ChannelID: row.ChannelID, SponsorID: row.SponsorID, MemberTurns: row.Brake.MemberTurns, TurnBudget: b.cfg.TurnBudget,
		PauseReason: row.Brake.PauseReason, Members: make([]runs.TeamMember, 0, len(members)), CreatedAt: row.CreatedAt, Version: row.Version,
	}
	for _, m := range members {
		out.Members = append(out.Members, memberView(m))
	}
	return out, nil
}

// memberView returns the API form of a member row.
func memberView(m TeamMemberRow) runs.TeamMember {
	return runs.TeamMember{
		ID: m.ID, AgentID: m.AgentID, HomeChannelID: m.HomeChannelID, WorkThreadID: m.WorkThreadID, Handle: m.Handle, CreatedAt: m.CreatedAt,
	}
}

// Enable implements runs.Teams (enableChannelTeam, team.update): a text channel that is neither
// archived nor read-only becomes a team channel with the caller as its sponsor; on a team channel
// the caller takes over the sponsorship. Audited.
func (t *Teams) Enable(ctx context.Context, channelID ids.ID) (runs.Team, error) {
	s := t.s
	b, err := s.teamBox()
	if err != nil {
		return runs.Team{}, err
	}
	a := caller(ctx)
	var out runs.Team
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		if err := s.authorizeChannel(ctx, a, authorize.ActionTeamUpdate, channelID); err != nil {
			return err
		}
		ch, err := s.d.Channels.TriggerChannel(ctx, channelID)
		if err != nil {
			return err
		}
		if reason := teamChannelRefusal(ch); reason != "" {
			return teamConflict(reason)
		}
		row, err := b.d.Repo.UpsertTeam(ctx, channelID, a.ID, s.now())
		if err != nil {
			return err
		}
		if err := s.auditTeam(ctx, a, authorize.ActionTeamUpdate, "", channelID, auditing.Attrs{attrOperation: auditing.Code(opEnable)}); err != nil {
			return err
		}
		out, err = s.teamView(ctx, b, row)
		return err
	})
	return out, err
}

// teamChannelRefusal returns the reason a channel cannot be a team channel: a DM, a forum, an
// archived or a read-only channel; "" for a channel that can.
func teamChannelRefusal(ch chat.TriggerChannel) string {
	switch {
	case ch.Kind == channelDM:
		return reasonDMChannel
	case ch.Kind != channelText:
		return reasonChannelKind
	case ch.Archived:
		return reasonChannelArchived
	case ch.ReadOnly:
		return reasonChannelReadOnly
	}
	return ""
}

// Disable implements runs.Teams (disableChannelTeam, team.update): the team and its members go;
// work threads, messages and runs stay. A channel without team mode is no change. Audited when a
// team was removed.
func (t *Teams) Disable(ctx context.Context, channelID ids.ID) error {
	s := t.s
	b, err := s.teamBox()
	if err != nil {
		return err
	}
	a := caller(ctx)
	return s.d.UoW.Do(ctx, func(ctx context.Context) error {
		if err := s.authorizeChannel(ctx, a, authorize.ActionTeamUpdate, channelID); err != nil {
			return err
		}
		deleted, err := b.d.Repo.DeleteTeam(ctx, channelID)
		if err != nil || !deleted {
			return err
		}
		return s.auditTeam(ctx, a, authorize.ActionTeamUpdate, "", channelID, auditing.Attrs{attrOperation: auditing.Code(opDisable)})
	})
}

// AddMember implements runs.Teams (addTeamMember, team.update; 09 3.9.1): the handle rule before
// anything is read, then under the team's row lock the limits, the agent, the home channel, the
// readers rule, the work thread and the member. Audited.
func (t *Teams) AddMember(ctx context.Context, in runs.AddTeamMember) (runs.TeamMember, error) {
	s := t.s
	b, err := s.teamBox()
	if err != nil {
		return runs.TeamMember{}, err
	}
	if !domain.ValidHandle(in.Handle, b.cfg.HandleMaxChars) {
		return runs.TeamMember{}, fieldError("/handle", "invalid",
			"a handle has lower-case letters, digits, '-' and '_', starts with a letter or digit and is at most team.handle_max_chars long")
	}
	a := caller(ctx)
	var out TeamMemberRow
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		out, err = s.addMemberTx(ctx, b, a, in)
		return err
	})
	if err != nil {
		return runs.TeamMember{}, err
	}
	return memberView(out), nil
}

// addMemberTx is AddMember inside its unit of work.
func (s *Service) addMemberTx(ctx context.Context, b *teamBox, a actor.Actor, in runs.AddTeamMember) (TeamMemberRow, error) {
	if err := s.authorizeChannel(ctx, a, authorize.ActionTeamUpdate, in.ChannelID); err != nil {
		return TeamMemberRow{}, err
	}
	team, found, err := b.d.Repo.LockTeam(ctx, in.ChannelID)
	if err != nil {
		return TeamMemberRow{}, err
	}
	if !found {
		return TeamMemberRow{}, teamConflict(reasonTeamNotEnabled)
	}
	members, err := b.d.Repo.TeamMembers(ctx, in.ChannelID)
	if err != nil {
		return TeamMemberRow{}, err
	}
	if reason := memberRefusal(members, in, b.cfg.MaxMembers); reason != "" {
		return TeamMemberRow{}, teamConflict(reason)
	}
	home, err := s.homeChannel(ctx, b, team, in)
	if err != nil {
		return TeamMemberRow{}, err
	}
	thread, err := s.createWorkThread(ctx, a, home, in.AgentID, in.Handle)
	if err != nil {
		return TeamMemberRow{}, err
	}
	m := TeamMemberRow{
		ID: ids.New(), ChannelID: in.ChannelID, AgentID: in.AgentID, HomeChannelID: in.HomeChannelID, WorkThreadID: thread,
		Handle: in.Handle, CreatedBy: a.ID, CreatedAt: s.now(),
	}
	if err := b.d.Repo.InsertTeamMember(ctx, m); err != nil {
		return TeamMemberRow{}, err
	}
	return m, s.auditTeam(ctx, a, authorize.ActionTeamUpdate, "", in.ChannelID, auditing.Attrs{
		attrOperation: auditing.Code(opAdd), attrMemberID: auditing.ID(m.ID), attrAgentID: auditing.ID(in.AgentID),
	})
}

// memberRefusal returns the reason a new member cannot join the members: too many, its handle
// taken, the same agent and home channel twice; "" otherwise.
func memberRefusal(members []TeamMemberRow, in runs.AddTeamMember, maxMembers int) string {
	if len(members) >= maxMembers {
		return reasonTeamMaxMembers
	}
	for _, m := range members {
		switch {
		case m.Handle == in.Handle:
			return reasonTeamHandleTaken
		case m.AgentID == in.AgentID && m.HomeChannelID == in.HomeChannelID:
			return reasonTeamMemberExists
		}
	}
	return ""
}

// homeChannel checks the agent and its home channel for a new member (09 3.9.1): an active agent,
// a channel that is no DM, not archived or read-only, no team channel, operated by the agent, and
// whose readers are those of the team channel.
func (s *Service) homeChannel(ctx context.Context, b *teamBox, team TeamRow, in runs.AddTeamMember) (chat.TriggerChannel, error) {
	facts, err := s.d.Agents.RunFacts(ctx, []ids.ID{in.AgentID})
	if err != nil {
		return chat.TriggerChannel{}, err
	}
	if f, ok := facts[in.AgentID]; !ok || f.Status != agentActive {
		return chat.TriggerChannel{}, teamConflict(reasonAgentDisabled)
	}
	home, err := s.d.Channels.TriggerChannel(ctx, in.HomeChannelID)
	if err != nil {
		return chat.TriggerChannel{}, err
	}
	switch {
	case home.Kind == channelDM:
		return chat.TriggerChannel{}, teamConflict(reasonDMChannel)
	case home.Archived:
		return chat.TriggerChannel{}, teamConflict(reasonChannelArchived)
	case home.ReadOnly:
		return chat.TriggerChannel{}, teamConflict(reasonChannelReadOnly)
	case home.EffectiveOperatorID != in.AgentID:
		return chat.TriggerChannel{}, teamConflict(reasonTeamHomeNotOperated)
	}
	if _, isTeam, err := b.d.Repo.Team(ctx, in.HomeChannelID); err != nil || isTeam {
		if err != nil {
			return chat.TriggerChannel{}, err
		}
		return chat.TriggerChannel{}, teamConflict(reasonTeamHomeIsTeam)
	}
	same, err := s.sameReaders(ctx, b, team.ChannelID, in.HomeChannelID)
	if err != nil {
		return chat.TriggerChannel{}, err
	}
	if !same {
		return chat.TriggerChannel{}, teamConflict(domain.TeamRefusalReaders)
	}
	return home, nil
}

// sameReaders reports whether the persons who may read the two channels are the same set (the
// readers rule of 09 3.9.1).
func (s *Service) sameReaders(ctx context.Context, b *teamBox, a, c ids.ID) (bool, error) {
	ra, err := b.d.Readers.Readers(ctx, a)
	if err != nil {
		return false, err
	}
	rc, err := b.d.Readers.Readers(ctx, c)
	if err != nil {
		return false, err
	}
	slices.SortFunc(ra, compareIDs)
	slices.SortFunc(rc, compareIDs)
	return slices.Equal(slices.Compact(ra), slices.Compact(rc)), nil
}

// workThreadNote is the text of the system note a work thread in a text channel starts from.
const workThreadNote = "Work thread of a team channel member"

// createWorkThread creates the member's work thread in its home channel as the owner a: a forum
// post in a forum, a thread on a system note of the main thread in a text channel. Its operator
// is the channel's effective operator, the agent (R-DM-12).
func (s *Service) createWorkThread(ctx context.Context, a actor.Actor, home chat.TriggerChannel, agentID ids.ID, handle string) (ids.ID, error) {
	b, err := s.teamBox()
	if err != nil {
		return ids.ID{}, err
	}
	cmd := chat.CreateThreadCmd{ChannelID: home.ChannelID}
	if home.Kind == channelForum {
		title := "Team: @" + handle
		cmd.Title = &title
	} else {
		note, _, err := b.d.Messages.AppendSystemNote(ctx, home.MainThreadID, agentID, workThreadNote+" @"+handle)
		if err != nil {
			return ids.ID{}, err
		}
		cmd.Parent = &chat.ParentMessage{MessageID: note, ThreadID: home.MainThreadID}
	}
	th, err := s.d.Threads.Create(ctx, a, cmd)
	if err != nil {
		return ids.ID{}, err
	}
	return th.ID, nil
}

// RemoveMember implements runs.Teams (removeTeamMember, team.update): the member goes, its work
// thread and run stay. An unknown member is not_found. Audited.
func (t *Teams) RemoveMember(ctx context.Context, channelID, memberID ids.ID) error {
	s := t.s
	b, err := s.teamBox()
	if err != nil {
		return err
	}
	a := caller(ctx)
	return s.d.UoW.Do(ctx, func(ctx context.Context) error {
		if err := s.authorizeChannel(ctx, a, authorize.ActionTeamUpdate, channelID); err != nil {
			return err
		}
		if _, found, err := b.d.Repo.LockTeam(ctx, channelID); err != nil || !found {
			if err != nil {
				return err
			}
			return invisible()
		}
		deleted, err := b.d.Repo.DeleteTeamMember(ctx, channelID, memberID)
		if err != nil {
			return err
		}
		if !deleted {
			return invisible()
		}
		return s.auditTeam(ctx, a, authorize.ActionTeamUpdate, "", channelID, auditing.Attrs{
			attrOperation: auditing.Code(opRemove), attrMemberID: auditing.ID(memberID),
		})
	})
}

// Stop implements runs.Teams (stopChannelTeam, team.stop; 09 3.9.7): under the team's row lock
// every active run in the members' work threads is cancelled as cancelRun by the caller would,
// member inputs pause with stopped, a note goes to the team channel. Audited.
func (t *Teams) Stop(ctx context.Context, channelID ids.ID) ([]ids.ID, error) {
	s := t.s
	b, err := s.teamBox()
	if err != nil {
		return nil, err
	}
	a := caller(ctx)
	var stopped []ids.ID
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		stopped, err = s.stopTx(ctx, b, a, channelID)
		return err
	})
	return stopped, err
}

// stopTx is Stop inside its unit of work.
func (s *Service) stopTx(ctx context.Context, b *teamBox, a actor.Actor, channelID ids.ID) ([]ids.ID, error) {
	if err := s.authorizeChannel(ctx, a, authorize.ActionTeamStop, channelID); err != nil {
		return nil, err
	}
	team, found, err := b.d.Repo.LockTeam(ctx, channelID)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, invisible()
	}
	members, err := b.d.Repo.TeamMembers(ctx, channelID)
	if err != nil {
		return nil, err
	}
	stopped := []ids.ID{}
	for _, m := range members {
		active, found, err := s.d.Repo.ActiveRunOfThread(ctx, m.WorkThreadID)
		if err != nil {
			return nil, err
		}
		if !found {
			continue
		}
		if _, err := s.cancelTx(ctx, a, active.ID); err != nil {
			return nil, err
		}
		stopped = append(stopped, active.ID)
	}
	brake := team.Brake
	brake.PauseReason = domain.TeamPauseStopped
	if _, err := b.d.Repo.SaveTeamBrake(ctx, channelID, brake, s.now()); err != nil {
		return nil, err
	}
	if err := s.teamNote(ctx, b, channelID, a.ID, "Team stopped by an owner: every member run was stopped and member mentions are paused until an owner writes again."); err != nil {
		return nil, err
	}
	return stopped, s.auditTeam(ctx, a, authorize.ActionTeamStop, "", channelID, auditing.Attrs{attrStopped: auditing.Int(int64(len(stopped)))})
}

// teamNote posts a system note in the main thread of the team channel (09 3.9.5 to 3.9.7),
// authored by authorID.
func (s *Service) teamNote(ctx context.Context, b *teamBox, channelID, authorID ids.ID, text string) error {
	ch, err := s.d.Channels.TriggerChannel(ctx, channelID)
	if err != nil {
		return err
	}
	if ch.MainThreadID.IsZero() {
		return nil
	}
	_, _, err = b.d.Messages.AppendSystemNote(ctx, ch.MainThreadID, authorID, text)
	return err
}
