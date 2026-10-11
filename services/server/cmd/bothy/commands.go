// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"io"

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
// by name. Each ticket that builds a command of the register adds it here (M1-11 adds serve).
var commands = []command{
	{
		name:    "config",
		summary: "Check or print the configuration without starting the server",
		run: func(_ context.Context, args, env []string, stdout, stderr io.Writer) int {
			return configCommand(args, env, wire.ConfigSchema, stdout, stderr)
		},
	},
}
