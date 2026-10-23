// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"errors"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
)

// CloseOutcome is the result of CloseDecision.
type CloseOutcome uint8

// The outcomes of CloseDecision. The zero value is invalid.
const (
	// CloseSet: set valid_to (after the price_in_use check of the caller).
	CloseSet CloseOutcome = iota + 1
	// CloseNoChange: the row already ends at the requested time (06 section 2 "No effect").
	CloseNoChange
)

// ErrPriceClosed reports a close of a closed row with another end; the caller answers 409
// conflict with reason price_closed. A closed row is never reopened or moved; a gap is filled by
// a new row (ruling D-M2-22-18).
var ErrPriceClosed = errors.New("billing: the price row is already closed")

// CloseDecision decides closePrice for row and the new end validTo (M2-22 design note 12 a and
// b): validTo not after valid_from is validation_failed (invalid, pointer /valid_to); a closed row
// with the same end is CloseNoChange; a closed row with another end is ErrPriceClosed; an open row
// is CloseSet.
//
// Example:
//
//	out, err := domain.CloseDecision(row, req.ValidTo)
func CloseDecision(row PriceRow, validTo time.Time) (CloseOutcome, error) {
	if !validTo.After(row.ValidFrom) {
		return 0, failed([]errs.FieldError{field("/valid_to", codeInvalid, "valid_to is not after valid_from")})
	}
	if row.ValidTo.IsZero() {
		return CloseSet, nil
	}
	if row.ValidTo.Equal(validTo) {
		return CloseNoChange, nil
	}
	return 0, ErrPriceClosed
}
