// SPDX-License-Identifier: Apache-2.0

package control_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/OWNER/bothy/services/host/internal/control"
	"github.com/OWNER/bothy/services/host/internal/control/controltest"
	"github.com/OWNER/bothy/services/host/internal/startup"
)

// dataFrame returns the text frame {op, d}.
func dataFrame(t *testing.T, op string, d any) []byte {
	t.Helper()
	b, err := json.Marshal(map[string]any{"op": op, "d": d})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// TestDecodeHello (M2-45): the first frame of a data connection is a hello decoded with the strict
// rules of the control connection; any other op or a malformed frame is ErrFrame.
func TestDecodeHello(t *testing.T) {
	t.Parallel()
	h, err := control.DecodeHello(dataFrame(t, "hello", controltest.HelloWith(15000, 3, 1000, 2000, 3000)))
	if err != nil {
		t.Fatal(err)
	}
	if h.HeartbeatIntervalMs != 15000 || h.ClockLeewayMs != 1000 || h.IdleTimeoutMs != 2000 || h.DetachGraceMs != 3000 ||
		len(h.JWKS.Keys) != 1 || h.JWKS.Keys[0]["kid"] != "terminal-key-1" {
		t.Errorf("hello %+v", h)
	}
	for name, b := range map[string][]byte{
		"ready instead of hello": dataFrame(t, "ready", controltest.Ready(hostID, "none")),
		"not json":               []byte("hello"),
		"unknown member":         dataFrame(t, "hello", func() map[string]any { m := controltest.Hello(15000, 3); m["extra"] = 1; return m }()),
	} {
		if _, err := control.DecodeHello(b); !errors.Is(err, control.ErrFrame) {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// TestDataURL (M2-45): the data connection dials the data_path on the server of the pairing record
// (below the base path of the server URL, scheme wss); a path outside the channel hostData, with a
// query, a fragment or a dot segment, is ErrDataPath, so the server cannot steer the dial elsewhere.
func TestDataURL(t *testing.T) {
	t.Parallel()
	c, err := control.New(control.Config{ServerURL: "https://example.com/bothy/", HostID: hostID, AuthKey: key(), HostVersion: "1.0.0",
		BackoffBase: base, BackoffCap: capDelay}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })
	got, err := c.DataURL(control.DataPathPrefix + "0192f4a0-7b1c-7b22-9d4f-00000000a001/data")
	if want := "wss://example.com/bothy" + control.DataPathPrefix + "0192f4a0-7b1c-7b22-9d4f-00000000a001/data"; err != nil || got != want {
		t.Errorf("DataURL %q %v, want %q", got, err, want)
	}
	for _, p := range []string{
		"/api/v1/hosts/connect",
		"https://other.example.com" + control.DataPathPrefix + "x",
		control.DataPathPrefix + "x?y=1",
		control.DataPathPrefix + "x#y",
		control.DataPathPrefix + "../../connect",
		"",
	} {
		if u, err := c.DataURL(p); !errors.Is(err, control.ErrDataPath) || u != "" {
			t.Errorf("%q: %q %v", p, u, err)
		}
	}
}

// TestHTTPClientFollowsNoRedirect (M2-45): the data connections dial with the client of the control
// connection, which never follows a redirect.
func TestHTTPClientFollowsNoRedirect(t *testing.T) {
	t.Parallel()
	srv := controltest.New(t)
	c := newClient(t, srv, nil)
	hc := c.HTTPClient()
	if hc == nil || hc.CheckRedirect == nil {
		t.Fatal("no HTTP client or no redirect policy")
	}
	if err := hc.CheckRedirect(&http.Request{}, nil); !errors.Is(err, http.ErrUseLastResponse) {
		t.Errorf("redirect policy %v", err)
	}
	if c.HTTPClient() != hc {
		t.Error("the client changes between calls")
	}
}

// TestPeerLocal (24 section 2.3): without a live connection the server is not local; a server
// reached on loopback is.
func TestPeerLocal(t *testing.T) {
	t.Parallel()
	srv := controltest.New(t)
	c := newClient(t, srv, nil)
	if c.PeerLocal() {
		t.Error("local before any connection")
	}
	type result struct {
		r   startup.Ready
		err error
	}
	done := make(chan result, 1)
	go func() {
		r, err := c.Login(ctx(t))
		done <- result{r, err}
	}()
	handshake(t, srv, 60000, "none")
	select {
	case res := <-done:
		if res.err != nil {
			t.Fatal(res.err)
		}
	case <-ctx(t).Done():
		t.Fatal("login did not return")
	}
	if !c.PeerLocal() {
		t.Error("a server on loopback is not local")
	}
}
