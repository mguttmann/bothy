// SPDX-License-Identifier: Apache-2.0

package terminalsessions

import (
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/test/terminal/fakehost"
)

// TestHostHelloCarriesValues (M2-45 Design note 2a): hello on the control connection and on a data
// connection validates against HostChannelHello (hosttest and the Fake-Host check every frame
// through api/specschema) and carries clock_leeway_ms, idle_timeout_ms and detach_grace_ms equal
// to auth.clock_leeway, terminal.idle_timeout and terminal.detach_grace of the configuration.
func TestHostHelloCarriesValues(t *testing.T) {
	t.Parallel()
	w := newWorld(t, map[string]string{"auth.clock_leeway": "45s", "terminal.idle_timeout": "41m", "terminal.detach_grace": "9m"})
	const leeway, idle, detach = int64(45_000), int64(41 * 60_000), int64(9 * 60_000)
	c := w.h1.Control.Hello()
	if c.ClockLeewayMs != leeway || c.IdleTimeoutMs != idle || c.DetachGraceMs != detach {
		t.Fatalf("control hello: leeway %d, idle %d, detach %d", c.ClockLeewayMs, c.IdleTimeoutMs, c.DetachGraceMs)
	}
	w.h1.Default(fakehost.Script{Kind: fakehost.Pair})
	g := w.opened(w.alex, w.r1, openBody("shell"))
	dc, err := w.h1.WaitData(w.ctx(), ids.MustParse(g.Session.ID))
	if err != nil {
		t.Fatal(err)
	}
	if h := dc.Hello; h.ClockLeewayMs != leeway || h.IdleTimeoutMs != idle || h.DetachGraceMs != detach {
		t.Fatalf("data hello: leeway %d, idle %d, detach %d", h.ClockLeewayMs, h.IdleTimeoutMs, h.DetachGraceMs)
	}
}

// TestRelay_HostEndCloseOrder (M2-45 Design note 6): the host service reports a failure after
// message 2 with session.ended and keeps the data connection open; the server ends the session and
// closes both sides through its end path: open_failed with handshake_failed gives the device 4104
// and the host data connection 4100; open_failed with local_confirm_denied gives the device 4100;
// process_exited of a running session gives the device 4100 with process_exited. The close reason
// of both sides is the close_reason (asyncapi terminalStream 4100); the detail stays in the row.
func TestRelay_HostEndCloseOrder(t *testing.T) {
	t.Parallel()
	w := newWorld(t, nil)
	ctx := w.ctx()
	for _, c := range []struct {
		detail string
		device int
	}{
		{"handshake_failed", 4104},
		{"local_confirm_denied", 4100},
	} {
		w.h1.Default(fakehost.Script{Kind: fakehost.EndAfterHandshake, Reason: "open_failed", Detail: c.detail})
		g := w.opened(w.alex, w.r1, openBody("shell"))
		d := w.device(w.alex, g.Session.ID, fakehost.DeviceParams{})
		h, err := w.h1.WaitData(ctx, ids.MustParse(g.Session.ID))
		if err != nil {
			t.Fatal(err)
		}
		if err := d.Send(ctx, []byte(g.SessionToken)); err != nil {
			t.Fatal(err)
		}
		if m, err := d.Recv(ctx); err != nil || string(m) != "message-2" {
			t.Fatalf("%s: message 2: %q %v", c.detail, m, err)
		}
		if err := d.Send(ctx, []byte("start")); err != nil {
			t.Fatal(err)
		}
		w.closeCode(c.detail+", device", func() (int, string) { return d.CloseCode(ctx) }, c.device, "open_failed")
		w.closeCode(c.detail+", host", func() (int, string) { return h.CloseCode(ctx) }, 4100, "open_failed")
		w.waitState(g.Session.ID, "closed:open_failed:"+c.detail)
	}
	g, d := w.started(w.alex)
	h := w.h1.Data(ids.MustParse(g.Session.ID))
	if err := w.h1.Send("session.ended", map[string]any{"session_id": g.Session.ID, "close_reason": "process_exited", "exit_code": 0}); err != nil {
		t.Fatal(err)
	}
	w.closeCode("process exited, device", func() (int, string) { return d.CloseCode(ctx) }, 4100, "process_exited")
	w.closeCode("process exited, host", func() (int, string) { return h.CloseCode(ctx) }, 4100, "process_exited")
	w.waitState(g.Session.ID, "closed:process_exited")
}
