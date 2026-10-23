// SPDX-License-Identifier: Apache-2.0

package jobs_test

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/platform/jobs"
	"github.com/OWNER/bothy/services/server/internal/platform/jobs/internal/fakemodule"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
)

// uniqueRegistry registers fakemodule.echo and fakemodule.keyed with recorders on echo and keyed.
func uniqueRegistry(t *testing.T, echo chan<- jobs.Job[fakemodule.Echo], keyed chan<- jobs.Job[fakemodule.Keyed]) *jobs.Registry {
	t.Helper()
	reg := jobs.NewRegistry()
	if err := jobs.Register(reg, recorder(echo), jobs.HandlerOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := jobs.Register(reg, recorder(keyed), jobs.HandlerOptions{}); err != nil {
		t.Fatal(err)
	}
	return reg
}

// insertUnique runs InsertUniqueTx in a unit of work of its own.
func (e *env) insertUnique(t *testing.T, ins jobs.Inserter, args jobs.Args, at time.Time) jobs.InsertResult {
	t.Helper()
	var res jobs.InsertResult
	err := e.uow.Do(t.Context(), func(ctx context.Context) error {
		var err error
		res, err = ins.InsertUniqueTx(ctx, args, &jobs.InsertOptions{ScheduledAt: at})
		return err
	})
	if err != nil {
		t.Fatalf("InsertUniqueTx %s: %v", args.Kind(), err)
	}
	return res
}

// pending runs PendingTx in a unit of work of its own.
func (e *env) pending(t *testing.T, ins jobs.Inserter, args jobs.Args) (time.Time, bool) {
	t.Helper()
	var at time.Time
	var ok bool
	err := e.uow.Do(t.Context(), func(ctx context.Context) error {
		var err error
		at, ok, err = ins.PendingTx(ctx, args)
		return err
	})
	if err != nil {
		t.Fatalf("PendingTx %s: %v", args.Kind(), err)
	}
	return at, ok
}

// stamp is t in the precision of River's timestamptz columns.
func stamp(t time.Time) time.Time { return t.UTC().Truncate(time.Microsecond) }

// TestJobsUniqueInsertAndPending (M3-19 design note 9 a): a second unique insert of equal args
// does not insert and returns the standing job's time; other args insert; with a field tagged
// river:"unique" only that field counts; PendingTx finds the job until it completes and not after;
// outside a unit of work both are internal errors.
func TestJobsUniqueInsertAndPending(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	echo := make(chan jobs.Job[fakemodule.Echo], 4)
	keyed := make(chan jobs.Job[fakemodule.Keyed], 4)
	rt := e.runtime(t, testConfig(t, time.Second), jobs.RoleWorker, uniqueRegistry(t, echo, keyed))
	ins := rt.Inserter()
	later := stamp(time.Now().Add(time.Hour))

	// Equal args: the second insert stands back and reports the first job's time.
	target := ids.New()
	first := e.insertUnique(t, ins, fakemodule.Echo{Target: target, Seq: 1}, later)
	if !first.Inserted || !first.ScheduledAt.Equal(later) {
		t.Fatalf("first unique insert = %+v, want inserted at %s", first, later)
	}
	second := e.insertUnique(t, ins, fakemodule.Echo{Target: target, Seq: 1}, later.Add(time.Hour))
	if second.Inserted || !second.ScheduledAt.Equal(later) {
		t.Fatalf("second unique insert = %+v, want not inserted with the first time %s", second, later)
	}
	// Other args insert: without a tag every field counts.
	if other := e.insertUnique(t, ins, fakemodule.Echo{Target: target, Seq: 2}, later); !other.Inserted {
		t.Fatalf("unique insert of other args = %+v, want inserted", other)
	}
	if n := e.jobCount(t); n != 2 {
		t.Fatalf("%d jobs, want 2", n)
	}

	// A tagged field: args that differ only in Seq do not insert, another Key does.
	key := ids.New()
	if r := e.insertUnique(t, ins, fakemodule.Keyed{Key: key, Seq: 1}, later); !r.Inserted {
		t.Fatalf("first keyed insert = %+v, want inserted", r)
	}
	if r := e.insertUnique(t, ins, fakemodule.Keyed{Key: key, Seq: 2}, later.Add(time.Minute)); r.Inserted || !r.ScheduledAt.Equal(later) {
		t.Fatalf("keyed insert with another Seq = %+v, want not inserted at %s", r, later)
	}
	if r := e.insertUnique(t, ins, fakemodule.Keyed{Key: ids.New(), Seq: 1}, later); !r.Inserted {
		t.Fatalf("keyed insert with another Key = %+v, want inserted", r)
	}
	if at, ok := e.pending(t, ins, fakemodule.Keyed{Key: key, Seq: 99}); !ok || !at.Equal(later) {
		t.Fatalf("PendingTx(keyed, other Seq) = %s %v, want %s true", at, ok, later)
	}
	if at, ok := e.pending(t, ins, fakemodule.Echo{Target: target, Seq: 1}); !ok || !at.Equal(later) {
		t.Fatalf("PendingTx(echo) = %s %v, want %s true", at, ok, later)
	}
	if _, ok := e.pending(t, ins, fakemodule.Echo{Target: target, Seq: 3}); ok {
		t.Fatal("PendingTx(echo with other args) found a job")
	}

	// PendingTx finds a job until it completes, and a unique insert after that inserts again.
	// The insert and the read share one unit of work: River cannot see, run or finalize the job
	// before the commit, so the check holds on a machine of any speed.
	soon := stamp(time.Now())
	runTarget := ids.New()
	var res jobs.InsertResult
	var pendingAt time.Time
	var pendingOK bool
	if err := e.uow.Do(t.Context(), func(ctx context.Context) error {
		var err error
		if res, err = ins.InsertUniqueTx(ctx, fakemodule.Echo{Target: runTarget}, &jobs.InsertOptions{ScheduledAt: soon}); err != nil {
			return err
		}
		pendingAt, pendingOK, err = ins.PendingTx(ctx, fakemodule.Echo{Target: runTarget})
		return err
	}); err != nil {
		t.Fatalf("unique insert and PendingTx of the job that runs: %v", err)
	}
	if !res.Inserted {
		t.Fatalf("unique insert of the job that runs = %+v", res)
	}
	if !pendingOK || !pendingAt.Equal(soon) {
		t.Fatalf("PendingTx before the run = %s %v, want %s true", pendingAt, pendingOK, soon)
	}
	if j := receive(t, echo); j.Args.Target != runTarget {
		t.Fatalf("ran %+v, want the job of %s", j.Args, runTarget)
	}
	waitJobCompleted(t, e, runTarget)
	if _, ok := e.pending(t, ins, fakemodule.Echo{Target: runTarget}); ok {
		t.Fatal("PendingTx found a completed job")
	}
	if r := e.insertUnique(t, ins, fakemodule.Echo{Target: runTarget}, later); !r.Inserted || !r.ScheduledAt.Equal(later) {
		t.Fatalf("unique insert after completion = %+v, want inserted at %s", r, later)
	}

	// Outside a unit of work both refuse.
	if _, err := ins.InsertUniqueTx(context.Background(), fakemodule.Echo{Target: ids.New()}, nil); errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("InsertUniqueTx outside a unit = %v, want kind internal", err)
	}
	if _, _, err := ins.PendingTx(context.Background(), fakemodule.Echo{Target: target, Seq: 1}); errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("PendingTx outside a unit = %v, want kind internal", err)
	}
}

// waitJobCompleted waits until the echo job of target is completed in River's table.
func waitJobCompleted(t *testing.T, e *env, target ids.ID) {
	t.Helper()
	testkit.Until(t, "the job of "+target.String()+" completes", func() bool {
		var n int
		e.query(t, "SELECT count(*) FROM "+table("river_job")+" WHERE state = 'completed' AND args->>'target' = '"+target.String()+"'", &n)
		return n == 1
	})
}

// TestJobsUniqueRefusals: like InsertTx, both refuse the phase sync, an unregistered kind and
// arguments of another type with an internal error, and insert nothing.
func TestJobsUniqueRefusals(t *testing.T) {
	t.Parallel()
	var insertErr, pendingErr error
	var rtRef atomic.Pointer[jobs.Runtime]
	e := newEnv(t, func(reg *txn.Registry) {
		if err := reg.SetHook(txn.PhaseSync, fakemodule.Hook(func(ctx context.Context) error {
			ins := rtRef.Load().Inserter()
			_, insertErr = ins.InsertUniqueTx(ctx, fakemodule.Echo{Target: ids.New()}, nil)
			_, _, pendingErr = ins.PendingTx(ctx, fakemodule.Echo{Target: ids.New()})
			return nil
		})); err != nil {
			t.Fatal(err)
		}
	})
	rt, err := jobs.New(e.pool, testConfig(t, time.Second), jobs.RoleAPI, uniqueRegistry(t, make(chan jobs.Job[fakemodule.Echo]), make(chan jobs.Job[fakemodule.Keyed])), nil)
	if err != nil {
		t.Fatal(err)
	}
	rtRef.Store(rt)
	if err := e.uow.Do(t.Context(), func(context.Context) error { return nil }); err != nil {
		t.Fatal(err)
	}
	for name, err := range map[string]error{"InsertUniqueTx": insertErr, "PendingTx": pendingErr} {
		if errs.KindOf(err) != errs.KindInternal || !strings.Contains(err.Error(), "sync") {
			t.Errorf("%s in phase sync = %v, want kind internal naming the phase", name, err)
		}
	}
	for _, args := range []jobs.Args{fakemodule.Tick{}, &fakemodule.Echo{Target: ids.New()}, nil} {
		err := e.uow.Do(t.Context(), func(ctx context.Context) error {
			_, err := rt.Inserter().InsertUniqueTx(ctx, args, nil)
			return err
		})
		if errs.KindOf(err) != errs.KindInternal {
			t.Errorf("InsertUniqueTx(%T) = %v, want kind internal", args, err)
		}
		err = e.uow.Do(t.Context(), func(ctx context.Context) error {
			_, _, err := rt.Inserter().PendingTx(ctx, args)
			return err
		})
		if errs.KindOf(err) != errs.KindInternal {
			t.Errorf("PendingTx(%T) = %v, want kind internal", args, err)
		}
	}
	if n := e.jobCount(t); n != 0 {
		t.Fatalf("%d jobs, want 0", n)
	}
}

// TestJobsLatestPendingTx (M3-19, D-M3-19-7): LatestPendingTx compares only the named fields and
// returns the latest of the standing jobs, so a marker keyed by its end time is found by its owner
// alone; an unknown field name and a call outside a unit of work are internal errors.
func TestJobsLatestPendingTx(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	echo := make(chan jobs.Job[fakemodule.Echo], 4)
	keyed := make(chan jobs.Job[fakemodule.Keyed], 4)
	rt := e.runtime(t, testConfig(t, time.Second), jobs.RoleWorker, uniqueRegistry(t, echo, keyed))
	ins := rt.Inserter()
	later := stamp(time.Now().Add(time.Hour))
	target := ids.New()
	for i, at := range []time.Time{later, later.Add(time.Hour)} {
		if r := e.insertUnique(t, ins, fakemodule.Echo{Target: target, Seq: int64(i + 1)}, at); !r.Inserted {
			t.Fatalf("unique insert %d = %+v, want inserted", i+1, r)
		}
	}
	latest := func(args jobs.Args, fields ...string) (time.Time, bool, error) {
		var at time.Time
		var ok bool
		err := e.uow.Do(t.Context(), func(ctx context.Context) error {
			var err error
			at, ok, err = ins.LatestPendingTx(ctx, args, fields...)
			return err
		})
		return at, ok, err
	}
	if at, ok, err := latest(fakemodule.Echo{Target: target}, "target"); err != nil || !ok || !at.Equal(later.Add(time.Hour)) {
		t.Fatalf("LatestPendingTx(target) = %s %v %v, want %s true", at, ok, err, later.Add(time.Hour))
	}
	if _, ok, err := latest(fakemodule.Echo{Target: ids.New()}, "target"); err != nil || ok {
		t.Fatalf("LatestPendingTx(other target) = %v %v, want none", ok, err)
	}
	if _, _, err := latest(fakemodule.Echo{Target: target}, "nope"); errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("LatestPendingTx with an unknown field = %v, want kind internal", err)
	}
	if _, _, err := latest(fakemodule.Echo{Target: target}); errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("LatestPendingTx without a field = %v, want kind internal", err)
	}
	if _, _, err := ins.LatestPendingTx(context.Background(), fakemodule.Echo{Target: target}, "target"); errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("LatestPendingTx outside a unit = %v, want kind internal", err)
	}
}
