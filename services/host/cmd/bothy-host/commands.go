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
// arguments after the name and returns the exit code: 0 success, 1 failure, 2 usage error.
// Commands parse their flags with the standard library package flag.
type command struct {
	name, summary string
	run           func(ctx context.Context, args []string, stdout, stderr io.Writer) int
}

// commands is the table of registered subcommands. It is empty until M2-42 registers the first
// command; the binary exists already so that release-build has a main package
// (docs/DECISIONS.md D-M2-38-6). Order does not matter: the usage text sorts by name.
var commands []command
