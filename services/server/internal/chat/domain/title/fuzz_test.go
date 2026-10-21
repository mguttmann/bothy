// SPDX-License-Identifier: Apache-2.0

package title_test

import (
	"testing"

	"github.com/OWNER/bothy/services/server/internal/chat/domain/title"
)

// FuzzIsPlaceholder: no input panics, the decision is the same for a title and its normal form,
// and Normalize is idempotent.
func FuzzIsPlaceholder(f *testing.F) {
	for _, s := range []string{"", " ", "Neuer Chat 3", "new chat", "Test2", "C#", "\U0001F600", "Chat ٣", "\xff"} {
		f.Add(s)
	}
	p := title.NewPlaceholders(title.DefaultEntries())
	f.Fuzz(func(t *testing.T, s string) {
		n := title.Normalize(s)
		if title.Normalize(n) != n {
			t.Fatalf("Normalize is not idempotent on %q", s)
		}
		if p.IsPlaceholder(s) != p.IsPlaceholder(n) {
			t.Fatalf("IsPlaceholder(%q) differs from its normal form %q", s, n)
		}
	})
}
