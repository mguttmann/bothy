// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
)

// TestBackupAndExpireUsage holds the command lines of backup and expire: unknown kinds, missing
// flags and a configuration that does not load are usage errors (2), before anything runs.
func TestBackupAndExpireUsage(t *testing.T) {
	t.Parallel()
	// No server.public_url: the configuration does not load.
	env := []string{"BOTHY_SECRETS__DIR=/run/bothy/secrets"}
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"backup"}, "unknown kind"},
		{[]string{"backup", "restore_test"}, "unknown kind"},
		{[]string{"backup", "full"}, "unknown kind"},
		{[]string{"backup", "pg_full", "extra"}, "unexpected argument"},
		{[]string{"backup", "pg_full", "--bogus"}, "flag provided but not defined"},
		{[]string{"backup", "pg_full"}, "server.public_url"},
		{[]string{"expire"}, "only --offsite exists"},
		{[]string{"expire", "--offsite"}, "--credentials needs"},
		{[]string{"expire", "--offsite", "--credentials", "/tmp/x", "extra"}, "unexpected argument"},
		{[]string{"expire", "--offsite", "--credentials", "/tmp/x"}, "server.public_url"},
	} {
		var stdout, stderr bytes.Buffer
		if got := run(context.Background(), tc.args, env, &stdout, &stderr); got != exitUsage {
			t.Errorf("run(%q) = %d, want %d; stderr %s", tc.args, got, exitUsage, stderr.String())
		}
		if !strings.Contains(stderr.String(), tc.want) {
			t.Errorf("run(%q) stderr does not name %q:\n%s", tc.args, tc.want, stderr.String())
		}
	}
}

// TestServeNeedsConfiguration: the default command loads the configuration before it starts
// anything; a configuration that does not load is exit code 2.
func TestServeNeedsConfiguration(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	if got := run(context.Background(), []string{"--retry", "1s"}, []string{"BOTHY_SECRETS__DIR=/run/bothy/secrets"}, &stdout, &stderr); got != exitUsage {
		t.Fatalf("run = %d, want %d; stderr %s", got, exitUsage, stderr.String())
	}
	if !strings.Contains(stderr.String(), "server.public_url") {
		t.Errorf("stderr does not name the missing key:\n%s", stderr.String())
	}
}

// TestConfigEnvDropsImageVariables: the entry point's own variables never reach the
// configuration, every other variable does.
func TestConfigEnvDropsImageVariables(t *testing.T) {
	t.Parallel()
	got := configEnv([]string{"BOTHY_PGBACKREST_PG_HOST=postgres", "BOTHY_PG_ARCHIVE_TIMEOUT=60s",
		"BOTHY_PGBACKREST_REPO_HOST=backup", "BOTHY_PGBACKREST_REPO2=on", "BOTHY_BACKUP__OFFSITE__TARGET=", "BOTHY_PGBACKREST_TYPO=x", "PATH=/bin"})
	want := []string{"BOTHY_BACKUP__OFFSITE__TARGET=", "BOTHY_PGBACKREST_TYPO=x", "PATH=/bin"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("configEnv = %q, want %q", got, want)
	}
}

// fakeTarget records what the scheduler loop drives.
type fakeTarget struct {
	mu      sync.Mutex
	primed  int
	samples int
	ticks   int
	start   time.Time
}

func (f *fakeTarget) Prime(context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.primed++
	return nil
}

func (f *fakeTarget) Sample(context.Context) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.samples++
}

func (f *fakeTarget) NewScheduler(start time.Time) ticker {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.start = start
	return &countTicker{target: f}
}

// countTicker counts the ticks of the loop.
type countTicker struct{ target *fakeTarget }

func (c *countTicker) Tick(context.Context, time.Time) []string {
	c.target.mu.Lock()
	defer c.target.mu.Unlock()
	c.target.ticks++
	return nil
}

func (c *countTicker) Wait() {}

// TestScheduleWaitsForMinuteBoundaries: the loop primes the gauges, creates the scheduler with
// the start time (so nothing before it is caught up), samples once at start and then waits for
// the start of each next minute of the clock. The fake wait moves a fake clock instead of
// sleeping; the scheduler itself is tested in internal/status/app.
func TestScheduleWaitsForMinuteBoundaries(t *testing.T) {
	t.Parallel()
	start := time.Date(2026, 10, 18, 1, 59, 30, 0, time.UTC)
	clk := clock.NewFake(start)
	target := &fakeTarget{}
	var waits []time.Time
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	wait := func(_ context.Context, _ clock.Clock, at time.Time) bool {
		waits = append(waits, at)
		if len(waits) == 3 {
			return false
		}
		clk.Set(at)
		return true
	}
	schedule(ctx, target, clk, wait)
	want := []time.Time{
		time.Date(2026, 10, 18, 2, 0, 0, 0, time.UTC),
		time.Date(2026, 10, 18, 2, 1, 0, 0, time.UTC),
		time.Date(2026, 10, 18, 2, 2, 0, 0, time.UTC),
	}
	if len(waits) != len(want) {
		t.Fatalf("waits %v, want %v", waits, want)
	}
	for i := range want {
		if !waits[i].Equal(want[i]) {
			t.Errorf("wait %d until %v, want %v", i, waits[i], want[i])
		}
	}
	if !target.start.Equal(start) {
		t.Errorf("scheduler started at %v, want the process start %v", target.start, start)
	}
	if target.primed != 1 || target.samples != 3 || target.ticks != 2 {
		t.Errorf("primed %d, samples %d, ticks %d; want 1, 3 (start and two ticks), 2", target.primed, target.samples, target.ticks)
	}
}

// TestSleepUntil: an instant in the past returns at once, a canceled context returns false.
func TestSleepUntil(t *testing.T) {
	t.Parallel()
	clk := clock.NewFake(time.Date(2026, 10, 18, 2, 0, 0, 0, time.UTC))
	if !sleepUntil(context.Background(), clk, clk.Now().Add(-time.Second)) {
		t.Error("sleepUntil of a past instant = false")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if sleepUntil(ctx, clk, clk.Now().Add(time.Hour)) {
		t.Error("sleepUntil with a canceled context = true")
	}
}
