// SPDX-License-Identifier: Apache-2.0

package ws

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
)

// upgrade sends a WebSocket upgrade request with the cookie and the Origin (each only when not
// empty) and returns the status of the answer.
func upgrade(t *testing.T, url, cookie, origin string) int {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Sec-WebSocket-Version", "13")
	// The handshake key is 16 random bytes in base64 (RFC 6455 section 4.1).
	key := make([]byte, 16)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
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

// TestConsoleWebSocketForeignOrigin (ID-T19, SEC-CORS-3, D-M6-01-5): an upgrade that carries the
// console cookie with a foreign Origin or without one is 403 before the upgrade; with the own
// Origin it upgrades. Without the cookie a missing Origin stays allowed (native apps) and a foreign
// one stays refused, as M1-64 built it.
func TestConsoleWebSocketForeignOrigin(t *testing.T) {
	t.Parallel()
	s := newTestServer(t, 16, nil)
	for _, tc := range []struct {
		name, cookie, origin string
		want                 int
	}{
		{"cookie without origin", "v", "", http.StatusForbidden},
		{"cookie with foreign origin", "v", "https://evil.example.com", http.StatusForbidden},
		{"cookie with own origin", "v", publicURL, http.StatusSwitchingProtocols},
		{"no cookie without origin", "", "", http.StatusSwitchingProtocols},
		{"no cookie with foreign origin", "", "https://evil.example.com", http.StatusForbidden},
	} {
		if got := upgrade(t, s.srv.URL+"/api/v1/gateway", tc.cookie, tc.origin); got != tc.want {
			t.Errorf("%s: status %d, want %d", tc.name, got, tc.want)
		}
	}
}
