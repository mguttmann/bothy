// SPDX-License-Identifier: Apache-2.0

package wsrelay

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
	"github.com/OWNER/bothy/services/server/internal/terminal/app"
)

// hostServer serves /dev as the device and /host?h=<id> as the host side of session on r, the
// host side logged in as the host of the query.
func hostServer(t *testing.T, r *Relay, session, device ids.ID) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		ws, err := websocket.Accept(w, req, &websocket.AcceptOptions{CompressionMode: websocket.CompressionDisabled})
		if err != nil {
			return
		}
		ws.SetReadLimit(-1)
		var sd *side
		var code int
		if req.URL.Path == "/dev" {
			sd, code = r.attachDevice(session, device, ws)
		} else {
			sd, code = r.attachHost(session, ids.MustParse(req.URL.Query().Get("h")), ws)
		}
		if sd == nil {
			_ = ws.Close(websocket.StatusCode(code), "")
			return
		}
		runSide(context.Background(), r, sd)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// closeOfSide waits for the close code on ch.
func closeOfSide(t *testing.T, ctx context.Context, what string, ch chan int, want int) {
	t.Helper()
	select {
	case c := <-ch:
		if c != want {
			t.Fatalf("%s closed with %d, want %d", what, c, want)
		}
	case <-ctx.Done():
		t.Fatalf("%s not closed", what)
	}
}

// TestHostData_OnlyAfterSessionOpen (Design note 9): a data connection for a session without an
// expectation on this node gets 1000 (the host service waits for session.open); one of another
// host 4100; a second unpaired host side replaces the first (1000 for the first); a host side
// while one is paired gets 1000. The closed session (4100) and the bad proof (4113) are answered
// before the relay, by DataSession and M2-43's login.
func TestHostData_OnlyAfterSessionOpen(t *testing.T) {
	t.Parallel()
	r, err := NewRelay(RelayConfig{BufferMaxBytes: 1 << 16, OpenTimeout: time.Minute}, &fakeRelayUC{}, nil, manualTimers{}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	session, host, other, device := ids.New(), ids.New(), ids.New(), ids.New()
	srv := hostServer(t, r, session, device)
	ctx := testkit.Context(t)
	hostPath := func(h ids.ID) string { return "/host?h=" + h.String() }

	_, noSlot := dial(t, ctx, srv, hostPath(host))
	closeOfSide(t, ctx, "host side without an expectation", noSlot, 1000)

	r.Expect(app.Expectation{SessionID: session, HostID: host, DeviceID: device, Deadline: time.Now().Add(time.Minute)})
	_, foreign := dial(t, ctx, srv, hostPath(other))
	closeOfSide(t, ctx, "host side of another host", foreign, 4100)

	_, first := dial(t, ctx, srv, hostPath(host))
	waitSlot(t, ctx, r, session, func(s *slot) bool { return s.hst != nil })
	_, second := dial(t, ctx, srv, hostPath(host))
	closeOfSide(t, ctx, "replaced unpaired host side", first, 1000)

	dev, _ := dial(t, ctx, srv, "/dev")
	if err := dev.Write(ctx, websocket.MessageBinary, []byte("m1")); err != nil {
		t.Fatal(err)
	}
	waitSlot(t, ctx, r, session, func(s *slot) bool { return s.paired })
	_, third := dial(t, ctx, srv, hostPath(host))
	closeOfSide(t, ctx, "host side while one is paired", third, 1000)
	select {
	case c := <-second:
		t.Fatalf("the paired host side closed with %d", c)
	default:
	}
}

// waitSlot waits until the slot of session satisfies cond.
func waitSlot(t *testing.T, ctx context.Context, r *Relay, session ids.ID, cond func(*slot) bool) {
	t.Helper()
	for {
		r.mu.Lock()
		s := r.slots[session]
		ok := s != nil && cond(s)
		r.mu.Unlock()
		if ok {
			return
		}
		if ctx.Err() != nil {
			t.Fatal("the slot never reached the expected state")
		}
		time.Sleep(5 * time.Millisecond)
	}
}
