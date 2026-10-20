// SPDX-License-Identifier: Apache-2.0

package rest

import (
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// OptionalID converts an optional id for a response: the zero ID is an absent member.
func OptionalID(id ids.ID) *Uuid {
	if id.IsZero() {
		return nil
	}
	u := Uuid(id)
	return &u
}

// OptionalTime converts an optional time for a response in UTC with millisecond precision
// (06 section 2): the zero time is an absent member.
func OptionalTime(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	v := t.UTC().Truncate(time.Millisecond)
	return &v
}

// IDs converts a list of ids for a response; the result is never nil, so an empty list is [].
func IDs(list []ids.ID) []Uuid {
	out := make([]Uuid, 0, len(list))
	for _, id := range list {
		out = append(out, Uuid(id))
	}
	return out
}
