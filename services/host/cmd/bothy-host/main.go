// SPDX-License-Identifier: Apache-2.0

// Command bothy-host is the host service of the terminal (docs/spec/24-terminal.md 2.2). Every
// task runs as a subcommand ("bothy-host <command> [arguments]"); the register of command names
// is docs/spec/02-architecture.md "Code layout", and a command is only added here when its
// ticket builds it (M2-42 registers install, pair, doctor, run, restart, session-user and
// pty-helper).
//
// Exit codes: 0 success, 1 failure (a failed check of doctor, a runtime failure), 2 usage or
// environment error (unknown flag, missing administrator rights, unreadable configuration), 78 a
// refused start of run (EX_CONFIG, after which no service manager restarts it).
//
// Example:
//
//	bothy-host --help
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"text/tabwriter"
)

// Exit codes shared by all commands.
const (
	exitOK    = 0
	exitUsage = 2
)

func main() {
	os.Exit(run(context.Background(), os.Args[1:], os.Stdout, os.Stderr))
}

// run executes the command line args (without the program name) against the registered
// commands and returns the process exit code.
func run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	return dispatch(ctx, commands, args, stdout, stderr)
}

// dispatch selects the command named by args[0] from table and runs it with the remaining
// arguments, returning its exit code. Without arguments it prints the usage to stderr and
// returns 2; for help, -h or --help it prints the usage to stdout and returns 0; for an unknown
// name it prints an error line and the usage to stderr and returns 2. The table is a parameter
// so tests can pass their own.
func dispatch(ctx context.Context, table []command, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		writeUsage(stderr, table)
		return exitUsage
	}
	name := args[0]
	if name == "help" || name == "-h" || name == "--help" {
		writeUsage(stdout, table)
		return exitOK
	}
	for _, c := range table {
		if c.name == name {
			return c.run(ctx, args[1:], stdout, stderr)
		}
	}
	// A failed write to stderr cannot be reported anywhere; the exit code still says it.
	_, _ = fmt.Fprintf(stderr, "bothy-host: unknown command %q\n", name)
	writeUsage(stderr, table)
	return exitUsage
}

// writeUsage prints the usage text with the commands of table sorted by name. It sorts a copy
// because the table is shared package state. Write errors are ignored: the exit code already
// tells the caller what happened.
func writeUsage(w io.Writer, table []command) {
	var b strings.Builder
	b.WriteString("Usage: bothy-host <command> [arguments]\n\nCommands:\n")
	sorted := slices.Clone(table)
	slices.SortFunc(sorted, func(a, b command) int { return strings.Compare(a.name, b.name) })
	// tabwriter pads the names to the longest one plus two spaces before the summaries.
	tw := tabwriter.NewWriter(&b, 0, 0, 2, ' ', 0)
	for _, c := range sorted {
		_, _ = fmt.Fprintf(tw, "  %s\t%s\n", c.name, c.summary)
	}
	_ = tw.Flush()
	_, _ = io.WriteString(w, b.String())
}
