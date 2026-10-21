// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// behaviorReadStub answers ChannelBehavior with fixed values; every other method of
// BehaviorStore is unused here and panics through the nil embedded interface.
type behaviorReadStub struct {
	BehaviorStore
	row   BehaviorRow
	found bool
	err   error
}

func (s behaviorReadStub) ChannelBehavior(context.Context, ids.ID) (BehaviorRow, bool, error) {
	return s.row, s.found, s.err
}

// TestReadBehavior_StoreOutcomes: without a bound store the read is internal_error, a store
// error passes through unchanged, an unknown channel is not_found and a found row comes back.
func TestReadBehavior_StoreOutcomes(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	id := ids.New()

	if _, err := (&Service{}).readBehavior(ctx, id); !hasCode(err, errs.CodeInternalError) {
		t.Errorf("no store: %v, want internal_error", err)
	}
	boom := errors.New("boom")
	if _, err := (&Service{d: Deps{Behaviors: behaviorReadStub{err: boom}}}).readBehavior(ctx, id); !errors.Is(err, boom) {
		t.Errorf("store error: %v, want it unchanged", err)
	}
	if _, err := (&Service{d: Deps{Behaviors: behaviorReadStub{}}}).readBehavior(ctx, id); !hasCode(err, errs.CodeNotFound) {
		t.Errorf("unknown channel: %v, want not_found", err)
	}
	want := BehaviorRow{SettingsVersion: 4}
	got, err := (&Service{d: Deps{Behaviors: behaviorReadStub{row: want, found: true}}}).readBehavior(ctx, id)
	if err != nil || got.SettingsVersion != want.SettingsVersion {
		t.Errorf("found: %+v, %v, want settings_version %d", got, err, want.SettingsVersion)
	}
}

// hasCode reports whether the first *errs.Error in the chain of err has code c.
func hasCode(err error, c errs.Code) bool {
	e, ok := errs.As(err)
	return ok && e.Code == c
}
