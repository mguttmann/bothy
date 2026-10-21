// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// TestRepositoryUnreadCandidates (M3-19 design note 16 a): the threads of the given channels with
// last_message_seq above unread_floor_seq, each with its channel, the channel kind (text, forum,
// dm) and both counters, ascending by thread id; other channels, threads without a message above
// the floor and an empty channel list give nothing.
func TestRepositoryUnreadCandidates(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ctx := context.Background()
	alex, _ := e.person(t)
	space, text, forum, dm, other := ids.New(), ids.New(), ids.New(), ids.New(), ids.New()
	e.exec(t, `INSERT INTO spaces (id, name, sort_key, created_by) VALUES ($1, 'Support', 'a0', $2)`, space.String(), alex.String())
	for _, c := range []struct {
		id         ids.ID
		kind, name string
	}{{text, "text", "general"}, {forum, "forum", "questions"}, {other, "text", "elsewhere"}} {
		e.exec(t, `INSERT INTO channels (id, space_id, kind, name, sort_key, history_backfill_limit, created_by)
			VALUES ($1, $2, $3, $4, 'a0', 20, $5)`, c.id.String(), space.String(), c.kind, c.name, alex.String())
	}
	e.exec(t, `INSERT INTO channels (id, kind, name, sort_key, operator_inherit, history_backfill_limit, created_by)
		VALUES ($1, 'dm', '', 'a0', false, 20, $2)`, dm.String(), alex.String())
	thread := func(ch ids.ID, kind string, last, floor int) ids.ID {
		id := ids.New()
		e.exec(t, `INSERT INTO threads (id, channel_id, kind, created_by, last_message_seq, unread_floor_seq) VALUES ($1, $2, $3, $4, $5, $6)`,
			id.String(), ch.String(), kind, alex.String(), last, floor)
		return id
	}
	want := map[ids.ID]chat.ThreadPosition{}
	for _, p := range []chat.ThreadPosition{
		{ThreadID: thread(text, "main", 7, 1), ChannelID: text, ChannelKind: "text", LastMessageSeq: 7, UnreadFloorSeq: 1},
		{ThreadID: thread(text, "thread", 2, 0), ChannelID: text, ChannelKind: "text", LastMessageSeq: 2},
		{ThreadID: thread(forum, "thread", 5, 4), ChannelID: forum, ChannelKind: "forum", LastMessageSeq: 5, UnreadFloorSeq: 4},
		{ThreadID: thread(dm, "main", 3, 0), ChannelID: dm, ChannelKind: "dm", LastMessageSeq: 3},
	} {
		want[p.ThreadID] = p
	}
	thread(forum, "thread", 4, 4) // nothing unread for anybody
	thread(forum, "thread", 0, 0) // no message
	thread(other, "main", 9, 0)   // channel not asked
	got, err := e.repo.UnreadCandidates(ctx, []ids.ID{text, forum, dm})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(want) {
		t.Fatalf("candidates %+v", got)
	}
	for i, p := range got {
		if p != want[p.ThreadID] {
			t.Errorf("candidate %+v, want %+v", p, want[p.ThreadID])
		}
		if i > 0 && got[i-1].ThreadID.String() >= p.ThreadID.String() {
			t.Errorf("not ascending by thread id at %d: %v", i, got)
		}
	}
	if none, err := e.repo.UnreadCandidates(ctx, nil); err != nil || len(none) != 0 {
		t.Fatalf("no channels: %+v %v", none, err)
	}
}
