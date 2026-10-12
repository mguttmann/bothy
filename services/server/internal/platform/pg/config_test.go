// SPDX-License-Identifier: Apache-2.0

package pg

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// validConfig is a configuration with the defaults of the chapter tables and a TCP host.
func validConfig() Config {
	return Config{ // #nosec G101 -- a secret reference, not a credential
		Host:             "db.example.com",
		Port:             5432,
		Name:             "bothy",
		User:             "bothy_app",
		PasswordRef:      "file:/run/bothy/secrets/db_app_password",
		SSLMode:          SSLModeVerifyFull,
		MaxConns:         20,
		AcquireTimeout:   2 * time.Second,
		StatementTimeout: 5 * time.Second,
		LockTimeout:      2 * time.Second,
		IdleInTxTimeout:  10 * time.Second,
	}.WithAppName("bothy-api")
}

func noPassword(context.Context) (string, error) { return "", nil }

func TestSSLModeValidation(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"disable", "verify-full"} {
		c := validConfig()
		c.SSLMode = mode
		if err := c.Validate(); err != nil {
			t.Errorf("sslmode %q rejected: %v", mode, err)
		}
	}
	for _, mode := range []string{"require", "prefer", "allow", "verify-ca", "", "DISABLE"} {
		c := validConfig()
		c.SSLMode = mode
		err := c.Validate()
		if err == nil {
			t.Errorf("sslmode %q accepted", mode)
			continue
		}
		if !strings.Contains(err.Error(), "database.sslmode") {
			t.Errorf("sslmode %q: error %q does not name database.sslmode", mode, err)
		}
	}
}

func TestConfigValidateRejects(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		edit func(*Config)
		key  string
	}{
		{"empty host", func(c *Config) { c.Host = "" }, "database.host"},
		{"host with NUL", func(c *Config) { c.Host = "db\x00x" }, "database.host"},
		{"port zero", func(c *Config) { c.Port = 0 }, "database.port"},
		{"port too large", func(c *Config) { c.Port = 65536 }, "database.port"},
		{"empty name", func(c *Config) { c.Name = "" }, "database.name"},
		{"empty user", func(c *Config) { c.User = "" }, "database.user"},
		{"empty password ref", func(c *Config) { c.PasswordRef = "" }, "database.password_ref"},
		{"sslrootcert with NUL", func(c *Config) { c.SSLRootCert = "/a\x00b" }, "database.sslrootcert"},
		{"max conns zero", func(c *Config) { c.MaxConns = 0 }, "database.max_conns"},
		{"acquire timeout zero", func(c *Config) { c.AcquireTimeout = 0 }, "database.acquire_timeout"},
		{"negative statement timeout", func(c *Config) { c.StatementTimeout = -time.Millisecond }, "database.statement_timeout"},
		{"negative lock timeout", func(c *Config) { c.LockTimeout = -time.Second }, "database.lock_timeout"},
		{"negative idle timeout", func(c *Config) { c.IdleInTxTimeout = -time.Second }, "database.idle_in_tx_timeout"},
		{"statement timeout above int32 ms", func(c *Config) { c.StatementTimeout = 2147483648 * time.Millisecond }, "database.statement_timeout"},
		{"application name not ASCII", func(c *Config) { *c = c.WithAppName("bothy-\u00e4") }, "application name"},
		{"application name too long", func(c *Config) { *c = c.WithAppName(strings.Repeat("a", 64)) }, "application name"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := validConfig()
			tc.edit(&c)
			err := c.Validate()
			if err == nil {
				t.Fatal("Validate accepted the configuration")
			}
			if !strings.Contains(err.Error(), tc.key) {
				t.Errorf("error %q does not name %s", err, tc.key)
			}
		})
	}
}

// TestConfigAcceptsZeroSessionTimeouts: bothy migrate runs with statement_timeout and
// idle_in_transaction_session_timeout 0 (disabled), as PostgreSQL defines 0.
func TestConfigAcceptsZeroSessionTimeouts(t *testing.T) {
	t.Parallel()
	c := validConfig()
	c.StatementTimeout, c.LockTimeout, c.IdleInTxTimeout = 0, 0, 0
	if err := c.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	pc, err := poolConfig(c, noPassword)
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"statement_timeout", "lock_timeout", "idle_in_transaction_session_timeout"} {
		if got := pc.ConnConfig.RuntimeParams[k]; got != "0" {
			t.Errorf("%s = %q, want 0", k, got)
		}
	}
}

func TestPoolConfigRuntimeParams(t *testing.T) {
	t.Parallel()
	c := validConfig()
	c.StatementTimeout = 1500 * time.Microsecond // rounds up to whole milliseconds, never to 0
	pc, err := poolConfig(c, noPassword)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"statement_timeout":                   "2",
		"lock_timeout":                        "2000",
		"idle_in_transaction_session_timeout": "10000",
		"application_name":                    "bothy-api",
		"default_transaction_isolation":       "read committed",
		"TimeZone":                            "UTC",
	}
	got := pc.ConnConfig.RuntimeParams
	if len(got) != len(want) {
		t.Errorf("runtime params = %v, want %v", got, want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("runtime param %s = %q, want %q", k, got[k], v)
		}
	}
	if pc.MaxConns != 20 || pc.MinConns != 0 {
		t.Errorf("MaxConns, MinConns = %d, %d, want 20, 0", pc.MaxConns, pc.MinConns)
	}
	if pc.ConnConfig.Host != "db.example.com" || pc.ConnConfig.Port != 5432 ||
		pc.ConnConfig.Database != "bothy" || pc.ConnConfig.User != "bothy_app" {
		t.Errorf("connection = %s:%d %s %s", pc.ConnConfig.Host, pc.ConnConfig.Port, pc.ConnConfig.Database, pc.ConnConfig.User)
	}
	if pc.ConnConfig.Tracer == nil {
		t.Error("no tracer set")
	}
	if pc.BeforeConnect == nil {
		t.Error("no BeforeConnect hook: a rotated password would need a restart")
	}
}

// TestConfigIgnoresLibpqEnvironment holds 02 "Configuration loading" item 5: only cmd/bothy reads
// the environment. pgx reads the libpq variables and ~/.pgpass on its own; none of them may
// change the connection. Not parallel: t.Setenv.
func TestConfigIgnoresLibpqEnvironment(t *testing.T) {
	want, err := poolConfig(validConfig(), noPassword)
	if err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	if err := os.WriteFile(filepath.Join(home, ".pgpass"), []byte("*:*:*:*:pgpass-EXAMPLE\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for k, v := range map[string]string{
		"HOME": home, "PGHOST": "other.example.com", "PGPORT": "6543", "PGDATABASE": "other",
		"PGUSER": "postgres", "PGPASSWORD": "env-EXAMPLE", "PGSSLMODE": "disable",
		"PGSSLROOTCERT": "/nonexistent/root.crt", "PGSSLCERT": "/nonexistent/c.crt",
		"PGSSLKEY": "/nonexistent/c.key", "PGAPPNAME": "other", "PGOPTIONS": "-c search_path=x",
		"PGTZ": "Europe/Berlin", "PGTARGETSESSIONATTRS": "read-only", "PGCONNECT_TIMEOUT": "99",
		"PGREQUIREAUTH": "password", "PGCHANNELBINDING": "require", "PGSSLNEGOTIATION": "direct",
		"PGMINPROTOCOLVERSION": "3.2", "PGSSLSNI": "0",
	} {
		t.Setenv(k, v)
	}
	got, err := poolConfig(validConfig(), noPassword)
	if err != nil {
		t.Fatalf("poolConfig with libpq variables: %v", err)
	}
	g, w := got.ConnConfig, want.ConnConfig
	switch {
	case g.Host != w.Host || g.Port != w.Port || g.Database != w.Database || g.User != w.User:
		t.Errorf("connection changed: %s:%d %s %s", g.Host, g.Port, g.Database, g.User)
	case g.Password != "":
		t.Error("a password from the environment or ~/.pgpass reached the configuration")
	case g.ValidateConnect != nil:
		t.Error("PGTARGETSESSIONATTRS set a connect check")
	case g.RequireAuth != w.RequireAuth || g.ChannelBinding != w.ChannelBinding:
		t.Errorf("auth settings changed: %q %q", g.RequireAuth, g.ChannelBinding)
	case g.SSLNegotiation != w.SSLNegotiation || g.MinProtocolVersion != w.MinProtocolVersion:
		t.Errorf("negotiation changed: %q %q", g.SSLNegotiation, g.MinProtocolVersion)
	case g.ConnectTimeout != w.ConnectTimeout:
		t.Errorf("connect timeout = %v, want %v", g.ConnectTimeout, w.ConnectTimeout)
	case len(g.Fallbacks) != 0:
		t.Errorf("fallbacks = %d, want none", len(g.Fallbacks))
	case g.TLSConfig == nil || len(g.TLSConfig.Certificates) != 0 || g.TLSConfig.ServerName != "db.example.com":
		t.Errorf("TLS changed: %+v", g.TLSConfig)
	}
	if len(g.RuntimeParams) != len(w.RuntimeParams) {
		t.Errorf("runtime params = %v, want %v", g.RuntimeParams, w.RuntimeParams)
	}
	for k, v := range w.RuntimeParams {
		if g.RuntimeParams[k] != v {
			t.Errorf("runtime param %s = %q, want %q", k, g.RuntimeParams[k], v)
		}
	}
}

func TestPoolConfigTLS(t *testing.T) {
	t.Parallel()
	t.Run("disable", func(t *testing.T) {
		t.Parallel()
		c := validConfig()
		c.SSLMode = SSLModeDisable
		pc, err := poolConfig(c, noPassword)
		if err != nil {
			t.Fatal(err)
		}
		if pc.ConnConfig.TLSConfig != nil || len(pc.ConnConfig.Fallbacks) != 0 {
			t.Errorf("disable: TLS %+v, fallbacks %d", pc.ConnConfig.TLSConfig, len(pc.ConnConfig.Fallbacks))
		}
	})
	t.Run("verify-full with system roots", func(t *testing.T) {
		t.Parallel()
		pc, err := poolConfig(validConfig(), noPassword)
		if err != nil {
			t.Fatal(err)
		}
		tc := pc.ConnConfig.TLSConfig
		if tc == nil || tc.ServerName != "db.example.com" || tc.RootCAs != nil || tc.InsecureSkipVerify {
			t.Errorf("verify-full: %+v", tc)
		}
	})
	t.Run("verify-full with sslrootcert", func(t *testing.T) {
		t.Parallel()
		c := validConfig()
		c.SSLRootCert = writeCA(t)
		pc, err := poolConfig(c, noPassword)
		if err != nil {
			t.Fatal(err)
		}
		tc := pc.ConnConfig.TLSConfig
		if tc == nil || tc.RootCAs == nil || tc.InsecureSkipVerify {
			t.Errorf("verify-full with CA: %+v", tc)
		}
	})
	t.Run("sslrootcert missing", func(t *testing.T) {
		t.Parallel()
		c := validConfig()
		c.SSLRootCert = filepath.Join(t.TempDir(), "missing.pem")
		if _, err := poolConfig(c, noPassword); err == nil || !strings.Contains(err.Error(), "database.sslrootcert") {
			t.Errorf("missing CA file: err = %v, want one naming database.sslrootcert", err)
		}
	})
	t.Run("unix socket", func(t *testing.T) {
		t.Parallel()
		c := validConfig()
		c.Host = "/run/postgresql"
		pc, err := poolConfig(c, noPassword)
		if err != nil {
			t.Fatal(err)
		}
		if pc.ConnConfig.TLSConfig != nil || pc.ConnConfig.Host != "/run/postgresql" {
			t.Errorf("socket: host %q, TLS %+v", pc.ConnConfig.Host, pc.ConnConfig.TLSConfig)
		}
	})
	t.Run("values with quotes and backslashes", func(t *testing.T) {
		t.Parallel()
		c := validConfig()
		c.Name = `it's a \ name`
		pc, err := poolConfig(c, noPassword)
		if err != nil {
			t.Fatal(err)
		}
		if pc.ConnConfig.Database != c.Name {
			t.Errorf("database = %q, want %q", pc.ConnConfig.Database, c.Name)
		}
	})
}

// writeCA writes a self-signed CA certificate in PEM form and returns its path.
func writeCA(t *testing.T) string {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "EXAMPLE CA"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "ca.pem")
	if err := os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestConfigIgnoresServiceFile: a libpq service file (PGSERVICE) must not change how pgx runs
// statements or how long pool connections live (02 "Configuration loading" item 5).
func TestConfigIgnoresServiceFile(t *testing.T) {
	want, err := poolConfig(validConfig(), noPassword)
	if err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	service := "[bothy]\ndefault_query_exec_mode=simple_protocol\nstatement_cache_capacity=0\n" +
		"description_cache_capacity=0\npool_max_conn_lifetime=1s\npool_max_conn_idle_time=1s\n" +
		"pool_health_check_period=1h\npool_max_conn_lifetime_jitter=1h\npool_ping_timeout=1h\n"
	if err := os.WriteFile(filepath.Join(home, ".pg_service.conf"), []byte(service), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("PGSERVICE", "bothy")
	got, err := poolConfig(validConfig(), noPassword)
	if err != nil {
		t.Fatalf("poolConfig with PGSERVICE: %v", err)
	}
	g, w := got.ConnConfig, want.ConnConfig
	if g.DefaultQueryExecMode != w.DefaultQueryExecMode || g.StatementCacheCapacity != w.StatementCacheCapacity ||
		g.DescriptionCacheCapacity != w.DescriptionCacheCapacity {
		t.Errorf("exec mode %v, caches %d %d; want %v, %d %d", g.DefaultQueryExecMode, g.StatementCacheCapacity,
			g.DescriptionCacheCapacity, w.DefaultQueryExecMode, w.StatementCacheCapacity, w.DescriptionCacheCapacity)
	}
	if got.MaxConnLifetime != want.MaxConnLifetime || got.MaxConnIdleTime != want.MaxConnIdleTime ||
		got.HealthCheckPeriod != want.HealthCheckPeriod || got.MaxConnLifetimeJitter != want.MaxConnLifetimeJitter ||
		got.PingTimeout != want.PingTimeout {
		t.Errorf("pool settings changed: lifetime %v idle %v health %v jitter %v ping %v", got.MaxConnLifetime,
			got.MaxConnIdleTime, got.HealthCheckPeriod, got.MaxConnLifetimeJitter, got.PingTimeout)
	}
}
