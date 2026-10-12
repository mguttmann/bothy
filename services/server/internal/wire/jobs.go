// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"context"
	"database/sql"
	"io/fs"
	"time"

	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/platform/jobs"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
)

// jobsKeys are the configuration keys of the job runtime (02 "Konfigurationswerte dieses
// Kapitels").
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
// all run jobs (02 "Rollen eines Prozesses").
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

// NewJobRegistry returns the job registry of a process: every role (api, worker, all) builds
// the same one here, so an api process accepts exactly the kinds a worker runs. It is empty
// until the first module with a job (sync.partition_maintenance, M1-16) adds its Register and
// AddPeriodic calls; the error is the first rejected registration.
func NewJobRegistry(_ *Modules) (*jobs.Registry, error) {
	return jobs.NewRegistry(), nil
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

// MigrateUpSteps returns the steps bothy migrate up runs around the goose migrations of fsys on
// pool, the pool of migrate as bothy_owner: River's migrations into schema (jobs.schema) before
// them (M1-14, upgrade.md rule M11) and the steps of MigrationSteps after them (the instance row,
// M1-15). result receives what the River step applied; it may be nil. pgtest.WithSteps takes
// MigrationSteps without River, because the serve tests add River's tables themselves
// (docs/DECISIONS.md INT-W5/3). The error is the one of MigrationSteps.
func MigrateUpSteps(pool *pg.Pool, fsys fs.FS, schema string, result *jobs.MigrateResult) (pg.MigratorOptions, error) {
	opts, err := MigrationSteps(pool, fsys)
	if err != nil {
		return pg.MigratorOptions{}, err
	}
	opts.Before = append([]pg.Step{riverStep{pool: pool, schema: schema, result: result}}, opts.Before...)
	return opts, nil
}
