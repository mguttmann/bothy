// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"errors"
	"math/big"
	"net/url"
	"strings"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/textcheck"
)

// Unit is price_table.unit (03 section 4.9).
type Unit string

// The units of the price table. Prices of the *_mtok units are per million tokens.
const (
	UnitInput        Unit = "input_mtok"
	UnitOutput       Unit = "output_mtok"
	UnitCacheRead    Unit = "cache_read_mtok"
	UnitCacheWrite   Unit = "cache_write_mtok"
	UnitCacheWrite1h Unit = "cache_write_1h_mtok"
	// UnitRequest is never used for a computation: neither CostReport nor cost_entries carry a
	// request count (spec gap, ruling D-M2-22-3).
	UnitRequest Unit = "request"
)

// Units returns every unit in the order of the CHECK of 03.
func Units() []Unit {
	return []Unit{UnitInput, UnitOutput, UnitCacheRead, UnitCacheWrite, UnitCacheWrite1h, UnitRequest}
}

// TokenUnits returns the five token units in the order of the counts of Tokens.
func TokenUnits() []Unit {
	return []Unit{UnitInput, UnitOutput, UnitCacheRead, UnitCacheWrite, UnitCacheWrite1h}
}

// count returns the count of t that unit u prices; 0 for UnitRequest.
func (u Unit) count(t Tokens) int64 {
	switch u {
	case UnitInput:
		return t.In
	case UnitOutput:
		return t.Out
	case UnitCacheRead:
		return t.CacheRead
	case UnitCacheWrite:
		return t.CacheWrite
	case UnitCacheWrite1h:
		return t.CacheWrite1h
	case UnitRequest:
		return 0
	}
	return 0
}

// The values of cost_entries.source.
const (
	SourceComputed        = "computed"
	SourceAdapterReported = "adapter_reported"
	SourceUnpriced        = "unpriced"
	SourceReconciled      = "reconciled"
)

// ErrAmountTooLarge reports a computed amount above MaxAmount; Book maps it to too_large.
var ErrAmountTooLarge = errors.New("billing: the computed amount reaches 2^53")

// PriceRow is one row of price_table. ValidTo is zero while the row is open.
type PriceRow struct {
	ID              ids.ID
	Provider, Model string
	Unit            Unit
	PriceMicros     int64
	Currency        string
	ValidFrom       time.Time
	ValidTo         time.Time
	SourceURL       string
	RetrievedAt     time.Time
}

// Covers reports whether t lies in [ValidFrom, ValidTo); a zero ValidTo is open.
func (p PriceRow) Covers(t time.Time) bool {
	return !t.Before(p.ValidFrom) && (p.ValidTo.IsZero() || t.Before(p.ValidTo))
}

// PriceSet returns the price set S(provider, model, t) of rows: every row with exactly equal
// provider and model (case-sensitive) that covers t (M2-22 design note 4 a). The order of rows is
// kept.
func PriceSet(rows []PriceRow, provider, model string, t time.Time) []PriceRow {
	var out []PriceRow
	for _, r := range rows {
		if r.Provider == provider && r.Model == model && r.Covers(t) {
			out = append(out, r)
		}
	}
	return out
}

// Priced is the result of Compute: the source, amount and currency of one entry.
type Priced struct {
	Source       string
	AmountMicros int64
	Currency     string
}

// Compute prices one report with its price set (10 section 12.1; M2-22 design note 4 b and c).
// The set is complete when it is not empty, all its token rows have one currency and every unit
// whose count is above 0 has its row; rows of the unit request are left out. A complete set gives
// SourceComputed with the exact amount in the set's currency, and the adapter's amount is ignored.
// Otherwise a report with an amount gives SourceAdapterReported with the report's amount and
// currency, else SourceUnpriced with amount 0 and the report's currency.
//
// The amount is the sum over the needed units of count times price_micros, divided once by one
// million and rounded half up once per entry, computed with math/big. Errors: ErrAmountTooLarge
// when the result is above MaxAmount.
//
// Example:
//
//	p, err := domain.Compute(r, domain.PriceSet(rows, r.Provider, r.Model, occurredAt))
func Compute(r Report, set []PriceRow) (Priced, error) {
	byUnit, currency, complete := tokenRows(set)
	var sum big.Int
	for _, u := range TokenUnits() {
		n := u.count(r.Tokens)
		if !complete || n <= 0 {
			continue
		}
		p, ok := byUnit[u]
		if !ok {
			complete = false
			break
		}
		sum.Add(&sum, new(big.Int).Mul(big.NewInt(n), big.NewInt(p.PriceMicros)))
	}
	if !complete {
		if r.AmountMicros != nil {
			return Priced{Source: SourceAdapterReported, AmountMicros: *r.AmountMicros, Currency: r.Currency}, nil
		}
		return Priced{Source: SourceUnpriced, Currency: r.Currency}, nil
	}
	amount, err := roundMillionths(&sum)
	if err != nil {
		return Priced{}, err
	}
	return Priced{Source: SourceComputed, AmountMicros: amount, Currency: currency}, nil
}

// tokenRows indexes the token rows of a price set by unit (rows of the unit request are left
// out) and returns their one currency; ok is false for an empty set, two rows of one unit or two
// currencies, which make the set incomplete.
func tokenRows(set []PriceRow) (byUnit map[Unit]PriceRow, currency string, ok bool) {
	byUnit = make(map[Unit]PriceRow, len(set))
	for _, p := range set {
		if p.Unit == UnitRequest {
			continue
		}
		if _, dup := byUnit[p.Unit]; dup || (currency != "" && p.Currency != currency) {
			return nil, "", false
		}
		byUnit[p.Unit] = p
		currency = p.Currency
	}
	return byUnit, currency, len(byUnit) > 0
}

// million is the divisor of the *_mtok prices.
var million = big.NewInt(1_000_000)

// roundMillionths divides a non-negative sum by one million, rounding half up once.
func roundMillionths(sum *big.Int) (int64, error) {
	q, rem := new(big.Int).QuoRem(sum, million, new(big.Int))
	if rem.Lsh(rem, 1).Cmp(million) >= 0 {
		q.Add(q, big.NewInt(1))
	}
	if !q.IsInt64() || q.Int64() > MaxAmount {
		return 0, ErrAmountTooLarge
	}
	return q.Int64(), nil
}

// PriceInput is a price row to create (schema PriceCreate, and a row of the start file).
type PriceInput struct {
	Provider, Model, Unit string
	PriceMicros           int64
	Currency              string
	// ValidTo is zero for an open row.
	ValidFrom, ValidTo time.Time
	SourceURL          string
	RetrievedAt        time.Time
}

// ValidatePrice checks a price row (M2-22 design note 11 b): provider and model non-empty after
// trimming and passing textcheck.DisplayText; unit one of Units (invalid_enum); price_micros not
// negative and not above MaxAmount; currency of the form ^[A-Z]{3}$; valid_to, when set, after
// valid_from (invalid); source_url an absolute https URL with a host, without user info and
// fragment (invalid_format); retrieved_at not after now (invalid), because a retrieval in the
// future is not a fact. All violations are collected into one validation_failed.
//
// Example:
//
//	err := domain.ValidatePrice(in, clk.Now())
func ValidatePrice(p PriceInput, now time.Time) error {
	var fs []errs.FieldError
	fs = append(fs, checkName("/provider", p.Provider)...)
	fs = append(fs, checkName("/model", p.Model)...)
	if !validUnit(p.Unit) {
		fs = append(fs, field("/unit", codeInvalidEnum, "the unit is not a unit of the price table"))
	}
	fs = append(fs, checkCount("/price_micros", p.PriceMicros)...)
	if !ValidCurrency(p.Currency) {
		fs = append(fs, field("/currency", codeInvalidFormat, "the currency is not an ISO 4217 code"))
	}
	if !p.ValidTo.IsZero() && !p.ValidTo.After(p.ValidFrom) {
		fs = append(fs, field("/valid_to", codeInvalid, "valid_to is not after valid_from"))
	}
	if !ValidSourceURL(p.SourceURL) {
		fs = append(fs, field("/source_url", codeInvalidFormat, "the source is not an absolute https URL without user info and fragment"))
	}
	if p.RetrievedAt.After(now) {
		fs = append(fs, field("/retrieved_at", codeInvalid, "retrieved_at lies in the future"))
	}
	return failed(fs)
}

// validUnit reports whether u is a unit of the price table.
func validUnit(u string) bool {
	for _, x := range Units() {
		if string(x) == u {
			return true
		}
	}
	return false
}

// ValidSourceURL reports whether s is an absolute https URL with a host and without user info
// and fragment (03 price_table.source_url: the public pricing page), free of control, bidi and
// space characters. It is the form internal/audit accepts under source_url (19 SEC-AUD-5), so a
// row that passes here can always be audited.
func ValidSourceURL(s string) bool {
	if textcheck.DisplayText(s) != nil || strings.ContainsAny(s, " #") {
		return false
	}
	u, err := url.Parse(s)
	return err == nil && u.Scheme == "https" && u.Opaque == "" && u.Host != "" && u.User == nil
}
