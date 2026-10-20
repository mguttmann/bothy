// SPDX-License-Identifier: Apache-2.0

// Package hmackey supplies the master key of the derived v1 secrets of the outgoing webhooks
// (M2-26, webhook.hmac_master_key_ref): the value behind the reference through the secret
// provider, hex-decoded like every key of generator key (bothy secrets init writes 32 bytes as
// hex). The key is held only in the redacting secret value of the provider and in the caller's
// memory for one derivation; errors never contain it (SEC-SEC-1, SEC-SEC-3). Changing the master
// key changes every derived secret; owners then rotate (rotate_hmac_secret).
//
// Example:
//
//	m := hmackey.New(provider, cfg.HMACMasterKeyRef)
//	if err := m.Check(ctx); err != nil { ... } // the start fails on a missing or malformed key
package hmackey

import (
	"context"
	"encoding/hex"
	"errors"
	"strings"

	"github.com/OWNER/bothy/services/server/internal/apps/app"
	"github.com/OWNER/bothy/services/server/internal/platform/secrets"
)

// minKeyBytes is the smallest master key accepted: 32 bytes, the length generator key writes.
const minKeyBytes = 32

// errKey reports a value that is not at least 32 bytes as hex; it never repeats the value.
var errKey = errors.New("hmackey: webhook.hmac_master_key_ref does not hold at least 32 bytes as hex")

// Resolver returns the stored value behind a secret reference (*secrets.Provider).
type Resolver interface {
	Resolve(ctx context.Context, raw string) (secrets.Value, secrets.Version, error)
}

// Master implements app.HMACMasterKey. It resolves the reference on every call, so the cache of
// the provider (secrets.cache_ttl) decides when a replaced file takes effect.
type Master struct {
	src Resolver
	ref string
}

// Compile-time check: Master is the outgoing port of the use cases.
var _ app.HMACMasterKey = (*Master)(nil)

// New returns the master key behind ref through src.
func New(src Resolver, ref string) *Master { return &Master{src: src, ref: ref} }

// Key returns the decoded master key. Errors: the resolution error or errKey.
func (m *Master) Key(ctx context.Context) ([]byte, error) {
	v, _, err := m.src.Resolve(ctx, m.ref)
	if err != nil {
		return nil, err
	}
	key, err := hex.DecodeString(strings.TrimSpace(string(v.Bytes())))
	if err != nil || len(key) < minKeyBytes {
		return nil, errKey
	}
	return key, nil
}

// Check resolves the key once, so a missing or malformed master key fails the start instead of
// the first subscription with HMAC.
func (m *Master) Check(ctx context.Context) error {
	_, err := m.Key(ctx)
	return err
}
