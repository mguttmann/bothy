// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"log/slog"

	"github.com/OWNER/bothy/services/server/internal/billing"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// Config are the configuration values of the billing use cases, bound by internal/wire.
type Config struct {
	// ShowPersonalEstimates is billing.show_personal_estimates: amounts of runs in mode personal
	// are shown as information too (10 section 12.2).
	ShowPersonalEstimates bool `config:"billing.show_personal_estimates"`
	// PageSizeDefault is api.page_size_default, the page size of listCostEstimates without limit.
	PageSizeDefault int `config:"api.page_size_default"`
	// PageSizeMax is api.page_size_max, the cap of a requested page size.
	PageSizeMax int `config:"api.page_size_max"`
}

// Deps are the collaborators of Service.
type Deps struct {
	// Repo reads and writes price_table and cost_entries.
	Repo Repository
	// UoW runs the price changes; BookRun joins the caller's unit.
	UoW txn.UnitOfWork
	// Authorizer decides cost.read.thread, price.read and price.manage.
	Authorizer authorize.Authorizer
	// Audit records the price changes (10 section 14 row "Prices").
	Audit auditing.Recorder
	// Clock is the time of a price change and reconciled_at; nil means clock.System().
	Clock clock.Clock
	// Scheduler inserts the job billing.reconcile (M2-28); nil refuses a reconciliation that
	// writes entries with an internal error.
	Scheduler ReconcileScheduler
	// Cursors encodes the cursors of listCostEstimates; nil refuses a request with a cursor or a
	// further page with an internal error.
	Cursors Cursors
	// Logger reports a run or session the job leaves unchanged because its valid entries mix
	// currencies; nil discards.
	Logger *slog.Logger
}

// Service implements billing.Costs and billing.Prices.
type Service struct {
	cfg Config
	d   Deps
}

// Compile-time checks: Service is both ports of billing.
var (
	_ billing.Costs  = (*Service)(nil)
	_ billing.Prices = (*Service)(nil)
)

// New returns the service. Errors: internal when Repo, UoW, Authorizer or Audit is missing.
func New(cfg Config, d Deps) (*Service, error) {
	if d.Repo == nil || d.UoW == nil || d.Authorizer == nil || d.Audit == nil {
		return nil, errs.New(errs.CodeInternalError, "billing: the service needs the repository, the unit of work, the authorizer and the recorder")
	}
	if d.Clock == nil {
		d.Clock = clock.System()
	}
	if d.Logger == nil {
		d.Logger = slog.New(slog.DiscardHandler)
	}
	return &Service{cfg: cfg, d: d}, nil
}

// caller returns the principal of the request in ctx; anonymous when none is set, which authorize
// refuses.
func caller(ctx context.Context) actor.Actor {
	if a, ok := actor.FromContext(ctx); ok {
		return a
	}
	return actor.Anonymous()
}

// check authorizes act on r for the principal of ctx and returns the decision as an error (nil
// for allow).
func (s *Service) check(ctx context.Context, act authorize.Action, r authorize.Resource) error {
	d, err := s.d.Authorizer.Authorize(ctx, caller(ctx), act, r)
	if err != nil {
		return err
	}
	return d.Err()
}

// needUnit refuses a call outside a unit of work.
func needUnit(ctx context.Context, what string) error {
	if !txn.InUnit(ctx) {
		return errs.New(errs.CodeInternalError, "billing: "+what+" outside a unit of work")
	}
	return nil
}
