// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"encoding/json"
	gosync "sync"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/gateway/app"
	"github.com/OWNER/bothy/services/server/internal/gateway/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/runs"
)

// fakePatcher is app.RunPatcher: it records the patches and their actors and answers err; on
// success it records the authorize check of run.update as the real use case does through the
// Authorizer.
type fakePatcher struct {
	mu      gosync.Mutex
	err     error
	patches []runs.PatchRun
	actors  []ids.ID
}

func (p *fakePatcher) Patch(ctx context.Context, in runs.PatchRun) (runs.PatchResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	a, _ := actor.FromContext(ctx)
	p.patches, p.actors = append(p.patches, in), append(p.actors, a.ID)
	if p.err != nil {
		return runs.PatchResult{}, p.err
	}
	authorize.RecordCheck(ctx, authorize.Check{Action: "run.update", Kind: authorize.CheckAuthorize, Allowed: true})
	return runs.PatchResult{Version: in.BaseVersion + 1}, nil
}

// calls returns the recorded patches and actors.
func (p *fakePatcher) calls() ([]runs.PatchRun, []ids.ID) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]runs.PatchRun(nil), p.patches...), append([]ids.ID(nil), p.actors...)
}

// runUpdateEnv returns an env with run.update registered over p and the agent identified on a
// connection; deny empties rl.agent_runtime.
func runUpdateEnv(t *testing.T, p *fakePatcher, deny bool) (*env, *app.Connection, *fakeConn) {
	t.Helper()
	e := newEnv(t, func(_ *app.Config, d *app.Deps) {
		d.Runs = p
		d.AgentLimiter = agentLimiter{deny: deny}
	})
	if err := e.gw.Register("run.update", authorize.ActionRunUpdate); err != nil {
		t.Fatal(err)
	}
	c, fc := e.open()
	e.send(c, "identify", map[string]any{"protocol_version": 1, "api_token": agentToken})
	wantOpen(t, fc)
	return e, c, fc
}

// runUpdateBody is the body of run.update with ref r-1.
func runUpdateBody(run ids.ID) map[string]any {
	return map[string]any{"run_id": run, "base_version": 7, "ref": "r-1",
		"ops": []map[string]any{{"op": "replace", "path": "/status_line", "value": "Reading export.log"}}}
}

// frameCount returns the number of frames sent on fc.
func frameCount(fc *fakeConn) int {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	return len(fc.frames)
}

// TestRunUpdateOp: a success patches through runs with the connection's actor and sends no
// frame; rate_limited, not_found, forbidden, validation_failed, base_version_conflict and
// conflict run_not_active answer an error frame with op run.update, the detail and ref echoed,
// and the connection stays open; a malformed d closes with 4002.
func TestRunUpdateOp(t *testing.T) {
	t.Parallel()
	t.Run("success", func(t *testing.T) {
		t.Parallel()
		p := &fakePatcher{}
		e, c, fc := runUpdateEnv(t, p, false)
		run := ids.New()
		before := frameCount(fc)
		e.send(c, "run.update", runUpdateBody(run))
		wantOpen(t, fc)
		if n := frameCount(fc); n != before {
			t.Fatalf("%d frames after a success", n-before)
		}
		patches, actors := p.calls()
		if len(patches) != 1 || patches[0].RunID != run || patches[0].BaseVersion != 7 || len(patches[0].Ops) != 1 {
			t.Fatalf("patches %+v", patches)
		}
		op := patches[0].Ops[0]
		if op.Op != "replace" || op.Path != "/status_line" || string(op.Value) != `"Reading export.log"` {
			t.Fatalf("op %+v", op)
		}
		if actors[0] != e.agent.ID {
			t.Fatalf("actor %v, want the agent", actors[0])
		}
	})
	for _, tc := range []struct {
		name, code, detail string
		deny               bool
		err                error
	}{
		{name: "rate_limited", code: "rate_limited", detail: "retry_after=3", deny: true},
		{name: "not_found", code: "not_found", err: errs.New(errs.CodeNotFound, "")},
		{name: "forbidden", code: "forbidden", err: errs.New(errs.CodeForbidden, "")},
		{name: "validation_failed", code: "validation_failed", detail: "/ops/0: patch_path_forbidden",
			err: errs.New(errs.CodeValidationFailed, "x").WithFields(errs.FieldError{Pointer: "/ops/0", Code: "patch_path_forbidden"})},
		{name: "base_version_conflict", code: "base_version_conflict", err: errs.New(errs.CodeBaseVersionConflict, "")},
		{name: "conflict", code: "conflict", detail: "run_not_active", err: errs.New(errs.CodeConflict, "").WithReason("run_not_active")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			p := &fakePatcher{err: tc.err}
			e, c, fc := runUpdateEnv(t, p, tc.deny)
			e.send(c, "run.update", runUpdateBody(ids.New()))
			wantOpen(t, fc)
			frames := sent[app.ErrorFrame](fc)
			want := app.ErrorFrame{Op: "run.update", Code: tc.code, Detail: tc.detail, Ref: "r-1"}
			if len(frames) != 1 || frames[0] != want {
				t.Fatalf("error frames %+v, want %+v", frames, want)
			}
			if patches, _ := p.calls(); tc.deny && len(patches) != 0 {
				t.Fatalf("patched %d times past the rate limit", len(patches))
			}
		})
	}
	t.Run("malformed", func(t *testing.T) {
		t.Parallel()
		p := &fakePatcher{}
		e, c, fc := runUpdateEnv(t, p, false)
		e.gw.Handle(context.Background(), c, "run.update", json.RawMessage(`{"run_id":"`+ids.New().String()+`","ops":"replace"}`))
		wantClosed(t, fc, domain.CloseDecodeError)
		if patches, _ := p.calls(); len(patches) != 0 {
			t.Fatalf("patched %d times for a malformed body", len(patches))
		}
	})
}
