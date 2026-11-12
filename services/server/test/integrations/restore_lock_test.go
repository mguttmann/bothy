// SPDX-License-Identifier: Apache-2.0

package integrations_test

import (
	"net/http"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/wire"
	"github.com/OWNER/bothy/services/server/test/harness"
)

// TestInternalListener_NoIncomingWebhookBeforeRestoreFinalize (R-DM-50, 16 section 8.4): on a
// restored database whose restore was not finalized, a process of every role stays not ready and
// its internal listener serves no incoming webhook. A post with a valid token answers the 404 of
// an unknown route and stores no message, so a sender inside the stack (Alertmanager) writes
// nothing on the restored timeline before bothy restore finalize.
func TestInternalListener_NoIncomingWebhookBeforeRestoreFinalize(t *testing.T) {
	t.Parallel()
	h := newHW(t, options{})
	k := h.create(h.tracker, h.eskalation, "")
	// A restore to a point in time: the cluster is on a newer timeline than the stored one.
	tag, err := h.s.AdminDB().Exec(h.ctx, `UPDATE instance SET pg_timeline_id = pg_timeline_id + 1 WHERE pg_timeline_id IS NOT NULL`)
	if err != nil || tag.RowsAffected() != 1 {
		t.Fatalf("the instance row stores no database identity: %d rows, %v", tag.RowsAffected(), err)
	}
	for _, role := range []wire.Role{wire.RoleAPI, wire.RoleWorker, wire.RoleAll} {
		n := harness.StartNode(t, h.s, harness.Options{EgressDenyAll: true, Role: role, Config: harness.RaisedRateLimits(), NotReady: true})
		content := "posted before the finalize, role " + string(role)
		r := h.send(n.InternalURL+"/api/v1/hooks/"+k.id+"/"+k.token, marshal(t, native(content)), nil)
		if code, _, _, _, _ := r.problem(); r.status != http.StatusNotFound || code != "not_found" {
			t.Errorf("%s: a post under the start lock answers %d %s, want 404 not_found", role, r.status, r.body)
		}
		if stored := h.count(`SELECT 1 FROM messages WHERE content_md = $1`, content); stored != 0 {
			t.Errorf("%s: a post under the start lock stored %d messages", role, stored)
		}
	}
}
