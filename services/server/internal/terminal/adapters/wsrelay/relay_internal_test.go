// SPDX-License-Identifier: Apache-2.0

package wsrelay

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	gosync "sync"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/terminal/app"
	"github.com/OWNER/bothy/services/server/internal/terminal/domain"
	"github.com/OWNER/bothy/services/server/test/terminal/fakehost"
)

// fakeRelayUC records the callbacks of the relay.
type fakeRelayUC struct {
	mu    gosync.Mutex
	calls []string
	// state is the session state RelaySessionState answers ("" means opening).
	state string
}

func (f *fakeRelayUC) add(s string) {
	f.mu.Lock()
	f.calls = append(f.calls, s)
	f.mu.Unlock()
}

func (f *fakeRelayUC) RelayActivate(context.Context, ids.ID) error { f.add("activate"); return nil }
func (f *fakeRelayUC) RelayDetach(context.Context, ids.ID) error   { f.add("detach"); return nil }
func (f *fakeRelayUC) RelayOpenFailed(_ context.Context, _ ids.ID, d domain.CloseDetail) error {
	f.add("open_failed:" + string(d))
	return nil
}
func (f *fakeRelayUC) RelayCounters(_ context.Context, _ ids.ID, c app.Counts) error {
	f.add("counters")
	return nil
}
func (f *fakeRelayUC) RelayUnpaired(_ context.Context, s domain.Side) { f.add("unpaired:" + string(s)) }
func (f *fakeRelayUC) RelaySessionState(context.Context, ids.ID) (string, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.state == "" {
		return domain.SessionOpening, true, nil
	}
	return f.state, true, nil
}
func (f *fakeRelayUC) RelayOpeningTimedOut(context.Context, ids.ID) error {
	f.add("open_failed:" + string(domain.DetailOpenTimeout))
	return nil
}

// manualTimers never fires on its own.
type manualTimers struct{}

func (manualTimers) AfterFunc(time.Duration, func()) func() bool { return func() bool { return true } }

// relayServer serves /dev and /host of one session on r.
func relayServer(t *testing.T, r *Relay, session, host, device ids.ID) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		ws, err := websocket.Accept(w, req, &websocket.AcceptOptions{CompressionMode: websocket.CompressionDisabled})
		if err != nil {
			return
		}
		ws.SetReadLimit(-1)
		var sd *side
		var code int
		if strings.HasSuffix(req.URL.Path, "/dev") {
			sd, code = r.attachDevice(session, device, ws)
		} else {
			sd, code = r.attachHost(session, host, ws)
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

// dial opens a client of path and returns it with a channel of its close code.
func dial(t *testing.T, ctx context.Context, srv *httptest.Server, path string) (*websocket.Conn, chan int) {
	t.Helper()
	ws, _, err := fakehost.Dial(ctx, "ws"+strings.TrimPrefix(srv.URL, "http")+path)
	if err != nil {
		t.Fatal(err)
	}
	codes := make(chan int, 1)
	go func() { //nolint:gosec // G118: the reader lives as long as the connection, not as ctx
		for {
			if _, _, err := ws.Read(context.Background()); err != nil {
				var ce websocket.CloseError
				if errors.As(err, &ce) {
					codes <- int(ce.Code)
				} else {
					codes <- 0
				}
				return
			}
		}
	}()
	return ws, codes
}

// TestShutdown_Relay4013 (Design note 14): the stop step closes the device stream and the data
// connection of a paired session with 4013 and waits for their readers; no side attaches after it.
func TestShutdown_Relay4013(t *testing.T) {
	t.Parallel()
	uc := &fakeRelayUC{}
	r, err := NewRelay(RelayConfig{BufferMaxBytes: 1 << 16, OpenTimeout: time.Minute}, uc, nil, manualTimers{}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	session, host, device := ids.New(), ids.New(), ids.New()
	r.Expect(app.Expectation{SessionID: session, HostID: host, DeviceID: device, Deadline: time.Now().Add(time.Minute), Started: true})
	srv := relayServer(t, r, session, host, device)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	dev, devCode := dial(t, ctx, srv, "/dev")
	_, hostCode := dial(t, ctx, srv, "/host")
	if err := dev.Write(ctx, websocket.MessageBinary, []byte("m1")); err != nil {
		t.Fatal(err)
	}
	for paired := false; !paired; {
		r.mu.Lock()
		paired = r.slots[session] != nil && r.slots[session].paired
		r.mu.Unlock()
		if ctx.Err() != nil {
			t.Fatal("the sides never paired")
		}
		time.Sleep(5 * time.Millisecond)
	}
	if err := r.Shutdown(ctx); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
	// The pairing ended here: its counts are written (code review M2-44, M1).
	uc.mu.Lock()
	calls := strings.Join(uc.calls, ",")
	uc.mu.Unlock()
	if !strings.Contains(calls, "counters") {
		t.Fatalf("callbacks %s, want the counters of the pairing", calls)
	}
	for name, ch := range map[string]chan int{"device": devCode, "host": hostCode} {
		select {
		case c := <-ch:
			if c != 4013 {
				t.Errorf("%s closed with %d, want 4013", name, c)
			}
		case <-ctx.Done():
			t.Fatalf("%s not closed", name)
		}
	}
	if sd, code := r.attachDevice(session, device, nil); sd != nil || code != 4013 {
		t.Fatalf("attach after shutdown: %v %d", sd, code)
	}
}
