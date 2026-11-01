// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// TestStatsRunChats (owner request 2026-11-01, 25 4 rule 6): the chats of the asked threads in the
// channel set, with the channel name and the thread title, the main thread without a title; a
// thread of a channel outside the set and an unknown thread are left out.
func TestStatsRunChats(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	alex, _ := e.person(t)
	space, text, forum := ids.New(), ids.New(), ids.New()
	e.exec(t, `INSERT INTO spaces (id, name, sort_key, created_by) VALUES ($1, 'Engineering', 'a0', $2)`, space.String(), alex.String())
	e.exec(t, `INSERT INTO channels (id, space_id, kind, name, sort_key, history_backfill_limit, created_by)
		VALUES ($1, $2, 'text', 'backend', 'a0', 20, $3)`, text.String(), space.String(), alex.String())
	e.exec(t, `INSERT INTO channels (id, space_id, kind, name, sort_key, history_backfill_limit, created_by)
		VALUES ($1, $2, 'forum', 'travel-service', 'a1', 20, $3)`, forum.String(), space.String(), alex.String())
	main, post := ids.New(), ids.New()
	e.exec(t, `INSERT INTO threads (id, channel_id, kind, title, created_by) VALUES ($1, $2, 'main', 'ignored', $3)`,
		main.String(), text.String(), alex.String())
	e.exec(t, `INSERT INTO threads (id, channel_id, kind, title, created_by) VALUES ($1, $2, 'thread', 'Payment integration', $3)`,
		post.String(), forum.String(), alex.String())

	got, err := e.repo.StatsRunChats(context.Background(), []ids.ID{text, forum}, []ids.ID{main, post, ids.New()})
	if err != nil {
		t.Fatal(err)
	}
	want := map[ids.ID]chat.StatsRunChat{
		main: {ThreadID: main, ChannelID: text, ChannelName: "backend"},
		post: {ThreadID: post, ChannelID: forum, ChannelName: "travel-service", ThreadTitle: "Payment integration"},
	}
	if len(got) != len(want) {
		t.Fatalf("chats %+v", got)
	}
	for _, c := range got {
		if want[c.ThreadID] != c {
			t.Fatalf("chat %+v, want %+v", c, want[c.ThreadID])
		}
	}
	got, err = e.repo.StatsRunChats(context.Background(), []ids.ID{text}, []ids.ID{main, post})
	if err != nil || len(got) != 1 || got[0].ThreadID != main {
		t.Fatalf("chats outside the channel set: %+v, %v", got, err)
	}
}
