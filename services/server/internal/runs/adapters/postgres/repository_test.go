// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"reflect"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
	"github.com/OWNER/bothy/services/server/internal/runs/adapters/postgres"
	"github.com/OWNER/bothy/services/server/internal/runs/app"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// env is one test database with the repository, a unit of work and the owner pool for fixtures.
type env struct {
	repo  *postgres.Repository
	uow   *pg.UnitOfWork
	owner *pg.Pool
}

func newEnv(t *testing.T) env {
	t.Helper()
	db := pgtest.New(t)
	pool := db.Pool(t, pg.RoleApp)
	reg := txn.NewRegistry()
	reg.Seal()
	uow := pg.NewUnitOfWork(pool, pg.UnitConfig{StatementTimeout: 30 * time.Second, LockTimeout: 10 * time.Second, IdleInTxTimeout: 60 * time.Second}, reg)
	return env{repo: postgres.NewRepository(pool), uow: uow, owner: db.Pool(t, pg.RoleOwner)}
}

// exec runs a fixture statement as bothy_owner.
func (e env) exec(t *testing.T, sql string, args ...any) {
	t.Helper()
	if _, err := e.owner.Exec(context.Background(), sql, args...); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
}

// world is the fixture: the person Alex, the agents Robin and Sam, one channel with two threads
// and a human message in the first.
type world struct {
	alex, robin, sam   ids.ID
	thread, other, msg ids.ID
}

func (e env) world(t *testing.T) world {
	t.Helper()
	w := world{alex: ids.New(), robin: ids.New(), sam: ids.New(), thread: ids.New(), other: ids.New(), msg: ids.New()}
	space, channel := ids.New(), ids.New()
	e.exec(t, `INSERT INTO principals (id, kind, handle, display_name) VALUES ($1, 'user', 'alex', 'Alex')`, w.alex.String())
	e.exec(t, `INSERT INTO users (principal_id) VALUES ($1)`, w.alex.String())
	for handle, id := range map[string]ids.ID{"robin": w.robin, "sam": w.sam} {
		e.exec(t, `INSERT INTO principals (id, kind, handle, display_name) VALUES ($1, 'agent', $2, $2)`, id.String(), handle)
		e.exec(t, `INSERT INTO agents (principal_id, adapter_kind) VALUES ($1, 'custom')`, id.String())
	}
	e.exec(t, `INSERT INTO spaces (id, name, sort_key, created_by) VALUES ($1, 'Support', 'a0', $2)`, space.String(), w.alex.String())
	e.exec(t, `INSERT INTO channels (id, space_id, kind, name, sort_key, history_backfill_limit, created_by)
		VALUES ($1, $2, 'text', 'general', 'a0', 20, $3)`, channel.String(), space.String(), w.alex.String())
	e.exec(t, `INSERT INTO threads (id, channel_id, kind, created_by) VALUES ($1, $2, 'main', $3)`, w.thread.String(), channel.String(), w.alex.String())
	e.exec(t, `INSERT INTO threads (id, channel_id, kind, created_by) VALUES ($1, $2, 'thread', $3)`, w.other.String(), channel.String(), w.alex.String())
	e.exec(t, `INSERT INTO messages (id, thread_id, channel_id, thread_seq, author_principal_id, client_msg_id,
		client_payload_sha256, origin, content_md) VALUES ($1, $2, $3, 1, $4, $1, '\x00', 'human', 'hello')`,
		w.msg.String(), w.thread.String(), channel.String(), w.alex.String())
	return w
}

// newRun is a queued run of agent in thread at queued.
func newRun(w world, thread, agent ids.ID, queued time.Time) app.NewRun {
	return app.NewRun{
		ID: ids.New(), ThreadID: thread, AgentID: agent, TriggeredBy: w.alex, TriggerKind: "message",
		TriggeringMessageID: w.msg, BillingModeExpected: "organization", InvocationJTI: ids.New(), QueuedAt: queued,
	}
}

// do runs fn in one unit of work and fails the test on an error.
func (e env) do(t *testing.T, fn func(ctx context.Context) error) {
	t.Helper()
	if err := e.uow.Do(context.Background(), fn); err != nil {
		t.Fatal(err)
	}
}

// insert inserts n in its own unit and returns the stored row; inserted must be want.
func (e env) insert(t *testing.T, n app.NewRun, want bool) app.RunRow {
	t.Helper()
	var row app.RunRow
	e.do(t, func(ctx context.Context) error {
		var inserted bool
		var err error
		row, inserted, err = e.repo.InsertRun(ctx, n)
		if err == nil && inserted != want {
			t.Errorf("InsertRun inserted %v, want %v", inserted, want)
		}
		return err
	})
	return row
}

// transition runs w in its own unit; changed must be want.
func (e env) transition(t *testing.T, w app.TransitionWrite, want bool) app.RunRow {
	t.Helper()
	var row app.RunRow
	e.do(t, func(ctx context.Context) error {
		var changed bool
		var err error
		row, changed, err = e.repo.TransitionRun(ctx, w)
		if err == nil && changed != want {
			t.Errorf("TransitionRun %s -> %s at version %d changed %v, want %v", w.From, w.To, w.Version, changed, want)
		}
		return err
	})
	return row
}

// TestRepository_InsertAndActiveRun: a run starts queued with version 1, the stored row reads
// back equal, a second active run of the thread is not inserted (R-DM-14), and after an end state
// the next insert succeeds. Writes outside a unit of work fail.
func TestRepository_InsertAndActiveRun(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	w := e.world(t)
	ctx := context.Background()
	queued := time.Date(2026, 10, 20, 9, 0, 0, 123456789, time.UTC)
	n := newRun(w, w.thread, w.robin, queued)
	if _, _, err := e.repo.InsertRun(ctx, n); err == nil {
		t.Fatal("InsertRun outside a unit of work")
	}
	row := e.insert(t, n, true)
	want := app.RunRow{
		ID: n.ID, ThreadID: w.thread, AgentID: w.robin, TriggerKind: "message", TriggeringMessageID: w.msg,
		TriggeredBy: w.alex, State: domain.StateQueued, Version: 1, BillingModeExpected: "organization",
		InvocationJTI: n.InvocationJTI, BillingEvidence: []byte("{}"), Plan: []byte("{}"), Metrics: []byte("{}"), Currency: "USD",
		CostIsEstimate: true,
		QueuedAt:       queued.Truncate(time.Microsecond),
	}
	if !reflect.DeepEqual(row, want) {
		t.Errorf("inserted run:\n got %+v\nwant %+v", row, want)
	}
	got, found, err := e.repo.Run(ctx, n.ID)
	if err != nil || !found || !reflect.DeepEqual(got, want) {
		t.Errorf("Run = %+v, %v, %v; want the inserted row", got, found, err)
	}
	if _, found, err := e.repo.Run(ctx, ids.New()); err != nil || found {
		t.Errorf("Run of an unknown id: found %v, %v", found, err)
	}
	active, found, err := e.repo.ActiveRunOfThread(ctx, w.thread)
	if err != nil || !found || active.ID != n.ID {
		t.Errorf("ActiveRunOfThread = %v, %v, %v; want %v", active.ID, found, err, n.ID)
	}
	if _, found, err := e.repo.ActiveRunOfThread(ctx, w.other); err != nil || found {
		t.Errorf("ActiveRunOfThread of a thread without runs: found %v, %v", found, err)
	}

	second := e.insert(t, newRun(w, w.thread, w.sam, queued.Add(time.Second)), false)
	if second.ID != (ids.ID{}) {
		t.Errorf("a refused insert returned row %v", second.ID)
	}
	e.transition(t, app.TransitionWrite{ID: n.ID, From: domain.StateQueued, Version: 1, To: domain.StateCancelled,
		Reason: "user_cancelled", At: queued.Add(time.Minute)}, true)
	if _, found, err := e.repo.ActiveRunOfThread(ctx, w.thread); err != nil || found {
		t.Errorf("ActiveRunOfThread after the end state: found %v, %v", found, err)
	}
	e.insert(t, newRun(w, w.thread, w.sam, queued.Add(2*time.Minute)), true)
}

// TestRepository_TransitionCompareAndSwap: a wrong version or a wrong from state changes nothing;
// started_at keeps its first value through waiting_input -> running, version rises by one per
// change, the reason is NULL unless given, and an end state stamps finished_at and the duration.
func TestRepository_TransitionCompareAndSwap(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	w := e.world(t)
	ctx := context.Background()
	t0 := time.Date(2026, 10, 20, 9, 0, 0, 0, time.UTC)
	n := newRun(w, w.thread, w.robin, t0)
	e.insert(t, n, true)

	stale := e.transition(t, app.TransitionWrite{ID: n.ID, From: domain.StateQueued, Version: 2, To: domain.StateRunning, At: t0}, false)
	wrongFrom := e.transition(t, app.TransitionWrite{ID: n.ID, From: domain.StateRunning, Version: 1, To: domain.StateCompleted, At: t0}, false)
	missing := e.transition(t, app.TransitionWrite{ID: ids.New(), From: domain.StateQueued, Version: 1, To: domain.StateRunning, At: t0}, false)
	if stale.ID != (ids.ID{}) || wrongFrom.ID != (ids.ID{}) || missing.ID != (ids.ID{}) {
		t.Error("a refused transition returned a row")
	}
	row, _, err := e.repo.Run(ctx, n.ID)
	if err != nil || row.State != domain.StateQueued || row.Version != 1 || !row.StartedAt.IsZero() {
		t.Fatalf("row after refused transitions: %+v, %v", row, err)
	}

	start := t0.Add(1500 * time.Millisecond)
	row = e.transition(t, app.TransitionWrite{ID: n.ID, From: domain.StateQueued, Version: 1, To: domain.StateRunning, At: start}, true)
	if row.State != domain.StateRunning || row.Version != 2 || !row.StartedAt.Equal(start) || row.StateReason != "" || row.HasDuration {
		t.Errorf("after start: %+v", row)
	}
	row = e.transition(t, app.TransitionWrite{ID: n.ID, From: domain.StateRunning, Version: 2, To: domain.StateWaitingInput, At: start.Add(time.Second)}, true)
	row = e.transition(t, app.TransitionWrite{ID: n.ID, From: domain.StateWaitingInput, Version: 3, To: domain.StateRunning, At: start.Add(time.Minute)}, true)
	if !row.StartedAt.Equal(start) || row.Version != 4 {
		t.Errorf("waiting_input -> running moved started_at to %v (version %d), want %v", row.StartedAt, row.Version, start)
	}
	end := start.Add(2*time.Minute + 250*time.Millisecond)
	row = e.transition(t, app.TransitionWrite{ID: n.ID, From: domain.StateRunning, Version: 4, To: domain.StateFailed,
		Reason: "runtime_error", At: end}, true)
	if row.State != domain.StateFailed || row.StateReason != "runtime_error" || row.Version != 5 || !row.FinishedAt.Equal(end) ||
		!row.HasDuration || row.DurationMS != 120250 {
		t.Errorf("after failed: %+v", row)
	}

	// A run that never started ends without a duration.
	q := newRun(w, w.other, w.sam, t0)
	e.insert(t, q, true)
	row = e.transition(t, app.TransitionWrite{ID: q.ID, From: domain.StateQueued, Version: 1, To: domain.StateCancelled,
		Reason: "user_cancelled", At: end}, true)
	if !row.FinishedAt.Equal(end) || row.HasDuration || !row.StartedAt.IsZero() {
		t.Errorf("cancelled before start: %+v", row)
	}
}

// TestRepository_Steps: ordinals are 1, 2, 3 in insertion order, an empty detail stores "{}",
// StepCounts counts per run in one statement and leaves runs without steps out, Steps pages by
// ordinal and AllSteps reads every step.
func TestRepository_Steps(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	w := e.world(t)
	ctx := context.Background()
	t0 := time.Date(2026, 10, 20, 9, 0, 0, 0, time.UTC)
	a := newRun(w, w.thread, w.robin, t0)
	b := newRun(w, w.other, w.sam, t0)
	e.insert(t, a, true)
	e.insert(t, b, true)

	if _, err := e.repo.AppendStep(ctx, app.StepRow{ID: ids.New(), RunID: a.ID, Kind: "status", Name: "x", Status: "running"}); err == nil {
		t.Fatal("AppendStep outside a unit of work")
	}
	var appended []app.StepRow
	e.do(t, func(ctx context.Context) error {
		parent := ids.New()
		for i, s := range []app.StepRow{
			{ID: parent, RunID: a.ID, Kind: "tool_call", Name: "search", Title: "Search", Status: "running", StartedAt: t0},
			{ID: ids.New(), RunID: a.ID, ParentStepID: parent, Kind: "subagent", Name: "review", Status: "completed",
				Detail: []byte(`{"note":"ok"}`), StartedAt: t0, FinishedAt: t0.Add(time.Second)},
			{ID: ids.New(), RunID: a.ID, Kind: "status", Name: "adapter_reason", Status: "failed", Ordinal: 99},
		} {
			got, err := e.repo.AppendStep(ctx, s)
			if err != nil {
				return err
			}
			if got.Ordinal != i+1 {
				t.Errorf("step %d got ordinal %d", i, got.Ordinal)
			}
			appended = append(appended, got)
		}
		return nil
	})
	if string(appended[0].Detail) != "{}" || appended[1].ParentStepID != appended[0].ID || appended[2].StartedAt.IsZero() ||
		!appended[1].FinishedAt.Equal(t0.Add(time.Second)) || !appended[0].FinishedAt.IsZero() {
		t.Errorf("appended steps: %+v", appended)
	}

	counts, err := e.repo.StepCounts(ctx, []ids.ID{a.ID, b.ID})
	if err != nil || !reflect.DeepEqual(counts, map[ids.ID]int{a.ID: 3}) {
		t.Errorf("StepCounts = %v, %v; want only %v: 3", counts, err, a.ID)
	}
	if counts, err := e.repo.StepCounts(ctx, nil); err != nil || len(counts) != 0 {
		t.Errorf("StepCounts of no runs = %v, %v", counts, err)
	}
	page, err := e.repo.Steps(ctx, app.StepsRead{RunID: a.ID, AfterOrdinal: 0, Limit: 2})
	if err != nil || ordinals(page) != "1,2" {
		t.Errorf("first page %s, %v", ordinals(page), err)
	}
	page, err = e.repo.Steps(ctx, app.StepsRead{RunID: a.ID, AfterOrdinal: 2, Limit: 2})
	if err != nil || ordinals(page) != "3" {
		t.Errorf("second page %s, %v", ordinals(page), err)
	}
	all, err := e.repo.AllSteps(ctx, a.ID)
	if err != nil || !reflect.DeepEqual(all, appended) {
		t.Errorf("AllSteps:\n got %+v\nwant %+v (%v)", all, appended, err)
	}
	if none, err := e.repo.AllSteps(ctx, b.ID); err != nil || len(none) != 0 {
		t.Errorf("AllSteps of a run without steps = %v, %v", none, err)
	}
}

// ordinals renders the ordinals of steps as "1,2".
func ordinals(steps []app.StepRow) string {
	out := ""
	for i, s := range steps {
		if i > 0 {
			out += ","
		}
		out += strconv.Itoa(s.Ordinal)
	}
	return out
}

// TestRepository_ListsAndCounts: ThreadRuns and ActiveRuns page in keyset order (queued_at DESC,
// id DESC), also across a tie of queued_at; ActiveRuns keeps only active runs and filters by agent;
// ActiveCounts groups the active runs by agent and state.
func TestRepository_ListsAndCounts(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	w := e.world(t)
	ctx := context.Background()
	t0 := time.Date(2026, 10, 20, 9, 0, 0, 0, time.UTC)

	// Thread history: three ended runs and one active run of the same thread. Two share queued_at,
	// so id DESC decides between them.
	var history []app.NewRun
	for i, at := range []time.Time{t0, t0.Add(time.Minute), t0.Add(time.Minute), t0.Add(3 * time.Minute)} {
		n := newRun(w, w.thread, w.robin, at)
		e.insert(t, n, true)
		history = append(history, n)
		if i < 3 {
			e.transition(t, app.TransitionWrite{ID: n.ID, From: domain.StateQueued, Version: 1, To: domain.StateCancelled,
				Reason: "user_cancelled", At: at.Add(time.Second)}, true)
		}
	}
	wantOrder := []ids.ID{history[3].ID}
	tie := []ids.ID{history[1].ID, history[2].ID}
	slices.SortFunc(tie, func(x, y ids.ID) int { return -compareID(x, y) })
	wantOrder = append(wantOrder, tie...)
	wantOrder = append(wantOrder, history[0].ID)

	var got []ids.ID
	var after *app.RunKey
	for {
		page, err := e.repo.ThreadRuns(ctx, app.ThreadRunsRead{ThreadID: w.thread, After: after, Limit: 3})
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range page {
			got = append(got, r.ID)
		}
		if len(page) < 3 {
			break
		}
		last := page[len(page)-1]
		after = &app.RunKey{QueuedAt: last.QueuedAt, ID: last.ID}
	}
	if !slices.Equal(got, wantOrder) {
		t.Errorf("ThreadRuns order:\n got %v\nwant %v", got, wantOrder)
	}
	if other, err := e.repo.ThreadRuns(ctx, app.ThreadRunsRead{ThreadID: w.other, Limit: 10}); err != nil || len(other) != 0 {
		t.Errorf("ThreadRuns of another thread = %d rows, %v", len(other), err)
	}

	// Active runs: the active run of w.thread (Robin, queued) and one of w.other (Sam, running).
	sam := newRun(w, w.other, w.sam, t0.Add(2*time.Minute))
	e.insert(t, sam, true)
	e.transition(t, app.TransitionWrite{ID: sam.ID, From: domain.StateQueued, Version: 1, To: domain.StateRunning, At: t0.Add(150 * time.Second)}, true)

	first, err := e.repo.ActiveRuns(ctx, app.ActiveRunsRead{Limit: 1})
	if err != nil || len(first) != 1 || first[0].ID != history[3].ID {
		t.Fatalf("first active batch = %v, %v; want %v", runIDs(first), err, history[3].ID)
	}
	next, err := e.repo.ActiveRuns(ctx, app.ActiveRunsRead{Limit: 5, After: &app.RunKey{QueuedAt: first[0].QueuedAt, ID: first[0].ID}})
	if err != nil || !slices.Equal(runIDs(next), []ids.ID{sam.ID}) {
		t.Errorf("second active batch = %v, %v; want %v", runIDs(next), err, sam.ID)
	}
	robins, err := e.repo.ActiveRuns(ctx, app.ActiveRunsRead{AgentID: w.robin, Limit: 5})
	if err != nil || !slices.Equal(runIDs(robins), []ids.ID{history[3].ID}) {
		t.Errorf("active runs of Robin = %v, %v", runIDs(robins), err)
	}
	if nobody, err := e.repo.ActiveRuns(ctx, app.ActiveRunsRead{AgentID: ids.New(), Limit: 5}); err != nil || len(nobody) != 0 {
		t.Errorf("active runs of an unknown agent = %v, %v", runIDs(nobody), err)
	}
	if _, err := e.repo.ActiveRuns(ctx, app.ActiveRunsRead{Limit: 0}); err == nil {
		t.Error("ActiveRuns without a limit succeeded")
	}

	counts, err := e.repo.ActiveCounts(ctx)
	if err != nil {
		t.Fatal(err)
	}
	slices.SortFunc(counts, func(x, y app.ActiveCount) int { return compareID(x.AgentID, y.AgentID) })
	want := []app.ActiveCount{
		{AgentID: w.robin, State: domain.StateQueued, Count: 1},
		{AgentID: w.sam, State: domain.StateRunning, Count: 1},
	}
	slices.SortFunc(want, func(x, y app.ActiveCount) int { return compareID(x.AgentID, y.AgentID) })
	if !reflect.DeepEqual(counts, want) {
		t.Errorf("ActiveCounts = %+v, want %+v", counts, want)
	}
}

// runIDs returns the ids of rows in order.
func runIDs(rows []app.RunRow) []ids.ID {
	out := make([]ids.ID, len(rows))
	for i, r := range rows {
		out[i] = r.ID
	}
	return out
}

// compareID orders ids by their bytes, the order of uuid in PostgreSQL.
func compareID(x, y ids.ID) int {
	return slices.Compare(x[:], y[:])
}

// TestRepository_RunBinding: the acting-for read of M2-14 (Design note 3c and 3g) works outside
// a unit of work, returns the run, its agent, the triggering person, the thread and the stored
// state, follows a state change and reports an unknown run as not found without an error.
func TestRepository_RunBinding(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	w := e.world(t)
	ctx := context.Background()
	t0 := time.Date(2026, 10, 20, 9, 0, 0, 0, time.UTC)
	n := newRun(w, w.thread, w.robin, t0)
	e.insert(t, n, true)

	got, found, err := e.repo.RunBinding(ctx, n.ID)
	want := domain.RunBinding{ID: n.ID, AgentID: w.robin, TriggeredBy: w.alex, ThreadID: w.thread, State: string(domain.StateQueued)}
	if err != nil || !found || got != want {
		t.Fatalf("RunBinding = %+v, %v, %v; want %+v", got, found, err, want)
	}
	e.transition(t, app.TransitionWrite{ID: n.ID, From: domain.StateQueued, Version: 1, To: domain.StateCancelled,
		Reason: "user_cancelled", At: t0.Add(time.Second)}, true)
	got, found, err = e.repo.RunBinding(ctx, n.ID)
	want.State = string(domain.StateCancelled)
	if err != nil || !found || got != want {
		t.Errorf("RunBinding after the end = %+v, %v, %v; want %+v", got, found, err, want)
	}
	got, found, err = e.repo.RunBinding(ctx, ids.New())
	if err != nil || found || got != (domain.RunBinding{}) {
		t.Errorf("RunBinding of an unknown run = %+v, %v, %v; want not found", got, found, err)
	}
}

// TestRepository_LiveSteps (FEAT-topology-agent-tree): the statement of the agent tree reads, per
// run, the task steps with title and detail.last_tool, the running tool calls and the tool calls
// that started a task, in step order; a finished tool call without a task child and a status
// step stay out, and a run without such steps is absent.
func TestRepository_LiveSteps(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	w := e.world(t)
	ctx := context.Background()
	t0 := time.Date(2026, 11, 1, 9, 0, 0, 0, time.UTC)
	a := newRun(w, w.thread, w.robin, t0)
	b := newRun(w, w.other, w.sam, t0)
	e.insert(t, a, true)
	e.insert(t, b, true)
	call, flow, agent, running := ids.New(), ids.New(), ids.New(), ids.New()
	e.do(t, func(ctx context.Context) error {
		for _, s := range []app.StepRow{
			{ID: call, RunID: a.ID, Kind: "tool_call", Name: "Workflow", Status: "completed", StartedAt: t0, FinishedAt: t0.Add(time.Second)},
			{ID: flow, RunID: a.ID, ParentStepID: call, Kind: "workflow", Name: "local_workflow", Title: "Build and review", Status: "running",
				StartedAt: t0},
			{ID: ids.New(), RunID: a.ID, Kind: "tool_call", Name: "Read", Status: "completed", StartedAt: t0, FinishedAt: t0},
			{ID: agent, RunID: a.ID, ParentStepID: flow, Kind: "subagent", Name: "workflow_agent", Title: "review", Status: "completed",
				Detail: []byte(`{"type":"workflow_agent","last_tool":"Grep"}`), StartedAt: t0, FinishedAt: t0.Add(2 * time.Second)},
			{ID: running, RunID: a.ID, Kind: "tool_call", Name: "Bash", Status: "running", StartedAt: t0},
			{ID: ids.New(), RunID: a.ID, Kind: "status", Name: "compacting", Status: "running", StartedAt: t0},
			{ID: ids.New(), RunID: b.ID, Kind: "tool_call", Name: "Read", Status: "completed", StartedAt: t0, FinishedAt: t0},
		} {
			if _, err := e.repo.AppendStep(ctx, s); err != nil {
				return err
			}
		}
		return nil
	})
	got, err := e.repo.LiveSteps(ctx, []ids.ID{a.ID, b.ID})
	if err != nil {
		t.Fatal(err)
	}
	if _, has := got[b.ID]; has || len(got) != 1 {
		t.Fatalf("runs %v", got)
	}
	s := got[a.ID]
	if len(s) != 4 || s[0].ID != call || s[1].ID != flow || s[2].ID != agent || s[3].ID != running {
		t.Fatalf("steps %+v", s)
	}
	if s[1].ParentID != call || s[1].Title != "Build and review" || s[2].LastTool != "Grep" || s[2].Status != "completed" ||
		!s[2].FinishedAt.Equal(t0.Add(2*time.Second)) || !s[3].FinishedAt.IsZero() || !s[0].ParentID.IsZero() || s[3].Name != "Bash" {
		t.Fatalf("steps %+v", s)
	}
	if none, err := e.repo.LiveSteps(ctx, nil); err != nil || len(none) != 0 {
		t.Fatalf("no runs: %v %v", none, err)
	}
}
