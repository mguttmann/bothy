// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"errors"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// TestEffectiveFollowupMode: the owner's permission (absent, true, false) times the adapter's
// report (steer, queue, absent, unknown): steer or queue only when permitted and reported exactly,
// none otherwise (09 2.2, R-AG-01; fail closed for an unknown report).
func TestEffectiveFollowupMode(t *testing.T) {
	t.Parallel()
	yes, no := true, false
	for _, permitted := range []*bool{nil, &yes, &no} {
		for _, reported := range []string{"steer", "queue", "", "interrupt"} {
			want := domain.FollowupNone
			if permitted == nil || *permitted {
				switch reported {
				case "steer":
					want = domain.FollowupSteer
				case "queue":
					want = domain.FollowupQueue
				}
			}
			if got := domain.EffectiveFollowupMode(permitted, reported); got != want {
				t.Errorf("EffectiveFollowupMode(%v, %q) = %q, want %q", permitted, reported, got, want)
			}
		}
	}
}

// TestFollowupEventNeeded: steer sends in every active state, queue and none only in
// waiting_input (the answer to the open question), nobody in an end state.
func TestFollowupEventNeeded(t *testing.T) {
	t.Parallel()
	for _, mode := range []domain.FollowupMode{domain.FollowupSteer, domain.FollowupQueue, domain.FollowupNone} {
		for _, st := range domain.States() {
			want := st == domain.StateWaitingInput || (mode == domain.FollowupSteer && st.Active())
			if got := domain.FollowupEventNeeded(mode, st); got != want {
				t.Errorf("FollowupEventNeeded(%s, %s) = %v, want %v", mode, st, got, want)
			}
		}
	}
}

// TestAdvanceFollowup: every state as from with delivered and absorbed as to: forward moves
// change, the same state is no change, everything else is invalid_state_transition.
func TestAdvanceFollowup(t *testing.T) {
	t.Parallel()
	changes := map[[2]domain.FollowupState]bool{
		{domain.FollowupQueued, domain.FollowupDelivered}:   true,
		{domain.FollowupQueued, domain.FollowupAbsorbed}:    true,
		{domain.FollowupDelivered, domain.FollowupAbsorbed}: true,
	}
	same := map[[2]domain.FollowupState]bool{
		{domain.FollowupDelivered, domain.FollowupDelivered}: true,
		{domain.FollowupAbsorbed, domain.FollowupAbsorbed}:   true,
	}
	for _, from := range domain.FollowupStates() {
		for _, to := range []domain.FollowupState{domain.FollowupDelivered, domain.FollowupAbsorbed} {
			pair := [2]domain.FollowupState{from, to}
			changed, err := domain.AdvanceFollowup(from, to)
			switch {
			case changes[pair]:
				if err != nil || !changed {
					t.Errorf("%s -> %s: changed %v, err %v; want a change", from, to, changed, err)
				}
			case same[pair]:
				if err != nil || changed {
					t.Errorf("%s -> %s: changed %v, err %v; want no change", from, to, changed, err)
				}
			default:
				if e, ok := errs.As(err); !errors.Is(err, domain.ErrFollowupTransition) || !ok || e.Code != errs.CodeInvalidStateTransition || changed {
					t.Errorf("%s -> %s: changed %v, err %v; want ErrFollowupTransition", from, to, changed, err)
				}
			}
		}
	}
	if len(domain.FollowupStates()) != 5 {
		t.Fatalf("FollowupStates() = %v, want five", domain.FollowupStates())
	}
}
