// SPDX-License-Identifier: Apache-2.0

package wsrelay

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	gosync "sync"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
	"github.com/OWNER/bothy/services/server/internal/terminal/domain"
	"github.com/OWNER/bothy/services/server/test/terminal/fakehost"
)

// The console cookies of the stream tests: good authenticates Alex's console session, revoked a
// revoked one; every other value is unknown.
const (
	goodCookie    = "console-good"
	revokedCookie = "console-revoked"
	appToken      = "app-access-token"
)

// consoleDevice is the device of Alex's console session; appDevice the device of her app.
var (
	consoleDevice = ids.MustParse("019a0000-0000-7000-8000-000000001801")
	appDevice     = ids.MustParse("019a0000-0000-7000-8000-000000001802")
	alexID        = ids.MustParse("019a0000-0000-7000-8000-000000001803")
)

// consoleAuthenticator stands for identity's Authenticator: the console cookie path and the DPoP
// path of an app access token, each recorded.
type consoleAuthenticator struct {
	mu    gosync.Mutex
	creds []actor.Credentials
}

// Authenticate implements actor.Authenticator.
func (a *consoleAuthenticator) Authenticate(_ context.Context, c actor.Credentials) (actor.Actor, error) {
	a.mu.Lock()
	a.creds = append(a.creds, c)
	a.mu.Unlock()
	switch {
	case c.ConsoleCookie.Reveal() == goodCookie && c.Authorization == "":
		return actor.Actor{ID: alexID, Kind: actor.KindUser, SessionID: ids.New(), DeviceID: consoleDevice, ClientComponent: "console"}, nil
	case c.ConsoleCookie.Reveal() == revokedCookie:
		return actor.Actor{}, errs.New(errs.CodeSessionRevoked, "")
	case c.ConsoleCookie.IsZero() && c.Authorization == "DPoP "+appToken:
		return actor.Actor{ID: alexID, Kind: actor.KindUser, SessionID: ids.New(), DeviceID: appDevice, ClientComponent: "app"}, nil
	}
	return actor.Actor{}, errs.New(errs.CodeUnauthenticated, "")
}

// Revalidate implements actor.Authenticator; the stream does not call it.
func (a *consoleAuthenticator) Revalidate(context.Context, actor.Actor) error { return nil }

// streamUC records the principals StreamAccess decided for and the failed identifies.
type streamUC struct {
	mu     gosync.Mutex
	access []actor.Actor
	failed int
}

// StreamAccess implements StreamUseCases: every principal passes (authorize terminal.stream has
// its own tests).
func (u *streamUC) StreamAccess(_ context.Context, a actor.Actor, _ ids.ID) (domain.StreamCode, string, error) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.access = append(u.access, a)
	return 0, "", nil
}

// StreamAuthFailed implements StreamUseCases.
func (u *streamUC) StreamAuthFailed(context.Context, ids.ID) error {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.failed++
	return nil
}

// accessed returns the principals of StreamAccess so far.
func (u *streamUC) accessed() []actor.Actor {
	u.mu.Lock()
	defer u.mu.Unlock()
	return append([]actor.Actor(nil), u.access...)
}

type fixedNonce struct{}

func (fixedNonce) CurrentNonce() string { return "bm9uY2U" }

type acceptAll struct{}

func (acceptAll) Validate(string, []byte) error { return nil }

// consoleStream is the stream handler of the console tests behind an httptest server.
type consoleStream struct {
	srv  *httptest.Server
	auth *consoleAuthenticator
	uc   *streamUC
	path string
}

// newConsoleStream starts the handler whose server.public_url is the URL of the test server.
func newConsoleStream(t *testing.T) *consoleStream {
	t.Helper()
	cs := &consoleStream{auth: &consoleAuthenticator{}, uc: &streamUC{}, path: "/api/v1/terminal/sessions/" + ids.New().String() + "/stream"}
	mux := http.NewServeMux()
	cs.srv = httptest.NewServer(mux)
	t.Cleanup(cs.srv.Close)
	relay, err := NewRelay(RelayConfig{BufferMaxBytes: 1 << 16, OpenTimeout: time.Minute}, &fakeRelayUC{}, nil, manualTimers{}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	h, err := NewStreamHandler(StreamConfig{PublicURL: cs.srv.URL, OpenTimeout: time.Minute, MaxFrameBytes: 8192, MaxJSONDepth: 32},
		StreamDeps{UseCases: cs.uc, Authenticator: cs.auth, Nonces: fixedNonce{}, Relay: relay, Schemas: acceptAll{}, Timers: manualTimers{}})
	if err != nil {
		t.Fatal(err)
	}
	mux.Handle("GET /api/v1/terminal/sessions/{"+sessionPathValue+"}/stream", h)
	return cs
}

// upgradeStatus sends a bare upgrade request with the cookie and the Origin (each only when not
// empty) and returns the status of the answer.
func (cs *consoleStream) upgradeStatus(t *testing.T, cookie, origin string) int {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, cs.srv.URL+cs.path, nil)
	if err != nil {
		t.Fatal(err)
	}
	key := make([]byte, 16)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Sec-WebSocket-Version", "13")
	req.Header.Set("Sec-WebSocket-Key", base64.StdEncoding.EncodeToString(key))
	if cookie != "" {
		req.Header.Set("Cookie", actor.ConsoleCookieName+"="+cookie)
	}
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	return resp.StatusCode
}

// identify dials the stream with the cookie (none when empty) and the own Origin, reads hello and
// sends identify with d; it returns the open stream, which the test closes at its end.
func (cs *consoleStream) identify(t *testing.T, cookie string, d map[string]any) *websocket.Conn {
	t.Helper()
	ctx := testkit.Context(t)
	h := http.Header{"Origin": []string{cs.srv.URL}}
	if cookie != "" {
		h.Set("Cookie", actor.ConsoleCookieName+"="+cookie)
	}
	ws, _, err := fakehost.DialHeader(ctx, "ws"+cs.srv.URL[len("http"):]+cs.path, h)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ws.CloseNow() })
	if _, _, err := ws.Read(ctx); err != nil {
		t.Fatalf("hello: %v", err)
	}
	b, _ := json.Marshal(map[string]any{"op": "identify", "d": d})
	if err := ws.Write(ctx, websocket.MessageText, b); err != nil {
		t.Fatal(err)
	}
	return ws
}

// closeCode reads ws until the server closes it and returns the close code; it fails the test
// when no close arrives within testkit.Patience.
func closeCode(t *testing.T, ws *websocket.Conn) int {
	t.Helper()
	ctx := testkit.Context(t)
	for {
		_, _, err := ws.Read(ctx)
		if err == nil {
			continue
		}
		code := websocket.CloseStatus(err)
		if code == -1 {
			t.Fatalf("the stream ended without a close code: %v", err)
		}
		return int(code)
	}
}

// TestStreamUpgrade_ForeignOrigin (08 section 7.5, SEC-CORS-3, M6-18): an upgrade with the console
// cookie and a foreign or missing Origin is answered 403 before the upgrade; with the own Origin
// it upgrades; without the cookie a missing Origin stays allowed (native apps) and a foreign one
// refused.
func TestStreamUpgrade_ForeignOrigin(t *testing.T) {
	t.Parallel()
	cs := newConsoleStream(t)
	for _, c := range []struct {
		name, cookie, origin string
		want                 int
	}{
		{"cookie, foreign origin", goodCookie, "https://evil.example.com", http.StatusForbidden},
		{"cookie, no origin", goodCookie, "", http.StatusForbidden},
		{"cookie, null origin", goodCookie, "null", http.StatusForbidden},
		{"cookie, own origin", goodCookie, cs.srv.URL, http.StatusSwitchingProtocols},
		{"no cookie, no origin", "", "", http.StatusSwitchingProtocols},
		{"no cookie, foreign origin", "", "https://evil.example.com", http.StatusForbidden},
	} {
		if got := cs.upgradeStatus(t, c.cookie, c.origin); got != c.want {
			t.Errorf("%s: status %d, want %d", c.name, got, c.want)
		}
	}
	if len(cs.auth.creds) != 0 {
		t.Fatal("a refused upgrade reached the authenticator")
	}
}

// TestStreamIdentify_NoTokenOnlyFromConsole (08 section 7.5, TerminalStreamIdentify, M6-18): an
// identify without a token is accepted only on a connection that carries the console session of
// the upgrade, for the device of that session; without a console session (also next to an app's
// access token), with a token on a console connection, with another device or with a revoked or
// unknown cookie the stream closes with 4004 (4012 for the revoked one); the accepted identify
// reaches authorize terminal.stream with the console principal.
func TestStreamIdentify_NoTokenOnlyFromConsole(t *testing.T) {
	t.Parallel()
	cs := newConsoleStream(t)
	noToken := map[string]any{"protocol_version": 1, "device_id": consoleDevice.String()}
	withToken := map[string]any{"protocol_version": 1, "device_id": consoleDevice.String(), "access_token": appToken, "dpop": "proof"}
	for _, c := range []struct {
		name   string
		cookie string
		d      map[string]any
		want   int
	}{
		{"no console session", "", noToken, int(domain.StreamAuthFailed)},
		{"app device without a token", "", map[string]any{"protocol_version": 1, "device_id": appDevice.String()}, int(domain.StreamAuthFailed)},
		{"token on a console connection", goodCookie, withToken, int(domain.StreamAuthFailed)},
		{"another device", goodCookie, map[string]any{"protocol_version": 1, "device_id": appDevice.String()}, int(domain.StreamAuthFailed)},
		{"unknown cookie", "console-unknown", noToken, int(domain.StreamAuthFailed)},
		{"revoked console session", revokedCookie, noToken, int(domain.StreamRevoked)},
	} {
		if got := closeCode(t, cs.identify(t, c.cookie, c.d)); got != c.want {
			t.Errorf("%s: close %d, want %d", c.name, got, c.want)
		}
	}
	if n := len(cs.uc.accessed()); n != 0 {
		t.Fatalf("%d refused identifies reached authorize", n)
	}
	// The accepted identify reaches authorize terminal.stream; a refused one never does (above).
	cs.identify(t, goodCookie, noToken)
	testkit.Until(t, "authorize terminal.stream of the console identify", func() bool { return len(cs.uc.accessed()) > 0 })
	acc := cs.uc.accessed()
	if len(acc) != 1 || acc[0].ClientComponent != "console" || acc[0].DeviceID != consoleDevice {
		t.Fatalf("authorize saw %+v, want the console principal", acc)
	}
}
