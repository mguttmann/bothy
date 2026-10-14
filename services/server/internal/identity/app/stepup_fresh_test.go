// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/identity/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// freshChallenges is the challenge store of the ConsumeFresh tests: one locked session row and
// the recorded consumption; the other methods are never called.
type freshChallenges struct {
	app.ChallengeStore
	row      app.SessionRecord
	found    bool
	lockErr  error
	consumed time.Time
}

func (c *freshChallenges) LockSessionForStepUpConsumption(_ context.Context, id, userID ids.ID) (app.SessionRecord, bool, error) {
	if c.lockErr != nil {
		return app.SessionRecord{}, false, c.lockErr
	}
	return c.row, c.found && c.row.ID == id && c.row.UserID == userID, nil
}

func (c *freshChallenges) ConsumeStepUp(_ context.Context, _ ids.ID, at time.Time) error {
	c.consumed = at
	return nil
}

// freshRule is the fresh step-up check: fresh exactly when the step-up is not consumed.
type freshRule struct{}

func (freshRule) Fresh(f authz.FreshStepUpFacts, _ time.Time) bool {
	return !f.StepUpAt.IsZero() && f.StepUpConsumedAt.Before(f.StepUpAt)
}

// The placeholder collaborators are shared with stepup_new_test.go.

// TestConsumeFresh_UseCase (M2-40, Design note 4): ConsumeFresh needs a unit of work and the
// fresh rule, refuses agents and principals without a session with step_up_required, an unknown
// or revoked session with session_revoked, a consumed step-up with step_up_required, passes the
// lock error through, and consumes a fresh step-up at the time of the call.
func TestConsumeFresh_UseCase(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC)
	alex, sess := ids.New(), ids.New()
	cfg := app.Config{StepUpChallengeTTL: time.Minute, StepUpMaxAge: time.Minute, LoginFailWindow: time.Minute, StepUpFailLimit: 3}
	build := func(ch *freshChallenges, fresh authz.FreshStepUpCheck) *app.StepUps {
		s, err := app.NewStepUps(cfg, []byte("pepper"), app.StepUpDeps{Challenges: ch, Sessions: stubSessions{}, Devices: stubDevices{},
			Auth: stubAuth{}, UoW: stubUoW{}, Authorizer: stubAuthorizer{}, Audit: stubAudit{}, Clock: clock.NewFake(now), Fresh: fresh})
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	unit, _ := txn.NewUnit(txn.NewRegistry(), txn.Options{})
	inUnit := txn.WithUnit(context.Background(), unit)
	person := actor.Actor{Kind: actor.KindUser, ID: alex, SessionID: sess}
	live := app.SessionRecord{ID: sess, UserID: alex, ExpiresAt: now.Add(time.Hour), StepUpAt: now.Add(-time.Second), StepUpMethod: "device_key"}

	codeOf := func(err error) errs.Code {
		if e, ok := errs.As(err); ok {
			return e.Code
		}
		return ""
	}
	ch := &freshChallenges{row: live, found: true}
	if err := build(ch, freshRule{}).ConsumeFresh(context.Background(), person); codeOf(err) != errs.CodeInternalError {
		t.Fatalf("outside a unit of work: %v", err)
	}
	if err := build(ch, nil).ConsumeFresh(inUnit, person); codeOf(err) != errs.CodeInternalError {
		t.Fatalf("without the fresh rule: %v", err)
	}
	agent := actor.Actor{Kind: actor.KindAgent, ID: ids.New()}
	if err := build(ch, freshRule{}).ConsumeFresh(inUnit, agent); codeOf(err) != errs.CodeStepUpRequired {
		t.Fatalf("an agent: %v", err)
	}
	if err := build(&freshChallenges{}, freshRule{}).ConsumeFresh(inUnit, person); codeOf(err) != errs.CodeSessionRevoked {
		t.Fatalf("an unknown session: %v", err)
	}
	revoked := live
	revoked.RevokedAt = now.Add(-time.Minute)
	if err := build(&freshChallenges{row: revoked, found: true}, freshRule{}).ConsumeFresh(inUnit, person); codeOf(err) != errs.CodeSessionRevoked {
		t.Fatalf("a revoked session: %v", err)
	}
	boom := errors.New("lock failed")
	if err := build(&freshChallenges{lockErr: boom}, freshRule{}).ConsumeFresh(inUnit, person); !errors.Is(err, boom) {
		t.Fatalf("the lock error: %v", err)
	}
	used := live
	used.StepUpConsumedAt = live.StepUpAt
	if err := build(&freshChallenges{row: used, found: true}, freshRule{}).ConsumeFresh(inUnit, person); codeOf(err) != errs.CodeStepUpRequired {
		t.Fatalf("a consumed step-up: %v", err)
	}
	if err := build(ch, freshRule{}).ConsumeFresh(inUnit, person); err != nil || !ch.consumed.Equal(now) {
		t.Fatalf("a fresh step-up: %v, consumed at %v", err, ch.consumed)
	}
}
