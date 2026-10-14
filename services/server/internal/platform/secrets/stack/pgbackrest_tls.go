// SPDX-License-Identifier: Apache-2.0

package stack

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"time"
)

// PGBackRestTLSValidity is the validity of the CA and of both host certificates in
// pgbackrest_tls/ (docs/operations/configuration.md section 5, docs/DECISIONS.md D-M7-01-4).
// The certificates only authenticate two containers of one stack to each other, so they are
// long-lived; a rotation regenerates the whole directory (remove it from _store, run init).
const PGBackRestTLSValidity = 10 * 365 * 24 * time.Hour

// pgBackRestHosts are the hosts of pgbackrest_tls/: the database host and the repository host.
// Each one is TLS server and client: postgres pushes WAL to backup, backup runs its backup
// commands against the pgBackRest server in postgres.
var pgBackRestHosts = []string{servicePostgres, serviceBackup}

// serialBytes is the length of a certificate serial number; 16 random bytes stay below the
// 20-byte limit of RFC 5280 4.1.2.2 after the sign bit is cleared.
const serialBytes = 16

// generatePGBackRestTLS returns the files of the directory entry dir ("pgbackrest_tls/"): ca.crt,
// and per host <host>.crt and <host>.key. The CA is self-signed and may sign only leaf
// certificates; its private key exists only in memory during the run and is never written, so no
// service can mint further certificates. Host certificates carry the host name as common name
// (the pgBackRest option tls-server-auth matches it) and as DNS name (the peer checks it), and
// both extended key usages. Keys are ECDSA P-256 in PKCS#8 PEM. Serial numbers come from rnd;
// now is the start of the validity.
//
// Errors come from rnd or from the standard library's key generation and signing.
//
// Example:
//
//	files, err := generatePGBackRestTLS("pgbackrest_tls/", rand.Reader, time.Now())
//	// files["pgbackrest_tls/ca.crt"] is the PEM CA certificate
func generatePGBackRestTLS(dir string, rnd io.Reader, now time.Time) (map[string][]byte, error) {
	notBefore := now.UTC().Truncate(time.Second)
	notAfter := notBefore.Add(PGBackRestTLSValidity)
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rnd)
	if err != nil {
		return nil, fmt.Errorf("CA key: %w", err)
	}
	caSerial, err := serial(rnd)
	if err != nil {
		return nil, err
	}
	caTemplate := &x509.Certificate{
		SerialNumber:          caSerial,
		Subject:               pkix.Name{CommonName: "bothy pgBackRest CA"},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLen:            0,
		MaxPathLenZero:        true,
	}
	caDER, err := x509.CreateCertificate(rnd, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		return nil, fmt.Errorf("CA certificate: %w", err)
	}
	ca, err := x509.ParseCertificate(caDER)
	if err != nil {
		return nil, fmt.Errorf("CA certificate: %w", err)
	}
	files := map[string][]byte{dir + "ca.crt": pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER})}
	for _, host := range pgBackRestHosts {
		crt, key, err := hostCertificate(host, ca, caKey, rnd, notBefore, notAfter)
		if err != nil {
			return nil, fmt.Errorf("certificate of %s: %w", host, err)
		}
		files[dir+host+".crt"] = crt
		files[dir+host+".key"] = key
	}
	return files, nil
}

// hostCertificate returns the PEM certificate and PEM PKCS#8 key of host, signed by ca.
func hostCertificate(host string, ca *x509.Certificate, caKey *ecdsa.PrivateKey, rnd io.Reader, notBefore, notAfter time.Time) (crt, key []byte, err error) {
	k, err := ecdsa.GenerateKey(elliptic.P256(), rnd)
	if err != nil {
		return nil, nil, err
	}
	sn, err := serial(rnd)
	if err != nil {
		return nil, nil, err
	}
	template := &x509.Certificate{
		SerialNumber:          sn,
		Subject:               pkix.Name{CommonName: host},
		DNSNames:              []string{host},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rnd, template, ca, &k.PublicKey, caKey)
	if err != nil {
		return nil, nil, err
	}
	pkcs8, err := x509.MarshalPKCS8PrivateKey(k)
	if err != nil {
		return nil, nil, err
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
		pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: pkcs8}), nil
}

// serial returns a positive random certificate serial number read from rnd.
func serial(rnd io.Reader) (*big.Int, error) {
	b := make([]byte, serialBytes)
	if _, err := io.ReadFull(rnd, b); err != nil {
		return nil, fmt.Errorf("serial number: %w", err)
	}
	b[0] &= 0x7f
	return new(big.Int).SetBytes(b), nil
}
