// SPDX-License-Identifier: Apache-2.0

// Package faketls places the in-process provider Fakes of the push adapters (apnstest of M3-20,
// fcmtest of M3-21) on a given address, under a given host name and certificate, so the same
// Fake that the in-process tests start on 127.0.0.1 can also serve in its own container on a
// Compose network (fakepush of the mobile E2E test, brief M5-11). Both Fakes share it, so the
// listener rules exist once. It imports no testing package; the server binary never links it.
//
// Example:
//
//	srv := httptest.NewUnstartedServer(handler)
//	if err := faketls.Listen(srv, "apnstest", ":8443", &leaf); err != nil { ... }
//	srv.StartTLS()
//	base := faketls.BaseURL(srv, "fakepush") // https://fakepush:8443
package faketls

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http/httptest"
)

// Listen prepares the unstarted srv: with a non-empty addr it replaces the listener httptest
// chose (127.0.0.1 on a free port) with one on addr, and with a non-nil cert it serves cert
// (TLS 1.2 or later) instead of the certificate of httptest. Call it before StartTLS.
//
// The error, prefixed with name, reports a failed listen on addr; the listener of httptest is
// closed then, so nothing stays open.
func Listen(srv *httptest.Server, name, addr string, cert *tls.Certificate) error {
	if addr != "" {
		var lc net.ListenConfig
		ln, err := lc.Listen(context.Background(), "tcp", addr)
		_ = srv.Listener.Close()
		if err != nil {
			return fmt.Errorf("%s: listen on %s: %w", name, addr, err)
		}
		srv.Listener = ln
	}
	if cert != nil {
		srv.TLS = &tls.Config{Certificates: []tls.Certificate{*cert}, MinVersion: tls.VersionTLS12}
	}
	return nil
}

// BaseURL returns the base URL of the started srv: srv.URL with an empty host, else
// https://<host>:<port of the listener>, the name a client on the same network uses and the
// certificate must carry.
func BaseURL(srv *httptest.Server, host string) string {
	if host == "" {
		return srv.URL
	}
	_, port, _ := net.SplitHostPort(srv.Listener.Addr().String())
	return "https://" + net.JoinHostPort(host, port)
}
