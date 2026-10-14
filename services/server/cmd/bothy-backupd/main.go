// SPDX-License-Identifier: Apache-2.0

// Command bothy-backupd runs in the container backup of the image bothy-postgres
// (docs/spec/16-betrieb.md 2 and 8.2, brief M7-01): it supervises the pgBackRest TLS server
// of the repository host and creates the stanza on its first start. Schedules, the logical and
// the object backup and the restore test come with later tickets (M7-02, M7-03); their use cases
// live in the module status, not here.
//
// It is a second program of the server module and not a command of the server binary, because the server
// binary with its API role has no business in the database image (docs/DECISIONS.md
// D-M7-01-3). Its version is the server version (internal/platform/buildinfo), so one release
// builds and versions both images from one commit.
//
// Commands:
//
//	bothy-backupd [--pgbackrest PATH] [--stanza NAME] [--retry DURATION]
//	bothy-backupd healthcheck [--addr HOST:PORT] [--server-name NAME] [--ca FILE] [--cert FILE] [--key FILE] [--timeout DURATION]
//	bothy-backupd version
//
// Exit codes: 0 success, 1 failure, 2 usage error.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/OWNER/bothy/services/server/internal/platform/buildinfo"
)

// Exit codes, the same as those of the server binary (docs/spec/02-architektur.md "Befehle des Binarys").
const (
	exitOK      = 0
	exitFailure = 1
	exitUsage   = 2
)

// usage is the help of bothy-backupd.
const usage = `Usage: bothy-backupd [--pgbackrest PATH] [--stanza NAME] [--retry DURATION]
       bothy-backupd healthcheck [--addr HOST:PORT] [--server-name NAME] [--ca FILE]
                                 [--cert FILE] [--key FILE] [--timeout DURATION]
       bothy-backupd version

Without a command it runs the repository host of the container backup: it starts
"pgbackrest server" (the TLS server that postgres pushes WAL to), runs "pgbackrest
stanza-create" until it succeeds, and stops the server on SIGTERM or SIGINT. pgBackRest reads
/etc/pgbackrest/pgbackrest.conf, which the image's entry point writes.

--pgbackrest PATH   pgBackRest binary (default pgbackrest from PATH)
--stanza NAME       stanza of the instance (default: the stanza of the image)
--retry DURATION    wait between two attempts of stanza-create (default 5s)

healthcheck  succeeds when the TLS server completes a handshake with the client certificate
             and presents a certificate of the stack's CA for --server-name.
version      prints the version (the server version).

Exit codes: 0 ok, 1 failure, 2 usage error.
`

func main() {
	// Signals reach the default command through its own channel (run); the short commands need
	// none.
	os.Exit(run(context.Background(), os.Args[1:], os.Stdout, os.Stderr))
}

// run executes the command line args (without the program name) and returns the exit code.
// Signals for the default command come from signal.Notify here, so tests call serve directly
// with their own channel.
func run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 {
		switch args[0] {
		case "healthcheck":
			return healthcheckCommand(ctx, args[1:], stderr)
		case "version":
			if len(args) > 1 {
				return usageError(stderr, "version takes no arguments")
			}
			_, _ = fmt.Fprintln(stdout, buildinfo.Version())
			return exitOK
		case "help", "-h", "--help":
			_, _ = io.WriteString(stdout, usage)
			return exitOK
		}
	}
	opts, code := parseServe(args, stdout, stderr)
	if code >= 0 {
		return code
	}
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGTERM, syscall.SIGINT)
	defer signal.Stop(signals)
	return serve(ctx, opts, signals, stderr)
}

// parseServe parses the flags of the default command. It returns the options and -1, or an
// exit code when the command line asks for help or is wrong.
func parseServe(args []string, stdout, stderr io.Writer) (serveOptions, int) {
	fs := flag.NewFlagSet("bothy-backupd", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var o serveOptions
	fs.StringVar(&o.pgbackrest, "pgbackrest", "pgbackrest", "pgBackRest binary")
	fs.StringVar(&o.stanza, "stanza", defaultStanza, "stanza")
	fs.DurationVar(&o.retry, "retry", defaultRetry, "wait between attempts")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			_, _ = io.WriteString(stdout, usage)
			return o, exitOK
		}
		return o, usageError(stderr, err.Error())
	}
	switch {
	case fs.NArg() > 0:
		return o, usageError(stderr, fmt.Sprintf("unknown command %q", fs.Arg(0)))
	case o.pgbackrest == "":
		return o, usageError(stderr, "--pgbackrest needs a path")
	case o.stanza == "":
		return o, usageError(stderr, "--stanza needs a name")
	case o.retry <= 0:
		return o, usageError(stderr, "--retry must be positive")
	}
	return o, -1
}

// usageError prints "bothy-backupd: <msg>" and the usage on stderr and returns exitUsage.
func usageError(stderr io.Writer, msg string) int {
	_, _ = fmt.Fprintf(stderr, "bothy-backupd: %s\n", msg)
	_, _ = io.WriteString(stderr, usage)
	return exitUsage
}
