// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"testing"

	"github.com/OWNER/bothy/services/server/internal/gateway/domain"
)

// TestPickAutocompleteConnection: a live default wins; otherwise the newest live one; connections
// still catching up are skipped; none gives false (brief M2-17 design note 5).
func TestPickAutocompleteConnection(t *testing.T) {
	t.Parallel()
	c := func(consumer string, live bool, attached uint64) domain.AutocompleteCandidate {
		return domain.AutocompleteCandidate{Consumer: consumer, Live: live, Attached: attached}
	}
	cases := []struct {
		name   string
		cands  []domain.AutocompleteCandidate
		want   int
		wantOK bool
	}{
		{"default live wins over a newer one", []domain.AutocompleteCandidate{c("audit", true, 9), c("default", true, 2), c("x", true, 5)}, 1, true},
		{"newest live without default", []domain.AutocompleteCandidate{c("audit", true, 3), c("x", true, 7), c("y", true, 5)}, 1, true},
		{"default still catching up is skipped", []domain.AutocompleteCandidate{c("default", false, 9), c("audit", true, 1)}, 1, true},
		{"only catching up", []domain.AutocompleteCandidate{c("default", false, 1), c("audit", false, 2)}, -1, false},
		{"none", nil, -1, false},
	}
	for _, tc := range cases {
		got, ok := domain.PickAutocompleteConnection(tc.cands)
		if ok != tc.wantOK || ok && got != tc.want {
			t.Errorf("%s: %d %v, want %d %v", tc.name, got, ok, tc.want, tc.wantOK)
		}
	}
}
