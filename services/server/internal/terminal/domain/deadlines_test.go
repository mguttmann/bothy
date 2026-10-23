// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/terminal/domain"
)

// tick is one clock tick of the boundary tests.
const tick = time.Millisecond

// testWindows are distinct windows, so every deadline of a test is unambiguous.
var testWindows = domain.Windows{
	Idle: 30 * time.Minute, IdleGrace: time.Minute, MaxDuration: 8 * time.Hour,
	DetachGrace: 10 * time.Minute, OpenTimeout: 2 * time.Minute,
}

// t0 is the opening instant of the test sessions.
var t0 = time.Date(2026, 10, 20, 9, 0, 0, 0, time.UTC)

// TestRelayDeadline (Design note 7 of M2-46).
func TestRelayDeadline(t *testing.T) {
	t.Parallel()
	w := testWindows
	idle := w.Idle + w.IdleGrace
	active := domain.SessionTimes{State: domain.SessionActive, OpenedAt: t0, GrantedAt: t0, StartedAt: t0.Add(time.Second)}

	// Without input the idle reference is started_at.
	at, r := domain.RelayDeadline(active, time.Time{}, w)
	if r != domain.CloseIdleTimeout || !at.Equal(active.StartedAt.Add(idle)) {
		t.Fatalf("idle from started_at: got %v %s", at, r)
	}
	// Device input moves the idle deadline.
	input := t0.Add(time.Hour)
	at, r = domain.RelayDeadline(active, input, w)
	if r != domain.CloseIdleTimeout || !at.Equal(input.Add(idle)) {
		t.Fatalf("idle moved by device input: got %v %s", at, r)
	}
	// The stored last_input_at counts when it is later than the device input of this process.
	stored := active
	stored.LastInputAt = t0.Add(2 * time.Hour)
	at, _ = domain.RelayDeadline(stored, input, w)
	if !at.Equal(stored.LastInputAt.Add(idle)) {
		t.Fatalf("idle from last_input_at: got %v", at)
	}
	// One tick before the instant keeps, the instant ends.
	if domain.Reached(at, at.Add(-tick)) || !domain.Reached(at, at) || !domain.Reached(at, at.Add(tick)) {
		t.Fatal("Reached is not the instant itself")
	}

	// The maximum stays from opened_at after a reattach (a later grant, opened_at unchanged).
	late := domain.SessionTimes{State: domain.SessionActive, OpenedAt: t0, GrantedAt: t0.Add(7 * time.Hour),
		StartedAt: t0, LastInputAt: t0.Add(7*time.Hour + 50*time.Minute)}
	at, r = domain.RelayDeadline(late, t0.Add(7*time.Hour+55*time.Minute), w)
	if r != domain.CloseMaxDuration || !at.Equal(t0.Add(w.MaxDuration)) {
		t.Fatalf("max from opened_at: got %v %s", at, r)
	}

	// Detach from detached_at, earlier than idle.
	detached := active
	detached.State = domain.SessionDetached
	detached.DetachedAt = t0.Add(5 * time.Minute)
	at, r = domain.RelayDeadline(detached, t0.Add(4*time.Minute), w)
	if r != domain.CloseDetachExpired || !at.Equal(detached.DetachedAt.Add(w.DetachGrace)) {
		t.Fatalf("detach from detached_at: got %v %s", at, r)
	}
	// A detached_at outside the state detached is no deadline.
	stale := active
	stale.DetachedAt = t0
	if _, r = domain.RelayDeadline(stale, time.Time{}, w); r != domain.CloseIdleTimeout {
		t.Fatalf("detached_at in active: got %s", r)
	}
	// The earliest deadline wins: idle before detach when the detach grace is longer.
	long := w
	long.DetachGrace = 2 * time.Hour
	at, r = domain.RelayDeadline(detached, time.Time{}, long)
	if r != domain.CloseIdleTimeout || !at.Equal(detached.StartedAt.Add(idle)) {
		t.Fatalf("earliest wins: got %v %s", at, r)
	}

	// Ties: max_duration before detach_expired before idle_timeout.
	tie := domain.Windows{Idle: 50 * time.Minute, IdleGrace: 10 * time.Minute, MaxDuration: time.Hour, DetachGrace: time.Hour}
	all := domain.SessionTimes{State: domain.SessionDetached, OpenedAt: t0, StartedAt: t0, DetachedAt: t0}
	if at, r = domain.RelayDeadline(all, time.Time{}, tie); r != domain.CloseMaxDuration || !at.Equal(t0.Add(time.Hour)) {
		t.Fatalf("tie of three: got %v %s", at, r)
	}
	tie.MaxDuration = 2 * time.Hour
	if at, r = domain.RelayDeadline(all, time.Time{}, tie); r != domain.CloseDetachExpired || !at.Equal(t0.Add(time.Hour)) {
		t.Fatalf("tie of detach and idle: got %v %s", at, r)
	}
	tie.MaxDuration, tie.DetachGrace = time.Hour, 2*time.Hour
	if _, r = domain.RelayDeadline(all, time.Time{}, tie); r != domain.CloseMaxDuration {
		t.Fatalf("tie of max and idle: got %s", r)
	}
}

// verdict is the result of SweepVerdict.
type verdict struct {
	r  domain.CloseReason
	d  domain.CloseDetail
	ok bool
}

// sweep calls SweepVerdict with testWindows.
func sweep(host string, confirm, ended bool, s domain.SessionTimes, now time.Time) verdict {
	r, d, ok := domain.SweepVerdict(host, confirm, ended, s, now, testWindows)
	return verdict{r, d, ok}
}

// TestSweepVerdict (Design note 8 a of M2-46).
func TestSweepVerdict(t *testing.T) {
	t.Parallel()
	w := testWindows
	none := verdict{}
	active := domain.SessionTimes{State: domain.SessionActive, OpenedAt: t0, GrantedAt: t0, StartedAt: t0}
	quiet := t0.Add(time.Minute) // before every limit

	check := func(name string, got, want verdict) {
		t.Helper()
		if got != want {
			t.Errorf("%s: got %+v, want %+v", name, got, want)
		}
	}

	// Host state and login session end at once.
	check("active host", sweep("active", false, false, active, quiet), none)
	check("disabled", sweep("disabled", false, false, active, quiet), verdict{domain.CloseHostDisabled, "", true})
	check("retired", sweep("retired", false, false, active, quiet), verdict{domain.CloseHostRetired, "", true})
	check("pairing", sweep("pairing", false, false, active, quiet), verdict{domain.CloseHostRekeyed, "", true})
	check("confirming", sweep("confirming", false, false, active, quiet), verdict{domain.CloseHostRekeyed, "", true})
	check("login ended", sweep("active", false, true, active, quiet), verdict{domain.CloseSessionRevoked, "", true})

	// Opening: measured from the last grant.
	opening := domain.SessionTimes{State: domain.SessionOpening, OpenedAt: t0, GrantedAt: t0.Add(10 * time.Minute)}
	openAt := opening.GrantedAt.Add(w.OpenTimeout)
	timeout := verdict{domain.CloseOpenFailed, domain.DetailOpenTimeout, true}
	check("opening before", sweep("active", false, false, opening, openAt.Add(-tick)), none)
	check("opening at", sweep("active", false, false, opening, openAt), timeout)
	check("opening past opened_at, not past the grant", sweep("active", false, false, opening, t0.Add(w.OpenTimeout+time.Minute)), none)
	check("opening with local confirm", sweep("active", true, false, opening, openAt), none)
	noGrant := opening
	noGrant.GrantedAt = time.Time{}
	check("opening without grant time", sweep("active", false, false, noGrant, t0.Add(w.OpenTimeout)), timeout)
	check("opening without grant time before", sweep("active", false, false, noGrant, t0.Add(w.OpenTimeout-tick)), none)
	// With local confirm the rules after the opening rule still bound it.
	maxAt := t0.Add(w.MaxDuration)
	check("opening with local confirm at max", sweep("active", true, false, opening, maxAt), verdict{domain.CloseMaxDuration, "", true})

	// Maximum duration from opened_at.
	late := active
	late.GrantedAt, late.LastInputAt = maxAt.Add(-time.Minute), maxAt.Add(-time.Minute)
	check("max before", sweep("active", false, false, late, maxAt.Add(-tick)), none)
	check("max at", sweep("active", false, false, late, maxAt), verdict{domain.CloseMaxDuration, "", true})

	// Detach from detached_at.
	detached := active
	detached.State, detached.DetachedAt, detached.LastInputAt = domain.SessionDetached, t0.Add(time.Minute), t0.Add(time.Minute)
	detachAt := detached.DetachedAt.Add(w.DetachGrace)
	check("detach before", sweep("active", false, false, detached, detachAt.Add(-tick)), none)
	check("detach at", sweep("active", false, false, detached, detachAt), verdict{domain.CloseDetachExpired, "", true})

	// Idle with and without the stored last_input_at.
	idle := w.Idle + w.IdleGrace
	idleEnd := verdict{domain.CloseIdleTimeout, "", true}
	check("idle from started_at before", sweep("active", false, false, active, t0.Add(idle-tick)), none)
	check("idle from started_at at", sweep("active", false, false, active, t0.Add(idle)), idleEnd)
	typed := active
	typed.LastInputAt = t0.Add(time.Hour)
	check("idle from last_input_at before", sweep("active", false, false, typed, typed.LastInputAt.Add(idle-tick)), none)
	check("idle from last_input_at at", sweep("active", false, false, typed, typed.LastInputAt.Add(idle)), idleEnd)
	unstarted := domain.SessionTimes{State: domain.SessionActive, OpenedAt: t0}
	check("idle from opened_at", sweep("active", false, false, unstarted, t0.Add(idle)), idleEnd)

	// Precedence (note 8 a): host state, login, opening, max, detach, idle.
	all := domain.SessionTimes{State: domain.SessionDetached, OpenedAt: t0, GrantedAt: t0, StartedAt: t0, DetachedAt: t0}
	far := t0.Add(w.MaxDuration + time.Hour)
	check("host before login and max", sweep("disabled", false, true, all, far), verdict{domain.CloseHostDisabled, "", true})
	check("login before max", sweep("active", false, true, all, far), verdict{domain.CloseSessionRevoked, "", true})
	check("max before detach and idle", sweep("active", false, false, all, far), verdict{domain.CloseMaxDuration, "", true})
	check("detach before idle", sweep("active", false, false, all, t0.Add(idle+time.Hour)), verdict{domain.CloseDetachExpired, "", true})
	openingLong := domain.SessionTimes{State: domain.SessionOpening, OpenedAt: t0, GrantedAt: t0}
	check("login before opening", sweep("active", false, true, openingLong, far), verdict{domain.CloseSessionRevoked, "", true})
	check("opening before max", sweep("active", false, false, openingLong, far), timeout)

	// A closed session gives no verdict.
	closed := all
	closed.State = domain.SessionClosed
	check("closed", sweep("disabled", false, true, closed, far), none)
}

// TestPersistDue (Design note 7 d of M2-46).
func TestPersistDue(t *testing.T) {
	t.Parallel()
	every := time.Minute
	for _, c := range []struct {
		name string
		last time.Time
		now  time.Time
		want bool
	}{
		{"never persisted", time.Time{}, t0, true},
		{"one tick before", t0, t0.Add(every - tick), false},
		{"at the interval", t0, t0.Add(every), true},
		{"after the interval", t0, t0.Add(every + time.Second), true},
		{"just persisted", t0, t0, false},
	} {
		if got := domain.PersistDue(c.last, c.now, every); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}
