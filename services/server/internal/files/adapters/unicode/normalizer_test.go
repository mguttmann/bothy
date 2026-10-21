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
	decomposed := "Report Cafe\u0301.pdf"
	got := domain.CleanName(unicode.Normalizer{}.NFC(decomposed), 255)
	if got != "Report Caf\u00e9.pdf" {
		t.Fatalf("got %q", got)
	}
}
