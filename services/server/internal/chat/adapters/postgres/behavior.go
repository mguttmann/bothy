// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"errors"
	"math"

	"github.com/jackc/pgx/v5"

	"github.com/OWNER/bothy/services/server/internal/chat/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/chat/app"
	"github.com/OWNER/bothy/services/server/internal/chat/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Compile-time check: the repository is the behaviour store of the use cases (M2-02).
var _ app.BehaviorStore = (*Repository)(nil)

// ChannelBehavior implements app.BehaviorStore.
func (r *Repository) ChannelBehavior(ctx context.Context, id ids.ID) (app.BehaviorRow, bool, error) {
	row, err := r.read(ctx).GetChannelBehavior(ctx, uuid(id))
	if errors.Is(err, pgx.ErrNoRows) {
		return app.BehaviorRow{}, false, nil
	}
	if err != nil {
		return app.BehaviorRow{}, false, mapError(err, "reading the behaviour of a channel")
	}
	return toBehavior(row), true, nil
}

// ServedBehaviors implements app.BehaviorStore.
func (r *Repository) ServedBehaviors(ctx context.Context, channels, spaces []ids.ID) ([]app.BehaviorRow, error) {
	rows, err := r.read(ctx).ListServedBehaviors(ctx, sqlcgen.ListServedBehaviorsParams{ChannelIds: uuids(channels), SpaceIds: uuids(spaces)})
	if err != nil {
		return nil, mapError(err, "listing the behaviour of served channels")
	}
	out := make([]app.BehaviorRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, toBehavior(sqlcgen.GetChannelBehaviorRow(row)))
	}
	return out, nil
}

// SaveBehavior implements app.BehaviorStore.
func (r *Repository) SaveBehavior(ctx context.Context, id ids.ID, b domain.Behavior) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	if b.HistoryBackfillLimit < 0 || b.HistoryBackfillLimit > math.MaxInt32 {
		return errs.New(errs.CodeInternalError, "chat: history_backfill_limit outside the integer column")
	}
	err = q.SaveChannelBehavior(ctx, sqlcgen.SaveChannelBehaviorParams{
		ID: uuid(id), RequireMention: b.RequireMention, ThreadRequireMention: b.ThreadRequireMention, FreeResponse: b.FreeResponse,
		AutoThread: b.AutoThread, AllowMentionedAgents: b.AllowMentionedAgents, HistoryBackfillLimit: int32(b.HistoryBackfillLimit),
		Prompt: b.Prompt,
	})
	return mapError(err, "writing the behaviour of a channel")
}

// ReplaceSkills implements app.BehaviorStore.
func (r *Repository) ReplaceSkills(ctx context.Context, id ids.ID, skills []string) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	if err := q.DeleteChannelSkills(ctx, uuid(id)); err != nil {
		return mapError(err, "deleting the skills of a channel")
	}
	if len(skills) == 0 {
		return nil
	}
	return mapError(q.InsertChannelSkills(ctx, sqlcgen.InsertChannelSkillsParams{ChannelID: uuid(id), SkillRefs: skills}), "inserting the skills of a channel")
}

// RaiseSettingsVersion implements app.BehaviorStore.
func (r *Repository) RaiseSettingsVersion(ctx context.Context, id ids.ID) (int64, error) {
	q, err := r.write(ctx)
	if err != nil {
		return 0, err
	}
	v, err := q.RaiseSettingsVersion(ctx, uuid(id))
	if err != nil {
		return 0, mapError(err, "raising settings_version")
	}
	return v, nil
}

// toBehavior converts a behaviour row; skills are never nil.
func toBehavior(row sqlcgen.GetChannelBehaviorRow) app.BehaviorRow {
	skills := row.Skills
	if skills == nil {
		skills = []string{}
	}
	return app.BehaviorRow{
		ChannelID: idOf(row.ID), SpaceID: idOf(row.SpaceID), SettingsVersion: row.SettingsVersion,
		Behavior: domain.Behavior{
			RequireMention: row.RequireMention, ThreadRequireMention: row.ThreadRequireMention, FreeResponse: row.FreeResponse,
			AutoThread: row.AutoThread, AllowMentionedAgents: row.AllowMentionedAgents, HistoryBackfillLimit: int(row.HistoryBackfillLimit),
			Prompt: row.Prompt, Skills: skills,
		},
	}
}
