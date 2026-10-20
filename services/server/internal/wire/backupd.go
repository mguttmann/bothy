// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	auditapp "github.com/OWNER/bothy/services/server/internal/audit/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/platform/egress"
	"github.com/OWNER/bothy/services/server/internal/platform/otel"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/secrets"
	"github.com/OWNER/bothy/services/server/internal/status/adapters/executor"
	"github.com/OWNER/bothy/services/server/internal/status/adapters/otelmetrics"
	statuspg "github.com/OWNER/bothy/services/server/internal/status/adapters/postgres"
	"github.com/OWNER/bothy/services/server/internal/status/adapters/scratch"
	"github.com/OWNER/bothy/services/server/internal/status/adapters/s3mirror"
	statusapp "github.com/OWNER/bothy/services/server/internal/status/app"
	statusdomain "github.com/OWNER/bothy/services/server/internal/status/domain"
)

// The application names of the two pools of bothy-backupd in pg_stat_activity.
const (
	backupdAppName         = "bothy-backupd"
	backupdManifestAppName = "bothy-backupd-manifest"
	// backupdRestoreAppName is the pool of the restored copy of the restore test, on its socket.
	backupdRestoreAppName = "bothy-backupd-restore-test"
)

// The pool of the restored copy: one check at a time reads it; the scratch PostgreSQL listens on
// the default port number, which names its socket file.
const (
	restoreMaxConns   = 2
	restoreSocketPort = 5432
)

// manifestMaxConns is the pool of the manifest queries: one snapshot transaction at a time per
// run, at most two runs at once (a physical and a logical or objects run).
const manifestMaxConns = 2

// backupdLogKeyError is the log attribute of an error message.
const backupdLogKeyError = "error"

// offsiteDir is the directory of the off-site key below secrets.dir (configuration.md section 5).
const offsiteDir = "offsite"

// ErrUnknownBackupKind reports a kind that is not pg_full, pg_diff, logical or objects.
var ErrUnknownBackupKind = errors.New("unknown backup kind")

// BackupConfigError reports a configuration bothy-backupd cannot use; the command exits with 2.
type BackupConfigError struct{ err error }

// Error returns the reason, which names keys and never values.
func (e *BackupConfigError) Error() string { return e.err.Error() }

// Unwrap returns the reason.
func (e *BackupConfigError) Unwrap() error { return e.err }

// BackupDaemonOptions are what bothy-backupd passes besides the loaded values.
type BackupDaemonOptions struct {
	// Env is the process environment ("NAME=value"), for the secret provider's env: references.
	Env []string
	// Log receives the lines of the runs; nil means slog.Default().
	Log *slog.Logger
	// ToolOutput receives the stderr of pgBackRest, pg_dump and restic (the container log).
	ToolOutput io.Writer
	// Telemetry records the metrics bothy.backup.* and bothy.storage.volume.* (otel.SetupMetrics);
	// nil records none.
	Telemetry *otel.MeterTelemetry
	// ServerVersion is written into each manifest.
	ServerVersion string
	// Volumes maps the metric volume names pgdata, pgbackrest-repo and backup-scratch to their
	// mount points; files is added from file.fs.root with the driver fs.
	Volumes map[string]string
	// DumpDir holds the pg_dump file of a logical run; it is created when missing.
	DumpDir string
	// LocalResticRepo is the restic repository without an off-site target.
	LocalResticRepo string
	// RestoreDir is the directory of the restore test in backup-scratch (M7-03); it is created
	// when missing and emptied by every test. Empty leaves the restore test out.
	RestoreDir string
}

// BackupDaemon is the backup service of bothy-backupd (brief M7-02) built from the server
// configuration: the use cases of internal/status/app over the role bothy_backup (backup_runs),
// a superuser pool for the manifest (migrate.bootstrap_user), the executor of pgBackRest,
// pg_dump and restic, the object mirror of the driver s3 and the metrics. Its methods take kinds
// as strings, so the command needs only this package.
type BackupDaemon struct {
	svc       *statusapp.Service
	exec      *executor.Executor
	runsPool  *pg.Pool
	superPool *pg.Pool
	wait      time.Duration
	log       *slog.Logger
}

// backupdSettings are the bound and parsed values NewBackupDaemon builds from.
type backupdSettings struct {
	backup    BackupConfig
	migrate   MigrateConfig
	secrets   secrets.Config
	db        pg.Config
	schedules map[statusdomain.Kind]statusdomain.Schedule
	restore   statusdomain.Schedule
	target    *statusdomain.OffsiteTarget
}

// loadBackupdSettings binds and parses the keys of bothy-backupd from v; the error is a
// *BackupConfigError naming keys.
func loadBackupdSettings(v *config.Values) (backupdSettings, error) {
	var s backupdSettings
	fail := func(err error) (backupdSettings, error) { return backupdSettings{}, &BackupConfigError{err: err} }
	var err error
	if s.backup, err = BackupSettings(v); err != nil {
		return fail(err)
	}
	if s.migrate, err = MigrateSettings(v); err != nil {
		return fail(err)
	}
	if err := config.Bind(v, &s.secrets); err != nil {
		return fail(err)
	}
	if s.db, err = DatabaseConfig(v, backupdAppName); err != nil {
		return fail(err)
	}
	if err := s.db.Validate(); err != nil {
		return fail(err)
	}
	if s.schedules, err = parsedBackupSchedules(s.backup); err != nil {
		return fail(err)
	}
	if s.restore, err = statusdomain.ParseSchedule(s.backup.RestoreTestSchedule); err != nil {
		return fail(fmt.Errorf("%s: %w", keyRestoreTestSchedule, err))
	}
	if s.backup.OffsiteTarget != "" {
		t, err := statusdomain.ParseOffsiteTarget(s.backup.OffsiteTarget)
		if err != nil {
			return fail(fmt.Errorf("backup.offsite.target: %w", err))
		}
		s.target = &t
	}
	return s, nil
}

// NewBackupDaemon builds the backup service from v and o. It opens two pools, which connect on
// first use. The error is a *BackupConfigError for a configuration that cannot be used (it names
// keys, never values) and a plain error otherwise.
func NewBackupDaemon(ctx context.Context, v *config.Values, o BackupDaemonOptions) (*BackupDaemon, error) {
	log := o.Log
	if log == nil {
		log = slog.Default()
	}
	s, err := loadBackupdSettings(v)
	if err != nil {
		return nil, err
	}
	provider, err := NewSecretsProvider(v, o.Env, log)
	if err != nil {
		return nil, &BackupConfigError{err: err}
	}
	mirror, err := backupMirror(v, s.backup, s.target, provider, s.secrets.Dir, o.DumpDir, log)
	if err != nil {
		return nil, err
	}
	exec, err := executor.New(executor.Config{
		SecretsDir: s.secrets.Dir, Offsite: s.target, OffsiteRegion: s.backup.OffsiteRegion,
		OffsitePathStyle: s.backup.OffsiteUsePathStyle, LocalResticRepo: o.LocalResticRepo,
		DB: executor.DB{Host: s.db.Host, Port: s.db.Port, Name: s.db.Name, User: s.migrate.BootstrapUser,
			SSLMode: s.db.SSLMode, SSLRootCert: s.db.SSLRootCert,
			Password: secretBytes(provider, s.migrate.BootstrapPasswordRef)},
		Stderr: o.ToolOutput, Mirror: mirror,
	})
	if err != nil {
		return nil, err
	}
	var rec statusapp.Metrics
	if o.Telemetry != nil {
		r, err := otelmetrics.New(o.Telemetry.MeterProvider())
		if err != nil {
			return nil, err
		}
		rec = r
	}
	if err := os.MkdirAll(o.DumpDir, 0o700); err != nil {
		return nil, fmt.Errorf("create the dump directory: %w", err)
	}
	runsPool, err := pg.Open(ctx, s.db, provider, log)
	if err != nil {
		return nil, err
	}
	superPool, err := pg.Open(ctx, manifestDB(s), provider, log)
	if err != nil {
		runsPool.Close()
		return nil, err
	}
	clk := clock.System()
	deps := statusapp.Deps{
		Runs:    statuspg.NewRuns(runsPool, clk, ids.NewGenerator(clk, rand.Reader)),
		Exec:    exec,
		Source:  statuspg.NewManifestSource(superPool),
		Metrics: rec, Clock: clk, Log: log,
	}
	if o.RestoreDir != "" {
		sc, err := restoreScratch(s, o, exec, provider, log)
		if err != nil {
			runsPool.Close()
			superPool.Close()
			return nil, err
		}
		deps.Scratch, deps.Chain = sc, auditapp.RowsVerifier{}
	}
	svc, err := statusapp.New(serviceConfig(s, o), deps)
	if err != nil {
		runsPool.Close()
		superPool.Close()
		return nil, err
	}
	return &BackupDaemon{svc: svc, exec: exec, runsPool: runsPool, superPool: superPool, wait: s.migrate.DependencyWait, log: log}, nil
}

// restoreScratch is the scratch area of the restore test in o.RestoreDir: pgBackRest and restic
// through exec, the copy opened on its socket as migrate.bootstrap_user (restoreDB).
func restoreScratch(s backupdSettings, o BackupDaemonOptions, exec *executor.Executor, provider *secrets.Provider, log *slog.Logger) (*scratch.Scratch, error) {
	if err := os.MkdirAll(o.RestoreDir, 0o700); err != nil {
		return nil, fmt.Errorf("create the restore directory: %w", err)
	}
	filesRoot := ""
	if s.backup.FilesDriver == driverFS {
		filesRoot = s.backup.FilesRoot
	}
	return scratch.New(scratch.Config{
		Dir: o.RestoreDir, Tools: exec, FilesRoot: filesRoot, Stderr: o.ToolOutput,
		Open: func(ctx context.Context, socket string) (*pg.Pool, error) {
			return pg.Open(ctx, restoreDB(s, socket), provider, log)
		},
	})
}

// restoreDB is the pool of the restored copy: the Unix socket in the scratch directory, never a
// host of the network, as migrate.bootstrap_user, without TLS (a local socket) and without
// statement deadline, like the manifest pool. The scratch pg_hba.conf admits the socket, so the
// password reference of the bootstrap user is resolved but not needed.
func restoreDB(s backupdSettings, socket string) pg.Config {
	c := s.db
	c.Host, c.Port, c.SSLMode, c.SSLRootCert = socket, restoreSocketPort, pg.SSLModeDisable, ""
	c.User, c.PasswordRef, c.MaxConns = s.migrate.BootstrapUser, s.migrate.BootstrapPasswordRef, restoreMaxConns
	c.StatementTimeout, c.IdleInTxTimeout = 0, 0
	return c.WithAppName(backupdRestoreAppName)
}

// manifestDB is the pool of the manifest queries as migrate.bootstrap_user. The queries count
// every table and may run long, so it has no statement or idle deadline, like bothy migrate
// (ForMigrate); it never writes.
func manifestDB(s backupdSettings) pg.Config {
	super := s.db
	super.User, super.PasswordRef, super.MaxConns = s.migrate.BootstrapUser, s.migrate.BootstrapPasswordRef, manifestMaxConns
	super.StatementTimeout, super.IdleInTxTimeout = 0, 0
	return super.WithAppName(backupdManifestAppName)
}

// serviceConfig is the configuration of the use cases.
func serviceConfig(s backupdSettings, o BackupDaemonOptions) statusapp.Config {
	volumes := map[string]string{}
	for name, path := range o.Volumes {
		volumes[name] = path
	}
	if s.backup.FilesDriver == driverFS {
		volumes["files"] = s.backup.FilesRoot
	}
	return statusapp.Config{
		Schedules: s.schedules, OffsiteEnabled: s.target != nil, ObjectsDriver: s.backup.FilesDriver,
		FilesRoot: s.backup.FilesRoot, DumpDir: o.DumpDir,
		LocalRetentionFull: s.backup.PGLocalRetentionFull, RetentionFull: s.backup.PGRetentionFull,
		LogicalRetention: s.backup.LogicalRetention, ObjectsRetention: s.backup.ObjectsRetention,
		ServerVersion: o.ServerVersion, Volumes: volumes,
		Restore: statusapp.RestoreConfig{
			Schedule: s.restore, Scheduled: o.RestoreDir != "",
			SampleThreads: s.backup.RestoreTestSampleThreads, SampleFiles: s.backup.RestoreTestSampleFiles,
			RTOTarget: s.backup.RTOTarget,
		},
	}
}

// secretBytes reads the secret of ref through the provider on every call.
func secretBytes(p *secrets.Provider, ref string) func(ctx context.Context) ([]byte, error) {
	return func(ctx context.Context) ([]byte, error) {
		val, _, err := p.Resolve(ctx, ref)
		if err != nil {
			return nil, err
		}
		return val.Bytes(), nil
	}
}

// parsedBackupSchedules parses the four schedules of settings (the check backup_schedules already
// held them at load).
func parsedBackupSchedules(s BackupConfig) (map[statusdomain.Kind]statusdomain.Schedule, error) {
	exprs := map[statusdomain.Kind]struct{ key, expr string }{
		statusdomain.KindPGFull:  {"backup.pg.full_schedule", s.PGFullSchedule},
		statusdomain.KindPGDiff:  {"backup.pg.diff_schedule", s.PGDiffSchedule},
		statusdomain.KindLogical: {"backup.logical.schedule", s.LogicalSchedule},
		statusdomain.KindObjects: {"backup.objects.schedule", s.ObjectsSchedule},
	}
	out := make(map[statusdomain.Kind]statusdomain.Schedule, len(exprs))
	for kind, e := range exprs {
		sched, err := statusdomain.ParseSchedule(e.expr)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", e.key, err)
		}
		out[kind] = sched
	}
	return out, nil
}

// backupS3Source are the keys of the file bucket the mirror reads (file.s3.*).
type backupS3Source struct {
	Endpoint     string `config:"file.s3.endpoint"`
	Region       string `config:"file.s3.region"`
	Bucket       string `config:"file.s3.bucket"`
	Prefix       string `config:"file.s3.prefix"`
	UsePathStyle bool   `config:"file.s3.use_path_style"`
	AccessKeyRef string `config:"file.s3.access_key_ref"`
	SecretKeyRef string `config:"file.s3.secret_key_ref"`
}

// backupMirror returns the object mirror of the driver s3 into the off-site bucket, or nil with
// the driver fs or without an off-site target (then the object backup of the driver s3 fails
// with s3_mirror_not_configured: a mirror needs a second bucket).
func backupMirror(v *config.Values, s BackupConfig, target *statusdomain.OffsiteTarget, provider *secrets.Provider, secretsDir, scratch string, log *slog.Logger) (executor.Mirror, error) {
	if s.FilesDriver != driverS3 || target == nil {
		return nil, nil
	}
	var src backupS3Source
	if err := config.Bind(v, &src); err != nil {
		return nil, &BackupConfigError{err: err}
	}
	var limits filesStorageValues
	if err := config.Bind(v, &limits); err != nil {
		return nil, &BackupConfigError{err: err}
	}
	eg, err := EgressConfig(v)
	if err != nil {
		return nil, &BackupConfigError{err: err}
	}
	keyDir := filepath.Join(secretsDir, offsiteDir)
	var ca []byte
	if b, err := os.ReadFile(filepath.Join(keyDir, "ca.crt")); err == nil { //nolint:gosec // G304: a fixed name below secrets.dir.
		ca = b
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("read %s/ca.crt: %w", offsiteDir, err)
	}
	m, err := s3mirror.New(s3mirror.Config{
		Source: s3mirror.Source{
			Endpoint: src.Endpoint, Region: src.Region, Bucket: src.Bucket, Prefix: src.Prefix, UsePathStyle: src.UsePathStyle,
			Credentials: refCredentials(provider, src.AccessKeyRef, src.SecretKeyRef),
		},
		Destination: s3mirror.Destination{
			Target: *target, Region: s.OffsiteRegion, UsePathStyle: s.OffsiteUsePathStyle, CAPEM: ca,
			Credentials: fileCredentials(keyDir),
		},
		ScratchDir: scratch,
		Egress:     eg, EgressDeps: egress.Deps{Logger: log},
		MaxObjectBytes: limits.MaxUploadBytes, TransferTimeout: limits.TransferTimeout,
	})
	if err != nil {
		return nil, &BackupConfigError{err: err}
	}
	return m, nil
}

// refCredentials reads the two keys of the file bucket through the secret provider.
func refCredentials(p *secrets.Provider, idRef, secretRef string) s3mirror.Credentials {
	return func(ctx context.Context) (string, string, error) {
		id, _, err := p.Resolve(ctx, idRef)
		if err != nil {
			return "", "", err
		}
		key, _, err := p.Resolve(ctx, secretRef)
		if err != nil {
			return "", "", err
		}
		return strings.TrimSpace(string(id.Bytes())), strings.TrimSpace(string(key.Bytes())), nil
	}
}

// fileCredentials reads access_key_id and secret_access_key of dir on every call.
func fileCredentials(dir string) s3mirror.Credentials {
	return func(context.Context) (string, string, error) {
		id, err := os.ReadFile(filepath.Join(dir, "access_key_id")) //nolint:gosec // G304: a fixed name below secrets.dir.
		if err != nil {
			return "", "", fmt.Errorf("read %s/access_key_id: %w", offsiteDir, errors.Unwrap(err))
		}
		key, err := os.ReadFile(filepath.Join(dir, "secret_access_key")) //nolint:gosec // G304: a fixed name below secrets.dir.
		if err != nil {
			return "", "", fmt.Errorf("read %s/secret_access_key: %w", offsiteDir, errors.Unwrap(err))
		}
		return strings.TrimSpace(string(id)), strings.TrimSpace(string(key)), nil
	}
}

// IsBackupKind reports whether s is pg_full, pg_diff, logical or objects.
func IsBackupKind(s string) bool {
	_, ok := statusdomain.ParseKind(s)
	return ok
}

// Warnings returns the warnings of the start log ("local backup only" without an off-site
// target).
func (d *BackupDaemon) Warnings() []string { return d.svc.Warnings() }

// WaitReady waits up to startup.dependency_wait until the database answers as bothy_backup.
func (d *BackupDaemon) WaitReady(ctx context.Context) error { return d.runsPool.WaitReady(ctx, d.wait) }

// Run runs one kind now; see statusapp.Service.Run. It returns ErrUnknownBackupKind for any
// other string, and the error of the run (statusapp.ErrBusy when the slot runs already).
func (d *BackupDaemon) Run(ctx context.Context, kind string) error {
	k, ok := statusdomain.ParseKind(kind)
	if !ok {
		return ErrUnknownBackupKind
	}
	return d.svc.Run(ctx, k)
}

// RestoreTest runs the restore test once now; see statusapp.Service.RestoreTest. It returns
// statusapp.ErrBusy while one runs and an error when a check failed.
func (d *BackupDaemon) RestoreTest(ctx context.Context) error { return d.svc.RestoreTest(ctx) }

// Prime records the last successful run per kind in the gauges; an error is logged and
// returned.
func (d *BackupDaemon) Prime(ctx context.Context) error {
	err := d.svc.Prime(ctx)
	if err != nil {
		d.log.WarnContext(ctx, "bothy-backupd: read the last successful runs", slog.String(backupdLogKeyError, err.Error()))
	}
	return err
}

// Sample records the WAL archive time and the volumes.
func (d *BackupDaemon) Sample(ctx context.Context) { d.svc.Sample(ctx) }

// ExpireOffsite runs the owner's off-site expiry with the key in dir.
func (d *BackupDaemon) ExpireOffsite(ctx context.Context, dir string) error {
	return d.svc.ExpireOffsite(ctx, dir)
}

// PgBackRestEnv returns the environment of repo2 for the long-running pgBackRest children
// (pgbackrest server, stanza-create): the off-site repository with the key of offsite/, and
// expire-auto off.
func (d *BackupDaemon) PgBackRestEnv(ctx context.Context) ([]string, error) {
	return d.exec.PgBackRestEnv(ctx)
}

// NewScheduler returns the scheduler of the daemon that starts kinds due after start.
func (d *BackupDaemon) NewScheduler(start time.Time) *BackupScheduler {
	return &BackupScheduler{s: statusapp.NewScheduler(d.svc, start)}
}

// Close closes both pools.
func (d *BackupDaemon) Close() {
	d.runsPool.Close()
	d.superPool.Close()
}

// BackupScheduler starts the due kinds of a BackupDaemon; see statusapp.Scheduler.
type BackupScheduler struct{ s *statusapp.Scheduler }

// Tick starts every kind due in the window up to now and returns their names.
func (s *BackupScheduler) Tick(ctx context.Context, now time.Time) []string {
	kinds := s.s.Tick(ctx, now)
	out := make([]string, len(kinds))
	for i, k := range kinds {
		out[i] = string(k)
	}
	return out
}

// Wait waits for every run Tick started.
func (s *BackupScheduler) Wait() { s.s.Wait() }
