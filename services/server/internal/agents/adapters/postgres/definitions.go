// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/OWNER/bothy/services/server/internal/agents/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/agents/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Compile-time check: Repository is the definition store of the agents application layer.
var _ app.DefinitionStore = (*Repository)(nil)

// CurrentDefinition implements app.DefinitionStore.
func (r *Repository) CurrentDefinition(ctx context.Context, agentID ids.ID) (app.DefinitionRow, bool, error) {
	row, err := r.read(ctx).GetCurrentDefinition(ctx, uuid(agentID))
	return definitionRow(row, err, "reading the current definition")
}

// DefinitionAt implements app.DefinitionStore.
func (r *Repository) DefinitionAt(ctx context.Context, agentID ids.ID, version int64) (app.DefinitionRow, bool, error) {
	row, err := r.read(ctx).GetDefinitionAt(ctx, sqlcgen.GetDefinitionAtParams{AgentID: uuid(agentID), Version: version})
	return definitionRow(row, err, "reading a definition version")
}

// InsertDefinition implements app.DefinitionStore.
func (r *Repository) InsertDefinition(ctx context.Context, d app.DefinitionRow) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	err = q.InsertDefinition(ctx, sqlcgen.InsertDefinitionParams{
		AgentID: uuid(d.AgentID), Version: d.Version, Definition: d.Definition, OriginKind: d.OriginKind,
		CreatedAt: optionalTime(d.CreatedAt), CreatedBy: uuid(d.CreatedBy),
	})
	return mapError(err, "inserting a definition")
}

// DefinitionVersions implements app.DefinitionStore.
func (r *Repository) DefinitionVersions(ctx context.Context, agentID ids.ID, before int64, limit int) ([]app.DefinitionRow, error) {
	rows, err := r.read(ctx).ListDefinitionVersions(ctx, sqlcgen.ListDefinitionVersionsParams{
		AgentID: uuid(agentID), BeforeVersion: before, RowLimit: int32(min(max(limit, 1), 1<<30)),
	})
	if err != nil {
		return nil, mapError(err, "listing definition versions")
	}
	out := make([]app.DefinitionRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, app.DefinitionRow{
			AgentID: idOf(row.AgentID), Version: row.Version, OriginKind: row.OriginKind, CreatedAt: timeOf(row.CreatedAt), CreatedBy: idOf(row.CreatedBy),
		})
	}
	return out, nil
}

// definitionRow converts the result of a single-row definition query; no rows is found false.
func definitionRow(row sqlcgen.AgentDefinition, err error, what string) (app.DefinitionRow, bool, error) {
	if errors.Is(err, pgx.ErrNoRows) {
		return app.DefinitionRow{}, false, nil
	}
	if err != nil {
		return app.DefinitionRow{}, false, mapError(err, what)
	}
	return app.DefinitionRow{
		AgentID: idOf(row.AgentID), Version: row.Version, Definition: row.Definition, OriginKind: row.OriginKind,
		CreatedAt: timeOf(row.CreatedAt), CreatedBy: idOf(row.CreatedBy),
	}, true, nil
}
