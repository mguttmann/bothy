// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/platform/otel"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

// telemetryUsage is the help of "bothy telemetry" (docs/spec/02-architecture.md "Commands of
// the binary", docs/observability/privacy.md "Pseudonyms" item 2).
const telemetryUsage = `Usage: bothy telemetry pseudonym <principal_id> [--at TIME] [--config FILE]

pseudonym  prints the telemetry pseudonym (bothy.principal.pseudo_id) of a principal for the
           period that contains --at and the start of that period in UTC, so that the owner
           finds the spans and logs of one person. It loads the configuration as
           bothy config check does and reads telemetry.pseudonym_key_ref through the secret
           provider; it needs no database. The key itself is never printed.

Output on stdout:
  pseudonym <32 lowercase hex characters>
  period_start <start of the period, RFC 3339 in UTC>

<principal_id>  id of a human principal, a UUID in any letter case
--at TIME       instant in RFC 3339, for example 2026-10-11T12:00:00Z (default: now)
--config FILE   configuration file, not empty (default: BOTHY_CONFIG_FILE, else none)

Flags may come before or after <principal_id>.

Exit codes: 0 ok, 1 the key cannot be read or is shorter than 32 bytes, 2 usage error or a
configuration that does not load.
`

// subcommandPseudonym is the only subcommand of bothy telemetry.
const subcommandPseudonym = "pseudonym"

// keyLogsLevel is the configuration key of the log level of commands and the server.
const keyLogsLevel = "telemetry.logs.level"

// telemetryCommand runs "bothy telemetry pseudonym <principal_id> [--at TIME] [--config FILE]"
// with the arguments after "telemetry". env is the process environment, now the clock used
// without --at; both are parameters so that tests pass their own. ctx bounds the read of the
// key. It prints the pseudonym and the start of its period on stdout and returns 0; 1 when the
// key cannot be read or is shorter than otel.MinKeyBytes; 2 for a usage error, a principal id
// that is not a UUID or a configuration that does not load (02 "Commands of the binary": 2 is a
// call or configuration error). Every reason goes to stderr without the key.
func telemetryCommand(ctx context.Context, args, env []string, now func() time.Time, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		return telemetryUsageError(stderr, "")
	}
	if isHelpArg(args[0]) {
		_, _ = io.WriteString(stdout, telemetryUsage)
		return exitOK
	}
	if args[0] != subcommandPseudonym {
		return telemetryUsageError(stderr, fmt.Sprintf("unknown subcommand %q", args[0]))
	}
	o, err := parsePseudonymArgs(args[1:])
	if errors.Is(err, flag.ErrHelp) {
		_, _ = io.WriteString(stdout, telemetryUsage)
		return exitOK
	}
	if err != nil {
		return telemetryUsageError(stderr, err.Error())
	}
	at := now()
	if o.atGiven {
		at = o.at
	}

	s, values, ok := loadConfig(env, wire.ConfigSchema, configOptions{file: o.file}, stderr)
	if !ok {
		return exitUsage
	}
	cfg, err := wire.TelemetryConfig(values)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "bothy telemetry: %v\n", err)
		return exitFailure
	}
	provider, err := wire.NewSecretsProvider(values, env, commandLogger(stderr, s, values))
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "bothy telemetry: %v\n", err)
		return exitFailure
	}
	p, err := otel.NewPseudonymizer(ctx, provider, cfg.PseudonymKeyRef, cfg.PseudonymRotation, nil)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "bothy telemetry: %v\n", err)
		return exitFailure
	}
	pseudo, start, err := p.PseudonymWithPeriod(ctx, o.id, at)
	if errors.Is(err, otel.ErrPrincipalID) {
		return telemetryUsageError(stderr, fmt.Sprintf("principal id %q is not a UUID", o.id))
	}
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "bothy telemetry: %v\n", err)
		return exitFailure
	}
	_, _ = fmt.Fprintf(stdout, "pseudonym %s\nperiod_start %s\n", pseudo, start.Format(time.RFC3339))
	return exitOK
}

// pseudonymOptions holds the argument and the flags of telemetry pseudonym.
type pseudonymOptions struct {
	id, file string
	at       time.Time
	atGiven  bool
}

// parsePseudonymArgs parses "<principal_id> [--at TIME] [--config FILE]" with the flags before
// or after the id. It returns flag.ErrHelp for -h and --help; the message of any other error is
// the line telemetryUsageError prints.
func parsePseudonymArgs(args []string) (pseudonymOptions, error) {
	fs := flag.NewFlagSet("bothy telemetry pseudonym", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	file := fs.String("config", "", "configuration file")
	atText := fs.String("at", "", "instant in RFC 3339")
	var ids []string
	for {
		if err := fs.Parse(args); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				return pseudonymOptions{}, err
			}
			return pseudonymOptions{}, errors.New(printable(err.Error()))
		}
		if fs.NArg() == 0 {
			break
		}
		// The flag package stops at the first argument that is no flag; the id may come first.
		ids = append(ids, fs.Arg(0))
		args = fs.Args()[1:]
	}
	if len(ids) != 1 {
		return pseudonymOptions{}, fmt.Errorf("want exactly one principal id, got %d", len(ids))
	}
	if *file == "" && flagGiven(fs, "config") {
		return pseudonymOptions{}, errors.New("--config needs a file path")
	}
	o := pseudonymOptions{id: ids[0], file: *file}
	if flagGiven(fs, "at") {
		at, err := time.Parse(time.RFC3339, *atText)
		if err != nil {
			return pseudonymOptions{}, fmt.Errorf("--at %q is no RFC 3339 time, for example 2026-10-11T12:00:00Z", *atText)
		}
		o.at, o.atGiven = at, true
	}
	return o, nil
}

// telemetryUsageError prints msg (when set) and the usage on stderr and returns the usage exit
// code.
func telemetryUsageError(stderr io.Writer, msg string) int {
	if msg != "" {
		_, _ = fmt.Fprintf(stderr, "bothy telemetry: %s\n", msg)
	}
	_, _ = io.WriteString(stderr, telemetryUsage)
	return exitUsage
}

// commandLogger returns the logger of a short-lived command (brief M1-10): JSON lines on stderr
// through the attribute allowlist of platform/otel, never OTLP. The level is
// telemetry.logs.level when s has that key, else info, the level before any configuration is
// loaded.
func commandLogger(stderr io.Writer, s *config.Schema, values *config.Values) *slog.Logger {
	level := slog.LevelInfo
	if s != nil && values != nil {
		for _, k := range s.Keys() {
			if k.Name != keyLogsLevel {
				continue
			}
			if l, err := otel.ParseLevel(values.String(keyLogsLevel)); err == nil {
				level = l
			}
		}
	}
	return otel.NewCommandLogger(stderr, level)
}
