// SPDX-License-Identifier: Apache-2.0

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

	"github.com/OWNER/bothy/services/server/cmd/bothy/importtool"
	"github.com/OWNER/bothy/services/server/cmd/bothy/importtool/bothyapi"
	"github.com/OWNER/bothy/services/server/internal/platform/buildinfo"
)

// importUsage is the help of "bothy import" (docs/spec/22-import-migration.md 10.1).
const importUsage = `Usage: bothy import login --server URL [--name NAME] [--session-file PATH]
       bothy import logout [--session-file PATH]
       bothy import discord --guild ID --map FILE [--apply] ... (bothy import discord --help)

login   logs the import tool in as a command-line device (platform cli) through device
        pairing. It prints the pairing code alone on stdout; in the app or the web console
        choose Add device and enter the code (that step needs step-up there). It polls until
        the owner approves (every import.login_poll_interval) and then writes the session file.
        The tool never asks for a password, passkey or step-up.
logout  ends the session on the server and deletes the session file.
discord takes the history of a Discord server over: a dry run by default, --apply imports into
        the source the owner started in the web console; init-map and status (see its --help).

--server URL         origin of the server, like server.public_url: https, or http only for a
                     loopback host (127.0.0.0/8, ::1, localhost); no path
--name NAME          device name (default "bothy import on <host name>")
--session-file PATH  session file (default <user configuration directory>/bothy/import-session.json);
                     created with mode 0600 in a directory with mode 0700, refused when group or
                     others can access it

Exit codes: 0 ok; 1 pairing expired, rejected or interrupted, network or server error;
2 invalid arguments, already logged in to this server, not logged in, session revoked, or a
session file that is invalid or accessible to others.
`

// importDeps are the parts of importCommand that tests replace.
type importDeps struct {
	// sessionPath returns the default session file path.
	sessionPath func() (string, error)
	// hostname returns the host name for the default device name.
	hostname func() (string, error)
	// limits are the tool values; M8-04 fills them from the tool configuration.
	limits bothyapi.Limits
	// deps are the client dependencies.
	deps bothyapi.Deps
	// environ returns the process environment for the tool configuration and the bot token; nil
	// means an empty environment.
	environ func() []string
}

// toolEnv returns the environment of "bothy import discord".
func (d importDeps) toolEnv(stdout, stderr io.Writer) importtool.Env {
	var environ []string
	if d.environ != nil {
		environ = d.environ()
	}
	return importtool.Env{Environ: environ, Stdout: stdout, Stderr: stderr, Version: buildinfo.Version(),
		SessionPath: d.sessionPath, Server: d.deps}
}

// defaultImportDeps uses the user configuration directory, the host name and the defaults of 22 19.
func defaultImportDeps() importDeps {
	return importDeps{sessionPath: bothyapi.DefaultSessionPath, hostname: os.Hostname, limits: bothyapi.DefaultLimits()}
}

// importCommand runs "bothy import" with the arguments after the command name.
func importCommand(ctx context.Context, args []string, stdout, stderr io.Writer, deps importDeps) int {
	if len(args) == 0 {
		_, _ = io.WriteString(stderr, importUsage)
		return exitUsage
	}
	if isHelpArg(args[0]) {
		_, _ = io.WriteString(stdout, importUsage)
		return exitOK
	}
	switch args[0] {
	case "login":
		return importLogin(ctx, args[1:], stdout, stderr, deps)
	case "logout":
		return importLogout(ctx, args[1:], stdout, stderr, deps)
	case "discord":
		return importtool.Main(ctx, args[1:], deps.toolEnv(stdout, stderr))
	}
	_, _ = fmt.Fprintf(stderr, "bothy import: unknown subcommand %q\n", printable(args[0]))
	_, _ = io.WriteString(stderr, importUsage)
	return exitUsage
}

// importFlags parses the flags of a subcommand; code is not -1 when the caller must return it.
func importFlags(fs *flag.FlagSet, args []string, stdout, stderr io.Writer) int {
	fs.SetOutput(io.Discard)
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			_, _ = io.WriteString(stdout, importUsage)
			return exitOK
		}
		_, _ = fmt.Fprintf(stderr, "bothy import: %s\n", printable(err.Error()))
		return exitUsage
	}
	if fs.NArg() > 0 {
		_, _ = fmt.Fprintf(stderr, "bothy import: unexpected argument %q\n", printable(fs.Arg(0)))
		return exitUsage
	}
	return -1
}

// sessionFile returns the --session-file value or the default path.
func (d importDeps) sessionFile(flagValue string) (string, error) {
	if flagValue != "" {
		return flagValue, nil
	}
	return d.sessionPath()
}

// importLogin runs "bothy import login". Ctrl-C ends the poll with exit 1 and writes nothing.
func importLogin(ctx context.Context, args []string, stdout, stderr io.Writer, deps importDeps) int {
	fs := flag.NewFlagSet("bothy import login", flag.ContinueOnError)
	server := fs.String("server", "", "server origin")
	name := fs.String("name", "", "device name")
	session := fs.String("session-file", "", "session file")
	if code := importFlags(fs, args, stdout, stderr); code >= 0 {
		return code
	}
	if *server == "" {
		_, _ = fmt.Fprintln(stderr, "bothy import login: --server is required")
		return exitUsage
	}
	path, err := deps.sessionFile(*session)
	if err != nil {
		return importFailed(stderr, err)
	}
	// The host name only fills the default device name; without one the name is "bothy import".
	host, _ := deps.hostname()
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()
	return importFailed(stderr, bothyapi.Login(ctx, bothyapi.LoginOptions{
		ServerURL: *server, DeviceName: *name, Hostname: host, SessionFile: path, AppVersion: buildinfo.Version(),
		Limits: deps.limits, Deps: deps.deps, Stdout: stdout, Stderr: stderr,
	}))
}

// importLogout runs "bothy import logout".
func importLogout(ctx context.Context, args []string, stdout, stderr io.Writer, deps importDeps) int {
	fs := flag.NewFlagSet("bothy import logout", flag.ContinueOnError)
	session := fs.String("session-file", "", "session file")
	if code := importFlags(fs, args, stdout, stderr); code >= 0 {
		return code
	}
	path, err := deps.sessionFile(*session)
	if err != nil {
		return importFailed(stderr, err)
	}
	err = bothyapi.Logout(ctx, bothyapi.Options{SessionFile: path, Limits: deps.limits, Deps: deps.deps})
	if err == nil {
		_, _ = fmt.Fprintln(stderr, "logged out")
	}
	return importFailed(stderr, err)
}

// importFailed prints err (whose text never carries a secret) and returns its exit code: 0 for
// nil, 2 for an invocation or session problem the user must fix first, 1 otherwise.
func importFailed(stderr io.Writer, err error) int {
	if err == nil {
		return exitOK
	}
	_, _ = fmt.Fprintf(stderr, "bothy import: %s\n", err.Error())
	for _, usage := range []error{bothyapi.ErrInvalidServerURL, bothyapi.ErrAlreadyLoggedIn, bothyapi.ErrNotLoggedIn,
		bothyapi.ErrSessionRevoked, bothyapi.ErrInsecureSessionFile, bothyapi.ErrInvalidSessionFile} {
		if errors.Is(err, usage) {
			return exitUsage
		}
	}
	return exitFailure
}
