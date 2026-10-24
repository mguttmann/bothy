// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/OWNER/bothy/services/server/internal/agents"
	"github.com/OWNER/bothy/services/server/internal/agents/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/agents/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
)

// sessionRefConstraint is the unique index whose violation (sqlstateUniqueViolation) in
// UpdateSession means "the reference is current in another thread" (00069, the partial index
// (agent_id, external_session_ref) WHERE is_current).
const sessionRefConstraint = "agent_sessions_agent_ref_current_key"

// SessionStore implements app.SessionStore over agent_sessions (03, M2-12) with the generated
// queries of sqlcgen. It is safe for concurrent use.
type SessionStore struct {
	pool *pg.Pool
}

// Compile-time check: SessionStore is the session store of the agents application layer.
var _ app.SessionStore = (*SessionStore)(nil)

// NewSessionStore returns the session store over pool. Reads outside a unit of work use pool;
// reads inside one, the locks and the writes use its transaction.
//
// Example:
//
//	sessions := postgres.NewSessionStore(pool)
func NewSessionStore(pool *pg.Pool) *SessionStore { return &SessionStore{pool: pool} }

// read returns the queries for a read: the ambient transaction of ctx or the pool.
func (s *SessionStore) read(ctx context.Context) *sqlcgen.Queries {
	return sqlcgen.New(pg.Querier(ctx, s.pool))
}

// write returns the queries for a write or a lock: only the ambient transaction of ctx.
func (s *SessionStore) write(ctx context.Context) (*sqlcgen.Queries, error) {
	tx, err := pg.Tx(ctx)
	if err != nil {
		return nil, err
	}
	return sqlcgen.New(tx), nil
}

// ThreadSessions implements app.SessionStore.
func (s *SessionStore) ThreadSessions(ctx context.Context, threadID ids.ID) ([]agents.Session, error) {
	rows, err := s.read(ctx).ThreadSessions(ctx, uuid(threadID))
	return sessions(rows, err, "reading the sessions of a thread")
}

// CurrentSession implements app.SessionStore.
func (s *SessionStore) CurrentSession(ctx context.Context, threadID ids.ID) (agents.Session, bool, error) {
	row, err := s.read(ctx).CurrentSession(ctx, uuid(threadID))
	return session(row, err, "reading the current session")
}

// LockCurrentSession implements app.SessionStore.
func (s *SessionStore) LockCurrentSession(ctx context.Context, threadID ids.ID) (agents.Session, bool, error) {
	q, err := s.write(ctx)
	if err != nil {
		return agents.Session{}, false, err
	}
	row, err := q.LockCurrentSession(ctx, uuid(threadID))
	return session(row, err, "locking the current session")
}

// LockSessions implements app.SessionStore: the ids are sorted and deduplicated, and the query
// orders by id, so the locks are taken in ascending id (03 section 6.3).
func (s *SessionStore) LockSessions(ctx context.Context, list []ids.ID) ([]agents.Session, error) {
	q, err := s.write(ctx)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, nil
	}
	rows, err := q.LockSessions(ctx, sortedKeys(list))
	return sessions(rows, err, "locking sessions")
}

// CurrentByRef implements app.SessionStore.
func (s *SessionStore) CurrentByRef(ctx context.Context, agentID ids.ID, ref string) (agents.Session, bool, error) {
	row, err := s.read(ctx).CurrentByRef(ctx, sqlcgen.CurrentByRefParams{AgentID: uuid(agentID), ExternalSessionRef: ref})
	return session(row, err, "reading the current session of a reference")
}

// LatestByRef implements app.SessionStore.
func (s *SessionStore) LatestByRef(ctx context.Context, agentID ids.ID, ref string) (agents.Session, bool, error) {
	row, err := s.read(ctx).LatestByRef(ctx, sqlcgen.LatestByRefParams{AgentID: uuid(agentID), ExternalSessionRef: ref})
	return session(row, err, "reading the latest session of a reference")
}

// LatestInThreadByRef implements app.SessionStore.
func (s *SessionStore) LatestInThreadByRef(ctx context.Context, threadID, agentID ids.ID, ref string) (agents.Session, bool, error) {
	row, err := s.read(ctx).LatestInThreadByRef(ctx, sqlcgen.LatestInThreadByRefParams{
		ThreadID: uuid(threadID), AgentID: uuid(agentID), ExternalSessionRef: ref,
	})
	return session(row, err, "reading the latest session of a reference in a thread")
}

// InsertSession implements app.SessionStore: ON CONFLICT DO NOTHING returns no row when a
// partial unique index refused it, which is inserted false and never an error. Version 0 is the
// column default 1 of 03.
func (s *SessionStore) InsertSession(ctx context.Context, n app.NewSession) (agents.Session, bool, error) {
	q, err := s.write(ctx)
	if err != nil {
		return agents.Session{}, false, err
	}
	version := n.Version
	if version == 0 {
		version = 1
	}
	row, err := q.InsertSession(ctx, sqlcgen.InsertSessionParams{
		ID: uuid(n.ID), ThreadID: uuid(n.ThreadID), AgentID: uuid(n.AgentID), ExternalSessionRef: n.ExternalSessionRef,
		IsCurrent: n.IsCurrent, Started: n.Started, Model: optionalText(n.Model), CostBaselineMicros: n.CostBaselineMicros,
		Currency: optionalText(n.Currency), CreatedAt: pgtype.Timestamptz{Time: n.CreatedAt, Valid: true},
		EndedAt: optionalTime(n.EndedAt), EndReason: optionalText(n.EndReason), Version: version,
	})
	return session(row, err, "inserting a session")
}

// EndSession implements app.SessionStore.
func (s *SessionStore) EndSession(ctx context.Context, e app.SessionEnd) (agents.Session, bool, error) {
	q, err := s.write(ctx)
	if err != nil {
		return agents.Session{}, false, err
	}
	row, err := q.EndSession(ctx, sqlcgen.EndSessionParams{
		ID: uuid(e.ID), Version: e.Version, EndReason: pgtype.Text{String: e.Reason, Valid: true},
		EndedAt: pgtype.Timestamptz{Time: e.At, Valid: true},
	})
	return session(row, err, "ending a session")
}

// UpdateSession implements app.SessionStore. The unique violation of a reference current in
// another thread fails the statement and with it the transaction, so the caller ends the unit
// of work with its conflict.
func (s *SessionStore) UpdateSession(ctx context.Context, c app.SessionChange) (agents.Session, bool, error) {
	q, err := s.write(ctx)
	if err != nil {
		return agents.Session{}, false, err
	}
	row, err := q.UpdateSession(ctx, sqlcgen.UpdateSessionParams{
		ID: uuid(c.ID), Version: c.Version, ExternalSessionRef: c.ExternalSessionRef, Model: optionalText(c.Model), Started: c.Started,
	})
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == sqlstateUniqueViolation && pgErr.ConstraintName == sessionRefConstraint {
		return agents.Session{}, false, fmt.Errorf("%w", app.ErrSessionRefTaken)
	}
	return session(row, err, "updating a session")
}

// SetSessionCost implements app.SessionStore.
func (s *SessionStore) SetSessionCost(ctx context.Context, c app.SessionCost) (agents.Session, bool, error) {
	q, err := s.write(ctx)
	if err != nil {
		return agents.Session{}, false, err
	}
	row, err := q.SetSessionCost(ctx, sqlcgen.SetSessionCostParams{
		ID: uuid(c.ID), Version: c.Version, CumulativeCostMicros: c.CumulativeCostMicros, Currency: c.Currency,
	})
	return session(row, err, "writing the cost of a session")
}

// session converts the result of a single-row query; no rows is found (or inserted, changed)
// false.
func session(row sqlcgen.AgentSession, err error, what string) (agents.Session, bool, error) {
	if errors.Is(err, pgx.ErrNoRows) {
		return agents.Session{}, false, nil
	}
	if err != nil {
		return agents.Session{}, false, mapError(err, what)
	}
	return toSession(row), true, nil
}

// sessions converts the result of a list query.
func sessions(rows []sqlcgen.AgentSession, err error, what string) ([]agents.Session, error) {
	if err != nil {
		return nil, mapError(err, what)
	}
	out := make([]agents.Session, 0, len(rows))
	for _, row := range rows {
		out = append(out, toSession(row))
	}
	return out, nil
}

// toSession converts a row; NULL becomes the zero value. dev_environment_id is not carried
// (agents.Session).
func toSession(row sqlcgen.AgentSession) agents.Session {
	return agents.Session{
		ID: idOf(row.ID), ThreadID: idOf(row.ThreadID), AgentID: idOf(row.AgentID), ExternalSessionRef: row.ExternalSessionRef,
		IsCurrent: row.IsCurrent, Started: row.Started, Model: row.Model.String,
		CumulativeCostMicros: row.CumulativeCostMicros, CostBaselineMicros: row.CostBaselineMicros,
		Currency: row.Currency, CostIsEstimate: row.CostIsEstimate,
		CreatedAt: timeOf(row.CreatedAt), EndedAt: timeOf(row.EndedAt), EndReason: row.EndReason.String, Version: row.Version,
	}
}
