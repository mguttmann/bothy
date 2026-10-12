// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"testing"

	"github.com/OWNER/bothy/services/server/internal/notify/domain"
)

// TestReleaseDelayed holds 13 4.4 "Desktop aktiv": a delayed push the person has read is dropped.
func TestReleaseDelayed(t *testing.T) {
	t.Parallel()
	cases := []struct {
		hint, read int64
		release    bool
	}{
		{5, 4, true},
		{5, 5, false},
		{5, 6, false},
		{5, 0, true},
		{0, 0, true},
		{0, 9, true},
		{-1, 9, true},
	}
	for _, c := range cases {
		if got := domain.ReleaseDelayed(c.hint, c.read); got != c.release {
			t.Errorf("ReleaseDelayed(%d, %d) = %v, want %v", c.hint, c.read, got, c.release)
		}
	}
}
