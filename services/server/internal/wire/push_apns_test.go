// SPDX-License-Identifier: Apache-2.0

package wire_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/platform/egress"
	"github.com/OWNER/bothy/services/server/internal/platform/secrets"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

// apnsEnv is the configuration of an enabled APNs with the key at keyFile.
func apnsEnv(keyFile string) map[string]string {
	return map[string]string{
		"BOTHY_PUSH__APNS__TEAM_ID":         "EXAMPLETEAM",
		"BOTHY_PUSH__APNS__KEY_ID":          "EXAMPLEKEY",
		"BOTHY_PUSH__APNS__TOPICS":          "com.example.chat,com.example.chat.mac",
		"BOTHY_PUSH__APNS__PRIVATE_KEY_REF": "file:" + keyFile,
	}
}

// loadEnv loads the schema of the server with env.
func loadAPNsEnv(t *testing.T, env map[string]string) (*config.Values, error) {
	t.Helper()
	list := []string{testPublicURL}
	for k, v := range env {
		list = append(list, k+"="+v)
	}
	return config.Load(schema(t), config.Sources{Env: list})
}

// countingSecrets counts the resolves and passes them to the file provider of dir.
type countingSecrets struct {
	n  atomic.Int64
	sp *secrets.Provider
}

func (c *countingSecrets) Resolve(ctx context.Context, raw string) (secrets.Value, secrets.Version, error) {
	c.n.Add(1)
	return c.sp.Resolve(ctx, raw)
}

func newCountingSecrets(t *testing.T, dir string) *countingSecrets {
	t.Helper()
	sp, err := secrets.NewProvider(secrets.Options{Dir: dir, CacheTTL: 0})
	if err != nil {
		t.Fatal(err)
	}
	return &countingSecrets{sp: sp}
}

// writeAPNsKey writes a PKCS #8 key of curve to path with mode 0600.
func writeAPNsKey(t *testing.T, path string, curve elliptic.Curve) {
	t.Helper()
	k, err := ecdsa.GenerateKey(curve, rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(k)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestConfigLoad_JWTRefreshIntervalBounds(t *testing.T) {
	t.Parallel()
	for _, v := range []string{"19m59s", "60m1s"} {
		_, err := loadAPNsEnv(t, map[string]string{"BOTHY_PUSH__APNS__JWT_REFRESH_INTERVAL": v})
		if err == nil || !strings.Contains(err.Error(), "push.apns.jwt_refresh_interval") || strings.Contains(err.Error(), v) {
			t.Errorf("%s: %v, want an error naming the key and not the value", v, err)
		}
	}
	for _, v := range []string{"20m", "40m", "60m"} {
		if _, err := loadAPNsEnv(t, map[string]string{"BOTHY_PUSH__APNS__JWT_REFRESH_INTERVAL": v}); err != nil {
			t.Errorf("%s: %v", v, err)
		}
	}
}

func TestConfigCheck_PushAPNsComplete(t *testing.T) {
	t.Parallel()
	_, err := loadAPNsEnv(t, map[string]string{"BOTHY_PUSH__APNS__TEAM_ID": "EXAMPLETEAM"})
	if err == nil || !strings.Contains(err.Error(), "push.apns.key_id") || strings.Contains(err.Error(), "EXAMPLETEAM") {
		t.Fatalf("partial APNs: %v", err)
	}
	env := apnsEnv("/run/bothy/secrets/apns_key.p8")
	env["BOTHY_PUSH__APNS__TOPICS"] = "com.example.chat,com.example.chat"
	if _, err := loadAPNsEnv(t, env); err == nil || !strings.Contains(err.Error(), "push.apns.topics") {
		t.Fatalf("repeated topic: %v", err)
	}
	if _, err := loadAPNsEnv(t, apnsEnv("/run/bothy/secrets/apns_key.p8")); err != nil {
		t.Fatalf("complete APNs: %v", err)
	}
}

func TestAPNsWiring(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	egressCfg := egress.Config{Timeout: 1, MaxResponseBytes: 1}
	deps := func(s wire.APNsSecrets) wire.APNsDeps {
		return wire.APNsDeps{Secrets: s, Egress: egressCfg, EgressDeps: egress.Deps{}, Logger: slog.New(slog.DiscardHandler)}
	}
	t.Run("not enabled", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		v, err := loadAPNsEnv(t, map[string]string{"BOTHY_PUSH__APNS__PRIVATE_KEY_REF": "file:" + filepath.Join(dir, "missing.p8")})
		if err != nil {
			t.Fatal(err)
		}
		s := newCountingSecrets(t, dir)
		for _, role := range []wire.Role{wire.RoleAPI, wire.RoleWorker, wire.RoleAll} {
			ps, err := wire.APNsProviders(ctx, v, role, deps(s))
			if err != nil || len(ps) != 0 {
				t.Fatalf("%s: %d providers, %v", role, len(ps), err)
			}
		}
		if s.n.Load() != 0 {
			t.Fatalf("%d secret reads without APNs", s.n.Load())
		}
	})
	t.Run("enabled", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		key := filepath.Join(dir, "apns_key.p8")
		writeAPNsKey(t, key, elliptic.P256())
		v, err := loadAPNsEnv(t, apnsEnv(key))
		if err != nil {
			t.Fatal(err)
		}
		s := newCountingSecrets(t, dir)
		for role, want := range map[wire.Role]int{wire.RoleAPI: 0, wire.RoleWorker: 1, wire.RoleAll: 1} {
			ps, err := wire.APNsProviders(ctx, v, role, deps(s))
			if err != nil || len(ps) != want {
				t.Fatalf("%s: %d providers, %v; want %d", role, len(ps), err, want)
			}
			if want == 1 && ps[0].Name() != "apns" {
				t.Fatalf("%s: provider %q", role, ps[0].Name())
			}
		}
	})
	for name, prepare := range map[string]func(t *testing.T, path string){
		"missing key file": func(*testing.T, string) {},
		"P-384 key":        func(t *testing.T, path string) { writeAPNsKey(t, path, elliptic.P384()) },
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			key := filepath.Join(dir, "apns_key.p8")
			prepare(t, key)
			v, err := loadAPNsEnv(t, apnsEnv(key))
			if err != nil {
				t.Fatal(err)
			}
			_, err = wire.APNsProviders(ctx, v, wire.RoleWorker, deps(newCountingSecrets(t, dir)))
			if err == nil || !strings.Contains(err.Error(), "push.apns.private_key_ref") || strings.Contains(err.Error(), dir) {
				t.Fatalf("start error %v, want one naming push.apns.private_key_ref and no path", err)
			}
		})
	}
}
