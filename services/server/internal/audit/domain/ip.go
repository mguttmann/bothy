// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/netip"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// IPKeyBytes is the size of an IP pseudonym key and of the key-encryption key: 32 bytes.
const IPKeyBytes = 32

// gcmNonceBytes is the nonce size of AES-GCM, stored in front of the ciphertext.
const gcmNonceBytes = 12

// ErrIPKey is wrapped by the errors of the key functions; they never contain key material.
var ErrIPKey = errors.New("audit: IP key")

// IPHMAC returns the pseudonym of addr under key (20 "Audit-Log und IP-Adressen"):
// hex(HMAC-SHA-256(key, text)) with text = addr.Unmap() without zone in the form of
// netip.Addr.String, full length (64 hex characters). An IPv4-mapped IPv6 address gives the
// pseudonym of its IPv4 address.
func IPHMAC(key []byte, addr netip.Addr) string {
	m := hmac.New(sha256.New, key)
	m.Write([]byte(addr.Unmap().WithZone("").String()))
	return hex.EncodeToString(m.Sum(nil))
}

// KeyPeriod returns the period of privacy.audit_ip_key_period that contains now:
// from = floor(unix_ms(now) / period_ms) * period_ms, until = from + period, both UTC. period must
// be at least one millisecond.
func KeyPeriod(now time.Time, period time.Duration) (from, until time.Time) {
	p := period.Milliseconds()
	ms := now.UnixMilli()
	start := ms - ms%p
	if ms%p < 0 {
		start -= p
	}
	from = time.UnixMilli(start).UTC()
	return from, from.Add(period)
}

// DecodeKEK reads the key-encryption key of privacy.audit_ip_kek_ref: the generator "key" of the
// secret catalogue writes 32 random bytes as 64 lower-case hex characters; surrounding white
// space (a final newline) is ignored. The error wraps ErrIPKey.
func DecodeKEK(raw []byte) ([]byte, error) {
	k, err := hex.DecodeString(string(bytes.TrimSpace(raw)))
	if err != nil || len(k) != IPKeyBytes {
		return nil, fmt.Errorf("%w: the key-encryption key must be %d bytes as hex", ErrIPKey, IPKeyBytes)
	}
	return k, nil
}

// SealIPKey encrypts key with AES-256-GCM under kek: a random nonce from random, the row id as
// additional data, the result nonce || ciphertext (audit_ip_keys.key_ciphertext). Binding the id
// stops a ciphertext from being moved to another row.
func SealIPKey(kek, key []byte, id ids.ID, random io.Reader) ([]byte, error) {
	aead, err := newAEAD(kek)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcmNonceBytes)
	if _, err := io.ReadFull(random, nonce); err != nil {
		return nil, fmt.Errorf("%w: read a nonce: %w", ErrIPKey, err)
	}
	return aead.Seal(nonce, nonce, key, id[:]), nil
}

// OpenIPKey decrypts the result of SealIPKey for the row id. A wrong kek, id or a changed
// ciphertext is an error wrapping ErrIPKey.
func OpenIPKey(kek, sealed []byte, id ids.ID) ([]byte, error) {
	aead, err := newAEAD(kek)
	if err != nil {
		return nil, err
	}
	if len(sealed) < gcmNonceBytes {
		return nil, fmt.Errorf("%w: ciphertext too short", ErrIPKey)
	}
	key, err := aead.Open(nil, sealed[:gcmNonceBytes], sealed[gcmNonceBytes:], id[:])
	if err != nil || len(key) != IPKeyBytes {
		return nil, fmt.Errorf("%w: the key does not decrypt with the configured key-encryption key", ErrIPKey)
	}
	return key, nil
}

// newAEAD returns AES-256-GCM over kek.
func newAEAD(kek []byte) (cipher.AEAD, error) {
	if len(kek) != IPKeyBytes {
		return nil, fmt.Errorf("%w: the key-encryption key must be %d bytes", ErrIPKey, IPKeyBytes)
	}
	block, err := aes.NewCipher(kek)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrIPKey, err)
	}
	return cipher.NewGCM(block)
}

// IPKey is one row of audit_ip_keys (03 "4.12"): the key of one period, encrypted.
type IPKey struct {
	// ID is the UUIDv7 of the row; details.ip_key_id names it.
	ID ids.ID
	// ValidFrom and ValidUntil bound the period: ValidFrom <= t < ValidUntil.
	ValidFrom, ValidUntil time.Time
	// Ciphertext is the result of SealIPKey; nil after the destruction (M7-05).
	Ciphertext []byte
	// KeyRef is the reference of the key-encryption key at the time of creation
	// (privacy.audit_ip_kek_ref).
	KeyRef string
}

// Destroyed reports whether the key was destroyed; a destroyed key is never used.
func (k IPKey) Destroyed() bool { return len(k.Ciphertext) == 0 }
