// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/OWNER/bothy/services/server/internal/platform/config"
)

// configUsage is the help of "bothy config" (docs/spec/02-architektur.md "Befehle des Binarys",
// "Konfigurationsladen" item 7).
const configUsage = `Usage: bothy config check [--config FILE] [--role ROLE]
       bothy config print [--redacted] [--config FILE] [--role ROLE]

check  loads the configuration as the server start does, without starting, and prints
       "configuration ok" or every problem on stderr.
print  prints the effective value and the source of every key, sorted by name. Secret
       references are shown as references; nothing is resolved. The output is always
       redacted; --redacted only states it, and --redacted=false is a usage error.

--config FILE  configuration file, not empty (default: BOTHY_CONFIG_FILE, else none)
--role ROLE    process role (api, worker or all) as bothy serve --role sets it

Exit codes: 0 ok, 1 the configuration has problems, 2 usage error.
`

// configCommand runs "bothy config <check|print> [flags]" with the arguments after "config".
// env is the process environment and schema builds the configuration schema; both are parameters
// so that tests pass their own. It returns 0 when the configuration loads, 1 when it has problems
// (each printed on stderr) or the schema is invalid, and 2 for a usage error.
func configCommand(args, env []string, schema func() (*config.Schema, error), stdout, stderr io.Writer) int {
	if len(args) == 0 {
		return configUsageError(stderr, "")
	}
	sub := args[0]
	switch sub {
	case "help", "-h", "--help":
		_, _ = io.WriteString(stdout, configUsage)
		return exitOK
	case "check", "print":
	default:
		return configUsageError(stderr, fmt.Sprintf("unknown subcommand %q", sub))
	}

	fs := flag.NewFlagSet("bothy config "+sub, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	file := fs.String("config", "", "configuration file")
	role := fs.String("role", "", "process role")
	redacted := true
	if sub == "print" {
		// Accepted so that scripts can state it; print never shows more than this.
		fs.BoolVar(&redacted, "redacted", true, "redacted output")
	}
	if err := fs.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			_, _ = io.WriteString(stdout, configUsage)
			return exitOK
		}
		// The flag package puts the flag name into the message unquoted; printable keeps a name
		// with a line break or an escape sequence on one line (docs/DECISIONS.md M1-07/26).
		return configUsageError(stderr, printable(err.Error()))
	}
	if fs.NArg() > 0 {
		return configUsageError(stderr, fmt.Sprintf("unexpected argument %q", fs.Arg(0)))
	}
	if !redacted {
		return configUsageError(stderr, "the output of print is always redacted; --redacted=false is not available")
	}
	if *file == "" && flagGiven(fs, "config") {
		// An empty path would silently fall back to BOTHY_CONFIG_FILE; a script with an unset
		// variable must not get another file than it named.
		return configUsageError(stderr, "--config needs a file path")
	}

	s, err := schema()
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "bothy config: %v\n", err)
		return exitFailure
	}
	src := config.Sources{Env: env, File: *file}
	if flagGiven(fs, "role") {
		src.Flags = map[string]string{"server.role": *role}
	}
	values, err := config.Load(s, src)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return exitFailure
	}
	if sub == "check" {
		_, _ = io.WriteString(stdout, "configuration ok\n")
		return exitOK
	}
	writeConfig(stdout, s, values)
	return exitOK
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
