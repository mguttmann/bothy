// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/runs"
)

// The REST operations of the wake-ups (AR-04, docs/spec/09-agents.md 5.8.2): createRunWake,
// recoverRunWake, listPendingRunWakes, checkRunWake, fireRunWake, cancelRunWake, listThreadWakes
// and checkRunLaunch over runs.Wakes. The use cases authorize; the handlers only convert.

// BindWakes binds the wake-up operations; internal/wire calls it once after BindWakes of the use
// cases. A nil w is ignored.
func (h *Runs) BindWakes(w runs.Wakes) {
	if w != nil {
		h.wakes = w
	}
}

// wakeOps returns the bound wake-up operations; without them every operation answers
// internal_error.
func (h *Runs) wakeOps() (runs.Wakes, error) {
	if h.wakes == nil {
		return nil, errs.New(errs.CodeInternalError, "runs: the wake-ups are not bound")
	}
	return h.wakes, nil
}

// CreateRunWake implements rest.StrictServerInterface (POST /runs/{run_id}/wakes).
func (h *Runs) CreateRunWake(ctx context.Context, req rest.CreateRunWakeRequestObject) (rest.CreateRunWakeResponseObject, error) {
	w, err := h.wakeOps()
	if err != nil {
		return nil, err
	}
	b := req.Body
	in := runs.CreateWake{
		RunID: ids.ID(req.RunId), Kind: string(b.Kind), Reason: string(b.Reason), Label: deref(b.Label), Condition: conditionIn(b.Condition),
		NotBefore: timeOf(b.NotBefore), Deadline: timeOf(b.Deadline), Outage: derefInt(b.Outage),
	}
	if b.Deferred != nil {
		in.Deferred = *b.Deferred
	}
	res, err := w.Create(ctx, in)
	if err != nil {
		return nil, err
	}
	if !res.Created {
		return rest.CreateRunWake200JSONResponse{Body: WakeREST(res.Wake)}, nil
	}
	return rest.CreateRunWake201JSONResponse{Body: WakeREST(res.Wake)}, nil
}

// RecoverRunWake implements rest.StrictServerInterface (POST /runs/{run_id}/wakes/recover).
func (h *Runs) RecoverRunWake(ctx context.Context, req rest.RecoverRunWakeRequestObject) (rest.RecoverRunWakeResponseObject, error) {
	w, err := h.wakeOps()
	if err != nil {
		return nil, err
	}
	b := req.Body
	res, err := w.Recover(ctx, runs.RecoverWake{RunID: ids.ID(req.RunId), Reason: string(b.Reason), Label: deref(b.Label), NotBefore: timeOf(b.NotBefore)})
	if err != nil {
		return nil, err
	}
	if !res.Created {
		return rest.RecoverRunWake200JSONResponse{Body: WakeREST(res.Wake)}, nil
	}
	return rest.RecoverRunWake201JSONResponse{Body: WakeREST(res.Wake)}, nil
}

// ListPendingRunWakes implements rest.StrictServerInterface (GET /agents/me/wakes).
func (h *Runs) ListPendingRunWakes(ctx context.Context, _ rest.ListPendingRunWakesRequestObject) (rest.ListPendingRunWakesResponseObject, error) {
	w, err := h.wakeOps()
	if err != nil {
		return nil, err
	}
	list, err := w.ListPending(ctx)
	if err != nil {
		return nil, err
	}
	out := rest.RunWakeList{Items: make([]rest.RunWake, 0, len(list))}
	for _, x := range list {
		out.Items = append(out.Items, WakeREST(x))
	}
	return rest.ListPendingRunWakes200JSONResponse{Body: out}, nil
}

// CheckRunWake implements rest.StrictServerInterface (POST /wakes/{wake_id}/check).
func (h *Runs) CheckRunWake(ctx context.Context, req rest.CheckRunWakeRequestObject) (rest.CheckRunWakeResponseObject, error) {
	w, err := h.wakeOps()
	if err != nil {
		return nil, err
	}
	c, err := w.Check(ctx, ids.ID(req.WakeId))
	if err != nil {
		return nil, err
	}
	return rest.CheckRunWake200JSONResponse{Body: checkREST(c)}, nil
}

// FireRunWake implements rest.StrictServerInterface (POST /wakes/{wake_id}/fire).
func (h *Runs) FireRunWake(ctx context.Context, req rest.FireRunWakeRequestObject) (rest.FireRunWakeResponseObject, error) {
	w, err := h.wakeOps()
	if err != nil {
		return nil, err
	}
	b := req.Body
	in := runs.FireWake{WakeID: ids.ID(req.WakeId), Outcome: string(b.Outcome), ResultText: deref(b.ResultText), Reason: deref(b.Reason)}
	if b.RunId != nil {
		in.RunID = ids.ID(*b.RunId)
	}
	x, err := w.Fire(ctx, in)
	if err != nil {
		return nil, err
	}
	return rest.FireRunWake200JSONResponse{Body: WakeREST(x)}, nil
}

// CancelRunWake implements rest.StrictServerInterface (POST /wakes/{wake_id}/cancel).
func (h *Runs) CancelRunWake(ctx context.Context, req rest.CancelRunWakeRequestObject) (rest.CancelRunWakeResponseObject, error) {
	w, err := h.wakeOps()
	if err != nil {
		return nil, err
	}
	in := runs.CancelWake{WakeID: ids.ID(req.WakeId)}
	if req.Body != nil {
		in.Reason = deref(req.Body.Reason)
	}
	x, err := w.Cancel(ctx, in)
	if err != nil {
		return nil, err
	}
	return rest.CancelRunWake200JSONResponse{Body: WakeREST(x)}, nil
}

// ListThreadWakes implements rest.StrictServerInterface (GET /threads/{thread_id}/wakes).
func (h *Runs) ListThreadWakes(ctx context.Context, req rest.ListThreadWakesRequestObject) (rest.ListThreadWakesResponseObject, error) {
	w, err := h.wakeOps()
	if err != nil {
		return nil, err
	}
	page, err := w.ListByThread(ctx, runs.ThreadWakesQuery{ThreadID: ids.ID(req.ThreadId), Cursor: deref(req.Params.Cursor), Limit: derefInt(req.Params.Limit)})
	if err != nil {
		return nil, err
	}
	out := rest.RunWakePage{Items: make([]rest.RunWake, 0, len(page.Items))}
	for _, x := range page.Items {
		out.Items = append(out.Items, WakeREST(x))
	}
	if page.NextCursor != "" {
		out.NextCursor = &page.NextCursor
	}
	return rest.ListThreadWakes200JSONResponse{Body: out}, nil
}

// CheckRunLaunch implements rest.StrictServerInterface (POST /runs/{run_id}/launch-check).
func (h *Runs) CheckRunLaunch(ctx context.Context, req rest.CheckRunLaunchRequestObject) (rest.CheckRunLaunchResponseObject, error) {
	w, err := h.wakeOps()
	if err != nil {
		return nil, err
	}
	c, err := w.LaunchCheck(ctx, ids.ID(req.RunId))
	if err != nil {
		return nil, err
	}
	return rest.CheckRunLaunch200JSONResponse{Body: checkREST(c)}, nil
}

// checkREST converts the answer of an allowed check (schema RunWakeCheckResult).
func checkREST(c runs.WakeCheck) rest.RunWakeCheckResult {
	return rest.RunWakeCheckResult{ToolClass: rest.ToolClass(c.ToolClass), BillingMode: rest.BillingMode(c.BillingMode)}
}

// conditionIn converts the condition of a request; nil is the empty condition.
func conditionIn(c *rest.RunWakeCondition) runs.WakeCondition {
	if c == nil {
		return runs.WakeCondition{}
	}
	return runs.WakeCondition{Repository: deref(c.Repository), Number: derefInt(c.Number), HeadCommit: deref(c.HeadCommit), CommandLine: deref(c.CommandLine)}
}

// timeOf converts an optional timestamp; nil is zero.
func timeOf(t *time.Time) time.Time {
	if t == nil {
		return time.Time{}
	}
	return t.UTC()
}

// WakeREST converts a wake-up (schema RunWake); empty values become absent members, never null
// (ADR 0053).
func WakeREST(w runs.Wake) rest.RunWake {
	out := rest.RunWake{
		Id: rest.Uuid(w.ID), AgentId: rest.Uuid(w.AgentID), ThreadId: rest.Uuid(w.ThreadID), SourceRunId: rest.Uuid(w.SourceRunID),
		TriggeredBy: rest.Uuid(w.TriggeredBy), Kind: rest.RunWakeKind(w.Kind), Reason: rest.RunWakeReason(w.Reason), Label: w.Label,
		State: rest.RunWakeState(w.State), StateReason: optionalString(w.StateReason), ResultText: optionalString(w.ResultText),
		NotBefore: w.NotBefore.UTC().Truncate(time.Millisecond), Deadline: w.Deadline.UTC().Truncate(time.Millisecond),
		CreatedAt: w.CreatedAt.UTC().Truncate(time.Millisecond), EndedAt: optionalTime(w.EndedAt),
		FiredRunId: optionalID(w.FiredRunID), FiredIntoRunId: optionalID(w.FiredIntoRunID), Version: w.Version,
		Condition: rest.RunWakeCondition{
			Repository: optionalString(w.Condition.Repository), HeadCommit: optionalString(w.Condition.HeadCommit),
			CommandLine: optionalString(w.Condition.CommandLine),
		},
	}
	if w.Condition.Number != 0 {
		n := w.Condition.Number
		out.Condition.Number = &n
	}
	if w.Outage != 0 {
		o := w.Outage
		out.Outage = &o
	}
	return out
}
