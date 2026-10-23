// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/runs"
	"github.com/OWNER/bothy/services/server/internal/runs/app"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// runningRun inserts a run of Lotse in thread T in state running at version 3 and returns it.
func (w *world) runningRun() app.RunRow {
	w.t.Helper()
	row, ok, err := w.repo.InsertRun(context.Background(), app.NewRun{ID: ids.New(), ThreadID: w.thread, AgentID: w.lotse, TriggeredBy: w.alex,
		TriggerKind: "message", TriggeringMessageID: ids.New(), BillingModeExpected: "organization", InvocationJTI: ids.New(), QueuedAt: t0})
	if err != nil || !ok {
		w.t.Fatal(err)
	}
	w.repo.mu.Lock()
	for i := range w.repo.runs {
		if w.repo.runs[i].ID == row.ID {
			w.repo.runs[i].State, w.repo.runs[i].Version, w.repo.runs[i].StartedAt = domain.StateRunning, 3, t0
			row = w.repo.runs[i]
		}
	}
	w.repo.mu.Unlock()
	return row
}

// setState sets the state of run in the fake repository.
func (w *world) setState(run ids.ID, s domain.State) {
	w.repo.mu.Lock()
	defer w.repo.mu.Unlock()
	for i := range w.repo.runs {
		if w.repo.runs[i].ID == run {
			w.repo.runs[i].State = s
		}
	}
}

// stored returns the row of run.
func (w *world) stored(run ids.ID) app.RunRow {
	row, _, _ := w.repo.Run(context.Background(), run)
	return row
}

// as returns a context with the agent or person id as actor.
func as(id ids.ID, kind actor.Kind) context.Context {
	return actor.WithActor(context.Background(), actor.Actor{ID: id, Kind: kind})
}

// op builds one patch op with a JSON value ("" for none).
func op(o, path, value string) runs.PatchOp {
	p := runs.PatchOp{Op: o, Path: path}
	if value != "" {
		p.Value = json.RawMessage(value)
	}
	return p
}

// svcWith rebuilds the use cases over the unit of work u.
func (w *world) svcWith(u txn.UnitOfWork) {
	w.uow = u
	w.build()
}

// codeOf returns the public code and reason of err.
func codeOf(t *testing.T, err error) (errs.Code, string, []errs.FieldError) {
	t.Helper()
	e, ok := errs.As(err)
	if !ok {
		t.Fatalf("error %v is no errs error", err)
	}
	return e.PublicCode(), e.Reason, e.Fields
}

// TestPatchOrderOfChecks holds the order of design note 1: unknown run not_found; another agent's
// run forbidden; a person forbidden; a forbidden path validation_failed without the row lock; an
// ended run with a stale base conflict run_not_active; a stale base base_version_conflict with the
// document; a dynamic error rolls back every op of the patch.
func TestPatchOrderOfChecks(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	run := w.runningRun()
	// AgentSelf of M1-28: only the agent that owns the run may patch it.
	w.authz.principalCheck = func(p authorize.Principal, r authorize.Resource) bool {
		return p.PrincipalKind() == string(actor.KindAgent) && p.PrincipalID() == r.OwnerID
	}

	_, err := w.svc.Patch(as(w.lotse, actor.KindAgent), runs.PatchRun{RunID: ids.New(), BaseVersion: 3, Ops: []runs.PatchOp{op("replace", "/status_line", `"Reading"`)}})
	if c, _, _ := codeOf(t, err); c != errs.CodeNotFound {
		t.Fatalf("unknown run: %s", c)
	}
	_, err = w.svc.Patch(as(w.texter, actor.KindAgent), runs.PatchRun{RunID: run.ID, BaseVersion: 3, Ops: []runs.PatchOp{op("replace", "/status_line", `"Reading"`)}})
	if c, _, _ := codeOf(t, err); c != errs.CodeForbidden {
		t.Fatalf("other agent: %s", c)
	}
	_, err = w.svc.Patch(as(w.alex, actor.KindUser), runs.PatchRun{RunID: run.ID, BaseVersion: 3, Ops: []runs.PatchOp{op("replace", "/status_line", `"Reading"`)}})
	if c, _, _ := codeOf(t, err); c != errs.CodeForbidden {
		t.Fatalf("person: %s", c)
	}
	w.repo.mu.Lock()
	locks := w.repo.locks
	w.repo.mu.Unlock()
	_, err = w.svc.Patch(as(w.lotse, actor.KindAgent), runs.PatchRun{RunID: run.ID, BaseVersion: 3, Ops: []runs.PatchOp{op("replace", "/state", `"completed"`)}})
	c, _, fields := codeOf(t, err)
	if c != errs.CodeValidationFailed || len(fields) != 1 || fields[0].Pointer != "/ops/0" || fields[0].Code != "patch_path_forbidden" {
		t.Fatalf("forbidden path: %s %+v", c, fields)
	}
	w.repo.mu.Lock()
	if w.repo.locks != locks {
		t.Fatal("the static checks took the row lock")
	}
	w.repo.mu.Unlock()
	if w.repo.patches != 0 {
		t.Fatal("a refused patch was stored")
	}
	_, err = w.svc.Patch(as(w.lotse, actor.KindAgent), runs.PatchRun{RunID: run.ID, BaseVersion: 2, Ops: []runs.PatchOp{op("replace", "/status_line", `"Reading"`)}})
	c, _, _ = codeOf(t, err)
	if c != errs.CodeBaseVersionConflict {
		t.Fatalf("stale base: %s", c)
	}
	e, _ := errs.As(err)
	doc, ok := e.Current.(runs.Run)
	if !ok || doc.ID != run.ID || doc.Version != 3 {
		t.Fatalf("current = %#v", e.Current)
	}
	// A dynamic error in the second op rolls back the first.
	_, err = w.svc.Patch(as(w.lotse, actor.KindAgent), runs.PatchRun{RunID: run.ID, BaseVersion: 3, Ops: []runs.PatchOp{
		op("replace", "/status_line", `"Reading"`), op("replace", "/steps/"+ids.New().String()+"/status", `"completed"`)}})
	c, _, fields = codeOf(t, err)
	if c != errs.CodeValidationFailed || len(fields) != 1 || fields[0].Code != "patch_target_missing" || fields[0].Pointer != "/ops/1" {
		t.Fatalf("dynamic error: %s %+v", c, fields)
	}
	if got := w.stored(run.ID); got.StatusLine != "" || got.Version != 3 {
		t.Fatalf("rolled back patch left %q at version %d", got.StatusLine, got.Version)
	}
	// An ended run with a stale base gives run_not_active: the state comes first.
	w.setState(run.ID, domain.StateCancelled)
	_, err = w.svc.Patch(as(w.lotse, actor.KindAgent), runs.PatchRun{RunID: run.ID, BaseVersion: 1, Ops: []runs.PatchOp{op("replace", "/status_line", `"Reading"`)}})
	c, reason, _ := codeOf(t, err)
	if c != errs.CodeConflict || reason != "run_not_active" {
		t.Fatalf("ended run: %s %s", c, reason)
	}
	if len(w.pub.list()) != 0 {
		t.Fatal("a refused patch published")
	}
}

// TestPatchPublishesAfterCommitOnly: a rollback publishes nothing; a commit publishes exactly one
// run.updated with base, new version and normalized ops; a unit of work retried after a deadlock
// publishes once.
func TestPatchPublishesAfterCommitOnly(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	run := w.runningRun()
	res, err := w.svc.Patch(as(w.lotse, actor.KindAgent), runs.PatchRun{RunID: run.ID, BaseVersion: 3, Ops: []runs.PatchOp{
		op("replace", "/status_line", `"Reading export.log"`), op("replace", "/model", `"model-a"`)}})
	if err != nil {
		t.Fatal(err)
	}
	if res.Version != 4 {
		t.Fatalf("version %d", res.Version)
	}
	evs := w.pub.list()
	if len(evs) != 1 {
		t.Fatalf("%d events", len(evs))
	}
	ev := evs[0]
	u, ok := ev.Payload.(runs.RunUpdated)
	if !ok || ev.Type != "run.updated" || u.BaseVersion != 3 || u.Version != 4 || u.RunID != run.ID || u.ThreadID != w.thread || u.Truncated {
		t.Fatalf("event %+v", ev)
	}
	if ev.Resource.Kind != "run" || ev.Resource.ID != run.ID || ev.Resource.Version != 4 || ev.Actor == nil || ev.Actor.PrincipalID != w.lotse ||
		ev.Scope.ThreadID != w.thread || ev.Scope.ChannelID != w.channel || ev.Scope.SpaceID != w.space || ev.Audience != (ids.ID{}) {
		t.Fatalf("envelope parts %+v", ev)
	}
	// Both members were absent, so both normalize to add.
	if len(u.Ops) != 2 || u.Ops[0].Op != "add" || u.Ops[0].Path != "/status_line" || u.Ops[1].Op != "add" || u.Ops[1].Path != "/model" {
		t.Fatalf("ops %+v", u.Ops)
	}
	// A rollback after the write publishes nothing.
	w2 := newWorld(t)
	run2 := w2.runningRun()
	failing := &vetoUoW{}
	w2.svcWith(failing)
	if _, err := w2.svc.Patch(as(w2.lotse, actor.KindAgent), runs.PatchRun{RunID: run2.ID, BaseVersion: 3,
		Ops: []runs.PatchOp{op("replace", "/status_line", `"x"`)}}); !errors.Is(err, errVeto) {
		t.Fatalf("veto: %v", err)
	}
	if len(w2.pub.list()) != 0 {
		t.Fatal("a rolled back patch published")
	}
	// A retried unit of work publishes once.
	w3 := newWorld(t)
	run3 := w3.runningRun()
	w3.svcWith(&retryUoW{repo: w3.repo})
	if _, err := w3.svc.Patch(as(w3.lotse, actor.KindAgent), runs.PatchRun{RunID: run3.ID, BaseVersion: 3,
		Ops: []runs.PatchOp{op("replace", "/status_line", `"x"`)}}); err != nil {
		t.Fatal(err)
	}
	if n := len(w3.pub.list()); n != 1 {
		t.Fatalf("retried unit published %d times", n)
	}
}

// TestPatchWritesNoEventNoAudit: on success the appender, the audit recorder and the
// in-transaction bus see nothing, and the version rises by exactly one for several ops.
func TestPatchWritesNoEventNoAudit(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	run := w.runningRun()
	before := w.j.count("event")
	res, err := w.svc.Patch(as(w.lotse, actor.KindAgent), runs.PatchRun{RunID: run.ID, BaseVersion: 3, Ops: []runs.PatchOp{
		op("replace", "/status_line", `"a"`), op("add", "/metrics/tokens_in", `12`), op("replace", "/cost", `{"amount_micros":5,"currency":"EUR","is_estimate":true}`),
		op("add", "/steps/"+ids.New().String(), `{"kind":"tool_call","name":"read_file"}`)}})
	if err != nil {
		t.Fatal(err)
	}
	if res.Version != 4 || w.stored(run.ID).Version != 4 {
		t.Fatalf("version %d", res.Version)
	}
	if w.j.count("event") != before {
		t.Fatal("a patch appended a durable event")
	}
	if entries, _ := w.audit.list(); len(entries) != 0 {
		t.Fatalf("a patch wrote %d audit entries", len(entries))
	}
	if got := w.steps.list; !slices.Equal(got, []string{"lotse:tool_call"}) {
		t.Fatalf("bothy.run.steps %v", got)
	}
}

// TestPatchClearsStallWarning: a stored patch writes through WritePatch, whose statement clears
// stall_warned_at with version + 1 (the fake counts the write; adapters/postgres holds the column
// in its repository test).
func TestPatchClearsStallWarning(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	run := w.runningRun()
	if _, err := w.svc.Patch(as(w.lotse, actor.KindAgent), runs.PatchRun{RunID: run.ID, BaseVersion: 3,
		Ops: []runs.PatchOp{op("remove", "/status_line", "")}}); err != nil {
		t.Fatal(err)
	}
	// A patch whose ops are all no-ops is still one stored patch: movement clears the hint.
	if w.repo.patches != 1 || w.stored(run.ID).Version != 4 {
		t.Fatalf("patches %d version %d", w.repo.patches, w.stored(run.ID).Version)
	}
}

// TestPatchOversizeChangePublishedTruncated: when the encoded payload with ops reaches
// sync.event_payload_max_bytes, run.updated goes out without ops and with Truncated.
func TestPatchOversizeChangePublishedTruncated(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	w.payloadMax = 200
	w.build()
	run := w.runningRun()
	if _, err := w.svc.Patch(as(w.lotse, actor.KindAgent), runs.PatchRun{RunID: run.ID, BaseVersion: 3,
		Ops: []runs.PatchOp{op("replace", "/status_line", `"`+strings.Repeat("a", 300)+`"`)}}); err != nil {
		t.Fatal(err)
	}
	evs := w.pub.list()
	if len(evs) != 1 {
		t.Fatalf("%d events", len(evs))
	}
	u := evs[0].Payload.(runs.RunUpdated)
	if !u.Truncated || len(u.Ops) != 0 || u.Version != 4 {
		t.Fatalf("payload %+v", u)
	}
}

// vetoUoW runs fn and then refuses the commit with errVeto, like a vetoing step.
type vetoUoW struct{}

func (vetoUoW) Do(ctx context.Context, fn func(ctx context.Context) error, _ ...txn.Option) error {
	if err := (uow{}).Do(ctx, fn); err != nil {
		return err
	}
	return errVeto
}

// retryUoW runs fn, rolls the fake repository back and runs fn again, like a unit of work that
// retries after 40P01.
type retryUoW struct{ repo *repo }

func (r *retryUoW) Do(ctx context.Context, fn func(ctx context.Context) error, _ ...txn.Option) error {
	r.repo.mu.Lock()
	runsCopy, stepsCopy, patches := slices.Clone(r.repo.runs), slices.Clone(r.repo.steps), r.repo.patches
	r.repo.mu.Unlock()
	if err := (uow{}).Do(ctx, fn); err != nil {
		return err
	}
	r.repo.mu.Lock()
	r.repo.runs, r.repo.steps, r.repo.patches = runsCopy, stepsCopy, patches
	r.repo.mu.Unlock()
	return (uow{}).Do(ctx, fn)
}
