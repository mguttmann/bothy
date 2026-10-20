// SPDX-License-Identifier: Apache-2.0

package wire_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/gateway/adapters/wstest"
	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
)

// typingFrame is a server frame as the typing test reads it.
type typingFrame struct {
	op string
	d  json.RawMessage
}

// readUntilOp reads the frames of c until one with op arrives (included).
func readUntilOp(t *testing.T, c *wstest.Client, op string) []typingFrame {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), testkit.Patience)
	defer cancel()
	var out []typingFrame
	for {
		got, d, err := c.Next(ctx)
		if err != nil {
			t.Fatalf("waiting for %s after %d frames: %v", op, len(out), err)
		}
		out = append(out, typingFrame{op: got, d: d})
		if got == op {
			return out
		}
	}
}

// typingStarted returns the typing.started dispatches of fs.
func typingStarted(t *testing.T, fs []typingFrame) []json.RawMessage {
	t.Helper()
	var out []json.RawMessage
	for _, f := range fs {
		if f.op != "dispatch" {
			continue
		}
		var ev struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(f.d, &ev); err != nil {
			t.Fatal(err)
		}
		if ev.Type == "typing.started" {
			out = append(out, f.d)
		}
	}
	return out
}

// marker sends a focus over sync.max_focused_threads on c, which answers error and keeps the
// focus, and reads up to that error: every frame an earlier op caused is then in the result.
func marker(t *testing.T, c *wstest.Client) []typingFrame {
	t.Helper()
	tooMany := make([]string, 21)
	for i := range tooMany {
		tooMany[i] = ids.New().String()
	}
	if err := c.Send("focus", map[string]any{"thread_ids": tooMany}); err != nil {
		t.Fatal(err)
	}
	fs := readUntilOp(t, c, "error")
	var e struct {
		Op     string `json:"op"`
		Code   string `json:"code"`
		Detail string `json:"detail"`
	}
	if err := json.Unmarshal(fs[len(fs)-1].d, &e); err != nil || e != (struct {
		Op     string `json:"op"`
		Code   string `json:"code"`
		Detail string `json:"detail"`
	}{"focus", "validation_failed", "sync.max_focused_threads"}) {
		t.Fatalf("marker error %+v %v", e, err)
	}
	return fs
}

// TestGateway_TypingEndToEnd (M1-66) against a started api process with real modules: Alex (the
// Owner) and Sam have write on the Support channel, Robin has no grant; Sam and Robin focus its
// thread (Robin's focus is dropped); Alex's second device focuses it too. Alex types: Sam
// receives typing.started with the scope of the thread and expires_at = occurred_at +
// sync.typing_ttl; Robin and Alex's second device receive nothing.
func TestGateway_TypingEndToEnd(t *testing.T) {
	t.Parallel()
	w := newDeliveryWorld(t)
	space := w.newSpace("Support")
	support := w.newChannel(space, "support")
	sam := w.newPerson("sam", identity.DeviceStateActive)
	robin := w.newPerson("robin", identity.DeviceStateActive)
	w.must(w.owner, http.MethodPost, "/api/v1/grants", map[string]any{
		"subject_kind": "user", "subject_id": sam.user.String(), "target_kind": "channel", "target_id": support.id, "level": "write", "reason": "test"})

	samC, _ := w.ready(t, w.params(sam))
	robinC, _ := w.ready(t, w.params(robin))
	alexC, _ := w.ready(t, w.params(w.owner))
	alexSecond, _ := w.ready(t, w.params(w.addSession(w.owner)))
	for _, c := range []*wstest.Client{samC, robinC, alexSecond} {
		if err := c.Send("focus", map[string]any{"thread_ids": []string{support.thread}}); err != nil {
			t.Fatal(err)
		}
		marker(t, c)
	}

	if err := alexC.Send("typing.start", map[string]any{"thread_id": support.thread}); err != nil {
		t.Fatal(err)
	}
	if got := typingStarted(t, marker(t, alexC)); len(got) != 0 {
		t.Errorf("the typist received %d typing.started", len(got))
	}
	var evs []json.RawMessage
	for len(evs) == 0 {
		evs = typingStarted(t, readUntilOp(t, samC, "dispatch"))
	}
	if got := typingStarted(t, marker(t, samC)); len(got) != 0 {
		t.Errorf("Sam received %d more typing.started", len(got))
	}
	var ev struct {
		Seq               *int64 `json:"seq"`
		PermissionVersion *int64 `json:"permission_version"`
		OccurredAt        string `json:"occurred_at"`
		Actor             struct {
			ID   string `json:"id"`
			Kind string `json:"kind"`
		} `json:"actor"`
		Scope struct {
			SpaceID   string `json:"space_id"`
			ChannelID string `json:"channel_id"`
			ThreadID  string `json:"thread_id"`
		} `json:"scope"`
		Payload struct {
			ThreadID    string `json:"thread_id"`
			PrincipalID string `json:"principal_id"`
			ExpiresAt   string `json:"expires_at"`
		} `json:"payload"`
	}
	if err := json.Unmarshal(evs[0], &ev); err != nil {
		t.Fatal(err)
	}
	if ev.Seq != nil || ev.PermissionVersion != nil {
		t.Errorf("seq %v permission_version %v on an ephemeral event", ev.Seq, ev.PermissionVersion)
	}
	if ev.Scope.SpaceID != space || ev.Scope.ChannelID != support.id || ev.Scope.ThreadID != support.thread {
		t.Errorf("scope %+v, want %s %s %s", ev.Scope, space, support.id, support.thread)
	}
	if ev.Actor.ID != w.owner.user.String() || ev.Actor.Kind != "user" || ev.Payload.PrincipalID != w.owner.user.String() ||
		ev.Payload.ThreadID != support.thread {
		t.Errorf("actor %+v payload %+v", ev.Actor, ev.Payload)
	}
	occurred, err1 := time.Parse(time.RFC3339Nano, ev.OccurredAt)
	expires, err2 := time.Parse(time.RFC3339Nano, ev.Payload.ExpiresAt)
	if err1 != nil || err2 != nil || expires.Sub(occurred) != 8*time.Second {
		t.Errorf("occurred_at %q expires_at %q: %v %v", ev.OccurredAt, ev.Payload.ExpiresAt, err1, err2)
	}
	for name, c := range map[string]*wstest.Client{"Robin": robinC, "Alex's second device": alexSecond} {
		if got := typingStarted(t, marker(t, c)); len(got) != 0 {
			t.Errorf("%s received %d typing.started", name, len(got))
		}
	}
}
