// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// TestStepUpConsumption_StoreRoundTrip (M2-40): the session row of a person is locked for the
// consumption of a fresh step-up with its step-up columns; a foreign or unknown session is not
// found; the consumption writes step_up_consumed_at once; both need a unit of work, and consuming
// on an unknown session is an internal error.
func TestStepUpConsumption_StoreRoundTrip(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ctx := context.Background()
	alex := e.mustAddUser(t, "alex", "alex@example.com").Principal.ID
	sam := e.mustAddUser(t, "sam", "sam@example.com").Principal.ID
	sess := e.session(t, alex, e.device(t, alex, "active", time.Time{}, true), ids.ID{}, false)
	at := time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC)
	e.exec(t, `UPDATE auth_sessions SET step_up_at = $2, step_up_method = 'device_key' WHERE id = $1`, sess.String(), at)

	if _, _, err := e.repo.LockSessionForStepUpConsumption(ctx, sess, alex); errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("lock outside a unit of work: %v", err)
	}
	if err := e.repo.ConsumeStepUp(ctx, sess, at); errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("consume outside a unit of work: %v", err)
	}
	if err := e.write(t, func(ctx context.Context) error {
		if _, found, err := e.repo.LockSessionForStepUpConsumption(ctx, sess, sam); err != nil || found {
			t.Errorf("foreign session: %v %v", found, err)
		}
		if _, found, err := e.repo.LockSessionForStepUpConsumption(ctx, ids.New(), alex); err != nil || found {
			t.Errorf("unknown session: %v %v", found, err)
		}
		got, found, err := e.repo.LockSessionForStepUpConsumption(ctx, sess, alex)
		if err != nil || !found || got.ID != sess || !got.StepUpAt.Equal(at) || got.StepUpMethod != "device_key" || !got.StepUpConsumedAt.IsZero() {
			t.Errorf("locked %+v %v %v", got, found, err)
		}
		return e.repo.ConsumeStepUp(ctx, sess, at.Add(time.Second))
	}); err != nil {
		t.Fatal(err)
	}
	if err := e.write(t, func(ctx context.Context) error { return e.repo.ConsumeStepUp(ctx, ids.New(), at) }); errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("consume on an unknown session: %v", err)
	}
	if err := e.write(t, func(ctx context.Context) error {
		got, _, err := e.repo.LockSessionForStepUpConsumption(ctx, sess, alex)
		if err != nil || !got.StepUpConsumedAt.Equal(at.Add(time.Second)) {
			t.Errorf("after the consumption %+v %v", got, err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
