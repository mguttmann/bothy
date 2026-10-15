// SPDX-License-Identifier: Apache-2.0

// Package keystore is the adapter of the instance module to the key store
// secrets.signing_key_dir (ADR 0066): it generates Ed25519 signing keys, writes their private
// part as a PKCS#8 PEM file through the secret provider (secrets.Provider.Put, the only writable
// place) and reads it back through the provider's cache (secrets.cache_ttl). It implements
// app.KeyStore.
//
// Example (internal/wire):
//
//	ks, err := keystore.New(provider, cfg.SigningKeyDir)
//	keys, err := app.NewKeys(keyCfg, app.KeysDeps{Store: ks, ...})
package keystore

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"path"

	"github.com/OWNER/bothy/services/server/internal/instance"
	"github.com/OWNER/bothy/services/server/internal/instance/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/jose"
	"github.com/OWNER/bothy/services/server/internal/platform/secrets"
)

// pemType is the PEM block type of a PKCS#8 private key (RFC 7468 section 10).
const pemType = "PRIVATE KEY"

// Store implements app.KeyStore over the secret provider. It is safe for concurrent use.
type Store struct {
	provider *secrets.Provider
	dir      string
}

// Compile-time check: Store is the outgoing port of app.
var _ app.KeyStore = (*Store)(nil)

// New returns the key store over provider for the directory dir, the effective
// secrets.signing_key_dir; provider must have been built with the same directory, because only
// there Put writes. Errors: a nil provider, or a dir that is not an absolute clean path other
// than the root.
func New(provider *secrets.Provider, dir string) (*Store, error) {
	if provider == nil {
		return nil, errors.New("keystore: no secret provider")
	}
	if !path.IsAbs(dir) || path.Clean(dir) != dir || dir == "/" {
		return nil, errors.New("keystore: secrets.signing_key_dir must be an absolute clean path other than the root")
	}
	return &Store{provider: provider, dir: dir}, nil
}

// Thumbprint returns the kid of pub: the RFC 7638 thumbprint of its JWK, SHA-256 over
// {"crv":"Ed25519","kty":"OKP","x":...} in that member order, base64url without padding (19
// section 10, ADR 0066 rule 3).
func Thumbprint(pub ed25519.PublicKey) string {
	k, err := jose.JWKFromPublicKey(pub)
	if err != nil {
		// Only a public key of the wrong length fails; Generate never produces one.
		return ""
	}
	return k.Thumbprint()
}

// Generate creates an Ed25519 key from crypto/rand and writes its private part as PEM PRIVATE KEY
// (PKCS#8) to <dir>/<purpose>-<kid>.pem through secrets.Provider.Put, which creates the file with
// mode 0600 and never overwrites (app.KeyStore.Generate). The returned reference is
// "file:<dir>/<purpose>-<kid>.pem". Errors name the purpose and the reference, never key
// material.
func (s *Store) Generate(ctx context.Context, p instance.Purpose) (string, ed25519.PublicKey, string, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return "", nil, "", fmt.Errorf("keystore: generate a %s key: %w", p, err)
	}
	defer clear(priv)
	der, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return "", nil, "", fmt.Errorf("keystore: encode a %s key: %w", p, err)
	}
	defer clear(der)
	block := pem.EncodeToMemory(&pem.Block{Type: pemType, Bytes: der})
	defer clear(block)
	kid := Thumbprint(pub)
	ref := secrets.Ref{Scheme: "file", Path: path.Join(s.dir, string(p)+"-"+kid+".pem")}
	if err := s.provider.Put(ctx, ref, secrets.NewValue(block)); err != nil {
		return "", nil, "", err
	}
	return kid, pub, ref.String(), nil
}

// Load reads the private key behind ref through the provider (cached for secrets.cache_ttl) and
// returns it as an app.PrivateKey that keeps the seed in a secrets.Value (app.KeyStore.Load).
// Errors: a reference outside the key store, app.ErrKeyMissing for a file that does not exist,
// the read error of the provider, or a file that is not one PEM block PRIVATE KEY with an Ed25519
// PKCS#8 key; they never repeat the content.
func (s *Store) Load(ctx context.Context, ref string) (app.PrivateKey, error) {
	r, err := s.ref(ref)
	if err != nil {
		return nil, err
	}
	v, _, err := s.provider.Get(ctx, r)
	if errors.Is(err, secrets.ErrNotFound) {
		return nil, fmt.Errorf("%w: %w", app.ErrKeyMissing, err)
	}
	if err != nil {
		return nil, err
	}
	raw := v.Bytes()
	defer clear(raw)
	block, _ := pem.Decode(raw)
	if block == nil || block.Type != pemType {
		return nil, fmt.Errorf("keystore: %s is not a PEM %s", ref, pemType)
	}
	defer clear(block.Bytes)
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("keystore: %s is not a PKCS#8 key", ref)
	}
	priv, ok := parsed.(ed25519.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("keystore: %s is not an Ed25519 key", ref)
	}
	defer clear(priv)
	pub, _ := priv.Public().(ed25519.PublicKey)
	return &privateKey{seed: secrets.NewValue(priv.Seed()), pub: append(ed25519.PublicKey(nil), pub...)}, nil
}

// Remove deletes the private key behind ref (app.KeyStore.Remove); a missing file is no error,
// because removal is best effort after a commit and may run twice.
func (s *Store) Remove(ctx context.Context, ref string) error {
	r, err := s.ref(ref)
	if err != nil {
		return err
	}
	if err := s.provider.Delete(ctx, r); err != nil && !errors.Is(err, secrets.ErrNotFound) {
		return err
	}
	return nil
}

// ref parses a stored reference; only file references directly in the key store are accepted.
func (s *Store) ref(raw string) (secrets.Ref, error) {
	r, err := secrets.ParseRef(raw, s.dir)
	if err != nil {
		return secrets.Ref{}, err
	}
	if r.Scheme != "file" || path.Dir(r.Path) != s.dir {
		return secrets.Ref{}, fmt.Errorf("keystore: the reference %s is not in secrets.signing_key_dir", r)
	}
	return r, nil
}

// privateKey is a loaded key. The seed sits in a secrets.Value, which renders as "[redacted]" in
// every output path; the expanded key exists only during Sign.
type privateKey struct {
	seed secrets.Value
	pub  ed25519.PublicKey
}

// Public returns the public key.
func (k *privateKey) Public() ed25519.PublicKey { return k.pub }

// Sign returns the Ed25519 signature of msg.
func (k *privateKey) Sign(msg []byte) []byte {
	seed := k.seed.Bytes()
	defer clear(seed)
	priv := ed25519.NewKeyFromSeed(seed)
	defer clear(priv)
	return ed25519.Sign(priv, msg)
}

// String returns a fixed text, so a formatted key never shows its fields.
func (k *privateKey) String() string { return "signing key [redacted]" }
