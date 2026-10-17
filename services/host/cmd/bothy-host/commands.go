// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"io"
)

// command is one subcommand of the binary bothy-host.
//
// name is the word after "bothy-host" on the command line (docs/spec/02-architecture.md
// "Code layout"). summary is the one-line purpose shown in the usage text. run receives the
// arguments after the name and returns the exit code: 0 success, 1 failure, 2 usage error, 78 a
// refused start of run. Commands parse their flags with the standard library package flag.
type command struct {
	name, summary string
	run           func(ctx context.Context, args []string, stdout, stderr io.Writer) int
}

// commands is the table of registered subcommands (D-M2-42-2). Order does not matter: the usage
// text sorts by name.
var commands = []command{
	{name: "install", summary: "Install the host service, its users and the PTY helpers (administrator)", run: cmdInstall},
	{name: "session-user", summary: "Add or remove a session user: session-user add|remove <name> (administrator)", run: cmdSessionUser},
	{name: "restart", summary: "Restart the host service, also after a refused start (administrator)", run: cmdRestart},
	{name: "pair", summary: "Pair the host with a server: pair --server <url> --code <code> [--ca-file <pem>]", run: cmdPair},
	{name: "doctor", summary: "Run the self-checks HD1 to HD11 and print codes", run: cmdDoctor},
	{name: "run", summary: "Run the host service (started by the service manager)", run: cmdRun},
	{name: "pty-helper", summary: "Run the PTY helper of a session user (started by the service manager)", run: cmdPtyHelper},
}
