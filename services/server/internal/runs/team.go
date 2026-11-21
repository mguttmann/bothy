// SPDX-License-Identifier: Apache-2.0

package runs

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Team is the team of a team channel (getChannelTeam, docs/spec/09-agents.md 3.9.1): its sponsor,
// the brake state and the members in the order they were added.
type Team struct {
	ChannelID, SponsorID ids.ID
	// MemberTurns are the member inputs since an owner's last message; TurnBudget is
	// team.turn_budget now.
	MemberTurns, TurnBudget int
	// PauseReason is turn_budget, pingpong or stopped; "" while member inputs flow.
	PauseReason string
	Members     []TeamMember
	CreatedAt   time.Time
	Version     int64
}

// TeamMember is one member of a team channel.
type TeamMember struct {
	ID, AgentID, HomeChannelID, WorkThreadID ids.ID
	Handle                                   string
	CreatedAt                                time.Time
}

// AddTeamMember is the request of addTeamMember.
type AddTeamMember struct {
	ChannelID, AgentID, HomeChannelID ids.ID
	Handle                            string
}

// Teams is the port of the team channels (09 3.9): the operations of the owners. Every method
// authorizes the principal of ctx; a channel without team mode is not_found where the operation
// needs one.
type Teams interface {
	// Get returns the team of the channel (team.read).
	//
	// Example:
	//
	//	t, err := teams.Get(ctx, channelID)
	Get(ctx context.Context, channelID ids.ID) (Team, error)
	// Enable switches the channel to team mode with the caller as its sponsor (team.update).
	Enable(ctx context.Context, channelID ids.ID) (Team, error)
	// Disable removes the team mode and every member (team.update); no team is no change.
	Disable(ctx context.Context, channelID ids.ID) error
	// AddMember adds a member and creates its work thread (team.update).
	AddMember(ctx context.Context, in AddTeamMember) (TeamMember, error)
	// RemoveMember removes a member (team.update).
	RemoveMember(ctx context.Context, channelID, memberID ids.ID) error
	// Stop stops every active run of the members and pauses member inputs (team.stop); it returns
	// the runs it stopped.
	Stop(ctx context.Context, channelID ids.ID) ([]ids.ID, error)
}
