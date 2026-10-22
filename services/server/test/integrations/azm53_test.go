// SPDX-License-Identifier: Apache-2.0

package integrations_test

import (
	"net/http"
	"strconv"
	"testing"

	"github.com/OWNER/bothy/services/server/test/harness"
)

// TestAZM53_IncomingWebhookNeverTriggers (AZ-M53, SR4, T-TK-03): Support/Eskalation answers
// without mention (free_response); a native post of Tracker's webhook that mentions @lotse gives
// a message with origin app, no trigger_state, no run, no live box anchor and no run.state_changed
// or agent.invocation event; as a control Alex writes in the same channel without mention and one
// run of Lotse is queued, which proves the trigger path is live.
func TestAZM53_IncomingWebhookNeverTriggers(t *testing.T) {
	t.Parallel()
	h := newHW(t, options{})
	o := h.w.Owner
	var cur struct {
		SettingsVersion int64 `json:"settings_version"`
	}
	r := h.must(o, http.MethodGet, "/channels/"+h.eskalation+"/behavior", nil, http.StatusOK)
	var doc map[string]any
	r.JSON(t, &doc)
	r.JSON(t, &cur)
	delete(doc, "settings_version")
	doc["free_response"], doc["require_mention"] = true, false
	hdr := http.Header{}
	hdr.Set("If-Match", `"`+strconv.FormatInt(cur.SettingsVersion, 10)+`"`)
	harness.MustDo(t, o, harness.Request{Method: http.MethodPut, Path: "/channels/" + h.eskalation + "/behavior", Body: doc, Header: hdr}, http.StatusOK)

	runsInChannel := func() int {
		return h.count(`SELECT 1 FROM runs r JOIN threads t ON t.id = r.thread_id WHERE t.channel_id = $1`, h.eskalation)
	}
	before := runsInChannel()
	k := h.create(h.tracker, h.eskalation, "")
	res := h.postPath(k, native("@lotse the nightly backup failed, please look"))
	if res.status != http.StatusCreated {
		t.Fatalf("post: %d %s", res.status, res.body)
	}
	msg, _ := res.result(t)
	if m := h.message(msg); m.origin != "app" || m.triggerState != nil {
		t.Fatalf("stored %+v", m)
	}
	if n := runsInChannel(); n != before {
		t.Fatalf("%d runs after the webhook post, want %d", n, before)
	}
	if n := h.count(`SELECT 1 FROM runs WHERE triggering_message_id = $1`, msg); n != 0 {
		t.Fatal("a run names the webhook message")
	}
	if n := h.count(`SELECT 1 FROM event_log WHERE type IN ('run.state_changed', 'agent.invocation') AND channel_id = $1
		AND occurred_at >= (SELECT created_at FROM messages WHERE id = $2)`, h.eskalation, msg); n != 0 {
		t.Fatalf("%d run events after the webhook post", n)
	}
	if n := h.count(`SELECT 1 FROM messages WHERE channel_id = $1 AND kind = 'run' AND created_at >= (SELECT created_at FROM messages WHERE id = $2)`,
		h.eskalation, msg); n != 0 {
		t.Fatal("a live box anchor follows the webhook post")
	}
	// Control: a human message without mention in the same channel opens one queued run of Lotse.
	alex := h.w.People["Alex"]
	harness.MustDo(t, alex, harness.Request{Method: http.MethodPost, Path: "/threads/" + h.eskalationMain + "/messages",
		Body: map[string]any{"client_msg_id": newID(), "content_md": "The nightly backup failed again"}}, http.StatusCreated)
	if n := h.count(`SELECT 1 FROM runs r JOIN threads t ON t.id = r.thread_id WHERE t.channel_id = $1 AND r.state = 'queued' AND r.agent_id = $2`,
		h.eskalation, h.w.Agents["Lotse"].PrincipalID); n < 1 {
		t.Fatalf("the control message opened %d queued runs of Lotse", n)
	}
	if n := runsInChannel(); n != before+1 {
		t.Fatalf("%d runs after the control, want %d", n, before+1)
	}
}
