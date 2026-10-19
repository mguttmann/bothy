// SPDX-License-Identifier: Apache-2.0

package mimetype_test

import (
	"slices"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/files/adapters/mimetype"
)

// TestExtensions: known types give their extension with the dot, case does not matter, an alias
// finds its type, and an unknown type gives nil.
func TestExtensions(t *testing.T) {
	for _, c := range []struct {
		in   string
		want []string
	}{
		{"image/png", []string{".png"}},
		{"IMAGE/PNG", []string{".png"}},
		{"application/x-pdf", []string{".pdf"}},
		{"application/vnd.microsoft.portable-executable", []string{".exe"}},
		{"application/x-unknown-bothy", nil},
	} {
		if got := (mimetype.Extensions{}).Extensions(c.in); !slices.Equal(got, c.want) {
			t.Errorf("Extensions(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}
