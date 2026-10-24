// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// ConsumeFresh implements identity.StepUp (08 section 6.3 point 4, Design note 4 of M2-40). Two
// requests with one confirmation both pass authorize; only the lock of the session row orders
// them, so the check runs again on the locked row, not on the actor of the request, with the same
// rule as step 8 of authorize (authz.FreshStepUpCheck) and the clock of the call. The lock is the
// first domain row lock of the action's transaction (03 section 6.3), so the second opening waits
// and sees the consumption.
func (s *StepUps) ConsumeFresh(ctx context.Context, a actor.Actor) error {
	if _, ok := txn.UnitFrom(ctx); !ok {
		return errs.New(errs.CodeInternalError, "identity: ConsumeFresh runs only inside a unit of work")
	}
	if s.d.Fresh == nil {
		return errs.New(errs.CodeInternalError, "identity: ConsumeFresh needs the fresh step-up check of authz")
	}
	if a.Kind != actor.KindUser || a.SessionID.IsZero() {
		return errs.New(errs.CodeStepUpRequired, "")
	}
	sess, found, err := s.d.Challenges.LockSessionForStepUpConsumption(ctx, a.SessionID, a.ID)
	if err != nil {
		return err
	}
	now := s.now()
	if !found || !sess.Active(now) {
		return errs.New(errs.CodeSessionRevoked, "")
	}
	facts := authz.FreshStepUpFacts{
		StepUpAt: sess.StepUpAt, StepUpConsumedAt: sess.StepUpConsumedAt, StepUpMethod: sess.StepUpMethod, ClientComponent: string(sess.Component),
		StepUpUserVerified: sess.StepUpUserVerified,
	}
	if !s.d.Fresh.Fresh(facts, now) {
		if s.d.Fresh.PasskeyRequired(facts) {
			// The console rule of 08 section 6.3 point 3 (M6-18): the same reason authorize gives.
			return errs.New(errs.CodeStepUpRequired, "").WithReason(authorize.ReasonPasskeyRequired)
		}
		return errs.New(errs.CodeStepUpRequired, "")
	}
	return s.d.Challenges.ConsumeStepUp(ctx, a.SessionID, now)
}
