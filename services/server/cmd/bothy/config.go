// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/platform/secrets"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

// configUsage is the help of "bothy config" (docs/spec/02-architecture.md "Commands of the binary",
// "Configuration loading" item 7).
const configUsage = `Usage: bothy config check [--secrets] [--config FILE] [--role ROLE]
       bothy config print [--redacted] [--config FILE] [--role ROLE]

check  loads the configuration as the server start does, without starting, and prints
       "configuration ok" or every problem on stderr. Every secret reference that uses env:
       adds a warning on stderr, because file: is the recommended form (16-operations.md 5.2).
print  prints the effective value and the source of every key, sorted by name. Secret
       references are shown as references; nothing is resolved. The output is always
       redacted; --redacted only states it, and --redacted=false is a usage error.

--secrets      check only: also resolve every secret reference (*_ref key) through the
               secret provider and print "secret <key>: ok" or "secret <key>: <code>" per
               key, never a value; codes are invalid_ref, unsupported_scheme, not_found,
               insecure_file and read_failed. The reason of a failure goes to stderr.
               "configuration ok" follows only when every reference resolved.
--config FILE  configuration file, not empty (default: BOTHY_CONFIG_FILE, else none)
--role ROLE    process role (api, worker or all) as bothy serve --role sets it

Exit codes: 0 ok, 1 the configuration has problems or a secret cannot be read, 2 usage error.
`

// The subcommands of bothy config.
const (
	subcommandCheck = "check"
	subcommandPrint = "print"
)

// configCommand runs "bothy config <check|print> [flags]" with the arguments after "config".
// env is the process environment and schema builds the configuration schema; both are parameters
// so that tests pass their own. ctx bounds the reads of check --secrets. check warns on stderr
// about every env: reference, with or without --secrets. It returns 0 when the configuration
// loads (and with --secrets every reference resolves), 1 when it has problems (each printed on
// stderr), a secret cannot be read or the schema is invalid, and 2 for a usage error.
func configCommand(ctx context.Context, args, env []string, schema func() (*config.Schema, error), stdout, stderr io.Writer) int {
	if len(args) == 0 {
		return configUsageError(stderr, "")
	}
	sub := args[0]
	if isHelpArg(sub) {
		_, _ = io.WriteString(stdout, configUsage)
		return exitOK
	}
	switch sub {
	case subcommandCheck, subcommandPrint:
	default:
		return configUsageError(stderr, fmt.Sprintf("unknown subcommand %q", sub))
	}

	o, err := parseConfigFlags(sub, args[1:])
	if errors.Is(err, flag.ErrHelp) {
		_, _ = io.WriteString(stdout, configUsage)
		return exitOK
	}
	if err != nil {
		return configUsageError(stderr, err.Error())
	}

	s, values, ok := loadConfig(env, schema, o, stderr)
	if !ok {
		return exitFailure
	}
	if sub == subcommandPrint {
		writeConfig(stdout, s, values)
		return exitOK
	}
	warnEnvRefs(s, values, stderr)
	if o.resolve && !checkSecretRefs(ctx, s, values, env, stdout, stderr) {
		return exitFailure
	}
	_, _ = io.WriteString(stdout, "configuration ok\n")
	return exitOK
}

// configOptions holds the flags of config check and print.
type configOptions struct {
	file, role string
	roleGiven  bool // --role was set, also to an empty value
	resolve    bool // check --secrets
}

// parseConfigFlags parses and checks the flags of config check or print (sub): no argument
// besides the flags, print stays redacted, and --config names a file. It returns flag.ErrHelp for
// -h and --help; the message of any other error is the line configUsageError prints.
func parseConfigFlags(sub string, args []string) (configOptions, error) {
	fs := flag.NewFlagSet("bothy config "+sub, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	file := fs.String("config", "", "configuration file")
	role := fs.String("role", "", "process role")
	redacted, resolve := true, false
	if sub == subcommandPrint {
		// Accepted so that scripts can state it; print never shows more than this.
		fs.BoolVar(&redacted, "redacted", true, "redacted output")
	} else {
		// Only check reads secrets; print shows references and never resolves them.
		fs.BoolVar(&resolve, "secrets", false, "resolve every secret reference")
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return configOptions{}, err
		}
		// The flag package puts the flag name into the message unquoted; printable keeps a name
		// with a line break or an escape sequence on one line (docs/DECISIONS.md M1-07/26).
		return configOptions{}, errors.New(printable(err.Error()))
	}
	if fs.NArg() > 0 {
		return configOptions{}, fmt.Errorf("unexpected argument %q", fs.Arg(0))
	}
	if !redacted {
		return configOptions{}, errors.New("the output of print is always redacted; --redacted=false is not available")
	}
	if *file == "" && flagGiven(fs, "config") {
		// An empty path would silently fall back to BOTHY_CONFIG_FILE; a script with an unset
		// variable must not get another file than it named.
		return configOptions{}, errors.New("--config needs a file path")
	}
	return configOptions{file: *file, role: *role, roleGiven: flagGiven(fs, "role"), resolve: resolve}, nil
}

// loadConfig builds the schema and loads the configuration as the server start does, from env,
// the --config file and --role. On failure it prints the error on stderr and reports false: a
// configuration that does not load is exit code 1, not a usage error.
func loadConfig(env []string, schema func() (*config.Schema, error), o configOptions, stderr io.Writer) (*config.Schema, *config.Values, bool) {
	s, err := schema()
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "bothy config: %v\n", err)
		return nil, nil, false
	}
	src := config.Sources{Env: env, File: o.file}
	if o.roleGiven {
		src.Flags = map[string]string{"server.role": o.role}
	}
	values, err := config.Load(s, src)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return nil, nil, false
	}
	return s, values, true
}

// warnEnvRefs prints one warning on stderr for every secret_ref key of s, in name order, whose
// effective reference uses env:. Brief D2 and 16 5.2 allow env: but recommend file:; the warning
// is about the configuration, so it needs no read and comes with or without --secrets.
func warnEnvRefs(s *config.Schema, values *config.Values, stderr io.Writer) {
	for _, k := range s.Keys() {
		if k.Kind == config.KindSecretRef && strings.HasPrefix(values.String(k.Name), "env:") {
			_, _ = fmt.Fprintf(stderr, "warning: %s uses an env: reference; prefer file: (16-operations.md 5.2)\n", k.Name)
		}
	}
}

// checkSecretRefs resolves the reference of every secret_ref key of s through the provider of
// the process, in name order, and reports whether all of them resolved. Per key it prints
// "secret <key>: ok" or "secret <key>: <code>" (secrets.ErrorCode) on stdout and the error on
// stderr; the error names the reference, never the value (SEC-SEC-2). The values that were read
// are dropped at once.
func checkSecretRefs(ctx context.Context, s *config.Schema, values *config.Values, env []string, stdout, stderr io.Writer) bool {
	p, err := wire.NewSecretsProvider(values, env, slog.New(slog.NewTextHandler(stderr, nil)))
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "bothy config: %v\n", err)
		return false
	}
	ok := true
	for _, k := range s.Keys() {
		if k.Kind != config.KindSecretRef {
			continue
		}
		if _, _, err := p.Resolve(ctx, values.String(k.Name)); err != nil {
			ok = false
			_, _ = fmt.Fprintf(stdout, "secret %s: %s\n", k.Name, secrets.ErrorCode(err))
			_, _ = fmt.Fprintf(stderr, "config: %s: %v\n", k.Name, err)
			continue
		}
		_, _ = fmt.Fprintf(stdout, "secret %s: ok\n", k.Name)
	}
	return ok
}

// flagGiven reports whether the command line set the flag, also to an empty value.
func flagGiven(fs *flag.FlagSet, name string) bool {
	given := false
	fs.Visit(func(f *flag.Flag) { given = given || f.Name == name })
	return given
}

// configUsageError prints msg (when set) and the usage on stderr and returns the usage exit code.
func configUsageError(stderr io.Writer, msg string) int {
	if msg != "" {
		_, _ = fmt.Fprintf(stderr, "bothy config: %s\n", msg)
	}
	_, _ = io.WriteString(stderr, configUsage)
	return exitUsage
}

// writeConfig prints the header line with the file and one line per key:
// "<key> = <canonical text>  # <source>". Secret references are printed as they are; this
// command never resolves them.
func writeConfig(w io.Writer, s *config.Schema, v *config.Values) {
	var b strings.Builder
	file := "none"
	if v.File() != "" {
		file = printable(v.File())
	}
	b.WriteString("# configuration file: " + file + "\n")
	for _, k := range s.Keys() {
		fmt.Fprintf(&b, "%s = %s  # %s\n", k.Name, printable(v.Text(k.Name)), printable(v.Source(k.Name).String()))
	}
	_, _ = io.WriteString(w, b.String())
}

// printable returns text as it is, or Go-quoted when it would not read back as one unambiguous
// line: with invalid UTF-8 or a character that is not printable (a newline would forge a line of
// its own), with leading or trailing white space, or starting with a quote.
func printable(text string) string {
	if !utf8.ValidString(text) || strings.TrimSpace(text) != text || strings.HasPrefix(text, `"`) ||
		strings.IndexFunc(text, func(r rune) bool { return !unicode.IsPrint(r) }) >= 0 {
		return strconv.Quote(text)
	}
	return text
}
