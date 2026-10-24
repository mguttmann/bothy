// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"bytes"
	"math/big"
	"strconv"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// The values of cost_entries.billing_mode and .source the reconciliation reads (03 section 4.9).
const (
	modeOrganization = "organization"
	sourceReconciled = "reconciled"
)

// RequestEntry is one entry of a reconciliation (schema ReconcileEntry): the actual amount of one
// run in the period.
type RequestEntry struct {
	RunID        ids.ID
	AmountMicros int64
	Currency     string
}

// Request is a reconciliation of reconcileCosts or submitCostReconciliation (schema CostReconcile):
// the provider of the entries, the half-open period [PeriodStart, PeriodEnd) on occurred_at and the
// entries in request order.
type Request struct {
	Provider               string
	PeriodStart, PeriodEnd time.Time
	Entries                []RequestEntry
}

// Entry is one row of cost_entries with every column the reconciliation copies (03 section 4.9).
// Zero IDs are NULL.
type Entry struct {
	ID, RunID, AgentID, ThreadID, ChannelID ids.ID
	SpaceID, AgentSessionID                 ids.ID
	BillingMode, Provider, Model            string
	Tokens                                  Tokens
	AmountMicros                            int64
	Currency                                string
	IsEstimate                              bool
	Source                                  string
	OccurredAt                              time.Time
	ReconciledAt                            time.Time
	SupersedesID                            ids.ID
}

// Violation is one rule a request breaks: the JSON pointer into the body and the errors[].code
// (the list of M1-18).
type Violation struct {
	Pointer, Code string
}

// UnmatchedReason is why a request entry was not matched (M2-28 design note 4). It is never sent to
// a client: the answer only lists the run IDs.
type UnmatchedReason string

// The reasons of an unmatched request entry.
const (
	// UnmatchedNoEntries: the run has no valid entry in the period (also an unknown run).
	UnmatchedNoEntries UnmatchedReason = "no_entries"
	// UnmatchedPersonal: a valid entry in the period has billing mode personal; its amounts are
	// information, never invoice (03 cost_entries).
	UnmatchedPersonal UnmatchedReason = "personal"
	// UnmatchedProvider: a valid entry in the period has another provider than the request.
	UnmatchedProvider UnmatchedReason = "provider"
	// UnmatchedCurrency: a valid entry in the period has another currency than the request entry
	// (also mixed currencies); amounts are never converted (10 section 12.1).
	UnmatchedCurrency UnmatchedReason = "currency"
)

// Share is the amount one valid entry gets of the amount of its run (Split).
type Share struct {
	EntryID      ids.ID
	AmountMicros int64
}

// Validate checks the rules of a reconciliation that the schema cannot express (M2-28 design note
// 3 b): period_start before period_end (invalid at /period_end); a run_id that occurs again
// (invalid at /entries/<i>/run_id of the later occurrence); an amount above MaxAmount (too_large
// at /entries/<i>/amount_micros), the integer bound of 06. The schema checks the rest (negative
// amounts, the form of the currency, an empty provider). It returns every violation in body order,
// nil for a valid request.
//
// Example:
//
//	if v := domain.Validate(req); len(v) > 0 {
//		// 400 validation_failed with one field error per violation
//	}
func Validate(req Request) []Violation {
	var out []Violation
	if !req.PeriodStart.Before(req.PeriodEnd) {
		out = append(out, Violation{Pointer: "/period_end", Code: codeInvalid})
	}
	seen := make(map[ids.ID]bool, len(req.Entries))
	for i, e := range req.Entries {
		at := "/entries/" + strconv.Itoa(i)
		if seen[e.RunID] {
			out = append(out, Violation{Pointer: at + "/run_id", Code: codeInvalid})
		}
		seen[e.RunID] = true
		if e.AmountMicros > MaxAmount {
			out = append(out, Violation{Pointer: at + "/amount_micros", Code: codeTooLarge})
		}
	}
	return out
}

// InPeriod returns the entries of run whose occurred_at lies in the half-open period of req, in
// their order.
func InPeriod(valid []Entry, req Request, run ids.ID) []Entry {
	var out []Entry
	for _, e := range valid {
		if e.RunID == run && !e.OccurredAt.Before(req.PeriodStart) && e.OccurredAt.Before(req.PeriodEnd) {
			out = append(out, e)
		}
	}
	return out
}

// Match decides one request entry e against the valid entries of its run (M2-28 design note 4):
// only the entries of e.RunID in the period of req count. e is matched when there is at least one
// and all of them have billing mode organization, the provider of req and the currency of e;
// otherwise reason names the first rule that failed. A matched entry is a no-op when all of them
// are reconciled and their sum equals e.AmountMicros: the app submits the same days on every pass,
// and a repeat must not grow the chain.
func Match(valid []Entry, req Request, e RequestEntry) (matched, noop bool, reason UnmatchedReason) {
	in := InPeriod(valid, req, e.RunID)
	if len(in) == 0 {
		return false, false, UnmatchedNoEntries
	}
	allReconciled := true
	var sum int64
	for _, v := range in {
		switch {
		case v.BillingMode != modeOrganization:
			return false, false, UnmatchedPersonal
		case v.Provider != req.Provider:
			return false, false, UnmatchedProvider
		case v.Currency != e.Currency:
			return false, false, UnmatchedCurrency
		}
		allReconciled = allReconciled && v.Source == sourceReconciled
		sum += v.AmountMicros
	}
	return true, allReconciled && sum == e.AmountMicros, ""
}

// Split distributes amount over the valid entries of one run in the period in proportion to
// their current amounts (M2-28 design note 5, BR-10 one level down): share = floor(amount times
// amount_j divided by the sum), the remainder to the entry with the largest amount, ties to the
// smallest id; when every amount is 0 the whole amount goes to the entry with the smallest id.
// Every entry gets exactly one share, also 0, so no estimate stays valid next to a reconciled
// amount. The arithmetic runs in math/big, so values near the int64 limit cannot overflow. The
// shares come in the order of valid; amount and the entry amounts are never negative.
//
// Example:
//
//	shares := domain.Split(10, entries) // amounts 1, 1, 2: shares 2, 2, 6
func Split(amount int64, valid []Entry) []Share {
	out := make([]Share, len(valid))
	if len(valid) == 0 {
		return out
	}
	top := 0
	total := new(big.Int)
	for i, e := range valid {
		out[i].EntryID = e.ID
		total.Add(total, big.NewInt(e.AmountMicros))
		if larger(e, valid[top]) {
			top = i
		}
	}
	if total.Sign() == 0 {
		smallest := 0
		for i, e := range valid {
			if bytes.Compare(e.ID[:], valid[smallest].ID[:]) < 0 {
				smallest = i
			}
		}
		out[smallest].AmountMicros = amount
		return out
	}
	a := big.NewInt(amount)
	share := new(big.Int)
	var given int64
	for i, e := range valid {
		share.Mul(a, big.NewInt(e.AmountMicros))
		share.Quo(share, total)
		// share <= amount, so it fits into int64.
		out[i].AmountMicros = share.Int64()
		given += out[i].AmountMicros
	}
	out[top].AmountMicros += amount - given
	return out
}

// larger reports whether a takes the remainder before b: the larger amount, on a tie the smaller
// id.
func larger(a, b Entry) bool {
	if a.AmountMicros != b.AmountMicros {
		return a.AmountMicros > b.AmountMicros
	}
	return bytes.Compare(a.ID[:], b.ID[:]) < 0
}
