// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/OWNER/bothy/services/server/internal/identity/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/identity/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Compile-time check: Repository is the challenge store of the identity application layer.
var _ app.ChallengeStore = (*Repository)(nil)

// InsertChallenge implements app.ChallengeStore.
func (r *Repository) InsertChallenge(ctx context.Context, c app.ChallengeRecord) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	err = q.InsertChallenge(ctx, sqlcgen.InsertChallengeParams{
		ID: uuid(c.ID), Purpose: string(c.Purpose), SessionID: optionalUUID(c.SessionID),
		EnrollmentCodeID: optionalUUID(c.EnrollmentCodeID), ChallengeHash: c.Hash, WebauthnSession: c.WebAuthnSession,
		CreatedAt: ts(c.CreatedAt), ExpiresAt: ts(c.ExpiresAt),
	})
	return mapError(err, "inserting a challenge")
}

// LockChallenge implements app.ChallengeStore.
func (r *Repository) LockChallenge(ctx context.Context, id ids.ID) (app.ChallengeRecord, bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return app.ChallengeRecord{}, false, err
	}
	row, err := q.LockChallenge(ctx, uuid(id))
	if errors.Is(err, pgx.ErrNoRows) {
		return app.ChallengeRecord{}, false, nil
	}
	if err != nil {
		return app.ChallengeRecord{}, false, mapError(err, "locking a challenge")
	}
	return app.ChallengeRecord{
		ID: idOf(row.ID), SessionID: idOf(row.SessionID), EnrollmentCodeID: idOf(row.EnrollmentCodeID),
		Purpose: app.ChallengePurpose(row.Purpose), Hash: row.ChallengeHash, WebAuthnSession: row.WebauthnSession,
		Status: app.ChallengeStatus(row.Status), CreatedAt: timeOf(row.CreatedAt), ExpiresAt: timeOf(row.ExpiresAt),
		FinishedAt: timeOf(row.FinishedAt),
	}, true, nil
}

// FinishChallenge implements app.ChallengeStore.
func (r *Repository) FinishChallenge(ctx context.Context, id ids.ID, status app.ChallengeStatus, at time.Time) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	err = q.FinishChallenge(ctx, sqlcgen.FinishChallengeParams{ID: uuid(id), Status: string(status), FinishedAt: ts(at)})
	return mapError(err, "finishing a challenge")
}

// FailedStepUpTimes implements app.ChallengeStore.
func (r *Repository) FailedStepUpTimes(ctx context.Context, sessionID ids.ID, since time.Time) ([]time.Time, error) {
	rows, err := r.read(ctx).FailedStepUpTimes(ctx, sqlcgen.FailedStepUpTimesParams{SessionID: uuid(sessionID), Since: ts(since)})
	if err != nil {
		return nil, mapError(err, "counting failed step-ups")
	}
	out := make([]time.Time, 0, len(rows))
	for _, t := range rows {
		out = append(out, timeOf(t))
	}
	return out, nil
}

// LockSessionForStepUpConsumption implements app.ChallengeStore.
func (r *Repository) LockSessionForStepUpConsumption(ctx context.Context, id, userID ids.ID) (app.SessionRecord, bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return app.SessionRecord{}, false, err
	}
	row, err := q.LockSessionForStepUpConsumption(ctx, sqlcgen.LockSessionForStepUpConsumptionParams{ID: uuid(id), UserID: uuid(userID)})
	if errors.Is(err, pgx.ErrNoRows) {
		return app.SessionRecord{}, false, nil
	}
	if err != nil {
		return app.SessionRecord{}, false, mapError(err, "locking a session for the step-up")
	}
	s := toSession(row.AuthSession)
	s.StepUpUserVerified = row.StepUpUserVerified
	return s, true, nil
}

// ConsumeStepUp implements app.ChallengeStore.
func (r *Repository) ConsumeStepUp(ctx context.Context, id ids.ID, at time.Time) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	n, err := q.ConsumeStepUp(ctx, sqlcgen.ConsumeStepUpParams{ID: uuid(id), ConsumedAt: ts(at)})
	if err == nil && n != 1 {
		return errs.New(errs.CodeInternalError, "identity: consuming the step-up changed no row")
	}
	return mapError(err, "consuming the step-up of a session")
}

// SetSessionStepUp implements app.ChallengeStore.
func (r *Repository) SetSessionStepUp(ctx context.Context, id ids.ID, at time.Time, method string) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	err = q.SetSessionStepUp(ctx, sqlcgen.SetSessionStepUpParams{
		ID: uuid(id), StepUpAt: ts(at), StepUpMethod: pgtype.Text{String: method, Valid: true},
	})
	return mapError(err, "setting the step-up of a session")
}

// DeleteExpiredChallenges implements app.ChallengeStore.
func (r *Repository) DeleteExpiredChallenges(ctx context.Context, now, keepFailedSince time.Time) (int64, error) {
	q, err := r.write(ctx)
	if err != nil {
		return 0, err
	}
	n, err := q.DeleteExpiredChallenges(ctx, sqlcgen.DeleteExpiredChallengesParams{Now: ts(now), KeepFailedSince: ts(keepFailedSince)})
	if err != nil {
		return 0, mapError(err, "deleting expired challenges")
	}
	return n, nil
}
