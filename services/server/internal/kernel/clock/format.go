// SPDX-License-Identifier: Apache-2.0

package clock

import "time"

// APILayout is the form of every API time (06 section 2): UTC, milliseconds, suffix Z.
const APILayout = "2006-01-02T15:04:05.000Z"

// FormatAPI formats t in APILayout. The zero time gives the empty string, so an optional member
// with omitempty stays absent.
func FormatAPI(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Truncate(time.Millisecond).Format(APILayout)
}
