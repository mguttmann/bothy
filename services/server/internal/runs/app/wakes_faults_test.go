// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"cmp"
	"context"
	"errors"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/runs"
	"github.com/OWNER/bothy/services/server/internal/runs/app"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// wakeRepo is the fake WakeRepository: the wake-ups and the autonomy counts in memory.
type wakeRepo struct {
	mu       sync.Mutex
	wakes    []app.WakeRow
	autonomy map[[2]ids.ID]domain.Autonomy
}

func (r *wakeRepo) find(keep func(app.WakeRow) bool) (app.WakeRow, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, w := range r.wakes {
		if keep(w) {
			return w, true
		}
	}
	return app.WakeRow{}, false
}

func (r *wakeRepo) InsertWake(_ context.Context, n app.NewWake) (app.WakeRow, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, w := range r.wakes {
		if w.SourceRunID == n.SourceRunID && n.Outage != 0 && w.Outage == n.Outage {
			return app.WakeRow{}, false, nil
		}
	}
	row := app.WakeRow{
		ID: n.ID, AgentID: n.AgentID, ThreadID: n.ThreadID, SourceRunID: n.SourceRunID, TriggeredBy: n.TriggeredBy, Kind: n.Kind,
		Reason: n.Reason, State: n.State, StateReason: n.StateReason, Condition: n.Condition, Label: n.Label, Outage: n.Outage,
		NotBefore: n.NotBefore, Deadline: n.Deadline, CreatedAt: n.CreatedAt, Version: 1,
	}
	r.wakes = append(r.wakes, row)
	return row, true, nil
}

func (r *wakeRepo) Wake(_ context.Context, id ids.ID) (app.WakeRow, bool, error) {
	w, ok := r.find(func(w app.WakeRow) bool { return w.ID == id })
	return w, ok, nil
}

func (r *wakeRepo) LockWake(ctx context.Context, id ids.ID) (app.WakeRow, bool, error) {
	return r.Wake(ctx, id)
}

func (r *wakeRepo) WakeByOutage(_ context.Context, run ids.ID, outage int) (app.WakeRow, bool, error) {
	w, ok := r.find(func(w app.WakeRow) bool { return w.SourceRunID == run && outage != 0 && w.Outage == outage })
	return w, ok, nil
}

func (r *wakeRepo) WakeBySourceReason(_ context.Context, run ids.ID, reason domain.WakeReason) (app.WakeRow, bool, error) {
	w, ok := r.find(func(w app.WakeRow) bool { return w.SourceRunID == run && w.Reason == reason && w.Outage == 0 })
	return w, ok, nil
}

func (r *wakeRepo) PendingUsageWakeOfRun(_ context.Context, run ids.ID) (app.WakeRow, bool, error) {
	w, ok := r.find(func(w app.WakeRow) bool {
		return w.SourceRunID == run && w.Reason == domain.WakeReasonUsageLimit && w.Outage != 0 && w.State == domain.WakePending
	})
	return w, ok, nil
}

func (r *wakeRepo) UpdateWake(_ context.Context, u app.WakeWrite) (app.WakeRow, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i, w := range r.wakes {
		if w.ID != u.ID {
			continue
		}
		if w.Version != u.Version {
			return app.WakeRow{}, false, nil
		}
		w.State, w.StateReason, w.Version = u.State, u.StateReason, w.Version+1
		if w.State != domain.WakePending {
			w.EndedAt = u.At
		}
		if !u.NotBefore.IsZero() {
			w.NotBefore, w.Deadline = u.NotBefore, u.Deadline
		}
		w.ResultText = cmp.Or(u.ResultText, w.ResultText)
		w.FiredRunID, w.FiredIntoRunID = cmp.Or(u.FiredRunID, w.FiredRunID), cmp.Or(u.FiredIntoRunID, w.FiredIntoRunID)
		if u.ClearFired {
			w.FiredRunID, w.FiredIntoRunID = ids.ID{}, ids.ID{}
		}
		r.wakes[i] = w
		return w, true, nil
	}
	return app.WakeRow{}, false, nil
}

func (r *wakeRepo) count(keep func(app.WakeRow) bool) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for _, w := range r.wakes {
		if keep(w) {
			n++
		}
	}
	return n
}

func (r *wakeRepo) CountPendingWakes(_ context.Context, thread ids.ID) (int, error) {
	return r.count(func(w app.WakeRow) bool { return w.ThreadID == thread && w.State == domain.WakePending }), nil
}

func (r *wakeRepo) CountUsageWakes(_ context.Context, run ids.ID) (int, error) {
	return r.count(func(w app.WakeRow) bool { return w.SourceRunID == run && w.Reason == domain.WakeReasonUsageLimit }), nil
}

func (r *wakeRepo) all(keep func(app.WakeRow) bool) []app.WakeRow {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []app.WakeRow
	for _, w := range r.wakes {
		if keep(w) {
			out = append(out, w)
		}
	}
	return out
}

func (r *wakeRepo) PendingWakesOfAgent(_ context.Context, agent ids.ID) ([]app.WakeRow, error) {
	return r.all(func(w app.WakeRow) bool { return w.AgentID == agent && w.State == domain.WakePending }), nil
}

func (r *wakeRepo) ThreadWakes(_ context.Context, thread ids.ID, after *app.WakeKey, limit int) ([]app.WakeRow, error) {
	out := r.all(func(w app.WakeRow) bool {
		return w.ThreadID == thread && (after == nil || w.CreatedAt.Before(after.CreatedAt) || (w.CreatedAt.Equal(after.CreatedAt) && compareIDs(w.ID, after.ID) < 0))
	})
	slices.SortFunc(out, func(a, b app.WakeRow) int {
		return cmp.Or(b.CreatedAt.Compare(a.CreatedAt), compareIDs(b.ID, a.ID))
	})
	return out[:min(len(out), limit)], nil
}

func (r *wakeRepo) ReadyWakesOfThread(_ context.Context, thread ids.ID, now time.Time) ([]ids.ID, error) {
	var out []ids.ID
	for _, w := range r.all(func(w app.WakeRow) bool { return w.ThreadID == thread && w.State == domain.WakePending }) {
		if (w.Kind == domain.WakeAt && !now.Before(w.NotBefore)) || w.ResultText != "" {
			out = append(out, w.ID)
		}
	}
	return out, nil
}

func (r *wakeRepo) PendingWakeCounts(context.Context) (map[domain.WakeKind]int64, error) {
	m := map[domain.WakeKind]int64{}
	for _, w := range r.all(func(w app.WakeRow) bool { return w.State == domain.WakePending }) {
		m[w.Kind]++
	}
	return m, nil
}

func (r *wakeRepo) LockAutonomy(_ context.Context, thread, person ids.ID) (domain.Autonomy, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	a, ok := r.autonomy[[2]ids.ID{thread, person}]
	return a, ok, nil
}

func (r *wakeRepo) SaveAutonomy(_ context.Context, thread, person ids.ID, a domain.Autonomy) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.autonomy == nil {
		r.autonomy = map[[2]ids.ID]domain.Autonomy{}
	}
	r.autonomy[[2]ids.ID{thread, person}] = a
	return nil
}

func (r *wakeRepo) ResetAutonomy(_ context.Context, thread, person ids.ID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.autonomy[[2]ids.ID{thread, person}]; ok {
		r.autonomy[[2]ids.ID{thread, person}] = domain.Autonomy{}
	}
	return nil
}

// compareIDs orders ids bytewise.
func compareIDs(a, b ids.ID) int { return slices.Compare(a[:], b[:]) }

// wakeJobs, wakeNotes and wakeOwners are the fake scheduler, notes and owners.
type wakeJobs struct{ j *journal }

func (s wakeJobs) ScheduleWakeDue(_ context.Context, id ids.ID, at time.Time) error {
	s.j.add("wake_due", id)
	return nil
}

type wakeNotes struct{ j *journal }

func (n wakeNotes) AppendSystemNote(_ context.Context, thread, _ ids.ID, text string) (ids.ID, int64, error) {
	n.j.add("note", text)
	return ids.New(), 7, nil
}

type wakeOwners struct{ owner bool }

func (o wakeOwners) IsActiveOwner(context.Context, ids.ID) (bool, error) { return o.owner, nil }

// gatedWakeRepo, gatedWakeJobs, gatedWakeNotes and gatedOwners put the wake ports behind the gate.
type gatedWakeRepo struct {
	app.WakeRepository
	g *faultGate
}

func (r gatedWakeRepo) InsertWake(ctx context.Context, n app.NewWake) (app.WakeRow, bool, error) {
	if err := r.g.hit(); err != nil {
		return app.WakeRow{}, false, err
	}
	return r.WakeRepository.InsertWake(ctx, n)
}

func (r gatedWakeRepo) Wake(ctx context.Context, id ids.ID) (app.WakeRow, bool, error) {
	if err := r.g.hit(); err != nil {
		return app.WakeRow{}, false, err
	}
	return r.WakeRepository.Wake(ctx, id)
}

func (r gatedWakeRepo) LockWake(ctx context.Context, id ids.ID) (app.WakeRow, bool, error) {
	if err := r.g.hit(); err != nil {
		return app.WakeRow{}, false, err
	}
	return r.WakeRepository.LockWake(ctx, id)
}

func (r gatedWakeRepo) WakeByOutage(ctx context.Context, run ids.ID, outage int) (app.WakeRow, bool, error) {
	if err := r.g.hit(); err != nil {
		return app.WakeRow{}, false, err
	}
	return r.WakeRepository.WakeByOutage(ctx, run, outage)
}

func (r gatedWakeRepo) WakeBySourceReason(ctx context.Context, run ids.ID, reason domain.WakeReason) (app.WakeRow, bool, error) {
	if err := r.g.hit(); err != nil {
		return app.WakeRow{}, false, err
	}
	return r.WakeRepository.WakeBySourceReason(ctx, run, reason)
}

func (r gatedWakeRepo) PendingUsageWakeOfRun(ctx context.Context, run ids.ID) (app.WakeRow, bool, error) {
	if err := r.g.hit(); err != nil {
		return app.WakeRow{}, false, err
	}
	return r.WakeRepository.PendingUsageWakeOfRun(ctx, run)
}

func (r gatedWakeRepo) UpdateWake(ctx context.Context, u app.WakeWrite) (app.WakeRow, bool, error) {
	if err := r.g.hit(); err != nil {
		return app.WakeRow{}, false, err
	}
	return r.WakeRepository.UpdateWake(ctx, u)
}

func (r gatedWakeRepo) CountPendingWakes(ctx context.Context, thread ids.ID) (int, error) {
	if err := r.g.hit(); err != nil {
		return 0, err
	}
	return r.WakeRepository.CountPendingWakes(ctx, thread)
}

func (r gatedWakeRepo) CountUsageWakes(ctx context.Context, run ids.ID) (int, error) {
	if err := r.g.hit(); err != nil {
		return 0, err
	}
	return r.WakeRepository.CountUsageWakes(ctx, run)
}

func (r gatedWakeRepo) PendingWakesOfAgent(ctx context.Context, agent ids.ID) ([]app.WakeRow, error) {
	if err := r.g.hit(); err != nil {
		return nil, err
	}
	return r.WakeRepository.PendingWakesOfAgent(ctx, agent)
}

func (r gatedWakeRepo) ThreadWakes(ctx context.Context, thread ids.ID, after *app.WakeKey, limit int) ([]app.WakeRow, error) {
	if err := r.g.hit(); err != nil {
		return nil, err
	}
	return r.WakeRepository.ThreadWakes(ctx, thread, after, limit)
}

func (r gatedWakeRepo) ReadyWakesOfThread(ctx context.Context, thread ids.ID, now time.Time) ([]ids.ID, error) {
	if err := r.g.hit(); err != nil {
		return nil, err
	}
	return r.WakeRepository.ReadyWakesOfThread(ctx, thread, now)
}

func (r gatedWakeRepo) LockAutonomy(ctx context.Context, thread, person ids.ID) (domain.Autonomy, bool, error) {
	if err := r.g.hit(); err != nil {
		return domain.Autonomy{}, false, err
	}
	return r.WakeRepository.LockAutonomy(ctx, thread, person)
}

func (r gatedWakeRepo) SaveAutonomy(ctx context.Context, thread, person ids.ID, a domain.Autonomy) error {
	if err := r.g.hit(); err != nil {
		return err
	}
	return r.WakeRepository.SaveAutonomy(ctx, thread, person, a)
}

type gatedWakeJobs struct {
	app.WakeScheduler
	g *faultGate
}

func (s gatedWakeJobs) ScheduleWakeDue(ctx context.Context, id ids.ID, at time.Time) error {
	if err := s.g.hit(); err != nil {
		return err
	}
	return s.WakeScheduler.ScheduleWakeDue(ctx, id, at)
}

type gatedWakeNotes struct {
	app.WakeNotes
	g *faultGate
}

func (n gatedWakeNotes) AppendSystemNote(ctx context.Context, thread, author ids.ID, text string) (ids.ID, int64, error) {
	if err := n.g.hit(); err != nil {
		return ids.ID{}, 0, err
	}
	return n.WakeNotes.AppendSystemNote(ctx, thread, author, text)
}

type gatedOwners struct {
	app.Owners
	g *faultGate
}

func (o gatedOwners) IsActiveOwner(ctx context.Context, person ids.ID) (bool, error) {
	if err := o.g.hit(); err != nil {
		return false, err
	}
	return o.Owners.IsActiveOwner(ctx, person)
}

// wakeWorld is a world with bound wake-ups.
type wakeWorld struct {
	*world
	wakes *wakeRepo
	owner bool
}

// bind binds the wake-ups of the current build of w, behind the gate of w when it is set.
func (ww *wakeWorld) bind(t *testing.T) {
	t.Helper()
	d := app.WakeDeps{Repo: ww.wakes, Schedule: wakeJobs{ww.j}, Notes: wakeNotes{ww.j}, Owners: wakeOwners{ww.owner}}
	if g := ww.gate; g != nil {
		d = app.WakeDeps{Repo: gatedWakeRepo{d.Repo, g}, Schedule: gatedWakeJobs{d.Schedule, g}, Notes: gatedWakeNotes{d.Notes, g},
			Owners: gatedOwners{d.Owners, g}}
	}
	cfg := app.WakeConfig{MaxPending: 5, MaxWait: 7 * 24 * time.Hour, MinDelay: time.Minute, UsageWakeMax: 20, LabelMaxChars: 120,
		ChainMax: 25, IdleMax: 3}
	if err := ww.svc.BindWakes(cfg, d); err != nil {
		t.Fatal(err)
	}
}

// agentCtx is the context of Lotse.
func (ww *wakeWorld) agentCtx() context.Context {
	return actor.WithActor(context.Background(), actor.Actor{ID: ww.lotse, Kind: actor.KindAgent})
}

// running opens the run of Lotse in the thread in state.
func (ww *wakeWorld) running(t *testing.T, state domain.State) app.RunRow {
	t.Helper()
	openThreadRun(t, ww.world)
	ww.repo.runs[0].State = state
	return ww.repo.runs[0]
}

// pending stores a pending wake-up of run with the given kind, due now.
func (ww *wakeWorld) pending(run app.RunRow, kind domain.WakeKind, mut func(*app.WakeRow)) app.WakeRow {
	now := ww.clk.Now()
	w := app.WakeRow{
		ID: ids.New(), AgentID: run.AgentID, ThreadID: run.ThreadID, SourceRunID: run.ID, TriggeredBy: run.TriggeredBy, Kind: kind,
		Reason: domain.WakeReasonAgent, State: domain.WakePending, Condition: []byte("{}"), Label: "wake",
		NotBefore: now.Add(-time.Minute), Deadline: now.Add(time.Hour), CreatedAt: now.Add(-2 * time.Minute), Version: 1,
	}
	if mut != nil {
		mut(&w)
	}
	ww.wakes.mu.Lock()
	ww.wakes.wakes = append(ww.wakes.wakes, w)
	ww.wakes.mu.Unlock()
	return w
}

// refusalOK passes an injected error on and turns the 409 of a refusal into success.
func refusalOK(err error) error {
	if err != nil && !errors.Is(err, errInjected) && errs.KindOf(err) == errs.KindConflict {
		return nil
	}
	return err
}

// TestWakes_InfrastructureErrorsReachTheCaller: every wake-up operation (create, a repeated
// usage-limit request, a stored refusal, recover, check, fire as a wake run, as a wake input,
// failed and skipped, cancel, the lists, the launch check and the job runs.wake_due) returns an
// infrastructure error of any port it calls to its caller, so the unit of work rolls back (02
// section 8.3): the gate fails the first, second, ... call until a run passes without reaching it.
func TestWakes_InfrastructureErrorsReachTheCaller(t *testing.T) {
	t.Parallel()
	type wakeCase struct {
		name     string
		notOwner bool
		setup    func(t *testing.T, ww *wakeWorld) func() error
	}
	cases := []wakeCase{
		{name: "create", setup: func(t *testing.T, ww *wakeWorld) func() error {
			run := ww.running(t, domain.StateRunning)
			return func() error {
				_, err := ww.svc.Wakes().Create(ww.agentCtx(), runs.CreateWake{RunID: run.ID, Kind: "at", Reason: "agent", Label: "later",
					NotBefore: ww.clk.Now().Add(10 * time.Minute)})
				return err
			}
		}},
		{name: "create of a known outage", setup: func(t *testing.T, ww *wakeWorld) func() error {
			run := ww.running(t, domain.StateRunning)
			ww.pending(run, domain.WakeAt, func(w *app.WakeRow) { w.Reason, w.Outage = domain.WakeReasonUsageLimit, 1 })
			return func() error {
				_, err := ww.svc.Wakes().Create(ww.agentCtx(), runs.CreateWake{RunID: run.ID, Kind: "at", Reason: "usage_limit", Label: "limit",
					Outage: 1, NotBefore: ww.clk.Now().Add(3 * time.Hour)})
				return err
			}
		}},
		{name: "create refused and stored", notOwner: true, setup: func(t *testing.T, ww *wakeWorld) func() error {
			run := ww.running(t, domain.StateRunning)
			return func() error {
				_, err := ww.svc.Wakes().Create(ww.agentCtx(), runs.CreateWake{RunID: run.ID, Kind: "at", Reason: "agent", Label: "later",
					Deferred: true})
				return refusalOK(err)
			}
		}},
		{name: "recover", setup: func(t *testing.T, ww *wakeWorld) func() error {
			run := ww.running(t, domain.StateFailed)
			return func() error {
				_, err := ww.svc.Wakes().Recover(ww.agentCtx(), runs.RecoverWake{RunID: run.ID, Reason: "recovery", Label: "lost work"})
				return err
			}
		}},
		{name: "recover refused", notOwner: true, setup: func(t *testing.T, ww *wakeWorld) func() error {
			run := ww.running(t, domain.StateFailed)
			return func() error {
				_, err := ww.svc.Wakes().Recover(ww.agentCtx(), runs.RecoverWake{RunID: run.ID, Reason: "recovery", Label: "lost work"})
				return refusalOK(err)
			}
		}},
		{name: "check", setup: func(t *testing.T, ww *wakeWorld) func() error {
			w := ww.pending(ww.running(t, domain.StateCompleted), domain.WakePRChecks, nil)
			return func() error { _, err := ww.svc.Wakes().Check(ww.agentCtx(), w.ID); return err }
		}},
		{name: "check refused", notOwner: true, setup: func(t *testing.T, ww *wakeWorld) func() error {
			w := ww.pending(ww.running(t, domain.StateCompleted), domain.WakePRChecks, nil)
			return func() error { _, err := ww.svc.Wakes().Check(ww.agentCtx(), w.ID); return refusalOK(err) }
		}},
		{name: "fire as a wake run", setup: func(t *testing.T, ww *wakeWorld) func() error {
			w := ww.pending(ww.running(t, domain.StateCompleted), domain.WakePRChecks, nil)
			return func() error {
				_, err := ww.svc.Wakes().Fire(ww.agentCtx(), runs.FireWake{WakeID: w.ID, Outcome: "fired", ResultText: "checks green"})
				return err
			}
		}},
		{name: "fire as a wake input", setup: func(t *testing.T, ww *wakeWorld) func() error {
			w := ww.pending(ww.running(t, domain.StateRunning), domain.WakePRChecks, nil)
			return func() error {
				_, err := ww.svc.Wakes().Fire(ww.agentCtx(), runs.FireWake{WakeID: w.ID, Outcome: "fired", ResultText: "checks green"})
				return err
			}
		}},
		{name: "fire refused", notOwner: true, setup: func(t *testing.T, ww *wakeWorld) func() error {
			w := ww.pending(ww.running(t, domain.StateCompleted), domain.WakeCommand, nil)
			return func() error {
				_, err := ww.svc.Wakes().Fire(ww.agentCtx(), runs.FireWake{WakeID: w.ID, Outcome: "fired", ResultText: "done"})
				return err
			}
		}},
		{name: "fire failed", setup: func(t *testing.T, ww *wakeWorld) func() error {
			w := ww.pending(ww.running(t, domain.StateCompleted), domain.WakeCommand, nil)
			return func() error {
				_, err := ww.svc.Wakes().Fire(ww.agentCtx(), runs.FireWake{WakeID: w.ID, Outcome: "failed"})
				return err
			}
		}},
		{name: "fire input skipped", setup: func(t *testing.T, ww *wakeWorld) func() error {
			run := ww.running(t, domain.StateRunning)
			w := ww.pending(run, domain.WakeCommand, func(w *app.WakeRow) { w.State, w.FiredIntoRunID = domain.WakeFired, run.ID })
			return func() error {
				_, err := ww.svc.Wakes().Fire(ww.agentCtx(), runs.FireWake{WakeID: w.ID, Outcome: "input_skipped", RunID: run.ID})
				return err
			}
		}},
		{name: "cancel", setup: func(t *testing.T, ww *wakeWorld) func() error {
			w := ww.pending(ww.running(t, domain.StateRunning), domain.WakeAt, nil)
			return func() error {
				_, err := ww.svc.Wakes().Cancel(ww.agentCtx(), runs.CancelWake{WakeID: w.ID})
				return err
			}
		}},
		{name: "lists", setup: func(t *testing.T, ww *wakeWorld) func() error {
			run := ww.running(t, domain.StateRunning)
			for range 4 {
				ww.pending(run, domain.WakeAt, nil)
			}
			return func() error {
				if _, err := ww.svc.Wakes().ListPending(ww.agentCtx()); err != nil {
					return err
				}
				page, err := ww.svc.Wakes().ListByThread(ww.agentCtx(), runs.ThreadWakesQuery{ThreadID: ww.thread, Limit: 3})
				if err != nil {
					return err
				}
				_, err = ww.svc.Wakes().ListByThread(ww.agentCtx(), runs.ThreadWakesQuery{ThreadID: ww.thread, Cursor: page.NextCursor})
				return err
			}
		}},
		{name: "launch check", setup: func(t *testing.T, ww *wakeWorld) func() error {
			run := ww.running(t, domain.StateRunning)
			return func() error { _, err := ww.svc.Wakes().LaunchCheck(ww.agentCtx(), run.ID); return err }
		}},
		{name: "launch check refused", setup: func(t *testing.T, ww *wakeWorld) func() error {
			run := ww.running(t, domain.StateQueued)
			return func() error { _, err := ww.svc.Wakes().LaunchCheck(ww.agentCtx(), run.ID); return refusalOK(err) }
		}},
		{name: "job runs.wake_due", setup: func(t *testing.T, ww *wakeWorld) func() error {
			run := ww.running(t, domain.StateCompleted)
			due := ww.pending(run, domain.WakeAt, nil)
			late := ww.pending(run, domain.WakeAt, func(w *app.WakeRow) { w.Deadline = ww.clk.Now().Add(-time.Second) })
			early := ww.pending(run, domain.WakeAt, func(w *app.WakeRow) {
				w.NotBefore = ww.clk.Now().Add(time.Hour)
				w.Deadline = w.NotBefore.Add(time.Hour)
			})
			return func() error {
				for _, id := range []ids.ID{late.ID, early.ID, due.ID} {
					if err := ww.svc.WakeDue(context.Background(), id); err != nil {
						return err
					}
				}
				return nil
			}
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			for at := 1; ; at++ {
				if at > 300 {
					t.Fatal("the path did not end within 300 port calls")
				}
				ww := &wakeWorld{world: newWorld(t), wakes: &wakeRepo{}, owner: !c.notOwner}
				ww.bind(t)
				step := c.setup(t, ww)
				g := &faultGate{at: at}
				ww.gate = g
				ww.build()
				ww.bind(t)
				g.mu.Lock()
				g.armed = true
				g.mu.Unlock()
				err := step()
				if !g.fired {
					if err != nil {
						t.Fatalf("without a fault: %v", err)
					}
					if at == 1 {
						t.Fatal("the path called no port")
					}
					return
				}
				if !errors.Is(err, errInjected) {
					t.Fatalf("fault at port call %d: the caller got %v", at, err)
				}
			}
		})
	}
}
