// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"context"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/billing"
	"github.com/OWNER/bothy/services/server/internal/billing/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
)

// Cursors implements app.Cursors with the cursor codec of the HTTP layer (06 "Pagination").
type Cursors struct{}

// Compile-time check: Cursors is the cursor port of the billing use cases.
var _ app.Cursors = Cursors{}

// Encode implements app.Cursors with httpserver.EncodeCursor.
func (Cursors) Encode(list string, binding []byte, key any) (string, error) {
	return httpserver.EncodeCursor(list, binding, key)
}

// Decode implements app.Cursors with httpserver.DecodeCursor (400 cursor_invalid on a mismatch).
func (Cursors) Decode(list string, binding []byte, s string, key any) error {
	return httpserver.DecodeCursor(list, binding, s, key)
}

// ReconcileCosts implements rest.StrictServerInterface (POST /costs/reconcile, cost.reconcile):
// the owner's reconciliation by hand with step-up (M2-28).
func (h *Billing) ReconcileCosts(ctx context.Context, req rest.ReconcileCostsRequestObject) (rest.ReconcileCostsResponseObject, error) {
	if req.Body == nil {
		return nil, missingBody()
	}
	res, err := h.costs.Reconcile(ctx, reconcileInput(authorize.ActionCostReconcile, *req.Body))
	if err != nil {
		return nil, err
	}
	return rest.ReconcileCosts200JSONResponse{Body: resultREST(res)}, nil
}

// SubmitCostReconciliation implements rest.StrictServerInterface (POST /costs/reconciliations,
// cost.reconcile.submit): the submission of the reconciliation app (M2-28).
func (h *Billing) SubmitCostReconciliation(ctx context.Context,
	req rest.SubmitCostReconciliationRequestObject) (rest.SubmitCostReconciliationResponseObject, error) {
	if req.Body == nil {
		return nil, missingBody()
	}
	res, err := h.costs.Reconcile(ctx, reconcileInput(authorize.ActionCostReconcileSubmit, *req.Body))
	if err != nil {
		return nil, err
	}
	return rest.SubmitCostReconciliation200JSONResponse{Body: resultREST(res)}, nil
}

// ListCostEstimates implements rest.StrictServerInterface (GET /costs/estimates,
// cost.reconcile.read; M2-28 design note 12).
func (h *Billing) ListCostEstimates(ctx context.Context, req rest.ListCostEstimatesRequestObject) (rest.ListCostEstimatesResponseObject, error) {
	p := req.Params
	q := billing.EstimatesQuery{From: p.From, To: p.To, AgentIDs: make([]ids.ID, len(p.AgentId))}
	for i, a := range p.AgentId {
		q.AgentIDs[i] = ids.ID(a)
	}
	if p.Cursor != nil {
		q.Cursor = *p.Cursor
	}
	if p.Limit != nil {
		q.Limit = *p.Limit
	}
	page, err := h.costs.Estimates(ctx, q)
	if err != nil {
		return nil, err
	}
	out := rest.CostEstimatePage{Items: make([]rest.CostEstimate, len(page.Items))}
	for i, e := range page.Items {
		out.Items[i] = rest.CostEstimate{RunId: rest.Uuid(e.RunID), AgentId: rest.Uuid(e.AgentID), Provider: e.Provider, Currency: e.Currency,
			EstimateMicros: e.EstimateMicros, Reconciled: e.Reconciled}
	}
	if page.NextCursor != "" {
		next := page.NextCursor
		out.NextCursor = &next
	}
	return rest.ListCostEstimates200JSONResponse{Body: out}, nil
}

// reconcileInput converts the body CostReconcile with the action of the operation.
func reconcileInput(act authorize.Action, b rest.CostReconcile) billing.ReconcileInput {
	in := billing.ReconcileInput{Action: act, Provider: b.Provider, PeriodStart: b.PeriodStart, PeriodEnd: b.PeriodEnd,
		Entries: make([]billing.ReconcileEntry, len(b.Entries))}
	for i, e := range b.Entries {
		in.Entries[i] = billing.ReconcileEntry{RunID: ids.ID(e.RunId), AmountMicros: e.AmountMicros, Currency: e.Currency}
	}
	return in
}

// resultREST converts the result to the schema CostReconcileResult; unmatched_run_ids is never
// null (ADR 0053).
func resultREST(res billing.ReconcileResult) rest.CostReconcileResult {
	out := rest.CostReconcileResult{Reconciled: res.Reconciled, UnmatchedRunIds: make([]rest.Uuid, len(res.UnmatchedRunIDs))}
	for i, id := range res.UnmatchedRunIDs {
		out.UnmatchedRunIds[i] = rest.Uuid(id)
	}
	return out
}
