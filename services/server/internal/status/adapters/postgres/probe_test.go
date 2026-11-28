// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
	"github.com/OWNER/bothy/services/server/internal/status/adapters/postgres"
	"github.com/OWNER/bothy/services/server/internal/status/app"
	"github.com/OWNER/bothy/services/server/internal/status/domain"
)

// noEvents accepts status.updated without writing it: the test reads the stored states.
type noEvents struct{}

func (noEvents) Append(context.Context, ...eventlog.Event) error { return nil }

// noAgents lists no agent.
type noAgents struct{}

func (noAgents) ActiveAgents(context.Context) ([]app.ActiveAgent, error) { return nil, nil }

// outcomes are the shared outcomes of the checks of both probes.
type outcomes struct {
	mu  sync.Mutex
	api domain.Outcome
}

func (o *outcomes) set(v domain.Outcome) { o.mu.Lock(); o.api = v; o.mu.Unlock() }

func (o *outcomes) source() app.Source {
	return app.SourceFunc(func(context.Context, app.Window) domain.Outcome {
		o.mu.Lock()
		defer o.mu.Unlock()
		return o.api
	})
}

// TestProbe_TwoWorkersShareTheMemory holds AUD srv-stats-status-11: two workers run the rounds of
// status.probe on one database. Worker A sees a failing round, worker B a passing one, and A a
// failing one status.degrade_after after its first. With a memory per process A still held the
// start of its run of failing rounds and set degraded, although the check did not fail
// throughout; with the memory in status_component_probe B's passing round ended the run, so the
// component stays operational until the checks fail for status.degrade_after without a pass.
func TestProbe_TwoWorkersShareTheMemory(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := pgtest.New(t)
	pool := db.Pool(t, pg.RoleApp)
	reg := txn.NewRegistry()
	reg.Seal()
	uow := pg.NewUnitOfWork(pool, pg.UnitConfig{StatementTimeout: time.Minute, LockTimeout: time.Minute, IdleInTxTimeout: time.Minute}, reg)
	clk := clock.NewFake(time.Date(2026, 11, 28, 12, 0, 0, 0, time.UTC))
	cfg := domain.ProbeConfig{Interval: 30 * time.Second, DegradeAfter: 90 * time.Second, IncidentAfter: 5 * time.Minute, ResolveAfter: 5 * time.Minute}
	out := &outcomes{api: domain.Pass}
	newProbe := func() *app.Probe {
		p, err := app.NewProbe(cfg, app.Sources{Checks: map[string]app.Source{domain.SourceAPIHTTP: out.source()}, Agents: noAgents{}},
			app.ProbeDeps{Store: postgres.NewPage(pool), UoW: uow, Events: noEvents{}, Clock: clk})
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	a, b := newProbe(), newProbe()
	round := func(p *app.Probe, o domain.Outcome) {
		t.Helper()
		out.set(o)
		if err := p.Run(ctx); err != nil {
			t.Fatal(err)
		}
		clk.Advance(cfg.Interval)
	}
	state := func() string {
		t.Helper()
		var s string
		if err := pool.QueryRow(ctx, `SELECT state FROM status_components WHERE health_source = $1`, domain.GroupAPI).Scan(&s); err != nil {
			t.Fatal(err)
		}
		return s
	}

	round(a, domain.Pass) // creates the standard components
	round(a, domain.Fail) // A: the run of failing rounds begins
	round(b, domain.Pass) // B: a pass ends it
	round(a, domain.Fail) // A: degrade_after after its first failing round
	if got := state(); got != "operational" {
		t.Fatalf("after fail, pass, fail on two workers: %s, want operational", got)
	}
	// Failing rounds without a pass, alternating between the workers, degrade once they lasted
	// status.degrade_after.
	round(b, domain.Fail)
	round(a, domain.Fail)
	if got := state(); got != "operational" {
		t.Fatalf("before degrade_after of failing rounds: %s", got)
	}
	round(b, domain.Fail)
	if got := state(); got != "degraded" {
		t.Fatalf("after degrade_after of failing rounds on both workers: %s, want degraded", got)
	}
}
