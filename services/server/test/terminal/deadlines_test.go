// SPDX-License-Identifier: Apache-2.0

package terminal_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/terminal/domain"
	"github.com/OWNER/bothy/services/server/test/terminal/fakehost"
)

// dlQuiet bounds the wait in which a test shows that a write did NOT happen (a test constant, not a
// product window: the relay writes in its own goroutine, so absence needs a short look).
const dlQuiet = 300 * time.Millisecond

// dlTime returns the timestamp column col of the session row (zero for NULL).
func dlTime(w *world, session ids.ID, col string) time.Time {
	w.t.Helper()
	var at *time.Time
	q := `SELECT ` + col + ` FROM terminal_sessions WHERE id = $1`
	if err := w.s.AdminDB().QueryRow(context.Background(), q, session).Scan(&at); err != nil {
		w.t.Fatalf("%s of %s: %v", col, session, err)
	}
	if at == nil {
		return time.Time{}
	}
	return at.UTC()
}

// dlAdvanceTo moves the fake clock to at and runs every relay deadline it reaches.
func dlAdvanceTo(w *world, at time.Time) {
	w.t.Helper()
	d := at.Sub(w.clk.Now())
	if d < 0 {
		w.t.Fatalf("the clock is past %s already (%s)", at, w.clk.Now())
	}
	w.advance(d)
}

// dlStopTo moves the fake clock to at WITHOUT running a relay deadline (a dead relay).
func dlStopTo(w *world, at time.Time) {
	w.t.Helper()
	d := at.Sub(w.clk.Now())
	if d < 0 {
		w.t.Fatalf("the clock is past %s already (%s)", at, w.clk.Now())
	}
	w.jump(d)
}

// dlEcho sends m from the device and waits for the echo of the Accept script; it returns len(m).
func dlEcho(w *world, d *fakehost.Device, m []byte) int64 {
	w.t.Helper()
	ctx := w.ctx()
	if err := d.Send(ctx, m); err != nil {
		w.t.Fatal(err)
	}
	if back, err := d.Recv(ctx); err != nil || !fakehost.Equal(back, m) {
		w.t.Fatalf("echo of %d bytes: %d bytes, %v", len(m), len(back), err)
	}
	return int64(len(m))
}

// dlBytes returns a message of n bytes.
func dlBytes(n int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte('a' + i%26)
	}
	return b
}

// dlCounts returns "bytes_in:bytes_out" of the session row.
func dlCounts(w *world, session ids.ID) string {
	return w.text(`SELECT bytes_in || ':' || bytes_out FROM terminal_sessions WHERE id = $1`, session)
}

// dlTerminateCounts returns "bytes_in:bytes_out" of the terminal.terminate entry of the session.
func dlTerminateCounts(w *world, session ids.ID) string {
	return w.text(`SELECT (details->>'bytes_in') || ':' || (details->>'bytes_out') FROM audit_events
		WHERE action = 'terminal.terminate' AND resource_id = $1`, session)
}

// dlEnds returns the number of terminal.terminate entries and of closed events of the session.
func dlEnds(w *world, session ids.ID) (terminates, closedEvents int) {
	w.t.Helper()
	return w.count(`SELECT count(*) FROM audit_events WHERE action = 'terminal.terminate' AND resource_id = $1`, session),
		w.count(`SELECT count(*) FROM event_log WHERE type = 'terminal_session.changed' AND resource_id = $1
			AND payload->>'state' = 'closed'`, session)
}

// dlEndedRow checks the end of a session whose device stream is gone (it left or never came):
// state closed with want ("closed:<reason>[:<detail>]"), one terminal.terminate, one closed event,
// session.close with reason at the Fake-Host h, the host data connection (when one is given)
// closed with 4100 and the reason, and the metric raised.
func dlEndedRow(w *world, session ids.ID, h *fakehost.Host, data *fakehost.DataConn, want string, reason domain.CloseReason) {
	w.t.Helper()
	w.waitState(session.String(), want)
	if data != nil {
		w.closeCode("data connection", data.CloseCode, 4100, string(reason))
	}
	if got, err := h.WaitClose(w.ctx(), session); err != nil || got != string(reason) {
		w.t.Fatalf("session.close at the host: %q %v, want %s", got, err, reason)
	}
	if n, e := dlEnds(w, session); n != 1 || e != 1 {
		w.t.Fatalf("%d entries terminal.terminate and %d closed events, want 1 and 1", n, e)
	}
	w.eventually("bothy.terminal.sessions.closed "+string(reason), func() bool { return w.metrics.closedOf(reason) > 0 })
}

// dlReattach attaches Alex's second device to the detached session l with a fresh step-up, waits
// for the new data connection of the Fake-Host, connects and runs the handshake; it returns the
// new live session.
func dlReattach(w *world, l live) live {
	w.t.Helper()
	r := w.attach(w.alex2, l.id.String(), true)
	if r.Status != http.StatusOK {
		w.t.Fatalf("attach from the second device: %d %s", r.Status, r.Body)
	}
	var g grantBody
	r.JSON(w.t, &g)
	if err := l.host.WaitOpens(w.ctx(), l.id, 2); err != nil {
		w.t.Fatal(err)
	}
	w.eventually("the new data connection", func() bool { d := l.host.Data(l.id); return d != nil && d != l.data })
	d := w.device(w.alex2, l.id.String())
	w.handshake(d, g.SessionToken)
	w.waitState(l.id.String(), "active")
	return live{g: g, id: l.id, dev: d, data: l.host.Data(l.id), host: l.host}
}

// dlSweepEnded runs terminal.sweep once and returns how many sessions it ended.
func dlSweepEnded(w *world) int {
	w.t.Helper()
	res, err := w.jobModules().TerminalSweep.Sweep(w.ctx())
	if err != nil {
		w.t.Fatalf("terminal.sweep: %v", err)
	}
	return res.Ended
}

// dlSweepAgain runs terminal.sweep a second time and checks that it changes nothing more: it ends
// nothing, and every session keeps its state, one terminal.terminate entry and one closed event.
func dlSweepAgain(w *world, ended ...ids.ID) {
	w.t.Helper()
	before := map[ids.ID]string{}
	for _, id := range ended {
		before[id] = w.state(id.String())
	}
	if n := dlSweepEnded(w); n != 0 {
		w.t.Fatalf("the second run ended %d sessions", n)
	}
	for _, id := range ended {
		if got := w.state(id.String()); got != before[id] {
			w.t.Fatalf("the second run changed %s: %s, was %s", id, got, before[id])
		}
		if n, e := dlEnds(w, id); n != 1 || e != 1 {
			w.t.Fatalf("after the second run: %d entries terminal.terminate and %d closed events, want 1 and 1", n, e)
		}
	}
}

// TestTMT11_IdleWithoutHostReport (TM-T11, 24 section 6.3, Design note 7 a and c; red team focus of
// the brief): the Fake-Host runs the Accept script, which sends nothing after session.started (the
// mode "never reports idle": no session.ended, no output), and the device sends nothing after the
// handshake. One tick before terminal.idle_timeout plus terminal.idle_grace the session is active;
// at the instant the relay ends it with idle_timeout: 4100 with the reason on both connections,
// session.close at the host, one terminal.terminate, one closed event, the metric.
func TestTMT11_IdleWithoutHostReport(t *testing.T) {
	t.Parallel()
	w := newWorld(t, worldOptions{})
	l := w.start(w.alex, w.r1, w.h1)
	w.advance(idleTimeout + idleGrace - tick)
	if got := w.state(l.id.String()); got != "active" {
		t.Fatalf("one tick before the idle deadline: %s", got)
	}
	if n := len(w.h1.Closes()); n != 0 {
		t.Fatalf("%d session.close before the deadline", n)
	}
	w.advance(tick)
	w.assertEnded(l, domain.CloseIdleTimeout)
}

// TestTMT11_InputResetsIdle (TM-T11, Design note 7 a): a data frame of the device (any binary
// message after the handshake; the Accept script echoes it) moves the idle reference to its time:
// at the first deadline the session stays active; it ends with idle_timeout at the time of the
// frame plus terminal.idle_timeout plus terminal.idle_grace, not one tick before.
func TestTMT11_InputResetsIdle(t *testing.T) {
	t.Parallel()
	w := newWorld(t, worldOptions{})
	l := w.start(w.alex, w.r1, w.h1)
	first := w.clk.Now().Add(idleTimeout + idleGrace)
	w.advance(idleTimeout / 2)
	dlEcho(w, l.dev, []byte("data frame"))
	input := w.clk.Now()
	dlAdvanceTo(w, first)
	if got := w.state(l.id.String()); got != "active" {
		t.Fatalf("at the deadline without the frame: %s", got)
	}
	dlAdvanceTo(w, input.Add(idleTimeout+idleGrace-tick))
	if got := w.state(l.id.String()); got != "active" {
		t.Fatalf("one tick before the moved deadline: %s", got)
	}
	w.advance(tick)
	w.assertEnded(l, domain.CloseIdleTimeout)
}

// TestTMT11_IdleReportedByHost (TM-T11, 24 section 10.3, Design note 1): the host service reports
// session.ended with idle_timeout at terminal.idle_timeout, before the server's own deadline; the
// session ends with idle_timeout and no close_detail through the end path of the host report
// (source host; no column stores the source, so the reason and the single audit entry are what
// shows it), with 4100 on both connections; the relay's later deadline changes nothing.
func TestTMT11_IdleReportedByHost(t *testing.T) {
	t.Parallel()
	w := newWorld(t, worldOptions{})
	l := w.start(w.alex, w.r1, w.h1)
	w.advance(idleTimeout)
	if got := w.state(l.id.String()); got != "active" {
		t.Fatalf("at terminal.idle_timeout before the report: %s", got)
	}
	if err := w.h1.Send("session.ended", map[string]any{"session_id": l.id.String(), "close_reason": "idle_timeout"}); err != nil {
		t.Fatal(err)
	}
	w.assertEnded(l, domain.CloseIdleTimeout)
	w.advance(idleGrace)
	if got := w.state(l.id.String()); got != "closed:idle_timeout" {
		t.Fatalf("after the server's deadline: %s", got)
	}
	if n, e := dlEnds(w, l.id); n != 1 || e != 1 {
		t.Fatalf("after the server's deadline: %d entries terminal.terminate and %d closed events", n, e)
	}
}

// TestTMT11_MaxDurationAcrossReattach (TM-T11, Design note 7 b and c): Alex opens, the device drops
// (detached), Alex attaches from her second device with a fresh step-up and the session is active
// again; with a device frame less than terminal.idle_timeout apart (so idle never ends it) the
// session ends with max_duration at opened_at plus terminal.max_session_duration, unchanged by the
// reattach; one tick before it is active.
func TestTMT11_MaxDurationAcrossReattach(t *testing.T) {
	t.Parallel()
	w := newWorld(t, worldOptions{})
	l := w.start(w.alex, w.r1, w.h1)
	opened := dlTime(w, l.id, "opened_at")
	if err := l.dev.Close(); err != nil {
		t.Fatal(err)
	}
	w.waitState(l.id.String(), "detached")
	l2 := dlReattach(w, l)
	if got := dlTime(w, l.id, "opened_at"); !got.Equal(opened) {
		t.Fatalf("opened_at moved on the reattach: %s, was %s", got, opened)
	}
	end := opened.Add(maxDuration)
	step := idleTimeout - time.Minute
	for w.clk.Now().Add(step).Before(end.Add(-tick)) {
		w.advance(step)
		if got := w.state(l.id.String()); got != "active" {
			t.Fatalf("at %s: %s", w.clk.Now().Sub(opened), got)
		}
		dlEcho(w, l2.dev, []byte("keep"))
	}
	dlAdvanceTo(w, end.Add(-tick))
	if got := w.state(l.id.String()); got != "active" {
		t.Fatalf("one tick before the maximum: %s", got)
	}
	w.advance(tick)
	w.assertEnded(l2, domain.CloseMaxDuration)
}

// TestTMT11_DetachGraceWithoutHost (TM-T11, Design note 7 b and c): the device drops, the Fake-Host
// keeps the PTY (it never reports an end); at detached_at plus terminal.detach_grace the server ends
// the session with detach_expired (4100 on the data connection, session.close at the host); one
// tick before it is still detached.
func TestTMT11_DetachGraceWithoutHost(t *testing.T) {
	t.Parallel()
	w := newWorld(t, worldOptions{})
	l := w.start(w.alex, w.r1, w.h1)
	if err := l.dev.Close(); err != nil {
		t.Fatal(err)
	}
	w.waitState(l.id.String(), "detached")
	// The pairing ended: the relay closes the host data connection with 1000, the PTY stays.
	w.closeCode("data connection at the detach", l.data.CloseCode, 1000, "")
	// The relay armed terminal.detach_grace when it detached the session, before the row changed.
	detached := dlTime(w, l.id, "detached_at")
	dlAdvanceTo(w, detached.Add(detachGrace-tick))
	if got := w.state(l.id.String()); got != "detached" {
		t.Fatalf("one tick before the detach grace: %s", got)
	}
	w.advance(tick)
	dlEndedRow(w, l.id, w.h1, nil, "closed:detach_expired", domain.CloseDetachExpired)
}

// TestTMT11_SweepCatchesDeadNode (TM-T11, 02 section 12.3, Design note 8 a and b): the relay of the
// process is "dead": the tests move the clock with w.jump, so no relay deadline runs, and
// only the handler of terminal.sweep can end a session. Each row ends one session with the reason
// of its rule (in the order of note 8) and runs the handler a second time, which changes nothing
// more; the other rows show what stays open.
func TestTMT11_SweepCatchesDeadNode(t *testing.T) {
	t.Parallel()

	// A host disabled behind the module's back ends its sessions with host_disabled; a session on
	// another host stays.
	t.Run("host_disabled", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t, worldOptions{})
		l := w.start(w.alex, w.r1, w.h1)
		other := w.start(w.sam, w.r2, w.h2)
		w.exec(`UPDATE hosts SET state = 'disabled', disabled_at = $2, disabled_reason = 'owner' WHERE id = $1`, w.r1, w.clk.Now())
		w.sweep()
		w.assertEndedHostLeft(l, domain.CloseHostDisabled)
		if got := w.state(other.id.String()); got != "active" {
			t.Fatalf("the session on r2: %s", got)
		}
		dlSweepAgain(w, l.id)
	})

	// A login session whose expires_at passed (no event marks it) ends with session_revoked; the
	// session of another login session stays.
	t.Run("session_revoked", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t, worldOptions{})
		l := w.start(w.alex, w.r1, w.h1)
		other := w.start(w.sam, w.r2, w.h2)
		w.exec(`UPDATE auth_sessions SET expires_at = $2 WHERE id = (SELECT auth_session_id FROM terminal_sessions WHERE id = $1)`,
			l.id, w.clk.Now())
		w.sweep()
		w.assertEnded(l, domain.CloseSessionRevoked)
		if got := w.state(other.id.String()); got != "active" {
			t.Fatalf("the session of Sam: %s", got)
		}
		dlSweepAgain(w, l.id)
	})

	// An opening without a device stream stays one tick before its grant plus
	// terminal.open_timeout and ends with open_failed open_timeout at the instant.
	t.Run("open_timeout", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t, worldOptions{})
		g := w.open(w.alex, w.r1)
		id := ids.MustParse(g.Session.ID)
		data, err := w.h1.WaitData(w.ctx(), id)
		if err != nil {
			t.Fatal(err)
		}
		granted := ids.MustParse(w.text(`SELECT token_jti::text FROM terminal_sessions WHERE id = $1`, id)).Time()
		dlStopTo(w, granted.Add(openTimeout-tick))
		if n := dlSweepEnded(w); n != 0 || w.state(g.Session.ID) != "opening" {
			t.Fatalf("one tick before the open deadline: %d ended, %s", n, w.state(g.Session.ID))
		}
		w.jump(tick)
		w.sweep()
		dlEndedRow(w, id, w.h1, data, "closed:open_failed:open_timeout", domain.CloseOpenFailed)
		dlSweepAgain(w, id)
	})

	// At opened_at plus terminal.max_session_duration the session ends with max_duration, which
	// comes before idle_timeout in the order of note 8 (the idle limit passed long before).
	t.Run("max_duration", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t, worldOptions{})
		l := w.start(w.alex, w.r1, w.h1)
		dlStopTo(w, dlTime(w, l.id, "opened_at").Add(maxDuration))
		w.sweep()
		w.assertEnded(l, domain.CloseMaxDuration)
		dlSweepAgain(w, l.id)
	})

	// A detached session stays one tick before detached_at plus terminal.detach_grace and ends
	// with detach_expired at the instant.
	t.Run("detach_expired", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t, worldOptions{})
		l := w.start(w.alex, w.r1, w.h1)
		if err := l.dev.Close(); err != nil {
			t.Fatal(err)
		}
		w.waitState(l.id.String(), "detached")
		w.closeCode("data connection at the detach", l.data.CloseCode, 1000, "")
		detached := dlTime(w, l.id, "detached_at")
		dlStopTo(w, detached.Add(detachGrace-tick))
		if n := dlSweepEnded(w); n != 0 || w.state(l.id.String()) != "detached" {
			t.Fatalf("one tick before the detach grace: %d ended, %s", n, w.state(l.id.String()))
		}
		w.jump(tick)
		w.sweep()
		dlEndedRow(w, l.id, w.h1, nil, "closed:detach_expired", domain.CloseDetachExpired)
		dlSweepAgain(w, l.id)
	})

	// The idle rule measures from the stored last_input_at: input written by the relay after
	// started_at keeps the session past started_at plus the windows, and it ends with idle_timeout
	// at the stored value plus terminal.idle_timeout plus terminal.idle_grace.
	t.Run("idle_timeout", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t, worldOptions{})
		l := w.start(w.alex, w.r1, w.h1)
		started := dlTime(w, l.id, "started_at")
		w.advance(idleTimeout / 2)
		dlEcho(w, l.dev, []byte("input"))
		input := w.clk.Now()
		w.eventually("last_input_at written", func() bool { return dlTime(w, l.id, "last_input_at").Equal(input) })
		dlStopTo(w, input.Add(idleTimeout+idleGrace-tick))
		if !w.clk.Now().After(started.Add(idleTimeout + idleGrace)) {
			t.Fatal("the row does not tell the stored last_input_at from started_at")
		}
		if n := dlSweepEnded(w); n != 0 || w.state(l.id.String()) != "active" {
			t.Fatalf("one tick before the stored idle deadline: %d ended, %s", n, w.state(l.id.String()))
		}
		w.jump(tick)
		w.sweep()
		w.assertEnded(l, domain.CloseIdleTimeout)
		dlSweepAgain(w, l.id)
	})

	// Sessions within all windows stay: an opening one tick before its open deadline, an active
	// one and a detached one.
	t.Run("within_windows", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t, worldOptions{})
		detached := w.start(w.alex, w.r1, w.h1)
		if err := detached.dev.Close(); err != nil {
			t.Fatal(err)
		}
		w.waitState(detached.id.String(), "detached")
		active := w.start(w.sam, w.r2, w.h2)
		g := w.open(w.robin, w.r1)
		granted := ids.MustParse(w.text(`SELECT token_jti::text FROM terminal_sessions WHERE id = $1`, g.Session.ID)).Time()
		dlStopTo(w, granted.Add(openTimeout-tick))
		if n := dlSweepEnded(w); n != 0 {
			t.Fatalf("the sweep ended %d sessions within their windows", n)
		}
		for id, want := range map[string]string{detached.id.String(): "detached", active.id.String(): "active", g.Session.ID: "opening"} {
			if got := w.state(id); got != want {
				t.Fatalf("session %s: %s, want %s", id, got, want)
			}
		}
		if n := w.count(`SELECT count(*) FROM audit_events WHERE action = 'terminal.terminate'`); n != 0 {
			t.Fatalf("%d entries terminal.terminate", n)
		}
	})

	// An opening whose device dropped and that was attached again is measured from the new grant
	// (the jti of the attach), not from opened_at: it stays after opened_at plus
	// terminal.open_timeout and ends with open_failed open_timeout after the new grant plus it.
	t.Run("reattached_opening", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t, worldOptions{})
		g := w.open(w.alex, w.r1)
		id := ids.MustParse(g.Session.ID)
		d := w.device(w.alex, g.Session.ID)
		if err := d.Sync(w.ctx()); err != nil {
			t.Fatal(err)
		}
		if err := d.Close(); err != nil {
			t.Fatal(err)
		}
		opened := dlTime(w, id, "opened_at")
		// The jump stays within auth.dpop_proof_max_age, so the proofs of the attach (signed with
		// the real clock) are still fresh on the fake one.
		w.jump(openTimeout / 4)
		if r := w.attach(w.alex, g.Session.ID, true); r.Status != http.StatusOK {
			t.Fatalf("attach to the opening: %d %s", r.Status, r.Body)
		}
		if err := w.h1.WaitOpens(w.ctx(), id, 2); err != nil {
			t.Fatal(err)
		}
		granted := ids.MustParse(w.text(`SELECT token_jti::text FROM terminal_sessions WHERE id = $1`, id)).Time()
		if !granted.After(opened) || w.state(g.Session.ID) != "opening" {
			t.Fatalf("the attach: grant %s, opened %s, state %s", granted, opened, w.state(g.Session.ID))
		}
		dlStopTo(w, granted.Add(openTimeout-tick))
		if !w.clk.Now().After(opened.Add(openTimeout)) {
			t.Fatal("the sweep would not run past opened_at plus terminal.open_timeout")
		}
		if n := dlSweepEnded(w); n != 0 || w.state(g.Session.ID) != "opening" {
			t.Fatalf("past opened_at but before the new grant plus terminal.open_timeout: %d ended, %s", n, w.state(g.Session.ID))
		}
		w.jump(tick)
		w.sweep()
		dlEndedRow(w, id, w.h1, nil, "closed:open_failed:open_timeout", domain.CloseOpenFailed)
		dlSweepAgain(w, id)
	})

	// On a host that reports require_local_confirm the open rule does not apply: the opening stays
	// past its grant plus terminal.open_timeout and is bounded by the rules after it (here idle from
	// opened_at).
	t.Run("local_confirm", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t, worldOptions{})
		rep := report(r1Users(), false)
		rep["require_local_confirm"] = true
		if err := w.h1.Send("report", rep); err != nil {
			t.Fatal(err)
		}
		w.eventually("the report with require_local_confirm", func() bool {
			return w.count(`SELECT count(*) FROM hosts WHERE id = $1 AND reported_require_local_confirm`, w.r1) == 1
		})
		g := w.open(w.alex, w.r1)
		id := ids.MustParse(g.Session.ID)
		data, err := w.h1.WaitData(w.ctx(), id)
		if err != nil {
			t.Fatal(err)
		}
		opened := dlTime(w, id, "opened_at")
		granted := ids.MustParse(w.text(`SELECT token_jti::text FROM terminal_sessions WHERE id = $1`, id)).Time()
		dlStopTo(w, granted.Add(2*openTimeout))
		if n := dlSweepEnded(w); n != 0 || w.state(g.Session.ID) != "opening" {
			t.Fatalf("past the grant plus terminal.open_timeout: %d ended, %s", n, w.state(g.Session.ID))
		}
		dlStopTo(w, opened.Add(idleTimeout+idleGrace))
		w.sweep()
		dlEndedRow(w, id, w.h1, data, "closed:idle_timeout", domain.CloseIdleTimeout)
		dlSweepAgain(w, id)
	})
}

// TestTMT11_ProgressPersistence (TM-T11, Design note 7 d and 11): the relay writes bytes_in,
// bytes_out and last_input_at at most once per terminal.idle_grace while bytes flow and when a
// pairing is released, without a new version and without an event; the handshake message of each
// direction of every grant is not counted, every later byte exactly once.
func TestTMT11_ProgressPersistence(t *testing.T) {
	t.Parallel()

	// Periodic writes: the first counted frame is written at once, the next ones only after
	// terminal.idle_grace; the version and the events stay; the terminal.terminate entry of the
	// relay's idle end carries the relay's current counts (also those not written yet).
	t.Run("periodic_and_idle_end", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t, worldOptions{})
		l := w.start(w.alex, w.r1, w.h1)
		in := int64(len("start"))
		out := int64(len("start"))
		w.eventually("the first write", func() bool { return dlCounts(w, l.id) == fmt.Sprintf("%d:0", in) })
		version := w.text(`SELECT version::text FROM terminal_sessions WHERE id = $1`, l.id)
		events := w.count(`SELECT count(*) FROM event_log WHERE resource_id = $1`, l.id)

		a := dlEcho(w, l.dev, dlBytes(100))
		in, out = in+a, out+a
		time.Sleep(dlQuiet)
		if got := dlCounts(w, l.id); got != fmt.Sprintf("%d:0", len("start")) {
			t.Fatalf("written again within terminal.idle_grace: %s", got)
		}
		w.advance(idleGrace - tick)
		b := dlEcho(w, l.dev, dlBytes(50))
		typed := w.clk.Now()
		in, out = in+b, out+b
		time.Sleep(dlQuiet)
		if got := dlCounts(w, l.id); got != fmt.Sprintf("%d:0", len("start")) {
			t.Fatalf("written again one tick before terminal.idle_grace: %s", got)
		}
		// At terminal.idle_grace the flush timer writes the waiting counts without a further
		// frame (D-M2-46-22).
		w.advance(tick)
		w.eventually("the write after terminal.idle_grace", func() bool { return dlCounts(w, l.id) == fmt.Sprintf("%d:%d", in, out) })
		if got := dlTime(w, l.id, "last_input_at"); !got.Equal(typed) {
			t.Fatalf("last_input_at %s, want %s", got, typed)
		}
		c := dlEcho(w, l.dev, dlBytes(25))
		last := w.clk.Now()
		in, out = in+c, out+c
		if got := w.text(`SELECT version::text FROM terminal_sessions WHERE id = $1`, l.id); got != version {
			t.Fatalf("version %s after the progress writes, was %s", got, version)
		}
		if got := w.count(`SELECT count(*) FROM event_log WHERE resource_id = $1`, l.id); got != events {
			t.Fatalf("%d events after the progress writes, were %d", got, events)
		}

		dlAdvanceTo(w, last.Add(idleTimeout+idleGrace))
		w.assertEnded(l, domain.CloseIdleTimeout)
		want := fmt.Sprintf("%d:%d", in, out)
		if got := dlTerminateCounts(w, l.id); got != want {
			t.Fatalf("terminal.terminate counts %s, want %s", got, want)
		}
		if got := dlCounts(w, l.id); got != want {
			t.Fatalf("row counts %s, want %s", got, want)
		}
	})

	// Two pairings: the counts of the first are written when its device leaves; the reattach from
	// the second device starts a new handshake (not counted); the end carries the sum of both
	// pairings in the row and in terminal.terminate, each byte once.
	t.Run("sum_of_pairings", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t, worldOptions{})
		l := w.start(w.alex, w.r1, w.h1)
		in := int64(len("start"))
		out := int64(len("start"))
		a := dlEcho(w, l.dev, dlBytes(100))
		in, out = in+a, out+a
		if err := l.dev.Close(); err != nil {
			t.Fatal(err)
		}
		w.waitState(l.id.String(), "detached")
		w.eventually("the write at the release", func() bool { return dlCounts(w, l.id) == fmt.Sprintf("%d:%d", in, out) })

		l2 := dlReattach(w, l)
		in, out = in+int64(len("start")), out+int64(len("start"))
		c := dlEcho(w, l2.dev, dlBytes(30))
		in, out = in+c, out+c
		// Output of the host alone, so in and out differ.
		extra := dlBytes(7)
		if err := l2.data.Send(w.ctx(), extra); err != nil {
			t.Fatal(err)
		}
		if m, err := l2.dev.Recv(w.ctx()); err != nil || !fakehost.Equal(m, extra) {
			t.Fatalf("host output: %q %v", m, err)
		}
		out += int64(len(extra))

		w.advance(idleTimeout + idleGrace)
		w.assertEnded(l2, domain.CloseIdleTimeout)
		want := fmt.Sprintf("%d:%d", in, out)
		if got := dlTerminateCounts(w, l.id); got != want {
			t.Fatalf("terminal.terminate counts %s, want %s", got, want)
		}
		if got := dlCounts(w, l.id); got != want {
			t.Fatalf("row counts %s, want %s", got, want)
		}
	})
}
