// SPDX-License-Identifier: Apache-2.0

package wire_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// snapshotFollowups decodes the follow-ups of a run.updated snapshot as message id to state.
func snapshotFollowups(t *testing.T, f lbFrame) map[string]string {
	t.Helper()
	var doc struct {
		Followups map[string]struct {
			MessageID string `json:"message_id"`
			State     string `json:"state"`
		} `json:"followups"`
	}
	if err := json.Unmarshal(f.ev.Payload.Snapshot, &doc); err != nil {
		t.Fatalf("snapshot %s: %v", f.raw, err)
	}
	out := map[string]string{}
	for k, v := range doc.Followups {
		if v.MessageID != k {
			t.Fatalf("follow-up %s keyed by %s", v.MessageID, k)
		}
		out[k] = v.State
	}
	return out
}

// mention returns "@<handle> " of the agent of r: the run lives in the main thread of a channel
// with require_mention, so a follow-up there names the agent.
func (w *lbWorld) mention(t *testing.T, r lbRun) string {
	t.Helper()
	var handle string
	if err := w.queryRow(context.Background(), `SELECT handle FROM principals WHERE id = $1`, r.agent.id.String()).Scan(&handle); err != nil {
		t.Fatal(err)
	}
	return "@" + handle + " "
}

// TestLiveBoxFollowups is the gateway half of M2-11 against one started api process with real
// modules: a follow-up raises the run version without run.updated, and the adapter's next patch
// reaches the focused client as a snapshot with the follow-up (design note 10); the follow-up
// states over the op run.update move forward only and keep the connection.
func TestLiveBoxFollowups(t *testing.T) {
	t.Parallel()
	w := newLBWorld(t)

	t.Run("TestFocusedClientSeesFollowupAfterVersionStep", func(t *testing.T) {
		r := w.running(t, "fu-focus")
		alex, _ := w.ready(t, w.params(w.owner))
		focus(t, alex, r.thread)
		snap := mustRunUpdated(t, alex, r.id)
		if got := snapshotFollowups(t, snap); len(got) != 0 {
			t.Fatalf("follow-ups before the follow-up %v", got)
		}
		m, state := w.say(w.owner, r.thread, w.mention(t, r)+"one more thing")
		if state != "received" {
			t.Fatalf("follow-up trigger_state %q", state)
		}
		if v := w.version(r.id); v != snap.ev.Payload.Version+1 {
			t.Fatalf("version %d after the follow-up, snapshot at %d", v, snap.ev.Payload.Version)
		}
		noRunUpdated(t, alex, r.id, 3*lbCoalesce, "Alex after the follow-up")
		w.patchOK(t, r.agent, r.id, snap.ev.Payload.Version+1, op("replace", "/status_line", "Reading the addition"))
		f := mustRunUpdated(t, alex, r.id)
		if !f.snapshot() || snapshotFollowups(t, f)[m] != "queued" {
			t.Fatalf("frame after the version step %s", f.raw)
		}
	})

	t.Run("TestFollowupStatesOverGateway", func(t *testing.T) {
		r := w.running(t, "fu-gateway")
		m, state := w.say(w.owner, r.thread, w.mention(t, r)+"an addition")
		if state != "received" {
			t.Fatalf("follow-up trigger_state %q", state)
		}
		base := w.version(r.id)
		runUpdate(t, r.conn, r.id, base, "fu-absorbed", op("replace", "/followups/"+m+"/state", "absorbed"))
		eventually(t, 15*time.Second, "the absorbed follow-up", func() bool { return w.version(r.id) == base+1 })
		runUpdate(t, r.conn, r.id, base+1, "fu-back", op("replace", "/followups/"+m+"/state", "delivered"))
		wantOpError(t, r.conn, "invalid_state_transition", "fu-back")
		runUpdate(t, r.conn, r.id, base+1, "fu-missing", op("replace", "/followups/"+r.id+"/state", "delivered"))
		e := wantOpError(t, r.conn, "validation_failed", "fu-missing")
		if !strings.Contains(deref(e.Detail), "patch_target_missing") {
			t.Fatalf("detail %q", deref(e.Detail))
		}
		// The connection stays: the next op is stored.
		runUpdate(t, r.conn, r.id, base+1, "", op("replace", "/status_line", "Still here"))
		eventually(t, 15*time.Second, "the patch after the errors", func() bool { return w.version(r.id) == base+2 })
	})
}
