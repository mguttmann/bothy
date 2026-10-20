// SPDX-License-Identifier: Apache-2.0

package stack

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"text/template"
	"time"
)

// Mode is a routing mode of the TLS front.
type Mode string

// The routing modes of the TLS front (infra/compose/test/terminal/routing-<mode>.conf).
const (
	// ModeSingle sends every path to api (profiles default and limits).
	ModeSingle Mode = "single"
	// ModeHash sends every path to M2-47's example edge (hash routing, profile twonode).
	ModeHash Mode = "hash"
	// ModeSplit sends the device stream to api, the host data connection to api-2 and the rest to
	// the edge (TM-T15 without hash routing).
	ModeSplit Mode = "split"
)

// InstanceHost and InstancePort are the instance origin https://chat.example.com:8443 of the
// stack (server.public_url); the TLS front listens on 8443 in its container.
const (
	InstanceHost   = "chat.example.com"
	InstancePort   = 8443
	InstanceOrigin = "https://chat.example.com:8443"
)

// FrontTimeouts are the profile values the timeouts of the TLS front are computed from, as
// M2-47's edge computes them.
type FrontTimeouts struct {
	// IdleTimeout and IdleGrace are terminal.idle_timeout and terminal.idle_grace.
	IdleTimeout, IdleGrace time.Duration
	// HeartbeatInterval and HeartbeatMissLimit are terminal.host_heartbeat_interval and
	// terminal.host_heartbeat_miss_limit.
	HeartbeatInterval  time.Duration
	HeartbeatMissLimit int
}

// frontDir is the folder of the front files relative to the repository root.
const frontDir = "infra/compose/test/terminal"

// RenderFront writes nginx.conf (the template with the timeouts of t) and routing.conf (the file
// of mode) into dir, the folder TERMSUITE_FRONT_DIR that the TLS front mounts read-only.
//
// Errors: a missing template or an unknown mode.
func RenderFront(root, dir string, mode Mode, t FrontTimeouts) error {
	tmpl, err := template.ParseFiles(filepath.Join(root, frontDir, "nginx.conf.tmpl"))
	if err != nil {
		return err
	}
	var b bytes.Buffer
	// The stream paths outlast the end of an idle session by the server (idle_timeout plus
	// idle_grace) by one detection window of the heartbeats, so the front never cuts a quiet
	// stream before the server closes it with 4100 (TM-T11, ruling D-M2-48-3).
	other := t.HeartbeatInterval * time.Duration(t.HeartbeatMissLimit)
	if err := tmpl.Execute(&b, struct{ StreamSeconds, OtherSeconds int64 }{
		StreamSeconds: int64((t.IdleTimeout + t.IdleGrace + other) / time.Second),
		OtherSeconds:  int64(other / time.Second),
	}); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "nginx.conf"), b.Bytes(), 0o600); err != nil {
		return err
	}
	return SetFrontRouting(root, dir, mode)
}

// SetFrontRouting replaces routing.conf in dir with the file of mode; the caller reloads nginx.
//
// Errors: an unknown mode.
func SetFrontRouting(root, dir string, mode Mode) error {
	switch mode {
	case ModeSingle, ModeHash, ModeSplit:
	default:
		return fmt.Errorf("stack: unknown routing mode %q", mode)
	}
	b, err := os.ReadFile(filepath.Join(root, frontDir, "routing-"+string(mode)+".conf")) //nolint:gosec // G304: a fixed file of the repository
	if err != nil {
		return err
	}
	tmp := filepath.Join(dir, "routing.conf.new")
	if err := os.WriteFile(tmp, b, 0o600); err != nil { //nolint:gosec // G703: dir is the temp folder of the stack
		return err
	}
	return os.Rename(tmp, filepath.Join(dir, "routing.conf"))
}

// CA is the test CA of one stack: ECDSA P-256, valid one day, its key only in memory.
type CA struct {
	cert *x509.Certificate
	pem  []byte
	pool *x509.CertPool
}

// NewCA creates a test CA and a server certificate for chat.example.com signed by it and writes
// front.crt and front.key (mode 0600) and ca.crt into dir.
//
// Errors: a failed key generation or write.
func NewCA(dir string) (*CA, error) {
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	caTmpl := &x509.Certificate{
		SerialNumber: serial(), Subject: pkix.Name{CommonName: "termsuite test CA"},
		NotBefore: now.Add(-time.Minute), NotAfter: now.Add(24 * time.Hour),
		IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTmpl, caTmpl, &caKey.PublicKey, caKey)
	if err != nil {
		return nil, err
	}
	caCert, err := x509.ParseCertificate(caDER)
	if err != nil {
		return nil, err
	}
	leafKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	leafTmpl := &x509.Certificate{
		SerialNumber: serial(), Subject: pkix.Name{CommonName: InstanceHost}, DNSNames: []string{InstanceHost},
		NotBefore: now.Add(-time.Minute), NotAfter: now.Add(24 * time.Hour),
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	leafDER, err := x509.CreateCertificate(rand.Reader, leafTmpl, caCert, &leafKey.PublicKey, caKey)
	if err != nil {
		return nil, err
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(leafKey)
	if err != nil {
		return nil, err
	}
	caPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER})
	leafPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: leafDER})
	for name, b := range map[string][]byte{
		"front.crt": leafPEM,
		"front.key": pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}),
		"ca.crt":    caPEM,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), b, 0o600); err != nil {
			return nil, err
		}
	}
	pool := x509.NewCertPool()
	pool.AddCert(caCert)
	return &CA{cert: caCert, pem: caPEM, pool: pool}, nil
}

// Pool returns a pool that holds only the test CA.
func (c *CA) Pool() *x509.CertPool { return c.pool }

// PEM returns the certificate of the test CA in PEM form (the trust store entry of the hosts).
func (c *CA) PEM() []byte { return bytes.Clone(c.pem) }

// serial returns a random 128-bit serial number.
func serial() *big.Int {
	n, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		panic(err)
	}
	return n
}
