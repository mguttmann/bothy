// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/runs/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/runs/app"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// Compile-time check: the repository serves the team channels (FEAT-TEAM-CHANNEL-SERVER).
var _ app.TeamRepository = (*Repository)(nil)

// UpsertTeam implements app.TeamRepository.
func (r *Repository) UpsertTeam(ctx context.Context, channelID, sponsorID ids.ID, at time.Time) (app.TeamRow, error) {
	q, err := r.write(ctx)
	if err != nil {
		return app.TeamRow{}, err
	}
	row, err := q.UpsertTeamChannel(ctx, sqlcgen.UpsertTeamChannelParams{ChannelID: uuid(channelID), SponsorID: uuid(sponsorID), At: ts(at)})
	if err != nil {
		return app.TeamRow{}, mapError(err, "switching a channel to team mode")
	}
	return toTeam(row), nil
}

// Team implements app.TeamRepository.
func (r *Repository) Team(ctx context.Context, channelID ids.ID) (app.TeamRow, bool, error) {
	row, err := r.read(ctx).GetTeamChannel(ctx, uuid(channelID))
	return oneTeam(row, err, "reading a team")
}

// LockTeam implements app.TeamRepository.
func (r *Repository) LockTeam(ctx context.Context, channelID ids.ID) (app.TeamRow, bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return app.TeamRow{}, false, err
	}
	row, err := q.LockTeamChannel(ctx, uuid(channelID))
	return oneTeam(row, err, "locking a team")
}

// DeleteTeam implements app.TeamRepository.
func (r *Repository) DeleteTeam(ctx context.Context, channelID ids.ID) (bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return false, err
	}
	n, err := q.DeleteTeamChannel(ctx, uuid(channelID))
	return n > 0, mapError(err, "removing a team")
}

// SaveTeamBrake implements app.TeamRepository.
func (r *Repository) SaveTeamBrake(ctx context.Context, channelID ids.ID, b domain.TeamBrake, at time.Time) (app.TeamRow, error) {
	q, err := r.write(ctx)
	if err != nil {
		return app.TeamRow{}, err
	}
	row, err := q.UpdateTeamBrake(ctx, sqlcgen.UpdateTeamBrakeParams{
		MemberTurns: count32(b.MemberTurns), PingpongFrom: optionalUUID(b.From), PingpongTo: optionalUUID(b.To),
		PingpongCount: count32(b.PingpongCount), PauseReason: optionalText(b.PauseReason), At: ts(at), ChannelID: uuid(channelID),
	})
	if err != nil {
		return app.TeamRow{}, mapError(err, "storing the brake of a team")
	}
	return toTeam(row), nil
}

// InsertTeamMember implements app.TeamRepository.
func (r *Repository) InsertTeamMember(ctx context.Context, m app.TeamMemberRow) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	_, err = q.InsertTeamMember(ctx, sqlcgen.InsertTeamMemberParams{
		ID: uuid(m.ID), ChannelID: uuid(m.ChannelID), AgentID: uuid(m.AgentID), HomeChannelID: uuid(m.HomeChannelID),
		WorkThreadID: uuid(m.WorkThreadID), Handle: m.Handle, CreatedBy: uuid(m.CreatedBy), CreatedAt: ts(m.CreatedAt),
	})
	return mapError(err, "adding a team member")
}

// TeamMembers implements app.TeamRepository.
func (r *Repository) TeamMembers(ctx context.Context, channelID ids.ID) ([]app.TeamMemberRow, error) {
	rows, err := r.read(ctx).TeamMembers(ctx, uuid(channelID))
	if err != nil {
		return nil, mapError(err, "reading the members of a team")
	}
	out := make([]app.TeamMemberRow, len(rows))
	for i, row := range rows {
		out[i] = toTeamMember(row)
	}
	return out, nil
}

// TeamMemberByWorkThread implements app.TeamRepository.
func (r *Repository) TeamMemberByWorkThread(ctx context.Context, threadID ids.ID) (app.TeamMemberRow, bool, error) {
	row, err := r.read(ctx).TeamMemberByWorkThread(ctx, uuid(threadID))
	if errors.Is(err, pgx.ErrNoRows) {
		return app.TeamMemberRow{}, false, nil
	}
	if err != nil {
		return app.TeamMemberRow{}, false, mapError(err, "reading the member of a work thread")
	}
	return toTeamMember(row), true, nil
}

// DeleteTeamMember implements app.TeamRepository.
func (r *Repository) DeleteTeamMember(ctx context.Context, channelID, memberID ids.ID) (bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return false, err
	}
	n, err := q.DeleteTeamMember(ctx, sqlcgen.DeleteTeamMemberParams{ChannelID: uuid(channelID), ID: uuid(memberID)})
	return n > 0, mapError(err, "removing a team member")
}

// InsertTeamInput implements app.TeamRepository.
func (r *Repository) InsertTeamInput(ctx context.Context, in app.TeamInputRow) (bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return false, err
	}
	n, err := q.InsertTeamInput(ctx, sqlcgen.InsertTeamInputParams{
		MessageID: uuid(in.MessageID), MemberID: uuid(in.MemberID), SourceMessageID: uuid(in.SourceMessageID), Source: in.Source,
		SourceMemberID: optionalUUID(in.SourceMemberID), AuthorityID: uuid(in.AuthorityID), CreatedAt: ts(in.CreatedAt),
	})
	return n > 0, mapError(err, "recording a team input")
}

// TeamInput implements app.TeamRepository.
func (r *Repository) TeamInput(ctx context.Context, messageID ids.ID) (app.TeamInputRow, bool, error) {
	row, err := r.read(ctx).GetTeamInput(ctx, uuid(messageID))
	if errors.Is(err, pgx.ErrNoRows) {
		return app.TeamInputRow{}, false, nil
	}
	if err != nil {
		return app.TeamInputRow{}, false, mapError(err, "reading a team input")
	}
	return app.TeamInputRow{
		MessageID: idOf(row.MessageID), MemberID: idOf(row.MemberID), SourceMessageID: idOf(row.SourceMessageID),
		SourceMemberID: idOf(row.SourceMemberID), AuthorityID: idOf(row.AuthorityID), Source: row.Source, CreatedAt: timeOf(row.CreatedAt),
	}, true, nil
}

// TeamInputDelivered implements app.TeamRepository.
func (r *Repository) TeamInputDelivered(ctx context.Context, sourceMessageID, memberID ids.ID) (bool, error) {
	ok, err := r.read(ctx).TeamInputDelivered(ctx, sqlcgen.TeamInputDeliveredParams{SourceMessageID: uuid(sourceMessageID), MemberID: uuid(memberID)})
	return ok, mapError(err, "reading whether a source reached a member")
}

// InsertTeamMirror implements app.TeamRepository.
func (r *Repository) InsertTeamMirror(ctx context.Context, sourceMessageID, mirrorMessageID, memberID ids.ID, at time.Time) (bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return false, err
	}
	n, err := q.InsertTeamMirror(ctx, sqlcgen.InsertTeamMirrorParams{
		SourceMessageID: uuid(sourceMessageID), MirrorMessageID: uuid(mirrorMessageID), MemberID: optionalUUID(memberID), CreatedAt: ts(at),
	})
	return n > 0, mapError(err, "recording a team mirror")
}

// TeamMirrored implements app.TeamRepository.
func (r *Repository) TeamMirrored(ctx context.Context, sourceMessageID ids.ID) (bool, error) {
	_, err := r.read(ctx).GetTeamMirror(ctx, uuid(sourceMessageID))
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return err == nil, mapError(err, "reading a team mirror")
}

// count32 converts a count of the brake to its integer column: the configured limits keep it far
// below the bound, which only guards the conversion.
func count32(n int) int32 {
	switch {
	case n < 0:
		return 0
	case n > maxInt32:
		return maxInt32
	}
	return int32(n) // #nosec G115 -- bounded to [0, maxInt32] above
}

// oneTeam converts a single team result; no row is not found.
func oneTeam(row sqlcgen.TeamChannel, err error, what string) (app.TeamRow, bool, error) {
	if errors.Is(err, pgx.ErrNoRows) {
		return app.TeamRow{}, false, nil
	}
	if err != nil {
		return app.TeamRow{}, false, mapError(err, what)
	}
	return toTeam(row), true, nil
}

// toTeam converts a team_channels row.
func toTeam(row sqlcgen.TeamChannel) app.TeamRow {
	return app.TeamRow{
		ChannelID: idOf(row.ChannelID), SponsorID: idOf(row.SponsorID),
		Brake: domain.TeamBrake{
			MemberTurns: int(row.MemberTurns), From: idOf(row.PingpongFrom), To: idOf(row.PingpongTo), PingpongCount: int(row.PingpongCount),
			PauseReason: textOf(row.PauseReason),
		},
		CreatedAt: timeOf(row.CreatedAt), Version: row.Version,
	}
}

// textOf converts a nullable text column; NULL becomes "".
func textOf(t pgtype.Text) string {
	if !t.Valid {
		return ""
	}
	return t.String
}

// toTeamMember converts a team_members row.
func toTeamMember(row sqlcgen.TeamMember) app.TeamMemberRow {
	return app.TeamMemberRow{
		ID: idOf(row.ID), ChannelID: idOf(row.ChannelID), AgentID: idOf(row.AgentID), HomeChannelID: idOf(row.HomeChannelID),
		WorkThreadID: idOf(row.WorkThreadID), CreatedBy: idOf(row.CreatedBy), Handle: row.Handle, CreatedAt: timeOf(row.CreatedAt),
	}
}
