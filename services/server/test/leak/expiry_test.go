// SPDX-License-Identifier: Apache-2.0

package leak

import (
	"testing"
	"time"
)

// TestExpiriesFollowTheClock holds that every expiry ends expiryLead after the moment it is
// taken, even when the build of a world is slower than expiryLead between two grants (the
// server refuses an expiry before the creation time), and that last is the latest of them.
func TestExpiriesFollowTheClock(t *testing.T) {
	now := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	e := &expiries{now: func() time.Time { return now }}
	first := e.next()
	now = now.Add(3 * expiryLead)
	second := e.next()
	if !first.Equal(now.Add(-3 * expiryLead).Add(expiryLead)) {
		t.Fatalf("first = %s, want expiryLead after its moment", first)
	}
	if !second.After(now) || !second.Equal(now.Add(expiryLead)) {
		t.Fatalf("second = %s, want expiryLead after %s", second, now)
	}
	if !e.last.Equal(second) {
		t.Fatalf("last = %s, want %s", e.last, second)
	}
}
