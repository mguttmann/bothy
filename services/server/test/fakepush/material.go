// SPDX-License-Identifier: Apache-2.0

package fakepush

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"time"
)

// The files a run writes into Options.Dir: the APNs key and the FCM service account are secret
// files of the server (mode 0600, owned by the user of the fake, 65532 in the image, which the
// server shares), the CA is public (mode 0644) and becomes SSL_CERT_FILE of the server.
const (
	// FileAPNsKey is the ES256 key of push.apns.private_key_ref, PKCS #8 PEM like Apple's .p8.
	FileAPNsKey = "apns_key.p8"
	// FileFCMAccount is the service account JSON of push.fcm.service_account_ref.
	FileFCMAccount = "fcm_service_account.json"
	// FileCA is the PEM certificate of the per-run CA that issued the TLS leaf of both fakes.
	FileCA = "ca.pem"
)

// validity is how long the run CA and the leaf are valid: a device run takes minutes to hours,
// as the test CA of infra/compose/e2e.sh.
const validity = 48 * time.Hour

// rsaBits is the size of the service account key; the FCM adapter needs at least 2048 bits.
const rsaBits = 2048

// material is what a run generates: the keys the fakes verify the server's tokens with, and the
// TLS leaf of both fakes for the host name with the PEM of its CA. The CA key exists only while
// the leaf is signed.
type material struct {
	apnsKey *ecdsa.PrivateKey
	fcmKey  *rsa.PrivateKey
	leaf    tls.Certificate
	caPEM   []byte
}

// generate creates the material of a run for host. The error reports a failed key generation or
// certificate signing.
func generate(host string) (*material, error) {
	m := &material{}
	var err error
	if m.apnsKey, err = ecdsa.GenerateKey(elliptic.P256(), rand.Reader); err != nil {
		return nil, fmt.Errorf("fakepush: APNs key: %w", err)
	}
	if m.fcmKey, err = rsa.GenerateKey(rand.Reader, rsaBits); err != nil {
		return nil, fmt.Errorf("fakepush: FCM key: %w", err)
	}
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("fakepush: CA key: %w", err)
	}
	leafKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("fakepush: leaf key: %w", err)
	}
	now := time.Now()
	caTmpl := &x509.Certificate{SerialNumber: serial(), Subject: pkix.Name{CommonName: "fakepush test CA"},
		NotBefore: now.Add(-time.Hour), NotAfter: now.Add(validity), IsCA: true, BasicConstraintsValid: true, MaxPathLenZero: true,
		KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageCRLSign}
	caDER, err := x509.CreateCertificate(rand.Reader, caTmpl, caTmpl, &caKey.PublicKey, caKey)
	if err != nil {
		return nil, fmt.Errorf("fakepush: CA certificate: %w", err)
	}
	ca, err := x509.ParseCertificate(caDER)
	if err != nil {
		return nil, fmt.Errorf("fakepush: CA certificate: %w", err)
	}
	leafTmpl := &x509.Certificate{SerialNumber: serial(), Subject: pkix.Name{CommonName: host}, DNSNames: []string{host},
		NotBefore: now.Add(-time.Hour), NotAfter: now.Add(validity), BasicConstraintsValid: true,
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	leafDER, err := x509.CreateCertificate(rand.Reader, leafTmpl, ca, &leafKey.PublicKey, caKey)
	if err != nil {
		return nil, fmt.Errorf("fakepush: leaf certificate: %w", err)
	}
	m.leaf = tls.Certificate{Certificate: [][]byte{leafDER, caDER}, PrivateKey: leafKey}
	m.caPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER})
	return m, nil
}

// serial returns a random positive serial number of 128 bits.
func serial() *big.Int {
	n, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil || n.Sign() == 0 {
		return big.NewInt(1)
	}
	return n
}

// apnsKeyPEM returns the APNs key as one PEM block of type PRIVATE KEY (PKCS #8), the form of an
// Apple .p8 key that apns.ParsePrivateKey requires.
func (m *material) apnsKeyPEM() ([]byte, error) {
	der, err := x509.MarshalPKCS8PrivateKey(m.apnsKey)
	if err != nil {
		return nil, fmt.Errorf("fakepush: APNs key: %w", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), nil
}

// writeFile writes b to dir/name with mode, through a temporary file in dir and a rename, so a
// reader never sees a partial file. The error names the file, never its content.
func writeFile(dir, name string, b []byte, mode os.FileMode) error {
	f, err := os.CreateTemp(dir, "."+name+".*")
	if err != nil {
		return fmt.Errorf("fakepush: write %s: %w", name, err)
	}
	tmp := f.Name()
	_, werr := f.Write(b)
	cerr := f.Close()
	if err := errors.Join(werr, cerr, os.Chmod(tmp, mode)); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("fakepush: write %s: %w", name, err)
	}
	if err := os.Rename(tmp, filepath.Join(dir, name)); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("fakepush: write %s: %w", name, err)
	}
	return nil
}
