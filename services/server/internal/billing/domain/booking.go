// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"errors"
	"strconv"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
)

// CurrencySum is the sum of the entries of one currency.
type CurrencySum struct {
	Currency     string
	AmountMicros int64
}

// Book prices the reports of one finished run, each with its price set (sets[i] belongs to
// reports[i]), and returns the entries in the order of the reports and the sums per currency in
// the order in which a currency first appears (M2-22 design note 5 e and f). Amounts of different
// currencies are never converted.
//
// Errors: validation_failed with too_large under /<i>/amount_micros when an entry or the sum of
// a currency is above MaxAmount; an internal error when sets and reports differ in length.
//
// Example:
//
//	priced, sums, err := domain.Book(reports, sets)
func Book(reports []Report, sets [][]PriceRow) ([]Priced, []CurrencySum, error) {
	if len(sets) != len(reports) {
		return nil, nil, errs.New(errs.CodeInternalError, "billing: one price set per report")
	}
	out := make([]Priced, 0, len(reports))
	var sums []CurrencySum
	for i, r := range reports {
		p, err := Compute(r, sets[i])
		if errors.Is(err, ErrAmountTooLarge) {
			return nil, nil, tooLarge(i)
		}
		if err != nil {
			return nil, nil, err
		}
		out = append(out, p)
		j := indexOf(sums, p.Currency)
		if j < 0 {
			sums = append(sums, CurrencySum{Currency: p.Currency})
			j = len(sums) - 1
		}
		if sums[j].AmountMicros > MaxAmount-p.AmountMicros {
			return nil, nil, tooLarge(i)
		}
		sums[j].AmountMicros += p.AmountMicros
	}
	return out, sums, nil
}

// tooLarge is the validation error of report i whose amount reaches the bound.
func tooLarge(i int) error {
	return failed([]errs.FieldError{field("/"+strconv.Itoa(i)+"/amount_micros", codeTooLarge, "the amount reaches 2^53")})
}

// indexOf returns the index of currency c in sums, -1 for none.
func indexOf(sums []CurrencySum, c string) int {
	for i, s := range sums {
		if s.Currency == c {
			return i
		}
	}
	return -1
}
