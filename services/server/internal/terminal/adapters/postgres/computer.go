// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"errors"
	"math"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/terminal/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/terminal/app"
	"github.com/OWNER/bothy/services/server/internal/terminal/domain"
)

// Compile-time check: Repository is the computer store of the application layer (CU-04).
var _ app.ComputerStore = (*Repository)(nil)

// The unique indexes of computer_sessions the adapter maps to conflicts (27 8.5, R-CU-06).
const (
	constraintComputerHost = "computer_sessions_host_open_key"
	constraintComputerRun  = "computer_sessions_run_open_key"
)

// InsertComputerSession implements app.ComputerStore: a second open session of the host is 409
// conflict computer_busy, of the run computer_run_busy.
func (r *Repository) InsertComputerSession(ctx context.Context, s domain.ComputerSession) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	err = q.InsertComputerSession(ctx, sqlcgen.InsertComputerSessionParams{
		ID: uuid(s.ID), HostID: uuid(s.HostID), UserID: uuid(s.UserID), AgentID: uuid(s.AgentID), RunID: uuid(s.RunID),
		TokenJti: uuid(s.TokenJTI), Generation: int32Of(s.Generation), KeyGeneration: int32Of(s.KeyGeneration), OpenedAt: optionalTime(s.OpenedAt),
	})
	switch {
	case isUnique(err, constraintComputerHost):
		return errs.New(errs.CodeConflict, "").WithReason("computer_busy")
	case isUnique(err, constraintComputerRun):
		return errs.New(errs.CodeConflict, "").WithReason("computer_run_busy")
	}
	return mapError(err, "inserting a computer session")
}

// ComputerSession implements app.ComputerStore.
func (r *Repository) ComputerSession(ctx context.Context, id ids.ID) (domain.ComputerSession, bool, error) {
	row, err := r.read(ctx).GetComputerSession(ctx, uuid(id))
	return oneComputer(row, err, "reading a computer session")
}

// LockComputerSession implements app.ComputerStore.
func (r *Repository) LockComputerSession(ctx context.Context, id ids.ID) (domain.ComputerSession, bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return domain.ComputerSession{}, false, err
	}
	row, err := q.LockComputerSession(ctx, uuid(id))
	return oneComputer(row, err, "locking a computer session")
}

// LockOpenComputerSessionsOfHost implements app.ComputerStore.
func (r *Repository) LockOpenComputerSessionsOfHost(ctx context.Context, hostID ids.ID) ([]domain.ComputerSession, error) {
	q, err := r.write(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := q.LockOpenComputerSessionsOfHost(ctx, uuid(hostID))
	return computerList(rows, err, "locking the computer sessions of a host")
}

// LockOpenComputerSessionsOfUser implements app.ComputerStore.
func (r *Repository) LockOpenComputerSessionsOfUser(ctx context.Context, userID ids.ID) ([]domain.ComputerSession, error) {
	q, err := r.write(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := q.LockOpenComputerSessionsOfUser(ctx, uuid(userID))
	return computerList(rows, err, "locking the computer sessions of a person")
}

// LockOpenComputerSessionsOfRun implements app.ComputerStore.
func (r *Repository) LockOpenComputerSessionsOfRun(ctx context.Context, runID ids.ID) ([]domain.ComputerSession, error) {
	q, err := r.write(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := q.LockOpenComputerSessionsOfRun(ctx, uuid(runID))
	return computerList(rows, err, "locking the computer sessions of a run")
}

// OpenComputerSessionOfRun implements app.ComputerStore.
func (r *Repository) OpenComputerSessionOfRun(ctx context.Context, runID ids.ID) (bool, error) {
	ok, err := r.read(ctx).OpenComputerSessionOfRun(ctx, uuid(runID))
	return ok, mapError(err, "reading the computer session of a run")
}

// RunHadComputerSession implements app.ComputerStore.
func (r *Repository) RunHadComputerSession(ctx context.Context, runID ids.ID) (bool, error) {
	ok, err := r.read(ctx).RunHadComputerSession(ctx, uuid(runID))
	return ok, mapError(err, "reading the computer sessions of a run")
}

// ComputerSessionsByIDs implements app.ComputerStore.
func (r *Repository) ComputerSessionsByIDs(ctx context.Context, list []ids.ID) ([]domain.ComputerSession, error) {
	if len(list) == 0 {
		return nil, nil
	}
	rows, err := r.read(ctx).ComputerSessionsByIDs(ctx, uuids(list))
	return computerList(rows, err, "reading computer sessions")
}

// CountOpenComputerSessions implements app.ComputerStore.
func (r *Repository) CountOpenComputerSessions(ctx context.Context, hostID, runID ids.ID) (int, int, error) {
	row, err := r.read(ctx).CountOpenComputerSessions(ctx, sqlcgen.CountOpenComputerSessionsParams{HostID: uuid(hostID), RunID: uuid(runID)})
	if err != nil {
		return 0, 0, mapError(err, "counting computer sessions")
	}
	return int(row.OfHost), int(row.OfRun), nil
}

// OpenComputerHosts implements app.ComputerStore.
func (r *Repository) OpenComputerHosts(ctx context.Context) ([]ids.ID, error) {
	rows, err := r.read(ctx).OpenComputerHosts(ctx)
	return idList(rows, err, "reading the busy desktops")
}

// ListComputerSessions implements app.ComputerStore.
func (r *Repository) ListComputerSessions(ctx context.Context, userID ids.ID, state string, limit int) ([]app.ComputerRow, error) {
	rows, err := r.read(ctx).ListComputerSessionsOfUser(ctx, sqlcgen.ListComputerSessionsOfUserParams{
		UserID: uuid(userID), State: state, MaxRows: maxRows(limit)})
	if err != nil {
		return nil, mapError(err, "listing computer sessions")
	}
	out := make([]app.ComputerRow, 0, len(rows))
	for _, x := range rows {
		out = append(out, app.ComputerRow{ComputerSession: toComputer(sqlcgen.ComputerSession{
			ID: x.ID, HostID: x.HostID, UserID: x.UserID, AgentID: x.AgentID, RunID: x.RunID, State: x.State, PauseReason: x.PauseReason,
			TokenJti: x.TokenJti, Generation: x.Generation, KeyGeneration: x.KeyGeneration, AttachedAt: x.AttachedAt, OpenedAt: x.OpenedAt,
			StartedAt: x.StartedAt, PausedAt: x.PausedAt, PausedMs: x.PausedMs, ClosedAt: x.ClosedAt, StoppedBy: x.StoppedBy,
			CloseReason: x.CloseReason, CloseDetail: x.CloseDetail, ActionCount: x.ActionCount, FrontApp: x.FrontApp, Version: x.Version,
		}), Desktop: x.Desktop})
	}
	return out, nil
}

// HostName implements app.ComputerStore.
func (r *Repository) HostName(ctx context.Context, id ids.ID) (string, error) {
	name, err := r.read(ctx).HostName(ctx, uuid(id))
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return name, mapError(err, "reading the name of a host")
}

// AttachComputerSession implements app.ComputerStore.
func (r *Repository) AttachComputerSession(ctx context.Context, id ids.ID, at time.Time) (bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return false, err
	}
	n, err := q.AttachComputerSession(ctx, sqlcgen.AttachComputerSessionParams{ID: uuid(id), AttachedAt: optionalTime(at)})
	return n == 1, mapError(err, "attaching a computer session")
}

// StartComputerSession implements app.ComputerStore.
func (r *Repository) StartComputerSession(ctx context.Context, id ids.ID, state string, at time.Time) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	p := sqlcgen.StartComputerSessionParams{ID: uuid(id), State: state, StartedAt: optionalTime(at)}
	if state == domain.ComputerPaused {
		p.PauseReason, p.PausedAt = optionalText(domain.PauseLocked), optionalTime(at)
	}
	n, err := q.StartComputerSession(ctx, p)
	return oneRow(n, err, "starting a computer session")
}

// UpdateComputerState implements app.ComputerStore.
func (r *Repository) UpdateComputerState(ctx context.Context, id ids.ID, w app.ComputerStateWrite) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	n, err := q.UpdateComputerState(ctx, sqlcgen.UpdateComputerStateParams{
		ID: uuid(id), State: w.State, PauseReason: optionalText(w.PauseReason), PausedAt: optionalTime(w.PausedAt), AddPausedMs: w.AddPausedMS,
		ActionCount: int32Of(w.ActionCount), FrontApp: optionalText(w.FrontApp)})
	return oneRow(n, err, "writing the state of a computer session")
}

// EndComputerSession implements app.ComputerStore: the one closing statement.
func (r *Repository) EndComputerSession(ctx context.Context, id ids.ID, end domain.ComputerEnd, w app.ComputerEndWrite, at time.Time) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	n, err := q.EndComputerSession(ctx, sqlcgen.EndComputerSessionParams{
		ID: uuid(id), ClosedAt: optionalTime(at), CloseReason: optionalText(string(end.Reason)), CloseDetail: optionalText(string(end.Detail)),
		StoppedBy: optionalUUID(end.StoppedBy), AddPausedMs: w.AddPausedMS, ReportedPausedMs: w.ReportedPausedMS, ActionCount: int32Of(w.ActionCount)})
	return oneRow(n, err, "ending a computer session")
}

// StaleOpeningComputerSessions implements app.ComputerStore.
func (r *Repository) StaleOpeningComputerSessions(ctx context.Context, before time.Time) ([]ids.ID, error) {
	rows, err := r.read(ctx).StaleOpeningComputerSessions(ctx, optionalTime(before))
	return idList(rows, err, "reading the stale computer openings")
}

// SetComputerDisabledGeneration implements app.ComputerStore.
func (r *Repository) SetComputerDisabledGeneration(ctx context.Context, hostID ids.ID, generation, version int64) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	n, err := q.SetComputerDisabledGeneration(ctx, sqlcgen.SetComputerDisabledGenerationParams{
		ID: uuid(hostID), Generation: pgtype.Int4{Int32: int32Of(generation), Valid: true}, Version: version})
	return oneRow(n, err, "writing the disabled generation of a host")
}

// oneComputer converts one row of a single read.
func oneComputer(row sqlcgen.ComputerSession, err error, what string) (domain.ComputerSession, bool, error) {
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ComputerSession{}, false, nil
	}
	if err != nil {
		return domain.ComputerSession{}, false, mapError(err, what)
	}
	return toComputer(row), true, nil
}

// computerList converts the rows of a list read.
func computerList(rows []sqlcgen.ComputerSession, err error, what string) ([]domain.ComputerSession, error) {
	if err != nil {
		return nil, mapError(err, what)
	}
	out := make([]domain.ComputerSession, 0, len(rows))
	for _, x := range rows {
		out = append(out, toComputer(x))
	}
	return out, nil
}

// toComputer converts a row of computer_sessions.
func toComputer(x sqlcgen.ComputerSession) domain.ComputerSession {
	return domain.ComputerSession{ID: idOf(x.ID), HostID: idOf(x.HostID), UserID: idOf(x.UserID), AgentID: idOf(x.AgentID), RunID: idOf(x.RunID),
		State: x.State, PauseReason: x.PauseReason.String, TokenJTI: idOf(x.TokenJti), Generation: int64(x.Generation),
		KeyGeneration: int64(x.KeyGeneration), AttachedAt: timeOf(x.AttachedAt), OpenedAt: timeOf(x.OpenedAt), StartedAt: timeOf(x.StartedAt),
		PausedAt: timeOf(x.PausedAt), PausedMS: x.PausedMs, ClosedAt: timeOf(x.ClosedAt), StoppedBy: idOf(x.StoppedBy),
		CloseReason: domain.ComputerCloseReason(x.CloseReason.String), CloseDetail: domain.ComputerCloseDetail(x.CloseDetail.String),
		ActionCount: int64(x.ActionCount), FrontApp: x.FrontApp.String, Version: x.Version}
}

// int32Of converts a count or generation for an integer column, capped at its range (the host
// service reports counts the server only stores).
func int32Of(n int64) int32 {
	switch {
	case n > math.MaxInt32:
		return math.MaxInt32
	case n < math.MinInt32:
		return math.MinInt32
	}
	return int32(n)
}
