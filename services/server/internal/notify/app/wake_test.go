// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/notify/domain"
	"github.com/OWNER/bothy/services/server/internal/sync"
)

// wakeEvent returns run.wake_changed of a wake-up of person in thread at log seq (AR-04).
func (w *pushWorld) wakeEvent(seq int64, thread, agent, person, note ids.ID, state string) sync.Event {
	payload := map[string]any{"wake_id": ids.New(), "thread_id": thread, "agent_id": agent, "triggered_by": person, "state": state}
	if !note.IsZero() {
		payload["note_message_id"], payload["note_thread_seq"] = note, 7
	}
	return w.event(seq, "run.wake_changed", "thread", thread, ids.ID{}, payload)
}

// TestClassify_WakeChanged (13 section 2, R-PN-12, AR-04): a wake-up that ends refused, expired
// or failed gives wake_ended to its person only, pointing at the note; fired, cancelled and pending
// give nothing; another reader gets nothing.
func TestClassify_WakeChanged(t *testing.T) {
	t.Parallel()
	for _, state := range []string{"refused", "expired", "failed"} {
		w, _, _ := runWorld(t)
		note := ids.New()
		w.mustHandle(w.wakeEvent(60, w.thread, w.lotse, w.alex, note, state))
		h := w.hints()
		wantKinds(t, h, w.alex, state+" alex", domain.KindWakeEnded)
		wantKinds(t, h, w.sam, state+" sam")
		for _, j := range w.jobsOf(w.alex) {
			if j.MessageID != note || j.ThreadID != w.thread {
				t.Errorf("%s: job %+v, want the note in the thread", state, j)
			}
		}
	}
	for _, state := range []string{"fired", "cancelled", "pending"} {
		w, _, _ := runWorld(t)
		w.mustHandle(w.wakeEvent(60, w.thread, w.lotse, w.alex, ids.New(), state))
		if len(w.sched.sends) != 0 {
			t.Errorf("%s: %d jobs", state, len(w.sched.sends))
		}
	}
}

// TestClassify_UsageLimitedRun (R-PN-12, AR-04): a run that failed with usage_limited gives its
// person wake_ended instead of run_failed; the other readers keep their message hint.
func TestClassify_UsageLimitedRun(t *testing.T) {
	t.Parallel()
	w, run, _ := runWorld(t, 2, 3)
	ev := w.runEvent(50, run, w.thread, w.lotse, w.alex, "failed")
	ev.Payload = []byte(`{"run":{"id":"` + run.String() + `","thread_id":"` + w.thread.String() + `","channel_id":"` +
		w.threads[w.thread].ChannelID.String() + `","agent_id":"` + w.lotse.String() + `","triggered_by":"` + w.alex.String() +
		`","state":"failed","state_reason":"usage_limited"}}`)
	w.mustHandle(ev)
	h := w.hints()
	wantKinds(t, h, w.alex, "alex", domain.KindWakeEnded)
	wantKinds(t, h, w.sam, "sam", domain.KindMessage)
}
