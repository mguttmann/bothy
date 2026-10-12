// SPDX-License-Identifier: Apache-2.0

// Command bothy is the single server binary. Every task runs as a subcommand
// ("bothy <command> [arguments]"); the register of command names is
// docs/spec/02-architecture.md section "Commands of the binary", and a command is only added here
// when its ticket builds it.
//
// Exit codes: 0 success, 1 failure, 2 usage error. Codes from 3 upwards are specific to a
// command and documented in its help, so orchestrators and scripts can tell "called wrongly"
// from "failed at run time" without parsing text.
//
// Example:
//
//	bothy --help
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
)

// Exit codes shared by all commands (docs/spec/02-architecture.md "Commands of the binary").
const (
	exitOK      = 0
	exitFailure = 1
	exitUsage   = 2
)

func main() {
	// Signal handling belongs to the commands that need it (serve, M1-11); a plain background
	// context keeps short commands free of it.
	os.Exit(run(context.Background(), os.Args[1:], os.Environ(), os.Stdout, os.Stderr))
}

// run executes the command line args (without the program name) against the registered
// commands and returns the process exit code. env holds the process environment as os.Environ
// returns it; main reads it once here, and no other package reads the environment
// (docs/spec/02-architecture.md "Configuration loading" item 5).
func run(ctx context.Context, args, env []string, stdout, stderr io.Writer) int {
	return dispatch(ctx, commands, args, env, stdout, stderr)
}

// dispatch selects the command named by args[0] from table and runs it with the remaining
// arguments and env, returning its exit code. Without arguments it prints the usage to stderr and returns
// 2; for help, -h or --help it prints the usage to stdout and returns 0; for an unknown name it
// prints an error line and the usage to stderr and returns 2. The table is a parameter so tests
// can pass their own.
func dispatch(ctx context.Context, table []command, args, env []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		writeUsage(stderr, table)
		return exitUsage
	}
	name := args[0]
	if isHelpArg(name) {
		writeUsage(stdout, table)
		return exitOK
	}
	for _, c := range table {
		if c.name == name {
			return c.run(ctx, args[1:], env, stdout, stderr)
		}
	}
	// A failed write to stderr cannot be reported anywhere; the exit code still says it.
	_, _ = fmt.Fprintf(stderr, "bothy: unknown command %q\n", name)
	writeUsage(stderr, table)
	return exitUsage
}

// isHelpArg reports whether a command or subcommand word asks for help: "help", "-h" or "--help".
// bothy, bothy config and bothy secrets accept the same three words, so they are spelled here once.
func isHelpArg(arg string) bool {
	return arg == "help" || arg == "-h" || arg == "--help"
}

// writeUsage prints the usage text with the commands of table sorted by name. It sorts a copy
// because the table is shared package state. Write errors are ignored: the usage goes to the
// terminal or a pipe, and the exit code already tells the caller what happened.
func writeUsage(w io.Writer, table []command) {
	var b strings.Builder
	b.WriteString("Usage: bothy <command> [arguments]\n\nCommands:\n")
	if len(table) == 0 {
		b.WriteString("  (none in this build)\n")
	}
	sorted := slices.Clone(table)
	slices.SortFunc(sorted, func(a, b command) int { return strings.Compare(a.name, b.name) })
	width := 0
	for _, c := range sorted {
		width = max(width, len(c.name))
	}
	for _, c := range sorted {
		fmt.Fprintf(&b, "  %-*s  %s\n", width, c.name, c.summary)
	}
	_, _ = io.WriteString(w, b.String())
}
