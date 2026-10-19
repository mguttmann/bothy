// SPDX-License-Identifier: Apache-2.0

package clock_test

import (
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
)

func TestFormatAPI(t *testing.T) {
	berlin := time.FixedZone("UTC+2", 2*60*60)
	cases := []struct {
		in   time.Time
		want string
	}{
		{time.Time{}, ""},
		{time.Date(2026, 10, 20, 9, 30, 1, 123456789, time.UTC), "2026-10-20T09:30:01.123Z"},
		{time.Date(2026, 10, 20, 11, 30, 1, 0, berlin), "2026-10-20T09:30:01.000Z"},
	}
	for _, c := range cases {
		if got := clock.FormatAPI(c.in); got != c.want {
			t.Errorf("FormatAPI(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}
