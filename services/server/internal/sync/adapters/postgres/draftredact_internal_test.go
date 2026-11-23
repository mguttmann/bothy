// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"encoding/json"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
)

// TestBlankDraftText: content_md of a stored draft.updated becomes empty and the other members
// stay; another event type stays as stored; a payload that is not an object or has no draft is an
// internal error.
func TestBlankDraftText(t *testing.T) {
	t.Parallel()
	got, err := blankDraftText(typeDraftUpdated, 1, json.RawMessage(`{"draft":{"id":"d1","content_md":"secret"},"v":1}`))
	if err != nil {
		t.Fatal(err)
	}
	var p struct {
		Draft map[string]string `json:"draft"`
		V     int               `json:"v"`
	}
	if err := json.Unmarshal(got, &p); err != nil || p.Draft["content_md"] != "" || p.Draft["id"] != "d1" || p.V != 1 {
		t.Fatalf("redacted %s (%v), want an empty content_md and the other members", got, err)
	}
	other := json.RawMessage(`{"draft_id":"d1"}`)
	if got, err := blankDraftText("draft.deleted", 1, other); err != nil || string(got) != string(other) {
		t.Fatalf("draft.deleted gave %s, %v; want it unchanged", got, err)
	}
	for name, payload := range map[string]string{"not an object": `[1]`, "no draft": `{"v":1}`} {
		if _, err := blankDraftText(typeDraftUpdated, 1, json.RawMessage(payload)); !isInternal(err) {
			t.Errorf("%s: err = %v, want internal_error", name, err)
		}
	}
}

func isInternal(err error) bool {
	e, ok := errs.As(err)
	return ok && e.Code == errs.CodeInternalError
}
