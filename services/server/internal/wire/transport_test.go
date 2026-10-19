// SPDX-License-Identifier: Apache-2.0

package wire_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"io"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/platform/lifecycle"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

// writeTLSPair generates a CA and a leaf for 127.0.0.1, writes the chain to tls_cert and the key
// to tls_key in dir (mode 0600) and returns the pool with the CA.
func writeTLSPair(t *testing.T, dir string) *x509.CertPool {
	t.Helper()
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	caTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "Test CA"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTmpl, caTmpl, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	ca, err := x509.ParseCertificate(caDER)
	if err != nil {
		t.Fatal(err)
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	leafTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(2), Subject: pkix.Name{CommonName: "127.0.0.1"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		IPAddresses: []net.IP{net.IPv4(127, 0, 0, 1)}, KeyUsage: x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	leafDER, err := x509.CreateCertificate(rand.Reader, leafTmpl, ca, &key.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	chain := append(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: leafDER}),
		pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER})...)
	for name, b := range map[string][]byte{"tls_cert": chain, "tls_key": pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})} {
		if err := os.WriteFile(filepath.Join(dir, name), b, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	pool := x509.NewCertPool()
	pool.AddCert(ca)
	return pool
}

// envValue returns the value of name in env.
func envValue(env []string, name string) string {
	for _, e := range env {
		if v, ok := strings.CutPrefix(e, name+"="); ok {
			return v
		}
	}
	return ""
}

// waitPublic waits for the line of the public listener and returns its port.
func (p *process) waitPublic(t *testing.T) int {
	t.Helper()
	const msg = "public listener started"
	deadline := testkit.Deadline()
	for time.Now().Before(deadline) {
		if port := p.logs.port(msg); port != 0 {
			return port
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("no %q line; logs:\n%s", msg, p.logs)
	return 0
}

// TestServeTLSPublicListener holds Scope 5 of brief M1-21: with both references set the public
// listener of bothy serve speaks TLS with the certificate from the secret provider, and both
// listeners send HSTS with http.hsts_max_age because server.public_url uses https.
func TestServeTLSPublicListener(t *testing.T) {
	t.Parallel()
	db := newServeDB(t)
	completeSetup(t, db)
	env := serveEnv(t, db, "BOTHY_SERVER__PUBLIC_URL=https://127.0.0.1:8443", "BOTHY_HTTP__HSTS_MAX_AGE=600s",
		"BOTHY_SERVER__TLS_CERT_REF=file:<secrets.dir>/tls_cert", "BOTHY_SERVER__TLS_KEY_REF=file:<secrets.dir>/tls_key")
	pool := writeTLSPair(t, envValue(env, "BOTHY_SECRETS__DIR"))
	p := start(t, wire.RoleAPI, env, wire.Deps{})
	internal := p.waitInternal(t)
	waitReadyz(t, internal, http.StatusOK)
	public := p.waitPublic(t)

	client := &http.Client{Timeout: testkit.Patience, Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12}}}
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://127.0.0.1:"+strconv.Itoa(public)+"/nope", nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request over TLS: %v; logs:\n%s", err, p.logs)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound || resp.TLS == nil || resp.Header.Get("Strict-Transport-Security") != "max-age=600" ||
		!strings.Contains(string(body), `"code":"not_found"`) {
		t.Fatalf("public over TLS: %d, TLS %v, HSTS %q, body %s", resp.StatusCode, resp.TLS != nil, resp.Header.Get("Strict-Transport-Security"), body)
	}
	// Plaintext to the TLS port is no request: the TLS listener has no plaintext path.
	if code, _ := get(t, public, "/healthz"); code == http.StatusOK || code == http.StatusNotFound {
		t.Fatalf("plaintext on the TLS port answered %d", code)
	}

	// The internal listener stays plaintext, with the headers of SEC-HDR-1 including HSTS.
	ireq, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://127.0.0.1:"+strconv.Itoa(internal)+"/healthz", nil)
	iresp, err := http.DefaultClient.Do(ireq)
	if err != nil {
		t.Fatal(err)
	}
	_ = iresp.Body.Close()
	if iresp.StatusCode != http.StatusOK || iresp.Header.Get("Strict-Transport-Security") != "max-age=600" || iresp.Header.Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("internal /healthz: %d, headers %v", iresp.StatusCode, iresp.Header)
	}
	if code, body := get(t, internal, "/nope"); code != http.StatusNotFound || !strings.Contains(body, `"code":"not_found"`) {
		t.Fatalf("internal unknown path: %d %q, want a 404 problem", code, body)
	}
	p.sigs <- syscall.SIGTERM
	if code := p.exit(t); code != lifecycle.ExitOK {
		t.Fatalf("exit = %d; logs:\n%s", code, p.logs)
	}
}

// TestServeTLSBadMaterialFailsBeforeTheWait holds Scope 5: the TLS material is loaded and checked
// at start step 2 (secrets), so a broken pair ends the start before the internal listener and the
// database wait, with an error that names the key and not the content.
func TestServeTLSBadMaterialFailsBeforeTheWait(t *testing.T) {
	t.Parallel()
	db := pgtest.New(t)
	env := serveEnv(t, db, "BOTHY_SERVER__PUBLIC_URL=https://127.0.0.1:8443", "BOTHY_STARTUP__DEPENDENCY_WAIT=1h",
		"BOTHY_DATABASE__PORT=1",
		"BOTHY_SERVER__TLS_CERT_REF=file:<secrets.dir>/tls_cert", "BOTHY_SERVER__TLS_KEY_REF=file:<secrets.dir>/tls_key")
	dir := envValue(env, "BOTHY_SECRETS__DIR")
	writeTLSPair(t, dir)
	const broken = "-----BEGIN CERTIFICATE-----\nnot-a-certificate\n-----END CERTIFICATE-----\n"
	if err := os.WriteFile(filepath.Join(dir, "tls_cert"), []byte(broken), 0o600); err != nil {
		t.Fatal(err)
	}
	began := time.Now()
	p := start(t, wire.RoleAPI, env, wire.Deps{})
	if code := p.exit(t); code != lifecycle.ExitFailure {
		t.Fatalf("exit = %d", code)
	}
	// A start that waited for the database would wait startup.dependency_wait (1h).
	if time.Since(began) > testkit.Patience {
		t.Fatal("the start waited for the database before it checked the TLS material")
	}
	err := p.runner.Err()
	if err == nil || !strings.Contains(err.Error(), "start secrets") || !strings.Contains(err.Error(), "server.tls_cert_ref") ||
		strings.Contains(err.Error(), "not-a-certificate") {
		t.Fatalf("Err() = %v", err)
	}
	if p.logs.port("internal listener started") != 0 || strings.Contains(p.logs.String(), "not-a-certificate") {
		t.Fatalf("the internal listener opened or the logs show the content:\n%s", p.logs)
	}
}

// TestServeWorkerIgnoresTLS holds that the worker, which opens no public listener, does not read
// the TLS references: a worker shares the configuration of the api container.
func TestServeWorkerIgnoresTLS(t *testing.T) {
	t.Parallel()
	db := newServeDB(t)
	env := serveEnv(t, db, "BOTHY_SERVER__PUBLIC_URL=https://127.0.0.1:8443",
		"BOTHY_SERVER__TLS_CERT_REF=file:<secrets.dir>/missing_cert", "BOTHY_SERVER__TLS_KEY_REF=file:<secrets.dir>/missing_key")
	p := start(t, wire.RoleWorker, env, wire.Deps{})
	waitReadyz(t, p.waitInternal(t), http.StatusOK)
	p.sigs <- syscall.SIGTERM
	if code := p.exit(t); code != lifecycle.ExitOK {
		t.Fatalf("exit = %d; logs:\n%s", code, p.logs)
	}
}
