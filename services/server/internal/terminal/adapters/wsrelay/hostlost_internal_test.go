// SPDX-License-Identifier: Apache-2.0

package wsrelay

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/terminal/app"
	"github.com/OWNER/bothy/services/server/internal/terminal/domain"
	"github.com/OWNER/bothy/services/server/test/terminal/fakehost"
)

// TestHostLost_EndedSessionClosesDeviceWithItsEnd (24 13, found by the conformance suite of
// M2-48): when the host side of a started session ends by itself, the device closes with 4102
// while the session is open; when the session already ended elsewhere (another node, a
// revocation, a host that read 4110 to 4112), whose terminal_session.changed has not reached this
// node, the device closes with 4100 and the close reason instead of 4102.
func TestHostLost_EndedSessionClosesDeviceWithItsEnd(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name   string
		ended  *domain.End
		code   int
		reason string
	}{
		{"session open", nil, 4102, ""},
		{"session ended elsewhere", &domain.End{Reason: domain.CloseClientClosed}, 4100, "client_closed"},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			uc := &fakeRelayUC{ended: c.ended}
			r, err := NewRelay(RelayConfig{BufferMaxBytes: 1 << 16, OpenTimeout: time.Minute}, uc, nil, manualTimers{}, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			session, host, device := ids.New(), ids.New(), ids.New()
			r.Expect(app.Expectation{SessionID: session, HostID: host, DeviceID: device, Deadline: time.Now().Add(time.Minute), Started: true})
			srv := relayServer(t, r, session, host, device)
			ctx := testCtx(t)
			_, devCode := devReason(t, ctx, srv)
			hst, _ := dial(t, ctx, srv, "/host")
			for paired := false; !paired; {
				r.mu.Lock()
				paired = r.slots[session] != nil && r.slots[session].paired
				r.mu.Unlock()
				if ctx.Err() != nil {
					t.Fatal("the sides never paired")
				}
				time.Sleep(5 * time.Millisecond)
			}
			if err := hst.Close(websocket.StatusNormalClosure, ""); err != nil {
				t.Fatal(err)
			}
			select {
			case got := <-devCode:
				if got.code != c.code || got.reason != c.reason {
					t.Errorf("the device closed with %d %q, want %d %q", got.code, got.reason, c.code, c.reason)
				}
			case <-ctx.Done():
				t.Fatal("the device was not closed")
			}
		})
	}
}

// closeOf is a close code with its reason.
type closeOf struct {
	code   int
	reason string
}

// devReason dials the device side of srv and reports its close code and reason.
func devReason(t *testing.T, ctx context.Context, srv *httptest.Server) (*websocket.Conn, chan closeOf) {
	t.Helper()
	ws, _, err := fakehost.Dial(ctx, "ws"+strings.TrimPrefix(srv.URL, "http")+"/dev")
	if err != nil {
		t.Fatal(err)
	}
	codes := make(chan closeOf, 1)
	go func() { //nolint:gosec // G118: the reader lives as long as the connection, not as ctx
		for {
			if _, _, err := ws.Read(context.Background()); err != nil {
				var ce websocket.CloseError
				if errors.As(err, &ce) {
					codes <- closeOf{int(ce.Code), ce.Reason}
				} else {
					codes <- closeOf{}
				}
				return
			}
		}
	}()
	return ws, codes
}
