// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/sync/domain"
)

// TestResyncReasonOrder holds the check order of 04 "resync_required" (brief M1-63 design note 2):
// epoch first, then cursor ahead, retention and too far behind; without an epoch no epoch check.
func TestResyncReasonOrder(t *testing.T) {
	t.Parallel()
	current, other := ids.New(), ids.New()
	const maxEvents = 100
	cases := []struct {
		name      string
		after     int64
		epoch     *ids.ID
		head, min int64
		want      domain.Reason
	}{
		{"up to date", 50, &current, 50, 1, domain.ReasonNone},
		{"behind within the limit", 10, &current, 50, 1, domain.ReasonNone},
		{"no epoch, no check", 10, nil, 50, 1, domain.ReasonNone},
		{"epoch changed", 10, &other, 50, 1, domain.ReasonEpochChanged},
		{"epoch changed and cursor ahead", 90, &other, 50, 1, domain.ReasonEpochChanged},
		{"cursor ahead", 51, &current, 50, 1, domain.ReasonCursorAhead},
		{"cursor ahead without epoch", 51, nil, 50, 1, domain.ReasonCursorAhead},
		{"after exactly min - 1", 19, &current, 50, 20, domain.ReasonNone},
		{"after below min - 1", 18, &current, 50, 20, domain.ReasonRetention},
		{"retention before too far behind", 0, &current, 500, 300, domain.ReasonRetention},
		{"too far behind", 10, &current, 10 + maxEvents + 1, 1, domain.ReasonTooFarBehind},
		{"exactly the limit", 10, &current, 10 + maxEvents, 1, domain.ReasonNone},
		{"empty log after retention, at head", 40, &current, 40, 41, domain.ReasonNone},
		{"empty log after retention, behind", 39, &current, 40, 41, domain.ReasonRetention},
		{"fresh instance", 0, nil, 0, 1, domain.ReasonNone},
	}
	for _, c := range cases {
		if got := domain.ResyncReason(c.after, c.epoch, c.head, c.min, current, maxEvents); got != c.want {
			t.Errorf("%s: %q, want %q", c.name, got, c.want)
		}
	}
}

// TestNextCursor: a full page ends at the seq of its last row, any other page at the head, also
// when authorize dropped every row (the cursor is "last checked", R-SY-03).
func TestNextCursor(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		head        int64
		limit, rows int
		last        int64
		want        int64
	}{
		{"page full", 1000, 3, 3, 17, 17},
		{"page not full", 1000, 3, 2, 17, 1000},
		{"no row", 1000, 3, 0, 0, 1000},
		{"all rows dropped by authorize", 1000, 3, 3, 900, 900},
	}
	for _, c := range cases {
		if got := domain.NextCursor(c.head, c.limit, c.rows, c.last); got != c.want {
			t.Errorf("%s: %d, want %d", c.name, got, c.want)
		}
	}
}
