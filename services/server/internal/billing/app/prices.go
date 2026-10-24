// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"

	"github.com/OWNER/bothy/services/server/internal/billing"
	"github.com/OWNER/bothy/services/server/internal/billing/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// The reasons of the conflicts of createPrice and closePrice (rulings D-M2-22-17 and D-M2-22-18).
const (
	reasonPriceOverlap          = "price_overlap"
	reasonPriceCurrencyMismatch = "price_currency_mismatch"
	reasonPriceClosed           = "price_closed"
	reasonPriceInUse            = "price_in_use"
)

// The audit attributes of a price change (10 section 14 row "Prices": row, source, retrieval
// time; M2-22 design note 14).
const (
	attrPriceID     = "price_id"
	attrOp          = "op"
	attrValidFrom   = "valid_from"
	attrValidTo     = "valid_to"
	attrRetrievedAt = "retrieved_at"
	attrSourceURL   = "source_url"
	attrNoChange    = "no_change"
	opCreate        = "create"
	opClose         = "close"
)

// instance is the resource of price.read and price.manage.
var instance = authorize.Resource{Kind: authorize.ResourceInstance}

// List implements billing.Prices (listPrices, price.read; M2-22 design note 10): owner only,
// no step-up and no audit; every row, closed ones included.
func (s *Service) List(ctx context.Context) ([]billing.Price, error) {
	if err := s.check(ctx, authorize.ActionPriceRead, instance); err != nil {
		return nil, err
	}
	rows, err := s.d.Repo.ListPrices(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]billing.Price, len(rows))
	for i, r := range rows {
		out[i] = toPrice(r)
	}
	return out, nil
}

// Create implements billing.Prices (createPrice, price.manage; M2-22 design note 11) in one unit
// of work: authorize first (owner with step-up; authz audits a denial), validate the body, refuse
// an overlapping row of the same provider and model in another currency (409
// price_currency_mismatch), insert with a new UUIDv7 and the actor as created_by (an exclusion or
// unique violation is 409 price_overlap) and audit the change. A valid_from in the past is
// allowed: it cannot change a computed booking (design note 11 d).
func (s *Service) Create(ctx context.Context, cmd billing.CreatePrice) (billing.Price, error) {
	var out domain.PriceRow
	err := s.d.UoW.Do(ctx, func(ctx context.Context) error {
		if err := s.check(ctx, authorize.ActionPriceManage, instance); err != nil {
			return err
		}
		in := domain.PriceInput{Provider: cmd.Provider, Model: cmd.Model, Unit: cmd.Unit, PriceMicros: cmd.PriceMicros,
			Currency: cmd.Currency, ValidFrom: cmd.ValidFrom, ValidTo: cmd.ValidTo, SourceURL: cmd.SourceURL, RetrievedAt: cmd.RetrievedAt}
		if err := domain.ValidatePrice(in, s.d.Clock.Now()); err != nil {
			return err
		}
		mismatch, err := s.d.Repo.CurrencyMismatch(ctx, in.Provider, in.Model, in.Currency, in.ValidFrom, in.ValidTo)
		if err != nil {
			return err
		}
		if mismatch {
			return errs.New(errs.CodeConflict, "a row of this provider and model in another currency overlaps the validity").
				WithReason(reasonPriceCurrencyMismatch)
		}
		out = domain.PriceRow{ID: ids.New(), Provider: in.Provider, Model: in.Model, Unit: domain.Unit(in.Unit),
			PriceMicros: in.PriceMicros, Currency: in.Currency, ValidFrom: in.ValidFrom, ValidTo: in.ValidTo,
			SourceURL: in.SourceURL, RetrievedAt: in.RetrievedAt}
		if err := s.d.Repo.InsertPrice(ctx, out, caller(ctx).ID); err != nil {
			if errors.Is(err, ErrPriceOverlap) {
				return errs.New(errs.CodeConflict, "the validity overlaps a row of the same provider, model and unit").
					WithReason(reasonPriceOverlap)
			}
			return err
		}
		return s.audit(ctx, out, opCreate, false)
	})
	if err != nil {
		return billing.Price{}, err
	}
	return toPrice(out), nil
}

// Close implements billing.Prices (closePrice, price.manage; M2-22 design note 12) in one unit of
// work: authorize first, lock the row FOR UPDATE (unknown: not_found), decide with
// domain.CloseDecision (the same end again is 200 with an audit entry no_change, another end on a
// closed row 409 price_closed), refuse an end that cuts off a computed booking (409
// price_in_use, R-DM-23), set valid_to and audit.
func (s *Service) Close(ctx context.Context, cmd billing.ClosePrice) (billing.Price, error) {
	var out domain.PriceRow
	err := s.d.UoW.Do(ctx, func(ctx context.Context) error {
		if err := s.check(ctx, authorize.ActionPriceManage, instance); err != nil {
			return err
		}
		var err error
		out, err = s.closeRow(ctx, cmd)
		return err
	})
	if err != nil {
		return billing.Price{}, err
	}
	return toPrice(out), nil
}

// closeRow is Close after authorize, inside its unit of work.
func (s *Service) closeRow(ctx context.Context, cmd billing.ClosePrice) (domain.PriceRow, error) {
	row, found, err := s.d.Repo.LockPrice(ctx, cmd.PriceID)
	if err != nil {
		return domain.PriceRow{}, err
	}
	if !found {
		return domain.PriceRow{}, errs.New(errs.CodeNotFound, "")
	}
	outcome, err := domain.CloseDecision(row, cmd.ValidTo)
	if errors.Is(err, domain.ErrPriceClosed) {
		return domain.PriceRow{}, errs.New(errs.CodeConflict, "the price row is already closed with another end").WithReason(reasonPriceClosed)
	}
	if err != nil {
		return domain.PriceRow{}, err
	}
	if outcome == domain.CloseNoChange {
		return row, s.audit(ctx, row, opClose, true)
	}
	inUse, err := s.d.Repo.PriceInUse(ctx, row, cmd.ValidTo)
	if err != nil {
		return domain.PriceRow{}, err
	}
	if inUse {
		return domain.PriceRow{}, errs.New(errs.CodeConflict, "a computed booking at or after the new end used this row").WithReason(reasonPriceInUse)
	}
	if err := s.d.Repo.SetValidTo(ctx, row.ID, cmd.ValidTo); err != nil {
		return domain.PriceRow{}, err
	}
	row.ValidTo = cmd.ValidTo
	return row, s.audit(ctx, row, opClose, false)
}

// audit records the allow entry of a price change with row, source and retrieval time.
func (s *Service) audit(ctx context.Context, p domain.PriceRow, op string, noChange bool) error {
	attrs := auditing.Attrs{
		attrPriceID: auditing.ID(p.ID), attrOp: auditing.Code(op), attrValidFrom: auditing.Time(p.ValidFrom),
		attrRetrievedAt: auditing.Time(p.RetrievedAt), attrSourceURL: auditing.SourceURL(p.SourceURL),
	}
	if !p.ValidTo.IsZero() {
		attrs[attrValidTo] = auditing.Time(p.ValidTo)
	}
	if noChange {
		attrs[attrNoChange] = auditing.Bool(true)
	}
	return s.d.Audit.Record(ctx, auditing.Entry{Action: string(authorize.ActionPriceManage), Decision: auditing.DecisionAllow,
		ResourceKind: string(authorize.ResourceInstance), Attrs: attrs})
}

// toPrice maps a row to the port.
func toPrice(r domain.PriceRow) billing.Price {
	return billing.Price{ID: r.ID, Provider: r.Provider, Model: r.Model, Unit: string(r.Unit), PriceMicros: r.PriceMicros,
		Currency: r.Currency, ValidFrom: r.ValidFrom, ValidTo: r.ValidTo, SourceURL: r.SourceURL, RetrievedAt: r.RetrievedAt}
}
