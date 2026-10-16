// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"io"
	"time"

	"github.com/OWNER/bothy/services/server/internal/wire"
)

// command is one subcommand of the binary bothy.
//
// name is the word after "bothy" on the command line and must appear in the register of names
// (docs/spec/02-architecture.md section "Commands of the binary"). summary is the one-line purpose
// shown in the usage text. run receives the arguments after the name and the process environment
// ("NAME=value" entries, read once by main) and returns the exit code: 0 success, 1 failure,
// 2 usage error, 3 and above only as documented in the command's own help.
type command struct {
	name, summary string
	run           func(ctx context.Context, args, env []string, stdout, stderr io.Writer) int
}

// commands is the table of registered subcommands. Order does not matter: the usage text sorts
// by name. Each ticket that builds a command of the register adds it here.
var commands = []command{
	{
		name:    "serve",
		summary: "Start the server in the role api, worker or all",
		run: func(ctx context.Context, args, env []string, stdout, stderr io.Writer) int {
			return serveCommand(ctx, args, env, stdout, stderr, nil)
		},
	},
	{
		name:    "healthcheck",
		summary: "Probe a loopback health endpoint for a container without shell",
		// The probe needs no configuration, secrets or database (brief M1-11 D5).
		run: func(ctx context.Context, args, _ []string, stdout, stderr io.Writer) int {
			return healthcheckCommand(ctx, args, stdout, stderr)
		},
	},
	{
		name:    "config",
		summary: "Check or print the configuration without starting the server",
		run: func(ctx context.Context, args, env []string, stdout, stderr io.Writer) int {
			return configCommand(ctx, args, env, wire.ConfigSchema, stdout, stderr)
		},
	},
	{
		name:    "migrate",
		summary: "Apply, list or validate the database migrations",
		run: func(ctx context.Context, args, env []string, stdout, stderr io.Writer) int {
			return migrateCommand(ctx, args, defaultMigrateEnv(env, stdout, stderr))
		},
	},
	{
		name:    "secrets",
		summary: "Create the secrets of the Compose stack or export them encrypted",
		// init and export read only their flags: the secrets-init container has no BOTHY_
		// environment and no configuration file (brief M1-09), so env is not passed on.
		run: func(ctx context.Context, args, _ []string, stdout, stderr io.Writer) int {
			return secretsCommand(ctx, args, stdout, stderr)
		},
	},
	{
		name:    "setup",
		summary: "Create the first owner or a recovery code on the host",
		run: func(ctx context.Context, args, env []string, stdout, stderr io.Writer) int {
			return setupCommand(ctx, args, env, stdout, stderr, defaultSetupDeps())
		},
	},
	{
		name:    "telemetry",
		summary: "Map a principal id to its telemetry pseudonym for the owner",
		run: func(ctx context.Context, args, env []string, stdout, stderr io.Writer) int {
			return telemetryCommand(ctx, args, env, time.Now, stdout, stderr)
		},
	},
}
