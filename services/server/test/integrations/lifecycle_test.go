// SPDX-License-Identifier: Apache-2.0

package integrations_test

import (
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/test/harness"
)

// shiftClock is the real time plus an offset the test moves (Advance): the world is built on real
// time, so the DPoP proofs of the clients stay fresh, and the webhook posts after a move need no
// proof.
type shiftClock struct {
	mu     sync.Mutex
	offset time.Duration
}

func (c *shiftClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return time.Now().UTC().Add(c.offset)
}

func (c *shiftClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.offset += d
}

// TestLastUsedAtGranularity (D-M2-27-9): two posts within identity.last_active_granularity write
// last_used_at once; after the window it moves.
func TestLastUsedAtGranularity(t *testing.T) {
	t.Parallel()
	clk := &shiftClock{}
	h := newHW(t, options{opts: func(o *harness.Options) { o.Clock = clk }})
	k := h.create(h.tracker, h.eskalation, "")
	used := func() time.Time {
		t.Helper()
		var at time.Time
		h.row(`SELECT last_used_at FROM incoming_webhooks WHERE id = $1`, []any{k.id}, &at)
		return at
	}
	if r := h.postPath(k, native("first")); r.status != http.StatusCreated {
		t.Fatalf("%d %s", r.status, r.body)
	}
	first := used()
	h.postPath(k, native("second"))
	if !used().Equal(first) {
		t.Fatal("last_used_at moved within the window")
	}
	var window time.Duration
	window, err := time.ParseDuration(h.s.ConfigValue("identity.last_active_granularity"))
	if err != nil {
		t.Fatal(err)
	}
	clk.Advance(window + time.Second)
	h.postPath(k, native("third"))
	if !used().After(first) {
		t.Fatal("last_used_at did not move after the window")
	}
}

// TestChannelArchiveRevokesIncomingWebhooks (D-M2-27-14): archiveChannel revokes the active
// webhooks of the channel in its transaction; a webhook of another channel stays active.
func TestChannelArchiveRevokesIncomingWebhooks(t *testing.T) {
	t.Parallel()
	h := newHW(t, options{})
	o := h.w.Owner
	ch := created(t, h.must(o, http.MethodPost, "/spaces/"+h.w.Spaces["Support"]+"/channels", map[string]any{"kind": "text", "name": "Alt"}, http.StatusCreated))
	h.grantApp(h.tracker, ch, "write")
	gone := h.create(h.tracker, ch, "")
	stays := h.create(h.tracker, h.eskalation, "")
	h.must(o, http.MethodPost, "/channels/"+ch+"/archive", map[string]any{"reason": "Done"}, http.StatusOK)
	var revoked, archived time.Time
	h.row(`SELECT w.revoked_at, c.archived_at FROM incoming_webhooks w JOIN channels c ON c.id = w.channel_id WHERE w.id = $1`, []any{gone.id}, &revoked, &archived)
	if !revoked.Equal(archived) {
		t.Fatalf("revoked_at %v, archived_at %v", revoked, archived)
	}
	if n := h.count(`SELECT 1 FROM incoming_webhooks WHERE id = $1 AND revoked_at IS NULL`, stays.id); n != 1 {
		t.Fatal("the webhook of another channel was revoked")
	}
}
