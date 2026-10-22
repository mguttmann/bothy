// SPDX-License-Identifier: Apache-2.0

package wsrelay

import (
	"context"
	"slices"
	"strings"
	gosync "sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
	"github.com/OWNER/bothy/services/server/internal/terminal/app"
	"github.com/OWNER/bothy/services/server/internal/terminal/domain"
	"github.com/OWNER/bothy/services/server/test/terminal/fakehost"
)

// The relay across several nodes (M2-47 Design notes 3 and 8) with fake timers and recorded
// callbacks: the expectation of terminal.session_open on a node without sides, the rules of the
// lone side and terminal.session_paired.

// clusterRelay returns a relay in mode multi with fake timers and a recorder of sent signals.
func clusterRelay(t *testing.T, uc *fakeRelayUC) (*Relay, *fakehost.Timers, *sentSignals) {
	t.Helper()
	timers := &fakehost.Timers{}
	r, err := NewRelay(RelayConfig{BufferMaxBytes: 1 << 16, OpenTimeout: time.Minute, Multi: true}, uc, nil, timers, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	sent := &sentSignals{}
	r.BindSender(sent.send)
	return r, timers, sent
}

// sentSignals records the signals the relay sends.
type sentSignals struct {
	mu   gosync.Mutex
	list []domain.Signal
}

func (s *sentSignals) send(_ context.Context, sig domain.Signal) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.list = append(s.list, sig)
	return nil
}

func (s *sentSignals) all() []domain.Signal {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.list)
}

// calls returns the recorded callbacks.
func (f *fakeRelayUC) all() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return strings.Join(f.calls, ",")
}

// testCtx is a context bounded by testkit.Patience for one test.
func testCtx(t *testing.T) context.Context { return testkit.Context(t) }

// waitFor polls cond until it holds, bounded by testkit.Patience.
func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	testkit.Until(t, what, cond)
}

// TestRelay_SidelessExpectationDeadline: the deadline of an expectation without sides decides by
// the session state read from the database: a session still in opening ends (compare-and-swap),
// an active one is left alone; on a host with require_local_confirm an expectation that received
// terminal.session_paired only removes itself, without it the opening ends; the node's own
// terminal.session_paired, which comes back through the listener after a pairing here that ended
// before session.started, changes nothing, so the opening still ends.
func TestRelay_SidelessExpectationDeadline(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name          string
		state         string
		confirm, mark bool
		pairedHere    bool
		want          string
		slotGone      bool
	}{
		{"opening", domain.SessionOpening, false, false, false, "open_failed:open_timeout", false},
		{"active_elsewhere", domain.SessionActive, false, false, false, "", false},
		{"confirm_paired_elsewhere", domain.SessionOpening, true, true, false, "", true},
		{"confirm_never_paired", domain.SessionOpening, true, false, false, "open_failed:open_timeout", false},
		{"confirm_own_pairing_returned", domain.SessionOpening, true, true, true, "open_failed:open_timeout", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			uc := &fakeRelayUC{state: c.state}
			r, timers, _ := clusterRelay(t, uc)
			session := ids.New()
			r.Expect(app.Expectation{SessionID: session, HostID: ids.New(), DeviceID: ids.New(), Deadline: time.Now().Add(time.Minute),
				LocalConfirm: c.confirm})
			if c.pairedHere {
				// A pairing on this node that ended before session.started.
				r.mu.Lock()
				r.slots[session].pairedOnce = true
				r.mu.Unlock()
			}
			if c.mark {
				r.MarkPairedElsewhere(session)
			}
			if timers.Fire() != 1 {
				t.Fatal("no deadline")
			}
			if got := uc.all(); got != c.want {
				t.Fatalf("callbacks %q, want %q", got, c.want)
			}
			r.mu.Lock()
			gone := r.slots[session] == nil
			r.mu.Unlock()
			if gone != c.slotGone {
				t.Fatalf("slot gone %v, want %v", gone, c.slotGone)
			}
		})
	}
}

// TestRelay_LoneSides (Design note 8): a lone device of an opening and a lone host data connection
// of an opening close with 4102 at their own timer, count bothy.terminal.relay.unpaired with their
// side and end the opening; a lone device of a started session gets 4102 without an end; a host
// data connection of a started session has no timer; a host data connection that arrives after
// terminal.session_open pairs instead of getting 1000.
func TestRelay_LoneSides(t *testing.T) {
	t.Parallel()
	t.Run("device_opening", func(t *testing.T) {
		t.Parallel()
		ctx := testCtx(t)
		uc := &fakeRelayUC{}
		r, timers, _ := clusterRelay(t, uc)
		session, host, device := ids.New(), ids.New(), ids.New()
		r.Expect(app.Expectation{SessionID: session, HostID: host, DeviceID: device, Deadline: time.Now().Add(time.Minute)})
		srv := relayServer(t, r, session, host, device)
		_, code := dial(t, ctx, srv, "/dev")
		waitFor(t, "the device side", func() bool { return timers.Pending() == 2 })
		timers.Fire()
		if c := <-code; c != 4102 {
			t.Fatalf("device closed with %d", c)
		}
		waitFor(t, "the callbacks", func() bool {
			return strings.Contains(uc.all(), "unpaired:client") && strings.Contains(uc.all(), "open_failed:open_timeout")
		})
	})
	t.Run("host_opening", func(t *testing.T) {
		t.Parallel()
		ctx := testCtx(t)
		uc := &fakeRelayUC{}
		r, timers, _ := clusterRelay(t, uc)
		session, host, device := ids.New(), ids.New(), ids.New()
		r.Expect(app.Expectation{SessionID: session, HostID: host, DeviceID: device, Deadline: time.Now().Add(time.Minute)})
		srv := relayServer(t, r, session, host, device)
		_, code := dial(t, ctx, srv, "/host")
		waitFor(t, "the host side", func() bool { return timers.Pending() == 2 })
		timers.Fire()
		if c := <-code; c != 4102 {
			t.Fatalf("host closed with %d", c)
		}
		waitFor(t, "the callbacks", func() bool {
			return strings.Contains(uc.all(), "unpaired:host") && strings.Contains(uc.all(), "open_failed:open_timeout")
		})
	})
	t.Run("started", func(t *testing.T) {
		t.Parallel()
		ctx := testCtx(t)
		uc := &fakeRelayUC{state: domain.SessionDetached}
		r, timers, _ := clusterRelay(t, uc)
		session, host, device := ids.New(), ids.New(), ids.New()
		r.Expect(app.Expectation{SessionID: session, HostID: host, DeviceID: device, Deadline: time.Now().Add(time.Minute), Started: true})
		srv := relayServer(t, r, session, host, device)
		_, hostCode := dial(t, ctx, srv, "/host")
		waitFor(t, "the host side", func() bool {
			r.mu.Lock()
			defer r.mu.Unlock()
			return r.slots[session].hst != nil
		})
		if timers.Pending() != 1 {
			t.Fatalf("%d timers, want only the deadline (no lone timer for the host side)", timers.Pending())
		}
		timers.Fire()
		r.mu.Lock()
		hostStays := r.slots[session].hst != nil
		r.mu.Unlock()
		if !hostStays {
			t.Fatal("the host side of a started session closed at the deadline")
		}
		select {
		case c := <-hostCode:
			t.Fatalf("host closed with %d", c)
		default:
		}
		if strings.Contains(uc.all(), "unpaired") || strings.Contains(uc.all(), "open_failed") {
			t.Fatalf("callbacks %s", uc.all())
		}
	})
}

// TestRelay_SessionPairedSentOnce: the pairing node announces the pairing of a session in opening
// on a host with require_local_confirm exactly once per pairing; other sessions announce nothing.
func TestRelay_SessionPairedSentOnce(t *testing.T) {
	t.Parallel()
	ctx := testkit.Context(t)
	for _, confirm := range []bool{true, false} {
		uc := &fakeRelayUC{}
		r, _, sent := clusterRelay(t, uc)
		session, host, device := ids.New(), ids.New(), ids.New()
		r.Expect(app.Expectation{SessionID: session, HostID: host, DeviceID: device, Deadline: time.Now().Add(time.Minute), LocalConfirm: confirm})
		srv := relayServer(t, r, session, host, device)
		dial(t, ctx, srv, "/dev")
		dial(t, ctx, srv, "/host")
		waitFor(t, "the pairing", func() bool {
			r.mu.Lock()
			defer r.mu.Unlock()
			return r.slots[session].paired
		})
		want := 0
		if confirm {
			want = 1
		}
		waitFor(t, "the signal", func() bool { return len(sent.all()) == want })
		time.Sleep(50 * time.Millisecond)
		got := sent.all()
		if len(got) != want || want == 1 && got[0] != (domain.Signal{Kind: domain.SignalSessionPaired, HostID: host, SessionID: session}) {
			t.Fatalf("confirm %v: sent %+v", confirm, got)
		}
	}
}
