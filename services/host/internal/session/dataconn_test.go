// SPDX-License-Identifier: Apache-2.0

package session

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/coder/websocket"

	"github.com/OWNER/bothy/services/host/internal/control"
	"github.com/OWNER/bothy/services/host/internal/platform/testkit"
	"github.com/OWNER/bothy/services/host/internal/session/sessiontest"
)

// wsServer serves every websocket connection of the path /data with serve and returns the ws URL
// of /data; the server runs on 127.0.0.1 with a port the system chooses.
func wsServer(t *testing.T, serve func(ctx context.Context, ws *websocket.Conn)) string {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /data", func(w http.ResponseWriter, r *http.Request) {
		ws, err := websocket.Accept(w, r, &websocket.AcceptOptions{CompressionMode: websocket.CompressionDisabled})
		if err != nil {
			return
		}
		defer func() { _ = ws.CloseNow() }()
		serve(r.Context(), ws)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return "ws" + strings.TrimPrefix(srv.URL, "http") + "/data"
}

// helloFrame returns the text frame hello with the values v.
func helloFrame(t *testing.T, v sessiontest.Values) []byte {
	t.Helper()
	b, err := json.Marshal(map[string]any{"op": "hello", "d": sessiontest.Hello([]map[string]any{sessiontest.NewSigner("terminal-key-1").JWK()}, v)})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// closeOf reads ws until it ends and returns the close code of the other side, -1 for a lost
// connection.
func closeOf(ctx context.Context, ws *websocket.Conn) int {
	for {
		if _, _, err := ws.Read(ctx); err != nil {
			var ce websocket.CloseError
			if errors.As(err, &ce) {
				return int(ce.Code)
			}
			return -1
		}
	}
}

func dialTest(ctx context.Context, url string) (*dataConn, error) {
	return dialData(ctx, http.DefaultClient, url, identifyData{HostID: hostID, HostVersion: "1.2.3", SessionID: sessID},
		func(string) string { return "proof" })
}

// TestDialDataRefusesAnUnusableServer: a failed upgrade, a first message that is not text and a
// hello without the values of M2-45 give no data connection; an undecodable hello is answered
// with 4002.
func TestDialDataRefusesAnUnusableServer(t *testing.T) {
	ctx := testkit.Context(t)
	t.Run("upgrade refused", func(t *testing.T) {
		url := wsServer(t, func(context.Context, *websocket.Conn) {})
		dc, err := dialTest(ctx, strings.TrimSuffix(url, "/data")+"/elsewhere")
		if err == nil || dc != nil {
			t.Fatalf("dial of a path without a data endpoint: %v", err)
		}
	})
	t.Run("binary first message", func(t *testing.T) {
		url := wsServer(t, func(ctx context.Context, ws *websocket.Conn) {
			_ = ws.Write(ctx, websocket.MessageBinary, []byte{1})
			closeOf(ctx, ws)
		})
		if dc, err := dialTest(ctx, url); !errors.Is(err, errHello) || dc != nil {
			t.Fatalf("binary hello: %v", err)
		}
	})
	t.Run("hello without an idle value", func(t *testing.T) {
		got := make(chan int, 1)
		v := sessiontest.DefaultValues
		v.IdleTimeoutMs = 0
		hello := helloFrame(t, v)
		url := wsServer(t, func(ctx context.Context, ws *websocket.Conn) {
			_ = ws.Write(ctx, websocket.MessageText, hello)
			got <- closeOf(ctx, ws)
		})
		if dc, err := dialTest(ctx, url); !errors.Is(err, errHello) || dc != nil {
			t.Fatalf("hello without idle_timeout_ms: %v", err)
		}
		select {
		case code := <-got:
			if code != control.CodeDecodeError {
				t.Errorf("close code %d, want %d", code, control.CodeDecodeError)
			}
		case <-ctx.Done():
			t.Fatal("the server saw no close")
		}
	})
}

// TestDataConnTextFrameEnds: a text frame after identify ends the data connection like the relay:
// close 4002, and nothing reaches the session.
func TestDataConnTextFrameEnds(t *testing.T) {
	ctx := testkit.Context(t)
	got := make(chan int, 1)
	hello := helloFrame(t, sessiontest.DefaultValues)
	url := wsServer(t, func(ctx context.Context, ws *websocket.Conn) {
		_ = ws.Write(ctx, websocket.MessageText, hello)
		if _, _, err := ws.Read(ctx); err != nil {
			return
		}
		_ = ws.Write(ctx, websocket.MessageText, []byte(`{"op":"noise"}`))
		got <- closeOf(ctx, ws)
	})
	dc, err := dialTest(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case code := <-got:
		if code != control.CodeDecodeError {
			t.Errorf("close code %d, want %d", code, control.CodeDecodeError)
		}
	case <-ctx.Done():
		t.Fatal("the server saw no close")
	}
	select {
	case <-dc.done:
	case <-ctx.Done():
		t.Fatal("the data connection did not end")
	}
	if _, ok := <-dc.in; ok {
		t.Error("the text frame reached the session")
	}
}

// rawConn dials url and returns the host end of a websocket connection without the goroutines of
// dialData, so a test controls the queue of the writer.
func rawConn(t *testing.T, ctx context.Context, url string, queue int) *dataConn {
	t.Helper()
	ws, resp, err := websocket.Dial(ctx, url, nil)
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ws.CloseNow() })
	return &dataConn{ws: ws, in: make(chan []byte, 1), out: make(chan []byte, queue), done: make(chan struct{}), code: -1}
}

// TestDataConnSlowDeviceLosesTheConnection: a full send queue closes the connection with 1000
// instead of blocking the session (a slow device never blocks the helper).
func TestDataConnSlowDeviceLosesTheConnection(t *testing.T) {
	ctx := testkit.Context(t)
	got := make(chan int, 1)
	url := wsServer(t, func(ctx context.Context, ws *websocket.Conn) { got <- closeOf(ctx, ws) })
	dc := rawConn(t, ctx, url, 1)
	dc.send([]byte("queued"))
	dc.send([]byte("one too many"))
	select {
	case code := <-got:
		if code != CodeNormal {
			t.Errorf("close code %d, want %d", code, CodeNormal)
		}
	case <-ctx.Done():
		t.Fatal("a full queue did not close the connection")
	}
	select {
	case <-dc.done:
	case <-ctx.Done():
		t.Fatal("the end was not recorded")
	}
	if dc.closeCode() != CodeNormal {
		t.Errorf("recorded close code %d", dc.closeCode())
	}
	// A send after the end neither blocks nor panics.
	dc.send([]byte("after the end"))
}

// TestDataConnWriteFailureEnds: a failed write ends the connection.
func TestDataConnWriteFailureEnds(t *testing.T) {
	ctx := testkit.Context(t)
	url := wsServer(t, func(ctx context.Context, ws *websocket.Conn) { closeOf(ctx, ws) })
	dc := rawConn(t, ctx, url, 4)
	_ = dc.ws.CloseNow()
	go dc.write(ctx)
	dc.send([]byte("lost"))
	select {
	case <-dc.done:
	case <-ctx.Done():
		t.Fatal("a failed write did not end the connection")
	}
	if dc.closeCode() != CodeNormal {
		t.Errorf("recorded close code %d", dc.closeCode())
	}
}
