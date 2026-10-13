// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"os/exec"
	"slices"
	"strings"
	"testing"
)

// TestFilestestNotInBinary keeps the conformance suite of the files module out of the binary
// (02 "6. Code-Layout"; pattern of M1-33).
func TestFilestestNotInBinary(t *testing.T) {
	t.Parallel()
	deps := goListDeps(t, exec.CommandContext(t.Context(), "go", "list", "-deps", "../../cmd/bothy"))
	if !slices.Contains(deps, "github.com/OWNER/bothy/services/server/internal/wire") {
		t.Fatalf("go list -deps does not list internal/wire; the check would prove nothing")
	}
	for _, d := range deps {
		if strings.HasSuffix(d, "/filestest") {
			t.Errorf("cmd/bothy depends on %s", d)
		}
	}
}
