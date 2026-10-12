// SPDX-License-Identifier: Apache-2.0

package httpserver_test

import (
	"bufio"
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/platform/health"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
)

// fakeListener hands out the server ends of net.Pipe connections whose RemoteAddr the test
// chooses, so a non-loopback peer can be simulated without binding anything but 127.0.0.1
// (PLAN P-10).
type fakeListener struct {
	conns     chan net.Conn
	closed    chan struct{}
	closeOnce sync.Once
}

func newFakeListener() *fakeListener {
	return &fakeListener{conns: make(chan net.Conn), closed: make(chan struct{})}
}

func (l *fakeListener) Accept() (net.Conn, error) {
	select {
	case c := <-l.conns:
		return c, nil
	case <-l.closed:
		return nil, net.ErrClosed
	}
}

func (l *fakeListener) Close() error {
	l.closeOnce.Do(func() { close(l.closed) })
	return nil
}

func (l *fakeListener) Addr() net.Addr { return &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8080} }

// dial queues a connection from peer and returns the client end. peer is "ip:port" or, with
// the prefix "unix:", a Unix socket path.
func (l *fakeListener) dial(t *testing.T, peer string) net.Conn {
	t.Helper()
	server, client := net.Pipe()
	var remote net.Addr
	if path, ok := strings.CutPrefix(peer, "unix:"); ok {
		remote = &net.UnixAddr{Name: path, Net: "unix"}
	} else {
		remote = net.TCPAddrFromAddrPort(netip.MustParseAddrPort(peer))
	}
	select {
	case l.conns <- &peerConn{Conn: server, remote: remote}:
	case <-time.After(5 * time.Second):
		t.Fatal("the server did not accept")
	}
	t.Cleanup(func() { _ = client.Close() })
	return client
}

// peerConn is a pipe end with a chosen remote address.
type peerConn struct {
	net.Conn
	remote net.Addr
}

func (c *peerConn) RemoteAddr() net.Addr { return c.remote }

// counted serves 200 "ok" and counts the requests that reached it.
func counted(n *atomic.Int32) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n.Add(1)
		_, _ = io.WriteString(w, "ok")
	})
}

// serveOn runs an http.Server for h on ln until the test ends.
func serveOn(t *testing.T, ln net.Listener, h http.Handler) {
	t.Helper()
	srv := &http.Server{Handler: h, ReadHeaderTimeout: 5 * time.Second}
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = srv.Serve(ln)
	}()
	t.Cleanup(func() {
		_ = srv.Close()
		<-done
	})
}

const rawRequest = "GET / HTTP/1.1\r\nHost: chat.example.com\r\nConnection: close\r\n\r\n"

// roundTrip writes rawRequest on c and returns the status of the response, or the read error.
func roundTrip(t *testing.T, c net.Conn) (int, error) {
	t.Helper()
	_ = c.SetDeadline(time.Now().Add(5 * time.Second))
	go func() { _, _ = io.WriteString(c, rawRequest) }()
	resp, err := http.ReadResponse(bufio.NewReader(c), nil)
	if err != nil {
		return 0, err
	}
	_ = resp.Body.Close()
	return resp.StatusCode, nil
}

// refused writes a request on c and checks that the connection ends without a single byte.
func refused(t *testing.T, c net.Conn) {
	t.Helper()
	_ = c.SetDeadline(time.Now().Add(5 * time.Second))
	go func() { _, _ = io.WriteString(c, rawRequest) }()
	b, err := io.ReadAll(c)
	if len(b) != 0 {
		t.Fatalf("the refused peer got %d bytes: %q", len(b), b)
	}
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		t.Fatal("the connection stayed open; want it closed at accept time")
	}
}

func TestSECT01_PlaintextFromUntrustedPeerRefused(t *testing.T) {
	t.Parallel()
	var n atomic.Int32
	fl := newFakeListener()
	trusted := []netip.Prefix{netip.MustParsePrefix("198.51.100.0/24")}
	serveOn(t, httpserver.PeerFilter(fl, trusted, "public", slog.New(slog.DiscardHandler)), counted(&n))
	refused(t, fl.dial(t, "192.0.2.10:40000"))
	if n.Load() != 0 {
		t.Fatalf("the handler ran %d times for a refused peer", n.Load())
	}
}

func TestSECT01_PlaintextFromTrustedProxyAccepted(t *testing.T) {
	t.Parallel()
	var n atomic.Int32
	fl := newFakeListener()
	trusted := []netip.Prefix{netip.MustParsePrefix("198.51.100.0/24")}
	serveOn(t, httpserver.PeerFilter(fl, trusted, "public", slog.New(slog.DiscardHandler)), counted(&n))
	status, err := roundTrip(t, fl.dial(t, "198.51.100.7:40000"))
	if err != nil || status != http.StatusOK || n.Load() != 1 {
		t.Fatalf("trusted proxy: status %d, error %v, handler ran %d times", status, err, n.Load())
	}
}

func TestSECT01_LoopbackAccepted(t *testing.T) {
	t.Parallel()
	var n atomic.Int32
	ln, err := httpserver.ListenPublic(context.Background(), httpserver.TransportConfig{
		ListenAddr: "127.0.0.1:0", PublicURL: mustURL(t, "http://127.0.0.1:8080"),
	}, nil, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	serveOn(t, ln, counted(&n))
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://"+ln.Addr().String()+"/", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("real loopback listener: status %d", resp.StatusCode)
	}

	// No trusted proxies at all: loopback still counts, also IPv6 and IPv4-mapped.
	fl := newFakeListener()
	serveOn(t, httpserver.PeerFilter(fl, nil, "public", slog.New(slog.DiscardHandler)), counted(&n))
	for _, peer := range []string{"[::1]:40000", "[::ffff:127.0.0.1]:40000", "127.0.0.2:40000"} {
		if status, err := roundTrip(t, fl.dial(t, peer)); err != nil || status != http.StatusOK {
			t.Errorf("%s: status %d, error %v", peer, status, err)
		}
	}
}

func TestSECT01_MappedUntrustedRefused(t *testing.T) {
	t.Parallel()
	var n atomic.Int32
	fl := newFakeListener()
	serveOn(t, httpserver.PeerFilter(fl, []netip.Prefix{netip.MustParsePrefix("198.51.100.0/24")}, "public", slog.New(slog.DiscardHandler)), counted(&n))
	refused(t, fl.dial(t, "[::ffff:192.0.2.10]:40000"))
	refused(t, fl.dial(t, "unix:/run/bothy/api.sock"))
	if n.Load() != 0 {
		t.Fatalf("the handler ran %d times", n.Load())
	}
}

func TestSECT01_TrustedMappedAndIPv6Accepted(t *testing.T) {
	t.Parallel()
	var n atomic.Int32
	fl := newFakeListener()
	trusted := []netip.Prefix{netip.MustParsePrefix("198.51.100.7/32"), netip.MustParsePrefix("2001:db8::/32")}
	serveOn(t, httpserver.PeerFilter(fl, trusted, "public", slog.New(slog.DiscardHandler)), counted(&n))
	for _, peer := range []string{"[::ffff:198.51.100.7]:40000", "[2001:db8::7]:40000"} {
		if status, err := roundTrip(t, fl.dial(t, peer)); err != nil || status != http.StatusOK {
			t.Errorf("%s: status %d, error %v", peer, status, err)
		}
	}
	refused(t, fl.dial(t, "[2001:db9::7]:40000"))
}

func TestSECT01_RefusalKeepsAccepting(t *testing.T) {
	t.Parallel()
	var n atomic.Int32
	fl := newFakeListener()
	trusted := []netip.Prefix{netip.MustParsePrefix("198.51.100.0/24")}
	serveOn(t, httpserver.PeerFilter(fl, trusted, "public", slog.New(slog.DiscardHandler)), counted(&n))
	refused(t, fl.dial(t, "192.0.2.10:40000"))
	status, err := roundTrip(t, fl.dial(t, "198.51.100.7:40000"))
	if err != nil || status != http.StatusOK || n.Load() != 1 {
		t.Fatalf("after a refusal: status %d, error %v, handler ran %d times", status, err, n.Load())
	}
}

func TestSECT01_RefusalLogHasNoAddress(t *testing.T) {
	t.Parallel()
	var buf syncBuffer
	log := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	var n atomic.Int32
	fl := newFakeListener()
	serveOn(t, httpserver.PeerFilter(fl, nil, "public", log), counted(&n))
	for range 3 {
		refused(t, fl.dial(t, "192.0.2.10:40000"))
	}
	// The third refusal is logged before the fourth Accept returns; one more trusted connection
	// makes sure the filter is past it.
	if _, err := roundTrip(t, fl.dial(t, "127.0.0.1:40000")); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if strings.Contains(out, "192.0.2.10") {
		t.Fatalf("a log line contains the peer address:\n%s", out)
	}
	const hint = "add the address of the reverse proxy as seen by the server to server.trusted_proxies"
	if strings.Count(out, hint) != 1 || strings.Count(out, `"level":"WARN"`) != 1 {
		t.Fatalf("want the hint once at Warn:\n%s", out)
	}
	if strings.Count(out, `"level":"DEBUG"`) != 2 || strings.Count(out, `"bothy.reason":"public"`) != 3 {
		t.Fatalf("want two later refusals at Debug and the listener name:\n%s", out)
	}
}

func TestSECT01_InternalListenerNotFiltered(t *testing.T) {
	t.Parallel()
	reg := health.NewRegistry()
	fl := newFakeListener()
	// The internal listener is opened with Listen and served without PeerFilter (R-SE-09).
	serveOn(t, fl, httpserver.InternalHandler(httpserver.Internal(reg), httpserver.SecurityHeadersFor(mustURL(t, "https://chat.example.com"), time.Hour)))
	c := fl.dial(t, "192.0.2.10:40000")
	_ = c.SetDeadline(time.Now().Add(5 * time.Second))
	go func() { _, _ = io.WriteString(c, "GET /healthz HTTP/1.1\r\nHost: x\r\nConnection: close\r\n\r\n") }()
	resp, err := http.ReadResponse(bufio.NewReader(c), nil)
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("internal /healthz from an untrusted peer: %v, %v", resp, err)
	}
	_ = resp.Body.Close()
}
