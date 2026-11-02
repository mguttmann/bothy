// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/terminal/domain"
)

// ownerWindows are testWindows with the windows of the owner account mode
// (FIX-terminal-session-lifecycle): the detach lifetime and the maximum of an owner account
// session.
var ownerWindows = func() domain.Windows {
	w := testWindows
	w.OwnerDetachGrace, w.OwnerMaxDuration = 24*time.Hour, 7*24*time.Hour
	return w
}()

// TestOwnerAccountDetachOutlivesTheGrace (FIX-terminal-session-lifecycle, D-TSL-1 to D-TSL-3): a
// detached session in the owner account mode stays past terminal.detach_grace, the idle timeout and
// terminal.max_session_duration, ends with detach_expired after the owner detach lifetime, and
// after a reattach idle counts again from the grant. A session of a separate session user keeps
// the old windows.
func TestOwnerAccountDetachOutlivesTheGrace(t *testing.T) {
	t.Parallel()
	w := ownerWindows
	detachedAt := t0.Add(10 * time.Minute)
	owner := domain.SessionTimes{State: domain.SessionDetached, OwnerAccount: true, OpenedAt: t0, GrantedAt: t0,
		StartedAt: t0.Add(time.Second), DetachedAt: detachedAt, LastInputAt: detachedAt.Add(-time.Second)}

	// Two hours after the detach: past the old grace (10m), idle (31m) and far before 24h.
	if r, _, ok := domain.SweepVerdict("active", false, false, owner, detachedAt.Add(2*time.Hour), w); ok {
		t.Fatalf("an owner account session detached for 2h ended with %s", r)
	}
	// Past terminal.max_session_duration (8h here) the owner session still lives.
	if r, _, ok := domain.SweepVerdict("active", false, false, owner, t0.Add(w.MaxDuration+time.Hour), w); ok {
		t.Fatalf("an owner account session ended at the old maximum with %s", r)
	}
	at, r := domain.RelayDeadline(owner, time.Time{}, w)
	if r != domain.CloseDetachExpired || !at.Equal(detachedAt.Add(w.OwnerDetachGrace)) {
		t.Fatalf("relay deadline of a detached owner session: %v %s", at, r)
	}
	if r, _, ok := domain.SweepVerdict("active", false, false, owner, detachedAt.Add(w.OwnerDetachGrace-tick), w); ok {
		t.Fatalf("one tick before the owner detach lifetime ended with %s", r)
	}
	if r, _, ok := domain.SweepVerdict("active", false, false, owner, detachedAt.Add(w.OwnerDetachGrace), w); !ok || r != domain.CloseDetachExpired {
		t.Fatalf("at the owner detach lifetime: %s %v", r, ok)
	}

	// Reattached 20h later from another device: idle counts from the new grant, not from the old
	// last input, so neither the sweep nor the relay ends the session at once.
	grant := detachedAt.Add(20 * time.Hour)
	back := owner
	back.State, back.DetachedAt, back.GrantedAt = domain.SessionActive, time.Time{}, grant
	if r, _, ok := domain.SweepVerdict("active", false, false, back, grant.Add(time.Minute), w); ok {
		t.Fatalf("a reattached owner session ended with %s", r)
	}
	at, r = domain.RelayDeadline(back, time.Time{}, w)
	if r != domain.CloseIdleTimeout || !at.Equal(grant.Add(w.Idle+w.IdleGrace)) {
		t.Fatalf("idle of a reattached owner session: %v %s", at, r)
	}
	// The idle timeout of an attached owner session stays.
	if r, _, ok := domain.SweepVerdict("active", false, false, back, grant.Add(w.Idle+w.IdleGrace), w); !ok || r != domain.CloseIdleTimeout {
		t.Fatalf("idle of an attached owner session: %s %v", r, ok)
	}
	// The owner maximum still bounds it.
	if r, _, ok := domain.SweepVerdict("active", false, false, owner, t0.Add(w.OwnerMaxDuration), w); !ok || r != domain.CloseMaxDuration {
		t.Fatalf("owner maximum: %s %v", r, ok)
	}

	// A session of a separate session user keeps terminal.detach_grace and the idle across a detach.
	plain := owner
	plain.OwnerAccount = false
	if r, _, ok := domain.SweepVerdict("active", false, false, plain, detachedAt.Add(w.DetachGrace), w); !ok || r != domain.CloseDetachExpired {
		t.Fatalf("a separate session user: %s %v", r, ok)
	}
	// Without owner windows configured the owner session falls back to the plain ones.
	if r, _, ok := domain.SweepVerdict("active", false, false, owner, detachedAt.Add(testWindows.DetachGrace), testWindows); !ok || r != domain.CloseDetachExpired {
		t.Fatalf("no owner windows: %s %v", r, ok)
	}
}
