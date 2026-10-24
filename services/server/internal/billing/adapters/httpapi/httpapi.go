// SPDX-License-Identifier: Apache-2.0

// Package httpapi holds the REST handlers of the billing module as part of
// rest.StrictServerInterface (M2-22): getThreadCosts, listPrices, createPrice and closePrice of
// docs/api/openapi.yaml. The use cases authorize; the handlers only convert and pass errors to the
// problem writer of the chain unchanged. The Idempotency-Key of createPrice and closePrice is
// handled by the middleware of the chain (M1-20).
//
// Example:
//
//	h := httpapi.New(svc, svc)
//	built, err := routes.Build(wire.Handlers{Billing: h}, wire.RouteOptions(), cat)
package httpapi

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/billing"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Billing implements the REST operations of billing.
type Billing struct {
	costs  billing.Costs
	prices billing.Prices
}

// New returns the handlers over the ports of billing.
func New(c billing.Costs, p billing.Prices) *Billing { return &Billing{costs: c, prices: p} }

// GetThreadCosts implements rest.StrictServerInterface (GET /threads/{thread_id}/costs).
func (h *Billing) GetThreadCosts(ctx context.Context, req rest.GetThreadCostsRequestObject) (rest.GetThreadCostsResponseObject, error) {
	tc, err := h.costs.ThreadCosts(ctx, ids.ID(req.ThreadId))
	if err != nil {
		return nil, err
	}
	return rest.GetThreadCosts200JSONResponse{Body: BreakdownREST(tc)}, nil
}

// ListPrices implements rest.StrictServerInterface (GET /prices).
func (h *Billing) ListPrices(ctx context.Context, _ rest.ListPricesRequestObject) (rest.ListPricesResponseObject, error) {
	ps, err := h.prices.List(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]rest.Price, len(ps))
	for i, p := range ps {
		items[i] = PriceREST(p)
	}
	return rest.ListPrices200JSONResponse{Body: rest.PriceList{Items: items}}, nil
}

// CreatePrice implements rest.StrictServerInterface (POST /prices): 201 with the new row.
func (h *Billing) CreatePrice(ctx context.Context, req rest.CreatePriceRequestObject) (rest.CreatePriceResponseObject, error) {
	if req.Body == nil {
		return nil, missingBody()
	}
	b := req.Body
	cmd := billing.CreatePrice{Provider: b.Provider, Model: b.Model, Unit: string(b.Unit), PriceMicros: b.PriceMicros,
		Currency: b.Currency, ValidFrom: b.ValidFrom, SourceURL: b.SourceUrl, RetrievedAt: b.RetrievedAt}
	if b.ValidTo != nil {
		cmd.ValidTo = *b.ValidTo
	}
	p, err := h.prices.Create(ctx, cmd)
	if err != nil {
		return nil, err
	}
	return rest.CreatePrice201JSONResponse{Body: PriceREST(p)}, nil
}

// ClosePrice implements rest.StrictServerInterface (POST /prices/{price_id}/close).
func (h *Billing) ClosePrice(ctx context.Context, req rest.ClosePriceRequestObject) (rest.ClosePriceResponseObject, error) {
	if req.Body == nil {
		return nil, missingBody()
	}
	p, err := h.prices.Close(ctx, billing.ClosePrice{PriceID: ids.ID(req.PriceId), ValidTo: req.Body.ValidTo})
	if err != nil {
		return nil, err
	}
	return rest.ClosePrice200JSONResponse{Body: PriceREST(p)}, nil
}

// BreakdownREST converts the thread costs to the schema CostBreakdown: never null, costs absent
// when the display rule hid them, agent_session_id absent without a session.
func BreakdownREST(tc billing.ThreadCosts) rest.CostBreakdown {
	out := rest.CostBreakdown{ThreadId: rest.Uuid(tc.ThreadID), Totals: []rest.CostTotal{}, Runs: []rest.RunCost{}}
	for _, t := range tc.Totals {
		out.Totals = append(out.Totals, rest.CostTotal{BillingMode: rest.BillingMode(t.BillingMode), AmountMicros: t.AmountMicros,
			Currency: t.Currency, IsEstimate: t.IsEstimate})
	}
	for _, r := range tc.Runs {
		rc := rest.RunCost{RunId: rest.Uuid(r.RunID), BillingMode: rest.BillingMode(r.BillingMode), Unpriced: r.Unpriced,
			Tokens: rest.TokenCounts{TokensIn: r.Tokens.In, TokensOut: r.Tokens.Out, TokensCacheRead: r.Tokens.CacheRead,
				TokensCacheWrite: r.Tokens.CacheWrite, TokensCacheWrite1h: r.Tokens.CacheWrite1h}}
		if !r.AgentSessionID.IsZero() {
			s := rest.Uuid(r.AgentSessionID)
			rc.AgentSessionId = &s
		}
		if r.Costs != nil {
			costs := make([]rest.Money, len(r.Costs))
			for i, m := range r.Costs {
				costs[i] = rest.Money{AmountMicros: m.AmountMicros, Currency: m.Currency, IsEstimate: m.IsEstimate}
			}
			rc.Costs = &costs
		}
		out.Runs = append(out.Runs, rc)
	}
	return out
}

// PriceREST converts a price row to the schema Price; valid_to is absent while the row is open.
func PriceREST(p billing.Price) rest.Price {
	out := rest.Price{Id: rest.Uuid(p.ID), Provider: p.Provider, Model: p.Model, Unit: rest.PriceUnit(p.Unit), PriceMicros: p.PriceMicros,
		Currency: p.Currency, ValidFrom: timestamp(p.ValidFrom), SourceUrl: p.SourceURL, RetrievedAt: timestamp(p.RetrievedAt)}
	if !p.ValidTo.IsZero() {
		t := timestamp(p.ValidTo)
		out.ValidTo = &t
	}
	return out
}

// timestamp is t in UTC at the millisecond precision of the schema Timestamp.
func timestamp(t time.Time) time.Time { return t.UTC().Truncate(time.Millisecond) }

// missingBody is the 400 of a request without its required body; the schema validation of the
// chain normally answers first.
func missingBody() error {
	return errs.New(errs.CodeValidationFailed, "the request body is required").
		WithFields(errs.FieldError{Pointer: "", Code: "required", Detail: "the request body is required"})
}
