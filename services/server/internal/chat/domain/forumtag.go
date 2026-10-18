// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// ForumTag is one row of forum_tags (03 section 4.5). A tag has no version of its own: every tag
// change bumps the version of its channel (Channel.Touch).
type ForumTag struct {
	ID, ChannelID ids.ID
	// Name is trimmed; Color is a category colour key or "" for none.
	Name, Color string
	// SortKey is the fractional index key of the order in the forum.
	SortKey string
	// Restricted tags are set only with manage (05 thread.tags.set).
	Restricted bool
	// DeletedAt is zero while the tag is not deleted.
	DeletedAt time.Time
}

// Deleted reports a deleted tag.
func (t ForumTag) Deleted() bool { return !t.DeletedAt.IsZero() }

// TagChange is the body of updateForumTag after the text checks: nil fields stay unchanged.
type TagChange struct {
	Name, Color *string
	// ClearColor removes the colour.
	ClearColor bool
	Restricted *bool
}

// Check validates the combination: at least one member, and not color together with clear_color.
// Errors: validation_failed on "" (empty body) or /clear_color.
func (c TagChange) Check() error {
	switch {
	case c.Name == nil && c.Color == nil && !c.ClearColor && c.Restricted == nil:
		return fieldError("", FieldRequired, "the body changes nothing")
	case c.Color != nil && c.ClearColor:
		return fieldError("/clear_color", FieldNotAllowedWithColor, "clear_color and color exclude each other")
	}
	return nil
}

// Update applies c and returns the names of the changed fields in the order name, color,
// restricted.
func (t *ForumTag) Update(c TagChange) []string {
	var changed []string
	set := func(field string, dst *string, v *string) {
		if v != nil && *v != *dst {
			*dst = *v
			changed = append(changed, field)
		}
	}
	set("name", &t.Name, c.Name)
	if c.ClearColor {
		empty := ""
		set("color", &t.Color, &empty)
	} else {
		set("color", &t.Color, c.Color)
	}
	if c.Restricted != nil && *c.Restricted != t.Restricted {
		t.Restricted = *c.Restricted
		changed = append(changed, "restricted")
	}
	return changed
}
