// SPDX-License-Identifier: Apache-2.0

package locale_test

import (
	"testing"

	"github.com/OWNER/bothy/services/server/internal/identity/adapters/locale"
)

func TestCanonical(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]string{"de": "de", "en-us": "en-US", "EN-GB": "en-GB", "pt-BR": "pt-BR", "zh-hant-tw": "zh-Hant-TW"} {
		if got, ok := (locale.Parser{}).Canonical(in); !ok || got != want {
			t.Errorf("Canonical(%q) = %q, %v; want %q", in, got, ok, want)
		}
	}
	for _, in := range []string{"", "x", "de_DE_", "123", "de-", "not a tag", "de\u202e"} {
		if got, ok := (locale.Parser{}).Canonical(in); ok {
			t.Errorf("Canonical(%q) = %q, want refused", in, got)
		}
	}
}
