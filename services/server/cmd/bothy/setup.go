// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"slices"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/platform/otel"
	"github.com/OWNER/bothy/services/server/internal/platform/otel/attr"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

// setupUsage is the help of "bothy setup" (docs/spec/02-architecture.md "Commands of the binary",
// docs/spec/08-identity.md section 4).
const setupUsage = `Usage: bothy setup bootstrap-owner --handle HANDLE --display-name NAME [--locale TAG] [--time-zone ZONE] [--config FILE]
       bothy setup issue-enrollment-code --user HANDLE [--config FILE]

bootstrap-owner        creates the first owner in one transaction: the local identity provider
                       if missing, the person, its owner role and local identity, a one-time
                       enrollment code (first_login), the audit entry instance.bootstrap and
                       the setup mark. Until then the server answers every request except
                       discovery with 503 setup_required.
issue-enrollment-code  issues a recovery code for an active owner who lost every device.

Both connect to the database as the application role (like bothy serve), need the
writable signing key store of the api service, and print the code exactly once as the only
line on stdout; logs go to stderr and never contain it. Hand the code over in person.

--handle HANDLE      handle of the first owner, lower case (^[a-z0-9][a-z0-9._-]*$)
--display-name NAME  display name of the first owner
--locale TAG         BCP 47 locale (default de)
--time-zone ZONE     IANA time zone (default UTC)
--user HANDLE        handle of the active owner to recover
--config FILE        configuration file, not empty (default: BOTHY_CONFIG_FILE, else none)

Exit codes: 0 ok, 1 configuration, secret or database error, 2 invalid arguments (also a
handle that is no active owner), 3 bootstrap-owner: setup is already complete (nothing
changed), 4 issue-enrollment-code: setup is not complete yet.
`

// The subcommands and the exit codes of bothy setup (02 section 3.3).
const (
	subcommandBootstrap = "bootstrap-owner"
	subcommandRecovery  = "issue-enrollment-code"
	exitSetupComplete   = 3
	exitSetupPending    = 4
	// setupCloseBudget bounds the flush of audit denials when the command ends.
	setupCloseBudget = 5 * time.Second
)

// setupDeps are the parts of setupCommand that tests replace: the assembly over the loaded
// values.
type setupDeps struct {
	open func(ctx context.Context, v *config.Values, env []string, log *slog.Logger) (setupRunner, error)
}

// setupRunner is what bothy setup runs: wire.Setup in production.
type setupRunner interface {
	BootstrapOwner(ctx context.Context, cmd wire.BootstrapOwnerCmd) (wire.BootstrapResult, error)
	IssueRecoveryCode(ctx context.Context, handle string) (wire.IssuedEnrollmentCode, error)
	Close(ctx context.Context)
}

// defaultSetupDeps opens wire.Setup.
func defaultSetupDeps() setupDeps {
	return setupDeps{open: func(ctx context.Context, v *config.Values, env []string, log *slog.Logger) (setupRunner, error) {
		return wire.NewSetup(ctx, v, env, log)
	}}
}

// setupOptions are the flags of the two subcommands.
type setupOptions struct {
	file, handle, displayName, locale, timeZone, user string
}

// setupCommand runs "bothy setup <subcommand> [flags]" with the arguments after "setup" and returns
// the exit code of the usage text.
func setupCommand(ctx context.Context, args, env []string, stdout, stderr io.Writer, d setupDeps) int {
	if len(args) == 0 {
		return setupUsageError(stderr, "")
	}
	if isHelpArg(args[0]) {
		_, _ = io.WriteString(stdout, setupUsage)
		return exitOK
	}
	sub := args[0]
	if sub != subcommandBootstrap && sub != subcommandRecovery {
		return setupUsageError(stderr, fmt.Sprintf("unknown subcommand %q", printable(sub)))
	}
	o, err := parseSetupFlags(sub, args[1:])
	if errors.Is(err, flag.ErrHelp) {
		_, _ = io.WriteString(stdout, setupUsage)
		return exitOK
	}
	if err != nil {
		return setupUsageError(stderr, err.Error())
	}
	s, values, ok := loadConfig(env, wire.ConfigSchema, configOptions{file: o.file}, stderr)
	if !ok {
		return exitFailure
	}
	log := commandLogger(stderr, s, values)
	runner, err := d.open(ctx, values, env, log)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "bothy setup: %v\n", err)
		return exitFailure
	}
	defer func() {
		cctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), setupCloseBudget)
		defer cancel()
		runner.Close(cctx)
	}()
	if sub == subcommandBootstrap {
		return runBootstrap(ctx, runner, o, values, env, stdout, stderr, log)
	}
	return runRecovery(ctx, runner, o, values, env, stdout, stderr, log)
}

// parseSetupFlags parses the flags of sub; both take --config, bootstrap-owner its four person
// flags (handle and display name required), issue-enrollment-code --user (required).
func parseSetupFlags(sub string, args []string) (setupOptions, error) {
	fs := flag.NewFlagSet("bothy setup "+sub, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var o setupOptions
	fs.StringVar(&o.file, "config", "", "configuration file")
	if sub == subcommandBootstrap {
		fs.StringVar(&o.handle, "handle", "", "handle")
		fs.StringVar(&o.displayName, "display-name", "", "display name")
		fs.StringVar(&o.locale, "locale", "de", "locale")
		fs.StringVar(&o.timeZone, "time-zone", "UTC", "time zone")
	} else {
		fs.StringVar(&o.user, "user", "", "handle of the owner")
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return setupOptions{}, err
		}
		return setupOptions{}, errors.New(printable(err.Error()))
	}
	if fs.NArg() > 0 {
		return setupOptions{}, fmt.Errorf("unexpected argument %q", printable(fs.Arg(0)))
	}
	if o.file == "" && flagGiven(fs, "config") {
		return setupOptions{}, errors.New("--config needs a file path")
	}
	switch {
	case sub == subcommandBootstrap && o.handle == "":
		return setupOptions{}, errors.New("--handle is required")
	case sub == subcommandBootstrap && o.displayName == "":
		return setupOptions{}, errors.New("--display-name is required")
	case sub == subcommandRecovery && o.user == "":
		return setupOptions{}, errors.New("--user is required")
	}
	return o, nil
}

// runBootstrap runs bootstrap-owner and prints the code.
func runBootstrap(ctx context.Context, r setupRunner, o setupOptions, v *config.Values, env []string, stdout, stderr io.Writer, log *slog.Logger) int {
	res, err := r.BootstrapOwner(ctx, wire.BootstrapOwnerCmd{Handle: o.handle, DisplayName: o.displayName, Locale: o.locale, TimeZone: o.timeZone})
	switch {
	case errors.Is(err, wire.ErrSetupCompleted):
		_, _ = fmt.Fprintln(stderr, "bothy setup: setup is already complete; nothing changed")
		return exitSetupComplete
	case err != nil:
		return setupFailed(stderr, err)
	}
	if !printCode(ctx, res.Code, authorize.ActionInstanceBootstrap, res.OwnerID.String(), v, env, stdout, stderr, log) {
		return exitFailure
	}
	return exitOK
}

// runRecovery runs issue-enrollment-code and prints the code.
func runRecovery(ctx context.Context, r setupRunner, o setupOptions, v *config.Values, env []string, stdout, stderr io.Writer, log *slog.Logger) int {
	code, err := r.IssueRecoveryCode(ctx, o.user)
	switch {
	case errors.Is(err, wire.ErrSetupPending):
		_, _ = fmt.Fprintln(stderr, "bothy setup: setup is not complete; run bothy setup bootstrap-owner first")
		return exitSetupPending
	case errors.Is(err, wire.ErrNotActiveOwner):
		_, _ = fmt.Fprintf(stderr, "bothy setup: %s is not an active owner\n", printable(o.user))
		return exitUsage
	case err != nil:
		return setupFailed(stderr, err)
	}
	if !printCode(ctx, code, authorize.ActionOwnerRecover, "", v, env, stdout, stderr, log) {
		return exitFailure
	}
	return exitOK
}

// setupFailed prints err and returns 2 for invalid input (validation_failed, conflict on the
// handle) and 1 otherwise.
func setupFailed(stderr io.Writer, err error) int {
	_, _ = fmt.Fprintf(stderr, "bothy setup: %v\n", err)
	if slices.Contains([]errs.Kind{errs.KindBadRequest, errs.KindConflict, errs.KindUnprocessable}, errs.KindOf(err)) {
		return exitUsage
	}
	return exitFailure
}

// printCode writes the code as the only line on stdout (02 section 3.3), then one info line on
// stderr with the action and the pseudonym of the person when it can be computed (19 SEC-LOG-4: a
// person appears in logs only as pseudonym, so the handle is not logged), and the expiry in RFC
// 3339 UTC as a text line. The code goes nowhere else. It reports false when stdout fails.
func printCode(ctx context.Context, c wire.IssuedEnrollmentCode, act authorize.Action, personID string, v *config.Values,
	env []string, stdout, stderr io.Writer, log *slog.Logger) bool {
	if _, err := fmt.Fprintln(stdout, c.Code); err != nil {
		// The code is committed but lost; a new one replaces it (issue-enrollment-code).
		_, _ = fmt.Fprintf(stderr, "bothy setup: the code could not be written to stdout: %v; issue a new one with bothy setup issue-enrollment-code\n", err)
		return false
	}
	attrs := []slog.Attr{slog.String(attr.Action, string(act))}
	if personID != "" {
		if p, ok := pseudonym(ctx, v, env, personID, log); ok {
			attrs = append(attrs, slog.String(attr.PrincipalPseudoID, p))
		}
	}
	log.LogAttrs(ctx, slog.LevelInfo, msgCodeIssued, attrs...)
	// The log allowlist has no time key; the expiry is for the operator, so it goes to stderr as text.
	_, _ = fmt.Fprintf(stderr, "bothy setup: the code is valid until %s; hand it over in person\n", c.ExpiresAt.UTC().Format(time.RFC3339))
	return true
}

// msgCodeIssued is the info line of a setup command that issued a code.
const msgCodeIssued = "enrollment code issued"

// pseudonym returns the telemetry pseudonym of personID; ok is false when the key cannot be read,
// which never fails the command after its commit.
func pseudonym(ctx context.Context, v *config.Values, env []string, personID string, log *slog.Logger) (string, bool) {
	cfg, err := wire.TelemetryConfig(v)
	if err != nil {
		return "", false
	}
	sp, err := wire.NewSecretsProvider(v, env, log)
	if err != nil {
		return "", false
	}
	p, err := otel.NewPseudonymizer(ctx, sp, cfg.PseudonymKeyRef, cfg.PseudonymRotation, nil)
	if err != nil {
		return "", false
	}
	s, err := p.Pseudonym(ctx, personID, time.Now())
	return s, err == nil
}

// setupUsageError prints msg (when set) and the usage on stderr and returns the usage exit code.
func setupUsageError(stderr io.Writer, msg string) int {
	if msg != "" {
		_, _ = fmt.Fprintf(stderr, "bothy setup: %s\n", msg)
	}
	_, _ = io.WriteString(stderr, setupUsage)
	return exitUsage
}
