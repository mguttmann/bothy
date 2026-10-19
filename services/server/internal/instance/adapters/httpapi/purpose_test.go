// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"testing"

	"github.com/OWNER/bothy/services/server/internal/instance"
)

// TestManagedPurpose pins that the purpose of a rotation request reaches the use case only as one
// of the managed constants (terminal among them since M2-39); any other text becomes the empty
// purpose, which checkRotate refuses as not_supported.
func TestManagedPurpose(t *testing.T) {
	for _, p := range instance.Purposes() {
		if got := managedPurpose(string(p)); got != p {
			t.Errorf("managedPurpose(%q) = %q, want %q", p, got, p)
		}
	}
	for _, s := range []string{"", "access_token/../x", "ACCESS_TOKEN", "TERMINAL"} {
		if got := managedPurpose(s); got != "" {
			t.Errorf("managedPurpose(%q) = %q, want the empty purpose", s, got)
		}
	}
}
