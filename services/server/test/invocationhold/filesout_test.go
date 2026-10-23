// SPDX-License-Identifier: Apache-2.0

package invocationhold_test

import (
	"net/http"
	"testing"

	"github.com/OWNER/bothy/services/server/test/harness"
)

// TestAgentAttachments_FilesOutSwitch (09 2.2, design note 15): Lotse uploads a file in Support
// and sends it in a thread: 201; after the owner sets operator_config.supports_files_out = false
// with updateAgent, the next send with a file gives 403 feature_disabled and stores neither the
// message nor an attachment row; a send without files still passes.
func TestAgentAttachments_FilesOutSwitch(t *testing.T) {
	t.Parallel()
	w := newWorld(t, worldOptions{})
	thread := w.w.Threads["Support/Allgemein"]
	content := []byte("a summary from the support agent")
	first := w.upload(w.lotse, w.allgemein, "summary.txt", content, content)
	if r := w.send(w.lotse, thread, "The summary", first); r.Status != http.StatusCreated {
		t.Fatalf("createMessage of Lotse with a file: %d %s", r.Status, r.Body)
	}
	agent := harness.MustDo(t, w.owner, harness.Request{Method: http.MethodGet, Path: "/agents/" + w.lotse.PrincipalID}, http.StatusOK)
	h := http.Header{}
	h.Set("If-Match", agent.Header.Get("ETag"))
	harness.MustDo(t, w.owner, harness.Request{Method: http.MethodPatch, Path: "/agents/" + w.lotse.PrincipalID, Header: h,
		Body: map[string]any{"operator_config": map[string]any{"supports_files_out": false}}}, http.StatusOK)

	second := w.upload(w.lotse, w.allgemein, "second.txt", content, content)
	messages := w.count(`SELECT count(*) FROM messages WHERE thread_id = $1`, thread)
	r := w.send(w.lotse, thread, "Another summary", second)
	if r.Status != http.StatusForbidden || problemCode(r.Body) != "feature_disabled" {
		t.Fatalf("createMessage after the switch: %d %s", r.Status, r.Body)
	}
	if n := w.count(`SELECT count(*) FROM messages WHERE thread_id = $1`, thread); n != messages {
		t.Fatalf("%d messages stored by the refused send", n-messages)
	}
	if n := w.count(`SELECT count(*) FROM message_attachments WHERE file_id = $1`, second); n != 0 {
		t.Fatalf("%d attachment rows of the refused send", n)
	}
	if r := w.send(w.lotse, thread, "A summary without a file"); r.Status != http.StatusCreated {
		t.Fatalf("createMessage of Lotse without a file: %d %s", r.Status, r.Body)
	}
}
