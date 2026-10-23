// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// TestFollowups_Errors (M2-11): the writing follow-up methods refuse to run outside a unit of
// work, every method returns the error of a failed statement instead of a result, and raising
// the version of an unknown run is not_found.
func TestFollowups_Errors(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	run, msg := ids.New(), ids.New()
	list := []ids.ID{msg}
	writes := map[string]func(ctx context.Context) error{
		"InsertFollowup": func(ctx context.Context) error { return e.repo.InsertFollowup(ctx, run, msg) },
		"LockOpenFollowups": func(ctx context.Context) error {
			_, err := e.repo.LockOpenFollowups(ctx, run)
			return err
		},
		"LockFollowup": func(ctx context.Context) error {
			_, _, err := e.repo.LockFollowup(ctx, run, msg)
			return err
		},
		"SetFollowupStates": func(ctx context.Context) error {
			return e.repo.SetFollowupStates(ctx, run, list, domain.FollowupDelivered)
		},
		"MoveFollowups": func(ctx context.Context) error { return e.repo.MoveFollowups(ctx, run, ids.New(), list) },
		"RaiseRunVersion": func(ctx context.Context) error {
			_, err := e.repo.RaiseRunVersion(ctx, run)
			return err
		},
		"LockActiveRunOfThread": func(ctx context.Context) error {
			_, _, err := e.repo.LockActiveRunOfThread(ctx, ids.New())
			return err
		},
	}
	reads := map[string]func(ctx context.Context) error{
		"RunFollowups": func(ctx context.Context) error {
			_, err := e.repo.RunFollowups(ctx, run)
			return err
		},
		"FollowupStatesOf": func(ctx context.Context) error {
			_, err := e.repo.FollowupStatesOf(ctx, run, list)
			return err
		},
		"FollowupOfMessage": func(ctx context.Context) error {
			_, _, err := e.repo.FollowupOfMessage(ctx, msg)
			return err
		},
	}
	for name, f := range writes {
		if err := f(context.Background()); errs.KindOf(err) != errs.KindInternal {
			t.Errorf("%s outside a unit of work: %v", name, err)
		}
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	for name, f := range reads {
		if err := f(cancelled); err == nil {
			t.Errorf("%s with a failed statement: no error", name)
		}
	}
	for name, f := range writes {
		var got error
		// The unit's own statements fail too once its context is cancelled; only the method's
		// error counts here.
		_ = e.uow.Do(context.Background(), func(ctx context.Context) error {
			ctx, cancel := context.WithCancel(ctx)
			cancel()
			got = f(ctx)
			return got
		})
		if got == nil {
			t.Errorf("%s with a failed statement: no error", name)
		}
	}
	err := e.uow.Do(context.Background(), func(ctx context.Context) error {
		_, err := e.repo.RaiseRunVersion(ctx, run)
		return err
	})
	if errs.KindOf(err) != errs.KindNotFound {
		t.Fatalf("RaiseRunVersion of an unknown run: %v", err)
	}
}
