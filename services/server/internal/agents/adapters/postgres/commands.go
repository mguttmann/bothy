// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/OWNER/bothy/services/server/internal/agents/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/agents/app"
	"github.com/OWNER/bothy/services/server/internal/agents/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Compile-time check: Repository is the command store of the agents application layer.
var _ app.CommandStore = (*Repository)(nil)

// sqlstateUniqueViolation is unique_violation (the unique partial index of commands).
const sqlstateUniqueViolation = "23505"

// The owner kinds of a stored command.
const (
	ownerAgent = "agent"
	ownerApp   = "app"
)

// optionJSON is one element of command_options.v1, exactly the schema CommandOption: choices and
// autocomplete are left out when absent (ADR 0053).
type optionJSON struct {
	Name         string       `json:"name"`
	Description  string       `json:"description"`
	Type         string       `json:"type"`
	Required     bool         `json:"required"`
	Choices      []choiceJSON `json:"choices,omitempty"`
	Autocomplete bool         `json:"autocomplete,omitempty"`
}

// choiceJSON is the schema CommandChoice.
type choiceJSON struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// encodeOptions returns the column value of the options.
func encodeOptions(opts []domain.CommandOption) ([]byte, error) {
	out := make([]optionJSON, 0, len(opts))
	for _, o := range opts {
		j := optionJSON{Name: o.Name, Description: o.Description, Type: o.Type, Required: o.Required, Autocomplete: o.Autocomplete}
		for _, c := range o.Choices {
			j.Choices = append(j.Choices, choiceJSON(c))
		}
		out = append(out, j)
	}
	b, err := json.Marshal(out)
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternalError, err, "agents: encoding command options")
	}
	return b, nil
}

// decodeOptions reads the column value; a value outside the schema is an internal error.
func decodeOptions(b []byte) ([]domain.CommandOption, error) {
	var in []optionJSON
	if err := json.Unmarshal(b, &in); err != nil {
		return nil, errs.Wrap(errs.CodeInternalError, err, "agents: stored command options outside their schema")
	}
	out := make([]domain.CommandOption, 0, len(in))
	for _, j := range in {
		o := domain.CommandOption{Name: j.Name, Description: j.Description, Type: j.Type, Required: j.Required, Autocomplete: j.Autocomplete}
		for _, c := range j.Choices {
			o.Choices = append(o.Choices, domain.CommandChoice(c))
		}
		out = append(out, o)
	}
	return out, nil
}

// toCommand converts a row with the kind of its owner.
func toCommand(c sqlcgen.Command, ownerIsAgent bool) (domain.Command, error) {
	opts, err := decodeOptions(c.Options)
	if err != nil {
		return domain.Command{}, err
	}
	kind := ownerApp
	if ownerIsAgent {
		kind = ownerAgent
	}
	out := domain.Command{
		ID: idOf(c.ID), OwnerID: idOf(c.OwnerPrincipalID), OwnerKind: kind, Version: c.Version, CreatedAt: c.CreatedAt.Time,
		Def: domain.CommandDefinition{
			Name: c.Name, Description: c.Description, ScopeKind: c.ScopeKind, ScopeSpaceID: idOf(c.ScopeSpaceID),
			ScopeChannelID: idOf(c.ScopeChannelID), Options: opts, RequiredLevel: c.RequiredLevel, Control: c.Control,
		},
	}
	if c.RemovedAt.Valid {
		out.RemovedAt = c.RemovedAt.Time
	}
	return out, nil
}

// Command implements app.CommandStore.
func (r *Repository) Command(ctx context.Context, id ids.ID) (domain.Command, bool, error) {
	row, err := r.read(ctx).GetCommand(ctx, uuid(id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Command{}, false, nil
	}
	if err != nil {
		return domain.Command{}, false, mapError(err, "reading a command")
	}
	c, err := toCommand(row.Command, row.OwnerIsAgent)
	return c, err == nil, err
}

// CountLiveCommands implements app.CommandStore.
func (r *Repository) CountLiveCommands(ctx context.Context, owner ids.ID) (int, error) {
	q, err := r.write(ctx)
	if err != nil {
		return 0, err
	}
	n, err := q.CountLiveCommands(ctx, uuid(owner))
	if err != nil {
		return 0, mapError(err, "counting commands")
	}
	return int(n), nil
}

// InsertCommand implements app.CommandStore: a violation of the unique partial index is
// app.ErrCommandTaken.
func (r *Repository) InsertCommand(ctx context.Context, c domain.Command) (domain.Command, error) {
	q, err := r.write(ctx)
	if err != nil {
		return domain.Command{}, err
	}
	opts, err := encodeOptions(c.Def.Options)
	if err != nil {
		return domain.Command{}, err
	}
	created, err := q.InsertCommand(ctx, sqlcgen.InsertCommandParams{
		ID: uuid(c.ID), OwnerPrincipalID: uuid(c.OwnerID), Name: c.Def.Name, Description: c.Def.Description,
		ScopeKind: c.Def.ScopeKind, ScopeSpaceID: optionalUUID(c.Def.ScopeSpaceID), ScopeChannelID: optionalUUID(c.Def.ScopeChannelID),
		Options: opts, RequiredLevel: c.Def.RequiredLevel, Control: c.Def.Control,
	})
	if err != nil {
		return domain.Command{}, commandWriteError(err, "inserting a command")
	}
	c.Version, c.CreatedAt = 1, created.Time
	return c, nil
}

// UpdateCommand implements app.CommandStore: changed is false when no live row has the id and
// version; a violation of the unique partial index is app.ErrCommandTaken.
func (r *Repository) UpdateCommand(ctx context.Context, id ids.ID, version int64, def domain.CommandDefinition) (int64, bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return 0, false, err
	}
	opts, err := encodeOptions(def.Options)
	if err != nil {
		return 0, false, err
	}
	v, err := q.UpdateCommandDefinition(ctx, sqlcgen.UpdateCommandDefinitionParams{
		ID: uuid(id), Version: version, Name: def.Name, Description: def.Description, ScopeKind: def.ScopeKind,
		ScopeSpaceID: optionalUUID(def.ScopeSpaceID), ScopeChannelID: optionalUUID(def.ScopeChannelID), Options: opts,
		RequiredLevel: def.RequiredLevel, Control: def.Control,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, commandWriteError(err, "updating a command")
	}
	return v, true, nil
}

// RemoveCommand implements app.CommandStore: changed is false when no live row has the id and
// version.
func (r *Repository) RemoveCommand(ctx context.Context, id ids.ID, version int64) (int64, bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return 0, false, err
	}
	row, err := q.RemoveCommand(ctx, sqlcgen.RemoveCommandParams{ID: uuid(id), Version: version})
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, mapError(err, "removing a command")
	}
	return row.Version, true, nil
}

// CoveringCommands implements app.CommandStore.
func (r *Repository) CoveringCommands(ctx context.Context, q app.CoveringQuery) ([]domain.Command, error) {
	p := sqlcgen.ListCoveringCommandsParams{SpaceID: optionalUUID(q.SpaceID), ChannelID: uuid(q.ChannelID),
		OwnerPrincipalID: optionalUUID(q.OwnerID)}
	if q.Name != "" {
		p.LowerName = pgtype.Text{String: strings.ToLower(q.Name), Valid: true}
	}
	rows, err := r.read(ctx).ListCoveringCommands(ctx, p)
	if err != nil {
		return nil, mapError(err, "listing commands")
	}
	out := make([]domain.Command, 0, len(rows))
	for _, row := range rows {
		c, err := toCommand(row.Command, row.OwnerIsAgent)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, nil
}

// commandWriteError maps a unique violation of commands to app.ErrCommandTaken.
func commandWriteError(err error, what string) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == sqlstateUniqueViolation {
		return app.ErrCommandTaken
	}
	return mapError(err, what)
}
