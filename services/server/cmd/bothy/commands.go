// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"io"
)

// command is one subcommand of the binary bothy.
//
// name is the word after "bothy" on the command line and must appear in the register of names
// (docs/spec/02-architecture.md section "Commands of the binary"). summary is the one-line purpose
// shown in the usage text. run receives the arguments after the name and returns the exit code:
// 0 success, 1 failure, 2 usage error, 3 and above only as documented in the command's own help.
type command struct {
	name, summary string
	run           func(ctx context.Context, args []string, stdout, stderr io.Writer) int
}

// commands is the table of registered subcommands. Order does not matter: the usage text sorts
// by name. It is empty until the tickets that build the commands add them: M1-07 adds config,
// M1-11 adds serve, and later tickets the rest of the register.
var commands = []command{}
