// SPDX-License-Identifier: Apache-2.0

// Command bothy-computer-mcp is the stdio MCP server bothy_computer of computer use
// (docs/spec/27-computer-use.md building block C1, package internal/computermcp). The Claude Code
// runner starts it through the per-turn MCP file of a host turn (27 10.1) with exactly the
// environment variables BOTHY_COMPUTER_DIR, BOTHY_COMPUTER_NONCE, BOTHY_COMPUTER_TIMEOUT_MS,
// BOTHY_COMPUTER_PINS and BOTHY_COMPUTER_SERVER; it reads JSON-RPC lines on stdin, answers on
// stdout and logs only tool names, codes and durations as JSON lines on stderr.
//
// Flags: --version prints the version and exits; --help prints the usage. Any other argument is
// a usage error.
//
// Exit codes: 0 stdin ended or a signal stopped it, 1 reading stdin or writing stdout failed,
// 2 a usage or configuration error (named on stderr, never with the nonce).
//
// Example (what the runner writes into the MCP file):
//
//	{"mcpServers": {"bothy_computer": {"type": "stdio", "command": "/opt/bothy/bin/bothy-computer-mcp", "args": [], "env": {...}}}}
package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/OWNER/bothy/services/host/internal/buildinfo"
	"github.com/OWNER/bothy/services/host/internal/computermcp"
)

// Exit codes.
const (
	exitOK    = 0
	exitIO    = 1
	exitUsage = 2
)

const usage = `usage: bothy-computer-mcp [--version | --help]
Started by the Claude Code runner with the environment of docs/spec/27-computer-use.md 10.1;
speaks MCP on stdin and stdout.`

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := run(ctx, os.Args[1:], os.Getenv, os.Stdin, os.Stdout, os.Stderr)
	stop()
	os.Exit(code)
}

// run is the whole program with its inputs as parameters, for the tests: the arguments without
// the program name, the environment, stdin, stdout and stderr. It returns the exit code.
func run(ctx context.Context, args []string, getenv func(string) string, stdin io.Reader, stdout, stderr io.Writer) int {
	switch {
	case len(args) == 1 && args[0] == "--version":
		_, _ = fmt.Fprintln(stdout, buildinfo.Version)
		return exitOK
	case len(args) == 1 && (args[0] == "--help" || args[0] == "-h"):
		_, _ = fmt.Fprintln(stdout, usage)
		return exitOK
	case len(args) != 0:
		_, _ = fmt.Fprintln(stderr, usage)
		return exitUsage
	}
	cfg, err := computermcp.ConfigFromEnv(getenv)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return exitUsage
	}
	cfg.Version = buildinfo.Version
	cfg.Log = slog.New(slog.NewJSONHandler(stderr, nil))
	srv, err := computermcp.New(cfg)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return exitUsage
	}
	if err := srv.Serve(ctx, stdin, stdout); err != nil {
		_, _ = fmt.Fprintln(stderr, "bothy-computer-mcp: stdio failed")
		return exitIO
	}
	return exitOK
}
