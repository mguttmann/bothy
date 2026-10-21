// SPDX-License-Identifier: Apache-2.0

package apns

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/x509"
	"encoding/pem"
	"errors"

	"github.com/OWNER/bothy/services/server/internal/kernel/jose"
	"github.com/OWNER/bothy/services/server/internal/platform/secrets"
)

// pemTypePKCS8 is the PEM block type of the .p8 key Apple issues: PKCS #8 (RFC 5208).
const pemTypePKCS8 = "PRIVATE KEY"

// ParsePrivateKey parses the .p8 key of APNs: exactly one PEM block of type PRIVATE KEY with a
// PKCS #8 ECDSA key on P-256, and nothing but white space around it (13 5.1, ADR 0067).
//
// Errors: the reason (no PEM block, a second block, another block type, not PKCS #8, not ECDSA
// or not P-256); the text never contains a byte of v.
//
// Example:
//
//	v, _, err := provider.Resolve(ctx, cfg.PrivateKeyRef)
//	key, err := apns.ParsePrivateKey(v)
func ParsePrivateKey(v secrets.Value) (*ecdsa.PrivateKey, error) {
	block, rest := pem.Decode(v.Bytes())
	switch {
	case block == nil:
		return nil, errors.New("apns: the key is not a PEM block")
	case block.Type != pemTypePKCS8:
		return nil, errors.New("apns: the key must be a PEM block of type PRIVATE KEY (PKCS #8)")
	case len(bytes.TrimSpace(rest)) != 0:
		return nil, errors.New("apns: the key must be exactly one PEM block")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, errors.New("apns: the key is not a PKCS #8 private key")
	}
	key, ok := parsed.(*ecdsa.PrivateKey)
	if !ok {
		return nil, errors.New("apns: the key is not an ECDSA key")
	}
	if key.Curve != elliptic.P256() {
		return nil, errors.New("apns: the key is not on the curve P-256")
	}
	return key, nil
}

// newSigner returns the ES256 signer of the APNs key as a jose.Signer: Alg is ES256 and Sign
// returns R || S as exactly 64 bytes, each half left-padded to 32 bytes (RFC 7518 3.4), through
// jose.ES256Signer. count, when set, is called once per signature, so tests can count the JWT
// creations. It is a jose.SignerFunc, because the module boundaries let only kernel types cross
// into kernel/jose (go-arch-lint deep scan).
func newSigner(key *ecdsa.PrivateKey, count func()) jose.SignerFunc {
	inner := jose.ES256Signer{Key: key}
	return jose.SignerFunc{Algorithm: jose.AlgES256, Fn: func(signingInput []byte) ([]byte, error) {
		if count != nil {
			count()
		}
		return inner.Sign(signingInput)
	}}
}
