// SPDX-License-Identifier: Apache-2.0

package httpserver

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/platform/otel/attr"
	"github.com/OWNER/bothy/services/server/internal/platform/secrets"
)

// LogRotationFailed is the message of a rotated certificate or key that cannot be used; the
// server keeps serving the previous pair (02 "14.4 Failure of a dependency", row
// secret provider). It is logged once per failed pair of versions.
const LogRotationFailed = "TLS certificate rotation failed, keeping the previous certificate"

// errorTypeRotation is error.type of LogRotationFailed.
const errorTypeRotation = "tls_rotation"

// rotationRetry is how often a rotated pair that failed is parsed again. A failure that passes
// with time (a leaf whose NotBefore lies ahead of the server clock) heals without a restart, and
// a broken file costs at most one parse per minute instead of one per handshake.
const rotationRetry = time.Minute

// Names of the configuration keys in the errors of this file.
const (
	keyTLSCert   = "server.tls_cert_ref"
	keyTLSKey    = "server.tls_key_ref"
	keyPublicURL = "server.public_url"
)

// certSource is the tls.Config.GetCertificate of the public listener in TLS mode. It holds the
// pair it validated last and asks the provider for both references on every handshake; the
// provider answers from its cache within secrets.cache_ttl, so a rotated file is read at most
// once per TTL. A new pair of versions is parsed and validated; when that fails, the previous
// certificate stays in use and one error line is logged for that pair.
type certSource struct {
	provider SecretGetter
	certRef  secrets.Ref
	keyRef   secrets.Ref
	host     string
	log      *slog.Logger
	now      func() time.Time

	mu       sync.Mutex
	cert     *tls.Certificate
	versions string
	failed   map[string]time.Time // pair of versions -> time of its last failed parse
}

// newCertSource checks cfg, loads the pair through provider and validates it; see
// CheckTransport for the errors. log nil means slog.Default().
func newCertSource(ctx context.Context, cfg TransportConfig, provider SecretGetter, log *slog.Logger) (*certSource, error) {
	if cfg.TLSCertRef == (secrets.Ref{}) || cfg.TLSKeyRef == (secrets.Ref{}) {
		return nil, errHalfTLS
	}
	if cfg.PublicURL == nil || cfg.PublicURL.Scheme != schemeHTTPS || cfg.PublicURL.Hostname() == "" {
		return nil, errors.New("httpserver: with server.tls_cert_ref and server.tls_key_ref set, server.public_url must use https")
	}
	if provider == nil {
		return nil, errors.New("httpserver: server.tls_cert_ref and server.tls_key_ref need the secret provider")
	}
	if log == nil {
		log = slog.Default()
	}
	clk := cfg.Clock
	if clk == nil {
		clk = clock.System()
	}
	s := &certSource{
		provider: provider, certRef: cfg.TLSCertRef, keyRef: cfg.TLSKeyRef,
		host: cfg.PublicURL.Hostname(), log: log, now: clk.Now, failed: map[string]time.Time{},
	}
	certPEM, keyPEM, versions, err := s.read(ctx)
	if err != nil {
		return nil, err
	}
	cert, err := s.parse(certPEM, keyPEM)
	if err != nil {
		return nil, err
	}
	s.cert, s.versions = cert, versions
	return s, nil
}

// read returns the PEM texts of both references and their versions as one key. The errors name
// the key and carry the reference error of the provider, which names the reference, never the
// content.
func (s *certSource) read(ctx context.Context) (certPEM, keyPEM []byte, versions string, err error) {
	cv, cver, err := s.provider.Get(ctx, s.certRef)
	if err != nil {
		return nil, nil, "", fmt.Errorf("httpserver: %s cannot be read (%s): %w", keyTLSCert, secrets.ErrorCode(err), err)
	}
	kv, kver, err := s.provider.Get(ctx, s.keyRef)
	if err != nil {
		return nil, nil, "", fmt.Errorf("httpserver: %s cannot be read (%s): %w", keyTLSKey, secrets.ErrorCode(err), err)
	}
	return cv.Bytes(), kv.Bytes(), string(cver) + "/" + string(kver), nil
}

// parse builds the certificate from the PEM texts and checks it: a PEM certificate chain with the
// leaf first, a private key that matches the leaf, the validity period around now and the host of
// server.public_url (x509.Certificate.VerifyHostname). It clears both texts. The errors are fixed
// texts with key names; none repeats the input or the error of crypto/tls, which may quote it.
func (s *certSource) parse(certPEM, keyPEM []byte) (*tls.Certificate, error) {
	defer clear(certPEM)
	defer clear(keyPEM)
	block, _ := pem.Decode(certPEM)
	if block == nil || block.Type != "CERTIFICATE" {
		return nil, errors.New("httpserver: server.tls_cert_ref holds no PEM certificate chain with the leaf first")
	}
	if _, err := x509.ParseCertificate(block.Bytes); err != nil {
		return nil, errors.New("httpserver: server.tls_cert_ref: the first PEM block is not a valid X.509 certificate")
	}
	pair, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, errors.New("httpserver: server.tls_cert_ref and server.tls_key_ref do not match, or server.tls_key_ref holds no PEM private key")
	}
	leaf := pair.Leaf
	if leaf == nil {
		if leaf, err = x509.ParseCertificate(pair.Certificate[0]); err != nil {
			return nil, errors.New("httpserver: server.tls_cert_ref: the leaf is not a valid X.509 certificate")
		}
		pair.Leaf = leaf
	}
	now := s.now()
	switch {
	case now.Before(leaf.NotBefore):
		return nil, errors.New("httpserver: server.tls_cert_ref: the leaf certificate is not yet valid")
	case now.After(leaf.NotAfter):
		return nil, errors.New("httpserver: server.tls_cert_ref: the leaf certificate has expired")
	}
	if err := leaf.VerifyHostname(s.host); err != nil {
		return nil, errors.New("httpserver: server.tls_cert_ref: the leaf certificate does not cover the host of server.public_url")
	}
	return &pair, nil
}

// GetCertificate returns the current certificate for a handshake. It asks the provider for both
// references; when their versions changed it validates the new pair and switches to it, or keeps
// the previous one and logs LogRotationFailed once for that pair of versions. A failed pair is
// tried again at most once per rotationRetry, because a reason can pass with time (a leaf not yet
// valid). It never fails a handshake because of a rotation: a valid certificate was loaded at
// start. The whole call holds the lock, so a handshake that read an older pair cannot switch back
// after a newer one was installed; within secrets.cache_ttl the provider answers from memory.
func (s *certSource) GetCertificate(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	ctx := context.Background()
	if hello != nil && hello.Context() != nil {
		ctx = hello.Context()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	certPEM, keyPEM, versions, err := s.read(ctx)
	failedAt, failed := s.failed[versions]
	if err != nil || versions == s.versions || (failed && s.now().Sub(failedAt) < rotationRetry) {
		// A read error after the start happens only when the handshake was cancelled: the
		// provider keeps the cached value when a refresh fails and logs that itself.
		clear(certPEM)
		clear(keyPEM)
		return s.cert, nil
	}
	cert, err := s.parse(certPEM, keyPEM)
	if err != nil {
		if !failed {
			s.log.LogAttrs(ctx, slog.LevelError, LogRotationFailed, slog.String(attr.ErrorType, errorTypeRotation))
		}
		s.failed[versions] = s.now()
		return s.cert, nil
	}
	s.cert, s.versions = cert, versions
	clear(s.failed)
	return s.cert, nil
}
