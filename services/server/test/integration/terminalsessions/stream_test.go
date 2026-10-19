// SPDX-License-Identifier: Apache-2.0

package terminalsessions

import (
	"encoding/base64"
	"net/http"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/test/terminal/fakehost"
)

// TestStream_IdentifyDecisions (Design note 8) with TestTerminalOps_NonHumanTokens on the stream:
// every row of the close code table; the denials have deny entries.
func TestStream_IdentifyDecisions(t *testing.T) {
	w := newWorld(t, nil)
	ctx := w.ctx()
	w.h1.Default(fakehost.Script{Kind: fakehost.Ignore})
	g := w.opened(w.alex, w.r1, openBody("shell"))
	sid := g.Session.ID
	cases := []struct {
		name string
		c    func() *fakehost.Device
		code int
	}{
		{"bad proof", func() *fakehost.Device { return w.device(w.alex, sid, fakehost.DeviceParams{BadProof: true}) }, 4004},
		{"wrong nonce", func() *fakehost.Device { return w.device(w.alex, sid, fakehost.DeviceParams{Nonce: "bm9uY2U"}) }, 4004},
		{"wrong htu", func() *fakehost.Device {
			return w.device(w.alex, sid, fakehost.DeviceParams{HTU: "https://chat.example.com/api/v1/terminal/sessions/" + sid + "/stream"})
		}, 4004},
		{"no token", func() *fakehost.Device { return w.device(w.alex, sid, fakehost.DeviceParams{NoToken: true}) }, 4004},
		{"device_id of D2 with D1's token", func() *fakehost.Device {
			return w.device(w.alex, sid, fakehost.DeviceParams{DeviceID: w.alex2.DeviceID})
		}, 4004},
		{"agent token", func() *fakehost.Device {
			return w.device(w.lotse, sid, fakehost.DeviceParams{DeviceID: ids.New().String()})
		}, 4004},
		{"the other device", func() *fakehost.Device { return w.device(w.alex2, sid, fakehost.DeviceParams{}) }, 4103},
		{"Sam on Alex's session", func() *fakehost.Device { return w.device(w.sam, sid, fakehost.DeviceParams{}) }, 4100},
		{"unknown session", func() *fakehost.Device {
			d, _, err := fakehost.DialDevice(t, ctx, w.s.PublicURL, w.s.PublicURL, ids.New(), w.alex, fakehost.DeviceParams{DeviceID: w.alex.DeviceID})
			if err != nil {
				t.Fatal(err)
			}
			return d
		}, 4100},
		{"protocol version 2", func() *fakehost.Device { return w.device(w.alex, sid, fakehost.DeviceParams{ProtocolVersion: 2}) }, 4002},
		{"binary first", func() *fakehost.Device { return w.device(w.alex, sid, fakehost.DeviceParams{BinaryFirst: true}) }, 4002},
		{"unknown member", func() *fakehost.Device {
			return w.device(w.alex, sid, fakehost.DeviceParams{Extra: map[string]any{"cursor": 1}})
		}, 4002},
	}
	for _, c := range cases {
		d := c.c()
		w.closeCode(c.name, func() (int, string) { return d.CloseCode(ctx) }, c.code, "")
	}
	w.waitDenial("terminal.stream", "not_self")
	w.waitDenial("terminal.stream", "subject_kind_not_allowed")
	// An own closed session: 4100 with its close_reason.
	w.must(w.alex, http.MethodPost, "/terminal/sessions/"+sid+"/close", map[string]any{"reason": "done"}, http.StatusOK)
	closed := w.device(w.alex, sid, fakehost.DeviceParams{})
	w.closeCode("closed", func() (int, string) { return closed.CloseCode(ctx) }, 4100, "client_closed")
	// A session of S1 after the logout of S1: 4012.
	g2 := w.opened(w.alex, w.r1, openBody("shell"))
	w.must(w.alex, http.MethodPost, "/auth/logout", nil, http.StatusNoContent)
	revoked := w.device(w.alex, g2.Session.ID, fakehost.DeviceParams{})
	w.closeCode("logout", func() (int, string) { return revoked.CloseCode(ctx) }, 4012, "")
	// No identify before terminal.open_timeout: 4004.
	silent := w.device(w.alex2, g2.Session.ID, fakehost.DeviceParams{Silent: true})
	if err := silent.Sync(ctx); err != nil {
		t.Fatal(err)
	}
	w.timers.Fire()
	w.closeCode("silent", func() (int, string) { return silent.CloseCode(ctx) }, 4004, "")
}

// TestStream_ForeignOriginRejected (SEC-CORS-3, part of SEC-T04): a foreign Origin gets 403 before
// the upgrade on both paths; the instance origin and no Origin are accepted.
func TestStream_ForeignOriginRejected(t *testing.T) {
	w := newWorld(t, nil)
	ctx := w.ctx()
	sid := ids.New()
	for _, path := range []string{"/api/v1/terminal/sessions/" + sid.String() + "/stream"} {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, w.s.PublicURL+path, nil)
		req.Header.Set("Origin", "https://evil.example.com")
		req.Header.Set("Connection", "Upgrade")
		req.Header.Set("Upgrade", "websocket")
		req.Header.Set("Sec-WebSocket-Version", "13")
		req.Header.Set("Sec-WebSocket-Key", base64.StdEncoding.EncodeToString(make([]byte, 16)))
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusForbidden {
			t.Fatalf("%s with a foreign origin: %d", path, resp.StatusCode)
		}
	}
	d, status, err := fakehost.DialDevice(t, ctx, w.s.PublicURL, w.s.PublicURL, sid, w.alex, fakehost.DeviceParams{DeviceID: w.alex.DeviceID, Origin: w.s.PublicURL})
	if err != nil {
		t.Fatalf("own origin: %d %v", status, err)
	}
	w.closeCode("own origin, unknown session", func() (int, string) { return d.CloseCode(ctx) }, 4100, "")
	// The data connection: the library's same-host check refuses a foreign Origin.
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, w.s.PublicURL+"/api/v1/hosts/sessions/"+sid.String()+"/stream", nil)
	req.Header.Set("Origin", "https://evil.example.com")
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Sec-WebSocket-Version", "13")
	req.Header.Set("Sec-WebSocket-Key", base64.StdEncoding.EncodeToString(make([]byte, 16)))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("data connection with a foreign origin: %d", resp.StatusCode)
	}
}
