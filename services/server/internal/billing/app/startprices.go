// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"

	"github.com/OWNER/bothy/services/server/internal/billing/domain"
	"github.com/OWNER/bothy/services/server/internal/billing/domain/startprices"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// attrRows is the audit attribute of price.load_start: the number of rows loaded.
const attrRows = "rows"

// StartPriceLoader is the system action price.load_start (docs/spec/10-backend-policy.md 12.3,
// AUD-PKG-R2-owner-server): it loads the embedded start file into an empty price table once,
// right after bothy setup bootstrap-owner created the first owner. It runs without a principal
// and never authorizes; later maintenance stays with createPrice and closePrice.
type StartPriceLoader struct {
	uow   txn.UnitOfWork
	repo  Repository
	audit auditing.Recorder
	clock clock.Clock
}

// NewStartPriceLoader returns the loader over the unit of work, the price storage and the audit
// recorder; clk nil means clock.System(). Errors: a missing collaborator.
func NewStartPriceLoader(uow txn.UnitOfWork, repo Repository, audit auditing.Recorder, clk clock.Clock) (*StartPriceLoader, error) {
	if uow == nil || repo == nil || audit == nil {
		return nil, errors.New("billing: the start price loader needs a unit of work, the storage and the audit")
	}
	if clk == nil {
		clk = clock.System()
	}
	return &StartPriceLoader{uow: uow, repo: repo, audit: audit, clock: clk}, nil
}

// Load inserts every row of the start file with createdBy (the first owner) as created_by and one
// audit entry price.load_start with the number of rows, all in one unit of work. A price table
// that has any row, closed ones included, is left as it is and Load returns 0 without an audit
// entry: the owner or an earlier load already set the prices.
//
// Errors: those of the start file, the storage and the audit; nothing is written then.
func (l *StartPriceLoader) Load(ctx context.Context, createdBy ids.ID) (int, error) {
	rows, err := startprices.Rows()
	if err != nil {
		return 0, err
	}
	loaded := 0
	err = l.uow.Do(ctx, func(ctx context.Context) error {
		loaded = 0
		have, err := l.repo.ListPrices(ctx)
		if err != nil || len(have) > 0 {
			return err
		}
		for _, in := range rows {
			p := domain.PriceRow{ID: ids.New(), Provider: in.Provider, Model: in.Model, Unit: domain.Unit(in.Unit),
				PriceMicros: in.PriceMicros, Currency: in.Currency, ValidFrom: in.ValidFrom, ValidTo: in.ValidTo,
				SourceURL: in.SourceURL, RetrievedAt: in.RetrievedAt}
			if err := l.repo.InsertPrice(ctx, p, createdBy); err != nil {
				return err
			}
		}
		loaded = len(rows)
		return l.audit.Record(ctx, auditing.Entry{Action: string(authorize.ActionPriceLoadStart), Decision: auditing.DecisionAllow,
			ResourceKind: string(authorize.ResourceInstance), Attrs: auditing.Attrs{attrRows: auditing.Int(int64(loaded))}})
	})
	if err != nil {
		return 0, err
	}
	return loaded, nil
}
