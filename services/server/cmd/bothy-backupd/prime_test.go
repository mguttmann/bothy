// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"maps"
	"slices"
	"strconv"
	"testing"
	"time"

	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/platform/otel"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

// primeTarget is a fakeTarget whose Prime answers with errs in order and with nil once they are
// used up; before lists, per call of Prime, the ticks the loop had run by then.
type primeTarget struct {
	*fakeTarget
	errs   []error
	before []int
}

func (p *primeTarget) Prime(context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.before = append(p.before, p.ticks)
	if len(p.errs) == 0 {
		return nil
	}
	err := p.errs[0]
	p.errs = p.errs[1:]
	return err
}

// TestSchedulePrimesUntilItSucceeds: the loop reads the last successful runs at its start. The
// database may not answer yet, so it reads them again before every tick until that succeeded
// once, and never after that. Without the repetition the gauges of the last success stayed empty
// until the next successful run of each kind, up to a week for the restore test, and the alerts
// that read them saw no value.
func TestSchedulePrimesUntilItSucceeds(t *testing.T) {
	t.Parallel()
	start := time.Date(2026, 10, 18, 1, 59, 30, 0, time.UTC)
	clk := clock.NewFake(start)
	down := errors.New("connection refused")
	target := &primeTarget{fakeTarget: &fakeTarget{}, errs: []error{down, down}}
	waits := 0
	wait := func(_ context.Context, _ clock.Clock, at time.Time) bool {
		waits++
		if waits == 5 {
			return false
		}
		clk.Set(at)
		return true
	}
	schedule(context.Background(), target, clk, wait)
	// At the start, before the first tick and before the second tick, which succeeds.
	if want := []int{0, 0, 1}; !slices.Equal(target.before, want) {
		t.Errorf("Prime ran after %v ticks, want after %v: at the start and before each tick until it succeeded", target.before, want)
	}
	if target.ticks != 4 {
		t.Errorf("%d ticks, want 4: a failed read must not stop the scheduler", target.ticks)
	}
}

// primeDaemon builds the backup daemon of the default command for db as loadDaemon does, with
// the role bothy_backup for backup_runs and reader instead of the OTLP exporter.
func primeDaemon(t *testing.T, db *pgtest.DB, reader sdkmetric.Reader) *wire.BackupDaemon {
	t.Helper()
	ctx := testkit.Context(t)
	cfg, super, dir := db.Config(pg.RoleBackup), db.SuperConfig(), db.SecretsDir()
	schema, err := wire.ConfigSchema()
	if err != nil {
		t.Fatal(err)
	}
	values, err := config.Load(schema, config.Sources{Env: []string{
		"BOTHY_SERVER__PUBLIC_URL=https://chat.example.com",
		"BOTHY_SECRETS__DIR=" + dir,
		"BOTHY_DATABASE__HOST=" + cfg.Host,
		"BOTHY_DATABASE__PORT=" + strconv.Itoa(cfg.Port),
		"BOTHY_DATABASE__NAME=" + db.Name(),
		"BOTHY_DATABASE__USER=" + string(pg.RoleBackup),
		"BOTHY_DATABASE__PASSWORD_REF=file:" + dir + "/db_backup_password",
		"BOTHY_DATABASE__SSLMODE=disable",
		"BOTHY_MIGRATE__BOOTSTRAP_USER=" + super.User,
		"BOTHY_MIGRATE__BOOTSTRAP_PASSWORD_REF=file:" + dir + "/db_superuser_password",
		"BOTHY_STARTUP__DEPENDENCY_WAIT=" + testkit.PatienceSetting,
		"BOTHY_FILE__FS__ROOT=" + t.TempDir(),
		"BOTHY_TELEMETRY__ENABLED=false",
	}})
	if err != nil {
		t.Fatal(err)
	}
	telCfg, err := wire.TelemetryConfig(values)
	if err != nil {
		t.Fatal(err)
	}
	tel, err := otel.SetupMetrics(ctx, telCfg, otel.MetricsDeps{ServiceName: serviceName, ServiceVersion: "1.2.3", Reader: reader})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tel.Shutdown(context.WithoutCancel(ctx)) })
	d, err := wire.NewBackupDaemon(ctx, values, wire.BackupDaemonOptions{
		Log: slog.New(slog.NewTextHandler(io.Discard, nil)), ToolOutput: io.Discard, Telemetry: tel,
		ServerVersion: "1.2.3", DumpDir: t.TempDir(),
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(d.Close)
	return d
}

// lastSuccesses returns what reader holds of the two gauges of a last success, in Unix seconds:
// bothy.backup.last_success.timestamp by its bothy.backup.kind, and
// bothy.backup.restore_test.last_success.timestamp under the name restore_test.
func lastSuccesses(t *testing.T, reader *sdkmetric.ManualReader) map[string]float64 {
	t.Helper()
	var rm metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &rm); err != nil {
		t.Fatal(err)
	}
	got := map[string]float64{}
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name != "bothy.backup.last_success.timestamp" && m.Name != "bothy.backup.restore_test.last_success.timestamp" {
				continue
			}
			g, ok := m.Data.(metricdata.Gauge[float64])
			if !ok {
				t.Fatalf("%s: data %T, want a float64 gauge", m.Name, m.Data)
			}
			for _, dp := range g.DataPoints {
				kind := "restore_test"
				if v, ok := dp.Attributes.Value(attribute.Key("bothy.backup.kind")); ok {
					kind = v.AsString()
				}
				got[kind] = dp.Value
			}
		}
	}
	return got
}

// TestScheduleReportsTheLastSuccessesOnceBackupRunsAnswers: the scheduler loop of the default
// command with the daemon built from the configuration, PostgreSQL, the role bothy_backup and
// the metrics of the process. backup_runs holds a succeeded run of every kind, but cannot be read
// when the process starts (here the table is out of reach, as at the first start of a stack
// before bothy migrate has created it; a database that does not answer yet fails the same
// call). Nothing is reported then. One tick after the table answers, the gauges that the alerts
// BothyBackupStale and BothyRestoreTestStale read hold the last success of every kind, without
// a new run.
func TestScheduleReportsTheLastSuccessesOnceBackupRunsAnswers(t *testing.T) {
	db := pgtest.New(t)
	ctx := testkit.Context(t)
	super := db.SuperConn(t)
	succeeded := time.Date(2026, 10, 17, 2, 0, 0, 0, time.UTC)
	for id, kind := range map[string]string{
		"0192f4a0-7b1c-7000-8a00-000000000001": "pg_diff",
		"0192f4a0-7b1c-7000-8a00-000000000002": "logical",
		"0192f4a0-7b1c-7000-8a00-000000000003": "objects",
		"0192f4a0-7b1c-7000-8a00-000000000004": "restore_test",
	} {
		if _, err := super.Exec(ctx, "INSERT INTO backup_runs (id, kind, state, started_at, finished_at) VALUES ($1, $2, 'succeeded', $3, $3)", id, kind, succeeded); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := super.Exec(ctx, "ALTER TABLE backup_runs RENAME TO backup_runs_later"); err != nil {
		t.Fatal(err)
	}

	reader := sdkmetric.NewManualReader()
	d := primeDaemon(t, db, reader)
	// A Sunday noon: no schedule is due in the minutes the loop runs, so no run starts.
	clk := clock.NewFake(time.Date(2026, 10, 18, 12, 0, 30, 0, time.UTC))
	waits := 0
	wait := func(_ context.Context, _ clock.Clock, at time.Time) bool {
		waits++
		switch waits {
		case 1:
			// The start of the process is over and could read nothing.
			if got := lastSuccesses(t, reader); len(got) != 0 {
				t.Errorf("last successes while backup_runs cannot be read: %v, want none", got)
			}
			if _, err := super.Exec(ctx, "ALTER TABLE backup_runs_later RENAME TO backup_runs"); err != nil {
				t.Error(err)
				return false
			}
		case 2:
			return false
		}
		clk.Set(at)
		return true
	}
	schedule(ctx, daemonTarget{d}, clk, wait)

	at := float64(succeeded.Unix())
	want := map[string]float64{"physical": at, "logical": at, "objects": at, "restore_test": at}
	if got := lastSuccesses(t, reader); !maps.Equal(got, want) {
		t.Errorf("last successes one tick after backup_runs answers: %v, want %v", got, want)
	}
}
