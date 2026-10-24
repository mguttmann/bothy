// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/billing"
	"github.com/OWNER/bothy/services/server/internal/billing/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// ValidateReports implements billing.Costs with the pure rules of domain.ValidateReports (M2-22
// design note 3), so runs checks a finishRun body before its unit of work without importing a
// foreign domain (02 section 5).
func (s *Service) ValidateReports(reports []billing.CostReport) error {
	return domain.ValidateReports(toReports(reports))
}

// BookRun implements billing.Costs inside the caller's unit of work (M2-22 design note 5 e): for
// each report in order it loads the price set of provider, model and cmd.OccurredAt with FOR
// SHARE, prices the report (domain.Book) and inserts one entry with a new UUIDv7, is_estimate
// true and no reconciliation columns. It never authorizes: run.finish authorized the unit.
//
// Errors: internal outside a unit of work; validation_failed (too_large, /<i>/amount_micros)
// for an amount at the bound of 06; the errors of the repository.
func (s *Service) BookRun(ctx context.Context, cmd billing.BookRun) (billing.Booking, error) {
	if err := needUnit(ctx, "BookRun"); err != nil {
		return billing.Booking{}, err
	}
	if len(cmd.Reports) == 0 {
		return billing.Booking{}, nil
	}
	reports := toReports(cmd.Reports)
	sets := make([][]domain.PriceRow, len(reports))
	for i, r := range reports {
		set, err := s.d.Repo.LockPriceSet(ctx, r.Provider, r.Model, cmd.OccurredAt)
		if err != nil {
			return billing.Booking{}, err
		}
		sets[i] = set
	}
	priced, sums, err := domain.Book(reports, sets)
	if err != nil {
		return billing.Booking{}, err
	}
	entries := make([]NewEntry, len(reports))
	out := billing.Booking{Entries: make([]billing.Entry, len(reports))}
	for i, r := range reports {
		id := ids.New()
		entries[i] = NewEntry{
			ID: id, RunID: cmd.RunID, AgentID: cmd.AgentID, ThreadID: cmd.ThreadID, ChannelID: cmd.ChannelID,
			SpaceID: cmd.SpaceID, AgentSessionID: cmd.AgentSessionID, BillingMode: cmd.BillingMode,
			Provider: r.Provider, Model: r.Model, Tokens: r.Tokens, AmountMicros: priced[i].AmountMicros,
			Currency: priced[i].Currency, Source: priced[i].Source, OccurredAt: cmd.OccurredAt,
		}
		out.Entries[i] = billing.Entry{ID: id, Provider: r.Provider, Model: r.Model, Source: priced[i].Source,
			AmountMicros: priced[i].AmountMicros, Currency: priced[i].Currency, IsEstimate: true}
	}
	if err := s.d.Repo.InsertEntries(ctx, entries); err != nil {
		return billing.Booking{}, err
	}
	for _, c := range sums {
		out.Sums = append(out.Sums, billing.CurrencySum{Currency: c.Currency, AmountMicros: c.AmountMicros})
	}
	return out, nil
}

// ThreadCosts implements billing.Costs (getThreadCosts, cost.read.thread; M2-22 design note 8):
// authorize first, so an invisible and an unknown thread give the same not_found (AZ-P4); then
// the valid entries of the thread, whatever channel they were booked in, per run and per currency
// with the display rule of 10 section 12.2. A thread without entries gives empty lists.
func (s *Service) ThreadCosts(ctx context.Context, threadID ids.ID) (billing.ThreadCosts, error) {
	if err := s.check(ctx, authorize.ActionCostReadThread, authorize.Resource{Kind: authorize.ResourceThread, ID: threadID}); err != nil {
		return billing.ThreadCosts{}, err
	}
	es, err := s.d.Repo.ThreadEntries(ctx, threadID)
	if err != nil {
		return billing.ThreadCosts{}, err
	}
	b := domain.Breakdown(es, s.cfg.ShowPersonalEstimates)
	out := billing.ThreadCosts{ThreadID: threadID, Totals: []billing.CostTotal{}, Runs: []billing.RunCost{}}
	for _, t := range b.Totals {
		out.Totals = append(out.Totals, billing.CostTotal{BillingMode: t.BillingMode,
			Money: billing.Money{AmountMicros: t.AmountMicros, Currency: t.Currency, IsEstimate: t.IsEstimate}})
	}
	for _, r := range b.Runs {
		rc := billing.RunCost{RunID: r.RunID, AgentSessionID: r.AgentSessionID, BillingMode: r.BillingMode,
			Tokens: toTokenCounts(r.Tokens), Unpriced: r.Unpriced}
		for _, m := range r.Costs {
			rc.Costs = append(rc.Costs, billing.Money{AmountMicros: m.AmountMicros, Currency: m.Currency, IsEstimate: m.IsEstimate})
		}
		out.Runs = append(out.Runs, rc)
	}
	return out, nil
}

// Totals implements billing.Costs (M2-22 design note 7): the valid sums of one level per billing
// mode and currency. A channel filter that is neither All nor has IDs returns nothing (fail
// closed, 03 principle 12); ForDisplay hides personal amounts unless
// billing.show_personal_estimates. It never authorizes: its callers do.
//
// Errors: internal for an unknown level; the errors of the repository.
func (s *Service) Totals(ctx context.Context, q billing.TotalsQuery) ([]billing.Total, error) {
	switch q.Level {
	case billing.LevelRun, billing.LevelThread, billing.LevelSession, billing.LevelAgent, billing.LevelSpace, billing.LevelInstance:
	default:
		return nil, errs.New(errs.CodeInternalError, "billing: unknown level of a total")
	}
	if !q.Channels.All && len(q.Channels.IDs) == 0 {
		return []billing.Total{}, nil
	}
	ts, err := s.d.Repo.Totals(ctx, TotalsFilter{Level: q.Level, ID: q.ID, From: q.From, To: q.To,
		AllChannels: q.Channels.All, ChannelIDs: q.Channels.IDs})
	if err != nil {
		return nil, err
	}
	out := make([]billing.Total, 0, len(ts))
	for _, t := range ts {
		bt := billing.Total{BillingMode: t.BillingMode, Currency: t.Currency, IsEstimate: t.IsEstimate,
			Tokens: toTokenCounts(t.Tokens), Entries: t.Entries}
		if !q.ForDisplay || domain.ShowAmounts(t.BillingMode, s.cfg.ShowPersonalEstimates) {
			amount := t.AmountMicros
			bt.AmountMicros = &amount
		}
		out = append(out, bt)
	}
	return out, nil
}

// toReports maps the port reports to the domain.
func toReports(rs []billing.CostReport) []domain.Report {
	out := make([]domain.Report, len(rs))
	for i, r := range rs {
		out[i] = domain.Report{Provider: r.Provider, Model: r.Model, AmountMicros: r.AmountMicros, Currency: r.Currency,
			Tokens: domain.Tokens{In: r.Tokens.In, Out: r.Tokens.Out, CacheRead: r.Tokens.CacheRead,
				CacheWrite: r.Tokens.CacheWrite, CacheWrite1h: r.Tokens.CacheWrite1h}}
	}
	return out
}

// toTokenCounts maps domain tokens to the port.
func toTokenCounts(t domain.Tokens) billing.TokenCounts {
	return billing.TokenCounts{In: t.In, Out: t.Out, CacheRead: t.CacheRead, CacheWrite: t.CacheWrite, CacheWrite1h: t.CacheWrite1h}
}
