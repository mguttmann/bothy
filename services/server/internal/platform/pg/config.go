// SPDX-License-Identifier: Apache-2.0

package pg

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// The values of database.sslmode (docs/spec/19-sicherheit.md SEC-TLS-5): verify-full checks
// certificate and host name; disable is only for the internal Compose network without egress
// and host port. Every other libpq mode either skips the host name check or falls back to
// plaintext, so it is a configuration error.
const (
	SSLModeDisable    = "disable"
	SSLModeVerifyFull = "verify-full"
)

// maxTimeout is the largest deadline PostgreSQL accepts: statement_timeout, lock_timeout and
// idle_in_transaction_session_timeout are int32 milliseconds.
const maxTimeout = math.MaxInt32 * time.Millisecond

// maxAppNameBytes is the longest application_name PostgreSQL keeps (NAMEDATALEN - 1); a longer
// one would be cut silently.
const maxAppNameBytes = 63

// Config is the connection configuration of one pool. internal/wire binds it from the
// configuration keys in the `config` tags (docs/spec/02-architektur.md "Konfigurationsladen"
// item 5) and sets the application name of the process role with WithAppName, which has no key.
//
// A zero StatementTimeout, LockTimeout or IdleInTxTimeout switches that deadline off, as in
// PostgreSQL; bothy migrate uses that for its own connections. The configuration keys of the
// server have a minimum of 1ms, so a server process cannot switch them off.
type Config struct {
	// Host is a host name or address, or a Unix socket directory when it starts with "/" (then
	// SSLMode is ignored: no TLS over a local socket, SEC-TLS-5).
	Host string `config:"database.host"`
	// Port is the TCP port, also the number in the socket file name.
	Port int `config:"database.port"`
	// Name is the database to connect to.
	Name string `config:"database.name"`
	// User is the role of the process: bothy_app for api and worker, bothy_owner for migrate.
	User string `config:"database.user"`
	// PasswordRef is a secret reference resolved through the secret provider for every new
	// connection.
	PasswordRef string `config:"database.password_ref"`
	// SSLMode is SSLModeDisable or SSLModeVerifyFull.
	SSLMode string `config:"database.sslmode"`
	// SSLRootCert is a PEM file with the CA certificates for verify-full; empty uses the system
	// roots, because managed services sign with their own CA.
	SSLRootCert string `config:"database.sslrootcert"`
	// MaxConns is the size of the pool.
	MaxConns int `config:"database.max_conns"`
	// AcquireTimeout bounds how long Acquire, Begin and Ping wait for a connection.
	AcquireTimeout time.Duration `config:"database.acquire_timeout"`
	// StatementTimeout is the session default of statement_timeout.
	StatementTimeout time.Duration `config:"database.statement_timeout"`
	// LockTimeout is the session default of lock_timeout.
	LockTimeout time.Duration `config:"database.lock_timeout"`
	// IdleInTxTimeout is the session default of idle_in_transaction_session_timeout.
	IdleInTxTimeout time.Duration `config:"database.idle_in_tx_timeout"`

	// appName is the application_name of the connections (see WithAppName).
	appName string
}

// WithAppName returns a copy of c whose connections report name as application_name, by
// convention "bothy-<role>" (bothy-api, bothy-worker, bothy-migrate), so pg_stat_activity shows
// which process holds a connection. Empty leaves application_name unset. Validate rejects a
// name that is not printable ASCII or longer than 63 bytes, which PostgreSQL would alter.
func (c Config) WithAppName(name string) Config {
	c.appName = name
	return c
}

// AppName returns the application name set with WithAppName.
func (c Config) AppName() string { return c.appName }

// Validate checks c and returns one error that lists every problem, each naming its
// configuration key and never a value. The configuration schema already rejects most of these;
// Validate guards the values that tests and wire build by hand.
func (c Config) Validate() error {
	problems := c.textProblems()
	problems = append(problems, c.numberProblems()...)
	if len(problems) > 0 {
		return errors.New("pg: invalid configuration: " + strings.Join(problems, "; "))
	}
	return nil
}

// textProblems checks the text settings of c.
func (c Config) textProblems() []string {
	var problems []string
	for _, f := range []struct{ key, v string }{
		{"database.host", c.Host}, {"database.name", c.Name}, {"database.user", c.User},
	} {
		switch {
		case f.v == "":
			problems = append(problems, f.key+" must not be empty")
		case strings.ContainsRune(f.v, 0):
			problems = append(problems, f.key+" must not contain a NUL character")
		}
	}
	if c.PasswordRef == "" {
		problems = append(problems, "database.password_ref must not be empty")
	}
	if c.SSLMode != SSLModeDisable && c.SSLMode != SSLModeVerifyFull {
		problems = append(problems, fmt.Sprintf("database.sslmode must be %s or %s (SEC-TLS-5)", SSLModeDisable, SSLModeVerifyFull))
	}
	if strings.ContainsRune(c.SSLRootCert, 0) {
		problems = append(problems, "database.sslrootcert must not contain a NUL character")
	}
	if !printableASCII(c.appName) || len(c.appName) > maxAppNameBytes {
		problems = append(problems, fmt.Sprintf("the application name must be printable ASCII of at most %d bytes", maxAppNameBytes))
	}
	return problems
}

// numberProblems checks the port, the pool size and the deadlines of c.
func (c Config) numberProblems() []string {
	var problems []string
	if c.Port < 1 || c.Port > math.MaxUint16 {
		problems = append(problems, "database.port must be between 1 and 65535")
	}
	if c.MaxConns < 1 || c.MaxConns > math.MaxInt32 {
		problems = append(problems, fmt.Sprintf("database.max_conns must be between 1 and %d", math.MaxInt32))
	}
	if c.AcquireTimeout <= 0 {
		problems = append(problems, "database.acquire_timeout must be positive")
	}
	for _, d := range []struct {
		key string
		v   time.Duration
	}{
		{"database.statement_timeout", c.StatementTimeout},
		{"database.lock_timeout", c.LockTimeout},
		{"database.idle_in_tx_timeout", c.IdleInTxTimeout},
	} {
		if d.v < 0 || d.v > maxTimeout {
			problems = append(problems, fmt.Sprintf("%s must be between 0 and %dms", d.key, int64(math.MaxInt32)))
		}
	}
	return problems
}

// ForMigrate returns a copy of c with the session settings of bothy migrate (docs/spec/16-betrieb.md
// "Upgrade", docs/operations/upgrade.md rule M6): lock_timeout is lockTimeout
// (migrate.lock_timeout) for every migration step, so a step that waits for a lock fails instead
// of blocking the application; statement_timeout and idle_in_transaction_session_timeout are
// off, because a migration may legitimately run long. The application name becomes
// bothy-migrate.
func (c Config) ForMigrate(lockTimeout time.Duration) Config {
	c.LockTimeout = lockTimeout
	c.StatementTimeout, c.IdleInTxTimeout = 0, 0
	c.appName = "bothy-migrate"
	return c
}

// Timeouts returns the session deadlines of c, the defaults SetLocalTimeouts overrides per
// transaction.
func (c Config) Timeouts() Timeouts {
	return Timeouts{Statement: c.StatementTimeout, Lock: c.LockTimeout, IdleInTx: c.IdleInTxTimeout}
}

// isSocket reports whether host names a Unix socket directory (libpq and pgx use the same rule).
func isSocket(host string) bool { return strings.HasPrefix(host, "/") }

// printableASCII reports whether s holds only the bytes 0x20 to 0x7e.
func printableASCII(s string) bool {
	for i := range len(s) {
		if s[i] < 0x20 || s[i] > 0x7e {
			return false
		}
	}
	return true
}

// quoteConnValue quotes a value for the keyword/value form of a connection string: inside
// single quotes pgx knows only the backslash escapes \' and \\.
func quoteConnValue(s string) string {
	return "'" + strings.NewReplacer(`\`, `\\`, `'`, `\'`).Replace(s) + "'"
}

// connString renders c in the keyword/value form. It names every setting pgx would otherwise
// take from a libpq environment variable or a file in the home directory (PGHOST, PGSSLMODE,
// ~/.pgpass, ~/.postgresql/root.crt and the rest): an explicit value, also an empty one, wins
// over both, and only cmd/bothy reads the environment (02 "Konfigurationsladen" item 5). The
// password never appears here; BeforeConnect sets it per connection.
func connString(c Config) string {
	mode, rootCert := c.SSLMode, ""
	if mode == SSLModeVerifyFull {
		rootCert = c.SSLRootCert
	}
	if isSocket(c.Host) {
		mode, rootCert = SSLModeDisable, ""
	}
	connectTimeout := int64(math.Ceil(c.AcquireTimeout.Seconds()))
	pairs := [][2]string{
		{"host", c.Host},
		{"port", strconv.Itoa(c.Port)},
		{"dbname", c.Name},
		{"user", c.User},
		{"sslmode", mode},
		{"sslrootcert", rootCert},
		{"sslcert", ""},
		{"sslkey", ""},
		{"sslpassword", ""},
		{"sslsni", "1"},
		{"sslnegotiation", "postgres"},
		{"passfile", ""},
		{"target_session_attrs", "any"},
		{"connect_timeout", strconv.FormatInt(max(connectTimeout, 1), 10)},
		{"min_protocol_version", ""},
		{"max_protocol_version", ""},
		{"channel_binding", ""},
		{"require_auth", ""},
	}
	parts := make([]string, 0, len(pairs))
	for _, p := range pairs {
		parts = append(parts, p[0]+"="+quoteConnValue(p[1]))
	}
	return strings.Join(parts, " ")
}

// runtimeParams returns the session parameters every connection of c starts with: the three
// deadlines in whole milliseconds, the application name, READ COMMITTED (03 Grundsatz 11) and
// UTC (03 Grundsatz 2, also for the tstamp column goose writes).
func runtimeParams(c Config) map[string]string {
	p := map[string]string{
		"statement_timeout":                   millis(c.StatementTimeout),
		"lock_timeout":                        millis(c.LockTimeout),
		"idle_in_transaction_session_timeout": millis(c.IdleInTxTimeout),
		"default_transaction_isolation":       "read committed",
		"TimeZone":                            "UTC",
	}
	if c.appName != "" {
		p["application_name"] = c.appName
	}
	return p
}

// millis renders d in whole milliseconds, rounded up, so that a positive deadline below one
// millisecond never becomes 0, which PostgreSQL reads as "no deadline".
func millis(d time.Duration) string {
	ms := d / time.Millisecond
	if d%time.Millisecond > 0 {
		ms++
	}
	return strconv.FormatInt(int64(ms), 10)
}

// harden overwrites what the libpq environment, a service file or ~/.pgpass could still have
// put into cc, and sets the session parameters and the tracer of c.
func harden(cc *pgx.ConnConfig, c Config) {
	cc.Password = ""
	cc.Fallbacks = nil
	cc.ValidateConnect = nil
	cc.KerberosSrvName, cc.KerberosSpn = "", ""
	cc.RuntimeParams = runtimeParams(c)
	cc.Tracer = newTracer()
	// A libpq service file (PGSERVICE) can set pgx's own keys; simple_protocol would put the
	// parameter values into the statement text the server logs.
	cc.DefaultQueryExecMode = pgx.QueryExecModeCacheStatement
	cc.StatementCacheCapacity, cc.DescriptionCacheCapacity = pgxCacheCapacity, pgxCacheCapacity
}

// pgxCacheCapacity is pgx's default size of the statement and description caches.
const pgxCacheCapacity = 512

// The pgxpool defaults, set explicitly so that a libpq service file cannot change them.
const (
	poolMaxConnLifetime   = time.Hour
	poolMaxConnIdleTime   = 30 * time.Minute
	poolHealthCheckPeriod = time.Minute
)

// parseError wraps an error of the pgx parser and names the key of the CA file when that file
// cannot be read, the one parse failure a valid Config can still hit.
func parseError(c Config, err error) error {
	if c.SSLMode == SSLModeVerifyFull && c.SSLRootCert != "" && !isSocket(c.Host) {
		if _, statErr := os.Stat(c.SSLRootCert); statErr != nil {
			return fmt.Errorf("pg: database.sslrootcert: %w", statErr)
		}
		return fmt.Errorf("pg: database.sslrootcert or connection settings: %w", err)
	}
	return fmt.Errorf("pg: connection settings: %w", err)
}

// connConfig returns the configuration of a single connection for c (bootstrap and tests).
func connConfig(c Config) (*pgx.ConnConfig, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	cc, err := pgx.ParseConfig(connString(c))
	if err != nil {
		return nil, parseError(c, err)
	}
	harden(cc, c)
	return cc, nil
}

// poolConfig returns the pool configuration for c. password resolves the password of each new
// connection; Open passes the secret provider, so a rotated secret takes effect after
// secrets.cache_ttl without a restart.
func poolConfig(c Config, password func(context.Context) (string, error)) (*pgxpool.Config, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	pc, err := pgxpool.ParseConfig(connString(c))
	if err != nil {
		return nil, parseError(c, err)
	}
	harden(pc.ConnConfig, c)
	pc.MaxConns = int32(c.MaxConns) // #nosec G115 -- Validate keeps MaxConns within int32.
	pc.MinConns, pc.MinIdleConns = 0, 0
	pc.MaxConnLifetime, pc.MaxConnIdleTime, pc.HealthCheckPeriod = poolMaxConnLifetime, poolMaxConnIdleTime, poolHealthCheckPeriod
	pc.MaxConnLifetimeJitter, pc.PingTimeout = 0, 0
	pc.BeforeConnect = func(ctx context.Context, cc *pgx.ConnConfig) error {
		pw, err := password(ctx)
		if err != nil {
			return err
		}
		cc.Password = pw
		return nil
	}
	return pc, nil
}
