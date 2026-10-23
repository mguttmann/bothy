// SPDX-License-Identifier: Apache-2.0

// Package fakeapp is an HTTPS fake of an app's interaction address for tests (M2-25 section
// "Tests"): it listens only on 127.0.0.1 with a port the system chooses, presents a certificate of
// a test certificate authority created in memory for the names it serves (tracker.example.com by
// default), verifies the v1a signature of every request with the given Ed25519 public keys (those
// of getWebhookKeys in integration tests), records each request in memory and answers by script.
// The egress client reaches it through Deps.RootCAs = App.RootCAs and Deps.Resolver = Resolver,
// which maps the names to 127.0.0.1, and an egress.allowed_hosts entry with allow_private.
//
// Nothing of a request is ever printed: tests compare bodies and headers in memory (M2-25
// Boundaries; SEC-LOG-2).
//
// Example:
//
//	app := fakeapp.Start(t, keys)
//	app.Respond(fakeapp.Response{Status: http.StatusAccepted})
//	deps := egress.Deps{Resolver: fakeapp.NewResolver(fakeapp.DefaultHost), RootCAs: app.RootCAs}
//	// ... the code under test posts to app.URL("/hooks/interactions") ...
//	got := app.Requests() // got[0].Verified
package fakeapp

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// DefaultHost is the name the fake serves when Start gets no names (the app Tracker of 07 11.1).
const DefaultHost = "tracker.example.com"

// The headers of Standard Webhooks the fake reads (webhooks.md section 2).
const (
	headerID        = "webhook-id"
	headerTimestamp = "webhook-timestamp"
	headerSignature = "webhook-signature"
)

// Response is one scripted answer.
type Response struct {
	// Status is the HTTP status; 0 means 200.
	Status int
	// Body is written as it is, with Content-Type application/json when not empty.
	Body []byte
	// Location is the Location header of a redirect; empty for none.
	Location string
	// Delay waits this long before the answer; the wait ends early when the client gives up (the
	// request context ends), so a slow answer never outlives the request.
	Delay time.Duration
	// Hang waits until the client gives up and then answers nothing: a deterministic answer that
	// arrives after any deadline.
	Hang bool
}

// Request is one recorded request.
type Request struct {
	// Method, Path and Host are those of the request line and Host header.
	Method, Path, Host string
	// Header is a copy of the request headers.
	Header http.Header
	// Body is the request body as received.
	Body []byte
	// Verified reports that webhook-id, webhook-timestamp and at least one v1a entry are present
	// and a v1a entry verifies with one of the keys over "{webhook-id}.{webhook-timestamp}.{body}".
	Verified bool
	// HasV1 reports a v1 (HMAC) entry in webhook-signature.
	HasV1 bool
}

// App is a running fake. It is safe for concurrent use.
type App struct {
	// RootCAs trusts the test certificate authority of this fake only.
	RootCAs *x509.CertPool
	// Port is the TCP port on 127.0.0.1.
	Port uint16

	srv  *httptest.Server
	keys []ed25519.PublicKey
	done chan struct{}

	mu      sync.Mutex
	reqs    []Request
	queue   []Response
	handler func(Request) Response
	changed chan struct{}
}

// Start starts a fake that serves names (DefaultHost when none) and verifies v1a with keys. The
// fake stops at the end of the test. It fails the test when the certificate or the listener
// cannot be created.
func Start(t testing.TB, keys []ed25519.PublicKey, names ...string) *App {
	t.Helper()
	if len(names) == 0 {
		names = []string{DefaultHost}
	}
	cert, pool, err := newCertificate(names)
	if err != nil {
		t.Fatalf("fakeapp: certificate: %v", err)
	}
	ln, err := new(net.ListenConfig).Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("fakeapp: listen: %v", err)
	}
	a := &App{
		RootCAs: pool, Port: ln.Addr().(*net.TCPAddr).AddrPort().Port(), keys: append([]ed25519.PublicKey(nil), keys...),
		done: make(chan struct{}), changed: make(chan struct{}),
	}
	a.srv = httptest.NewUnstartedServer(a)
	_ = a.srv.Listener.Close()
	a.srv.Listener = ln
	a.srv.TLS = &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12}
	a.srv.StartTLS()
	t.Cleanup(func() {
		close(a.done) // releases every waiting handler before Close waits for them
		a.srv.Close()
	})
	return a
}

// URL returns the https address of path on the first name and port of the fake, for example
// https://tracker.example.com:<port>/hooks/interactions.
func (a *App) URL(path string) string { return a.URLFor(DefaultHost, path) }

// URLFor returns the https address of path on host and the port of the fake.
func (a *App) URLFor(host, path string) string {
	return "https://" + host + ":" + strconv.Itoa(int(a.Port)) + path
}

// HostPort returns host:<port>, the form of a webhook.allowed_hosts entry.
func (a *App) HostPort(host string) string { return host + ":" + strconv.Itoa(int(a.Port)) }

// Respond appends answers to the queue; each request takes the first. With an empty queue the
// handler of Handle answers, without one 200 with an empty body.
func (a *App) Respond(rs ...Response) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.queue = append(a.queue, rs...)
}

// Handle sets the function that answers a request when the queue is empty; nil removes it. The
// function gets the recorded request.
func (a *App) Handle(f func(Request) Response) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.handler = f
}

// SetKeys replaces the keys v1a is verified with: a test that starts the fake before the server
// (the manifest names the fake's port) sets the keys of getWebhookKeys afterwards.
func (a *App) SetKeys(keys []ed25519.PublicKey) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.keys = append([]ed25519.PublicKey(nil), keys...)
}

// Requests returns a copy of the recorded requests in arrival order.
func (a *App) Requests() []Request {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]Request(nil), a.reqs...)
}

// WaitRequests waits until at least n requests are recorded or ctx ends, and returns the recorded
// requests. It never sleeps a fixed time: every new request wakes it.
func (a *App) WaitRequests(ctx context.Context, n int) ([]Request, error) {
	for {
		a.mu.Lock()
		if len(a.reqs) >= n {
			out := append([]Request(nil), a.reqs...)
			a.mu.Unlock()
			return out, nil
		}
		ch := a.changed
		a.mu.Unlock()
		select {
		case <-ch:
		case <-ctx.Done():
			return a.Requests(), ctx.Err()
		}
	}
}

// ServeHTTP records the request, verifies it and answers by script.
func (a *App) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	rec := Request{Method: r.Method, Path: r.URL.Path, Host: r.Host, Header: r.Header.Clone(), Body: body}
	a.mu.Lock()
	keys := a.keys
	a.mu.Unlock()
	rec.Verified, rec.HasV1 = verify(keys, r.Header, body)
	a.mu.Lock()
	a.reqs = append(a.reqs, rec)
	close(a.changed)
	a.changed = make(chan struct{})
	resp := Response{}
	switch {
	case len(a.queue) > 0:
		resp, a.queue = a.queue[0], a.queue[1:]
	case a.handler != nil:
		h := a.handler
		a.mu.Unlock()
		resp = h(rec)
		a.mu.Lock()
	}
	a.mu.Unlock()
	a.answer(w, r, resp)
}

// answer writes resp after its delay.
func (a *App) answer(w http.ResponseWriter, r *http.Request, resp Response) {
	if resp.Hang {
		select {
		case <-r.Context().Done():
		case <-a.done:
		}
		return
	}
	if resp.Delay > 0 {
		t := time.NewTimer(resp.Delay)
		defer t.Stop()
		select {
		case <-t.C:
		case <-r.Context().Done():
			return
		case <-a.done:
			return
		}
	}
	if resp.Location != "" {
		w.Header().Set("Location", resp.Location)
	}
	if len(resp.Body) > 0 {
		w.Header().Set("Content-Type", "application/json")
	}
	status := resp.Status
	if status == 0 {
		status = http.StatusOK
	}
	w.WriteHeader(status)
	_, _ = w.Write(resp.Body)
}

// verify checks webhook-signature of a request against keys the way a receiver does
// (webhooks.md section 3): the signed content is "{webhook-id}.{webhook-timestamp}.{body}", the
// header holds entries "<scheme>,<base64>" separated by spaces, and one v1a entry must verify
// with one key. It reports the result and whether a v1 entry is present.
func verify(keys []ed25519.PublicKey, h http.Header, body []byte) (verified, hasV1 bool) {
	id, ts, sig := h.Get(headerID), h.Get(headerTimestamp), h.Get(headerSignature)
	content := append([]byte(id+"."+ts+"."), body...)
	for _, entry := range strings.Fields(sig) {
		scheme, value, ok := strings.Cut(entry, ",")
		if !ok {
			continue
		}
		if scheme == "v1" {
			hasV1 = true
			continue
		}
		raw, err := base64.StdEncoding.DecodeString(value)
		if scheme != "v1a" || err != nil || id == "" || ts == "" {
			continue
		}
		for _, k := range keys {
			if len(k) == ed25519.PublicKeySize && ed25519.Verify(k, content, raw) {
				verified = true
			}
		}
	}
	return verified, hasV1
}

// Oversized returns a 200 whose body is valid JSON of more than n bytes, for the bound
// http.max_json_body_bytes.
func Oversized(n int) Response {
	pad := bytes.Repeat([]byte("a"), n)
	return Response{Status: http.StatusOK, Body: append(append([]byte(`{"pad":"`), pad...), `"}`...)}
}

// InvalidJSON returns a 200 whose body is no JSON.
func InvalidJSON() Response {
	return Response{Status: http.StatusOK, Body: []byte(`{"client_msg_id":`)}
}

// Redirect returns a 302 to location.
func Redirect(location string) Response {
	return Response{Status: http.StatusFound, Location: location}
}

// Resolver is the name resolution of the egress client in tests (egress.Resolver): it answers its
// names with their addresses and every other name with "not found". It is safe for concurrent use.
type Resolver struct {
	mu sync.Mutex
	m  map[string][]netip.Addr
}

// NewResolver returns a resolver that maps every name in names to 127.0.0.1.
func NewResolver(names ...string) *Resolver {
	r := &Resolver{m: map[string][]netip.Addr{}}
	for _, n := range names {
		r.m[strings.ToLower(n)] = []netip.Addr{netip.MustParseAddr("127.0.0.1")}
	}
	return r
}

// Set answers host with addrs from now on, for example a private address for the address check.
func (r *Resolver) Set(host string, addrs ...netip.Addr) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.m[strings.ToLower(host)] = append([]netip.Addr(nil), addrs...)
}

// LookupNetIP implements egress.Resolver.
func (r *Resolver) LookupNetIP(_ context.Context, _, host string) ([]netip.Addr, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if a, ok := r.m[strings.ToLower(host)]; ok {
		return append([]netip.Addr(nil), a...), nil
	}
	return nil, &net.DNSError{Err: "no such host", Name: host, IsNotFound: true}
}

// newCertificate creates a test certificate authority in memory and a server certificate for
// names and 127.0.0.1 signed by it; the pool trusts only that authority.
func newCertificate(names []string) (tls.Certificate, *x509.CertPool, error) {
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, nil, err
	}
	now := time.Now()
	ca := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "fakeapp test authority"},
		NotBefore: now.Add(-time.Hour), NotAfter: now.Add(24 * time.Hour), IsCA: true, BasicConstraintsValid: true,
		KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, ca, ca, &caKey.PublicKey, caKey)
	if err != nil {
		return tls.Certificate{}, nil, err
	}
	caCert, err := x509.ParseCertificate(caDER)
	if err != nil {
		return tls.Certificate{}, nil, err
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, nil, err
	}
	leaf := &x509.Certificate{
		SerialNumber: big.NewInt(2), Subject: pkix.Name{CommonName: names[0]}, DNSNames: names,
		IPAddresses: []net.IP{net.IPv4(127, 0, 0, 1)}, NotBefore: now.Add(-time.Hour), NotAfter: now.Add(24 * time.Hour),
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, leaf, caCert, &key.PublicKey, caKey)
	if err != nil {
		return tls.Certificate{}, nil, err
	}
	pool := x509.NewCertPool()
	pool.AddCert(caCert)
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}, pool, nil
}
