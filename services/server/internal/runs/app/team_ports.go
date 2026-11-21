// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/messaging"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// TeamConfig holds the configuration of the team channels (09 21, FEAT-TEAM-CHANNEL-SERVER), bound
// in internal/wire next to Config. The brake defaults are high on purpose: the brake catches loops,
// it is no cost limit (owner answer of 2026-11-21).
type TeamConfig struct {
	// TurnBudget is team.turn_budget: member inputs per team channel since an owner's last message.
	TurnBudget int `config:"team.turn_budget"`
	// PingpongMax is team.pingpong_max: the longest alternation between two members.
	PingpongMax int `config:"team.pingpong_max"`
	// MaxMembers is team.max_members: members per team channel.
	MaxMembers int `config:"team.max_members"`
	// HandleMaxChars is team.handle_max_chars: the longest handle.
	HandleMaxChars int `config:"team.handle_max_chars"`
}

// check refuses limits below 1.
func (c TeamConfig) check() error {
	if c.TurnBudget < 1 || c.PingpongMax < 1 || c.MaxMembers < 1 || c.HandleMaxChars < 1 {
		return errors.New("runs: team.turn_budget, team.pingpong_max, team.max_members and team.handle_max_chars must be at least 1")
	}
	return nil
}

// limits returns the limits of the brake.
func (c TeamConfig) limits() domain.TeamLimits {
	return domain.TeamLimits{TurnBudget: c.TurnBudget, PingpongMax: c.PingpongMax}
}

// TeamRow is one row of team_channels (03 4.8).
type TeamRow struct {
	ChannelID, SponsorID ids.ID
	Brake                domain.TeamBrake
	CreatedAt            time.Time
	Version              int64
}

// TeamMemberRow is one row of team_members.
type TeamMemberRow struct {
	ID, ChannelID, AgentID, HomeChannelID, WorkThreadID, CreatedBy ids.ID
	Handle                                                         string
	CreatedAt                                                      time.Time
}

// TeamInputRow is one row of team_inputs; SourceMemberID is zero for NULL.
type TeamInputRow struct {
	MessageID, MemberID, SourceMessageID, SourceMemberID, AuthorityID ids.ID
	Source                                                            string
	CreatedAt                                                         time.Time
}

// TeamRepository is the outgoing port of the team tables (03 4.8); adapters/postgres implements
// it next to Repository. Reads join the ambient transaction; writes and locking reads need one.
// The lock order is the team_channels row before the runs of the work threads (03 6.3). Errors
// are kernel/errs errors.
type TeamRepository interface {
	// UpsertTeam switches the channel to team mode with the sponsor, or makes the sponsor the
	// sponsor of the existing team.
	UpsertTeam(ctx context.Context, channelID, sponsorID ids.ID, at time.Time) (TeamRow, error)
	// Team reads the team of the channel without a lock; found is false for none.
	Team(ctx context.Context, channelID ids.ID) (TeamRow, bool, error)
	// LockTeam reads the team of the channel with its row lock; found is false for none.
	LockTeam(ctx context.Context, channelID ids.ID) (TeamRow, bool, error)
	// DeleteTeam removes the team of the channel with its members; deleted is false for none.
	DeleteTeam(ctx context.Context, channelID ids.ID) (bool, error)
	// SaveTeamBrake stores the brake state of the team.
	SaveTeamBrake(ctx context.Context, channelID ids.ID, b domain.TeamBrake, at time.Time) (TeamRow, error)
	// InsertTeamMember inserts a member; the caller checked handle and pair under the team lock.
	InsertTeamMember(ctx context.Context, m TeamMemberRow) error
	// TeamMembers reads the members of the team in the order they were added.
	TeamMembers(ctx context.Context, channelID ids.ID) ([]TeamMemberRow, error)
	// TeamMemberByWorkThread reads the member whose work thread is the thread; found is false for
	// none.
	TeamMemberByWorkThread(ctx context.Context, threadID ids.ID) (TeamMemberRow, bool, error)
	// DeleteTeamMember removes the member of the team; deleted is false for none.
	DeleteTeamMember(ctx context.Context, channelID, memberID ids.ID) (bool, error)
	// InsertTeamInput records an input; inserted is false when its source already reached the
	// member.
	InsertTeamInput(ctx context.Context, in TeamInputRow) (bool, error)
	// TeamInput reads the input of the message; found is false for a message that is no input.
	TeamInput(ctx context.Context, messageID ids.ID) (TeamInputRow, bool, error)
	// TeamInputDelivered reports whether the source already reached the member.
	TeamInputDelivered(ctx context.Context, sourceMessageID, memberID ids.ID) (bool, error)
	// InsertTeamMirror records the mirror of a reply; inserted is false when it was mirrored.
	InsertTeamMirror(ctx context.Context, sourceMessageID, mirrorMessageID, memberID ids.ID, at time.Time) (bool, error)
	// TeamMirrored reports whether the reply was mirrored.
	TeamMirrored(ctx context.Context, sourceMessageID ids.ID) (bool, error)
}

// TeamMessages is the part of messaging the team channels write through (messaging app Service):
// the copies of 09 3.9.2 and 3.9.4 and the notes of 3.9.5 to 3.9.7. Neither authorizes.
type TeamMessages interface {
	CopyMessage(ctx context.Context, cmd messaging.CopyCmd) (messaging.Copied, error)
	AppendSystemNote(ctx context.Context, threadID, authorID ids.ID, text string) (ids.ID, int64, error)
}

// TeamReaders answers who may read a channel (authz.Recipients.Readers): the readers rule of 09
// 3.9.1 compares the sets of the team channel and a member's home channel.
type TeamReaders interface {
	Readers(ctx context.Context, channelID ids.ID) ([]ids.ID, error)
}

// TeamScheduler inserts the job runs.team_input of a message in the unit of work of ctx
// (adapters/jobs over jobs.Inserter.InsertTx); runID is the run a member's reply answers, zero for
// an owner's message.
type TeamScheduler interface {
	ScheduleTeamInput(ctx context.Context, messageID, runID ids.ID) error
}

// TeamDeps are the collaborators of the team channels; bind them with BindTeams. Without them a
// team operation answers internal_error, no channel is a team channel and the acceptance behaves
// as before the feature.
type TeamDeps struct {
	Repo     TeamRepository
	Messages TeamMessages
	Readers  TeamReaders
	Owners   Owners
	Schedule TeamScheduler
}
