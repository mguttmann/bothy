// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"context"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/runs"
)

// The REST operations of the team channels (docs/spec/09-agents.md 3.9, FEAT-TEAM-CHANNEL-SERVER):
// getChannelTeam, enableChannelTeam, disableChannelTeam, addTeamMember, removeTeamMember and
// stopChannelTeam over runs.Teams. The use cases authorize; the handlers only convert.

// BindTeams binds the team operations; internal/wire calls it once after BindTeams of the use
// cases. A nil t is ignored.
func (h *Runs) BindTeams(t runs.Teams) {
	if t != nil {
		h.teams = t
	}
}

// teamOps returns the bound team operations; without them every operation answers internal_error.
func (h *Runs) teamOps() (runs.Teams, error) {
	if h.teams == nil {
		return nil, errs.New(errs.CodeInternalError, "runs: the team channels are not bound")
	}
	return h.teams, nil
}

// GetChannelTeam implements rest.StrictServerInterface (GET /channels/{channel_id}/team).
func (h *Runs) GetChannelTeam(ctx context.Context, req rest.GetChannelTeamRequestObject) (rest.GetChannelTeamResponseObject, error) {
	t, err := h.teamOps()
	if err != nil {
		return nil, err
	}
	team, err := t.Get(ctx, ids.ID(req.ChannelId))
	if err != nil {
		return nil, err
	}
	return rest.GetChannelTeam200JSONResponse{Body: TeamREST(team)}, nil
}

// EnableChannelTeam implements rest.StrictServerInterface (PUT /channels/{channel_id}/team).
func (h *Runs) EnableChannelTeam(ctx context.Context, req rest.EnableChannelTeamRequestObject) (rest.EnableChannelTeamResponseObject, error) {
	t, err := h.teamOps()
	if err != nil {
		return nil, err
	}
	team, err := t.Enable(ctx, ids.ID(req.ChannelId))
	if err != nil {
		return nil, err
	}
	return rest.EnableChannelTeam200JSONResponse{Body: TeamREST(team)}, nil
}

// DisableChannelTeam implements rest.StrictServerInterface (DELETE /channels/{channel_id}/team).
func (h *Runs) DisableChannelTeam(ctx context.Context, req rest.DisableChannelTeamRequestObject) (rest.DisableChannelTeamResponseObject, error) {
	t, err := h.teamOps()
	if err != nil {
		return nil, err
	}
	if err := t.Disable(ctx, ids.ID(req.ChannelId)); err != nil {
		return nil, err
	}
	return rest.DisableChannelTeam204Response{}, nil
}

// AddTeamMember implements rest.StrictServerInterface (POST /channels/{channel_id}/team/members).
func (h *Runs) AddTeamMember(ctx context.Context, req rest.AddTeamMemberRequestObject) (rest.AddTeamMemberResponseObject, error) {
	t, err := h.teamOps()
	if err != nil {
		return nil, err
	}
	b := req.Body
	m, err := t.AddMember(ctx, runs.AddTeamMember{
		ChannelID: ids.ID(req.ChannelId), AgentID: ids.ID(b.AgentId), HomeChannelID: ids.ID(b.HomeChannelId), Handle: b.Handle,
	})
	if err != nil {
		return nil, err
	}
	return rest.AddTeamMember201JSONResponse{Body: TeamMemberREST(m)}, nil
}

// RemoveTeamMember implements rest.StrictServerInterface (DELETE
// /channels/{channel_id}/team/members/{member_id}).
func (h *Runs) RemoveTeamMember(ctx context.Context, req rest.RemoveTeamMemberRequestObject) (rest.RemoveTeamMemberResponseObject, error) {
	t, err := h.teamOps()
	if err != nil {
		return nil, err
	}
	if err := t.RemoveMember(ctx, ids.ID(req.ChannelId), ids.ID(req.MemberId)); err != nil {
		return nil, err
	}
	return rest.RemoveTeamMember204Response{}, nil
}

// StopChannelTeam implements rest.StrictServerInterface (POST /channels/{channel_id}/team/stop).
func (h *Runs) StopChannelTeam(ctx context.Context, req rest.StopChannelTeamRequestObject) (rest.StopChannelTeamResponseObject, error) {
	t, err := h.teamOps()
	if err != nil {
		return nil, err
	}
	stopped, err := t.Stop(ctx, ids.ID(req.ChannelId))
	if err != nil {
		return nil, err
	}
	out := rest.TeamStopResult{StoppedRunIds: make([]rest.Uuid, 0, len(stopped))}
	for _, id := range stopped {
		out.StoppedRunIds = append(out.StoppedRunIds, rest.Uuid(id))
	}
	return rest.StopChannelTeam200JSONResponse{Body: out}, nil
}

// TeamREST converts a team to the schema Team.
func TeamREST(t runs.Team) rest.Team {
	out := rest.Team{
		ChannelId: rest.Uuid(t.ChannelID), SponsorId: rest.Uuid(t.SponsorID), MemberTurns: clampInt32(t.MemberTurns),
		TurnBudget: clampInt32(t.TurnBudget), Members: make([]rest.TeamMember, 0, len(t.Members)), CreatedAt: timestamp(t.CreatedAt),
		Version: t.Version,
	}
	if t.PauseReason != "" {
		r := rest.TeamPauseReason(t.PauseReason)
		out.PauseReason = &r
	}
	for _, m := range t.Members {
		out.Members = append(out.Members, TeamMemberREST(m))
	}
	return out
}

// TeamMemberREST converts a member to the schema TeamMember.
func TeamMemberREST(m runs.TeamMember) rest.TeamMember {
	return rest.TeamMember{
		Id: rest.Uuid(m.ID), AgentId: rest.Uuid(m.AgentID), HomeChannelId: rest.Uuid(m.HomeChannelID), WorkThreadId: rest.Uuid(m.WorkThreadID),
		Handle: m.Handle, CreatedAt: timestamp(m.CreatedAt),
	}
}

// clampInt32 bounds a count to the int32 of the schema.
func clampInt32(n int) int32 { return int32(min(max(n, 0), 1<<31-1)) }
