// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"context"
	"database/sql"
	"errors"
	"io/fs"
	"time"

	appsjobs "github.com/OWNER/bothy/services/server/internal/apps/adapters/jobs"
	auditjobs "github.com/OWNER/bothy/services/server/internal/audit/adapters/jobs"
	authzjobs "github.com/OWNER/bothy/services/server/internal/authz/adapters/jobs"
	billingjobs "github.com/OWNER/bothy/services/server/internal/billing/adapters/jobs"
	filesjobs "github.com/OWNER/bothy/services/server/internal/files/adapters/jobs"
	identityjobs "github.com/OWNER/bothy/services/server/internal/identity/adapters/jobs"
	instancejobs "github.com/OWNER/bothy/services/server/internal/instance/adapters/jobs"
	notifyjobs "github.com/OWNER/bothy/services/server/internal/notify/adapters/jobs"
	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/platform/jobs"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	runsjobs "github.com/OWNER/bothy/services/server/internal/runs/adapters/jobs"
	syncjobs "github.com/OWNER/bothy/services/server/internal/sync/adapters/jobs"
	terminaljobs "github.com/OWNER/bothy/services/server/internal/terminal/adapters/jobs"
)

// jobsKeys are the configuration keys of the job runtime (02 "Configuration values
// of this chapter").
type jobsKeys struct {
	Schema          string        `config:"jobs.schema"`
	Default         int           `config:"jobs.queue.default.max_workers"`
	Files           int           `config:"jobs.queue.files.max_workers"`
	Delivery        int           `config:"jobs.queue.delivery.max_workers"`
	Maintenance     int           `config:"jobs.queue.maintenance.max_workers"`
	Privacy         int           `config:"jobs.queue.privacy.max_workers"`
	SoftStopTimeout time.Duration `config:"shutdown.jobs_soft_timeout"`
	// StatementTimeout bounds the count query of bothy.job.queue (M1-68).
	StatementTimeout time.Duration `config:"database.statement_timeout"`
}

// JobsConfig binds the configuration of the job runtime from v: jobs.schema, the parallelism of
// the five queues (jobs.queue.<queue>.max_workers), shutdown.jobs_soft_timeout and
// database.statement_timeout (the bound of the count query of bothy.job.queue). The logger and
// the meter provider (nil: the global one) are left to the caller. The error names the key whose binding failed, never a value.
func JobsConfig(v *config.Values) (jobs.Config, error) {
	var k jobsKeys
	if err := config.Bind(v, &k); err != nil {
		return jobs.Config{}, err
	}
	return jobs.Config{
		Schema: k.Schema,
		MaxWorkers: map[string]int{
			jobs.QueueDefault: k.Default, jobs.QueueFiles: k.Files, jobs.QueueDelivery: k.Delivery,
			jobs.QueueMaintenance: k.Maintenance, jobs.QueuePrivacy: k.Privacy,
		},
		SoftStopTimeout:  k.SoftStopTimeout,
		StatementTimeout: k.StatementTimeout,
	}, nil
}

// JobRole maps the process role to the role of the job runtime: api only inserts, worker and
// all run jobs (02 "Roles of a process").
func JobRole(r Role) jobs.Role {
	switch r {
	case RoleWorker:
		return jobs.RoleWorker
	case RoleAll:
		return jobs.RoleAll
	case RoleAPI:
		return jobs.RoleAPI
	default:
		// Serve refuses unknown roles before; jobs.New refuses this one as well.
		return jobs.Role(r)
	}
}

// JobModules are the module objects whose jobs the job registry takes: the partition maintenance
// (M1-16) and the idempotency key cleanup (M1-20) of sync, the expiry jobs of authz (M1-30), the
// rotation of the signing keys of instance (M1-32), the cleanup of identity (M1-35), the chain
// verification of audit (M1-50) and the webhook delivery of apps (M2-26). The use cases may be nil where no job runs
// (tests of the registration); a worker always passes them.
type JobModules struct {
	// Partitions is the use case of sync.partition_maintenance.
	Partitions syncjobs.Maintainer
	// PartitionInterval is sync.partition_maintenance_interval.
	PartitionInterval time.Duration
	// Cleanup is the use case of sync.idempotency_cleanup.
	Cleanup syncjobs.Cleaner
	// CleanupInterval is sync.idempotency_cleanup_interval.
	CleanupInterval time.Duration
	// Expiry is the use case of authz.expiry_sweep and authz.expiry_reminder.
	Expiry authzjobs.Expiry
	// ExpiryInterval is authz.expiry_sweep_interval, the period of both.
	ExpiryInterval time.Duration
	// KeyRotation is the use case of instance.key_rotation.
	KeyRotation instancejobs.Rotation
	// KeyRotationInterval is signing_key.rotation_check_interval.
	KeyRotationInterval time.Duration
	// AuthCleanup is the use case of identity.auth_cleanup.
	AuthCleanup identityjobs.Cleaner
	// AuthCleanupInterval is auth.cleanup_interval.
	AuthCleanupInterval time.Duration
	// ChainVerify is the use case of audit.chain_verify.
	ChainVerify auditjobs.Verifier
	// ChainVerifyInterval is audit.checkpoint_interval.
	ChainVerifyInterval time.Duration
	// Deliveries is the use case of apps.webhook_deliver (M2-26).
	Deliveries appsjobs.Deliverer
	// Reconciliation is the use case of billing.reconcile (M2-28); never periodic.
	Reconciliation billingjobs.Applier
	// FilesScan is the scan pass of files.scan (M3-04); nil in the role api.
	FilesScan filesjobs.Runner
	// QueuedExpiry is the use case of runs.queued_expiry (M2-08); nil only where no job runs.
	QueuedExpiry runsjobs.Expirer
	// FilesImages is the use case of files.thumbnail (M3-06); nil in the role api.
	FilesImages filesjobs.ImageRunner
	// FilesImageTimeout is file.image.processing_timeout, the River timeout of files.thumbnail.
	FilesImageTimeout time.Duration
	// FilesPurge and FilesCleanup are the use cases of files.purge_objects and
	// files.cleanup_pending (M3-08); nil in the role api. FilesGCInterval is file.gc_interval, the
	// period of files.cleanup_pending, set in every role.
	FilesPurge      filesjobs.PurgeRunner
	FilesCleanup    filesjobs.CleanupRunner
	FilesGCInterval time.Duration
	// FilesExtractText and FilesExtractRefresh are the use cases of files.extract_text and
	// files.extract_text_refresh (M3-15); nil in the role api. FilesExtractInterval is
	// search.extract.refresh_interval, the period of the refresh.
	FilesExtractText     filesjobs.TextRunner
	FilesExtractRefresh  filesjobs.RefreshRunner
	FilesExtractInterval time.Duration
	// RunsLifecycle is the use case of runs.cancel_timeout and runs.stall_check (M2-09); nil only
	// where no job runs. StallInterval is agent.presence_timeout, the interval of
	// runs.stall_check.
	RunsLifecycle runsjobs.Lifecycle
	StallInterval time.Duration
	// ComponentExpiry is the use case of runs.component_expiry (M2-15); nil only where no job
	// runs.
	ComponentExpiry runsjobs.ComponentExpirer
	// InvocationHolds is the use case of runs.invocation_hold_timeout (M3-07); nil only where no
	// job runs.
	InvocationHolds runsjobs.Releaser
	// TerminalSweep is the use case of terminal.sweep (M2-46).
	TerminalSweep terminaljobs.Sweeper
	// TerminalSweepInterval is terminal.sweep_interval.
	TerminalSweepInterval time.Duration
	// Push is the push path of notify, the use cases of notify.push_send, notify.coalesce_flush
	// and notify.badge_sync (M3-19); nil only where no job runs.
	Push notifyjobs.UseCases
}

// TerminalSweepConfig binds terminal.sweep_interval, the period of terminal.sweep (M2-46).
type TerminalSweepConfig struct {
	Interval time.Duration `config:"terminal.sweep_interval"`
}

// TerminalSweepConfigFrom binds TerminalSweepConfig from v; the error names the key, never a value.
func TerminalSweepConfigFrom(v *config.Values) (TerminalSweepConfig, error) {
	var c TerminalSweepConfig
	if err := config.Bind(v, &c); err != nil {
		return TerminalSweepConfig{}, err
	}
	return c, nil
}

// NewJobRegistry returns the job registry of a process: every role (api, worker, all) builds
// the same one here, so an api process accepts exactly the kinds a worker runs. It registers
// sync.partition_maintenance (M1-16), sync.idempotency_cleanup (M1-20), authz.expiry_sweep and
// authz.expiry_reminder (M1-30), instance.key_rotation (M1-32), identity.auth_cleanup (M1-35) and
// audit.chain_verify (M1-50), apps.webhook_deliver on the queue delivery (M2-26), billing.reconcile
// on the queue default, never periodic (M2-28), files.scan
// on the queue files without a River job timeout (M3-04), files.thumbnail on the queue files with
// the timeout file.image.processing_timeout (M3-06), files.purge_objects and the periodic
// files.cleanup_pending on the queue maintenance (M3-08), files.extract_text on the queue files
// without a River job timeout and the periodic files.extract_text_refresh on the queue maintenance
// (M3-15), runs.queued_expiry on the queue default
// (M2-08), runs.cancel_timeout and the periodic runs.stall_check on the queue maintenance (M2-09),
// runs.component_expiry, scheduled per card, on the queue maintenance (M2-15), terminal.sweep on
// the queue maintenance, periodic with terminal.sweep_interval and once at the start (M2-46), the
// six jobs of notify on the queue delivery (M3-19, notify.gateway_send M3-23) and
// runs.invocation_hold_timeout on the queue default (M3-07); the error is the first rejected
// registration.
func NewJobRegistry(m *JobModules) (*jobs.Registry, error) {
	if m == nil {
		return nil, errors.New("wire: NewJobRegistry needs the job modules")
	}
	reg := jobs.NewRegistry()
	if err := syncjobs.Register(reg, m.Partitions, m.PartitionInterval); err != nil {
		return nil, err
	}
	if err := syncjobs.RegisterIdempotencyCleanup(reg, m.Cleanup, m.CleanupInterval); err != nil {
		return nil, err
	}
	if err := authzjobs.Register(reg, m.Expiry, m.ExpiryInterval); err != nil {
		return nil, err
	}
	if err := instancejobs.Register(reg, m.KeyRotation, m.KeyRotationInterval); err != nil {
		return nil, err
	}
	if err := identityjobs.Register(reg, m.AuthCleanup, m.AuthCleanupInterval); err != nil {
		return nil, err
	}
	if err := auditjobs.Register(reg, m.ChainVerify, m.ChainVerifyInterval); err != nil {
		return nil, err
	}
	if err := appsjobs.Register(reg, m.Deliveries); err != nil {
		return nil, err
	}
	if err := billingjobs.Register(reg, m.Reconciliation); err != nil {
		return nil, err
	}
	if err := registerFilesJobs(reg, m); err != nil {
		return nil, err
	}
	if err := registerRunsJobs(reg, m); err != nil {
		return nil, err
	}
	if err := terminaljobs.Register(reg, m.TerminalSweep, m.TerminalSweepInterval); err != nil {
		return nil, err
	}
	if err := notifyjobs.Register(reg, m.Push); err != nil {
		return nil, err
	}
	return reg, nil
}

// registerFilesJobs registers the jobs of files into reg: files.scan (M3-04), the thumbnail job,
// the cleanup jobs (M3-08) and the extraction jobs (M3-15); the error is the first rejected
// registration.
func registerFilesJobs(reg *jobs.Registry, m *JobModules) error {
	if err := filesjobs.Register(reg, m.FilesScan); err != nil {
		return err
	}
	if err := filesjobs.RegisterThumbnail(reg, m.FilesImages, m.FilesImageTimeout); err != nil {
		return err
	}
	if err := filesjobs.RegisterCleanup(reg, m.FilesPurge, m.FilesCleanup, m.FilesGCInterval); err != nil {
		return err
	}
	return filesjobs.RegisterExtract(reg, m.FilesExtractText, m.FilesExtractRefresh, m.FilesExtractInterval)
}

// registerRunsJobs registers the jobs of runs into reg: runs.queued_expiry (M2-08),
// runs.cancel_timeout and runs.stall_check (M2-09), runs.component_expiry (M2-15) and
// runs.invocation_hold_timeout (M3-07); the error is the first rejected registration.
func registerRunsJobs(reg *jobs.Registry, m *JobModules) error {
	if err := runsjobs.Register(reg, m.QueuedExpiry); err != nil {
		return err
	}
	if err := runsjobs.RegisterLifecycle(reg, m.RunsLifecycle, m.StallInterval); err != nil {
		return err
	}
	if err := runsjobs.RegisterComponentExpiry(reg, m.ComponentExpiry); err != nil {
		return err
	}
	return runsjobs.RegisterHold(reg, m.InvocationHolds)
}

// riverStep is the before-step of bothy migrate up that creates the schema of River's tables
// and applies River's migrations (docs/operations/upgrade.md rule M11).
type riverStep struct {
	pool   *pg.Pool
	schema string
	// result receives what the run applied, for the output of the command.
	result *jobs.MigrateResult
}

// Name names the step in errors.
func (s riverStep) Name() string { return "river" }

// Run runs jobs.Migrate on the pool of migrate; db is the same pool behind database/sql, which
// River's driver cannot use.
func (s riverStep) Run(ctx context.Context, _ *sql.DB) error {
	res, err := jobs.Migrate(ctx, s.pool, s.schema)
	if s.result != nil {
		*s.result = res
	}
	return err
}

// MigrateUpConfig are the configured values the steps of bothy migrate up read.
type MigrateUpConfig struct {
	// JobsSchema is jobs.schema, the schema of River's tables.
	JobsSchema string
	// Sync holds the partition values of the event log (sync.event_log_partition_size,
	// sync.event_log_partitions_ahead, sync.event_retention, database.lock_timeout).
	Sync SyncConfig
}

// MigrateUpSteps returns the steps bothy migrate up runs around the goose migrations of fsys on
// pool, the pool of migrate as bothy_owner: River's migrations into cfg.JobsSchema before them
// (M1-14, upgrade.md rule M11), the steps of MigrationSteps after them (the instance row,
// M1-15), and last the partition maintenance of event_log with the values of cfg.Sync, so a fresh
// or upgraded database has partitions before serve starts (M1-16, upgrade.md rule M10). result
// receives what the River step applied; it may be nil. pgtest.WithSteps takes MigrationSteps
// without River and partitions, because the serve tests add River's tables themselves
// (docs/DECISIONS.md M1-15/16) and the event log tests create their partitions with their own
// size (M1-16/7).
//
// The error is the one of MigrationSteps, or a partition configuration the maintenance refuses.
func MigrateUpSteps(pool *pg.Pool, fsys fs.FS, cfg MigrateUpConfig, result *jobs.MigrateResult) (pg.MigratorOptions, error) {
	opts, err := MigrationSteps(pool, fsys)
	if err != nil {
		return pg.MigratorOptions{}, err
	}
	reg, err := NewTxnRegistry(NewModules(pool, nil))
	if err != nil {
		return pg.MigratorOptions{}, err
	}
	// Zero deadlines keep the session values of migrate, as in MigrationSteps.
	sy, err := NewSync(pool, pg.NewUnitOfWork(pool, pg.UnitConfig{}, reg), cfg.Sync, nil, nil)
	if err != nil {
		return pg.MigratorOptions{}, err
	}
	opts.Before = append([]pg.Step{riverStep{pool: pool, schema: cfg.JobsSchema, result: result}}, opts.Before...)
	opts.After = append(opts.After, partitionStep{maintenance: sy.Maintenance()})
	return opts, nil
}
