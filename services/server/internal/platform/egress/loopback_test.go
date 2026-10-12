// SPDX-License-Identifier: Apache-2.0

package egress_test

import (
	"errors"
	"io"
	"net/http"
	"net/netip"
	"strconv"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/platform/egress"
)

// loopbackGet sends a GET with c and reads and closes the response.
func loopbackGet(t *testing.T, c *http.Client, target string) (reply, error) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, target, nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := c.Do(req)
	if err != nil {
		return reply{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return reply{status: resp.StatusCode, header: resp.Header, text: string(b), url: resp.Request.URL}, nil
}

func TestLoopbackClientRefusesNonLoopback(t *testing.T) {
	t.Parallel()
	res := newResolver(map[string][]netip.Addr{
		"health.example.com": {firstHost("10.0.0.0/8")},
		"mixed.example.com":  {netip.MustParseAddr("192.0.2.10"), netip.MustParseAddr("169.254.169.254")},
	})
	c := egress.NewLoopbackResolver(5*time.Second, res)
	for _, target := range []string{
		"http://health.example.com:8081/healthz",
		"http://mixed.example.com:8081/healthz",
		"http://192.0.2.10:8081/healthz",
		"http://[2001:db8::10]:8081/healthz",
		"http://[::ffff:192.0.2.10]:8081/healthz",
		"http://0.0.0.0:8081/healthz",
	} {
		r, err := loopbackGet(t, c, target)
		if err == nil {
			t.Fatalf("%s: reached with status %d", target, r.status)
		}
		wantBlocked(t, err, egress.ReasonAddressBlocked)
	}
}

func TestLoopbackClientReachesLoopback(t *testing.T) {
	t.Parallel()
	srv := newServer(t, ok)
	port := strconv.Itoa(int(srv.port(t)))
	res := newResolver(map[string][]netip.Addr{"localhost": {loopback4}}) // the test server listens on 127.0.0.1 only
	c := egress.NewLoopbackResolver(5*time.Second, res)
	for _, target := range []string{
		srv.URL + "/healthz",
		"http://[::ffff:127.0.0.1]:" + port + "/healthz",
		"http://localhost:" + port + "/healthz",
	} {
		r, err := loopbackGet(t, c, target)
		if err != nil {
			t.Fatalf("%s: %v", target, err)
		}
		if r.text != "ok" {
			t.Fatalf("%s: body %q", target, r.text)
		}
	}
	if real := egress.NewLoopback(5 * time.Second); real.Timeout != 5*time.Second {
		t.Fatalf("NewLoopback timeout %v", real.Timeout)
	}
	if _, err := loopbackGet(t, egress.NewLoopback(5*time.Second), srv.URL+"/healthz"); err != nil {
		t.Fatalf("NewLoopback: %v", err)
	}
}

func TestLoopbackClientRefusesHTTPSAndUserInfo(t *testing.T) {
	t.Parallel()
	res := newResolver(nil)
	c := egress.NewLoopbackResolver(5*time.Second, res)
	_, err := loopbackGet(t, c, "https://127.0.0.1:8443/healthz")
	wantBlocked(t, err, egress.ReasonSchemeNotAllowed)
	_, err = loopbackGet(t, c, "http://user:pw@127.0.0.1:8081/healthz")
	wantBlocked(t, err, egress.ReasonURLNotAllowed)
	if n := res.calls.Load(); n != 0 {
		t.Errorf("resolver called %d times, want 0", n)
	}
}

func TestLoopbackClientNoRedirects(t *testing.T) {
	t.Parallel()
	target := newServer(t, ok)
	origin := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusFound)
	})
	r, err := loopbackGet(t, egress.NewLoopback(5*time.Second), origin.URL)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if r.status != http.StatusFound {
		t.Fatalf("status %d, want 302", r.status)
	}
	if n := target.count.Load(); n != 0 {
		t.Errorf("target saw %d requests, want 0", n)
	}
}

func TestNewLoopbackNeedsPositiveTimeout(t *testing.T) {
	t.Parallel()
	for _, d := range []time.Duration{0, -time.Second} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("NewLoopback(%v) did not panic", d)
				}
			}()
			_ = egress.NewLoopback(d)
		}()
	}
}

func TestLoopbackBlockedErrorIsErrBlocked(t *testing.T) {
	t.Parallel()
	c := egress.NewLoopbackResolver(5*time.Second, newResolver(nil))
	_, err := loopbackGet(t, c, "http://192.0.2.10/")
	if !errors.Is(err, egress.ErrBlocked) {
		t.Fatalf("%v is not ErrBlocked", err)
	}
}
