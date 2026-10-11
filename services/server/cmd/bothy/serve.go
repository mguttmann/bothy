// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/OWNER/bothy/services/server/internal/platform/buildinfo"
	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/platform/lifecycle"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

// serveUsage is the help of "bothy serve" (docs/spec/02-architektur.md "Befehle des Binarys",
// "Start", "Shutdown").
const serveUsage = `Usage: bothy serve [--role=api|worker|all] [--config FILE]

Starts the server: configuration and telemetry, secrets, the internal listener with /healthz
and /readyz (server.internal_listen_addr), the database within startup.dependency_wait, the
schema check, then for api and all the public listener (server.listen_addr). It never runs
migrations; an older schema ends the start with the hint to run "bothy migrate up".

SIGTERM or SIGINT start the graceful shutdown within shutdown.total_timeout; a second signal
ends the process at once. JSON log lines go to stdout, a start error to stderr.

--role ROLE    process role, overrides server.role (api, worker or all; default all)
--config FILE  configuration file, not empty (default: BOTHY_CONFIG_FILE, else none)

Exit codes: 0 graceful end, 1 failed start, fatal runtime error or shutdown over
shutdown.total_timeout, 2 usage or configuration error (also stop budgets above
shutdown.total_timeout), 3 forced end by a second signal.
`

// exitForced is the code of serve for the end by a second signal (02 "Befehle des Binarys").
const exitForced = lifecycle.ExitForced

// serveCommand runs "bothy serve" with the arguments after the command name. signals replaces
// os/signal in tests; nil listens for SIGTERM and SIGINT.
func serveCommand(ctx context.Context, args, env []string, stdout, stderr io.Writer, signals <-chan os.Signal) int {
	fs := flag.NewFlagSet("bothy serve", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	file := fs.String("config", "", "configuration file")
	role := fs.String("role", "", "process role")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			_, _ = io.WriteString(stdout, serveUsage)
			return exitOK
		}
		return serveUsageError(stderr, printable(err.Error()))
	}
	if fs.NArg() > 0 {
		return serveUsageError(stderr, fmt.Sprintf("unexpected argument %q", printable(fs.Arg(0))))
	}
	if *file == "" && flagGiven(fs, "config") {
		return serveUsageError(stderr, "--config needs a file path")
	}
	s, err := wire.ConfigSchema()
	if err != nil {
		return serveFail(stderr, err, exitUsage)
	}
	src := config.Sources{Env: env, File: *file}
	if flagGiven(fs, "role") {
		src.Flags = map[string]string{"server.role": *role}
	}
	values, err := config.Load(s, src)
	if err != nil {
		return serveFail(stderr, err, exitUsage)
	}
	runner, err := wire.Serve(ctx, values, wire.Role(values.String("server.role")), wire.Deps{
		Env: env, Stdout: stdout, Signals: signals, Version: buildinfo.Version(),
	})
	if err != nil {
		return serveFail(stderr, err, exitUsage)
	}
	code := runner.Run(ctx)
	// The log handlers keep no error text (signals.md "Logs"); stderr carries the cause, for
	// example the hint to run "bothy migrate up".
	if err := runner.Err(); err != nil && code != exitOK {
		_, _ = fmt.Fprintf(stderr, "bothy serve: %v\n", err)
	}
	return code
}

func serveFail(stderr io.Writer, err error, code int) int {
	_, _ = fmt.Fprintf(stderr, "bothy serve: %v\n", err)
	return code
}

// serveUsageError prints msg and the usage on stderr and returns the usage exit code.
func serveUsageError(stderr io.Writer, msg string) int {
	_, _ = fmt.Fprintf(stderr, "bothy serve: %s\n", msg)
	_, _ = io.WriteString(stderr, serveUsage)
	return exitUsage
}
