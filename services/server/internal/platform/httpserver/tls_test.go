// SPDX-License-Identifier: Apache-2.0

package httpserver_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"log/slog"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
	"github.com/OWNER/bothy/services/server/internal/platform/secrets"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
)

// mustURL parses s or stops the test.
func mustURL(t *testing.T, s string) *url.URL {
	t.Helper()
	u, err := url.Parse(s)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

// testCA is a certificate authority generated in the test; nothing is read from disk.
type testCA struct {
	cert *x509.Certificate
	key  *ecdsa.PrivateKey
	pool *x509.CertPool
}

func newTestCA(t *testing.T) *testCA {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "Test CA"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(24 * time.Hour),
		IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	pool := x509.NewCertPool()
	pool.AddCert(cert)
	return &testCA{cert: cert, key: key, pool: pool}
}

// leafOpts shape a leaf certificate.
type leafOpts struct {
	serial    int64
	host      string // a DNS name or an IP literal
	notBefore time.Time
	notAfter  time.Time
}

// leaf returns the PEM chain (leaf, then CA) and the PEM private key of a new leaf.
func (ca *testCA) leaf(t *testing.T, o leafOpts) (chainPEM, keyPEM []byte) {
	t.Helper()
	if o.notBefore.IsZero() {
		o.notBefore = time.Now().Add(-time.Hour)
	}
	if o.notAfter.IsZero() {
		o.notAfter = time.Now().Add(24 * time.Hour)
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(o.serial), Subject: pkix.Name{CommonName: o.host},
		NotBefore: o.notBefore, NotAfter: o.notAfter,
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	if ip := net.ParseIP(o.host); ip != nil {
		tmpl.IPAddresses = []net.IP{ip}
	} else {
		tmpl.DNSNames = []string{o.host}
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, ca.cert, &key.PublicKey, ca.key)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	chainPEM = append(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
		pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: ca.cert.Raw})...)
	return chainPEM, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
}

// tlsFixture holds the secret files, the provider and the transport configuration of one test.
type tlsFixture struct {
	dir      string
	clock    *clock.Fake
	provider *secrets.Provider
	cfg      httpserver.TransportConfig
}

// newTLSFixture writes chain and key with mode 0600 into a fresh directory and references them
// as file: secrets through a provider with a fake clock and a cache TTL of one minute.
func newTLSFixture(t *testing.T, chain, key []byte, publicURL string, log *slog.Logger) *tlsFixture {
	t.Helper()
	f := &tlsFixture{dir: t.TempDir(), clock: clock.NewFake(time.Now())}
	f.write(t, "tls_cert", chain)
	f.write(t, "tls_key", key)
	p, err := secrets.NewProvider(secrets.Options{Dir: f.dir, CacheTTL: time.Minute, Clock: f.clock, Logger: log})
	if err != nil {
		t.Fatal(err)
	}
	f.provider = p
	f.cfg = httpserver.TransportConfig{
		ListenAddr: "127.0.0.1:0", PublicURL: mustURL(t, publicURL),
		TLSCertRef: secrets.Ref{Scheme: "file", Path: filepath.Join(f.dir, "tls_cert")},
		TLSKeyRef:  secrets.Ref{Scheme: "file", Path: filepath.Join(f.dir, "tls_key")},
	}
	return f
}

// write replaces the secret file name with content, mode 0600.
func (f *tlsFixture) write(t *testing.T, name string, content []byte) {
	t.Helper()
	path := filepath.Join(f.dir, name)
	tmp := path + ".new"
	if err := os.WriteFile(tmp, content, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(tmp, path); err != nil {
		t.Fatal(err)
	}
}

// listen opens the TLS listener of the fixture and serves 200 "ok" on it.
func (f *tlsFixture) listen(t *testing.T, log *slog.Logger) net.Listener {
	t.Helper()
	ln, err := httpserver.ListenPublic(context.Background(), f.cfg, f.provider, log)
	if err != nil {
		t.Fatal(err)
	}
	var n atomic.Int32
	serveOn(t, ln, counted(&n))
	return ln
}

// handshake dials addr with cfg and returns the connection state, or the handshake error.
func handshake(t *testing.T, addr string, cfg *tls.Config) (tls.ConnectionState, error) {
	t.Helper()
	d := &tls.Dialer{NetDialer: &net.Dialer{Timeout: 5 * time.Second}, Config: cfg}
	ctx, cancel := context.WithTimeout(context.Background(), testkit.Patience)
	defer cancel()
	c, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return tls.ConnectionState{}, err
	}
	defer func() { _ = c.Close() }()
	return c.(*tls.Conn).ConnectionState(), nil
}

func TestSECT17_TLSBelow12Refused(t *testing.T) {
	t.Parallel()
	ca := newTestCA(t)
	chain, key := ca.leaf(t, leafOpts{serial: 2, host: "127.0.0.1"})
	f := newTLSFixture(t, chain, key, "https://127.0.0.1:8443", slog.New(slog.DiscardHandler))
	ln := f.listen(t, slog.New(slog.DiscardHandler))
	for _, v := range []uint16{tls.VersionTLS10, tls.VersionTLS11} {
		st, err := handshake(t, ln.Addr().String(), &tls.Config{RootCAs: ca.pool, ServerName: "127.0.0.1", MinVersion: v, MaxVersion: v})
		if err == nil {
			t.Fatalf("handshake with version %x succeeded (negotiated %x)", v, st.Version)
		}
	}
}

func TestSECT17_TLS12And13(t *testing.T) {
	t.Parallel()
	ca := newTestCA(t)
	chain, key := ca.leaf(t, leafOpts{serial: 2, host: "127.0.0.1"})
	f := newTLSFixture(t, chain, key, "https://127.0.0.1:8443", slog.New(slog.DiscardHandler))
	ln := f.listen(t, slog.New(slog.DiscardHandler))
	st, err := handshake(t, ln.Addr().String(), &tls.Config{RootCAs: ca.pool, MaxVersion: tls.VersionTLS12}) //nolint:gosec // G402: SEC-T17 limits the client to TLS 1.2 on purpose.
	if err != nil || st.Version != tls.VersionTLS12 {
		t.Fatalf("TLS 1.2 client: version %x, error %v", st.Version, err)
	}
	st, err = handshake(t, ln.Addr().String(), &tls.Config{RootCAs: ca.pool})
	if err != nil || st.Version != tls.VersionTLS13 {
		t.Fatalf("default client: version %x, error %v", st.Version, err)
	}
	cfg := httpserver.TLSConfigOf(ln)
	if cfg == nil || cfg.MinVersion != tls.VersionTLS12 || cfg.MaxVersion != 0 || cfg.CipherSuites != nil ||
		cfg.CurvePreferences != nil || cfg.ClientAuth != tls.NoClientCert || cfg.GetCertificate == nil {
		t.Fatalf("server TLS config %+v: want MinVersion 1.2 and the Go defaults otherwise", cfg)
	}
	// A real request over TLS, as an app would send it.
	client := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: ca.pool}}}
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://"+ln.Addr().String()+"/", nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("request over TLS: status %d", resp.StatusCode)
	}
}

func TestTLS_StartFailsOnBadMaterial(t *testing.T) {
	t.Parallel()
	ca := newTestCA(t)
	chain, key := ca.leaf(t, leafOpts{serial: 2, host: "chat.example.com"})
	_, otherKey := ca.leaf(t, leafOpts{serial: 3, host: "chat.example.com"})
	expired, expiredKey := ca.leaf(t, leafOpts{serial: 4, host: "chat.example.com",
		notBefore: time.Now().Add(-48 * time.Hour), notAfter: time.Now().Add(-time.Hour)})
	future, futureKey := ca.leaf(t, leafOpts{serial: 5, host: "chat.example.com",
		notBefore: time.Now().Add(time.Hour), notAfter: time.Now().Add(48 * time.Hour)})
	otherHost, otherHostKey := ca.leaf(t, leafOpts{serial: 6, host: "other.example.com"})
	const public = "https://chat.example.com"
	cases := []struct {
		name      string
		chain     []byte
		key       []byte
		publicURL string
		edit      func(c *httpserver.TransportConfig)
		want      []string
	}{
		{"only cert", chain, key, public, func(c *httpserver.TransportConfig) { c.TLSKeyRef = secrets.Ref{} }, []string{"server.tls_cert_ref", "server.tls_key_ref"}},
		{"only key", chain, key, public, func(c *httpserver.TransportConfig) { c.TLSCertRef = secrets.Ref{} }, []string{"server.tls_cert_ref", "server.tls_key_ref"}},
		{"mismatched pair", chain, otherKey, public, nil, []string{"server.tls_cert_ref", "server.tls_key_ref", "do not match"}},
		{"expired leaf", expired, expiredKey, public, nil, []string{"server.tls_cert_ref", "expired"}},
		{"leaf not yet valid", future, futureKey, public, nil, []string{"server.tls_cert_ref", "not yet valid"}},
		{"leaf without the host", otherHost, otherHostKey, public, nil, []string{"server.tls_cert_ref", "server.public_url"}},
		{"http public URL", chain, key, "http://chat.example.com", nil, []string{"server.public_url", "https"}},
		{"cert not readable", chain, key, public, func(c *httpserver.TransportConfig) { c.TLSCertRef.Path += ".missing" }, []string{"server.tls_cert_ref", "not_found"}},
		{"key not readable", chain, key, public, func(c *httpserver.TransportConfig) { c.TLSKeyRef.Path += ".missing" }, []string{"server.tls_key_ref", "not_found"}},
		{"no PEM", []byte("garbage"), key, public, nil, []string{"server.tls_cert_ref", "PEM"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			f := newTLSFixture(t, c.chain, c.key, c.publicURL, slog.New(slog.DiscardHandler))
			if c.edit != nil {
				c.edit(&f.cfg)
			}
			errCheck := httpserver.CheckTransport(context.Background(), f.cfg, f.provider)
			ln, err := httpserver.ListenPublic(context.Background(), f.cfg, f.provider, slog.New(slog.DiscardHandler))
			if err == nil {
				_ = ln.Close()
				t.Fatal("ListenPublic succeeded")
			}
			for _, e := range []error{errCheck, err} {
				if e == nil {
					t.Fatal("CheckTransport succeeded")
				}
				msg := e.Error()
				for _, w := range c.want {
					if !strings.Contains(msg, w) {
						t.Errorf("error %q does not name %q", msg, w)
					}
				}
				if strings.Contains(msg, "BEGIN") || strings.Contains(msg, "garbage") || strings.Contains(msg, string(c.key[40:60])) {
					t.Errorf("error %q contains PEM content", msg)
				}
			}
		})
	}
}

func TestTLS_PlaintextNeedsNoProvider(t *testing.T) {
	t.Parallel()
	cfg := httpserver.TransportConfig{ListenAddr: "127.0.0.1:0", PublicURL: mustURL(t, "https://chat.example.com")}
	if err := httpserver.CheckTransport(context.Background(), cfg, nil); err != nil {
		t.Fatalf("plaintext behind a TLS proxy: %v", err)
	}
	if httpserver.TLSEnabled(cfg) {
		t.Fatal("no references, yet TLS reported on")
	}
}

func TestTLS_CertificateRotation(t *testing.T) {
	t.Parallel()
	var logs syncBuffer
	log := slog.New(slog.NewJSONHandler(&logs, nil))
	ca := newTestCA(t)
	chain, key := ca.leaf(t, leafOpts{serial: 100, host: "127.0.0.1"})
	f := newTLSFixture(t, chain, key, "https://127.0.0.1:8443", log)
	ln := f.listen(t, log)
	serial := func() int64 {
		t.Helper()
		st, err := handshake(t, ln.Addr().String(), &tls.Config{RootCAs: ca.pool})
		if err != nil {
			t.Fatal(err)
		}
		return st.PeerCertificates[0].SerialNumber.Int64()
	}
	if got := serial(); got != 100 {
		t.Fatalf("serial %d, want 100", got)
	}

	// The new pair is written; until the provider cache expires the old one stays.
	chain2, key2 := ca.leaf(t, leafOpts{serial: 200, host: "127.0.0.1"})
	f.write(t, "tls_cert", chain2)
	f.write(t, "tls_key", key2)
	if got := serial(); got != 100 {
		t.Fatalf("within the cache TTL: serial %d, want 100", got)
	}
	f.clock.Advance(2 * time.Minute)
	if got := serial(); got != 200 {
		t.Fatalf("after the cache TTL: serial %d, want 200", got)
	}

	// A broken replacement keeps the previous certificate and logs one error for that version,
	// however many handshakes see it.
	f.write(t, "tls_cert", []byte("-----BEGIN CERTIFICATE-----\nbroken\n-----END CERTIFICATE-----\n"))
	f.clock.Advance(2 * time.Minute)
	for range 3 {
		if got := serial(); got != 200 {
			t.Fatalf("after a broken rotation: serial %d, want 200", got)
		}
	}
	out := logs.String()
	if n := strings.Count(out, `"level":"ERROR"`); n != 1 || !strings.Contains(out, "keeping the previous certificate") {
		t.Fatalf("want one error line for the broken version:\n%s", out)
	}
	if strings.Contains(out, "broken") || strings.Contains(out, "BEGIN") {
		t.Fatalf("the log contains PEM content:\n%s", out)
	}

	// A repaired file is picked up again.
	chain3, key3 := ca.leaf(t, leafOpts{serial: 300, host: "127.0.0.1"})
	f.write(t, "tls_cert", chain3)
	f.write(t, "tls_key", key3)
	f.clock.Advance(2 * time.Minute)
	if got := serial(); got != 300 {
		t.Fatalf("after the repair: serial %d, want 300", got)
	}
}

// TestTLS_RotationRetriesTimeDependentFailure holds that a rotated pair refused for a reason that
// passes with time (a leaf not yet valid) is tried again: once its NotBefore is reached, new
// handshakes see it without a restart, and the refusal is logged once.
func TestTLS_RotationRetriesTimeDependentFailure(t *testing.T) {
	t.Parallel()
	var logs syncBuffer
	log := slog.New(slog.NewJSONHandler(&logs, nil))
	ca := newTestCA(t)
	chain, key := ca.leaf(t, leafOpts{serial: 100, host: "127.0.0.1"})
	f := newTLSFixture(t, chain, key, "https://127.0.0.1:8443", log)
	f.cfg.Clock = f.clock
	ln := f.listen(t, log)
	serial := func() int64 {
		t.Helper()
		st, err := handshake(t, ln.Addr().String(), &tls.Config{RootCAs: ca.pool})
		if err != nil {
			t.Fatal(err)
		}
		return st.PeerCertificates[0].SerialNumber.Int64()
	}
	// The new leaf is valid for the real clock of the test client since 30 minutes; the server
	// clock lags 2 hours behind, so for the server it is not yet valid.
	chain2, key2 := ca.leaf(t, leafOpts{serial: 200, host: "127.0.0.1", notBefore: time.Now().Add(-30 * time.Minute)})
	f.clock.Advance(-2 * time.Hour)
	f.write(t, "tls_cert", chain2)
	f.write(t, "tls_key", key2)
	f.clock.Advance(2 * time.Minute)
	for range 2 {
		if got := serial(); got != 100 {
			t.Fatalf("before NotBefore: serial %d, want 100", got)
		}
	}
	f.clock.Advance(3 * time.Hour)
	if got := serial(); got != 200 {
		t.Fatalf("after NotBefore: serial %d, want 200; logs:\n%s", got, logs.String())
	}
	if n := strings.Count(logs.String(), `"level":"ERROR"`); n != 1 {
		t.Fatalf("want one error line, got %d:\n%s", n, logs.String())
	}
}
