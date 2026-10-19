// SPDX-License-Identifier: Apache-2.0

package unicode_test

import (
	"testing"

	"github.com/OWNER/bothy/services/server/internal/files/adapters/unicode"
	"github.com/OWNER/bothy/services/server/internal/files/domain"
)

// TestDecomposedNameStaysComposed: a decomposed name is composed by the adapter and stays so
// through CleanName (brief M3-03 TestCleanName).
func TestDecomposedNameStaysComposed(t *testing.T) {
	decomposed := "Bericht A\u0308rger.pdf"
	got := domain.CleanName(unicode.Normalizer{}.NFC(decomposed), 255)
	if got != "Bericht Ärger.pdf" {
		t.Fatalf("got %q", got)
	}
}
