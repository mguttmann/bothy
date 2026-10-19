// SPDX-License-Identifier: Apache-2.0

// Package secretbox seals the TOTP secrets of local accounts (identity/app SecretSealer,
// docs/spec/08-identity.md section 2.5, 19 secrets table, brief M1-39): AES-256-GCM with a
// 32-byte key resolved from auth.totp_key_ref through the secret provider (32 bytes as hex, the
// form bothy secrets init writes), a 12-byte random nonce and the additional data
// "totp/v1" || 0x00 || identity_id (16 bytes), so a ciphertext copied to another identity does not
// open. The stored layout is 0x01 || nonce || ciphertext and tag. Seal uses the current key and
// returns its reference for the row's totp_key_ref; Open resolves the reference of the row, so a
// new key applies to new rows while old rows stay readable.
//
// Example:
//
//	box, err := secretbox.New(ctx, resolver, cfg.TOTPKeyRef) // a wrong key length stops the start
//	ct, ref, err := box.Seal(identityID, secret)
//	secret, err := box.Open(identityID, ct, ref)
package secretbox

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/secrets"
)

// Resolver returns the stored value behind a secret reference. Errors never contain the value.
type Resolver interface {
	Resolve(ctx context.Context, ref string) ([]byte, error)
}

// Provider is the Resolver over the secret provider of the process (platform/secrets).
type Provider struct {
	// Secrets resolves the references.
	Secrets *secrets.Provider
}

// Resolve implements Resolver.
func (p Provider) Resolve(ctx context.Context, ref string) ([]byte, error) {
	v, _, err := p.Secrets.Resolve(ctx, ref)
	if err != nil {
		return nil, err
	}
	return v.Bytes(), nil
}

const (
	// version is the first byte of the stored layout.
	version = 0x01
	// keyBytes is the AES-256 key length.
	keyBytes = 32
	// nonceBytes is the GCM standard nonce length.
	nonceBytes = 12
	// aadContext starts the additional data, separating this use of the key from any other.
	aadContext = "totp/v1"
	// resolveTimeout bounds the resolution of an older key reference in Open, whose port has no
	// context.
	resolveTimeout = 5 * time.Second
)

// Box implements app.SecretSealer. It is safe for concurrent use.
type Box struct {
	src     Resolver
	ref     string
	current cipher.AEAD
}

// errKey reports a key that is not 32 bytes as hex; it never repeats the value.
var errKey = errors.New("secretbox: auth.totp_key_ref does not hold 32 bytes as hex")

// New resolves the current key ref through src once. Errors: the resolution error or errKey; the
// start fails with them.
func New(ctx context.Context, src Resolver, ref string) (*Box, error) {
	aead, err := resolve(ctx, src, ref)
	if err != nil {
		return nil, err
	}
	return &Box{src: src, ref: ref, current: aead}, nil
}

// resolve returns the AEAD of the key behind ref.
func resolve(ctx context.Context, src Resolver, ref string) (cipher.AEAD, error) {
	raw, err := src.Resolve(ctx, ref)
	if err != nil {
		return nil, fmt.Errorf("secretbox: resolving the TOTP key: %w", err)
	}
	key, err := hex.DecodeString(strings.TrimSpace(string(raw)))
	if err != nil || len(key) != keyBytes {
		return nil, errKey
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, errKey
	}
	return cipher.NewGCM(block)
}

// aad is the additional data of identityID.
func aad(identityID ids.ID) []byte {
	out := make([]byte, 0, len(aadContext)+1+len(identityID))
	out = append(out, aadContext...)
	out = append(out, 0x00)
	return append(out, identityID[:]...)
}

// Seal implements app.SecretSealer. Errors: a failing random source.
func (b *Box) Seal(identityID ids.ID, plaintext []byte) ([]byte, string, error) {
	nonce := make([]byte, nonceBytes)
	if _, err := rand.Read(nonce); err != nil {
		return nil, "", fmt.Errorf("secretbox: random nonce: %w", err)
	}
	out := make([]byte, 0, 1+nonceBytes+len(plaintext)+b.current.Overhead())
	out = append(out, version)
	out = append(out, nonce...)
	return b.current.Seal(out, nonce, plaintext, aad(identityID)), b.ref, nil
}

// errOpen is every failure to open: wrong identity, version, length, key or a tampered text.
var errOpen = errors.New("secretbox: the TOTP secret cannot be opened")

// Open implements app.SecretSealer.
func (b *Box) Open(identityID ids.ID, ciphertext []byte, keyRef string) ([]byte, error) {
	if len(ciphertext) < 1+nonceBytes+b.current.Overhead() || ciphertext[0] != version {
		return nil, errOpen
	}
	aead := b.current
	if keyRef != b.ref {
		ctx, cancel := context.WithTimeout(context.Background(), resolveTimeout)
		defer cancel()
		var err error
		if aead, err = resolve(ctx, b.src, keyRef); err != nil {
			return nil, err
		}
	}
	plain, err := aead.Open(nil, ciphertext[1:1+nonceBytes], ciphertext[1+nonceBytes:], aad(identityID))
	if err != nil {
		return nil, errOpen
	}
	return plain, nil
}
