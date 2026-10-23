// SPDX-License-Identifier: Apache-2.0

package billing

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// The values of cost_entries.source (03 section 4.9; 10 section 12.1).
const (
	// SourceComputed is an amount computed from the price set of provider, model and occurred_at.
	SourceComputed = "computed"
	// SourceAdapterReported is the amount the adapter reported, used when no complete price set
	// exists.
	SourceAdapterReported = "adapter_reported"
	// SourceUnpriced is neither a price nor an amount of the adapter: amount 0, shown as
	// "Price missing".
	SourceUnpriced = "unpriced"
	// SourceReconciled is a correction entry of the reconciliation (M2-28).
	SourceReconciled = "reconciled"
)

// The billing modes of cost_entries.billing_mode (03 section 4.9).
const (
	// ModePersonal is a run on the subscription of one person: amounts are information only.
	ModePersonal = "personal"
	// ModeOrganization is a run billed to the organization.
	ModeOrganization = "organization"
)

// Costs books run costs and reads valid sums (02 section 4.3: booking, correction, sums). The
// billing module uses no other module port (02 section 4.4): its callers pass every copy it
// stores.
//
// BookRun and Totals never call authorize: BookRun runs inside the unit of work of run.finish,
// which runs authorized, and every caller of Totals authorizes first and passes the visible
// channels. ThreadCosts serves the REST adapter of billing and authorizes cost.read.thread for the
// principal of ctx (kernel/actor).
type Costs interface {
	// ValidateReports checks the reports of a finishRun body before its unit of work (M2-22
	// design note 3). The error is validation_failed with pointers relative to the list
	// ("/0/provider"); the caller prefixes its member ("/costs").
	ValidateReports(reports []CostReport) error
	// BookRun writes one cost entry per report inside the caller's unit of work and returns them
	// with the sums per currency (design notes 4 and 5 e). Errors: internal outside a unit of
	// work; validation_failed (too_large under /<i>/amount_micros) when a computed amount reaches
	// the integer bound of 06.
	BookRun(ctx context.Context, cmd BookRun) (Booking, error)
	// ThreadCosts returns the costs of a thread for the principal of ctx (getThreadCosts,
	// cost.read.thread): per run and per billing mode and currency, with the display rule of 10
	// section 12.2 applied. Errors: not_found for an unknown or invisible thread, forbidden.
	ThreadCosts(ctx context.Context, threadID ids.ID) (ThreadCosts, error)
	// Totals returns the valid sums of one level per billing mode and currency (design note 7),
	// ordered by billing mode, then currency.
	//
	// Example:
	//
	//	ts, err := costs.Totals(ctx, billing.TotalsQuery{Level: billing.LevelSpace, ID: space,
	//		Channels: billing.ChannelFilter{IDs: visible}, ForDisplay: true})
	Totals(ctx context.Context, q TotalsQuery) ([]Total, error)
}

// Prices maintains the price table (10 section 12.3) for the principal of ctx: listPrices
// (price.read) and createPrice and closePrice (price.manage, owner with step-up, audited).
type Prices interface {
	// List returns every row, closed ones included, ordered by provider, model, unit, valid_from
	// and id. Errors: forbidden (owner_only).
	List(ctx context.Context) ([]Price, error)
	// Create inserts a price row. Errors: forbidden, step_up_required, validation_failed, conflict
	// with reason price_overlap or price_currency_mismatch.
	Create(ctx context.Context, cmd CreatePrice) (Price, error)
	// Close ends the validity of a row. Errors: forbidden, step_up_required, not_found,
	// validation_failed, conflict with reason price_closed or price_in_use.
	Close(ctx context.Context, cmd ClosePrice) (Price, error)
}

// TokenCounts are the consumed tokens of one report, entry or sum (03 cost_entries).
type TokenCounts struct {
	// In and Out are tokens_in and tokens_out.
	In, Out int64
	// CacheRead is tokens_cache_read.
	CacheRead int64
	// CacheWrite is tokens_cache_write: cache writes with 5 minutes or without a stated duration.
	CacheWrite int64
	// CacheWrite1h is tokens_cache_write_1h: cache writes with 1 hour.
	CacheWrite1h int64
}

// CostReport is the cost of one model in one run as the adapter reports it (schema CostReport).
// Absent token fields are 0.
type CostReport struct {
	// Provider and Model name the model exactly as the runtime reports them.
	Provider, Model string
	// Tokens are the consumed tokens of this run.
	Tokens TokenCounts
	// AmountMicros is the adapter's amount; nil lets the server compute.
	AmountMicros *int64
	// Currency is the ISO 4217 code of the report.
	Currency string
}

// BookRun is the input of Costs.BookRun: the reports and the copies of the run at booking time.
type BookRun struct {
	// RunID, AgentID, ThreadID and ChannelID are the run and its copies.
	RunID, AgentID, ThreadID, ChannelID ids.ID
	// SpaceID is zero for a DM channel; AgentSessionID zero for a run without session.
	SpaceID, AgentSessionID ids.ID
	// BillingMode is runs.billing_mode_used.
	BillingMode string
	// OccurredAt is the end of the run; its prices apply.
	OccurredAt time.Time
	// Reports are the reports of the finish in order.
	Reports []CostReport
}

// Entry is one written cost entry.
type Entry struct {
	// ID is the new entry.
	ID ids.ID
	// Provider and Model are copied from the report.
	Provider, Model string
	// Source is SourceComputed, SourceAdapterReported or SourceUnpriced.
	Source string
	// AmountMicros and Currency are the booked amount; 0 for SourceUnpriced.
	AmountMicros int64
	Currency     string
	// IsEstimate is true for every booked entry.
	IsEstimate bool
}

// CurrencySum is the sum of the entries of one currency.
type CurrencySum struct {
	Currency     string
	AmountMicros int64
}

// Booking is the result of Costs.BookRun: the entries in the order of the reports and the sums
// in the order in which a currency first appears in Entries.
type Booking struct {
	Entries []Entry
	Sums    []CurrencySum
}

// Money is an amount with its currency and estimate flag (schema Money).
type Money struct {
	AmountMicros int64
	Currency     string
	IsEstimate   bool
}

// CostTotal is a valid sum of one billing mode and currency as getThreadCosts shows it.
type CostTotal struct {
	BillingMode string
	Money
}

// RunCost is the cost of one run of a thread (schema RunCost).
type RunCost struct {
	// RunID is the run; AgentSessionID its session, zero for none.
	RunID, AgentSessionID ids.ID
	// BillingMode is the mode of the run's entries.
	BillingMode string
	// Tokens are the summed token counts, always present.
	Tokens TokenCounts
	// Costs are one Money per currency; nil when the display rule hides amounts.
	Costs []Money
	// Unpriced reports a valid entry with source unpriced ("Price missing").
	Unpriced bool
}

// ThreadCosts is the result of Costs.ThreadCosts (schema CostBreakdown).
type ThreadCosts struct {
	ThreadID ids.ID
	// Totals are the sums per billing mode and currency with the display rule applied.
	Totals []CostTotal
	// Runs are the runs with at least one valid entry, by run id ascending.
	Runs []RunCost
}

// Level is the grouping of a TotalsQuery.
type Level string

// The levels of Totals (design note 7).
const (
	LevelRun      Level = "run"
	LevelThread   Level = "thread"
	LevelSession  Level = "session"
	LevelAgent    Level = "agent"
	LevelSpace    Level = "space"
	LevelInstance Level = "instance"
)

// ChannelFilter restricts Totals to the visible channels of the caller: All for every channel,
// otherwise the listed IDs. A filter that is neither All nor has IDs returns nothing (fail closed,
// 03 principle 12).
type ChannelFilter struct {
	All bool
	IDs []ids.ID
}

// TotalsQuery is the input of Costs.Totals.
type TotalsQuery struct {
	// Level and ID name the level; ID is ignored for LevelInstance.
	Level Level
	ID    ids.ID
	// From and To are a half-open range on occurred_at; zero means open.
	From, To time.Time
	// Channels restricts the channels.
	Channels ChannelFilter
	// ForDisplay hides personal amounts unless billing.show_personal_estimates is on.
	ForDisplay bool
}

// Total is one valid sum of Totals.
type Total struct {
	BillingMode, Currency string
	// AmountMicros is nil when the display rule hides the amount.
	AmountMicros *int64
	// IsEstimate is true when any summed entry is an estimate.
	IsEstimate bool
	// Tokens are the summed token counts.
	Tokens TokenCounts
	// Entries is the number of summed entries.
	Entries int64
}

// Price is one row of the price table (schema Price).
type Price struct {
	ID              ids.ID
	Provider, Model string
	// Unit is a PriceUnit value.
	Unit        string
	PriceMicros int64
	Currency    string
	// ValidFrom is the start; ValidTo the end, zero while open.
	ValidFrom, ValidTo time.Time
	// SourceURL is the public pricing page; RetrievedAt the time it was read.
	SourceURL   string
	RetrievedAt time.Time
}

// CreatePrice is the input of Prices.Create (schema PriceCreate).
type CreatePrice struct {
	Provider, Model, Unit string
	PriceMicros           int64
	Currency              string
	// ValidTo is zero for an open row.
	ValidFrom, ValidTo time.Time
	SourceURL          string
	RetrievedAt        time.Time
}

// ClosePrice is the input of Prices.Close (schema PriceClose).
type ClosePrice struct {
	PriceID ids.ID
	ValidTo time.Time
}
