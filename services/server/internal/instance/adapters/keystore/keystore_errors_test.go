// SPDX-License-Identifier: Apache-2.0

//go:build unix

package keystore_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/instance"
	"github.com/OWNER/bothy/services/server/internal/platform/secrets"
)

// writePEM writes one PEM block PRIVATE KEY with der to name in dir with mode.
func writePEM(t *testing.T, dir, name string, der []byte, mode os.FileMode) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), mode); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, mode); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestKeyStore_LoadRefusesOtherKeys: a PEM PRIVATE KEY that is no PKCS#8 key, a PKCS#8 key of
// another algorithm and a file the provider refuses to read are errors of Load that name the
// reference only.
func TestKeyStore_LoadRefusesOtherKeys(t *testing.T) {
	t.Parallel()
	s, dir := newStore(t)
	ctx := context.Background()
	ec, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	ecDER, err := x509.MarshalPKCS8PrivateKey(ec)
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]struct {
		path string
		want string
	}{
		"not PKCS#8":  {writePEM(t, dir, "access_token-a.pem", []byte("EXAMPLE"), 0o600), "is not a PKCS#8 key"},
		"not Ed25519": {writePEM(t, dir, "access_token-b.pem", ecDER, 0o600), "is not an Ed25519 key"},
	}
	for name, c := range cases {
		_, err := s.Load(ctx, "file:"+c.path)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: Load = %v, want an error containing %q", name, err, c.want)
		}
	}
	insecure := writePEM(t, dir, "access_token-c.pem", ecDER, 0o644)
	if _, err := s.Load(ctx, "file:"+insecure); !errors.Is(err, secrets.ErrInsecureFile) {
		t.Errorf("Load of a group-readable file = %v, want ErrInsecureFile", err)
	}
}

// TestKeyStore_WriteFailures: Generate and Remove hand on the write errors of the provider, and
// Remove refuses a reference outside the key store.
func TestKeyStore_WriteFailures(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores the directory mode")
	}
	s, dir := newStore(t)
	ctx := context.Background()
	_, _, ref, err := s.Generate(ctx, instance.PurposeAccessToken)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Remove(ctx, "env:BOTHY_SECRET_X"); err == nil {
		t.Error("Remove accepted a reference outside the key store")
	}
	if err := os.Chmod(dir, 0o500); err != nil { // #nosec G302 -- a test directory without write permission.
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) }) // #nosec G302 -- restores the test directory for its removal.
	if _, _, _, err := s.Generate(ctx, instance.PurposeInvocation); !errors.Is(err, secrets.ErrWriteFailed) {
		t.Errorf("Generate into a read-only key store = %v, want ErrWriteFailed", err)
	}
	if err := s.Remove(ctx, ref); !errors.Is(err, secrets.ErrWriteFailed) {
		t.Errorf("Remove in a read-only key store = %v, want ErrWriteFailed", err)
	}
}
