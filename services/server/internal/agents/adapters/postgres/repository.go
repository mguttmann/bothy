// SPDX-License-Identifier: Apache-2.0

// Package postgres is the PostgreSQL adapter of the agents module (M1-51): the sqlc queries in
// queries/, the generated code in sqlcgen/ and Repository, which implements app.Store over the
// table agents (docs/spec/03-datenmodell.md section 4.4) and app.ConsumerStore over
// agent_consumer_cursors (M2-07). Only this package runs SQL on them (03 Grundsatz 12).
//
// Example:
//
//	repo := postgres.NewRepository(pool)
//	svc, err := app.New(cfg, app.Deps{Store: repo, ...})
package postgres

import (
	"bytes"
	"context"
	"errors"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/OWNER/bothy/services/server/internal/agents"
	"github.com/OWNER/bothy/services/server/internal/agents/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/agents/app"
	"github.com/OWNER/bothy/services/server/internal/agents/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
)

// Repository implements app.Store with the generated queries of sqlcgen. It is safe for
// concurrent use.
type Repository struct {
	pool *pg.Pool
}

// Compile-time check: Repository is the store of the agents application layer.
var _ app.Store = (*Repository)(nil)

// NewRepository returns the repository over pool. Reads outside a unit of work use pool; reads
// and writes inside one use its transaction.
func NewRepository(pool *pg.Pool) *Repository { return &Repository{pool: pool} }

// read returns the queries for a read: the ambient transaction of ctx or the pool.
func (r *Repository) read(ctx context.Context) *sqlcgen.Queries {
	return sqlcgen.New(pg.Querier(ctx, r.pool))
}

// write returns the queries for a write or a lock: only the ambient transaction of ctx.
func (r *Repository) write(ctx context.Context) (*sqlcgen.Queries, error) {
	tx, err := pg.Tx(ctx)
	if err != nil {
		return nil, err
	}
	return sqlcgen.New(tx), nil
}

// Agent implements app.Store.
func (r *Repository) Agent(ctx context.Context, id ids.ID) (domain.Agent, bool, error) {
	row, err := r.read(ctx).GetAgent(ctx, uuid(id))
	return one(row, err, "reading an agent")
}

// LockAgent implements app.Store.
func (r *Repository) LockAgent(ctx context.Context, id ids.ID) (domain.Agent, bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return domain.Agent{}, false, err
	}
	row, err := q.LockAgent(ctx, uuid(id))
	return one(row, err, "locking an agent")
}

// LockAgents implements app.Store.
func (r *Repository) LockAgents(ctx context.Context, list []ids.ID) ([]domain.Agent, error) {
	q, err := r.write(ctx)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, nil
	}
	rows, err := q.LockAgents(ctx, sortedKeys(list))
	return many(rows, err, "locking agents")
}

// Agents implements app.Store: the list is sorted and deduplicated like LockAgents, so the
// statement sees each id once.
func (r *Repository) Agents(ctx context.Context, list []ids.ID) ([]domain.Agent, error) {
	if len(list) == 0 {
		return nil, nil
	}
	rows, err := r.read(ctx).GetAgents(ctx, sortedKeys(list))
	return many(rows, err, "reading agents")
}

// sortedKeys returns the ids of list ascending and without duplicates as query parameters.
func sortedKeys(list []ids.ID) []pgtype.UUID {
	keys := make([]pgtype.UUID, 0, len(list))
	for _, id := range slices.Compact(slices.SortedFunc(slices.Values(list), func(a, b ids.ID) int { return bytes.Compare(a[:], b[:]) })) {
		keys = append(keys, uuid(id))
	}
	return keys
}

// LockPersonalAgents implements app.Store.
func (r *Repository) LockPersonalAgents(ctx context.Context) ([]domain.Agent, error) {
	q, err := r.write(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := q.LockPersonalAgents(ctx)
	return many(rows, err, "locking the personal agents")
}

// ListAgents implements app.Store.
func (r *Repository) ListAgents(ctx context.Context, after ids.ID, limit int) ([]domain.Agent, error) {
	rows, err := r.read(ctx).ListAgents(ctx, sqlcgen.ListAgentsParams{
		AfterID: optionalUUID(after), RowLimit: int32(min(max(limit, 1), 1<<30)),
	})
	return many(rows, err, "listing agents")
}

// PersonalAgents implements app.Store.
func (r *Repository) PersonalAgents(ctx context.Context) ([]domain.Agent, error) {
	rows, err := r.read(ctx).ListPersonalAgents(ctx)
	return many(rows, err, "reading the personal agents")
}

// InsertAgent implements app.Store.
func (r *Repository) InsertAgent(ctx context.Context, a domain.Agent) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	err = q.InsertAgent(ctx, sqlcgen.InsertAgentParams{
		PrincipalID: uuid(a.ID), AdapterKind: a.AdapterKind, Description: a.Description, OperatorConfig: a.Config.JSON(),
		BillingMode: string(a.Policy.Mode), CredentialHolderID: optionalUUID(a.Policy.HolderID), ToolClass: string(a.ToolClass),
		Version: a.Version,
	})
	return mapError(err, "inserting an agent")
}

// UpdateAgent implements app.Store.
func (r *Repository) UpdateAgent(ctx context.Context, a domain.Agent) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	err = q.UpdateAgent(ctx, sqlcgen.UpdateAgentParams{
		PrincipalID: uuid(a.ID), Description: a.Description, OperatorConfig: a.Config.JSON(), BillingMode: string(a.Policy.Mode),
		CredentialHolderID: optionalUUID(a.Policy.HolderID), BillingModeChangedAt: optionalTime(a.BillingModeChangedAt),
		BillingModeChangedBy: optionalUUID(a.BillingModeChangedBy), ToolClass: string(a.ToolClass), Version: a.Version,
	})
	return mapError(err, "updating an agent")
}

// one converts the result of a single-row query; no rows is found false.
func one(row sqlcgen.Agent, err error, what string) (domain.Agent, bool, error) {
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Agent{}, false, nil
	}
	if err != nil {
		return domain.Agent{}, false, mapError(err, what)
	}
	a, err := toAgent(row)
	return a, err == nil, err
}

// many converts the result of a list query.
func many(rows []sqlcgen.Agent, err error, what string) ([]domain.Agent, error) {
	if err != nil {
		return nil, mapError(err, what)
	}
	out := make([]domain.Agent, 0, len(rows))
	for _, row := range rows {
		a, err := toAgent(row)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, nil
}

// toAgent converts a row. The checks of the table hold the enumerations; the two JSON columns
// are read through their schemas, and a stored value outside them is an internal error.
func toAgent(row sqlcgen.Agent) (domain.Agent, error) {
	cfg, err := domain.OperatorConfigFromStore(row.OperatorConfig)
	if err != nil {
		return domain.Agent{}, errs.Wrap(errs.CodeInternalError, err, "agents: a stored operator_config outside its schema")
	}
	modes, err := domain.ReportedDetailsFromStore(row.ReportedDetails)
	if err != nil {
		return domain.Agent{}, errs.Wrap(errs.CodeInternalError, err, "agents: stored reported_details outside its schema")
	}
	return domain.Agent{
		ID: idOf(row.PrincipalID), AdapterKind: row.AdapterKind, Description: row.Description, Config: cfg,
		Policy:               domain.Policy{Mode: domain.BillingMode(row.BillingMode), HolderID: idOf(row.CredentialHolderID)},
		BillingModeChangedAt: timeOf(row.BillingModeChangedAt), BillingModeChangedBy: idOf(row.BillingModeChangedBy),
		ToolClass: domain.ToolClass(row.ToolClass),
		Reported: domain.Reported{
			State: domain.RuntimeState(row.ReportedRuntimeState), BillingMode: domain.BillingMode(row.ReportedBillingMode.String),
			ToolClass: domain.ToolClass(row.ReportedToolClass.String), RuntimeVersion: row.ReportedRuntimeVersion.String,
			LastHeartbeatAt: timeOf(row.LastHeartbeatAt), BillingModesSupported: modes,
		},
		Version: row.Version,
	}, nil
}

// SQLSTATEs the adapter maps to client errors.
const (
	sqlstateCheckViolation      = "23514"
	sqlstateForeignKeyViolation = "23503"
)

// mapError turns a database error of the operation what into a kernel/errs error: a check or
// foreign key violation is validation_failed with the field of the column, never SQL text, a
// constraint name or a value; a kernel/errs error passes; anything else is internal.
func mapError(err error, what string) error {
	if err == nil {
		return nil
	}
	if _, ok := errs.As(err); ok {
		return err
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && (pgErr.Code == sqlstateCheckViolation || pgErr.Code == sqlstateForeignKeyViolation) {
		field := "/" + pgErr.ColumnName
		if pgErr.ColumnName == "" {
			field = constraintFields[pgErr.ConstraintName]
		}
		if field != "" && field != "/" {
			return errs.Wrap(errs.CodeValidationFailed, err, "the value of "+field[1:]+" is not allowed").
				WithFields(errs.FieldError{Pointer: field, Code: "invalid", Detail: "not allowed"})
		}
	}
	return errs.Wrap(errs.CodeInternalError, err, "agents: "+what+" failed")
}

// constraintFields maps the named checks and foreign keys of agents to the field they guard.
//
//nolint:gosec // G101: constraint names and JSON pointers, no credentials.
var constraintFields = map[string]string{
	"agents_credential_holder_check":      "/credential_holder_id",
	"agents_credential_holder_id_fkey":    "/credential_holder_id",
	"agents_billing_mode_check":           "/billing_mode",
	"agents_tool_class_check":             "/tool_class",
	"agents_principal_id_fkey":            "/principal_id",
	"agents_billing_mode_changed_by_fkey": "/billing_mode_changed_by",
	"agents_reported_runtime_state_check": "/reported_runtime_state",
	"agents_reported_billing_mode_check":  "/reported_billing_mode",
	"agents_reported_tool_class_check":    "/reported_tool_class",
}

// uuid converts an id for a parameter.
func uuid(id ids.ID) pgtype.UUID { return pgtype.UUID{Bytes: id, Valid: true} }

// optionalUUID converts an optional id: the zero ID becomes NULL.
func optionalUUID(id ids.ID) pgtype.UUID { return pgtype.UUID{Bytes: id, Valid: !id.IsZero()} }

// optionalTime converts an optional time: the zero time becomes NULL.
func optionalTime(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: !t.IsZero()}
}

// idOf converts a column; NULL becomes the zero ID.
func idOf(u pgtype.UUID) ids.ID {
	if !u.Valid {
		return ids.ID{}
	}
	return ids.ID(u.Bytes)
}

// timeOf converts a column; NULL becomes the zero time. Times are UTC (03 Grundsatz 2).
func timeOf(t pgtype.Timestamptz) time.Time {
	if !t.Valid {
		return time.Time{}
	}
	return t.Time.UTC()
}

// Compile-time check: Repository is the consumer store of the agents application layer.
var _ app.ConsumerStore = (*Repository)(nil)

// TakeConsumer implements app.ConsumerStore.
func (r *Repository) TakeConsumer(ctx context.Context, key agents.ConsumerKey, holder ids.ID, head int64) (int64, bool, error) {
	row, err := r.read(ctx).TakeConsumer(ctx, sqlcgen.TakeConsumerParams{
		AgentID: uuid(key.AgentID), ConsumerName: key.Consumer, HeadSeq: head, HolderSessionID: uuid(holder),
	})
	if err != nil {
		return 0, false, mapError(err, "taking a consumer over")
	}
	return row.AckedSeq, row.Created, nil
}

// SaveConsumerAck implements app.ConsumerStore.
func (r *Repository) SaveConsumerAck(ctx context.Context, key agents.ConsumerKey, holder ids.ID, acked int64) (bool, error) {
	n, err := r.read(ctx).SaveConsumerAck(ctx, sqlcgen.SaveConsumerAckParams{
		AckedSeq: acked, AgentID: uuid(key.AgentID), ConsumerName: key.Consumer, HolderSessionID: uuid(holder),
	})
	if err != nil {
		return false, mapError(err, "saving a consumer acknowledgement")
	}
	return n > 0, nil
}

// RestartConsumer implements app.ConsumerStore.
func (r *Repository) RestartConsumer(ctx context.Context, key agents.ConsumerKey, holder ids.ID, seq int64) (bool, error) {
	n, err := r.read(ctx).RestartConsumer(ctx, sqlcgen.RestartConsumerParams{
		AckedSeq: seq, AgentID: uuid(key.AgentID), ConsumerName: key.Consumer, HolderSessionID: uuid(holder),
	})
	if err != nil {
		return false, mapError(err, "restarting a consumer")
	}
	return n > 0, nil
}

// AckConsumer implements app.ConsumerStore.
func (r *Repository) AckConsumer(ctx context.Context, key agents.ConsumerKey, seq int64) (int64, bool, error) {
	acked, err := r.read(ctx).AckConsumer(ctx, sqlcgen.AckConsumerParams{AckedSeq: seq, AgentID: uuid(key.AgentID), ConsumerName: key.Consumer})
	return consumerRow(acked, err, "acknowledging consumer events")
}

// GetConsumer implements app.ConsumerStore.
func (r *Repository) GetConsumer(ctx context.Context, key agents.ConsumerKey) (int64, bool, error) {
	acked, err := r.read(ctx).GetConsumer(ctx, sqlcgen.GetConsumerParams{AgentID: uuid(key.AgentID), ConsumerName: key.Consumer})
	return consumerRow(acked, err, "reading a consumer")
}

// consumerRow maps the result of a one-row consumer query: no row is found false.
func consumerRow(acked int64, err error, what string) (int64, bool, error) {
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return 0, false, nil
	case err != nil:
		return 0, false, mapError(err, what)
	}
	return acked, true, nil
}

// MoveAllConsumers sets acked_seq of every consumer to headSeq in the unit of work of ctx
// (app.CursorMoveStore, bothy restore finalize).
func (r *Repository) MoveAllConsumers(ctx context.Context, headSeq int64) (int64, error) {
	tx, err := pg.Tx(ctx)
	if err != nil {
		return 0, err
	}
	return sqlcgen.New(tx).MoveAllConsumers(ctx, headSeq)
}

// Compile-time check: Repository moves the cursors of bothy restore finalize.
var _ app.CursorMoveStore = (*Repository)(nil)
