// SPDX-License-Identifier: Apache-2.0

//go:build unix

package wire_test

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	auditapp "github.com/OWNER/bothy/services/server/internal/audit/app"
	instancejobs "github.com/OWNER/bothy/services/server/internal/instance/adapters/jobs"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/platform/health"
	"github.com/OWNER/bothy/services/server/internal/platform/jobs"
	"github.com/OWNER/bothy/services/server/internal/platform/lifecycle"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/secrets"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

func TestSigningKeysConfigBindsDefaults(t *testing.T) {
	t.Parallel()
	v, err := config.Load(schema(t), config.Sources{Env: []string{testPublicURL}})
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := wire.SigningKeysConfig(v)
	if err != nil {
		t.Fatal(err)
	}
	k := cfg.Keys
	if cfg.Dir != "/var/lib/bothy/signing-keys" || cfg.CheckInterval != time.Minute || k.CacheTTL != 30*time.Second ||
		k.JWKSMaxAge != time.Hour || k.InvocationRotation != 30*24*time.Hour || k.AccessTokenRotation != 30*24*time.Hour ||
		k.AccessTokenTTL != 10*time.Minute || k.InvocationTokenTTL != time.Minute || k.WebhookDeliveryTimeout != 15*time.Second ||
		k.ClockLeeway != 30*time.Second {
		t.Fatalf("SigningKeysConfig = %+v", cfg)
	}
}

// TestKeyRotationJobRegistered: the registry of every role holds instance.key_rotation on queue
// maintenance every signing_key.rotation_check_interval; only worker and all run River.
func TestKeyRotationJobRegistered(t *testing.T) {
	t.Parallel()
	reg, err := wire.NewJobRegistry(&wire.JobModules{TerminalSweepInterval: time.Minute, FilesGCInterval: time.Minute, PartitionInterval: time.Minute, CleanupInterval: time.Hour, KeyRotationInterval: 3 * time.Minute, ExpiryInterval: time.Minute, AuthCleanupInterval: time.Hour, ChainVerifyInterval: time.Hour, StallInterval: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	if q, ok := reg.Queue(instancejobs.KindKeyRotation); !ok || q != jobs.QueueMaintenance {
		t.Fatalf("queue of %s = %q, %v", instancejobs.KindKeyRotation, q, ok)
	}
	var found bool
	for _, p := range reg.Periodic() {
		found = found || (p.Args.Kind() == instancejobs.KindKeyRotation && p.Interval == 3*time.Minute)
	}
	if !found {
		t.Fatalf("periodic jobs %+v lack %s every 3m", reg.Periodic(), instancejobs.KindKeyRotation)
	}
	if _, err := wire.NewJobRegistry(&wire.JobModules{TerminalSweepInterval: time.Minute, FilesGCInterval: time.Minute, PartitionInterval: time.Minute, CleanupInterval: time.Hour, ExpiryInterval: time.Minute, AuthCleanupInterval: time.Hour, ChainVerifyInterval: time.Hour, StallInterval: time.Minute}); err == nil {
		t.Error("a registry without signing_key.rotation_check_interval was built")
	}
}

// allowAll is an Authorizer that allows; the readiness test does not rotate.
type allowAll struct{}

func (allowAll) Authorize(context.Context, authorize.Principal, authorize.Action, authorize.Resource) (authorize.Decision, error) {
	return authorize.Decision{Outcome: authorize.OutcomeAllow}, nil
}

func (allowAll) Scope(context.Context, authorize.Principal, authorize.Action) (authorize.Scope, error) {
	return authorize.Scope{}, nil
}

// Observe implements authorize.Authorizer; the fake keeps no cache.
func (allowAll) Observe(context.Context, authorize.LogEvent) {}

// TestStartup_SigningKeys_Readiness: /readyz reports signing_keys while a purpose lacks an active
// key and is ready after EnsureActive.
func TestStartup_SigningKeys_Readiness(t *testing.T) {
	t.Parallel()
	db := newServeDB(t)
	pool := db.Pool(t, pg.RoleApp)
	a, err := wire.NewAudit(pool, pg.UnitConfig{StatementTimeout: 30 * time.Second, LockTimeout: 10 * time.Second, IdleInTxTimeout: time.Minute},
		auditapp.Config{DenyAggregateWindow: time.Minute, DenyAggregateMaxKeys: 10, IPKeyPeriod: 24 * time.Hour, IPKeyRetention: 48 * time.Hour,
			IPKEKRef: "file:<secrets.dir>/audit_ip_kek"}, testKEK{}, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	v, err := config.Load(schema(t), config.Sources{Env: []string{testPublicURL}})
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := wire.SigningKeysConfig(v)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Dir = keyStoreDir(t)
	sp, err := secrets.NewProvider(secrets.Options{Dir: filepath.Dir(cfg.Dir), SigningKeyDir: cfg.Dir, CacheTTL: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	keys, err := wire.NewSigningKeys(cfg, pool, sp, a.UoW(), allowAll{}, a.Recorder(), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	reg := health.NewRegistry()
	reg.AddReadiness(wire.CheckSigningKeys, keys.Ready)
	reg.SetStarted()
	readyz := func() (int, string) {
		rec := httptest.NewRecorder()
		reg.Handler().ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/readyz", nil))
		return rec.Code, rec.Body.String()
	}
	if code, body := readyz(); code != http.StatusServiceUnavailable || !strings.Contains(body, `"signing_keys"`) {
		t.Fatalf("/readyz before EnsureActive = %d %s, want 503 naming signing_keys", code, body)
	}
	if err := keys.Keys.EnsureActive(context.Background()); err != nil {
		t.Fatal(err)
	}
	if code, body := readyz(); code != http.StatusOK {
		t.Fatalf("/readyz after EnsureActive = %d %s", code, body)
	}
	// An active key whose private file cannot be read (here: removed) cannot sign; a process that
	// has not cached it reports signing_keys too.
	entries, err := os.ReadDir(cfg.Dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, de := range entries {
		if strings.HasPrefix(de.Name(), "invocation-") {
			if err := os.Remove(filepath.Join(cfg.Dir, de.Name())); err != nil {
				t.Fatal(err)
			}
		}
	}
	fresh, err := secrets.NewProvider(secrets.Options{Dir: filepath.Dir(cfg.Dir), SigningKeyDir: cfg.Dir, CacheTTL: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	other, err := wire.NewSigningKeys(cfg, pool, fresh, a.UoW(), allowAll{}, a.Recorder(), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := other.Ready(context.Background()); err == nil || !strings.Contains(err.Error(), "invocation") {
		t.Fatalf("Ready with an unreadable active key = %v, want an error naming invocation", err)
	}
}

// TestStartup_SigningKeys_NotWritable: the start ends in every role when the key store is
// missing, has another mode or is not a directory, naming secrets.signing_key_dir; the public
// listener never opens.
func TestStartup_SigningKeys_NotWritable(t *testing.T) {
	t.Parallel()
	db := newServeDB(t)
	wide := filepath.Join(t.TempDir(), "wide")
	if err := os.Mkdir(wide, 0o700); err != nil {
		t.Fatal(err)
	}
	setMode(t, wide, 0o755)
	for name, c := range map[string]struct {
		role wire.Role
		dir  string
	}{
		"missing (api)":   {wire.RoleAPI, filepath.Join(t.TempDir(), "missing")},
		"mode 0755 (all)": {wire.RoleAll, wide},
		"read-only (worker)": {wire.RoleWorker, func() string {
			d := filepath.Join(t.TempDir(), "ro")
			if err := os.Mkdir(d, 0o500); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { setMode(t, d, 0o700) })
			return d
		}()},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			p := start(t, c.role, serveEnv(t, db, "BOTHY_SECRETS__SIGNING_KEY_DIR="+c.dir), wire.Deps{})
			if code := p.exit(t); code != lifecycle.ExitFailure {
				t.Fatalf("exit = %d; logs:\n%s", code, p.logs)
			}
			if err := p.runner.Err(); err == nil || !strings.Contains(err.Error(), "secrets.signing_key_dir") || !strings.Contains(err.Error(), "0700") {
				t.Fatalf("Err() = %v", err)
			}
			if p.logs.port("public listener started") != 0 {
				t.Fatal("public listener opened")
			}
		})
	}
}
