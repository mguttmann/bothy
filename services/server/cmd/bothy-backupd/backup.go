// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

// backupCommand runs "bothy-backupd backup <kind> [--config FILE]": one run of kind now, for the
// runbook and the tests (brief M7-02 design note 1). It returns 0 when the run succeeded, 1 when
// it failed or the slot of the kind runs already, 2 for a usage or configuration error. The run
// writes its backup_runs row and, on success, its manifest; the metrics of the run, the WAL
// archive and the volumes are exported before the command ends.
func backupCommand(ctx context.Context, args, env []string, stderr io.Writer) int {
	fs := flag.NewFlagSet("bothy-backupd backup", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	file := fs.String("config", "", "configuration file")
	kind, rest := "", args
	if len(rest) > 0 && rest[0] != "" && rest[0][0] != '-' {
		kind, rest = rest[0], rest[1:]
	}
	if err := fs.Parse(rest); err != nil {
		return usageError(stderr, "backup: "+err.Error())
	}
	if kind == "" && fs.NArg() > 0 {
		kind = fs.Arg(0)
	} else if fs.NArg() > 0 {
		return usageError(stderr, fmt.Sprintf("backup: unexpected argument %q", fs.Arg(0)))
	}
	if !wire.IsBackupKind(kind) {
		return usageError(stderr, fmt.Sprintf("backup: unknown kind %q (pg_full, pg_diff, logical, objects)", kind))
	}
	d, code := loadDaemon(ctx, env, *file, stderr, stderr)
	if d == nil {
		return code
	}
	defer d.close(ctx)
	for _, w := range d.d.Warnings() {
		_, _ = fmt.Fprintf(stderr, "bothy-backupd: warning: %s\n", w)
	}
	if err := d.d.WaitReady(ctx); err != nil {
		_, _ = fmt.Fprintf(stderr, "bothy-backupd: %v\n", err)
		return exitFailure
	}
	_ = d.d.Prime(ctx) // logged by Prime; the run itself needs no history
	err := d.d.Run(ctx, kind)
	d.d.Sample(ctx)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "bothy-backupd: backup %s: %v\n", kind, err)
		return exitFailure
	}
	_, _ = fmt.Fprintf(stderr, "bothy-backupd: backup %s succeeded\n", kind)
	return exitOK
}

// expireCommand runs "bothy-backupd expire --offsite --credentials DIR [--config FILE]", the
// owner's expiry of the off-site target (R-OP-09, docs/operations/backup-restore.md 9). The key
// in DIR is the owner's separate account; the secrets of the service never delete off-site.
func expireCommand(ctx context.Context, args, env []string, stderr io.Writer) int {
	fs := flag.NewFlagSet("bothy-backupd expire", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	offsite := fs.Bool("offsite", false, "expire the off-site target")
	dir := fs.String("credentials", "", "directory of the owner's off-site key")
	file := fs.String("config", "", "configuration file")
	if err := fs.Parse(args); err != nil {
		return usageError(stderr, "expire: "+err.Error())
	}
	switch {
	case fs.NArg() > 0:
		return usageError(stderr, fmt.Sprintf("expire: unexpected argument %q", fs.Arg(0)))
	case !*offsite:
		return usageError(stderr, "expire: only --offsite exists; repo1 expires after every physical backup")
	case *dir == "":
		return usageError(stderr, "expire: --credentials needs the directory of the owner's key")
	}
	d, code := loadDaemon(ctx, env, *file, stderr, stderr)
	if d == nil {
		return code
	}
	defer d.close(ctx)
	if err := d.d.ExpireOffsite(ctx, *dir); err != nil {
		_, _ = fmt.Fprintf(stderr, "bothy-backupd: expire --offsite: %v\n", err)
		return exitFailure
	}
	_, _ = io.WriteString(stderr, "bothy-backupd: expire --offsite succeeded\n")
	return exitOK
}

// restoreTestCommand runs "bothy-backupd restore-test [--config FILE]": the automatic restore
// test once now (docs/spec/16-betrieb.md 8.3, brief M7-03), for the runbook and the tests. It
// returns 0 when every check passed, 1 when a check failed or a test runs already, 2 for a usage
// or configuration error. The test writes its restore_test row of backup_runs; its metrics are
// exported before the command ends.
func restoreTestCommand(ctx context.Context, args, env []string, stderr io.Writer) int {
	fs := flag.NewFlagSet("bothy-backupd restore-test", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	file := fs.String("config", "", "configuration file")
	if err := fs.Parse(args); err != nil {
		return usageError(stderr, "restore-test: "+err.Error())
	}
	if fs.NArg() > 0 {
		return usageError(stderr, fmt.Sprintf("restore-test: unexpected argument %q", fs.Arg(0)))
	}
	d, code := loadDaemon(ctx, env, *file, stderr, stderr)
	if d == nil {
		return code
	}
	defer d.close(ctx)
	if err := d.d.WaitReady(ctx); err != nil {
		_, _ = fmt.Fprintf(stderr, "bothy-backupd: %v\n", err)
		return exitFailure
	}
	_ = d.d.Prime(ctx) // logged by Prime; the test itself needs no history
	err := d.d.RestoreTest(ctx)
	d.d.Sample(ctx)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "bothy-backupd: restore-test: %v\n", err)
		return exitFailure
	}
	_, _ = io.WriteString(stderr, "bothy-backupd: restore-test succeeded\n")
	return exitOK
}

// scheduleTarget is what the scheduler loop drives; daemonTarget adapts *wire.BackupDaemon.
type scheduleTarget interface {
	Prime(ctx context.Context) error
	Sample(ctx context.Context)
	NewScheduler(start time.Time) ticker
}

// ticker starts the due kinds; *wire.BackupScheduler implements it.
type ticker interface {
	Tick(ctx context.Context, now time.Time) []string
	Wait()
}

// daemonTarget is the scheduleTarget of a *wire.BackupDaemon.
type daemonTarget struct{ *wire.BackupDaemon }

// NewScheduler implements scheduleTarget.
func (d daemonTarget) NewScheduler(start time.Time) ticker { return d.BackupDaemon.NewScheduler(start) }

// schedule runs the scheduler of d until ctx ends: at the start of every minute of clk it starts
// the kinds that became due since the previous tick (none before the start of the process, so
// missed times are not caught up) and samples the WAL archive and the volumes. wait sleeps until
// an instant of clk or until ctx ends and reports whether the instant was reached. On return
// every started run has finished; a run canceled by the stop is recorded as failed with the code
// canceled.
func schedule(ctx context.Context, d scheduleTarget, clk clock.Clock, wait func(ctx context.Context, clk clock.Clock, at time.Time) bool) {
	// The database may not be up yet: Prime logs the error, and the gauges start empty and fill
	// with the first runs.
	_ = d.Prime(ctx)
	sched := d.NewScheduler(clk.Now())
	defer sched.Wait()
	d.Sample(ctx)
	for {
		next := clk.Now().Truncate(time.Minute).Add(time.Minute)
		if !wait(ctx, clk, next) {
			return
		}
		sched.Tick(ctx, clk.Now())
		d.Sample(ctx)
	}
}

// sleepUntil waits until clk reports at or later, or until ctx ends; it returns false when ctx
// ended. The system clock needs a timer; the wait is re-checked because a timer may fire early
// against a clock that was set back.
func sleepUntil(ctx context.Context, clk clock.Clock, at time.Time) bool {
	for {
		d := at.Sub(clk.Now())
		if d <= 0 {
			return true
		}
		t := time.NewTimer(d)
		select {
		case <-ctx.Done():
			t.Stop()
			return false
		case <-t.C:
		}
	}
}
