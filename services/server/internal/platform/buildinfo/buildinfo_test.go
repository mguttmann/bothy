// SPDX-License-Identifier: Apache-2.0

package buildinfo_test

import (
	"os"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/platform/buildinfo"
)

// TestVersionMatchesVERSIONFile holds 18 "Components" rule 5: the constant equals
// services/server/VERSION, so a release that bumps the file without the constant fails here.
func TestVersionMatchesVERSIONFile(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile("../../../VERSION")
	if err != nil {
		t.Fatalf("read services/server/VERSION: %v", err)
	}
	want := strings.TrimSpace(string(data))
	if want == "" {
		t.Fatal("services/server/VERSION is empty")
	}
	if got := buildinfo.Version(); got != want {
		t.Errorf("Version() = %q, services/server/VERSION says %q", got, want)
	}
}
