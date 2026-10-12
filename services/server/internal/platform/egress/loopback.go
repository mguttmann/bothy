// SPDX-License-Identifier: Apache-2.0

package egress

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"syscall"
	"time"
)

// NewLoopback returns a client that reaches only loopback addresses (127.0.0.0/8 and ::1, also
// as IPv4-mapped IPv6) over plain http, without redirects and without environment proxies; it is
// the client for `bothy healthcheck` (docs/spec/16-operations.md 4.2), so that the health probe
// needs no hole in the lint of 19 SEC-NET-1.
//
// A name is resolved with net.DefaultResolver at connection time and only its loopback
// addresses are dialed; any other address ends with a *BlockedError with ReasonAddressBlocked.
// An https URL ends with ReasonSchemeNotAllowed, user info with ReasonURLNotAllowed. A 3xx
// response is returned as it is; a 101 answer (protocol switch) is an error, because no deadline
// would bound reading the switched connection. timeout bounds the whole request and must be positive;
// NewLoopback panics otherwise, because a client without a time limit is a programming error.
//
// Example:
//
//	resp, err := egress.NewLoopback(5 * time.Second).Get("http://127.0.0.1:8081/healthz")
func NewLoopback(timeout time.Duration) *http.Client {
	return newLoopback(timeout, net.DefaultResolver)
}

// newLoopback is NewLoopback with a resolver.
func newLoopback(timeout time.Duration, r Resolver) *http.Client {
	if timeout <= 0 {
		panic("egress: NewLoopback needs a positive timeout")
	}
	l := newLoopbackDialer(timeout, r)
	transport := &http.Transport{ // nosemgrep: bothy-go-egress-only -- the egress package is the one place that builds transports (19 SEC-NET-1)
		DialContext:       l.dialContext,
		DisableKeepAlives: true,
	}
	return &http.Client{ // nosemgrep: bothy-go-egress-only -- the egress package is the one place that builds transports (19 SEC-NET-1)
		Transport:     &loopbackTransport{next: transport},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		Timeout:       timeout,
	}
}

// loopback dials loopback addresses only.
type loopback struct {
	resolver Resolver
	timeout  time.Duration
	// dial opens a connection to one loopback address ("ip:port").
	dial func(ctx context.Context, network, address string) (net.Conn, error)
}

// newLoopbackDialer returns the dialer of the loopback client.
func newLoopbackDialer(timeout time.Duration, r Resolver) *loopback {
	return &loopback{resolver: r, timeout: timeout, dial: loopbackConnect}
}

// loopbackTransport refuses everything but plain http without user info before the connection.
type loopbackTransport struct {
	next http.RoundTripper
}

// RoundTrip implements http.RoundTripper.
func (t *loopbackTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	tgt, refused := parseTarget(req.URL)
	if refused == nil && tgt.scheme != schemeHTTP {
		refused = &BlockedError{Reason: ReasonSchemeNotAllowed, Host: tgt.host}
	}
	if refused != nil {
		if req.Body != nil {
			_ = req.Body.Close()
		}
		return nil, refused
	}
	resp, err := t.next.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusSwitchingProtocols {
		_ = resp.Body.Close()
		return nil, errProtocolSwitch
	}
	return resp, nil
}

// CloseIdleConnections forwards to the transport; the loopback client keeps no idle connections.
func (t *loopbackTransport) CloseIdleConnections() { closeIdle(t.next) }

// dialContext resolves the host and dials its loopback addresses only.
func (l *loopback) dialContext(ctx context.Context, network, address string) (net.Conn, error) {
	hostname, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, fmt.Errorf("egress: dial address: %w", err)
	}
	host := strings.TrimSuffix(strings.ToLower(hostname), ".")
	ctx, cancel := context.WithTimeout(ctx, l.timeout)
	defer cancel()
	var answer []netip.Addr
	if addr, perr := netip.ParseAddr(host); perr == nil {
		answer = []netip.Addr{addr}
	} else if answer, err = l.resolver.LookupNetIP(ctx, "ip", host); err != nil {
		return nil, err
	}
	var first error
	for _, a := range answer {
		if !loopbackAddr(a) {
			continue
		}
		conn, err := l.dial(ctx, network, net.JoinHostPort(canonicalAddr(a).String(), port))
		if err == nil {
			return conn, nil
		}
		if first == nil {
			first = err
		}
	}
	if first == nil {
		return nil, &BlockedError{Reason: ReasonAddressBlocked, Host: host}
	}
	return nil, first
}

// controlLoopback returns a net.Dialer.Control that refuses every socket address but loopback.
func controlLoopback() func(network, address string, _ syscall.RawConn) error {
	return func(_, address string, _ syscall.RawConn) error {
		ap, err := netip.ParseAddrPort(address)
		if err != nil || !loopbackAddr(ap.Addr()) {
			return &BlockedError{Reason: ReasonAddressBlocked}
		}
		return nil
	}
}

// loopbackConnect dials one loopback address; the control function refuses any other socket
// address before connect, as a second line behind dialContext.
func loopbackConnect(ctx context.Context, network, address string) (net.Conn, error) {
	dialer := net.Dialer{Control: controlLoopback()} // nosemgrep: bothy-go-egress-only -- the egress package is the one place that builds transports (19 SEC-NET-1)
	return dialer.DialContext(ctx, network, address)
}
