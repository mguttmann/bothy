// SPDX-License-Identifier: Apache-2.0

package authorize_test

import (
	"slices"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
)

// TestDecisionReasonsAreKnown holds the reason literals of decision.go against the closed list
// of 07 section 4.1 (auditing.KnownReasons): a renamed reason would otherwise fall back to
// forbidden without notice and reach the audit as an unknown reason.
func TestDecisionReasonsAreKnown(t *testing.T) {
	t.Parallel()
	known := auditing.KnownReasons()
	for _, r := range authorize.DecisionReasons() {
		if !slices.Contains(known, auditing.Reason(r)) {
			t.Errorf("reason %q of decision.go is not in auditing.KnownReasons", r)
		}
	}
}
