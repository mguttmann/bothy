// SPDX-License-Identifier: Apache-2.0

package terminalsessions

import (
	"bytes"
	"crypto/rand"
	"net/http"
	"strconv"
	"strings"
	gosync "sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
	"github.com/OWNER/bothy/services/server/test/harness"
	"github.com/OWNER/bothy/services/server/test/terminal/fakehost"
)

// syncBuffer is a bytes.Buffer safe for the concurrent writes of the server's log.
type syncBuffer struct {
	mu gosync.Mutex
	b  bytes.Buffer
}

// Write implements io.Writer.
func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

// String returns the text written so far.
func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

// started opens a shell of c on R1 with H1 on Accept, connects the device and runs the handshake;
// the session is active.
func (w *world) started(c *harness.Client) (grantBody, *fakehost.Device) {
	w.t.Helper()
	w.h1.Default(fakehost.Script{Kind: fakehost.Accept})
	g := w.opened(c, w.r1, openBody("shell"))
	d := w.device(c, g.Session.ID, fakehost.DeviceParams{})
	w.handshake(d, g.SessionToken)
	w.waitState(g.Session.ID, "active")
	return g, d
}

// closeCode waits for the close of d and checks code and reason ("" for any reason).
func (w *world) closeCode(what string, got func() (int, string), code int, reason string) {
	w.t.Helper()
	c, r := got()
	if c != code || reason != "" && r != reason {
		w.t.Fatalf("%s: close %d %q, want %d %q", what, c, r, code, reason)
	}
}

// TestRelay_ForwardsBytesUnchanged (SEC-TERM-1, T-TM-01) with TestRelay_ByteCountersAndLastInput
// and TestRelay_CanaryNeverLeaks: frames of 1, 2, 1000, 65534 and 65535 random bytes arrive
// byte-identical in both directions, one message per message, in order; after the device leaves
// the session is detached and the row carries the transport bytes without the handshake; a canary
// and the token appear in no audit entry, no event and no log line.
func TestRelay_ForwardsBytesUnchanged(t *testing.T) {
	t.Parallel()
	logs := &syncBuffer{}
	w := newWorldLogs(t, nil, logs)
	g, d := w.started(w.alex)
	ctx := w.ctx()
	sum := int64(len("start"))
	canary := []byte("CANARY-terminal-content-7f3a")
	for _, n := range []int{1, 2, 1000, 65534, 65535} {
		m := make([]byte, n)
		_, _ = rand.Read(m)
		if n == 1000 {
			copy(m, canary)
		}
		if err := d.Send(ctx, m); err != nil {
			t.Fatal(err)
		}
		back, err := d.Recv(ctx)
		if err != nil || !bytes.Equal(back, m) {
			t.Fatalf("%d bytes: echo differs (%d bytes, %v)", n, len(back), err)
		}
		sum += int64(n)
	}
	_ = d.Close()
	w.waitState(g.Session.ID, "detached")
	w.eventually("the counters", func() bool {
		return w.text(`SELECT bytes_in || ':' || bytes_out FROM terminal_sessions WHERE id = $1`, g.Session.ID) == strconv.FormatInt(sum, 10)+":"+strconv.FormatInt(sum, 10)
	})
	if w.count(`SELECT count(*) FROM terminal_sessions WHERE id = $1 AND last_input_at IS NOT NULL`, g.Session.ID) != 1 {
		t.Fatal("last_input_at not written")
	}
	for _, secret := range []string{string(canary), g.SessionToken} {
		for _, q := range []string{
			`SELECT count(*) FROM audit_events WHERE details::text LIKE '%' || $1 || '%'`,
			`SELECT count(*) FROM event_log WHERE payload::text LIKE '%' || $1 || '%'`,
		} {
			if n := w.count(q, secret); n != 0 {
				t.Errorf("a secret in %s", q)
			}
		}
		if strings.Contains(logs.String(), secret) {
			t.Error("a secret in the log")
		}
	}
	if !strings.Contains(logs.String(), "terminal relay connection closed") {
		t.Error("no relay log record")
	}
}

// TestRelay_TextFrameAfterIdentify4002 and TestRelay_FrameOverNoiseLimit4002: a text frame or a
// binary message of 65536 bytes from the device gives it 4002 and the host side 1000; from the
// host the host side 4002 and the device 4102.
func TestRelay_TextFrameAfterIdentify4002(t *testing.T) {
	t.Parallel()
	w := newWorld(t, nil)
	ctx := w.ctx()
	g, d := w.started(w.alex)
	h, _ := w.h1.WaitData(ctx, ids.MustParse(g.Session.ID))
	if err := d.SendText(ctx, "hello"); err != nil {
		t.Fatal(err)
	}
	w.closeCode("device text", func() (int, string) { return d.CloseCode(ctx) }, 4002, "")
	w.closeCode("host after device text", func() (int, string) { return h.CloseCode(ctx) }, 1000, "")
	w.waitState(g.Session.ID, "detached")

	// Reattach (quick) and let the host send a message above the Noise limit.
	r := w.attach(w.alex, g.Session.ID, false)
	if r.Status != http.StatusOK {
		t.Fatalf("attach: %d %s", r.Status, r.Body)
	}
	var g2 grantBody
	r.JSON(t, &g2)
	d2 := w.device(w.alex, g.Session.ID, fakehost.DeviceParams{})
	w.handshake(d2, g2.SessionToken)
	h2 := w.h1.Data(ids.MustParse(g.Session.ID))
	if err := h2.Send(ctx, make([]byte, 65536)); err != nil {
		t.Fatal(err)
	}
	w.closeCode("host over the limit", func() (int, string) { return h2.CloseCode(ctx) }, 4002, "")
	w.closeCode("device after host over the limit", func() (int, string) { return d2.CloseCode(ctx) }, 4102, "")
}

// TestRelay_SlowDevice4014: with a small relay buffer the device that does not read is closed
// as the slow end (4014, when its socket still takes the close frame), the host side gets 1000 and
// the session is detached.
func TestRelay_SlowDevice4014(t *testing.T) {
	t.Parallel()
	w := newWorld(t, map[string]string{"terminal.relay_buffer_max_bytes": "4096"})
	ctx := w.ctx()
	g, d := w.started(w.alex)
	h := w.h1.Data(ids.MustParse(g.Session.ID))
	d.Pause()
	// The host floods; the device never reads, so the queue toward it fills.
	go func() {
		for i := 0; i < 4000; i++ {
			if h.Send(ctx, make([]byte, 60000)) != nil {
				return
			}
		}
	}()
	w.closeCode("host of a slow device", func() (int, string) { return h.CloseCode(ctx) }, 1000, "")
	w.waitState(g.Session.ID, "detached")
	d.Resume()
	if code, _ := d.CloseCode(ctx); code != 4014 && code != 0 {
		t.Fatalf("slow device closed with %d, want 4014 (or no close frame when its socket was full)", code)
	}
}

// TestRelay_FullBufferHoldsTheHostBack (PL/40, 24 6.3): with the default relay buffer a device
// that stops reading is not closed while the host floods (as a reattach replays its scrollback at
// once): the relay stops reading from the host and starts its stall timer (which never fires
// here), and once the device reads again every message arrives in order and no side closes.
func TestRelay_FullBufferHoldsTheHostBack(t *testing.T) {
	t.Parallel()
	w := newWorld(t, nil)
	ctx := w.ctx()
	g, d := w.started(w.alex)
	h := w.h1.Data(ids.MustParse(g.Session.ID))
	stall, err := time.ParseDuration(w.s.ConfigValue("terminal.relay_stall_timeout"))
	if err != nil {
		t.Fatal(err)
	}
	// More than the relay buffer plus every socket buffer between host, relay and device.
	const msgs, size = 600, 60000
	// The relay waits for room once it starts a stall timer after the mark. Comparing the count of
	// pending timers missed it: the deadline of the opening may stop after the mark (the relay
	// meets session.started after the row is active), and on a host with large socket buffers the
	// stall timer stops again within milliseconds (PL/47).
	mark := w.timers.Mark()
	d.Pause()
	sent := make(chan error, 1)
	go func() {
		for i := range msgs {
			m := make([]byte, size)
			m[0], m[1] = byte(i>>8), byte(i)
			if err := h.Send(ctx, m); err != nil {
				sent <- err
				return
			}
		}
		sent <- nil
	}()
	waited := func() bool { return w.timers.StartedSince(mark, stall) > 0 }
	testkit.Until(t, "the relay waits for room toward the device", func() bool { return waited() || len(sent) > 0 })
	if !waited() {
		t.Fatalf("the host's sends ended before the relay waited for room: %v", <-sent)
	}
	d.Resume()
	for i := range msgs {
		m, err := d.Recv(ctx)
		if err != nil {
			t.Fatalf("message %d: %v (the device was closed)", i, err)
		}
		if len(m) != size || int(m[0])<<8|int(m[1]) != i {
			t.Fatalf("message %d: %d bytes, number %d", i, len(m), int(m[0])<<8|int(m[1]))
		}
	}
	if err := testkit.Receive(t, "the host's last send", sent); err != nil {
		t.Fatalf("host send: %v", err)
	}
	w.waitState(g.Session.ID, "active")
}

// TestRelay_OpenTimeoutHostMissing, TestRelay_OpenTimeoutDeviceMissing and
// TestRelay_OpenTimeoutNoStart: at the deadline an opening without a host side gives the device
// 4102 and ends open_failed open_timeout; without a device the host side gets 4102 (M2-47); with both
// paired and no start both get 4100.
func TestRelay_OpenTimeout(t *testing.T) {
	t.Parallel()
	w := newWorld(t, nil)
	ctx := w.ctx()
	w.h1.Default(fakehost.Script{Kind: fakehost.Ignore})
	g := w.opened(w.alex, w.r1, openBody("shell"))
	d := w.device(w.alex, g.Session.ID, fakehost.DeviceParams{})
	if err := d.Sync(ctx); err != nil {
		t.Fatal(err)
	}
	w.timers.Fire()
	w.closeCode("host missing", func() (int, string) { return d.CloseCode(ctx) }, 4102, "")
	w.waitState(g.Session.ID, "closed:open_failed:open_timeout")
	if len(w.audits("terminal.terminate")) == 0 {
		t.Fatal("no terminal.terminate entry")
	}

	w.h1.Default(fakehost.Script{Kind: fakehost.Pair})
	g2 := w.opened(w.alex, w.r1, openBody("shell"))
	h, err := w.h1.WaitData(ctx, ids.MustParse(g2.Session.ID))
	if err != nil {
		t.Fatal(err)
	}
	if err := h.Sync(ctx); err != nil {
		t.Fatal(err)
	}
	w.timers.Fire()
	// A lone host data connection of an opening closes with 4102 (M2-47 Design note 8 b, ruling
	// D-M2-47-14; M2-44 closed it with 4100 and open_failed).
	w.closeCode("device missing", func() (int, string) { return h.CloseCode(ctx) }, 4102, "")
	w.waitState(g2.Session.ID, "closed:open_failed:open_timeout")

	g3 := w.opened(w.alex, w.r1, openBody("shell"))
	d3 := w.device(w.alex, g3.Session.ID, fakehost.DeviceParams{})
	h3, _ := w.h1.WaitData(ctx, ids.MustParse(g3.Session.ID))
	if err := d3.Send(ctx, []byte(g3.SessionToken)); err != nil {
		t.Fatal(err)
	}
	if _, err := d3.Recv(ctx); err != nil {
		t.Fatal(err)
	}
	w.timers.Fire()
	w.closeCode("paired without start, device", func() (int, string) { return d3.CloseCode(ctx) }, 4100, "open_failed")
	w.closeCode("paired without start, host", func() (int, string) { return h3.CloseCode(ctx) }, 4100, "open_failed")
	w.waitState(g3.Session.ID, "closed:open_failed:open_timeout")
}

// TestRelay_LocalConfirmHostDeadline: on a host that reports require_local_confirm the pairing
// meets the deadline: a session.started after it makes the session active.
func TestRelay_LocalConfirmHostDeadline(t *testing.T) {
	t.Parallel()
	w := newWorld(t, nil)
	ctx := w.ctx()
	if err := w.h1.Send("report", report(reportedUsers(), true)); err != nil {
		t.Fatal(err)
	}
	w.eventually("the report", func() bool {
		return w.count(`SELECT count(*) FROM hosts WHERE id = $1 AND reported_require_local_confirm`, w.r1.String()) == 1
	})
	w.h1.Default(fakehost.Script{Kind: fakehost.Pair})
	g := w.opened(w.alex, w.r1, openBody("shell"))
	d := w.device(w.alex, g.Session.ID, fakehost.DeviceParams{})
	if err := d.Send(ctx, []byte(g.SessionToken)); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Recv(ctx); err != nil {
		t.Fatal(err)
	}
	w.timers.Fire() // the deadline passes: met by the pairing
	if err := w.h1.Send("session.started", map[string]any{"session_id": g.Session.ID}); err != nil {
		t.Fatal(err)
	}
	w.waitState(g.Session.ID, "active")
	if d.Ended() {
		t.Fatal("the device stream ended")
	}
}

// TestRelay_HostRefusalCodes (T-TM-02): at opening H1 refuses with 4101 token_replayed: the device
// gets 4101 with that reason and the session ends open_failed token_replayed; with 4104
// profile_unknown the device gets 4104; with End(open_failed, token_replayed) the device gets 4101
// as well.
func TestRelay_HostRefusalCodes(t *testing.T) {
	t.Parallel()
	w := newWorld(t, nil)
	ctx := w.ctx()
	for _, c := range []struct {
		script fakehost.Script
		code   int
		state  string
	}{
		{fakehost.Script{Kind: fakehost.Refuse, Code: 4101, Detail: "token_replayed"}, 4101, "closed:open_failed:token_replayed"},
		{fakehost.Script{Kind: fakehost.Refuse, Code: 4104, Detail: "profile_unknown"}, 4104, "closed:open_failed:profile_unknown"},
	} {
		w.h1.Default(c.script)
		g := w.opened(w.alex, w.r1, openBody("shell"))
		d := w.device(w.alex, g.Session.ID, fakehost.DeviceParams{})
		if err := d.Send(ctx, []byte(g.SessionToken)); err != nil {
			t.Fatal(err)
		}
		w.closeCode(c.script.Detail, func() (int, string) { return d.CloseCode(ctx) }, c.code, c.script.Detail)
		w.waitState(g.Session.ID, c.state)
	}
	// session.ended instead of a refused data connection: the device gets the same code.
	w.h1.Default(fakehost.Script{Kind: fakehost.Ignore})
	g := w.opened(w.alex, w.r1, openBody("shell"))
	d := w.device(w.alex, g.Session.ID, fakehost.DeviceParams{})
	if err := d.Sync(ctx); err != nil {
		t.Fatal(err)
	}
	if err := w.h1.Send("session.ended", map[string]any{"session_id": g.Session.ID, "close_reason": "open_failed", "close_detail": "token_replayed"}); err != nil {
		t.Fatal(err)
	}
	w.closeCode("session.ended token_replayed", func() (int, string) { return d.CloseCode(ctx) }, 4101, "open_failed")
}

// TestRelay_EndsFromOtherPaths (T-TM-11): with a device connected, session.ended process_exited
// ends the stream with 4100 process_exited; a disable of R1 ends another with host_disabled; H1
// gets session.close where it holds the link.
func TestRelay_EndsFromOtherPaths(t *testing.T) {
	t.Parallel()
	w := newWorld(t, nil)
	ctx := w.ctx()
	g, d := w.started(w.alex)
	if err := w.h1.Send("session.ended", map[string]any{"session_id": g.Session.ID, "close_reason": "process_exited", "exit_code": 3}); err != nil {
		t.Fatal(err)
	}
	w.closeCode("process exited", func() (int, string) { return d.CloseCode(ctx) }, 4100, "process_exited")
	if w.count(`SELECT count(*) FROM terminal_sessions WHERE id = $1 AND exit_code = 3`, g.Session.ID) != 1 {
		t.Fatal("exit_code not stored")
	}
	g2, d2 := w.started(w.alex)
	get := w.must(w.robin, http.MethodGet, "/hosts/"+w.r1.String(), nil, http.StatusOK)
	resp, err := harness.DoWithStepUp(t.Context(), w.robin, harness.Request{Method: http.MethodPost, Path: "/hosts/" + w.r1.String() + "/state",
		Header: http.Header{"If-Match": []string{get.Header.Get("ETag")}}, Body: map[string]any{"state": "disabled", "reason": "maintenance"}})
	if err != nil || resp.Status != http.StatusOK {
		t.Fatalf("setHostState: %v %d %s", err, resp.Status, resp.Body)
	}
	w.closeCode("host disabled", func() (int, string) { return d2.CloseCode(ctx) }, 4100, "host_disabled")
	w.waitState(g2.Session.ID, "closed:host_disabled")
}

// TestControlLoss_ClosesStreams: H1's control link drops while a session is active: the device
// gets 4102 and the session is detached.
func TestControlLoss_ClosesStreams(t *testing.T) {
	t.Parallel()
	w := newWorld(t, nil)
	ctx := w.ctx()
	g, d := w.started(w.alex)
	w.h1.Drop()
	w.closeCode("control lost", func() (int, string) { return d.CloseCode(ctx) }, 4102, "")
	w.waitState(g.Session.ID, "detached")
}
