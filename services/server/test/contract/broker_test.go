// SPDX-License-Identifier: Apache-2.0

package contract

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/jose"
	"github.com/OWNER/bothy/services/server/test/harness"
)

func TestMain(m *testing.M) { os.Exit(harness.Main(m)) }

// post sends body to the broker path and returns status and body.
func post(t *testing.T, b *Broker, path, body string) (int, []byte) {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, b.URL()+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(resp.Body)
	return resp.StatusCode, buf.Bytes()
}

// TestBroker: owner and user credentials are fresh sessions with a usable private DPoP key; the
// step-up endpoint confirms a session it handed out and refuses an unknown one; the broker
// listens on loopback and refuses other peers.
func TestBroker(t *testing.T) {
	t.Parallel()
	s := harness.Start(t, harness.Options{EgressDenyAll: true, Config: harness.RaisedRateLimits()})
	alex := s.NewUser(t, harness.UserSpec{Handle: "alex", DisplayName: "Alex"})
	b, err := StartBroker(s, alex.PrincipalID)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = b.Close() })
	if u, err := url.Parse(b.URL()); err != nil || u.Hostname() != "127.0.0.1" {
		t.Fatalf("broker URL %q", b.URL())
	}
	seen := map[string]bool{}
	for kind, principal := range map[string]string{KindOwner: s.Owner.PrincipalID, KindUser: alex.PrincipalID} {
		code, body := post(t, b, "/principals", `{"kind":"`+kind+`"}`)
		if code != http.StatusOK {
			t.Fatalf("%s: %d %s", kind, code, body)
		}
		var c Credentials
		if err := json.Unmarshal(body, &c); err != nil {
			t.Fatal(err)
		}
		if c.PrincipalID != principal || c.SessionID == "" || c.DeviceID == "" || c.AccessToken == "" || c.RefreshToken == "" || seen[c.SessionID] {
			t.Fatalf("%s credentials %+v", kind, c)
		}
		seen[c.SessionID] = true
		var jwk map[string]string
		if err := json.Unmarshal(c.DPoPPrivateJWK, &jwk); err != nil || jwk["kty"] != "EC" || jwk["crv"] != "P-256" || jwk["d"] == "" {
			t.Fatalf("%s key %s", kind, c.DPoPPrivateJWK)
		}
		if _, err := jose.ParseJWK(mustJSON(t, map[string]string{"kty": "EC", "crv": "P-256", "x": jwk["x"], "y": jwk["y"]})); err != nil {
			t.Fatalf("%s public part: %v", kind, err)
		}
		if code, body := post(t, b, "/step-up", `{"session_id":"`+c.SessionID+`"}`); code != http.StatusOK {
			t.Fatalf("%s step-up: %d %s", kind, code, body)
		}
	}
	if code, _ := post(t, b, "/step-up", `{"session_id":"nope"}`); code != http.StatusNotFound {
		t.Errorf("unknown session: %d", code)
	}
	if code, _ := post(t, b, "/principals", `{"kind":"app"}`); code != http.StatusBadGateway {
		t.Errorf("unknown kind: %d", code)
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/principals", strings.NewReader(`{"kind":"owner"}`))
	req.RemoteAddr = "192.0.2.1:4000"
	b.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("foreign peer: %d", rec.Code)
	}
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
