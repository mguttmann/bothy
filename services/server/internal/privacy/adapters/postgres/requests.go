// SPDX-License-Identifier: Apache-2.0

// Package postgres holds the storage adapter of the module privacy (brief M7-06): app.Store over
// gdpr_requests, the participant of the job retention.privacy_job and the participant of privacy
// in the export (the requests of the person). Only this module reads and writes gdpr_requests (03
// principle 12).
package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/privacy/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/privacy/app"
	"github.com/OWNER/bothy/services/server/internal/privacy/domain"
)

// sqlstateUniqueViolation is the SQLSTATE of the partial unique index of open requests.
const sqlstateUniqueViolation = "23505"

// Requests implements app.Store over gdpr_requests. Every method runs in the ambient unit of
// work of ctx when there is one. It is safe for concurrent use.
type Requests struct {
	pool *pg.Pool
}

// Compile-time check.
var _ app.Store = (*Requests)(nil)

// NewRequests returns the store over pool.
func NewRequests(pool *pg.Pool) *Requests { return &Requests{pool: pool} }

// q returns the queries on the ambient transaction of ctx or on the pool.
func (s *Requests) q(ctx context.Context) *sqlcgen.Queries {
	return sqlcgen.New(pg.Querier(ctx, s.pool))
}

// Insert implements app.Store. Errors: app.ErrOpen for a second open request.
func (s *Requests) Insert(ctx context.Context, r domain.Request) error {
	progress, err := domain.MarshalProgress(r.Progress)
	if err != nil {
		return err
	}
	err = s.q(ctx).InsertRequest(ctx, sqlcgen.InsertRequestParams{
		ID: uuid(r.ID), SubjectUserID: uuid(r.SubjectUserID), Kind: string(r.Kind), State: string(r.State),
		RequestedBy: uuid(r.RequestedBy), ApprovedBy: optUUID(r.ApprovedBy), Reason: r.Reason, DueAt: ts(r.DueAt),
		Progress: progress, CreatedAt: ts(r.CreatedAt),
	})
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == sqlstateUniqueViolation {
		return app.ErrOpen
	}
	return err
}

// OpenOf implements app.Store.
func (s *Requests) OpenOf(ctx context.Context, subject ids.ID, kind domain.Kind) (domain.Request, bool, error) {
	return one(s.q(ctx).OpenRequestOf(ctx, sqlcgen.OpenRequestOfParams{SubjectUserID: uuid(subject), Kind: string(kind)}))
}

// Get implements app.Store.
func (s *Requests) Get(ctx context.Context, id ids.ID) (domain.Request, bool, error) {
	return one(s.q(ctx).GetRequest(ctx, uuid(id)))
}

// Lock implements app.Store.
func (s *Requests) Lock(ctx context.Context, id ids.ID) (domain.Request, bool, error) {
	return one(s.q(ctx).LockRequest(ctx, uuid(id)))
}

// Update implements app.Store.
func (s *Requests) Update(ctx context.Context, r domain.Request) error {
	progress, err := domain.MarshalProgress(r.Progress)
	if err != nil {
		return err
	}
	return s.q(ctx).UpdateRequest(ctx, sqlcgen.UpdateRequestParams{
		ID: uuid(r.ID), State: string(r.State), ApprovedBy: optUUID(r.ApprovedBy), DueAt: ts(r.DueAt),
		ResultFileID: optUUID(r.ResultFileID), ResultExpiresAt: optTS(r.ResultExpiresAt), Progress: progress, Error: text(r.Error),
		CompletedAt: optTS(r.CompletedAt),
	})
}

// List implements app.Store.
func (s *Requests) List(ctx context.Context, f app.ListFilter, after *app.Key, limit int) ([]domain.Request, error) {
	p := sqlcgen.ListRequestsParams{SubjectUserID: optUUID(f.Subject), State: text(string(f.State)), RowLimit: int32(min(limit, 1<<20))} // #nosec G115 -- bounded
	if after != nil {
		p.AfterCreatedAt, p.AfterID = ts(after.CreatedAt), uuid(after.ID)
	}
	return many(s.q(ctx).ListRequests(ctx, p))
}

// ReminderCandidates implements app.Store.
func (s *Requests) ReminderCandidates(ctx context.Context, until time.Time, limit int) ([]domain.Request, error) {
	return many(s.q(ctx).ReminderCandidates(ctx, sqlcgen.ReminderCandidatesParams{Until: ts(until), RowLimit: int32(min(limit, 1<<20))})) // #nosec G115 -- bounded
}

// ExpiredPackages implements app.Store.
func (s *Requests) ExpiredPackages(ctx context.Context, now time.Time, limit int) ([]domain.Request, error) {
	return many(s.q(ctx).ExpiredPackages(ctx, sqlcgen.ExpiredPackagesParams{Now: ts(now), RowLimit: int32(min(limit, 1<<20))})) // #nosec G115 -- bounded
}

// one converts a single row; pgx.ErrNoRows is not found.
func one(row sqlcgen.GdprRequest, err error) (domain.Request, bool, error) {
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Request{}, false, nil
	}
	if err != nil {
		return domain.Request{}, false, err
	}
	r, err := toRequest(row)
	return r, err == nil, err
}

// many converts rows.
func many(rows []sqlcgen.GdprRequest, err error) ([]domain.Request, error) {
	if err != nil {
		return nil, err
	}
	out := make([]domain.Request, 0, len(rows))
	for _, row := range rows {
		r, err := toRequest(row)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, nil
}

// toRequest converts a row.
func toRequest(row sqlcgen.GdprRequest) (domain.Request, error) {
	p, err := domain.UnmarshalProgress(row.Progress)
	if err != nil {
		return domain.Request{}, err
	}
	return domain.Request{
		ID: row.ID.Bytes, SubjectUserID: row.SubjectUserID.Bytes, Kind: domain.Kind(row.Kind), State: domain.State(row.State),
		RequestedBy: row.RequestedBy.Bytes, ApprovedBy: optID(row.ApprovedBy), Reason: row.Reason, DueAt: row.DueAt.Time.UTC(),
		ResultFileID: optID(row.ResultFileID), ResultExpiresAt: optTime(row.ResultExpiresAt), Progress: p, Error: row.Error.String,
		CreatedAt: row.CreatedAt.Time.UTC(), CompletedAt: optTime(row.CompletedAt),
	}, nil
}

func uuid(id ids.ID) pgtype.UUID { return pgtype.UUID{Bytes: id, Valid: true} }

func optUUID(id ids.ID) pgtype.UUID {
	if id.IsZero() {
		return pgtype.UUID{}
	}
	return uuid(id)
}

func optID(u pgtype.UUID) ids.ID {
	if !u.Valid {
		return ids.ID{}
	}
	return u.Bytes
}

func ts(t time.Time) pgtype.Timestamptz { return pgtype.Timestamptz{Time: t, Valid: true} }

func optTS(t time.Time) pgtype.Timestamptz {
	if t.IsZero() {
		return pgtype.Timestamptz{}
	}
	return ts(t)
}

func optTime(t pgtype.Timestamptz) time.Time {
	if !t.Valid {
		return time.Time{}
	}
	return t.Time.UTC()
}

func text(s string) pgtype.Text {
	if s == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: s, Valid: true}
}
