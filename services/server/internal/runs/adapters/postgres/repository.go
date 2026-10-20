// SPDX-License-Identifier: Apache-2.0

// Package postgres is the PostgreSQL adapter of the runs module (M2-05): the sqlc queries of runs
// and run_steps in queries/, the generated code in sqlcgen/ and Repository, which implements
// app.Repository over both tables (docs/spec/03-datenmodell.md section 4.8). Only this package runs
// SQL on them (03 Grundsatz 12). The tables have no channel_id; every read follows the visibility
// rule of docs/DECISIONS.md D-M2-05-9, which tools/lint/sql_visibility.yaml records per query.
//
// Example:
//
//	repo := postgres.NewRepository(pool)
//	row, inserted, err := repo.InsertRun(ctx, app.NewRun{...}) // inside a unit of work
package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/runs/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/runs/app"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// Repository implements app.Repository with the generated queries of sqlcgen. Reads join the
// ambient transaction of ctx or use the pool; writes need the ambient transaction. It is safe for
// concurrent use.
type Repository struct {
	pool *pg.Pool
}

// Compile-time check: Repository is the outgoing port of the run rows.
var _ app.Repository = (*Repository)(nil)

// NewRepository returns the repository over pool, a pool of bothy_app.
func NewRepository(pool *pg.Pool) *Repository { return &Repository{pool: pool} }

// read returns the queries for a read: the ambient transaction of ctx or the pool.
func (r *Repository) read(ctx context.Context) *sqlcgen.Queries {
	return sqlcgen.New(pg.Querier(ctx, r.pool))
}

// write returns the queries for a write: only the ambient transaction of ctx, because every write
// of a run belongs to the unit of work that appends its events.
func (r *Repository) write(ctx context.Context) (*sqlcgen.Queries, error) {
	tx, err := pg.Tx(ctx)
	if err != nil {
		return nil, err
	}
	return sqlcgen.New(tx), nil
}

// InsertRun implements app.Repository. The run starts in queued with version 1 and queued_at =
// r.QueuedAt; no returned row means the thread has an active run (the ON CONFLICT of
// runs_thread_active_key), which gives inserted false and no error.
func (r *Repository) InsertRun(ctx context.Context, n app.NewRun) (app.RunRow, bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return app.RunRow{}, false, err
	}
	row, err := q.InsertRun(ctx, sqlcgen.InsertRunParams{
		ID: uuid(n.ID), ThreadID: uuid(n.ThreadID), AgentID: uuid(n.AgentID), TriggerKind: n.TriggerKind,
		TriggeringMessageID: optionalUUID(n.TriggeringMessageID), TriggeringCommandID: optionalUUID(n.TriggeringCommandID),
		TriggeredBy: uuid(n.TriggeredBy),
		BillingModeExpected: n.BillingModeExpected, InvocationJti: uuid(n.InvocationJTI), QueuedAt: ts(n.QueuedAt),
	})
	return one(row, err, "inserting a run")
}

// Run implements app.Repository.
func (r *Repository) Run(ctx context.Context, id ids.ID) (app.RunRow, bool, error) {
	row, err := r.read(ctx).GetRun(ctx, uuid(id))
	return one(row, err, "reading a run")
}

// ActiveRunOfThread implements app.Repository.
func (r *Repository) ActiveRunOfThread(ctx context.Context, threadID ids.ID) (app.RunRow, bool, error) {
	row, err := r.read(ctx).ActiveRunOfThread(ctx, uuid(threadID))
	return one(row, err, "reading the active run of a thread")
}

// TransitionRun implements app.Repository: the compare-and-swap on state and version. No row
// with the id, w.From and w.Version gives changed false and no error; the row is then unchanged.
func (r *Repository) TransitionRun(ctx context.Context, w app.TransitionWrite) (app.RunRow, bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return app.RunRow{}, false, err
	}
	row, err := q.TransitionRun(ctx, sqlcgen.TransitionRunParams{
		ToState: string(w.To), Reason: w.Reason, At: ts(w.At), ID: uuid(w.ID),
		FromState: string(w.From), Version: w.Version,
	})
	return one(row, err, "changing the state of a run")
}

// LockRun implements app.Repository: the row of id with its row lock for the rest of the unit of
// work (SELECT ... FOR UPDATE). Errors: internal outside a unit of work.
func (r *Repository) LockRun(ctx context.Context, id ids.ID) (app.RunRow, bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return app.RunRow{}, false, err
	}
	row, err := q.LockRun(ctx, uuid(id))
	return one(row, err, "locking a run")
}

// RecordDelivery implements app.Repository: the columns of an allowed delivery and version + 1.
// Errors: not_found for an unknown run, internal outside a unit of work.
func (r *Repository) RecordDelivery(ctx context.Context, w app.DeliveryWrite) (app.RunRow, error) {
	q, err := r.write(ctx)
	if err != nil {
		return app.RunRow{}, err
	}
	row, err := q.RecordDelivery(ctx, sqlcgen.RecordDeliveryParams{
		Record: !w.JTI.IsZero(), Jti: optionalUUID(w.JTI), BillingModeExpected: w.BillingModeExpected, ID: uuid(w.ID),
	})
	out, found, err := one(row, err, "recording a delivery")
	if err == nil && !found {
		return app.RunRow{}, errs.New(errs.CodeNotFound, "runs: the run of a delivery is gone")
	}
	return out, err
}

// AppendStep implements app.Repository. A nil or empty Detail stores "{}", a zero StartedAt the
// time of the database (the column default of 03 run_steps).
func (r *Repository) AppendStep(ctx context.Context, s app.StepRow) (app.StepRow, error) {
	q, err := r.write(ctx)
	if err != nil {
		return app.StepRow{}, err
	}
	detail := s.Detail
	if len(detail) == 0 {
		detail = []byte("{}")
	}
	row, err := q.AppendStep(ctx, sqlcgen.AppendStepParams{
		ID: uuid(s.ID), RunID: uuid(s.RunID), ParentStepID: optionalUUID(s.ParentStepID),
		Kind: s.Kind, Name: s.Name, Title: s.Title, Status: s.Status, Detail: detail,
		StartedAt: optionalTS(s.StartedAt), FinishedAt: optionalTS(s.FinishedAt),
	})
	if err != nil {
		return app.StepRow{}, mapError(err, "appending a run step")
	}
	return toStep(row), nil
}

// StepCounts implements app.Repository in one statement; an empty runIDs reads nothing.
func (r *Repository) StepCounts(ctx context.Context, runIDs []ids.ID) (map[ids.ID]int, error) {
	out := make(map[ids.ID]int, len(runIDs))
	if len(runIDs) == 0 {
		return out, nil
	}
	keys := make([]pgtype.UUID, len(runIDs))
	for i, id := range runIDs {
		keys[i] = uuid(id)
	}
	rows, err := r.read(ctx).StepCounts(ctx, keys)
	if err != nil {
		return nil, mapError(err, "counting run steps")
	}
	for _, row := range rows {
		out[idOf(row.RunID)] = int(row.Steps)
	}
	return out, nil
}

// Steps implements app.Repository.
func (r *Repository) Steps(ctx context.Context, s app.StepsRead) ([]app.StepRow, error) {
	limit, err := rowLimit(s.Limit)
	if err != nil {
		return nil, err
	}
	rows, err := r.read(ctx).Steps(ctx, sqlcgen.StepsParams{
		RunID: uuid(s.RunID), AfterOrdinal: int32(min(max(s.AfterOrdinal, 0), maxInt32)), RowLimit: limit,
	})
	return steps(rows, err, "reading run steps")
}

// AllSteps implements app.Repository.
func (r *Repository) AllSteps(ctx context.Context, runID ids.ID) ([]app.StepRow, error) {
	rows, err := r.read(ctx).AllSteps(ctx, uuid(runID))
	return steps(rows, err, "reading all run steps")
}

// ThreadRuns implements app.Repository.
func (r *Repository) ThreadRuns(ctx context.Context, t app.ThreadRunsRead) ([]app.RunRow, error) {
	limit, err := rowLimit(t.Limit)
	if err != nil {
		return nil, err
	}
	at, id := after(t.After)
	rows, err := r.read(ctx).ThreadRuns(ctx, sqlcgen.ThreadRunsParams{
		ThreadID: uuid(t.ThreadID), AfterAt: at, AfterID: id, RowLimit: limit,
	})
	return runs(rows, err, "reading the runs of a thread")
}

// ActiveRuns implements app.Repository.
func (r *Repository) ActiveRuns(ctx context.Context, a app.ActiveRunsRead) ([]app.RunRow, error) {
	limit, err := rowLimit(a.Limit)
	if err != nil {
		return nil, err
	}
	at, id := after(a.After)
	rows, err := r.read(ctx).ActiveRuns(ctx, sqlcgen.ActiveRunsParams{
		AgentID: optionalUUID(a.AgentID), AfterAt: at, AfterID: id, RowLimit: limit,
	})
	return runs(rows, err, "reading the active runs")
}

// ActiveCounts implements app.Repository.
func (r *Repository) ActiveCounts(ctx context.Context) ([]app.ActiveCount, error) {
	rows, err := r.read(ctx).ActiveCounts(ctx)
	if err != nil {
		return nil, mapError(err, "counting the active runs")
	}
	out := make([]app.ActiveCount, len(rows))
	for i, row := range rows {
		out[i] = app.ActiveCount{AgentID: idOf(row.AgentID), State: domain.State(row.State), Count: row.Active}
	}
	return out, nil
}

// maxInt32 bounds the integer parameters of the queries.
const maxInt32 = 1<<31 - 1

// rowLimit converts the page or batch size of the use case, which comes from api.page_size_max
// and is never below one; anything else is a programming error of the caller.
func rowLimit(n int) (int32, error) {
	if n < 1 || n > maxInt32 {
		return 0, errs.New(errs.CodeInternalError, "runs: a read without a valid limit")
	}
	return int32(n), nil
}

// after converts the keyset position: nil gives two NULL parameters (the first page).
func after(k *app.RunKey) (pgtype.Timestamptz, pgtype.UUID) {
	if k == nil {
		return pgtype.Timestamptz{}, pgtype.UUID{}
	}
	return ts(k.QueuedAt), uuid(k.ID)
}

// one converts a single-row result: no row is found == false.
func one(row sqlcgen.Run, err error, what string) (app.RunRow, bool, error) {
	if errors.Is(err, pgx.ErrNoRows) {
		return app.RunRow{}, false, nil
	}
	if err != nil {
		return app.RunRow{}, false, mapError(err, what)
	}
	return toRun(row), true, nil
}

// runs converts a list result.
func runs(rows []sqlcgen.Run, err error, what string) ([]app.RunRow, error) {
	if err != nil {
		return nil, mapError(err, what)
	}
	out := make([]app.RunRow, len(rows))
	for i, row := range rows {
		out[i] = toRun(row)
	}
	return out, nil
}

// steps converts a list result.
func steps(rows []sqlcgen.RunStep, err error, what string) ([]app.StepRow, error) {
	if err != nil {
		return nil, mapError(err, what)
	}
	out := make([]app.StepRow, len(rows))
	for i, row := range rows {
		out[i] = toStep(row)
	}
	return out, nil
}

// toRun converts a row of runs.
func toRun(row sqlcgen.Run) app.RunRow {
	return app.RunRow{
		ID: idOf(row.ID), ThreadID: idOf(row.ThreadID), AgentID: idOf(row.AgentID), AgentSessionID: idOf(row.AgentSessionID),
		TriggerKind: row.TriggerKind, TriggeringMessageID: idOf(row.TriggeringMessageID),
		TriggeringCommandID: idOf(row.TriggeringCommandID), TriggeringInteractionID: idOf(row.TriggeringInteractionID),
		TriggeredBy: idOf(row.TriggeredBy), State: domain.State(row.State), StateReason: row.StateReason.String,
		Version: row.Version, BillingModeExpected: row.BillingModeExpected, BillingModeUsed: row.BillingModeUsed.String,
		InvocationJTI: idOf(row.InvocationJti), RenewedFromJTI: idOf(row.RenewedFromJti), Model: row.Model.String, StatusLine: row.StatusLine.String,
		LastTextExcerpt: row.LastTextExcerpt.String, Summary: row.Summary.String, Plan: row.Plan, Metrics: row.Metrics,
		CostMicros: row.CostMicros, Currency: row.Currency, CostIsEstimate: row.CostIsEstimate,
		QueuedAt: timeOf(row.QueuedAt), StartedAt: timeOf(row.StartedAt), FinishedAt: timeOf(row.FinishedAt),
		DurationMS: row.DurationMs.Int64, HasDuration: row.DurationMs.Valid,
	}
}

// toStep converts a row of run_steps.
func toStep(row sqlcgen.RunStep) app.StepRow {
	return app.StepRow{
		ID: idOf(row.ID), RunID: idOf(row.RunID), Ordinal: int(row.Ordinal), ParentStepID: idOf(row.ParentStepID),
		Kind: row.Kind, Name: row.Name, Title: row.Title, Status: row.Status, Detail: row.Detail,
		StartedAt: timeOf(row.StartedAt), FinishedAt: timeOf(row.FinishedAt),
	}
}

// mapError turns a database error of the operation what into a kernel/errs error: a kernel/errs
// error passes, anything else is internal (never SQL text or a value).
func mapError(err error, what string) error {
	if err == nil {
		return nil
	}
	if _, ok := errs.As(err); ok {
		return err
	}
	return errs.Wrap(errs.CodeInternalError, err, "runs: "+what+" failed")
}

// uuid converts an id for a parameter.
func uuid(id ids.ID) pgtype.UUID { return pgtype.UUID{Bytes: id, Valid: true} }

// optionalUUID converts an optional id: the zero ID becomes NULL.
func optionalUUID(id ids.ID) pgtype.UUID { return pgtype.UUID{Bytes: id, Valid: !id.IsZero()} }

// idOf converts a column; NULL becomes the zero ID.
func idOf(u pgtype.UUID) ids.ID {
	if !u.Valid {
		return ids.ID{}
	}
	return ids.ID(u.Bytes)
}

// ts converts a time for a parameter in UTC, cut to the microseconds the column keeps, so a
// value read back equals the value written.
func ts(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t.UTC().Truncate(time.Microsecond), Valid: true}
}

// optionalTS converts an optional time: the zero time becomes NULL.
func optionalTS(t time.Time) pgtype.Timestamptz {
	if t.IsZero() {
		return pgtype.Timestamptz{}
	}
	return ts(t)
}

// timeOf converts a column in UTC; NULL becomes the zero time.
func timeOf(t pgtype.Timestamptz) time.Time {
	if !t.Valid {
		return time.Time{}
	}
	return t.Time.UTC()
}
