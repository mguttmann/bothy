// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/gateway/domain"
)

// TestSessionTransitions holds the allowed transitions of a gateway session: identify or resume
// make it ready, a lost connection detaches a ready one, resume attaches a detached one again,
// and every state but closed may close; closed is final.
func TestSessionTransitions(t *testing.T) {
	t.Parallel()
	allowed := map[domain.State][]domain.State{
		domain.StateAwaitingIdentify: {domain.StateReady, domain.StateClosed},
		domain.StateReady:            {domain.StateDetached, domain.StateClosed},
		domain.StateDetached:         {domain.StateReady, domain.StateClosed},
		domain.StateClosed:           nil,
	}
	all := []domain.State{domain.StateAwaitingIdentify, domain.StateReady, domain.StateDetached, domain.StateClosed}
	for from, tos := range allowed {
		for _, to := range all {
			want := false
			for _, ok := range tos {
				if ok == to {
					want = true
				}
			}
			if got := from.CanBecome(to); got != want {
				t.Errorf("%s -> %s: %v, want %v", from, to, got, want)
			}
		}
	}
}

// TestHeartbeatDeadline is the rule of the heartbeat ruling (M1-64 note 14): the server closes
// when no heartbeat arrived for sync.heartbeat_interval times (sync.heartbeat_miss_limit + 1)
// since the last one; the extra interval absorbs the random delay of the first heartbeat.
func TestHeartbeatDeadline(t *testing.T) {
	t.Parallel()
	last := time.Date(2026, 10, 16, 9, 0, 0, 0, time.UTC)
	if got, want := domain.HeartbeatDeadline(last, 30*time.Second, 2), last.Add(90*time.Second); !got.Equal(want) {
		t.Errorf("deadline %v, want %v", got, want)
	}
	if got, want := domain.HeartbeatDeadline(last, 30*time.Second, 0), last.Add(30*time.Second); !got.Equal(want) {
		t.Errorf("miss limit 0: %v, want %v", got, want)
	}
}

// TestReauthAt: reauth_required goes out sync.reauth_lead_time before the expiry, at once when the
// lead time is longer than the remaining validity.
func TestReauthAt(t *testing.T) {
	t.Parallel()
	exp := time.Date(2026, 10, 16, 9, 10, 0, 0, time.UTC)
	if got, want := domain.ReauthAt(exp, time.Minute), exp.Add(-time.Minute); !got.Equal(want) {
		t.Errorf("reauth at %v, want %v", got, want)
	}
}
