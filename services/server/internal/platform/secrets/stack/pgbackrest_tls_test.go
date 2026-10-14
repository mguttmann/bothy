// SPDX-License-Identifier: Apache-2.0

//go:build unix

package stack_test

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/x509"
	"encoding/pem"
	"io/fs"
	"path/filepath"
	"slices"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/platform/secrets/stack"
)

// tlsFiles are the files of the entry pgbackrest_tls/ (brief M7-01, docs/DECISIONS.md
// D-M7-01-4): the CA certificate and one key pair per host. The CA key is never stored.
var tlsFiles = []string{
	"pgbackrest_tls/backup.crt", "pgbackrest_tls/backup.key", "pgbackrest_tls/ca.crt",
	"pgbackrest_tls/postgres.crt", "pgbackrest_tls/postgres.key",
}

func TestPGBackRestTLSGenerated(t *testing.T) {
	t.Parallel()
	target, source := dirs(t)
	r := mustInit(t, opts(target, source))
	if got := status(r, "pgbackrest_tls/"); got != stack.StatusGenerated {
		t.Fatalf("pgbackrest_tls/ status %q, want generated", got)
	}
	store := filepath.Join(target, "_store")
	var got []string
	for _, name := range files(t, store) {
		if filepath.Dir(name) == "pgbackrest_tls" {
			got = append(got, name)
		}
	}
	if !slices.Equal(got, tlsFiles) {
		t.Fatalf("_store/pgbackrest_tls holds %v, want %v", got, tlsFiles)
	}
	for _, name := range tlsFiles {
		if m := perm(t, filepath.Join(store, name)); m != 0o400 {
			t.Errorf("_store/%s mode %v, want 0400", name, m)
		}
	}
	if m := perm(t, filepath.Join(store, "pgbackrest_tls")); m != fs.ModeDir|0o700 {
		t.Errorf("_store/pgbackrest_tls mode %v, want drwx------", m)
	}

	ca := cert(t, read(t, store, "pgbackrest_tls/ca.crt"))
	if !ca.IsCA || ca.KeyUsage&x509.KeyUsageCertSign == 0 || !ca.BasicConstraintsValid || ca.MaxPathLen != 0 || !ca.MaxPathLenZero {
		t.Errorf("ca.crt is no CA limited to leaf certificates: IsCA %v usage %v maxPathLen %d", ca.IsCA, ca.KeyUsage, ca.MaxPathLen)
	}
	roots := x509.NewCertPool()
	roots.AddCert(ca)
	for _, host := range []string{"backup", "postgres"} {
		leaf := cert(t, read(t, store, "pgbackrest_tls/"+host+".crt"))
		if leaf.IsCA {
			t.Errorf("%s.crt is a CA", host)
		}
		if leaf.Subject.CommonName != host || !slices.Equal(leaf.DNSNames, []string{host}) {
			t.Errorf("%s.crt: CN %q, DNS names %v, want %s", host, leaf.Subject.CommonName, leaf.DNSNames, host)
		}
		// Each host is TLS server (archive-push to backup, backup commands to postgres) and client.
		for _, usage := range []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth} {
			if _, err := leaf.Verify(x509.VerifyOptions{Roots: roots, DNSName: host, KeyUsages: []x509.ExtKeyUsage{usage}, CurrentTime: leaf.NotBefore}); err != nil {
				t.Errorf("%s.crt does not chain to ca.crt for usage %v: %v", host, usage, err)
			}
		}
		if got := leaf.NotAfter.Sub(leaf.NotBefore); got != stack.PGBackRestTLSValidity {
			t.Errorf("%s.crt valid for %v, want stack.PGBackRestTLSValidity %v", host, got, stack.PGBackRestTLSValidity)
		}
		key := privateKey(t, read(t, store, "pgbackrest_tls/"+host+".key"))
		if key.Curve != elliptic.P256() {
			t.Errorf("%s.key curve %v, want P-256", host, key.Curve.Params().Name)
		}
		pub, ok := leaf.PublicKey.(*ecdsa.PublicKey)
		if !ok || !pub.Equal(&key.PublicKey) {
			t.Errorf("%s.key does not belong to %s.crt", host, host)
		}
	}
	if got := ca.NotAfter.Sub(ca.NotBefore); got != stack.PGBackRestTLSValidity {
		t.Errorf("ca.crt valid for %v, want %v", got, stack.PGBackRestTLSValidity)
	}
}

func TestPGBackRestTLSOnlyForPostgresAndBackup(t *testing.T) {
	t.Parallel()
	target, source := dirs(t)
	mustInit(t, opts(target, source))
	for _, service := range stack.Services() {
		var got []string
		for _, name := range files(t, filepath.Join(target, service)) {
			if filepath.Dir(name) == "pgbackrest_tls" {
				got = append(got, name)
			}
		}
		want := []string(nil)
		if service == "postgres" || service == "backup" {
			want = tlsFiles
		}
		if !slices.Equal(got, want) {
			t.Errorf("%s/ holds %v of pgbackrest_tls, want %v", service, got, want)
		}
		for _, name := range got {
			if read(t, filepath.Join(target, service), name) != read(t, filepath.Join(target, "_store"), name) {
				t.Errorf("%s/%s differs from _store", service, name)
			}
		}
	}
}

func TestPGBackRestTLSKeptOnSecondRun(t *testing.T) {
	t.Parallel()
	target, source := dirs(t)
	first := mustInit(t, opts(target, source))
	before := snapshot(t, target)
	second := mustInit(t, opts(target, source))
	diffSnapshots(t, before, snapshot(t, target))
	if got := status(second, "pgbackrest_tls/"); got != stack.StatusKept {
		t.Errorf("second run reports pgbackrest_tls/ as %q, want kept", got)
	}
	if fp(first, "pgbackrest_tls/") == "" || fp(first, "pgbackrest_tls/") != fp(second, "pgbackrest_tls/") {
		t.Errorf("fingerprint changed: %q then %q", fp(first, "pgbackrest_tls/"), fp(second, "pgbackrest_tls/"))
	}
}

// status returns the status of the report line named name.
func status(r stack.Report, name string) stack.Status {
	for _, l := range r.Lines {
		if l.Name == name {
			return l.Status
		}
	}
	return ""
}

// fp returns the fingerprint of the report line named name.
func fp(r stack.Report, name string) string {
	for _, l := range r.Lines {
		if l.Name == name {
			return l.Fingerprint
		}
	}
	return ""
}

// cert parses one PEM certificate.
func cert(t *testing.T, text string) *x509.Certificate {
	t.Helper()
	block, rest := pem.Decode([]byte(text))
	if block == nil || block.Type != "CERTIFICATE" || len(rest) != 0 {
		t.Fatalf("not exactly one PEM certificate")
	}
	c, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// privateKey parses one PEM PKCS#8 ECDSA key.
func privateKey(t *testing.T, text string) *ecdsa.PrivateKey {
	t.Helper()
	block, rest := pem.Decode([]byte(text))
	if block == nil || block.Type != "PRIVATE KEY" || len(rest) != 0 {
		t.Fatalf("not exactly one PEM PKCS#8 key")
	}
	k, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	ec, ok := k.(*ecdsa.PrivateKey)
	if !ok {
		t.Fatalf("key is %T, want ECDSA", k)
	}
	return ec
}
