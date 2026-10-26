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
//	bothy-smoke seed     [--base-url URL] [--public-url URL] [--out DIR]
//	bothy-smoke e2e-world [--base-url URL] [--public-url URL] [--out DIR]
//	bothy-smoke alerts   [--base-url URL] [--public-url URL] [--out DIR] [--ctl DIR] [--timeout D] --expect NAME|SEVERITY|SUMMARY ...
//	bothy-smoke alert-stub [--listen ADDR] [--out DIR] [--ctl DIR]
//	bothy-smoke portability [--base-url URL] [--public-url URL] [--out DIR] [--search-word WORD] [--timeout D]
//	bothy-smoke local-scan [--in DIR] [--loki URL] [--prometheus URL] [--tempo URL] [--since UNIX]
//
// precheck proves the setup lock before the bootstrap; run reads DIR/setup.out, drives the
// scenario and writes DIR/canaries.txt (mode 0600); scan reads IN/out/canaries.txt and scans every
// file below IN/otel and IN/logs, then checks that the collector export is not empty. The
// environment provides SMOKE_PUBLIC_URL (default of --public-url) and SMOKE_CANARY (run only).
// Steps are logged as "ok <step>" on stderr; no value is ever printed. Exit codes: 0 pass, 1 a
// check failed, 2 usage. seed prepares the console test of M6-03 instead of the scenario: it reads
// DIR/setup.out, gives the owner a password with TOTP and writes DIR/console-login.json (mode 0600).
// e2e-world prepares the desktop E2E test of M4-34 (infra/compose/e2e.sh): it reads DIR/setup.out,
// builds the world of smoke.BuildE2EWorld and writes DIR/e2e.json for the app tests and
// DIR/e2e-agent.json for the Fake-Agent (both mode 0600, never printed).
//
// alerts and alert-stub belong to the end-to-end test of the alert path (brief M7-20,
// tools/observability/alert_e2e.py). alerts reads DIR/setup.out, sets up the channel
// "Operations" with an app and an incoming webhook as the owner with step-up, writes the webhook
// URL to DIR/webhook-url (mode 0600, never printed), waits until every --expect alert has a firing
// message in the channel, removes CTL/firing so the test rule stops firing, waits for the
// resolved messages and writes the messages to DIR/alert-messages.json. alert-stub serves the
// external receivers and the metric of the test rule (smoke.AlertStub) until it is stopped.
//
// portability and local-scan belong to the portability test (brief M7-14,
// tools/portability/portability_run.py). portability reads DIR/portability-setup.out (a fresh
// enrollment code of the owner), runs smoke.Portability, writes the files of the test adapter to
// DIR/adapter (agent.json, token and dpop.pem, the secrets with mode 0600), waits for
// DIR/adapter/ready, which the adapter writes once it is connected, and writes
// DIR/canaries-portability.txt (mode 0600). local-scan exports what Loki, Prometheus and Tempo of
// the overlay local received since --since into IN/otel, scans IN/otel and IN/logs for the
// canaries of IN/out/canaries.txt and IN/out/canaries-portability.txt, and requires log lines,
// series and traces of the server.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
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
	// alertTimeout bounds each wait of alerts: the firing messages after the start of
	// Alertmanager, then the resolved ones, which Alertmanager sends at the next group_interval
	// (5m in alertmanager.yaml) after the rule stops.
	alertTimeout = 10 * time.Minute
	// alertPoll is the interval of the polls of the alert channel.
	alertPoll = 2 * time.Second
	// stubReadTimeout bounds the header read of one request to alert-stub.
	stubReadTimeout = 10 * time.Second
	// portabilityTimeout bounds the whole portability scenario: the scan of the two uploads
	// waits for clamd, which may still load its signatures after the start.
	portabilityTimeout = 15 * time.Minute
	// portabilityPoll is the interval of the polls of the portability scenario and of the wait
	// for the adapter.
	portabilityPoll = time.Second
	// exportTimeout bounds the export of the local stack, its repetitions included.
	exportTimeout = 5 * time.Minute
	// exportPoll is the interval between two exports while a signal of the server is missing.
	exportPoll = 5 * time.Second
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
		_, _ = fmt.Fprintln(stderr, "usage: bothy-smoke precheck|run|scan|seed|e2e-world|alerts|alert-stub|portability|local-scan [flags]")
		return 2
	}
	fs := flag.NewFlagSet("bothy-smoke "+args[0], flag.ContinueOnError)
	fs.SetOutput(stderr)
	baseURL := fs.String("base-url", "http://127.0.0.1:8080", "address the probe dials (the api listener in the shared namespace)")
	publicURL := fs.String("public-url", getenv("SMOKE_PUBLIC_URL"), "server.public_url; DPoP proofs name it")
	out := fs.String("out", "/out", "directory with setup.out; canaries.txt is written there")
	in := fs.String("in", "/in", "input of the scan: out/canaries.txt, otel/ and logs/")
	ctl := fs.String("ctl", "/ctl", "control directory of the alert path test (the file firing)")
	listen := fs.String("listen", ":9000", "listen address of alert-stub")
	timeout := fs.Duration("timeout", alertTimeout, "bound of each wait of alerts (firing, resolved)")
	var expect expectFlag
	fs.Var(&expect, "expect", "alert of the test rule as NAME|SEVERITY|SUMMARY (repeatable, alerts only)")
	searchWord := fs.String("search-word", "", "word of the trigger message the search must find (portability only)")
	loki := fs.String("loki", "http://loki:3100", "query address of Loki (local-scan only)")
	prometheus := fs.String("prometheus", "http://prometheus:9090", "query address of Prometheus (local-scan only)")
	tempo := fs.String("tempo", "http://tempo:3200", "query address of Tempo (local-scan only)")
	since := fs.Int64("since", 0, "start of the exported time range in Unix seconds (local-scan only)")
	ptimeout := fs.Duration("portability-timeout", portabilityTimeout, "bound of the portability scenario")
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
	case "seed":
		err = seed(*baseURL, *publicURL, *out)
	case "e2e-world":
		err = e2eWorld(*baseURL, *publicURL, *out)
	case "alerts":
		err = alerts(alertsArgs{baseURL: *baseURL, publicURL: *publicURL, out: *out, ctl: *ctl, timeout: *timeout, expect: expect}, stderr)
	case "alert-stub":
		err = alertStub(*listen, *out, *ctl)
	case "portability":
		err = portability(*baseURL, *publicURL, *out, *searchWord, *ptimeout, stderr)
	case "local-scan":
		err = localScan(*in, smoke.LocalStack{Loki: *loki, Prometheus: *prometheus, Tempo: *tempo}, *since, stderr)
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
	setupOut, err := os.ReadFile(filepath.Join(out, "setup.out")) // #nosec G304 G703 -- the output directory of the run, a flag of the harness
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
	if err := os.WriteFile(filepath.Join(out, "canaries.txt"), canaries.Marshal(), 0o600); err != nil { // #nosec G703 -- the output directory of the run
		return errors.Join(runErr, fmt.Errorf("write canaries.txt: %w", err))
	}
	return runErr
}

func scan(in string, stderr io.Writer) error {
	raw, err := os.ReadFile(filepath.Join(in, "out", "canaries.txt")) // #nosec G304 G703 -- the input directory of the scan, a flag of the harness
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

// seed runs smoke.Seed and writes the login of the console test; the file is never printed.
func seed(baseURL, publicURL, out string) error {
	setupOut, err := os.ReadFile(filepath.Join(out, "setup.out")) // #nosec G304 G703 -- the output directory of the run, a flag of the harness
	if err != nil {
		return errors.New("cannot read setup.out")
	}
	api, err := smoke.NewHTTPClient(baseURL, publicURL)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), precheckTimeout)
	defer cancel()
	login, err := smoke.Seed(ctx, api, setupOut)
	if err != nil {
		return err
	}
	b, err := json.Marshal(login) // #nosec G117 -- the login of the test seed, written with mode 0600 and never printed
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(out, "console-login.json"), b, 0o600) // #nosec G703 -- the output directory of the run
}

// e2eWorld runs smoke.BuildE2EWorld and writes the world of the app tests and the credentials of
// the Fake-Agent; neither file is ever printed. publicURL is the address the app types.
func e2eWorld(baseURL, publicURL, out string) error {
	setupOut, err := os.ReadFile(filepath.Join(out, "setup.out")) // #nosec G304 G703 -- the output directory of the run, a flag of the harness
	if err != nil {
		return errors.New("cannot read setup.out")
	}
	api, err := smoke.NewHTTPClient(baseURL, publicURL)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), precheckTimeout)
	defer cancel()
	world, agent, err := smoke.BuildE2EWorld(ctx, api, setupOut, publicURL)
	if err != nil {
		return err
	}
	for name, v := range map[string]any{"e2e.json": world, "e2e-agent.json": agent} {
		b, err := json.Marshal(v) // #nosec G117 -- codes and credentials of the test world, written with mode 0600 and never printed
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(out, name), b, 0o600); err != nil { // #nosec G703 -- the output directory of the run
			return fmt.Errorf("write %s: %w", name, err)
		}
	}
	return nil
}

// expectFlag collects the repeated flag --expect.
type expectFlag []smoke.ExpectedAlert

// String implements flag.Value.
func (e *expectFlag) String() string { return fmt.Sprint(len(*e)) + " alerts" }

// Set implements flag.Value.
func (e *expectFlag) Set(s string) error {
	a, err := smoke.ParseExpectedAlert(s)
	if err != nil {
		return err
	}
	*e = append(*e, a)
	return nil
}

// alertsArgs are the flags of alerts.
type alertsArgs struct {
	baseURL, publicURL, out, ctl string
	timeout                      time.Duration
	expect                       []smoke.ExpectedAlert
}

// alerts runs the probe side of the alert path test; see the package comment.
func alerts(a alertsArgs, stderr io.Writer) error {
	if len(a.expect) == 0 {
		return errors.New("alerts needs at least one --expect")
	}
	setupOut, err := os.ReadFile(filepath.Join(a.out, "setup.out")) // #nosec G304 G703 -- the output directory of the run, a flag of the harness
	if err != nil {
		return errors.New("cannot read setup.out")
	}
	api, err := smoke.NewHTTPClient(a.baseURL, a.publicURL)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), precheckTimeout)
	ch, err := smoke.SetupAlertChannel(ctx, api, setupOut)
	cancel()
	if err != nil {
		return err
	}
	// Written to a temporary name and renamed, so the runner never reads a partial URL.
	tmp := filepath.Join(a.out, "webhook-url.tmp")
	if err := os.WriteFile(tmp, []byte(ch.WebhookURL), 0o600); err != nil { // #nosec G703 -- the output directory of the run
		return errors.New("cannot write webhook-url")
	}
	if err := os.Rename(tmp, filepath.Join(a.out, "webhook-url")); err != nil { // #nosec G703 -- the output directory of the run
		return errors.New("cannot write webhook-url")
	}
	_, _ = fmt.Fprintln(stderr, "ok alert channel, app and incoming webhook")
	var msgs []smoke.AlertMessage
	for _, st := range []smoke.State{smoke.StateFiring, smoke.StateResolved} {
		wctx, wcancel := context.WithTimeout(context.Background(), a.timeout)
		msgs, err = smoke.AwaitAlerts(wctx, api, ch.ThreadID, a.expect, st, alertPoll, stderr)
		wcancel()
		if err != nil {
			break
		}
		if st == smoke.StateFiring {
			// The test rule reads the metric of alert-stub, which follows this file.
			if rerr := os.Remove(filepath.Join(a.ctl, smoke.StubFlagFile)); rerr != nil { // #nosec G703 -- the control directory of the run
				return fmt.Errorf("cannot remove the flag file: %w", rerr)
			}
		}
	}
	b, merr := json.MarshalIndent(msgs, "", "  ")
	if merr != nil {
		return errors.Join(err, merr)
	}
	// The messages are kept also after a failure, so the report can show what arrived.
	if werr := os.WriteFile(filepath.Join(a.out, "alert-messages.json"), b, 0o600); werr != nil { // #nosec G703 -- the output directory of the run
		return errors.Join(err, werr)
	}
	return err
}

// alertStub serves smoke.AlertStub on listen until SIGTERM or SIGINT. The stub runs as process 1
// of its container, where a signal without a handler is ignored, so it handles both itself.
func alertStub(listen, out, ctl string) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, os.Interrupt)
	defer stop()
	srv := &http.Server{Addr: listen, Handler: &smoke.AlertStub{Out: out, Ctl: ctl}, ReadHeaderTimeout: stubReadTimeout}
	done := make(chan error, 1)
	go func() { done <- srv.ListenAndServe() }()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		sctx, cancel := context.WithTimeout(context.Background(), stubReadTimeout)
		defer cancel()
		return srv.Shutdown(sctx)
	}
}

// portability runs smoke.Portability; see the package comment.
func portability(baseURL, publicURL, out, searchWord string, timeout time.Duration, stderr io.Writer) error {
	if len(searchWord) < 8 {
		return errors.New("portability needs --search-word with at least 8 characters")
	}
	setupOut, err := os.ReadFile(filepath.Join(out, "portability-setup.out")) // #nosec G304 G703 -- the output directory of the run, a flag of the harness
	if err != nil {
		return errors.New("cannot read portability-setup.out")
	}
	api, err := smoke.NewHTTPClient(baseURL, publicURL)
	if err != nil {
		return err
	}
	adapterDir := filepath.Join(out, "adapter")
	if err := os.MkdirAll(adapterDir, 0o700); err != nil { // #nosec G703 -- the output directory of the run
		return errors.New("cannot create the adapter directory")
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cfg := smoke.PortabilityConfig{
		PublicURL: publicURL, Canary: getenv("SMOKE_CANARY"), SearchWord: searchWord, SetupStdout: setupOut, Log: stderr, Poll: portabilityPoll,
		WriteAdapter: func(f smoke.AdapterFiles) error { return writeAdapterFiles(adapterDir, f) },
		AwaitAdapter: func(ctx context.Context) error { return awaitFile(ctx, filepath.Join(adapterDir, "ready")) },
	}
	canaries, runErr := smoke.Portability(ctx, cfg, api)
	if err := os.WriteFile(filepath.Join(out, "canaries-portability.txt"), canaries.Marshal(), 0o600); err != nil { // #nosec G703 -- the output directory of the run
		return errors.Join(runErr, fmt.Errorf("write canaries-portability.txt: %w", err))
	}
	return runErr
}

// writeAdapterFiles writes the secrets first and agent.json last, renamed into place, so the
// runner, which starts the adapter when agent.json appears, never sees a partial set.
func writeAdapterFiles(dir string, f smoke.AdapterFiles) error {
	if err := os.WriteFile(filepath.Join(dir, "token"), []byte(f.Token), 0o600); err != nil { // #nosec G703 -- the adapter directory of the run
		return errors.New("cannot write the adapter token")
	}
	if err := os.WriteFile(filepath.Join(dir, "dpop.pem"), f.KeyPEM, 0o600); err != nil { // #nosec G703 -- the adapter directory of the run
		return errors.New("cannot write the adapter key")
	}
	b, err := json.Marshal(map[string]string{"instance_id": f.InstanceID.String(), "agent_id": f.AgentID.String()})
	if err != nil {
		return err
	}
	tmp := filepath.Join(dir, "agent.json.tmp")
	if err := os.WriteFile(tmp, b, 0o600); err != nil { // #nosec G703 -- the adapter directory of the run
		return errors.New("cannot write agent.json")
	}
	return os.Rename(tmp, filepath.Join(dir, "agent.json")) // #nosec G703 -- the adapter directory of the run
}

// awaitFile returns once path exists or ctx ends.
func awaitFile(ctx context.Context, path string) error {
	for {
		if _, err := os.Stat(path); err == nil { // #nosec G703 -- the adapter directory of the run
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("the adapter did not report ready: %w", ctx.Err())
		case <-time.After(portabilityPoll):
		}
	}
}

// localScan exports the local stack into IN/otel, scans it with the container logs and requires
// telemetry of the server in every signal.
func localScan(in string, ls smoke.LocalStack, since int64, stderr io.Writer) error {
	if since <= 0 {
		return errors.New("local-scan needs --since")
	}
	var canaries smoke.Canaries
	var err error
	for _, name := range []string{"canaries.txt", "canaries-portability.txt"} {
		raw, err := os.ReadFile(filepath.Join(in, "out", name)) // #nosec G304 G703 -- the input directory of the scan, a flag of the harness
		if err != nil {
			return fmt.Errorf("cannot read out/%s", name)
		}
		c, err := smoke.ParseCanaries(raw)
		if err != nil {
			return err
		}
		if len(canaries) > 0 && len(c) > 0 && c[0].Label == smoke.LabelCanary {
			c = c[1:] // the run canary leads the first list only; the positive control uses it
		}
		canaries = append(canaries, c...)
	}
	ctx, cancel := context.WithTimeout(context.Background(), exportTimeout)
	defer cancel()
	hc := &http.Client{Transport: &http.Transport{Proxy: nil}, Timeout: time.Minute}
	// The server exports metrics every BOTHY_OTEL_METRIC_EXPORT_INTERVAL_MS and the collector
	// batches, so the export repeats until every signal of the server arrived or ctx ends; the
	// last export is scanned.
	var counts smoke.TelemetryCounts
	for {
		counts, err = smoke.ExportLocal(ctx, hc, ls, time.Unix(since, 0), time.Now(), serviceName, filepath.Join(in, "otel"))
		if err != nil {
			return err
		}
		if counts.LogLines > 0 && counts.Series > 0 && counts.Traces > 0 {
			break
		}
		select {
		case <-ctx.Done():
		case <-time.After(exportPoll):
			continue
		}
		break
	}
	hits, err := smoke.Scan(canaries, smoke.Roots(in))
	if err != nil {
		return err
	}
	for _, h := range hits {
		_, _ = fmt.Fprintf(stderr, "leak: %s\n", h)
	}
	if len(hits) > 0 {
		return fmt.Errorf("%d occurrences of canary values in the local stack or the container logs", len(hits))
	}
	_, _ = fmt.Fprintf(stderr, "ok no canary value in Loki, Prometheus, Tempo or the container logs (%d canaries)\n", len(canaries))
	if counts.LogLines == 0 || counts.Series == 0 || counts.Traces == 0 {
		return fmt.Errorf("telemetry of %s missing in the local stack: %d log lines, %d series, %d traces", serviceName, counts.LogLines, counts.Series, counts.Traces)
	}
	_, _ = fmt.Fprintf(stderr, "ok telemetry through the overlay local: %d log lines, %d series, %d traces\n", counts.LogLines, counts.Series, counts.Traces)
	return nil
}
