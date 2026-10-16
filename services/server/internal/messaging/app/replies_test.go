// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	mw "github.com/OWNER/bothy/services/server/test/messagingworld"
)

// TestReplyTargetInThread (M1-61): a message lies in its thread, deleted or not; another thread
// and an unknown id answer false alike.
func TestReplyTargetInThread(t *testing.T) {
	t.Parallel()
	w := mw.New(t, subscribe)
	_, _, main := w.TextChannel()
	other := w.MainThread(w.Channel(w.Space("Other"), "text", "general"))
	m := w.Send("owner", main, "hello", http.StatusCreated)
	msg := ids.MustParse(m["id"].(string))
	ctx := context.Background()
	for _, c := range []struct {
		message, thread ids.ID
		want            bool
	}{{msg, main, true}, {msg, other, false}, {ids.New(), main, false}} {
		got, err := w.Svc.InThread(ctx, c.message, c.thread)
		if err != nil || got != c.want {
			t.Errorf("InThread(%s, %s) = %v, %v; want %v", c.message, c.thread, got, err, c.want)
		}
	}
}
