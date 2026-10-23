// SPDX-License-Identifier: Apache-2.0

package egress_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/platform/egress"
)

// TestHTTP2Option (brief M3-20 design note 1 d): with Options.HTTP2 a TLS target behind an
// allow_private rule negotiates h2 by ALPN, without it the client stays on HTTP/1.1, and a plain
// http target stays on HTTP/1.1 also with the option (there is no cleartext HTTP/2).
func TestHTTP2Option(t *testing.T) {
	t.Parallel()
	tlsSrv := httptest.NewUnstartedServer(http.HandlerFunc(ok))
	tlsSrv.EnableHTTP2 = true
	tlsSrv.StartTLS()
	t.Cleanup(tlsSrv.Close)
	plain := newServer(t, ok)
	tlsPort := (&server{Server: tlsSrv}).port(t)
	plainPort := plain.port(t)
	roots := tlsSrv.Client().Transport.(*http.Transport).TLSClientConfig.RootCAs
	cfg := egress.Config{
		AllowedHosts: []egress.Rule{{Host: "127.0.0.1", Port: tlsPort, AllowPrivate: true}, {Host: "127.0.0.1", Port: plainPort, AllowPrivate: true}},
		Timeout:      5 * time.Second, MaxResponseBytes: 1 << 16,
	}
	proto := func(t *testing.T, http2 bool, target string) int {
		t.Helper()
		c, err := egress.New(cfg, egress.Options{Purpose: "test", HTTP2: http2}, egress.Deps{RootCAs: roots})
		if err != nil {
			t.Fatal(err)
		}
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, target, nil)
		if err != nil {
			t.Fatal(err)
		}
		resp, err := c.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status %d", resp.StatusCode)
		}
		return resp.ProtoMajor
	}
	tlsURL := "https://127.0.0.1:" + strconv.Itoa(int(tlsPort)) + "/"
	plainURL := "http://127.0.0.1:" + strconv.Itoa(int(plainPort)) + "/"
	if got := proto(t, true, tlsURL); got != 2 {
		t.Errorf("HTTP2 true over TLS: ProtoMajor %d, want 2", got)
	}
	if got := proto(t, false, tlsURL); got != 1 {
		t.Errorf("HTTP2 false over TLS: ProtoMajor %d, want 1", got)
	}
	if got := proto(t, true, plainURL); got != 1 {
		t.Errorf("HTTP2 true over plain http: ProtoMajor %d, want 1", got)
	}
}
