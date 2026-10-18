// SPDX-License-Identifier: Apache-2.0

package ws

import (
	"encoding/json"
	"testing"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/gateway/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
)

// TestDecode_FocusAndTyping: the bodies of focus, typing.start and typing.stop decode into the
// frames of the app package after their schema; an unknown member and a non-UUID fail.
func TestDecode_FocusAndTyping(t *testing.T) {
	t.Parallel()
	schemas := map[string]string{"focus": "GatewayFocus", "typing.start": "GatewayTyping", "typing.stop": "GatewayTyping"}
	check, err := httpserver.NewSchemaValidator(rest.Spec, []string{"GatewayFocus", "GatewayTyping"})
	if err != nil {
		t.Fatal(err)
	}
	dec := NewDecoder(schemas, check)
	a, b := ids.New(), ids.New()
	raw, _ := json.Marshal(map[string]any{"thread_ids": []ids.ID{a, b}, "idle": true})
	got, err := dec.Decode("focus", raw)
	if f, ok := got.(app.Focus); err != nil || !ok || len(f.ThreadIDs) != 2 || f.ThreadIDs[0] != a || f.ThreadIDs[1] != b || !f.Idle {
		t.Errorf("focus: %#v %v", got, err)
	}
	got, err = dec.Decode("focus", json.RawMessage(`{"thread_ids":[]}`))
	if f, ok := got.(app.Focus); err != nil || !ok || len(f.ThreadIDs) != 0 || f.Idle {
		t.Errorf("empty focus: %#v %v", got, err)
	}
	for _, op := range []string{"typing.start", "typing.stop"} {
		raw, _ := json.Marshal(map[string]any{"thread_id": a})
		got, err := dec.Decode(op, raw)
		if f, ok := got.(app.Typing); err != nil || !ok || f.ThreadID != a {
			t.Errorf("%s: %#v %v", op, got, err)
		}
	}
	for op, body := range map[string]string{
		"focus":        `{"thread_ids":[],"extra":1}`,
		"typing.start": `{"thread_id":"not-a-uuid"}`,
		"typing.stop":  `{}`,
	} {
		if _, err := dec.Decode(op, json.RawMessage(body)); err == nil {
			t.Errorf("%s %s decoded", op, body)
		}
	}
}
