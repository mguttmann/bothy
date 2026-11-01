// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/OWNER/bothy/services/server/internal/chat/domain/fracindex"
	"github.com/OWNER/bothy/services/server/internal/kernel/designkeys"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/textcheck"
)

// The field error codes of this package besides the textcheck rules.
const (
	// FieldRequired marks a missing or blank value.
	FieldRequired = "required"
	// FieldTooLong marks a value above its limit.
	FieldTooLong = "too_long"
	// FieldInvalid marks a value outside its list or grammar.
	FieldInvalid = "invalid"
	// FieldNotAllowed marks a value the rule forbids in this combination.
	FieldNotAllowed = "not_allowed"
)

// Space is one row of spaces (03 section 4.5). Every change bumps Version once.
type Space struct {
	ID ids.ID
	// Name is trimmed; Icon and Color are "" for none.
	Name, Icon, Color string
	// SortKey is the fractional index key of the global order.
	SortKey string
	// DefaultOperatorAgentID is zero for none (set by M1-54).
	DefaultOperatorAgentID ids.ID
	// CreatedBy is the creator; CreatedAt the creation time.
	CreatedBy ids.ID
	CreatedAt time.Time
	// ArchivedAt is zero while the space is not archived.
	ArchivedAt time.Time
	// Version is the ETag, 1 for a new space.
	Version int64
	// Deletion is the deletion state (03 R-DM-57); a deleted space is archived too.
	Deletion Deletion
}

// NewSpace returns a new space with version 1; the arguments are checked already.
func NewSpace(id ids.ID, name, icon, color, sortKey string, by ids.ID) Space {
	return Space{ID: id, Name: name, Icon: icon, Color: color, SortKey: sortKey, CreatedBy: by, Version: 1}
}

// Archived reports an archived space.
func (s Space) Archived() bool { return !s.ArchivedAt.IsZero() }

// Update sets the fields that are given and differ, bumps the version once when one did and
// returns the names of the changed fields in the order name, icon, color.
func (s *Space) Update(name, icon, color *string) []string {
	var changed []string
	set := func(field string, dst *string, v *string) {
		if v != nil && *v != *dst {
			*dst = *v
			changed = append(changed, field)
		}
	}
	set("name", &s.Name, name)
	set("icon", &s.Icon, icon)
	set("color", &s.Color, color)
	if len(changed) > 0 {
		s.Version++
	}
	return changed
}

// Move sets the sort key and bumps the version.
func (s *Space) Move(key string) {
	s.SortKey = key
	s.Version++
}

// Archive sets the archive time and bumps the version.
func (s *Space) Archive(at time.Time) {
	s.ArchivedAt = at
	s.Version++
}

// SpaceName trims name and checks it: not blank, at most maxChars Unicode characters
// (space.name_max_chars), no control or bidi characters (SEC-IN-4). Errors: validation_failed on
// /name; the error never repeats the input.
func SpaceName(name string, maxChars int) (string, error) {
	if err := displayText("/name", "name", name); err != nil {
		return "", err
	}
	name = strings.TrimSpace(name)
	switch {
	case name == "":
		return "", fieldError("/name", FieldRequired, "the name is required")
	case utf8.RuneCountInString(name) > maxChars:
		return "", fieldError("/name", FieldTooLong, "the name is too long")
	}
	return name, nil
}

// CheckColor accepts only a category colour key of the design system (designkeys); a hex value,
// an emoji or an unknown key is validation_failed on /color.
func CheckColor(c string) error {
	if !designkeys.IsCategoryColor(c) {
		return fieldError("/color", FieldInvalid, "the color is not a category color of the design system")
	}
	return nil
}

// CheckIcon accepts only a key of the list space-icons of the design system; anything else is
// validation_failed on /icon.
func CheckIcon(i string) error {
	if !designkeys.IsSpaceIcon(i) {
		return fieldError("/icon", FieldInvalid, "the icon is not a space icon of the design system")
	}
	return nil
}

// CheckReason validates the reason of space.archive: not blank and free of control and bidi
// characters (SEC-IN-4, SEC-AUD-5). Errors: validation_failed on /reason.
func CheckReason(r string) error {
	if err := displayText("/reason", "reason", r); err != nil {
		return err
	}
	if strings.TrimSpace(r) == "" {
		return fieldError("/reason", FieldRequired, "the reason is required")
	}
	return nil
}

// SidebarKey returns the personal sort key after a sidebar PUT (04 section 14): key replaces the
// stored one and must be a fractional index key, global removes it, neither keeps stored. Errors:
// validation_failed on /sort_key for an invalid key and on /use_global_order when both are sent.
func SidebarKey(stored string, key *string, global bool) (string, error) {
	switch {
	case key != nil && global:
		return "", fieldError("/use_global_order", FieldNotAllowed, "use_global_order and sort_key exclude each other")
	case global:
		return "", nil
	case key == nil:
		return stored, nil
	case !fracindex.Valid(*key):
		return "", fieldError("/sort_key", FieldInvalid, "the sort key is not a fractional index key")
	}
	return *key, nil
}

// displayText checks s under SEC-IN-4. The error never repeats the input.
func displayText(pointer, what, s string) error {
	err := textcheck.DisplayText(s)
	if err == nil {
		return nil
	}
	var te *textcheck.Error
	if errors.As(err, &te) {
		return fieldError(pointer, string(te.Rule), "the "+what+" contains a forbidden character")
	}
	return fieldError(pointer, FieldInvalid, "the "+what+" is not valid")
}

// fieldError returns validation_failed with one field error.
func fieldError(pointer, code, detail string) *errs.Error {
	return errs.New(errs.CodeValidationFailed, detail).WithFields(errs.FieldError{Pointer: pointer, Code: code, Detail: detail})
}

// SidebarState is one row of user_space_states (03 section 4.5): the personal collapsed state and
// order of one space, last write wins (04 section 14).
type SidebarState struct {
	UserID, SpaceID ids.ID
	Collapsed       bool
	// SortKey is the personal key; "" uses the global order.
	SortKey string
	// ChannelOrder is the personal order of the channels and forums of the space (M4-51); nil
	// (NULL) is the default order.
	ChannelOrder []ids.ID
	// Version counts the writes of the row.
	Version int64
	// UpdatedByDeviceID is the device of the last write; zero for none (personal tokens).
	UpdatedByDeviceID ids.ID
}

// SidebarChannelState is one row of user_channel_states (03 section 4.5, M4-51): the personal
// order of the posts of one channel, last write wins (04 section 14).
type SidebarChannelState struct {
	UserID, ChannelID ids.ID
	// ThreadOrder is the personal order of the open posts; empty is the default order.
	ThreadOrder []ids.ID
	// Version counts the writes of the row.
	Version int64
	// UpdatedByDeviceID is the device of the last write; zero for none (personal tokens).
	UpdatedByDeviceID ids.ID
}

// CheckSidebarOrder checks a personal order list of the sidebar (M4-51, D-M4-51-8): every ID at
// most once and at most maxItems items (sidebar.order_max_items). It never checks that an ID
// names an existing or visible row, so the answer reveals nothing about rows the caller cannot
// see. Errors: validation_failed on pointer with duplicate or limit_reached.
func CheckSidebarOrder(pointer string, order []ids.ID, maxItems int) error {
	if len(order) > maxItems {
		return fieldError(pointer, FieldLimitReached, "the list has too many items")
	}
	seen := make(map[ids.ID]bool, len(order))
	for _, id := range order {
		if seen[id] {
			return fieldError(pointer, FieldDuplicate, "an item is named twice")
		}
		seen[id] = true
	}
	return nil
}
