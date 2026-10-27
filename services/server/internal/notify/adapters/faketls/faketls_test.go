// SPDX-License-Identifier: Apache-2.0

package faketls_test

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/notify/adapters/faketls"
)

func TestListenKeepsTheDefaultsOfHTTPTest(t *testing.T) {
	srv := httptest.NewUnstartedServer(http.NotFoundHandler())
	if err := faketls.Listen(srv, "fake", "", nil); err != nil {
		t.Fatal(err)
	}
	srv.StartTLS()
	t.Cleanup(srv.Close)
	if got := faketls.BaseURL(srv, ""); got != srv.URL {
		t.Fatalf("BaseURL = %q, want %q", got, srv.URL)
	}
}

func TestListenOnAddrWithCertificateAndHost(t *testing.T) {
	// The certificate of a started httptest server stands in for a leaf of a per-run CA.
	donor := httptest.NewTLSServer(http.NotFoundHandler())
	cert := donor.TLS.Certificates[0]
	donor.Close()

	srv := httptest.NewUnstartedServer(http.NotFoundHandler())
	if err := faketls.Listen(srv, "fake", "127.0.0.1:0", &cert); err != nil {
		t.Fatal(err)
	}
	srv.StartTLS()
	t.Cleanup(srv.Close)
	if len(srv.TLS.Certificates) != 1 || srv.TLS.MinVersion != tls.VersionTLS12 {
		t.Fatalf("TLS config = %+v, want the given certificate and TLS 1.2 or later", srv.TLS)
	}
	_, port, err := net.SplitHostPort(srv.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	if got, want := faketls.BaseURL(srv, "fakepush"), "https://fakepush:"+port; got != want {
		t.Fatalf("BaseURL = %q, want %q", got, want)
	}
}

func TestListenReportsAFailedListenAndClosesTheDefault(t *testing.T) {
	var lc net.ListenConfig
	busy, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = busy.Close() })
	srv := httptest.NewUnstartedServer(http.NotFoundHandler())
	err = faketls.Listen(srv, "fake", busy.Addr().String(), nil)
	if err == nil || !strings.HasPrefix(err.Error(), "fake: listen on ") {
		t.Fatalf("err = %v, want a failed listen named after the fake", err)
	}
	// The default listener of httptest is closed, so nothing is left open.
	if _, aerr := srv.Listener.Accept(); aerr == nil {
		t.Fatal("the default listener of httptest is still open")
	}
}
