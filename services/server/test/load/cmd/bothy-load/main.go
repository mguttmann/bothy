// SPDX-License-Identifier: Apache-2.0

// Command bothy-load is the driver of the capacity load test (brief M7-18,
// docs/spec/16-operations.md section 13). tools/load/capacity_run.py runs it next to an
// installation from the images; tests/load/README.md describes the scenario.
//
// Usage:
//
//	bothy-load ramp --base-url URL --public-url URL --dir DIR \
//	    --message-create-p99 D --fanout-p99 D --messages-per-minute N [--rates R1,R2,...]
//
// ramp reads the owner's enrollment code from DIR/setup.out, seeds the world of the largest step
// (load.Driver.Seed), writes DIR/seeded and waits for DIR/measure, which the runner writes after it
// restarted the server with its default rate limits; then it runs the ramp (load.Runner.Ramp) and
// writes DIR/result.json (load.Result, mode 0600). The thresholds come from the caller, which reads
// them from the tables of the specification. Progress lines go to stderr; no token, code or text
// is ever printed. Exit codes: 0 the result file was written (whatever the ramp measured), 1 the
// seeding or the run failed, 2 usage.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/OWNER/bothy/services/server/test/load"
)

const (
	// measureWait bounds the wait for DIR/measure: the runner restarts the server in between.
	measureWait = 30 * time.Minute
	// measurePoll is how often the command looks for DIR/measure.
	measurePoll = time.Second
)

func main() {
	os.Exit(run(os.Args[1:], os.Stderr))
}

// run is main without the process exit, so the usage errors are testable.
func run(args []string, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "ramp" {
		_, _ = fmt.Fprintln(stderr, "usage: bothy-load ramp --base-url URL --public-url URL --dir DIR --message-create-p99 D --fanout-p99 D --messages-per-minute N [--rates R1,R2,...]")
		return 2
	}
	fs := flag.NewFlagSet("ramp", flag.ContinueOnError)
	fs.SetOutput(stderr)
	base := fs.String("base-url", "", "URL the driver dials, for example http://127.0.0.1:8090")
	public := fs.String("public-url", "", "server.public_url of the installation (DPoP htu)")
	dir := fs.String("dir", "", "folder with setup.out; receives seeded and result.json")
	createP99 := fs.Duration("message-create-p99", 0, "perf.message_create_p99")
	fanoutP99 := fs.Duration("fanout-p99", 0, "perf.fanout_local_p99")
	perMinute := fs.Int("messages-per-minute", 0, "the default of ratelimit.rl.message.per_minute")
	rates := fs.String("rates", "", "comma-separated rising rates of the ramp in messages per second (default load.RampRates)")
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}
	targets := load.Targets{MessageCreateP99: *createP99, FanoutP99: *fanoutP99, MessagesPerMinute: *perMinute}
	ramp, err := parseRates(*rates)
	if err == nil {
		err = targets.Valid()
	}
	if err == nil && (*dir == "" || *base == "" || *public == "") {
		err = errors.New("--base-url, --public-url and --dir are required")
	}
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "bothy-load:", err)
		return 2
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := ramped(ctx, *base, *public, *dir, targets, ramp, stderr); err != nil {
		_, _ = fmt.Fprintln(stderr, "bothy-load:", err)
		return 1
	}
	return 0
}

// parseRates reads --rates; empty gives load.RampRates.
func parseRates(s string) ([]float64, error) {
	if strings.TrimSpace(s) == "" {
		return load.RampRates, nil
	}
	var out []float64
	for f := range strings.SplitSeq(s, ",") {
		v, err := strconv.ParseFloat(strings.TrimSpace(f), 64)
		if err != nil {
			return nil, fmt.Errorf("--rates: %q is no number", f)
		}
		out = append(out, v)
	}
	return out, nil
}

// ramped seeds, waits for the go of the runner, runs the ramp and writes the result file.
func ramped(ctx context.Context, base, public, dir string, targets load.Targets, rates []float64, log io.Writer) error {
	steps, err := load.PlanRamp(rates, targets.MessagesPerMinute)
	if err != nil {
		return err
	}
	code, err := os.ReadFile(filepath.Join(dir, "setup.out")) // #nosec G304 -- the folder of the run
	if err != nil {
		return fmt.Errorf("read the enrollment code: %w", err)
	}
	d, err := load.NewDriver(base, public, log)
	if err != nil {
		return err
	}
	gen := load.NewGenerator(load.Seed)
	world, err := d.Seed(ctx, strings.TrimSpace(string(code)), gen, load.People(steps))
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "seeded"), []byte("seeded\n"), 0o600); err != nil {
		return err
	}
	if err := waitFor(ctx, filepath.Join(dir, "measure")); err != nil {
		return err
	}
	r := &load.Runner{Driver: d, World: world, Gen: gen, Targets: targets, Warmup: load.Warmup, Window: load.Window, Drain: load.Drain}
	res := r.Ramp(ctx, steps)
	b, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		return err
	}
	if _, err := load.ParseResult(b); err != nil {
		return fmt.Errorf("the ramp produced no valid result: %w", err)
	}
	return os.WriteFile(filepath.Join(dir, "result.json"), append(b, '\n'), 0o600)
}

// waitFor polls until path exists, at most measureWait.
func waitFor(ctx context.Context, path string) error {
	ctx, cancel := context.WithTimeout(ctx, measureWait)
	defer cancel()
	for {
		if _, err := os.Stat(path); err == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("%s did not appear: %w", filepath.Base(path), ctx.Err())
		case <-time.After(measurePoll):
		}
	}
}
