// SPDX-License-Identifier: Apache-2.0

package httpserver

import (
	"context"
	"crypto/tls"
	"errors"
	"log/slog"
	"net"
	"net/netip"
	"net/url"
	"sync/atomic"

	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/platform/otel/attr"
	"github.com/OWNER/bothy/services/server/internal/platform/secrets"
)

// Messages of a refused plaintext connection (SEC-TLS-1); the attribute bothy.reason names the
// listener ("public"). The first refusal of a listener carries the hint what the operator can
// change; none names the peer (SEC-LOG-5).
const (
	// LogPlaintextRefused is the message of every refusal after the first one, at Debug.
	LogPlaintextRefused = "plaintext connection from an untrusted peer refused"
	// LogPlaintextRefusedHint is the message of the first refusal, at Warn.
	LogPlaintextRefusedHint = LogPlaintextRefused +
		"; add the address of the reverse proxy as seen by the server to server.trusted_proxies"
)

// TransportConfig is what the public listener needs for transport protection (19-sicherheit.md
// "Transport", 16-betrieb.md "Netz"). internal/wire fills it from the configuration.
type TransportConfig struct {
	// ListenAddr is server.listen_addr.
	ListenAddr string
	// PublicURL is server.public_url, parsed. TLS mode needs the scheme https and checks that the
	// leaf certificate covers its host.
	PublicURL *url.URL
	// TrustedProxies is server.trusted_proxies, the same parsed slice the chain uses for the
	// client IP (ChainConfig.TrustedProxies): one list decides who may send plaintext and whose
	// forwarding headers count.
	TrustedProxies []netip.Prefix
	// TLSCertRef is server.tls_cert_ref, the PEM certificate chain with the leaf first. The zero
	// Ref, together with a zero TLSKeyRef, means plaintext.
	TLSCertRef secrets.Ref
	// TLSKeyRef is server.tls_key_ref, the PEM private key of the leaf.
	TLSKeyRef secrets.Ref
	// Clock is the time against which the validity of a leaf is checked and failed rotations
	// are retried; nil means clock.System(). Tests share it with the secret provider.
	Clock clock.Clock
}

// SecretGetter is the part of the secret provider (secrets.Provider) that the TLS mode reads: the
// value of a reference and its version, cached for secrets.cache_ttl.
type SecretGetter interface {
	// Get returns the value of ref and its version; see secrets.Provider.Get.
	Get(ctx context.Context, ref secrets.Ref) (secrets.Value, secrets.Version, error)
}

// TLSEnabled reports whether cfg asks the server to terminate TLS itself: true when either
// reference is set. CheckTransport and ListenPublic refuse a configuration with only one of them.
func TLSEnabled(cfg TransportConfig) bool {
	return cfg.TLSCertRef != (secrets.Ref{}) || cfg.TLSKeyRef != (secrets.Ref{})
}

// CheckTransport checks cfg and, in TLS mode, loads and validates the certificate material
// through provider without opening a listener. internal/wire calls it at start step 2
// ("Geheimnisse auflösen", 02 "15.1 Start"), so bad material ends the start before the database
// wait. provider may be nil in plaintext mode.
//
// Errors name the configuration keys and the rule, never a secret value: only one of the two
// references set; TLS with a server.public_url that is not https; a reference that cannot be
// read (with the code of secrets.ErrorCode); no PEM certificate; a key that does not match the
// leaf; a leaf that is not yet valid, expired or does not cover the host of server.public_url.
func CheckTransport(ctx context.Context, cfg TransportConfig, provider SecretGetter) error {
	if !TLSEnabled(cfg) {
		return nil
	}
	_, err := newCertSource(ctx, cfg, provider, nil)
	return err
}

// ListenPublic opens the public listener on cfg.ListenAddr. In TLS mode (both references set) it
// returns a TLS listener with TLS 1.2 as the floor, the cipher suites and curves of the Go
// standard library, no client certificates and the certificate source of tlscert.go, which
// follows rotation through provider; there is no peer filter and no plaintext redirect listener
// (SEC-TLS-2, R-SE-08). In plaintext mode it returns the TCP listener wrapped by PeerFilter
// (SEC-TLS-1). log receives the refusal lines and the errors of a failed rotation; nil means
// slog.Default().
//
// The errors are those of CheckTransport and of Listen.
func ListenPublic(ctx context.Context, cfg TransportConfig, provider SecretGetter, log *slog.Logger) (net.Listener, error) {
	if log == nil {
		log = slog.Default()
	}
	if !TLSEnabled(cfg) {
		ln, err := Listen(ctx, cfg.ListenAddr)
		if err != nil {
			return nil, err
		}
		return PeerFilter(ln, cfg.TrustedProxies, "public", log), nil
	}
	src, err := newCertSource(ctx, cfg, provider, log)
	if err != nil {
		return nil, err
	}
	ln, err := Listen(ctx, cfg.ListenAddr)
	if err != nil {
		return nil, err
	}
	return &tlsListener{Listener: ln, cfg: serverTLSConfig(src)}, nil
}

// serverTLSConfig is the TLS configuration of the public listener: TLS 1.2 as the floor and
// otherwise the defaults of crypto/tls, so TLS 1.3 is preferred and the cipher suites and curves
// stay maintained with Go (SEC-TLS-2). NextProtos are those net/http offers by default for TLS
// (HTTP/2 and HTTP/1.1); http.Server.Serve serves HTTP/2 on connections that negotiated it.
func serverTLSConfig(src *certSource) *tls.Config {
	return &tls.Config{
		MinVersion:     tls.VersionTLS12,
		GetCertificate: src.GetCertificate,
		NextProtos:     []string{"h2", "http/1.1"},
	}
}

// tlsListener wraps every accepted connection in a TLS server connection; the handshake runs on
// the first read, in the goroutine of http.Server for that connection.
type tlsListener struct {
	net.Listener
	cfg *tls.Config
}

// Accept waits for the next connection and returns it as a *tls.Conn.
func (l *tlsListener) Accept() (net.Conn, error) {
	c, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	return tls.Server(c, l.cfg), nil
}

// PeerFilter returns inner with SEC-TLS-1 applied: Accept returns only connections whose peer is
// a loopback address (127.0.0.0/8, ::1) or lies in one of the trusted prefixes; IPv4-mapped IPv6
// addresses count as their IPv4 address. Every other connection, also one whose remote address is
// not TCP, is closed at once without a byte read or written, and Accept goes on with the next
// one, so http.Server never sees an error or backs off for a refused peer. Errors of inner pass
// through unchanged.
//
// name names the listener in the log, as bothy.reason ("public"). The first refusal per filter
// logs at Warn with LogPlaintextRefusedHint, later refusals at Debug with LogPlaintextRefused; no
// line contains the peer address (SEC-LOG-5). log nil means slog.Default().
//
// Example:
//
//	ln, err := httpserver.Listen(ctx, "0.0.0.0:8080")
//	srv.Serve(httpserver.PeerFilter(ln, trustedProxies, "public", log))
func PeerFilter(inner net.Listener, trusted []netip.Prefix, name string, log *slog.Logger) net.Listener {
	if log == nil {
		log = slog.Default()
	}
	return &peerFilter{Listener: inner, trusted: trusted, name: name, log: log}
}

// peerFilter is the listener of PeerFilter.
type peerFilter struct {
	net.Listener
	trusted []netip.Prefix
	name    string
	log     *slog.Logger
	warned  atomic.Bool
}

// Accept returns the next connection from an allowed peer and closes the others.
func (f *peerFilter) Accept() (net.Conn, error) {
	for {
		c, err := f.Listener.Accept()
		if err != nil {
			return nil, err
		}
		if f.allowed(c.RemoteAddr()) {
			return c, nil
		}
		_ = c.Close()
		f.logRefusal()
	}
}

// allowed reports whether remote may send plaintext: a TCP peer on loopback or in trusted.
func (f *peerFilter) allowed(remote net.Addr) bool {
	tcp, ok := remote.(*net.TCPAddr)
	if !ok || tcp == nil {
		return false
	}
	addr := normalize(tcp.AddrPort().Addr())
	if !addr.IsValid() {
		return false
	}
	return addr.IsLoopback() || isTrusted(addr, f.trusted)
}

// logRefusal logs the first refusal at Warn and every later one at Debug.
func (f *peerFilter) logRefusal() {
	ctx, listener := context.Background(), slog.String(attr.Reason, f.name)
	if f.warned.CompareAndSwap(false, true) {
		f.log.LogAttrs(ctx, slog.LevelWarn, LogPlaintextRefusedHint, listener)
		return
	}
	f.log.LogAttrs(ctx, slog.LevelDebug, LogPlaintextRefused, listener)
}

// errHalfTLS is the error of a configuration with only one TLS reference.
var errHalfTLS = errors.New("httpserver: server.tls_cert_ref and server.tls_key_ref must both be set or both be empty")
