// SPDX-License-Identifier: Apache-2.0

// Command bothy-smoke is the probe of the Compose smoke test (brief M1-73). infra/compose/smoke.sh
// runs it in the services smoke-probe and smoke-scan of infra/compose/test/compose.smoke.yaml;
// the package test/smoke documents the steps.
//
// Usage:
//
//	bothy-smoke precheck [--base-url URL] [--public-url URL]
//	bothy-smoke run      [--base-url URL] [--public-url URL] [--out DIR]
//	bothy-smoke scan     [--in DIR]
//
// precheck proves the setup lock before the bootstrap; run reads DIR/setup.out, drives the
// scenario and writes DIR/canaries.txt (mode 0600); scan reads IN/out/canaries.txt and scans every
// file below IN/otel and IN/logs, then checks that the collector export is not empty. The
// environment provides SMOKE_PUBLIC_URL (default of --public-url) and SMOKE_CANARY (run only).
// Steps are logged as "ok <step>" on stderr; no value is ever printed. Exit codes: 0 pass, 1 a
// check failed, 2 usage.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/OWNER/bothy/services/server/test/smoke"
)

const (
	// runTimeout bounds the whole scenario; its steps have their own shorter bounds.
	runTimeout = 5 * time.Minute
	// precheckTimeout bounds the two calls of the precheck.
	precheckTimeout = time.Minute
	// serviceName is service.name of the server's telemetry (platform/otel, M1-10).
	serviceName = "bothy-server"
	// messageRoute is the route pattern of createMessage, the http.route of its spans.
	messageRoute = "/api/v1/threads/{thread_id}/messages"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stderr))
}

// getenv reads a variable of the smoke harness. The probe is a separate binary of the test
// overlay, not the server, and Compose hands it exactly two values: SMOKE_PUBLIC_URL and
// SMOKE_CANARY (infra/compose/test/README.md).
func getenv(name string) string {
	return os.Getenv(name) //nolint:forbidigo // the probe's only inputs from Compose, not server configuration
}

// run executes one subcommand and returns the exit code.
func run(args []string, stderr io.Writer) int {
	if len(args) == 0 {
		_, _ = fmt.Fprintln(stderr, "usage: bothy-smoke precheck|run|scan [flags]")
		return 2
	}
	fs := flag.NewFlagSet("bothy-smoke "+args[0], flag.ContinueOnError)
	fs.SetOutput(stderr)
	baseURL := fs.String("base-url", "http://127.0.0.1:8080", "address the probe dials (the api listener in the shared namespace)")
	publicURL := fs.String("public-url", getenv("SMOKE_PUBLIC_URL"), "server.public_url; DPoP proofs name it")
	out := fs.String("out", "/out", "directory with setup.out; canaries.txt is written there")
	in := fs.String("in", "/in", "input of the scan: out/canaries.txt, otel/ and logs/")
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}
	var err error
	switch args[0] {
	case "precheck":
		err = precheck(*baseURL, *publicURL)
	case "run":
		err = scenario(*baseURL, *publicURL, *out, stderr)
	case "scan":
		err = scan(*in, stderr)
	default:
		_, _ = fmt.Fprintf(stderr, "bothy-smoke: unknown subcommand %q\n", args[0])
		return 2
	}
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "bothy-smoke %s: FAIL: %v\n", args[0], err)
		return 1
	}
	_, _ = fmt.Fprintf(stderr, "bothy-smoke %s: ok\n", args[0])
	return 0
}

func precheck(baseURL, publicURL string) error {
	api, err := smoke.NewHTTPClient(baseURL, publicURL)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), precheckTimeout)
	defer cancel()
	return smoke.Precheck(ctx, api)
}

func scenario(baseURL, publicURL, out string, stderr io.Writer) error {
	setupOut, err := os.ReadFile(filepath.Join(out, "setup.out")) // #nosec G304 -- the output directory of the run
	if err != nil {
		return errors.New("cannot read setup.out")
	}
	api, err := smoke.NewHTTPClient(baseURL, publicURL)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), runTimeout)
	defer cancel()
	cfg := smoke.Config{PublicURL: publicURL, Canary: getenv("SMOKE_CANARY"), SetupStdout: setupOut, Log: stderr}
	canaries, runErr := smoke.Run(ctx, cfg, api, func() smoke.Gateway { return smoke.NewWSGateway(baseURL) })
	// The canaries are written also after a failure, so a kept stack can still be scanned.
	if err := os.WriteFile(filepath.Join(out, "canaries.txt"), canaries.Marshal(), 0o600); err != nil {
		return errors.Join(runErr, fmt.Errorf("write canaries.txt: %w", err))
	}
	return runErr
}

func scan(in string, stderr io.Writer) error {
	raw, err := os.ReadFile(filepath.Join(in, "out", "canaries.txt")) // #nosec G304 -- the input directory of the scan
	if err != nil {
		return errors.New("cannot read out/canaries.txt")
	}
	canaries, err := smoke.ParseCanaries(raw)
	if err != nil {
		return err
	}
	hits, err := smoke.Scan(canaries, smoke.Roots(in))
	if err != nil {
		return err
	}
	for _, h := range hits {
		_, _ = fmt.Fprintf(stderr, "leak: %s\n", h)
	}
	if len(hits) > 0 {
		return fmt.Errorf("%d occurrences of canary values in the collector export or the container logs", len(hits))
	}
	_, _ = fmt.Fprintln(stderr, "ok no canary value in the collector export or the container logs")
	if err := smoke.CheckTelemetry(filepath.Join(in, "otel"), serviceName, messageRoute); err != nil {
		return err
	}
	_, _ = fmt.Fprintln(stderr, "ok telemetry present")
	return nil
}
