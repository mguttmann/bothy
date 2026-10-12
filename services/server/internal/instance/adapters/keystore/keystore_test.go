// SPDX-License-Identifier: Apache-2.0

//go:build unix

package keystore_test

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/instance"
	"github.com/OWNER/bothy/services/server/internal/instance/adapters/keystore"
	"github.com/OWNER/bothy/services/server/internal/platform/secrets"
)

// newStore returns a key store over a fresh 0700 directory.
func newStore(t *testing.T) (*keystore.Store, string) {
	t.Helper()
	base := t.TempDir()
	dir := filepath.Join(base, "signing-keys")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	p, err := secrets.NewProvider(secrets.Options{Dir: base, SigningKeyDir: dir, CacheTTL: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	s, err := keystore.New(p, dir)
	if err != nil {
		t.Fatal(err)
	}
	return s, dir
}

// TestThumbprint_RFC7638: the kid of the Ed25519 example key of RFC 8037 appendix A equals the
// RFC 7638 thumbprint computed by hand here (SHA-256 over {"crv","kty","x"} in that order,
// base64url without padding) and the value printed in RFC 8037 appendix A.3.
func TestThumbprint_RFC7638(t *testing.T) {
	t.Parallel()
	const x = "11qYAYKxCrfVS_7TyWQHOg7hcvPapiMlrwIaaPcHURo"
	pub, err := base64.RawURLEncoding.DecodeString(x)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(fmt.Appendf(nil, `{"crv":"Ed25519","kty":"OKP","x":"%s"}`, x))
	byHand := base64.RawURLEncoding.EncodeToString(sum[:])
	if byHand != "kPrK_qmxVWaYVA9wwBF6Iuo3vVzz7TxHCTwXBygrS4k" {
		t.Fatalf("hand-computed thumbprint %s differs from RFC 8037 A.3", byHand)
	}
	if got := keystore.Thumbprint(ed25519.PublicKey(pub)); got != byHand {
		t.Errorf("Thumbprint = %s, want %s", got, byHand)
	}
}

func TestKeyStore_GenerateLoadRemove(t *testing.T) {
	t.Parallel()
	s, dir := newStore(t)
	ctx := context.Background()
	kid, pub, ref, err := s.Generate(ctx, instance.PurposeInvocation)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if kid != keystore.Thumbprint(pub) || len(kid) != 43 {
		t.Errorf("kid %q is not the thumbprint of the public key", kid)
	}
	path := filepath.Join(dir, "invocation-"+kid+".pem")
	if ref != "file:"+path {
		t.Errorf("ref %q, want file:%s", ref, path)
	}
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o600 {
		t.Errorf("key file mode %04o, want 0600", fi.Mode().Perm())
	}
	raw, err := os.ReadFile(path) //nolint:gosec // G304: the test's own temp file.
	if err != nil {
		t.Fatal(err)
	}
	block, rest := pem.Decode(raw)
	if block == nil || block.Type != "PRIVATE KEY" || len(strings.TrimSpace(string(rest))) != 0 {
		t.Fatalf("key file is not one PEM block PRIVATE KEY")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		t.Fatalf("key file is not PKCS#8: %v", err)
	}
	if priv, ok := parsed.(ed25519.PrivateKey); !ok || !priv.Public().(ed25519.PublicKey).Equal(pub) {
		t.Fatal("key file does not hold the private key of the returned public key")
	}

	key, err := s.Load(ctx, ref)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !key.Public().Equal(pub) {
		t.Error("loaded key has another public key")
	}
	msg := []byte("header.payload")
	if !ed25519.Verify(pub, msg, key.Sign(msg)) {
		t.Error("signature of the loaded key does not verify")
	}
	for _, out := range []string{fmt.Sprintf("%v %+v %#v %s", key, key, key, key)} {
		if strings.Contains(out, base64.RawURLEncoding.EncodeToString(parsed.(ed25519.PrivateKey).Seed())) {
			t.Errorf("formatting the loaded key prints the seed: %q", out)
		}
	}

	if err := s.Remove(ctx, ref); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("key file still exists: %v", err)
	}
	if err := s.Remove(ctx, ref); err != nil {
		t.Errorf("Remove of a missing file = %v, want nil", err)
	}
	if _, err := s.Load(ctx, ref); err == nil {
		t.Error("Load of a removed key succeeded")
	}
}

func TestKeyStore_LoadRefusesForeignContent(t *testing.T) {
	t.Parallel()
	s, dir := newStore(t)
	path := filepath.Join(dir, "access_token-EXAMPLE.pem")
	if err := os.WriteFile(path, []byte("EXAMPLE-not-a-key"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := s.Load(context.Background(), "file:"+path)
	if err == nil {
		t.Fatal("Load accepted a file that is not a PEM key")
	}
	if strings.Contains(err.Error(), "EXAMPLE-not-a-key") {
		t.Errorf("error %q repeats the file content", err)
	}
	if _, err := s.Load(context.Background(), "env:BOTHY_SECRET_X"); err == nil {
		t.Error("Load accepted a reference outside the key store")
	}
}

func TestKeyStore_NewRefusesBadDir(t *testing.T) {
	t.Parallel()
	p, err := secrets.NewProvider(secrets.Options{Dir: "/run/bothy/secrets"})
	if err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{"", "relative", "/"} {
		if _, err := keystore.New(p, dir); err == nil {
			t.Errorf("New(%q) passed", dir)
		}
	}
	if _, err := keystore.New(nil, "/var/lib/bothy/signing-keys"); err == nil {
		t.Error("New without a provider passed")
	}
}
