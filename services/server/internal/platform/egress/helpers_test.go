// SPDX-License-Identifier: Apache-2.0

package egress_test

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/platform/egress"
)

// fakeResolver answers from a map and counts its calls; an unknown name is "not found". With
// block set it waits until the context ends, like a resolver that never answers.
type fakeResolver struct {
	answers map[string][]netip.Addr
	block   bool
	calls   atomic.Int64
	ended   chan struct{} // closed when a blocked lookup returns
	once    sync.Once
}

func newResolver(answers map[string][]netip.Addr) *fakeResolver {
	return &fakeResolver{answers: answers, ended: make(chan struct{})}
}

func (r *fakeResolver) LookupNetIP(ctx context.Context, network, host string) ([]netip.Addr, error) {
	r.calls.Add(1)
	if r.block {
		<-ctx.Done()
		r.once.Do(func() { close(r.ended) })
		return nil, ctx.Err()
	}
	if network != "ip" {
		return nil, fmt.Errorf("fake resolver: unexpected network %q", network)
	}
	addrs, ok := r.answers[host]
	if !ok {
		return nil, &net.DNSError{Err: "no such host", Name: host, IsNotFound: true}
	}
	return addrs, nil
}

// addrs parses address literals for resolver answers.
func addrs(texts ...string) []netip.Addr {
	out := make([]netip.Addr, 0, len(texts))
	for _, s := range texts {
		out = append(out, netip.MustParseAddr(s))
	}
	return out
}

// firstHost returns the first host address of a prefix. Private IPv4 host addresses are derived
// this way and never written as literals: the white-label rule tools/denylist/generic.rules
// flags them in every tracked file.
func firstHost(prefix string) netip.Addr { return netip.MustParsePrefix(prefix).Addr().Next() }

var loopback4 = netip.MustParseAddr("127.0.0.1")

// server is an httptest server on 127.0.0.1:0 that counts the requests it receives.
type server struct {
	*httptest.Server
	count atomic.Int64
}

func newServer(t *testing.T, h http.HandlerFunc) *server {
	t.Helper()
	s := &server{}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.count.Add(1)
		h(w, r)
	}))
	t.Cleanup(s.Close)
	return s
}

func newTLSServer(t *testing.T, h http.HandlerFunc) *server {
	t.Helper()
	s := &server{}
	s.Server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.count.Add(1)
		h(w, r)
	}))
	t.Cleanup(s.Close)
	return s
}

func ok(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) }

// port returns the port of the server.
func (s *server) port(t *testing.T) uint16 {
	t.Helper()
	u, err := url.Parse(s.URL)
	if err != nil {
		t.Fatal(err)
	}
	p, err := strconv.ParseUint(u.Port(), 10, 16)
	if err != nil {
		t.Fatal(err)
	}
	return uint16(p)
}

// syncBuffer is a bytes.Buffer safe for the dial goroutines of the transport.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// recordedMetrics records every blocked attempt.
type recordedMetrics struct {
	mu      sync.Mutex
	reasons []egress.Reason
}

func (m *recordedMetrics) Blocked(_ context.Context, reason egress.Reason) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.reasons = append(m.reasons, reason)
}

func (m *recordedMetrics) got() []egress.Reason {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]egress.Reason(nil), m.reasons...)
}

// harness is a client with its fakes and counters.
type harness struct {
	client  *egress.Client
	res     *fakeResolver
	dials   atomic.Int64
	dialed  []string
	mu      sync.Mutex
	log     *syncBuffer
	metrics *recordedMetrics
}

// errIntercepted replaces a dial in tests that must not reach the network.
var errIntercepted = errors.New("test: dial intercepted")

type setup struct {
	rules     []egress.Rule
	policy    egress.HostPolicy
	answers   map[string][]netip.Addr
	redirects egress.RedirectMode
	timeout   time.Duration
	maxBytes  int64
	opts      egress.Options // Timeout and MaxResponseBytes of the consumer
	roots     bool           // RootCAs from tlsServer
	tlsServer *server
	intercept bool // every dial fails with errIntercepted instead of connecting
	denyAll   bool // Deps.DenyAll
}

func newHarness(t *testing.T, s setup) *harness {
	t.Helper()
	h := &harness{res: newResolver(s.answers), log: &syncBuffer{}, metrics: &recordedMetrics{}}
	if s.timeout == 0 {
		s.timeout = 5 * time.Second
	}
	if s.maxBytes == 0 {
		s.maxBytes = 1 << 20
	}
	opts := s.opts
	opts.Purpose = "test"
	opts.Hosts = s.policy
	opts.Redirects = s.redirects
	deps := egress.Deps{
		Resolver: h.res,
		Logger:   slog.New(slog.NewJSONHandler(h.log, nil)),
		Metrics:  h.metrics,
		DenyAll:  s.denyAll,
	}
	if s.roots {
		pool := x509CertPool(t, s.tlsServer)
		deps.RootCAs = pool
	}
	c, err := egress.New(egress.Config{AllowedHosts: s.rules, Timeout: s.timeout, MaxResponseBytes: s.maxBytes}, opts, deps)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	egress.OnDial(c, func(_ context.Context, _, address string) error {
		h.dials.Add(1)
		h.mu.Lock()
		h.dialed = append(h.dialed, address)
		h.mu.Unlock()
		if s.intercept {
			return errIntercepted
		}
		return nil
	})
	h.client = c
	return h
}

func (h *harness) addresses() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]string(nil), h.dialed...)
}

// get sends a GET through Do.
func (h *harness) get(t *testing.T, rawURL string) (*http.Response, error) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, rawURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := h.client.Do(req)
	if err != nil && resp != nil {
		t.Errorf("Do returned a response with error %v", err)
	}
	return resp, err
}

// reply is a response read to the end and closed.
type reply struct {
	status int
	header http.Header
	text   string
	tls    *tls.ConnectionState
	url    *url.URL
}

// fetch sends a GET through Do and reads and closes the response.
func (h *harness) fetch(t *testing.T, rawURL string) (reply, error) {
	t.Helper()
	resp, err := h.get(t, rawURL)
	if err != nil {
		return reply{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	var b bytes.Buffer
	if _, err := b.ReadFrom(resp.Body); err != nil {
		t.Fatalf("read body: %v", err)
	}
	return reply{status: resp.StatusCode, header: resp.Header, text: b.String(), tls: resp.TLS, url: resp.Request.URL}, nil
}

// err sends a GET through Do for a test that expects an error, and closes a response if one
// comes back anyway.
func (h *harness) err(t *testing.T, rawURL string) error {
	t.Helper()
	resp, err := h.get(t, rawURL)
	if resp != nil {
		_ = resp.Body.Close()
		if err == nil {
			t.Errorf("%s: got status %d, want an error", rawURL, resp.StatusCode)
		}
	}
	return err
}

// wantBlocked asserts that err is a BlockedError with the reason.
func wantBlocked(t *testing.T, err error, reason egress.Reason) {
	t.Helper()
	if !errors.Is(err, egress.ErrBlocked) {
		t.Fatalf("error %v is not ErrBlocked", err)
	}
	var be *egress.BlockedError
	if !errors.As(err, &be) {
		t.Fatalf("error %v is not a *BlockedError", err)
	}
	if be.Reason != reason {
		t.Fatalf("reason %q, want %q (error %v)", be.Reason, reason, err)
	}
}

// x509CertPool returns a pool with the certificate of a TLS test server.
func x509CertPool(t *testing.T, s *server) *x509.CertPool {
	t.Helper()
	if s == nil {
		t.Fatal("roots without tlsServer")
	}
	pool := x509.NewCertPool()
	pool.AddCert(s.Certificate())
	return pool
}
