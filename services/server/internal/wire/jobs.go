// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"context"
	"database/sql"
	"errors"
	"io/fs"
	"time"

	authzjobs "github.com/OWNER/bothy/services/server/internal/authz/adapters/jobs"
	instancejobs "github.com/OWNER/bothy/services/server/internal/instance/adapters/jobs"
	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/platform/jobs"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	syncjobs "github.com/OWNER/bothy/services/server/internal/sync/adapters/jobs"
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
}

// JobsConfig binds the configuration of the job runtime from v: jobs.schema, the parallelism of
// the five queues (jobs.queue.<queue>.max_workers) and shutdown.jobs_soft_timeout. The logger is
// left to the caller. The error names the key whose binding failed, never a value.
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
		SoftStopTimeout: k.SoftStopTimeout,
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
// (M1-16) and the idempotency key cleanup (M1-20) of sync, the expiry jobs of authz (M1-30) and
// the rotation of the signing keys of instance (M1-32). The use cases may be nil where no job runs
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
}

// NewJobRegistry returns the job registry of a process: every role (api, worker, all) builds
// the same one here, so an api process accepts exactly the kinds a worker runs. It registers
// sync.partition_maintenance (M1-16), sync.idempotency_cleanup (M1-20), authz.expiry_sweep and
// authz.expiry_reminder (M1-30) and instance.key_rotation (M1-32); the error is the first rejected
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
	return reg, nil
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
