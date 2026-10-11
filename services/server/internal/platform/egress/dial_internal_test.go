// SPDX-License-Identifier: Apache-2.0

package egress

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"sync"
	"testing"
	"time"
)

// TestSocketAddressRecheck exercises the second line of defence: the control function of the
// dialer refuses a socket address that fails the check, even though the dial path only ever
// passes checked addresses.
func TestSocketAddressRecheck(t *testing.T) {
	t.Parallel()
	cases := []struct {
		network, address string
		allowPrivate     bool
		ok               bool
	}{
		{"tcp4", "192.0.2.10:443", false, true},
		{"tcp6", "[2001:db8::10]:443", false, true},
		{"tcp4", "127.0.0.1:443", false, false},
		{"tcp4", "127.0.0.1:443", true, true},
		{"tcp4", "169.254.169.254:80", true, false},
		{"tcp6", "[::ffff:169.254.169.254]:80", true, false},
		{"tcp6", "[fe80::1%en0]:443", true, false},
		{"tcp4", "not-an-address", true, false},
	}
	for _, tc := range cases {
		err := controlFor(tc.allowPrivate)(tc.network, tc.address, nil)
		if tc.ok != (err == nil) {
			t.Errorf("%s %s allow_private=%v: %v", tc.network, tc.address, tc.allowPrivate, err)
		}
		if err != nil && !errors.Is(err, ErrBlocked) {
			t.Errorf("%s: %v is not ErrBlocked", tc.address, err)
		}
	}
	loop := controlLoopback()
	if err := loop("tcp4", "127.0.0.1:8081", nil); err != nil {
		t.Errorf("loopback refused: %v", err)
	}
	if err := loop("tcp4", "192.0.2.10:8081", nil); !errors.Is(err, ErrBlocked) {
		t.Errorf("public address passed the loopback control: %v", err)
	}
}

func TestAddrAllowedRefusesInvalidAddress(t *testing.T) {
	t.Parallel()
	if addrAllowed(netip.Addr{}, true) {
		t.Error("the zero address passed")
	}
}

// acceptCount accepts on ln until it is closed and counts the connections.
func acceptCount(t *testing.T) (string, func() int) {
	t.Helper()
	ln, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	n := 0
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			mu.Lock()
			n++
			mu.Unlock()
			_ = c.Close()
		}
	}()
	t.Cleanup(func() { _ = ln.Close() })
	return ln.Addr().String(), func() int {
		time.Sleep(50 * time.Millisecond)
		mu.Lock()
		defer mu.Unlock()
		return n
	}
}

// TestConnectRechecksSocketAddress: the dialer of the client checks the socket address with the
// decision of the request, so even a dial of an unchecked address never connects.
func TestConnectRechecksSocketAddress(t *testing.T) {
	t.Parallel()
	address, accepted := acceptCount(t)
	refuse := context.WithValue(boundedContext(t), decisionKey{}, decision{host: "x", allowPrivate: false})
	if _, err := connect(refuse, "tcp", address); !errors.Is(err, ErrBlocked) {
		t.Fatalf("connect without allow_private: %v", err)
	}
	if n := accepted(); n != 0 {
		t.Fatalf("%d connections accepted, want 0", n)
	}
	allow := context.WithValue(boundedContext(t), decisionKey{}, decision{host: "x", allowPrivate: true})
	conn, err := connect(allow, "tcp", address)
	if err != nil {
		t.Fatalf("connect with allow_private: %v", err)
	}
	_ = conn.Close()
}

// TestLoopbackConnectRechecksSocketAddress: the dialer of the loopback client refuses a
// non-loopback socket address before connect, so no packet leaves the host.
func TestLoopbackConnectRechecksSocketAddress(t *testing.T) {
	t.Parallel()
	if _, err := loopbackConnect(boundedContext(t), "tcp", "192.0.2.10:8081"); !errors.Is(err, ErrBlocked) {
		t.Fatalf("loopbackConnect to a public address: %v", err)
	}
	address, accepted := acceptCount(t)
	conn, err := loopbackConnect(boundedContext(t), "tcp", address)
	if err != nil {
		t.Fatalf("loopbackConnect to loopback: %v", err)
	}
	_ = conn.Close()
	if n := accepted(); n != 1 {
		t.Fatalf("%d connections accepted, want 1", n)
	}
}

// boundedContext returns a context that ends after five seconds, so that a regression that
// dials for real fails instead of waiting for the connect timeout of the system.
func boundedContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	t.Cleanup(cancel)
	return ctx
}

// TestNewWiresTheCheckingDialer: the dialer New installs checks the socket address with the
// decision of the request.
func TestNewWiresTheCheckingDialer(t *testing.T) {
	t.Parallel()
	c, err := New(Config{Timeout: time.Second, MaxResponseBytes: 1}, Options{Purpose: "test"}, Deps{})
	if err != nil {
		t.Fatal(err)
	}
	address, accepted := acceptCount(t)
	refuse := context.WithValue(boundedContext(t), decisionKey{}, decision{host: "x", allowPrivate: false})
	if _, err := c.dial(refuse, "tcp", address); !errors.Is(err, ErrBlocked) {
		t.Fatalf("dial of an unchecked loopback address: %v", err)
	}
	if n := accepted(); n != 0 {
		t.Fatalf("%d connections accepted, want 0", n)
	}
}

// TestNewLoopbackWiresTheCheckingDialer: the dialer of the loopback client checks the socket
// address.
func TestNewLoopbackWiresTheCheckingDialer(t *testing.T) {
	t.Parallel()
	l := newLoopbackDialer(time.Second, nil)
	if _, err := l.dial(boundedContext(t), "tcp", "192.0.2.10:8081"); !errors.Is(err, ErrBlocked) {
		t.Fatalf("dial of a public address: %v", err)
	}
}
