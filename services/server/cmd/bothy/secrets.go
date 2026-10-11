// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/OWNER/bothy/services/server/internal/platform/secrets/stack"
)

// secretsUsage is the help of "bothy secrets" (docs/spec/02-architektur.md "Befehle des
// Binarys", docs/spec/16-betrieb.md 5.3 "Geheimnisse im Compose-Stack" and 5.4 "Hinterlegung").
//
// #nosec G101 -- help text; the name holds "secrets" because it is the help of bothy secrets.
const secretsUsage = `Usage: bothy secrets init --target DIR [--source DIR] [--owner SERVICE=UID:GID ...]
                          [--adopt-source]
       bothy secrets export --from DIR --age-recipient KEY [--age-recipient KEY ...]
                            [--output FILE|-] [--armor]

init    prepares the secret volume of the Compose stack: DIR/_store holds the one copy of
        every secret, DIR/<service> the copies that service mounts. It generates missing
        secrets, adopts the files of --source, never replaces a generated secret unless
        --adopt-source is given, and changes nothing on a second run with the same inputs.
        Per secret it prints "<status> <name> <fingerprint>", never a value.
export  writes the files of --from as a tar bundle encrypted with age to every recipient
        (escrow); plaintext never reaches a disk.

Both read only their flags: no configuration file, no environment, no database.

init flags:
--target DIR             volume root, not empty (required)
--source DIR             directory of supplied files or of a decrypted bundle, not empty
--owner SERVICE=UID:GID  owner of the copies of SERVICE (directory 0500, files 0400);
                         once per service, required for server and migrate
--adopt-source           let a differing source file replace a generated secret

export flags:
--from DIR               the _store directory to export, not empty (required)
--age-recipient KEY      age X25519 public key (age1...); repeatable, at least one
--output FILE            new file with mode 0600, never overwritten; - is stdout (default -)
--armor                  ASCII armor, needed when stdout is a terminal

Exit codes: 0 ok, 1 runtime failure or conflict, 2 usage error.
`

// The subcommands of bothy secrets.
const (
	subcommandInit   = "init"
	subcommandExport = "export"
)

// secretsOptions holds the flags of secrets init and export.
type secretsOptions struct {
	target, source, from, output string
	owners                       ownerFlag
	recipients                   listFlag
	adoptSource, armor           bool
}

// secretsFlags returns the flag set of the subcommand sub ("init" or "export") and the options
// its flags fill. Parse errors are returned, not printed: secretsCommand prints them with the
// usage.
func secretsFlags(sub string) (*flag.FlagSet, *secretsOptions) {
	o := &secretsOptions{owners: ownerFlag{owners: map[string]stack.Owner{}}}
	fs := flag.NewFlagSet("bothy secrets "+sub, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	switch sub {
	case subcommandInit:
		fs.StringVar(&o.target, "target", "", "volume root")
		fs.StringVar(&o.source, "source", "", "directory of supplied files")
		fs.Var(&o.owners, "owner", "owner of the copies of a service")
		fs.BoolVar(&o.adoptSource, "adopt-source", false, "let the source replace generated secrets")
	case subcommandExport:
		fs.StringVar(&o.from, "from", "", "_store directory")
		fs.Var(&o.recipients, "age-recipient", "age X25519 public key")
		fs.StringVar(&o.output, "output", "-", "output file or -")
		fs.BoolVar(&o.armor, "armor", false, "ASCII armor")
	}
	return fs, o
}

// secretsCommand runs "bothy secrets <init|export> [flags]" with the arguments after "secrets"
// (docs/spec/16-betrieb.md 5.3 and 5.4). It reads only its flags and the directories they name,
// never the environment or a configuration file, because the secrets-init container has neither.
//
// For help, -h and --help it prints secretsUsage on stdout and returns 0. A missing or unknown
// subcommand, a flag error, an unexpected argument or a missing required flag prints "bothy
// secrets[ <sub>]: <message>" and the usage on stderr and returns 2, like bare "bothy" and bare
// "bothy config": a call that does nothing must not pass as successful. No message repeats text
// of the command line; it names the position of an argument instead, because a key pasted into
// the wrong place must not reach the terminal or a log (docs/DECISIONS.md M1-09/21). An error of the stack
// package prints "bothy secrets <sub>: <error>" on stderr and returns 2 when it wraps
// stack.ErrUsage, else 1. init prints its report on stdout; export writes the bundle to --output
// or stdout.
func secretsCommand(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		return secretsUsageError(stderr, "bothy secrets", "missing subcommand")
	}
	sub := args[0]
	if isHelpArg(sub) {
		_, _ = io.WriteString(stdout, secretsUsage)
		return exitOK
	}
	switch sub {
	case subcommandInit, subcommandExport:
	default:
		return secretsUsageError(stderr, "bothy secrets", "unknown subcommand; want init or export")
	}

	prefix := "bothy secrets " + sub
	fs, o := secretsFlags(sub)
	if err := fs.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			_, _ = io.WriteString(stdout, secretsUsage)
			return exitOK
		}
		return secretsUsageError(stderr, prefix, parseErrorMessage(sub, args, fs, o, err))
	}
	if fs.NArg() > 0 {
		// Named by its position, never quoted: an age identity (AGE-SECRET-KEY-1...) pasted
		// without --age-recipient must not reach the terminal or a log, like a rejected
		// recipient in stack.Export. args[0] is the subcommand, so the index of the first
		// unexpected argument in args is its position after the subcommand.
		pos := len(args) - fs.NArg()
		return secretsUsageError(stderr, prefix, fmt.Sprintf(
			"unexpected argument %d after %s, not shown because it may be a key; every value needs its flag", pos, sub))
	}

	if msg := missingFlag(sub, fs, o); msg != "" {
		return secretsUsageError(stderr, prefix, msg)
	}
	if err := runSecrets(ctx, sub, o, stdout); err != nil {
		_, _ = fmt.Fprintf(stderr, "%s: %v\n", prefix, err)
		if errors.Is(err, stack.ErrUsage) {
			return exitUsage
		}
		return exitFailure
	}
	return exitOK
}

// missingFlag returns the usage message for the first required flag of sub that is missing or
// empty, or "" when all are set. An empty value usually comes from an unset variable in a script,
// so it is refused instead of being read as "no value" or as a default the script did not name.
func missingFlag(sub string, fs *flag.FlagSet, o *secretsOptions) string {
	if sub == subcommandInit {
		// An empty path from an unset variable must not silently mean "no source": a restore
		// with --adopt-source would generate new secrets instead of adopting the bundle.
		switch {
		case o.target == "":
			return "--target needs a directory path"
		case o.source == "" && flagGiven(fs, "source"):
			return "--source needs a directory path"
		case o.adoptSource && !flagGiven(fs, "source"):
			// Without a source the flag does nothing, and a restore would generate new secrets
			// instead of adopting the bundle.
			return "--adopt-source needs --source"
		}
		return ""
	}
	switch {
	case o.from == "":
		return "--from needs the _store directory"
	case o.output == "":
		// stack.Export reads "" as stdout; an unset variable must not turn a file into it.
		return "--output needs a file path or -"
	}
	return ""
}

// runSecrets runs init or export with the checked options: init prints its report on stdout,
// export writes the bundle to --output or stdout.
func runSecrets(ctx context.Context, sub string, o *secretsOptions, stdout io.Writer) error {
	if sub == subcommandInit {
		return secretsInit(ctx, o, stdout)
	}
	return stack.Export(ctx, stack.ExportOptions{
		From: o.from, Recipients: o.recipients, Output: o.output, Armor: o.armor, Stdout: stdout,
	})
}

// secretsInit runs stack.Init with the options of the flags and writes the report to stdout. A
// report that cannot be written is an error: the operator would not see the fingerprints or
// the reminder to export.
func secretsInit(ctx context.Context, o *secretsOptions, stdout io.Writer) error {
	r, err := stack.Init(ctx, stack.InitOptions{
		Target: o.target, Source: o.source, Owners: o.owners.owners, AdoptSource: o.adoptSource,
	})
	if err != nil {
		return err
	}
	if _, err := r.WriteTo(stdout); err != nil {
		return fmt.Errorf("write the report: %w", err)
	}
	return nil
}

// secretsUsageError prints "<prefix>: <msg>" and the usage on stderr and returns the usage exit
// code.
func secretsUsageError(stderr io.Writer, prefix, msg string) int {
	_, _ = fmt.Fprintf(stderr, "%s: %s\n", prefix, msg)
	_, _ = io.WriteString(stderr, secretsUsage)
	return exitUsage
}

// parseErrorMessage turns a parse error of fs into a message that names the position of the
// offending argument after the subcommand, never its text: the flag package quotes flag names
// and values, and an age identity given as one must not reach the terminal or a log. An invalid
// --owner keeps the reason ParseOwner or ownerFlag.Set gave, which never repeats the value.
func parseErrorMessage(sub string, args []string, fs *flag.FlagSet, o *secretsOptions, err error) string {
	// Parse consumes the arguments it has read; what is left follows the offending one, except
	// for bad flag syntax, which the flag package reports before consuming the argument.
	pos := len(args) - 1 - len(fs.Args())
	msg := err.Error()
	if strings.HasPrefix(msg, "bad flag syntax") {
		pos++
	}
	switch {
	case o.owners.failed != nil:
		return fmt.Sprintf("invalid --owner at argument %d: %v", pos, o.owners.failed)
	case strings.HasPrefix(msg, "flag needs an argument"):
		return fmt.Sprintf("argument %d needs a value", pos)
	}
	return fmt.Sprintf("argument %d is not a flag of %s or has an invalid value; it is not shown because it may be a key", pos, sub)
}

// ownerFlag collects the repeatable flag --owner SERVICE=UID:GID of secrets init. A service given
// twice is a usage error: which of two owners was meant is not for the command to guess. failed
// keeps the reason of the first rejected value, so the command can report it without the value.
type ownerFlag struct {
	owners map[string]stack.Owner
	failed error
}

// String returns the owners as "service=uid:gid" sorted by service, joined by ",".
func (o *ownerFlag) String() string {
	if o == nil {
		return ""
	}
	parts := make([]string, 0, len(o.owners))
	for _, service := range slices.Sorted(maps.Keys(o.owners)) {
		parts = append(parts, service+"="+strconv.Itoa(o.owners[service].UID)+":"+strconv.Itoa(o.owners[service].GID))
	}
	return strings.Join(parts, ",")
}

// Set parses one value with stack.ParseOwner and adds it. The service name in the duplicate
// message has passed ParseOwner, so it is a service-shaped name and safe to show.
func (o *ownerFlag) Set(s string) error {
	service, owner, err := stack.ParseOwner(s)
	if err == nil {
		if _, dup := o.owners[service]; dup {
			err = fmt.Errorf("service %s has more than one --owner", service)
		}
	}
	if err != nil {
		o.failed = err
		return err
	}
	o.owners[service] = owner
	return nil
}

// listFlag collects a repeatable string flag (--age-recipient).
type listFlag []string

// String returns the values joined by ",".
func (l *listFlag) String() string {
	if l == nil {
		return ""
	}
	return strings.Join(*l, ",")
}

// Set adds one value; stack.Export checks it.
func (l *listFlag) Set(s string) error {
	*l = append(*l, s)
	return nil
}
