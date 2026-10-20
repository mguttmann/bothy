// SPDX-License-Identifier: Apache-2.0

package device

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"
)

// Dialer maps the instance origin to the published port of the TLS front: a connection to
// host:port (chat.example.com:8443) goes to 127.0.0.1:Port, every other address is refused, and
// TLS verifies the server against Roots only, so DPoP htu, the Origin rule and server.public_url
// stay those of the instance.
type Dialer struct {
	// Host and Port name the instance origin (chat.example.com and 8443).
	Host string
	Port int
	// Mapped is the published port of the TLS front on 127.0.0.1.
	Mapped int
	// Roots holds only the test CA of the stack.
	Roots *x509.CertPool

	// OnConn, when set, sees every TCP connection the dialer opens (the device cuts its stream
	// through it, TM-T16).
	OnConn func(net.Conn)
}

// DialContext implements the dialer of an http.Transport.
//
// Errors: an address other than the instance origin, or a failed dial of 127.0.0.1.
func (d *Dialer) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	if addr != net.JoinHostPort(d.Host, strconv.Itoa(d.Port)) {
		return nil, fmt.Errorf("device: refusing to dial %s: only the instance origin is reachable", addr)
	}
	var nd net.Dialer // nosemgrep: bothy-go-egress-only -- test client: it dials only the TLS front of the test stack on 127.0.0.1
	c, err := nd.DialContext(ctx, network, net.JoinHostPort("127.0.0.1", strconv.Itoa(d.Mapped)))
	if err != nil {
		return nil, err
	}
	if d.OnConn != nil {
		d.OnConn(c)
	}
	return c, nil
}

// TLSConfig returns the TLS configuration of the device: the test CA as the only root and the
// instance host as server name.
func (d *Dialer) TLSConfig() *tls.Config {
	return &tls.Config{RootCAs: d.Roots, ServerName: d.Host, MinVersion: tls.VersionTLS12}
}

// Transport returns an http.Transport over the dialer.
func (d *Dialer) Transport() *http.Transport {
	return &http.Transport{DialContext: d.DialContext, TLSClientConfig: d.TLSConfig(), ForceAttemptHTTP2: false,
		MaxIdleConnsPerHost: 16, IdleConnTimeout: 30 * time.Second, Proxy: nil}
}

// Client returns an http.Client over Transport.
func (d *Dialer) Client() *http.Client {
	return &http.Client{Transport: d.Transport(), Timeout: 60 * time.Second}
}
