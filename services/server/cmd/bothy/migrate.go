// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"time"

	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/secrets"
	"github.com/OWNER/bothy/services/server/internal/wire"
	"github.com/OWNER/bothy/services/server/migrations"
)

// migrateUsage is the help of "bothy migrate" (docs/spec/02-architektur.md "Befehle des
// Binarys", docs/spec/16-betrieb.md "Rollen" and "Upgrade").
const migrateUsage = `Usage: bothy migrate up [--bootstrap] [--config FILE]
       bothy migrate status [--config FILE]
       bothy migrate validate

up        applies every pending migration of this binary as database.user, which must own
          schema public (bothy_owner); every step runs with lock_timeout =
          migrate.lock_timeout. It prints "applied <version> <name>" per migration and
          "database version <n>, applied <k>". Against a schema newer than this binary it
          prints "schema is newer than this binary; nothing to do" and changes nothing.
status    prints "<version> <name> <applied|pending> <applied_at|->" per migration of this
          binary and "database version <n>, binary requires <m>, pending <k>"; it only reads.
validate  checks the embedded migrations without a database (names, numbering, goose
          syntax) and prints "migrations ok".

--bootstrap    up only: first connect as migrate.bootstrap_user and create or update the roles
               bothy_owner, bothy_app, bothy_maint and bothy_monitor with the passwords of
               migrate.*_password_ref (sent only as SCRAM-SHA-256 verifiers) and the grants on
               database.name. Managed databases create the roles with their own tooling.
--config FILE  configuration file, not empty (default: BOTHY_CONFIG_FILE, else none)

Both up and status wait up to startup.dependency_wait for the database.

Exit codes: 0 ok, 1 failure, 2 usage or configuration error.
`

// The subcommands of bothy migrate.
const (
	subcommandUp       = "up"
	subcommandStatus   = "status"
	subcommandValidate = "validate"
)

// newerSchemaNotice is what migrate up prints against a database a newer binary migrated
// (docs/operations/upgrade.md "Rückweg").
const newerSchemaNotice = "schema is newer than this binary; nothing to do\n"

// migrateEnv is what the migrate subcommands need besides their flags; tests replace the
// migrations and the schema.
type migrateEnv struct {
	env    []string
	schema func() (*config.Schema, error)
	fsys   fs.FS
	steps  func() pg.MigratorOptions
	stdout io.Writer
	stderr io.Writer
}

// migrateCommand runs "bothy migrate <up|status|validate> [flags]" with the arguments after
// "migrate" and returns the exit code: 0 success, 1 failure, 2 usage or configuration error.
func migrateCommand(ctx context.Context, args []string, e migrateEnv) int {
	if len(args) == 0 {
		return migrateUsageError(e.stderr, "")
	}
	sub := args[0]
	if isHelpArg(sub) {
		_, _ = io.WriteString(e.stdout, migrateUsage)
		return exitOK
	}
	switch sub {
	case subcommandUp, subcommandStatus, subcommandValidate:
	default:
		return migrateUsageError(e.stderr, fmt.Sprintf("unknown subcommand %q", sub))
	}
	o, err := parseMigrateFlags(sub, args[1:])
	if errors.Is(err, flag.ErrHelp) {
		_, _ = io.WriteString(e.stdout, migrateUsage)
		return exitOK
	}
	if err != nil {
		return migrateUsageError(e.stderr, err.Error())
	}
	if sub == subcommandValidate {
		return migrateValidate(e)
	}
	r, code := newMigrateRun(o, e)
	if r == nil {
		return code
	}
	if sub == subcommandStatus {
		return r.status(ctx)
	}
	return r.up(ctx, o.bootstrap)
}

// migrateOptions holds the flags of the migrate subcommands.
type migrateOptions struct {
	file      string
	bootstrap bool
}

// parseMigrateFlags parses the flags of sub: --config for up and status, --bootstrap for up,
// none for validate. It returns flag.ErrHelp for -h and --help; the message of any other error
// is the line migrateUsageError prints.
func parseMigrateFlags(sub string, args []string) (migrateOptions, error) {
	fs := flag.NewFlagSet("bothy migrate "+sub, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var o migrateOptions
	if sub != subcommandValidate {
		fs.StringVar(&o.file, "config", "", "configuration file")
	}
	if sub == subcommandUp {
		fs.BoolVar(&o.bootstrap, "bootstrap", false, "create the roles first")
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return migrateOptions{}, err
		}
		return migrateOptions{}, errors.New(printable(err.Error()))
	}
	if fs.NArg() > 0 {
		return migrateOptions{}, fmt.Errorf("unexpected argument %q", printable(fs.Arg(0)))
	}
	if o.file == "" && flagGiven(fs, "config") {
		return migrateOptions{}, errors.New("--config needs a file path")
	}
	return o, nil
}

// migrateUsageError prints msg (when set) and the usage on stderr and returns the usage exit
// code.
func migrateUsageError(stderr io.Writer, msg string) int {
	if msg != "" {
		_, _ = fmt.Fprintf(stderr, "bothy migrate: %s\n", msg)
	}
	_, _ = io.WriteString(stderr, migrateUsage)
	return exitUsage
}

// migrateValidate runs migrate validate.
func migrateValidate(e migrateEnv) int {
	if err := pg.Validate(e.fsys); err != nil {
		_, _ = fmt.Fprintf(e.stderr, "bothy migrate: %v\n", err)
		return exitFailure
	}
	_, _ = io.WriteString(e.stdout, "migrations ok\n")
	return exitOK
}

// migrateRun is one run of migrate up or status with the loaded configuration.
type migrateRun struct {
	e        migrateEnv
	log      *slog.Logger
	provider *secrets.Provider
	db       pg.Config
	settings wire.MigrateConfig
}

// newMigrateRun loads the configuration as the server start does and builds the secret
// provider. A configuration that does not load or bind is exit code 2 (02 "Befehle des
// Binarys": configuration error); it returns nil and the code then.
func newMigrateRun(o migrateOptions, e migrateEnv) (*migrateRun, int) {
	fail := func(err error) (*migrateRun, int) {
		_, _ = fmt.Fprintf(e.stderr, "bothy migrate: %v\n", err)
		return nil, exitUsage
	}
	s, err := e.schema()
	if err != nil {
		return fail(err)
	}
	values, err := config.Load(s, config.Sources{Env: e.env, File: o.file})
	if err != nil {
		return fail(err)
	}
	log := slog.New(slog.NewTextHandler(e.stderr, nil))
	provider, err := wire.NewSecretsProvider(values, e.env, log)
	if err != nil {
		return fail(err)
	}
	db, err := wire.DatabaseConfig(values, "bothy-migrate")
	if err != nil {
		return fail(err)
	}
	settings, err := wire.MigrateSettings(values)
	if err != nil {
		return fail(err)
	}
	if err := db.Validate(); err != nil {
		return fail(err)
	}
	return &migrateRun{e: e, log: log, provider: provider, db: db, settings: settings}, exitOK
}

// failf prints one error line on stderr and returns exit code 1.
func (r *migrateRun) failf(format string, args ...any) int {
	_, _ = fmt.Fprintf(r.e.stderr, "bothy migrate: "+format+"\n", args...)
	return exitFailure
}

// migrator connects as database.user with the session settings of migrate, waits for the
// database and returns the migrator and the function that closes both.
func (r *migrateRun) migrator(ctx context.Context) (*pg.Migrator, func(), error) {
	pool, err := pg.Open(ctx, r.db.ForMigrate(r.settings.LockTimeout), r.provider, r.log)
	if err != nil {
		return nil, nil, err
	}
	if err := pool.WaitReady(ctx, r.settings.DependencyWait); err != nil {
		pool.Close()
		return nil, nil, err
	}
	db := pool.OpenSQL()
	closeAll := func() {
		_ = db.Close()
		pool.Close()
	}
	m, err := pg.NewMigrator(db, r.e.fsys, r.e.steps())
	if err != nil {
		closeAll()
		return nil, nil, err
	}
	return m, closeAll, nil
}

// up runs migrate up, with the role bootstrap first when bootstrap is set.
func (r *migrateRun) up(ctx context.Context, bootstrap bool) int {
	if bootstrap {
		if err := r.bootstrap(ctx); err != nil {
			return r.failf("bootstrap: %v", err)
		}
		_, _ = io.WriteString(r.e.stdout, "bootstrap: roles and grants are in place\n")
	}
	m, closeAll, err := r.migrator(ctx)
	if err != nil {
		return r.failf("%v", err)
	}
	defer closeAll()
	res, err := m.Up(ctx)
	r.printApplied(res.Applied)
	if err != nil {
		return r.failf("%v", err)
	}
	if res.Newer {
		_, _ = io.WriteString(r.e.stdout, newerSchemaNotice)
		return exitOK
	}
	_, _ = fmt.Fprintf(r.e.stdout, "database version %d, applied %d\n", res.Version, len(res.Applied))
	return exitOK
}

func (r *migrateRun) printApplied(applied []pg.AppliedMigration) {
	for _, a := range applied {
		_, _ = fmt.Fprintf(r.e.stdout, "applied %d %s\n", a.Version, a.Name)
	}
}

// bootstrap connects as migrate.bootstrap_user to database.name and runs pg.Bootstrap with the
// four passwords of migrate.*_password_ref (docs/spec/16-betrieb.md "Rollen").
func (r *migrateRun) bootstrap(ctx context.Context) error {
	passwords := make(map[pg.Role]secrets.Value, len(pg.Roles()))
	for role, ref := range r.settings.PasswordRefs() {
		v, _, err := r.provider.Resolve(ctx, ref)
		if err != nil {
			return fmt.Errorf("password of %s: %w", role, err)
		}
		passwords[role] = v
	}
	super := r.db.ForMigrate(r.settings.LockTimeout)
	super.User, super.PasswordRef, super.MaxConns = r.settings.BootstrapUser, r.settings.BootstrapPasswordRef, 1
	conn, err := pg.Connect(ctx, super, r.provider, r.settings.DependencyWait, r.log)
	if err != nil {
		return err
	}
	defer func() {
		closeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		_ = conn.Close(closeCtx)
	}()
	return pg.Bootstrap(ctx, conn, r.db.Name, passwords)
}

// status runs migrate status.
func (r *migrateRun) status(ctx context.Context) int {
	m, closeAll, err := r.migrator(ctx)
	if err != nil {
		return r.failf("%v", err)
	}
	defer closeAll()
	entries, version, err := m.Status(ctx)
	if err != nil {
		return r.failf("%v", err)
	}
	pending := 0
	for _, s := range entries {
		state, at := "applied", s.AppliedAt.UTC().Format(time.RFC3339)
		if !s.Applied {
			state, at = "pending", "-"
			pending++
		}
		_, _ = fmt.Fprintf(r.e.stdout, "%d %s %s %s\n", s.Version, s.Name, state, at)
	}
	_, _ = fmt.Fprintf(r.e.stdout, "database version %d, binary requires %d, pending %d\n", version, m.Highest(), pending)
	return exitOK
}

// defaultMigrateEnv is the environment of the real command: the server schema, the embedded
// migrations and the steps of internal/wire.
func defaultMigrateEnv(env []string, stdout, stderr io.Writer) migrateEnv {
	return migrateEnv{
		env: env, schema: wire.ConfigSchema, fsys: migrations.FS, steps: wire.MigrationSteps,
		stdout: stdout, stderr: stderr,
	}
}
