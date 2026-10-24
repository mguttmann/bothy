// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"slices"
	"strings"
	"time"

	"github.com/OWNER/bothy/services/server/internal/billing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// codeInvalid is the errors[].code of a broken rule (the list of M1-18).
const codeInvalid = "invalid"

// estimatesList is the list name of the cursors of listCostEstimates.
const estimatesList = "cost_estimates"

// estimatesKey is the keyset position of a cursor: the last run of the previous page.
type estimatesKey struct {
	After string `json:"after"`
}

// Estimates implements billing.Costs (listCostEstimates, cost.reconcile.read; M2-28 design note
// 12): authorize first (an owner in a session or an app with scope costs:reconcile; no audit of an
// allowed read, authz audits a denial), then one page of the estimates per run of the agents in
// the range, by run id ascending. The read is instance-level (R-AZ-28): no visible-channel
// predicate and no per-item check. The cursor is bound to the principal, the range and the agents.
//
// Errors: forbidden, unauthenticated, cursor_invalid, validation_failed (invalid at /to when the
// range is empty or reversed).
func (s *Service) Estimates(ctx context.Context, q billing.EstimatesQuery) (billing.EstimatePage, error) {
	if err := s.check(ctx, authorize.ActionCostReconcileRead, instance); err != nil {
		return billing.EstimatePage{}, err
	}
	if !q.From.Before(q.To) {
		return billing.EstimatePage{}, errs.New(errs.CodeValidationFailed, "the range is empty").
			WithFields(errs.FieldError{Pointer: "/to", Code: codeInvalid})
	}
	agents := slices.Clone(q.AgentIDs)
	slices.SortFunc(agents, compareIDs)
	agents = slices.Compact(agents)
	binding := estimatesBinding(caller(ctx).ID, q.From, q.To, agents)
	f := EstimatesFilter{From: q.From, To: q.To, AgentIDs: agents}
	if q.Cursor != "" {
		if s.d.Cursors == nil {
			return billing.EstimatePage{}, errs.New(errs.CodeInternalError, "billing: no cursor codec")
		}
		var k estimatesKey
		if err := s.d.Cursors.Decode(estimatesList, binding, q.Cursor, &k); err != nil {
			return billing.EstimatePage{}, err
		}
		after, err := ids.Parse(k.After)
		if err != nil {
			return billing.EstimatePage{}, errs.New(errs.CodeCursorInvalid, "")
		}
		f.After = after
	}
	limit := s.pageSize(q.Limit)
	items, err := s.d.Repo.Estimates(ctx, f, limit+1)
	if err != nil {
		return billing.EstimatePage{}, err
	}
	out := billing.EstimatePage{Items: items}
	if len(items) > limit {
		out.Items = items[:limit]
		if s.d.Cursors == nil {
			return billing.EstimatePage{}, errs.New(errs.CodeInternalError, "billing: no cursor codec")
		}
		next, err := s.d.Cursors.Encode(estimatesList, binding, estimatesKey{After: out.Items[limit-1].RunID.String()})
		if err != nil {
			return billing.EstimatePage{}, errs.Wrap(errs.CodeInternalError, err, "billing: encoding a cursor")
		}
		out.NextCursor = next
	}
	return out, nil
}

// pageSize is the page size of a request: 0 is api.page_size_default, larger values are capped at
// api.page_size_max; at least 1.
func (s *Service) pageSize(limit int) int {
	if limit <= 0 {
		limit = s.cfg.PageSizeDefault
	}
	if s.cfg.PageSizeMax > 0 && limit > s.cfg.PageSizeMax {
		limit = s.cfg.PageSizeMax
	}
	return max(limit, 1)
}

// estimatesBinding is the canonical filter of a cursor: principal, range and the sorted agents.
func estimatesBinding(principal ids.ID, from, to time.Time, agents []ids.ID) []byte {
	var b strings.Builder
	b.WriteString(principal.String())
	b.WriteString("\x00")
	b.WriteString(from.UTC().Format(time.RFC3339Nano))
	b.WriteString("\x00")
	b.WriteString(to.UTC().Format(time.RFC3339Nano))
	for _, a := range agents {
		b.WriteString("\x00")
		b.WriteString(a.String())
	}
	return []byte(b.String())
}
