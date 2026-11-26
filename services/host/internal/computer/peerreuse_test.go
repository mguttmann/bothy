// SPDX-License-Identifier: Apache-2.0

package computer

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/host/internal/confirm"
)

// starts is a fake table of process start times.
type starts map[int]time.Time

func (s starts) of(pid int) (time.Time, error) {
	if t, ok := s[pid]; ok {
		return t, nil
	}
	return time.Time{}, errors.New("no such process")
}

// TestPeerIdentityPIDReuse: the identity of a helper peer is its process ID with the start time of
// the process, which must lie before the connection was accepted; a process that took over the
// PID of a peer that exited (PID reuse) starts later and is refused, and so is a peer whose
// identity changed between the admission and a later check (27 9.4, D-CU-03-12).
func TestPeerIdentityPIDReuse(t *testing.T) {
	accepted := time.Unix(1_800_000_000, 0)
	st := starts{10: accepted.Add(-time.Minute), 20: accepted.Add(time.Second)}
	r := rule(procTable{10: helperPath, 20: helperPath}, "", nil, map[string]bool{helperPath: true})
	r.StartTime = st.of
	id, err := r.Identify(context.Background(), confirm.Peer{UID: uidAlex, PID: 10}, accepted)
	if err != nil || id.PID != 10 || !id.Start.Equal(accepted.Add(-time.Minute)) {
		t.Fatalf("genuine helper: %+v %v", id, err)
	}
	if _, err := r.Identify(context.Background(), confirm.Peer{UID: uidAlex, PID: 20}, accepted); !errors.Is(err, ErrPeerRefused) {
		t.Errorf("a process started after the accept (PID reuse): %v", err)
	}
	if err := r.Same(context.Background(), confirm.Peer{UID: uidAlex, PID: 10}, id); err != nil {
		t.Errorf("unchanged peer: %v", err)
	}
	st[10] = accepted.Add(time.Hour) // the peer exited and another process took its PID
	if err := r.Same(context.Background(), confirm.Peer{UID: uidAlex, PID: 10}, id); !errors.Is(err, ErrPeerRefused) {
		t.Errorf("reused PID after the admission: %v", err)
	}
	r.StartTime = nil
	if _, err := r.Identify(context.Background(), confirm.Peer{UID: uidAlex, PID: 10}, accepted); !errors.Is(err, ErrPeerRefused) {
		t.Errorf("no start time source: %v", err)
	}
}

// TestHelperRecheckAfterHelloPIDReuse: a peer whose PID changes identity while the host service
// waits for hello is refused after hello (the connection closes, the helper stays absent).
func TestHelperRecheckAfterHelloPIDReuse(t *testing.T) {
	r := newRig(t)
	r.h.o.Rule = rule(procTable{4242: helperPath}, "", nil, map[string]bool{helperPath: true})
	r.h.o.Rule.Console.ConsoleOwner = func(context.Context) (string, error) { return uidAlex, nil }
	now := time.Now()
	calls := 0
	r.h.o.Rule.StartTime = func(int) (time.Time, error) {
		calls++
		if calls == 1 {
			return now.Add(-time.Minute), nil
		}
		return now.Add(time.Minute), nil // another process took the PID before hello was read
	}
	r.connect(hello())
	deadline := time.Now().Add(wait)
	for time.Now().Before(deadline) && calls < 2 {
		time.Sleep(5 * time.Millisecond)
	}
	time.Sleep(50 * time.Millisecond)
	if st := r.h.State(); st.Connected {
		t.Fatalf("helper admitted after its identity changed: %+v", st)
	}
}
