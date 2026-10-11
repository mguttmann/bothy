// SPDX-License-Identifier: Apache-2.0

//go:build unix

package wire_test

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

func canary(t *testing.T) string {
	t.Helper()
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return "EXAMPLE-canary-" + hex.EncodeToString(b)
}

func loadValues(t *testing.T, s *config.Schema, env ...string) *config.Values {
	t.Helper()
	v, err := config.Load(s, config.Sources{Env: env})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return v
}

// TestNewSecretsProvider holds the wiring of brief D10: the provider of a process takes
// secrets.dir and secrets.cache_ttl from the loaded configuration, the environment that
// cmd/bothy passes in and the logger it is given, and refuses a configuration it cannot work
// with by naming the key, never the value.
func TestNewSecretsProvider(t *testing.T) {
	t.Parallel()

	t.Run("binds the configuration", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		fileSecret, envSecret := canary(t), canary(t)
		path := filepath.Join(dir, "db_app_password")
		if err := os.WriteFile(path, []byte(fileSecret+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		env := []string{"BOTHY_SECRET_EXAMPLE=" + envSecret}

		// secrets.dir is the directory of <secrets.dir>, the env driver reads the passed
		// environment, and a TTL of one hour keeps the first value although the file changes.
		values := loadValues(t, schema(t), "BOTHY_SECRETS__DIR="+dir, "BOTHY_SECRETS__CACHE_TTL=1h")
		p, err := wire.NewSecretsProvider(values, env, nil)
		if err != nil {
			t.Fatalf("NewSecretsProvider: %v", err)
		}
		v, _, err := p.Resolve(t.Context(), "file:<secrets.dir>/db_app_password")
		if err != nil || string(v.Bytes()) != fileSecret {
			t.Fatalf("file reference: err %v, value matches %v", err, string(v.Bytes()) == fileSecret)
		}
		v, _, err = p.Resolve(t.Context(), "env:BOTHY_SECRET_EXAMPLE")
		if err != nil || string(v.Bytes()) != envSecret {
			t.Fatalf("env reference: err %v, value matches %v", err, string(v.Bytes()) == envSecret)
		}
		rotated := canary(t)
		if err := os.WriteFile(path, []byte(rotated), 0o600); err != nil {
			t.Fatal(err)
		}
		if v, _, err = p.Resolve(t.Context(), "file:"+path); err != nil || string(v.Bytes()) != fileSecret {
			t.Fatalf("within secrets.cache_ttl the first value stays in use: err %v", err)
		}

		// After secrets.cache_ttl (at least 1s) the provider reads again: the rotated value; after
		// the file is gone the previous value with one error line on the logger that was passed in.
		var logs bytes.Buffer
		values = loadValues(t, schema(t), "BOTHY_SECRETS__DIR="+dir, "BOTHY_SECRETS__CACHE_TTL=1s")
		p, err = wire.NewSecretsProvider(values, nil, slog.New(slog.NewJSONHandler(&logs, nil)))
		if err != nil {
			t.Fatalf("NewSecretsProvider: %v", err)
		}
		if v, _, err = p.Resolve(t.Context(), "file:<secrets.dir>/db_app_password"); err != nil || string(v.Bytes()) != rotated {
			t.Fatalf("new provider: err %v, rotated value read %v", err, string(v.Bytes()) == rotated)
		}
		if _, _, err = p.Resolve(t.Context(), "env:BOTHY_SECRET_EXAMPLE"); err == nil {
			t.Fatal("env reference resolved although the provider got no environment")
		}
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
		time.Sleep(1100 * time.Millisecond) // the system clock of wire must pass secrets.cache_ttl
		for range 3 {
			if v, _, err = p.Resolve(t.Context(), "file:"+path); err != nil || string(v.Bytes()) != rotated {
				t.Fatalf("failed refresh: err %v, previous value kept %v", err, string(v.Bytes()) == rotated)
			}
		}
		if n := strings.Count(logs.String(), "secret refresh failed, keeping previous value"); n != 1 {
			t.Errorf("logger got %d refresh lines, want 1:\n%s", n, logs.String())
		}
		for _, secret := range []string{fileSecret, envSecret, rotated} {
			if strings.Contains(logs.String(), secret) {
				t.Errorf("log contains a secret:\n%s", logs.String())
			}
		}
	})

	t.Run("rejects", func(t *testing.T) {
		t.Parallel()
		// A schema without the provider keys cannot bind its Config.
		other, err := config.NewSchema([]config.Key{
			{Name: "server.role", Kind: config.KindString, Default: "all", Doc: "Role.", Chapter: "16-betrieb.md"},
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := wire.NewSecretsProvider(loadValues(t, other), nil, nil); err == nil || !strings.Contains(err.Error(), `"secrets.dir"`) {
			t.Errorf("schema without secrets.dir: error %v, want one naming the key", err)
		}
	})
}
