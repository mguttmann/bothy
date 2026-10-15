// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"slices"

	"github.com/OWNER/bothy/services/server/internal/chat/domain/fracindex"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Keyed is one entry of an order of siblings (channels of a space, tags of a forum): an id and
// its fractional index key.
type Keyed struct {
	ID      ids.ID
	SortKey string
}

// MovePosition is the body of moveChannel and moveForumTag (OpenAPI MovePosition): at most one of
// AfterID and BeforeID; none moves to the start (03 principle 5).
type MovePosition struct {
	AfterID, BeforeID ids.ID
}

// Check refuses both neighbours at once. Errors: validation_failed on /before_id.
func (p MovePosition) Check() error {
	if !p.AfterID.IsZero() && !p.BeforeID.IsZero() {
		return fieldError("/before_id", FieldNotAllowed, "after_id and before_id exclude each other")
	}
	return nil
}

// Index returns the position in order (the live siblings without the moved entry self) after
// which the entry goes: -1 for the start. Errors: validation_failed on /after_id or /before_id
// for a neighbour that is self or not in order.
func (p MovePosition) Index(order []Keyed, self ids.ID) (int, error) {
	if err := p.Check(); err != nil {
		return 0, err
	}
	find := func(id ids.ID) int { return slices.IndexFunc(order, func(k Keyed) bool { return k.ID == id }) }
	switch {
	case !p.AfterID.IsZero():
		i := find(p.AfterID)
		if p.AfterID == self || i < 0 {
			return 0, fieldError("/after_id", FieldInvalid, "after_id is the entry itself or no live sibling")
		}
		return i, nil
	case !p.BeforeID.IsZero():
		i := find(p.BeforeID)
		if p.BeforeID == self || i < 0 {
			return 0, fieldError("/before_id", FieldInvalid, "before_id is the entry itself or no live sibling")
		}
		return i - 1, nil
	}
	return -1, nil
}

// KeyAt returns a key after position i of order and before position i+1; i = -1 is the start.
// Errors: internal when the stored keys are broken.
func KeyAt(order []Keyed, i int) (string, error) {
	lo, hi := "", ""
	if i >= 0 && i < len(order) {
		lo = order[i].SortKey
	}
	if i+1 < len(order) {
		hi = order[i+1].SortKey
	}
	k, err := fracindex.KeyBetween(lo, hi)
	if err != nil {
		return "", errs.Wrap(errs.CodeInternalError, err, "chat: a stored order is broken")
	}
	return k, nil
}
