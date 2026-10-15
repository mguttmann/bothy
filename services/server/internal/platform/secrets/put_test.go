// SPDX-License-Identifier: Apache-2.0

//go:build unix

package secrets_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/platform/secrets"
)

// keyStore returns a fresh key store directory with mode 0700 and a provider whose
// secrets.signing_key_dir is that directory (secrets.dir is a sibling).
func keyStore(t *testing.T) (*secrets.Provider, string) {
	t.Helper()
	base := t.TempDir()
	keys := filepath.Join(base, "signing-keys")
	mkdir(t, keys, 0o700)
	secretsDir := filepath.Join(base, "secrets")
	if err := os.Mkdir(secretsDir, 0o700); err != nil {
		t.Fatal(err)
	}
	p, err := secrets.NewProvider(secrets.Options{
		Dir: secretsDir, SigningKeyDir: keys, CacheTTL: ttl, Clock: clock.NewFake(start), Logger: discardLogger(),
	})
	if err != nil {
		t.Fatal(err)
	}
	return p, keys
}

func TestSecretsPut_WritesBelowSigningKeyDir(t *testing.T) {
	t.Parallel()
	p, keys := keyStore(t)
	ctx := context.Background()
	ref := secrets.Ref{Scheme: "file", Path: filepath.Join(keys, "invocation-abc.pem")}
	canary := newCanary(t)
	if err := p.Put(ctx, ref, secrets.NewValue([]byte(canary))); err != nil {
		t.Fatalf("Put: %v", err)
	}
	fi, err := os.Stat(ref.Path)
	if err != nil {
		t.Fatal(err)
	}
	if m := fi.Mode().Perm(); m != 0o600 {
		t.Errorf("mode %04o, want 0600", m)
	}
	if got, _ := mustGet(t, p, ref); got != canary {
		t.Errorf("Get after Put returned another value")
	}
	entries, err := os.ReadDir(keys)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("key store holds %d entries after one Put, want 1 (no temporary file left)", len(entries))
	}
}

func TestSecretsPut_NeverOverwrites(t *testing.T) {
	t.Parallel()
	p, keys := keyStore(t)
	ctx := context.Background()
	ref := secrets.Ref{Scheme: "file", Path: filepath.Join(keys, "access_token-k1.pem")}
	if err := p.Put(ctx, ref, secrets.NewValue([]byte("EXAMPLE-first"))); err != nil {
		t.Fatal(err)
	}
	err := p.Put(ctx, ref, secrets.NewValue([]byte("EXAMPLE-second")))
	if !errors.Is(err, secrets.ErrExists) {
		t.Fatalf("second Put = %v, want ErrExists", err)
	}
	if strings.Contains(err.Error(), "EXAMPLE") {
		t.Errorf("error %q names the value", err)
	}
	b, rerr := os.ReadFile(ref.Path)
	if rerr != nil {
		t.Fatal(rerr)
	}
	if string(b) != "EXAMPLE-first" {
		t.Errorf("the existing file was changed")
	}
}

func TestSecretsPut_ReadOnlyOutsideSigningKeyDir(t *testing.T) {
	t.Parallel()
	p, keys := keyStore(t)
	ctx := context.Background()
	v := secrets.NewValue([]byte("EXAMPLE-value"))
	for name, ref := range map[string]secrets.Ref{
		"secrets.dir":       {Scheme: "file", Path: filepath.Join(filepath.Dir(keys), "secrets", "db_app_password")},
		"parent":            {Scheme: "file", Path: filepath.Join(filepath.Dir(keys), "x.pem")},
		"nested":            {Scheme: "file", Path: filepath.Join(keys, "sub", "x.pem")},
		"the dir itself":    {Scheme: "file", Path: keys},
		"sibling prefix":    {Scheme: "file", Path: keys + "-other/x.pem"},
		"env":               {Scheme: "env", Path: "BOTHY_SECRET_X"},
		"invalid file path": {Scheme: "file", Path: keys + "/../x.pem"},
	} {
		if err := p.Put(ctx, ref, v); !errors.Is(err, secrets.ErrReadOnly) {
			t.Errorf("%s: Put = %v, want ErrReadOnly", name, err)
		}
		if err := p.Delete(ctx, ref); !errors.Is(err, secrets.ErrReadOnly) {
			t.Errorf("%s: Delete = %v, want ErrReadOnly", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(keys), "x.pem")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("a refused Put wrote a file")
	}
}

func TestSecretsPut_ReadOnlyWithoutSigningKeyDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := newProvider(t, dir, clock.NewFake(start), discardLogger())
	ref := secrets.Ref{Scheme: "file", Path: filepath.Join(dir, "x.pem")}
	if err := p.Put(context.Background(), ref, secrets.NewValue([]byte("EXAMPLE"))); !errors.Is(err, secrets.ErrReadOnly) {
		t.Fatalf("Put without a signing key dir = %v, want ErrReadOnly", err)
	}
}

func TestSecretsDelete_RemovesFileAndCache(t *testing.T) {
	t.Parallel()
	p, keys := keyStore(t)
	ctx := context.Background()
	ref := secrets.Ref{Scheme: "file", Path: filepath.Join(keys, "webhook_out-k1.pem")}
	if err := p.Put(ctx, ref, secrets.NewValue([]byte("EXAMPLE-key"))); err != nil {
		t.Fatal(err)
	}
	mustGet(t, p, ref)
	if err := p.Delete(ctx, ref); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := os.Stat(ref.Path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("file still exists after Delete: %v", err)
	}
	if _, _, err := p.Get(ctx, ref); !errors.Is(err, secrets.ErrNotFound) {
		t.Errorf("Get after Delete = %v, want ErrNotFound (no cached value)", err)
	}
	if err := p.Delete(ctx, ref); !errors.Is(err, secrets.ErrNotFound) {
		t.Errorf("second Delete = %v, want ErrNotFound", err)
	}
}

func TestSecretsPut_DriverWithoutWriterIsReadOnly(t *testing.T) {
	t.Parallel()
	keys := t.TempDir()
	read := &fakeDriver{scheme: "file", get: func(context.Context, int32, secrets.Ref) (secrets.Value, secrets.Version, error) {
		return secrets.Value{}, "", secrets.ErrNotFound
	}}
	p, err := secrets.NewProvider(secrets.Options{Dir: keys, SigningKeyDir: keys, Drivers: []secrets.Driver{read}})
	if err != nil {
		t.Fatal(err)
	}
	ref := secrets.Ref{Scheme: "file", Path: filepath.Join(keys, "x.pem")}
	if err := p.Put(context.Background(), ref, secrets.NewValue([]byte("EXAMPLE"))); !errors.Is(err, secrets.ErrReadOnly) {
		t.Fatalf("Put through a read-only driver = %v, want ErrReadOnly", err)
	}
}

func TestSecretsPut_StartupCheck(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	good := filepath.Join(base, "good")
	mkdir(t, good, 0o700)
	if err := secrets.CheckSigningKeyDir(good); err != nil {
		t.Fatalf("CheckSigningKeyDir(0700 dir) = %v", err)
	}
	if entries, _ := os.ReadDir(good); len(entries) != 0 {
		t.Errorf("the check left %d entries in the directory", len(entries))
	}

	missing := filepath.Join(base, "missing")
	assertCheck(t, missing, "does not exist")

	wide := filepath.Join(base, "wide")
	mkdir(t, wide, 0o755)
	assertCheck(t, wide, "mode 0755")

	file := filepath.Join(base, "file")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	assertCheck(t, file, "not a directory")

	readOnly := filepath.Join(base, "readonly")
	mkdir(t, readOnly, 0o500)
	t.Cleanup(func() { mkdirMode(readOnly, 0o700) })
	assertCheck(t, readOnly, "mode 0500")

	if err := secrets.CheckSigningKeyDir("relative/dir"); err == nil {
		t.Error("a relative directory passed the check")
	}
}

func TestSecretsPut_StartupCheckForeignOwner(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "keys")
	mkdir(t, dir, 0o700)
	if err := secrets.CheckSigningKeyDirForUID(dir, os.Geteuid()+1); err == nil ||
		!strings.Contains(err.Error(), "owner uid") || !strings.Contains(err.Error(), "secrets.signing_key_dir") {
		t.Fatalf("check with another owner = %v, want an owner error naming secrets.signing_key_dir", err)
	}
}

// mkdirMode sets the mode of an existing directory; the cleanup of a read-only one needs it.
func mkdirMode(dir string, mode os.FileMode) { _ = os.Chmod(dir, mode) }

// mkdir creates dir with exactly mode.
func mkdir(t *testing.T, dir string, mode os.FileMode) {
	t.Helper()
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, mode); err != nil {
		t.Fatal(err)
	}
}

// assertCheck requires the startup check of dir to fail with a message that names the key, the
// expected mode and want.
func assertCheck(t *testing.T, dir, want string) {
	t.Helper()
	err := secrets.CheckSigningKeyDir(dir)
	if err == nil {
		t.Fatalf("CheckSigningKeyDir(%s) passed, want %q", filepath.Base(dir), want)
	}
	for _, part := range []string{"secrets.signing_key_dir", "0700", want} {
		if !strings.Contains(err.Error(), part) {
			t.Errorf("CheckSigningKeyDir(%s) = %q, want it to contain %q", filepath.Base(dir), err, part)
		}
	}
}
