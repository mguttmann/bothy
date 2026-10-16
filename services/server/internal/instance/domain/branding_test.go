// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"slices"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/instance/domain"
)

func TestBrandingVersionSum(t *testing.T) {
	t.Parallel()
	if got := domain.BrandingVersion(0, 0, 0); got != 0 {
		t.Errorf("all defaults: %d, want 0", got)
	}
	if got := domain.BrandingVersion(3, 1, 0); got != 4 {
		t.Errorf("3+1+0 = %d", got)
	}
	// Every write raises one version by one, so the sum rises strictly.
	prev := domain.BrandingVersion(2, 5, 1)
	for _, next := range []int64{domain.BrandingVersion(3, 5, 1), domain.BrandingVersion(2, 6, 1), domain.BrandingVersion(2, 5, 2)} {
		if next != prev+1 {
			t.Errorf("one write: %d, want %d", next, prev+1)
		}
	}
}

func TestBrandingChanges(t *testing.T) {
	t.Parallel()
	cur := domain.BrandingValues{ProductName: "Base Chat", AccentColor: "spruce"}
	if got := domain.BrandingChanges(cur, cur); len(got) != 0 {
		t.Errorf("no change: %v", got)
	}
	next := domain.BrandingValues{ProductName: "Team Chat", AccentColor: "spruce"}
	if got := domain.BrandingChanges(cur, next); !slices.Equal(got, []string{domain.KeyProductName}) {
		t.Errorf("name only: %v", got)
	}
	next = domain.BrandingValues{ProductName: "Team Chat", AccentColor: "loch"}
	if got := domain.BrandingChanges(cur, next); !slices.Equal(got, []string{domain.KeyAccentColor, domain.KeyProductName}) {
		t.Errorf("both: %v, want sorted keys", got)
	}
}
