// SPDX-License-Identifier: Apache-2.0

package terminalsessions

import (
	"net/http"
	"testing"

	"github.com/OWNER/bothy/services/server/test/harness"
)

// TestList_DenialIsAudited (TM-T17, R-AZ-04, found by the conformance suite of M2-48): the 403 of
// listTerminalSessions for an agent token is a denial of a request, so it writes one deny entry of
// terminal.read_own with reason subject_kind_not_allowed, like the other four session operations;
// only the rows a list filters out write none.
func TestList_DenialIsAudited(t *testing.T) {
	t.Parallel()
	w := newWorld(t, nil)
	expect(t, "Lotse list", w.lotse.Do(t.Context(), harness.Request{Method: http.MethodGet, Path: "/terminal/sessions"}),
		http.StatusForbidden, "forbidden", "subject_kind_not_allowed")
	w.eventually("the deny entry of the list", func() bool {
		return w.count(`SELECT count(*) FROM audit_events WHERE action = 'terminal.read_own' AND decision = 'deny'
			AND deny_reason = 'subject_kind_not_allowed' AND actor_principal_id = $1`, w.lotse.PrincipalID) == 1
	})
}
