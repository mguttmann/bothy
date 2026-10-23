// SPDX-License-Identifier: Apache-2.0

//go:build perf

// Package perfkit is the shared frame of the load tests of make perf (ADR 0063): it reads the
// thresholds and the reference profile from chapter 02, decides the profile mode of the host,
// starts the server process of a test with the limits of that mode, seeds a world through the
// public API with raised rate limits, restarts the server with the default configuration for the
// measurement, reads the durability settings and the clock offset of the database, and merges the
// results into .cache/perf/results.json.
//
// Example:
//
//	env := perfkit.Setup(t)
//	p := env.StartSeeding(t)
//	users := perfkit.NewUsers(t, p, "sender", 10)
//	env.Measure(t, p)
//	env.Record(t, p, []report.Result{env.Threshold(t, "perf.x").Result(t.Name(), v, n)}, nil)
package perfkit

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/wire"
	"github.com/OWNER/bothy/services/server/perf/internal/report"
	"github.com/OWNER/bothy/services/server/test/harness"
)

// Env is the frame of one load test: the targets of 02, the reference profile, the profile of
// this host and the limits of its mode.
type Env struct {
	Targets report.Targets
	Ref     report.Reference
	// Runtime is the container runtime of the database (docker info).
	Runtime report.Runtime
	Profile report.Profile
	// GOMAXPROCS of the measured server, DBCPUs and DBMemoryBytes of its database container; zero
	// outside the emulated mode.
	GOMAXPROCS    int
	DBCPUs        float64
	DBMemoryBytes int64
}

// Setup reads chapter 02 and the host and prints the mode first (ADR 0063 point 3). Errors fail t.
func Setup(t *testing.T) *Env {
	t.Helper()
	targets, err := report.Load(report.ArchitectureChapter())
	if err != nil {
		t.Fatal(err)
	}
	text, err := targets.Text("perf.reference_profile")
	if err != nil {
		t.Fatal(err)
	}
	ref, err := report.ParseReference(text)
	if err != nil {
		t.Fatal(err)
	}
	rt, err := runtimeResources(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	prof, err := report.Host(ref, rt)
	if err != nil {
		t.Fatal(err)
	}
	procs, cpus, mem := report.Limits(prof.Mode, ref)
	t.Logf("profile mode %s: host %d CPUs, %d bytes; database runtime %d CPUs, %d bytes; perf.reference_profile %d vCPU, %d bytes",
		prof.Mode, prof.CPUs, prof.MemoryBytes, rt.CPUs, rt.MemoryBytes, ref.CPUs, ref.MemoryBytes)
	return &Env{Targets: targets, Ref: ref, Runtime: rt, Profile: prof, GOMAXPROCS: procs, DBCPUs: cpus, DBMemoryBytes: mem}
}

// runtimeResources returns CPU count and memory of the container runtime of the databases
// (docker info: the Linux VM of Docker on macOS, the host on Linux), which the profile mode
// compares with perf.reference_profile (docs/DECISIONS.md M1-72/2).
func runtimeResources(ctx context.Context) (report.Runtime, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	out, err := exec.CommandContext(ctx, "docker", "info", "--format", "{{.NCPU}} {{.MemTotal}}").Output()
	if err != nil {
		return report.Runtime{}, fmt.Errorf("docker info: %w", err)
	}
	var rt report.Runtime
	if _, err := fmt.Sscan(string(out), &rt.CPUs, &rt.MemoryBytes); err != nil {
		return report.Runtime{}, fmt.Errorf("docker info %q: %w", out, err)
	}
	return rt, nil
}

// Threshold returns the target name of 02; errors fail t.
func (e *Env) Threshold(t testing.TB, name string) report.Threshold {
	t.Helper()
	th, err := e.Targets.Threshold(name)
	if err != nil {
		t.Fatal(err)
	}
	return th
}

// Count returns the plain number name of 02 (a load parameter); errors fail t.
func (e *Env) Count(t testing.TB, name string) int {
	t.Helper()
	n, err := e.Targets.Count(name)
	if err != nil {
		t.Fatal(err)
	}
	return n
}

// StartSeeding starts the server process of the test with every rate limit raised and the limits
// of the profile mode, plus the entries of extra.
func (e *Env) StartSeeding(t *testing.T, extra map[string]string) *harness.Process {
	t.Helper()
	cfg := harness.RaisedRateLimits()
	for k, v := range extra {
		cfg[k] = v
	}
	return harness.StartProcess(t, harness.ProcessOptions{Config: cfg, GOMAXPROCS: e.GOMAXPROCS, DBCPUs: e.DBCPUs, DBMemoryBytes: e.DBMemoryBytes})
}

// Measure restarts p on the same database with the default configuration plus extra (nil for
// none) and reads the durability settings of its database into the profile.
func (e *Env) Measure(t *testing.T, p *harness.Process, extra map[string]string) {
	t.Helper()
	if err := p.RestartWith(t.Context(), extra); err != nil {
		t.Fatal(err)
	}
	e.ReadDurability(t, p.AdminDB())
}

// Querier is a superuser connection or pool of the measured database.
type Querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// ReadDurability reads fsync, synchronous_commit and server_version (current_setting, what SHOW
// prints) of the measured database into the profile, before the first measurement (ADR 0063
// point 2). A value other than on is stored and ends the run not green; the measurement still runs.
func (e *Env) ReadDurability(t testing.TB, q Querier) {
	t.Helper()
	read := func(setting string) string {
		var v string
		if err := q.QueryRow(context.Background(), "SELECT current_setting($1)", setting).Scan(&v); err != nil {
			t.Fatal(err)
		}
		return v
	}
	e.Profile.Fsync, e.Profile.SynchronousCommit, e.Profile.PostgresVersion = read("fsync"), read("synchronous_commit"), read("server_version")
	if !e.Profile.Durable() {
		t.Errorf("fsync = %s, synchronous_commit = %s: the run cannot end green (ADR 0063 point 2)", e.Profile.Fsync, e.Profile.SynchronousCommit)
	}
}

// Record merges results and extra into the results file and fails t for every result that misses
// its threshold. The profile carries the durability values of ReadDurability; the resources of the
// database runtime go into extra.
func (e *Env) Record(t testing.TB, results []report.Result, extra map[string]any) {
	t.Helper()
	if extra == nil {
		extra = map[string]any{}
	}
	extra["profile.db_runtime_cpus"], extra["profile.db_runtime_memory_bytes"] = e.Runtime.CPUs, e.Runtime.MemoryBytes
	if err := report.Merge(report.ResultsPath(), e.Profile, results, extra); err != nil {
		t.Fatal(err)
	}
	for _, r := range results {
		t.Logf("%s: %g %s (threshold %s %g %s, %d samples)", r.Name, r.Value, r.Unit, r.Comparison, r.Threshold, r.Unit, r.Samples)
		if !r.Pass {
			t.Errorf("%s missed: %g %s, threshold %s %g %s", r.Name, r.Value, r.Unit, r.Comparison, r.Threshold, r.Unit)
		}
	}
}

// clockSamples is how many bracketed clock readings ClockOffset takes.
const clockSamples = 20

// ClockOffset returns how far the database clock of p runs ahead of the host clock
// (report.ClockOffset over bracketed SELECT clock_timestamp() samples).
func ClockOffset(t testing.TB, p *harness.Process) time.Duration {
	t.Helper()
	return ClockOffsetOf(t, p.AdminDB())
}

// ClockOffsetOf is ClockOffset for the superuser pool q of any measured database, such as the
// admin pool of an in-process harness server (perf/push_handoff).
func ClockOffsetOf(t testing.TB, q Querier) time.Duration {
	t.Helper()
	samples := make([]report.ClockSample, 0, clockSamples)
	for range clockSamples {
		var s report.ClockSample
		s.Before = time.Now()
		if err := q.QueryRow(context.Background(), "SELECT clock_timestamp()").Scan(&s.DB); err != nil {
			t.Fatal(err)
		}
		s.After = time.Now()
		samples = append(samples, s)
	}
	off, err := report.ClockOffset(samples)
	if err != nil {
		t.Fatal(err)
	}
	return off
}

// DefaultInt returns the default of an integer configuration key from the server's schema, the
// value a measured server runs with.
func DefaultInt(t testing.TB, key string) int {
	t.Helper()
	schema, err := wire.ConfigSchema()
	if err != nil {
		t.Fatal(err)
	}
	values, err := config.Load(schema, config.Sources{Env: []string{config.EnvName("server.public_url") + "=http://127.0.0.1:1"}})
	if err != nil {
		t.Fatal(err)
	}
	n, err := strconv.Atoi(values.Text(key))
	if err != nil {
		t.Fatalf("%s = %q: %v", key, values.Text(key), err)
	}
	return n
}

// DefaultDuration returns the default of a duration configuration key from the server's schema.
func DefaultDuration(t testing.TB, key string) time.Duration {
	t.Helper()
	schema, err := wire.ConfigSchema()
	if err != nil {
		t.Fatal(err)
	}
	values, err := config.Load(schema, config.Sources{Env: []string{config.EnvName("server.public_url") + "=http://127.0.0.1:1"}})
	if err != nil {
		t.Fatal(err)
	}
	d, err := time.ParseDuration(values.Text(key))
	if err != nil {
		t.Fatalf("%s = %q: %v", key, values.Text(key), err)
	}
	return d
}

// seedParallel is how many requests the seeding runs at once.
const seedParallel = 32

// Parallel runs f for 0..n-1 on seedParallel goroutines and fails t with the first error.
func Parallel(t testing.TB, n int, f func(ctx context.Context, i int) error) {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	var next atomic.Int64
	var once sync.Once
	var first error
	var wg sync.WaitGroup
	for range min(seedParallel, max(n, 1)) {
		wg.Go(func() {
			for {
				i := int(next.Add(1) - 1)
				if i >= n || ctx.Err() != nil {
					return
				}
				if err := f(ctx, i); err != nil {
					once.Do(func() { first = err; cancel() })
					return
				}
			}
		})
	}
	wg.Wait()
	if first != nil {
		t.Fatal(first)
	}
}

// clients numbers the simulated clients of a test binary, so each gets its own address.
var clients atomic.Int64

// ClientAddress returns the documentation address of simulated client i: its own /64 of
// 2001:db8::/32 (RFC 3849), because the server counts IPv6 clients by /64.
func ClientAddress(i int64) string { return fmt.Sprintf("2001:db8:%x:%x::1", i>>16, i&0xffff) }

// NewUsers creates n people with handles prefix-<i> through the public API (harness CreateUser).
// Each client then sends its requests from its own address (harness Client.ClientIP), as distinct
// clients behind the reverse proxy of a deployment do: the server limits failed credentials per
// client address (rl.public_auth), so a whole load from one address would measure that limit
// instead of the server.
func NewUsers(t testing.TB, p *harness.Process, prefix string, n int) []*harness.Client {
	t.Helper()
	out := make([]*harness.Client, n)
	Parallel(t, n, func(ctx context.Context, i int) error {
		c, err := p.CreateUser(ctx, harness.UserSpec{Handle: fmt.Sprintf("%s-%d", prefix, i), DisplayName: fmt.Sprintf("%s %d", prefix, i)})
		if c != nil {
			c.ClientIP = ClientAddress(clients.Add(1))
		}
		out[i] = c
		return err
	})
	return out
}

// Do sends r as c with a step-up when asked and returns the body; a status other than want is an
// error.
func Do(ctx context.Context, c *harness.Client, r harness.Request, want int) ([]byte, error) {
	resp, err := harness.DoWithStepUp(ctx, c, r)
	if err != nil {
		return nil, err
	}
	if resp.Status != want {
		return nil, fmt.Errorf("%s %s = %d %s", r.Method, r.Path, resp.Status, resp.Body)
	}
	return resp.Body, nil
}

// Space creates a space named name as the owner and returns its ID.
func Space(t testing.TB, p *harness.Process, name string) string {
	t.Helper()
	b, err := Do(t.Context(), p.Owner, harness.Request{Method: http.MethodPost, Path: "/spaces", Body: map[string]any{"name": name}}, http.StatusCreated)
	if err != nil {
		t.Fatal(err)
	}
	var s struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(b, &s); err != nil {
		t.Fatal(err)
	}
	return s.ID
}

// Channel is a text channel with its main thread.
type Channel struct{ ID, Thread string }

// CreateChannel creates the text channel name in space as the owner.
func CreateChannel(ctx context.Context, p *harness.Process, space, name string) (Channel, error) {
	b, err := Do(ctx, p.Owner, harness.Request{Method: http.MethodPost, Path: "/spaces/" + space + "/channels", Body: map[string]any{"name": name, "kind": "text"}}, http.StatusCreated)
	if err != nil {
		return Channel{}, err
	}
	var c struct {
		ID           string `json:"id"`
		MainThreadID string `json:"main_thread_id"`
	}
	if err := json.Unmarshal(b, &c); err != nil {
		return Channel{}, err
	}
	return Channel{ID: c.ID, Thread: c.MainThreadID}, nil
}

// Channels creates n text channels prefix-<i> in space.
func Channels(t testing.TB, p *harness.Process, space, prefix string, n int) []Channel {
	t.Helper()
	out := make([]Channel, n)
	Parallel(t, n, func(ctx context.Context, i int) error {
		c, err := CreateChannel(ctx, p, space, fmt.Sprintf("%s-%d", prefix, i))
		out[i] = c
		return err
	})
	return out
}

// Grant gives the person subject level ("read" or "write") on the target as the owner.
func Grant(ctx context.Context, p *harness.Process, subject, targetKind, target, level string) error {
	_, err := Do(ctx, p.Owner, harness.Request{Method: http.MethodPost, Path: "/grants", Body: map[string]any{
		"subject_kind": "user", "subject_id": subject, "target_kind": targetKind, "target_id": target, "level": level, "reason": "load test",
	}}, http.StatusCreated)
	return err
}

// ServerProcs returns the GOMAXPROCS value the measured server runs with.
func (e *Env) ServerProcs() int {
	if e.GOMAXPROCS > 0 {
		return e.GOMAXPROCS
	}
	return runtime.NumCPU()
}
