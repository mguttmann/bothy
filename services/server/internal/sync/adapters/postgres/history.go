// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/OWNER/bothy/services/server/internal/sync"
	"github.com/OWNER/bothy/services/server/internal/sync/adapters/postgres/sqlcgen"
)

// Compile-time check: Repository is the history of the worker consumers.
var _ sync.History = (*Repository)(nil)

// ResourceEvents implements sync.History (queries/history.sql): the retained events of one
// resource with one of q.Types and seq below q.Before, ascending by seq. No types read nothing.
// It never authorizes; inside a unit of work it reads the ambient transaction, else the pool.
func (r *Repository) ResourceEvents(ctx context.Context, q sync.ResourceQuery) ([]sync.Event, error) {
	if len(q.Types) == 0 {
		return []sync.Event{}, nil
	}
	rows, err := r.queries(ctx).ResourceEvents(ctx, sqlcgen.ResourceEventsParams{
		ResourceKind: q.ResourceKind, ResourceID: uuid(q.ResourceID), Types: q.Types, BeforeSeq: q.Before,
	})
	if err != nil {
		return nil, err
	}
	return toEvents(rows), nil
}

// PersonalEvents implements sync.History (queries/history.sql): the retained events with
// audience q.AudienceID, one of q.Types and seq above q.After, ascending by seq. No types read
// nothing. It never authorizes; inside a unit of work it reads the ambient transaction, else the
// pool.
func (r *Repository) PersonalEvents(ctx context.Context, q sync.PersonalQuery) ([]sync.Event, error) {
	if len(q.Types) == 0 {
		return []sync.Event{}, nil
	}
	rows, err := r.queries(ctx).PersonalEvents(ctx, sqlcgen.PersonalEventsParams{
		AudiencePrincipalID: uuid(q.AudienceID), Types: q.Types, AfterSeq: q.After,
	})
	if err != nil {
		return nil, err
	}
	return toEvents(rows), nil
}

// LastPersonalEvent implements sync.History (queries/history.sql): the newest retained event
// with audience q.AudienceID on the resource of q, with one of q.Types and seq below q.Before;
// found is false when there is none, and no types find nothing. It never authorizes; inside a
// unit of work it reads the ambient transaction, else the pool.
func (r *Repository) LastPersonalEvent(ctx context.Context, q sync.PersonalResourceQuery) (sync.Event, bool, error) {
	if len(q.Types) == 0 {
		return sync.Event{}, false, nil
	}
	row, err := r.queries(ctx).LastPersonalEvent(ctx, sqlcgen.LastPersonalEventParams{
		AudiencePrincipalID: uuid(q.AudienceID), ResourceKind: q.ResourceKind, ResourceID: uuid(q.ResourceID),
		Types: q.Types, BeforeSeq: q.Before,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return sync.Event{}, false, nil
	}
	if err != nil {
		return sync.Event{}, false, err
	}
	return toEvent(row), true, nil
}

// toEvents maps rows with toEvent, the mapping of the log tail; never nil.
func toEvents(rows []sqlcgen.EventLog) []sync.Event {
	out := make([]sync.Event, len(rows))
	for i, row := range rows {
		out[i] = toEvent(row)
	}
	return out
}
