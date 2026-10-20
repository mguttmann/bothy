// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	"github.com/OWNER/bothy/services/server/internal/platform/buildinfo"
	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/platform/otel"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

// serviceName is the OpenTelemetry service.name of bothy-backupd (signals.md "Resource").
const serviceName = "bothy-backupd"

// logKeyError is the log attribute of an error message.
const logKeyError = "error"

// envConfigFile names the configuration file when --config is not given, as for the server
// binary (docs/operations/configuration.md).
const envConfigFile = "BOTHY_CONFIG_FILE"

// telemetryFlushTimeout bounds the export of the last metrics when a command ends.
const telemetryFlushTimeout = 10 * time.Second

// The mount points of the Compose service backup (infra/compose/compose.yaml). The image
// defines them, so they are constants and not configuration: pgBackRest's repo1, the scratch
// volume and the database volume, which bothy-backupd sees read-only to measure it. The restore
// test uses its own directory of the scratch volume (M7-03).
const (
	repoDir    = "/var/lib/pgbackrest"
	scratchDir = "/var/lib/bothy-backup"
	restoreDir = scratchDir + "/restore"
	pgdataDir  = "/var/lib/postgresql"
)

// daemon is a loaded backup service with its telemetry.
type daemon struct {
	d   *wire.BackupDaemon
	tel *otel.MeterTelemetry
	log *slog.Logger
}

// close flushes the metrics and closes the service.
func (d *daemon) close(ctx context.Context) {
	d.d.Close()
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), telemetryFlushTimeout)
	defer cancel()
	if err := d.tel.ForceFlush(ctx); err != nil {
		d.log.WarnContext(ctx, "bothy-backupd: flush the metrics", slog.String(logKeyError, err.Error()))
	}
	_ = d.tel.Shutdown(ctx)
}

// loadDaemon loads the configuration (file, else BOTHY_CONFIG_FILE, else none, plus the
// environment env) with the schema of the server, starts the metrics and builds the backup
// service. A configuration that does not load is exit code 2; another setup error exit code 1.
// It returns nil and the code on failure, the reason printed on stderr.
func loadDaemon(ctx context.Context, env []string, file string, out io.Writer, stderr io.Writer) (*daemon, int) {
	fail := func(code int, err error) (*daemon, int) {
		_, _ = fmt.Fprintf(stderr, "bothy-backupd: %v\n", err)
		return nil, code
	}
	if file == "" {
		file = lookupEnv(env, envConfigFile)
	}
	schema, err := wire.ConfigSchema()
	if err != nil {
		return fail(exitFailure, err)
	}
	values, err := config.Load(schema, config.Sources{Env: configEnv(env), File: file})
	if err != nil {
		return fail(exitUsage, err)
	}
	log := slog.New(slog.NewTextHandler(out, nil))
	telCfg, err := wire.TelemetryConfig(values)
	if err != nil {
		return fail(exitUsage, err)
	}
	tel, err := otel.SetupMetrics(ctx, telCfg, otel.MetricsDeps{ServiceName: serviceName, ServiceVersion: buildinfo.Version(), Env: env})
	if err != nil {
		return fail(exitFailure, err)
	}
	d, err := wire.NewBackupDaemon(ctx, values, wire.BackupDaemonOptions{
		Env: env, Log: log, ToolOutput: out, Telemetry: tel,
		ServerVersion: buildinfo.Version(),
		Volumes: map[string]string{
			"pgdata": pgdataDir, "pgbackrest-repo": repoDir, "backup-scratch": scratchDir,
		},
		DumpDir: scratchDir + "/logical", LocalResticRepo: repoDir + "/restic", RestoreDir: restoreDir,
	})
	if err != nil {
		_ = tel.Shutdown(ctx)
		var cfgErr *wire.BackupConfigError
		if errors.As(err, &cfgErr) {
			return fail(exitUsage, err)
		}
		return fail(exitFailure, err)
	}
	return &daemon{d: d, tel: tel, log: log}, exitOK
}

// imageVariables are the variables of the image bothy-postgres that only its entry point reads
// (services/server/images/bothy-postgres/README.md). They share the prefix BOTHY_ with the
// configuration keys, so they are left out of the configuration sources; any other unknown
// BOTHY_ variable still ends the start (R-OP-04).
var imageVariables = map[string]bool{
	"BOTHY_PGBACKREST_REPO_HOST": true, "BOTHY_PGBACKREST_PG_HOST": true,
	"BOTHY_PGBACKREST_REPO2": true, "BOTHY_PG_ARCHIVE_TIMEOUT": true,
}

// configEnv returns env without the variables of the image.
func configEnv(env []string) []string {
	out := make([]string, 0, len(env))
	for _, e := range env {
		name, _, _ := strings.Cut(e, "=")
		if !imageVariables[name] {
			out = append(out, e)
		}
	}
	return out
}

// lookupEnv returns the value of name in env ("NAME=value" entries), "" when it is absent.
func lookupEnv(env []string, name string) string {
	for i := len(env) - 1; i >= 0; i-- {
		if v, ok := strings.CutPrefix(env[i], name+"="); ok {
			return v
		}
	}
	return ""
}
