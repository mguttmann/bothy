// SPDX-License-Identifier: Apache-2.0

// Package apitoken is the pure format of the long-lived API tokens of the identity module
// (docs/spec/08-identitaet.md section 12, ADR 0061): bothy_<kind>_<random>_<checksum> with the
// kinds agt, app, pat, whi and hpc, 256 random bits as 43 Base62 digits and a CRC-32 checksum of the
// random part as 6 Base62 digits. It generates, parses and hashes tokens; it reads no clock and no
// database.
//
// The checksum rejects typos and random matches without a database call; it is not a security
// control. The authenticity check is the HMAC lookup (Hash).
//
// Example:
//
//	secret, err := apitoken.Generate(nil, apitoken.KindAgent) // nil reads crypto/rand
//	p, err := apitoken.Parse(secret)                          // ErrMalformed for anything else
//	row.TokenPrefix, row.TokenHash = p.Prefix(), apitoken.Hash(pepper, secret)
package apitoken

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"hash/crc32"
	"io"
	"math/big"
	"strings"
)

// product is the working name of the product in the token prefix; the only place it is written
// in the identity module (ADR 0044, docs/spec/02-architektur.md section 6).
const product = "bothy"

// Prefix starts every API token: the product name and an underscore.
const Prefix = product + "_"

// Kind is the kind part of a token; it follows the principal (08 section 12).
type Kind string

// The kinds of 08 section 12 and ADR 0061.
const (
	// KindAgent is the token of an agent principal.
	KindAgent Kind = "agt"
	// KindApp is the token of an app principal.
	KindApp Kind = "app"
	// KindPersonal is the personal token of a person (switch tokens.personal_enabled).
	KindPersonal Kind = "pat"
	// KindIncomingWebhook is the token of an incoming webhook (issued by M2-27, never by
	// createToken, never accepted by the authenticator).
	KindIncomingWebhook Kind = "whi"
	// KindHostPairingCode is the pairing code of a terminal host (24-terminal.md section 3.2):
	// issued only by host.manage (M2-39), redeemed only by host.pair (M2-41), never accepted by
	// the authenticator.
	KindHostPairingCode Kind = "hpc"
)

// Valid reports whether k is one of the five kinds.
func (k Kind) Valid() bool {
	switch k {
	case KindAgent, KindApp, KindPersonal, KindIncomingWebhook, KindHostPairingCode:
		return true
	}
	return false
}

// Lengths of the format (ADR 0061 item 1).
const (
	// RandomBytes is the number of random bytes: 256 bits (ID-T27).
	RandomBytes = 32
	// RandomLen is the number of Base62 digits of the random part (62^43 > 2^256).
	RandomLen = 43
	// ChecksumLen is the number of Base62 digits of the checksum (62^6 > 2^32).
	ChecksumLen = 6
	// PrefixRandomLen is the number of random characters in token_prefix.
	PrefixRandomLen = 6
	// kindLen is the length of every kind.
	kindLen = 3
	// Length is the total length of a token: prefix, kind, underscore, random part, underscore
	// and checksum (60).
	Length = len(Prefix) + kindLen + 1 + RandomLen + 1 + ChecksumLen
)

// alphabet is the Base62 alphabet in digit order (ADR 0061 item 1).
const alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

// ErrMalformed reports a value that is not an API token of this format; it never names the value.
var ErrMalformed = errors.New("apitoken: malformed token")

// errKind reports a kind outside the five kinds.
var errKind = errors.New("apitoken: unknown kind")

// limit is 2^256: a random part decodes to a value below it.
var limit = new(big.Int).Lsh(big.NewInt(1), 8*RandomBytes)

// Parsed is a token split into its parts; Parse returns it only for a valid token.
type Parsed struct {
	// Kind is the kind part.
	Kind Kind
	// Random is the random part (43 Base62 digits).
	Random string
	// Checksum is the checksum part (6 Base62 digits).
	Checksum string
}

// Prefix returns token_prefix: bothy_<kind>_ and the first 6 characters of the random part (03
// api_tokens), which identifies a token in a list without revealing it.
func (p Parsed) Prefix() string {
	return Prefix + string(p.Kind) + "_" + p.Random[:PrefixRandomLen]
}

// HasPrefix reports whether s starts with the product prefix, the switch between the API token
// path and the access token path of the authenticator.
func HasPrefix(s string) bool { return strings.HasPrefix(s, Prefix) }

// Generate returns a new token of kind k with RandomBytes bytes read from src (nil means
// crypto/rand.Reader), read as a big-endian unsigned integer and written in Base62, left-padded to
// RandomLen digits, followed by its checksum. Errors: an unknown kind; a source that delivers
// fewer than RandomBytes bytes (never a weaker token).
func Generate(src io.Reader, k Kind) (string, error) {
	if !k.Valid() {
		return "", errKind
	}
	if src == nil {
		src = rand.Reader
	}
	b := make([]byte, RandomBytes)
	if _, err := io.ReadFull(src, b); err != nil {
		return "", err
	}
	random := encode(new(big.Int).SetBytes(b), RandomLen)
	return Prefix + string(k) + "_" + random + "_" + Checksum(random), nil
}

// Checksum returns the CRC-32 (IEEE) of the ASCII bytes of random in Base62, left-padded to
// ChecksumLen digits.
func Checksum(random string) string {
	return encode(new(big.Int).SetUint64(uint64(crc32.ChecksumIEEE([]byte(random)))), ChecksumLen)
}

// Parse checks s against ^bothy_(agt|app|pat|whi|hpc)_[0-9A-Za-z]{43}_[0-9A-Za-z]{6}$, the checksum
// and the range of the random part (below 2^256) and returns its parts. Errors: ErrMalformed for
// every other value.
func Parse(s string) (Parsed, error) {
	if len(s) != Length || !HasPrefix(s) {
		return Parsed{}, ErrMalformed
	}
	rest := s[len(Prefix):]
	k := Kind(rest[:kindLen])
	if !k.Valid() || rest[kindLen] != '_' || rest[kindLen+1+RandomLen] != '_' {
		return Parsed{}, ErrMalformed
	}
	p := Parsed{Kind: k, Random: rest[kindLen+1 : kindLen+1+RandomLen], Checksum: rest[kindLen+1+RandomLen+1:]}
	if !base62(p.Random) || !base62(p.Checksum) || !hmac.Equal([]byte(Checksum(p.Random)), []byte(p.Checksum)) {
		return Parsed{}, ErrMalformed
	}
	if decode(p.Random).Cmp(limit) >= 0 {
		return Parsed{}, ErrMalformed
	}
	return p, nil
}

// Hash returns HMAC-SHA-256(pepper, token), the only form a token is stored in (03 Grundsatz 9,
// api_tokens.token_hash). The key is the server pepper of auth.token_pepper_ref.
func Hash(pepper []byte, token string) []byte {
	m := hmac.New(sha256.New, pepper)
	m.Write([]byte(token))
	return m.Sum(nil)
}

// encode writes n in Base62, left-padded with '0' to width digits; n fits by construction.
func encode(n *big.Int, width int) string {
	out := make([]byte, width)
	base := big.NewInt(int64(len(alphabet)))
	v, digit := new(big.Int).Set(n), new(big.Int)
	for i := width - 1; i >= 0; i-- {
		v.DivMod(v, base, digit)
		out[i] = alphabet[digit.Int64()]
	}
	return string(out)
}

// decode reads the Base62 digits of s (checked by base62 before).
func decode(s string) *big.Int {
	n, base := new(big.Int), big.NewInt(int64(len(alphabet)))
	for i := 0; i < len(s); i++ {
		n.Mul(n, base)
		n.Add(n, big.NewInt(int64(strings.IndexByte(alphabet, s[i]))))
	}
	return n
}

// base62 reports whether s holds only digits of the alphabet.
func base62(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c < '0' || c > '9') && (c < 'A' || c > 'Z') && (c < 'a' || c > 'z') {
			return false
		}
	}
	return true
}
