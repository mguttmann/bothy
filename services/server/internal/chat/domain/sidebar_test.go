// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"testing"

	"github.com/OWNER/bothy/services/server/internal/chat/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// TestCheckSidebarOrder (M4-51, D-M4-51-8): a list of unique IDs up to the limit passes, the
// empty list too; a repeated ID is duplicate and a list above the limit limit_reached, both on
// the member's pointer.
func TestCheckSidebarOrder(t *testing.T) {
	t.Parallel()
	a, b, c := ids.New(), ids.New(), ids.New()
	for _, ok := range [][]ids.ID{nil, {}, {a}, {a, b, c}} {
		if err := domain.CheckSidebarOrder("/thread_order", ok, 3); err != nil {
			t.Errorf("%v: %v", ok, err)
		}
	}
	wantCode(t, domain.CheckSidebarOrder("/channel_order", []ids.ID{a, b, a}, 3), "/channel_order", domain.FieldDuplicate)
	wantCode(t, domain.CheckSidebarOrder("/thread_order", []ids.ID{a, b, c}, 2), "/thread_order", domain.FieldLimitReached)
}
