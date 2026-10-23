// SPDX-License-Identifier: Apache-2.0

package fcm

import (
	"bytes"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"log/slog"

	"github.com/OWNER/bothy/services/server/internal/platform/secrets"
)

// accountType is the value of type in a service account JSON.
const accountType = "service_account"

// minRSABits is the shortest RSA key kernel/jose signs RS256 with (ADR 0067).
const minRSABits = 2048

// redacted is the printed form of a ServiceAccount.
const redacted = "fcm.ServiceAccount(redacted)"

// ServiceAccount is the parsed Google service account of push.fcm.service_account_ref (13 11):
// the client e-mail (claim iss), the key ID (header kid) and the RSA private key that signs the
// assertion. Every field is unexported, and the printed, logged and JSON forms are redacted, so
// no part of it reaches a log record or an error text.
type ServiceAccount struct {
	clientEmail string
	keyID       string
	key         *rsa.PrivateKey
}

// String returns a fixed text without any field.
func (ServiceAccount) String() string { return redacted }

// GoString returns a fixed text without any field.
func (ServiceAccount) GoString() string { return redacted }

// LogValue returns a fixed text without any field.
func (ServiceAccount) LogValue() slog.Value { return slog.StringValue(redacted) }

// MarshalJSON returns a fixed JSON string without any field.
func (ServiceAccount) MarshalJSON() ([]byte, error) { return []byte(`"` + redacted + `"`), nil }

// accountFile is the part of the service account JSON the adapter reads; other members are
// ignored.
type accountFile struct {
	Type         string  `json:"type"`
	ClientEmail  string  `json:"client_email"`
	PrivateKeyID string  `json:"private_key_id"`
	PrivateKey   string  `json:"private_key"`
	TokenURI     *string `json:"token_uri"`
}

// ParseServiceAccount parses the service account JSON v (Design note 3 a of brief M3-21). It
// requires type = service_account, a non-empty client_email and private_key_id, a token_uri that
// is absent or equal to tokenURL (the adapter never takes a host from a secret file), and a
// private_key of exactly one PEM block of type PRIVATE KEY in PKCS #8 that holds an RSA key of at
// least 2048 bits.
//
// Errors name the field and the reason and never contain a byte of the value.
//
// Example:
//
//	sa, err := fcm.ParseServiceAccount(v, "https://oauth2.googleapis.com/token")
func ParseServiceAccount(v secrets.Value, tokenURL string) (ServiceAccount, error) {
	raw := bytes.TrimSpace(v.Bytes())
	if len(raw) == 0 || raw[0] != '{' {
		return ServiceAccount{}, errors.New("service account: not a JSON object")
	}
	var f accountFile
	if json.Unmarshal(raw, &f) != nil {
		return ServiceAccount{}, errors.New("service account: not a valid JSON object with string members")
	}
	switch {
	case f.Type != accountType:
		return ServiceAccount{}, errors.New("service account: type must be service_account")
	case f.ClientEmail == "":
		return ServiceAccount{}, errors.New("service account: client_email is missing or empty")
	case f.PrivateKeyID == "":
		return ServiceAccount{}, errors.New("service account: private_key_id is missing or empty")
	case f.TokenURI != nil && *f.TokenURI != tokenURL:
		return ServiceAccount{}, errors.New("service account: token_uri differs from the token endpoint of the adapter")
	}
	key, err := parseKey(f.PrivateKey)
	if err != nil {
		return ServiceAccount{}, err
	}
	return ServiceAccount{clientEmail: f.ClientEmail, keyID: f.PrivateKeyID, key: key}, nil
}

// parseKey decodes exactly one PEM block of type PRIVATE KEY with a PKCS #8 RSA key; nothing but
// white space may surround it.
func parseKey(s string) (*rsa.PrivateKey, error) {
	block, rest := pem.Decode([]byte(s))
	if block == nil {
		return nil, errors.New("service account: private_key is not a PEM block")
	}
	// pem.Decode skips text before the block, so the prefix is checked as well.
	if len(bytes.TrimSpace(rest)) != 0 || !bytes.HasPrefix(bytes.TrimSpace([]byte(s)), []byte("-----BEGIN ")) {
		return nil, errors.New("service account: private_key must be exactly one PEM block")
	}
	if block.Type != "PRIVATE KEY" {
		return nil, errors.New("service account: private_key must be a PEM block of type PRIVATE KEY (PKCS #8)")
	}
	k, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, errors.New("service account: private_key is not a PKCS #8 key")
	}
	rk, ok := k.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("service account: private_key must be an RSA key")
	}
	if rk.N.BitLen() < minRSABits {
		return nil, errors.New("service account: private_key must be an RSA key of at least 2048 bits")
	}
	return rk, nil
}
