// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/importer"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Times of the thread items of the lead tests: the run that posts the thread and a later run, whose
// later activity the mirror rules would take over.
const (
	firstRun = "2025-03-01T08:00:00Z"
	laterRun = "2025-03-02T08:00:00Z"
)

// threadData is the data of a thread item of the Discord channel with the last activity given.
func threadData(channel, activity string) string {
	return `{"channel_origin_key":"channel:` + channel + `","name":"T","created_at":"` + firstRun + `","last_activity_at":"` + activity +
		`","closed":false}`
}

// postThread posts the thread item of key in the Discord channel and returns its thread.
func (c *contentEnv) postThread(t *testing.T, key, channel string) ids.ID {
	t.Helper()
	res, err := c.svc.ApplyBatch(context.Background(), c.owner, c.src.ID, thread(key, o(key, "thread", threadData(channel, firstRun))))
	if err != nil || res.Items[0].State != importer.ItemPosted {
		t.Fatalf("thread %s of channel %s: %+v %v", key, channel, res, err)
	}
	return res.Items[0].TargetID
}

// replay posts the thread item of key again as a later run sends it, naming the Discord channel.
func (c *contentEnv) replay(key, channel string) (importer.BatchResult, error) {
	return c.svc.ApplyBatch(context.Background(), c.owner, c.src.ID, thread(key, o(key, "thread", threadData(channel, laterRun))))
}

// answered fails unless a replay answered the posted thread item without an error.
func answered(t *testing.T, name string, res importer.BatchResult, err error) {
	t.Helper()
	if err != nil || len(res.Items) != 1 || res.Items[0].State != importer.ItemPosted {
		t.Fatalf("%s: %+v %v, want the posted item", name, res, err)
	}
}

// mirrored returns how many mirror changes the fake of chat received.
func (c *contentEnv) mirrored() int {
	c.threads.mu.Lock()
	defer c.threads.mu.Unlock()
	return len(c.threads.mirrors)
}

// remap rewrites the channel entries of the stored mapping of the source, as a stored value that no
// operation of the interface writes (the mapping of a source changes only by its ownership list).
func (c *contentEnv) remap(t *testing.T, edit func(channels []any) []any) {
	t.Helper()
	c.store.mu.Lock()
	defer c.store.mu.Unlock()
	src := c.store.sources[c.src.ID]
	var m map[string]any
	if err := json.Unmarshal(src.Config, &m); err != nil {
		t.Fatal(err)
	}
	channels, _ := m["channels"].([]any)
	m["channels"] = edit(channels)
	config, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	src.Config = config
	c.store.sources[c.src.ID] = src
}

// channelEntry returns the entry of the Discord channel among the channel entries of a mapping.
func channelEntry(channels []any, channel string) map[string]any {
	for _, raw := range channels {
		if e, _ := raw.(map[string]any); e["discord_channel"] == channel {
			return e
		}
	}
	return nil
}

// TestPostedThreadItemOfAnotherChannel (22 7.2 and 12.1): whether a posted thread follows the
// source is decided by the channel the stored thread lies in, never by the request. A thread item
// that names another channel than that one (a channel the source leads, a channel the mapping does
// not name, a skipped channel, a channel led here) is refused with 409 target_outside_source,
// recorded as a denial, and changes nothing. Before, the server read the lead from the
// channel_origin_key of the request alone, so such an item switched the mirror of a thread in a
// channel led here on.
func TestPostedThreadItemOfAnotherChannel(t *testing.T) {
	t.Parallel()
	c := newContentEnv(t)
	ctx := context.Background()
	if _, err := c.svc.ApplyBatch(ctx, c.owner, c.src.ID, importer.Batch{Items: []importer.BatchObject{
		o("channel:23", "channel", `{"name":"gone","kind":"forum","sort_key":"a2"}`)}}); err != nil {
		t.Fatal(err)
	}
	// Thread 40 lies in channel 21, which this server leads; thread 41 in channel 20, which the
	// source leads.
	c.postThread(t, "thread:40", "21")
	c.postThread(t, "thread:41", "20")
	for _, tc := range []struct{ name, key, channel string }{
		{"a thread led here named as one of a channel the source leads", "thread:40", "20"},
		{"a thread led here named as one of a channel the mapping does not name", "thread:40", "29"},
		{"a thread led here named as one of a skipped channel", "thread:40", "23"},
		{"a thread the source leads named as one of a channel led here", "thread:41", "21"},
	} {
		denied := len(c.rec.denied)
		_, err := c.replay(tc.key, tc.channel)
		if e, ok := errs.As(err); !ok || e.Code != errs.CodeConflict || e.Reason != importer.ReasonTargetOutsideSource {
			t.Errorf("%s: %v, want 409 target_outside_source", tc.name, err)
		}
		if n := len(c.rec.denied) - denied; n != 1 {
			t.Errorf("%s: %d denials recorded, want 1", tc.name, n)
		}
	}
	if n := c.mirrored(); n != 0 {
		t.Errorf("refused thread items applied %d mirror changes", n)
	}
	// The item that names the channel of its thread is answered as before: no mirror where this
	// server leads, the mirror where the source leads.
	res, err := c.replay("thread:40", "21")
	answered(t, "the thread of a channel led here", res, err)
	if n := c.mirrored(); n != 0 {
		t.Errorf("the thread of a channel led here applied %d mirror changes", n)
	}
	res, err = c.replay("thread:41", "20")
	answered(t, "the thread of a channel the source leads", res, err)
	if n := c.mirrored(); n != 1 {
		t.Errorf("the thread of a channel the source leads applied %d mirror changes, want 1", n)
	}
}

// TestPostedThreadOfUnmappedChannel (22 7.2): a posted thread whose channel the stored mapping does
// not name is answered and never mirrored. Before, a channel without an entry counted as led by
// the source.
func TestPostedThreadOfUnmappedChannel(t *testing.T) {
	t.Parallel()
	c := newContentEnv(t)
	c.postThread(t, "thread:41", "20")
	c.remap(t, func(channels []any) []any {
		kept := make([]any, 0, len(channels))
		for _, raw := range channels {
			if e, _ := raw.(map[string]any); e["discord_channel"] != "20" {
				kept = append(kept, raw)
			}
		}
		return kept
	})
	res, err := c.replay("thread:41", "20")
	answered(t, "the thread of a channel the mapping does not name", res, err)
	if n := c.mirrored(); n != 0 {
		t.Errorf("the thread of a channel the mapping does not name applied %d mirror changes", n)
	}
}

// TestPostedThreadMovedHere (22 7.2): a thread that was moved here (thread.move) no longer lies in
// the channel its item names. Its item is still answered, so a full run goes on, and the channel
// the thread lies in now decides: no mirror in a channel outside the source and in a channel led
// here, the mirror in a channel the source leads. Before, the channel named by the request decided.
func TestPostedThreadMovedHere(t *testing.T) {
	t.Parallel()
	c := newContentEnv(t)
	id := c.postThread(t, "thread:41", "20")
	for _, tc := range []struct {
		name    string
		channel ids.ID
		mirrors int
	}{
		{"a thread moved into a channel outside the source", ids.New(), 0},
		{"a thread moved into a channel led here", c.items["channel:21"].TargetID, 0},
		{"a thread moved back into a channel the source leads", c.items["channel:20"].TargetID, 1},
	} {
		c.threads.move(id, tc.channel)
		before := c.mirrored()
		res, err := c.replay("thread:41", "20")
		answered(t, tc.name, res, err)
		if n := c.mirrored() - before; n != tc.mirrors {
			t.Errorf("%s applied %d mirror changes, want %d", tc.name, n, tc.mirrors)
		}
	}
	// Also the item that names the channel the moved thread lies in now cannot switch the mirror on.
	c.threads.move(id, c.items["channel:21"].TargetID)
	before := c.mirrored()
	res, err := c.replay("thread:41", "21")
	answered(t, "a moved thread named as one of the channel it lies in", res, err)
	if n := c.mirrored() - before; n != 0 {
		t.Errorf("a moved thread in a channel led here applied %d mirror changes", n)
	}
}

// TestPostedThreadOfSharedChannel (22 7.2): when the mapping puts two Discord channels onto one
// channel here, a thread there is mirrored only while the source leads both. Which of the two the
// item names changes nothing. Before, the item chose the entry that decided.
func TestPostedThreadOfSharedChannel(t *testing.T) {
	t.Parallel()
	c := newContentEnv(t)
	ctx := context.Background()
	// Discord channel 24 is mapped onto the existing channel of channel 22 and led here.
	c.remap(t, func(channels []any) []any {
		return append(channels, map[string]any{"discord_channel": "24", "target": map[string]any{"existing": c.channel.String()},
			"planned_operator": "none", "lead": "bothy"})
	})
	adopt := `{"existing_channel_id":"` + c.channel.String() + `","kind":"text","sort_key":"a3"}`
	if _, err := c.svc.ApplyBatch(ctx, c.owner, c.src.ID, importer.Batch{Items: []importer.BatchObject{
		o("channel:22", "channel", adopt), o("channel:24", "channel", adopt)}}); err != nil {
		t.Fatal(err)
	}
	c.postThread(t, "thread:47", "22")
	for _, channel := range []string{"22", "24"} {
		res, err := c.replay("thread:47", channel)
		answered(t, "a thread of a shared channel named as one of channel "+channel, res, err)
	}
	if n := c.mirrored(); n != 0 {
		t.Fatalf("a thread of a channel one mapped channel leads here applied %d mirror changes", n)
	}
	c.remap(t, func(channels []any) []any {
		channelEntry(channels, "24")["lead"] = "discord"
		return channels
	})
	res, err := c.replay("thread:47", "24")
	answered(t, "a thread of a shared channel the source leads", res, err)
	if n := c.mirrored(); n != 1 {
		t.Errorf("a thread of a shared channel the source leads applied %d mirror changes, want 1", n)
	}
}
