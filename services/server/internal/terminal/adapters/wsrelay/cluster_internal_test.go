// SPDX-License-Identifier: Apache-2.0

package wsrelay

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"slices"
	"strings"
	gosync "sync"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/otel/attr"
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

// TestRelay_LoneSideEndsAfterTheOpeningTimedOut (24 9 item 4, D-M2-47-8): a lone side of an
// opening that ends by itself after the opening ended at open_timeout on another node counts as
// unpaired with its side, exactly once, also when it closes before the end arrives through the
// log tail of this node (with several nodes the host service closes its data connection on the
// session.close of the node of its control connection; the flake of TM-T15 without hash
// routing). A side that ends while the opening still runs, or after another end, counts nothing.
func TestRelay_LoneSideEndsAfterTheOpeningTimedOut(t *testing.T) {
	t.Parallel()
	timedOut := domain.End{Reason: domain.CloseOpenFailed, Detail: domain.DetailOpenTimeout}
	clientClosed := domain.End{Reason: domain.CloseClientClosed}
	cases := []struct {
		name, path string
		end        *domain.End
		want       string
	}{
		{"host_after_open_timeout", "/host", &timedOut, "unpaired:host"},
		{"device_after_open_timeout", "/dev", &timedOut, "unpaired:client"},
		{"host_while_opening", "/host", nil, ""},
		{"host_after_another_end", "/host", &clientClosed, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			ctx := testCtx(t)
			uc := &fakeRelayUC{ended: c.end}
			r, timers, _ := clusterRelay(t, uc)
			session, host, device := ids.New(), ids.New(), ids.New()
			r.Expect(app.Expectation{SessionID: session, HostID: host, DeviceID: device, Deadline: time.Now().Add(time.Minute)})
			srv := relayServer(t, r, session, host, device)
			ws, _ := dial(t, ctx, srv, c.path)
			waitFor(t, "the lone side", func() bool { return timers.Pending() == 2 })
			_ = ws.Close(websocket.StatusNormalClosure, "")
			waitFor(t, "the side gone", func() bool {
				r.mu.Lock()
				defer r.mu.Unlock()
				s := r.slots[session]
				return s != nil && s.dev == nil && s.hst == nil
			})
			waitFor(t, "the read of the end", func() bool {
				uc.mu.Lock()
				defer uc.mu.Unlock()
				return uc.endReads == 1
			})
			// The count follows the read in the same effect.
			if c.want != "" {
				waitFor(t, "the count", func() bool { return unpairedCalls(uc.all()) != "" })
			}
			// The log tail delivers the end afterwards: the side is gone, nothing counts again.
			if c.end != nil {
				r.ObserveLogEvent(ctx, eventSessionChg, session, endPayload(t, host, *c.end))
			}
			if got := unpairedCalls(uc.all()); got != c.want {
				t.Fatalf("unpaired callbacks %q, want %q (all %q)", got, c.want, uc.all())
			}
		})
	}
}

// unpairedCalls returns the unpaired callbacks of calls, comma separated.
func unpairedCalls(calls string) string {
	var out []string
	for _, c := range strings.Split(calls, ",") {
		if strings.HasPrefix(c, "unpaired:") {
			out = append(out, c)
		}
	}
	return strings.Join(out, ",")
}

// endPayload is the payload of the terminal_session.changed of the end e of a session of host.
func endPayload(t *testing.T, host ids.ID, e domain.End) []byte {
	t.Helper()
	b, err := json.Marshal(sessionEnd{HostID: host.String(), State: domain.SessionClosed, CloseReason: string(e.Reason), CloseDetail: string(e.Detail)})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// TestRelay_EveryUnpairedCountLogsOneRecord (D-M2-47-11, TM-T15): whichever path counts a side
// as unpaired, its own lone timer, the end of the opening at open_timeout that the log tail
// brings before that timer, or the side's own close after that end, the node logs exactly one
// record "terminal relay side unpaired" with the side and its close code (the flake of TM-T15
// without hash routing: node B counted the host side through the log tail and logged nothing).
func TestRelay_EveryUnpairedCountLogsOneRecord(t *testing.T) {
	t.Parallel()
	timedOut := domain.End{Reason: domain.CloseOpenFailed, Detail: domain.DetailOpenTimeout}
	cases := []struct {
		name, path, side string
		end              *domain.End
		run              func(ctx context.Context, r *Relay, timers *fakehost.Timers, ws *websocket.Conn, session, host ids.ID)
		code             float64
	}{
		{"lone_timer", "/host", "host", nil,
			func(_ context.Context, _ *Relay, timers *fakehost.Timers, _ *websocket.Conn, _, _ ids.ID) {
				timers.Fire()
			}, 4102},
		{"end_from_the_log_tail", "/host", "host", nil,
			func(ctx context.Context, r *Relay, _ *fakehost.Timers, _ *websocket.Conn, session, host ids.ID) {
				r.ObserveLogEvent(ctx, eventSessionChg, session, endPayload(t, host, timedOut))
			}, 4102},
		{"end_from_the_log_tail_device", "/dev", "client", nil,
			func(ctx context.Context, r *Relay, _ *fakehost.Timers, _ *websocket.Conn, session, host ids.ID) {
				r.ObserveLogEvent(ctx, eventSessionChg, session, endPayload(t, host, timedOut))
			}, 4102},
		{"own_close_after_the_end", "/host", "host", &timedOut,
			func(_ context.Context, _ *Relay, _ *fakehost.Timers, ws *websocket.Conn, _, _ ids.ID) {
				_ = ws.Close(websocket.StatusNormalClosure, "")
			}, 1000},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			ctx := testCtx(t)
			uc := &fakeRelayUC{ended: c.end}
			r, timers, _ := clusterRelay(t, uc)
			logs := &lockedBuffer{}
			r.log = slog.New(slog.NewJSONHandler(logs, nil))
			session, host, device := ids.New(), ids.New(), ids.New()
			r.Expect(app.Expectation{SessionID: session, HostID: host, DeviceID: device, Deadline: time.Now().Add(time.Minute)})
			srv := relayServer(t, r, session, host, device)
			ws, _ := dial(t, ctx, srv, c.path)
			waitFor(t, "the lone side", func() bool { return timers.Pending() == 2 })
			c.run(ctx, r, timers, ws, session, host)
			waitFor(t, "the count", func() bool { return unpairedCalls(uc.all()) != "" })
			if got := unpairedCalls(uc.all()); got != "unpaired:"+c.side {
				t.Fatalf("unpaired callbacks %q", got)
			}
			recs := logs.records(msgSideUnpaired)
			if len(recs) != 1 || recs[0][attr.TerminalSide] != c.side || recs[0][attr.CloseCode] != c.code {
				t.Fatalf("records %v, want one with side %s and code %v", recs, c.side, c.code)
			}
		})
	}
}

// lockedBuffer collects JSON log lines for concurrent writers.
type lockedBuffer struct {
	mu  gosync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

// records returns the decoded records with message msg.
func (b *lockedBuffer) records(msg string) []map[string]any {
	b.mu.Lock()
	defer b.mu.Unlock()
	var out []map[string]any
	for _, line := range bytes.Split(b.buf.Bytes(), []byte("\n")) {
		var m map[string]any
		if json.Unmarshal(line, &m) == nil && m["msg"] == msg {
			out = append(out, m)
		}
	}
	return out
}
