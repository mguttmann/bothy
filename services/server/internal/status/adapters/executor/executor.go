// SPDX-License-Identifier: Apache-2.0

// Package executor is the adapter of app.Executor: it runs pgbackrest, pg_dump and restic for
// bothy-backupd (brief M7-02 design note 2). Every binary runs through exec.CommandContext with
// its arguments as a list, never through a shell, and with a minimal explicit environment (PATH,
// HOME, LANG and the variables of the tool), never the environment of the process, so no other
// secret reaches a child. Exit codes become error codes (<tool>_exit_<n>); the output of a tool
// is parsed where a step needs it and never enters an error. The tools' own diagnostics on
// stderr go to Config.Stderr, the container log.
//
// Example:
//
//	e, err := executor.New(executor.Config{SecretsDir: "/run/secrets", Offsite: &target,
//		DB: executor.DB{Host: "postgres", Port: 5432, Name: "bothy", User: "bothy_owner", Password: pw}})
package executor

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/OWNER/bothy/services/server/internal/status/app"
	"github.com/OWNER/bothy/services/server/internal/status/domain"
)

// The defaults of Config.
const (
	defaultPgBackRest      = "pgbackrest"
	defaultPgDump          = "pg_dump"
	defaultRestic          = "restic"
	defaultStanza          = "bothy"
	defaultLocalResticRepo = "/var/lib/pgbackrest/restic"
	defaultRegion          = "us-east-1"
)

// The tool names of the error codes.
const (
	toolPgBackRest = "pgbackrest"
	toolPgDump     = "pg_dump"
	toolRestic     = "restic"
)

// The file names in the secret directory and in an off-site credentials directory.
const (
	fileResticPassword = "restic_password"
	fileCipherPass     = "pgbackrest_cipher_pass" // #nosec G101 -- a file name, not a credential
	dirOffsite         = "offsite"
	fileAccessKeyID    = "access_key_id"
	fileSecretKey      = "secret_access_key" // #nosec G101 -- a file name, not a credential
	fileCA             = "ca.crt"
)

// baseEnv is the environment every child starts from. PATH is fixed, because no package outside
// the commands reads the process environment; HOME is a writable directory for the caches of the
// tools.
var baseEnv = []string{
	"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
	"HOME=/tmp",
	"LANG=C.UTF-8",
}

// Error codes of this adapter besides <tool>_exit_<n>, <tool>_not_found and
// <tool>_output_invalid.
const (
	codeCanceled             = "canceled"
	codeOffsiteNotConfigured = "offsite_not_configured"
	codeSecretUnreadable     = "secret_unreadable"
	codeMirrorNotConfigured  = "s3_mirror_not_configured"
	codeVolumeUsageFailed    = "volume_usage_failed"
	codeOffsiteForgetRefused = "restic_offsite_forget_refused"
)

// DB is the connection of pg_dump: migrate.bootstrap_user on the database of the instance.
type DB struct {
	Host string
	Port int
	Name string
	User string
	// Password returns the password of User on every call, so a rotated secret is used.
	Password func(ctx context.Context) ([]byte, error)
}

// Mirror is the object mirror of the driver s3 (adapters/s3mirror).
type Mirror interface {
	S3Mirror(ctx context.Context) (app.MirrorResult, error)
}

// Config is the configuration of the executor; bothy-backupd fills it from backup.*, secrets.dir
// and migrate.bootstrap_user.
type Config struct {
	// PgBackRest, PgDump and Restic are the binaries; defaults pgbackrest, pg_dump and restic,
	// looked up in the PATH of the process.
	PgBackRest, PgDump, Restic string
	// Stanza is the pgBackRest stanza; default bothy.
	Stanza string
	// SecretsDir is secrets.dir: restic_password, pgbackrest_cipher_pass and, for an off-site
	// target, offsite/access_key_id, offsite/secret_access_key and the optional offsite/ca.crt.
	// Required. The files are read on every step, so a rotated secret is used without restart.
	SecretsDir string
	// Offsite is backup.offsite.target; nil backs up locally only.
	Offsite *domain.OffsiteTarget
	// OffsiteRegion is backup.offsite.region; default us-east-1.
	OffsiteRegion string
	// OffsitePathStyle is backup.offsite.use_path_style.
	OffsitePathStyle bool
	// LocalResticRepo is the restic repository without an off-site target; default
	// /var/lib/pgbackrest/restic.
	LocalResticRepo string
	// DB is the connection of pg_dump.
	DB DB
	// Stderr receives the stderr of every child (the container log); nil discards it. It is
	// never parsed into an error.
	Stderr io.Writer
	// Mirror runs S3Mirror; nil makes S3Mirror fail with s3_mirror_not_configured.
	Mirror Mirror
}

// Executor implements app.Executor. It is safe for concurrent use; it keeps no state besides
// its configuration.
type Executor struct {
	cfg Config
	// resticMu orders the restic calls of this process on the repository: backups share it,
	// forget and prune and the first initialisation hold it alone. restic itself refuses a prune
	// while another process holds a lock, so without this order a local prune after a logical
	// run fails a backup of another kind running at the same time, or the other way round.
	resticMu sync.RWMutex
}

// Compile-time check: Executor is the outgoing port of app.
var _ app.Executor = (*Executor)(nil)

// New returns the executor of cfg with its defaults filled in. The error names a field, never a
// value.
func New(cfg Config) (*Executor, error) {
	if cfg.SecretsDir == "" {
		return nil, errors.New("executor: Config.SecretsDir is empty")
	}
	def := func(s *string, v string) {
		if *s == "" {
			*s = v
		}
	}
	def(&cfg.PgBackRest, defaultPgBackRest)
	def(&cfg.PgDump, defaultPgDump)
	def(&cfg.Restic, defaultRestic)
	def(&cfg.Stanza, defaultStanza)
	def(&cfg.LocalResticRepo, defaultLocalResticRepo)
	def(&cfg.OffsiteRegion, defaultRegion)
	if cfg.Stderr == nil {
		cfg.Stderr = io.Discard
	}
	return &Executor{cfg: cfg}, nil
}

// execError returns the *app.ExecError of code.
func execError(code string) error { return &app.ExecError{Code: code} }

// run runs bin with args, the base environment plus env and stdin, and returns its stdout. The
// error is an *app.ExecError: canceled, <tool>_not_found, <tool>_exit_<n> or
// <tool>_failed when the child could not start or was killed by a signal.
func (e *Executor) run(ctx context.Context, tool, bin string, args, env []string, stdin io.Reader) ([]byte, error) {
	if ctx.Err() != nil {
		return nil, execError(codeCanceled)
	}
	cmd := exec.CommandContext(ctx, bin, args...) // #nosec G204 -- configured binaries, arguments as a list, no shell
	cmd.Env = append(append(make([]string, 0, len(baseEnv)+len(env)), baseEnv...), env...)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = e.cfg.Stderr
	cmd.Stdin = stdin
	err := cmd.Run()
	if err == nil {
		return stdout.Bytes(), nil
	}
	if ctx.Err() != nil {
		return nil, execError(codeCanceled)
	}
	if errors.Is(err, exec.ErrNotFound) || errors.Is(err, fs.ErrNotExist) {
		return nil, execError(tool + "_not_found")
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() >= 0 {
		return nil, execError(fmt.Sprintf("%s_exit_%d", tool, exit.ExitCode()))
	}
	return nil, execError(tool + "_failed")
}

// readSecret reads the file name of dir without the trailing line break. The error is
// secret_unreadable, never the path or the content.
func readSecret(dir, name string) (string, error) {
	b, err := os.ReadFile(filepath.Join(dir, name)) // #nosec G304 -- a fixed file name in the configured secret directory
	if err != nil {
		return "", execError(codeSecretUnreadable)
	}
	v := strings.TrimRight(string(b), "\r\n")
	if v == "" {
		return "", execError(codeSecretUnreadable)
	}
	return v, nil
}

// offsiteKeys are the S3 credentials of the off-site target and the optional CA file.
type offsiteKeys struct {
	access, secret string
	// ca is the path of ca.crt, "" when the directory has none.
	ca string
}

// readOffsiteKeys reads access_key_id, secret_access_key and the optional ca.crt of dir.
func readOffsiteKeys(dir string) (offsiteKeys, error) {
	access, err := readSecret(dir, fileAccessKeyID)
	if err != nil {
		return offsiteKeys{}, err
	}
	secret, err := readSecret(dir, fileSecretKey)
	if err != nil {
		return offsiteKeys{}, err
	}
	k := offsiteKeys{access: access, secret: secret}
	ca := filepath.Join(dir, fileCA)
	if _, err := os.Stat(ca); err == nil {
		k.ca = ca
	}
	return k, nil
}

// serviceKeysDir is the directory of the service's own off-site credentials.
func (e *Executor) serviceKeysDir() string { return filepath.Join(e.cfg.SecretsDir, dirOffsite) }

// S3Mirror implements app.Executor through Config.Mirror.
func (e *Executor) S3Mirror(ctx context.Context) (app.MirrorResult, error) {
	if e.cfg.Mirror == nil {
		return app.MirrorResult{}, execError(codeMirrorNotConfigured)
	}
	return e.cfg.Mirror.S3Mirror(ctx)
}
