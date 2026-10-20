// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/chat/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// TestRepositoryLocateThreads (M2-05): one read answers the routing data of the known threads of
// the list with their channel and space, ascending by id; unknown ids are absent; the archived
// flag covers the channel and its space; inside a unit of work it sees the uncommitted rows.
func TestRepositoryLocateThreads(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ctx := context.Background()
	alex, _ := e.person(t)
	robin := e.agent(t, "robin")
	space, archivedSpace, forum, closedForum, gone := ids.New(), ids.New(), ids.New(), ids.New(), ids.New()
	e.exec(t, `INSERT INTO spaces (id, name, sort_key, created_by) VALUES ($1, 'Support', 'a0', $2)`, space.String(), alex.String())
	e.exec(t, `INSERT INTO spaces (id, name, sort_key, created_by, archived_at) VALUES ($1, 'Old', 'a1', $2, now())`,
		archivedSpace.String(), alex.String())
	e.exec(t, `INSERT INTO channels (id, space_id, kind, name, sort_key, history_backfill_limit, created_by)
		VALUES ($1, $2, 'forum', 'bugs', 'a0', 20, $3)`, forum.String(), space.String(), alex.String())
	e.exec(t, `INSERT INTO channels (id, space_id, kind, name, sort_key, history_backfill_limit, created_by, archived_at)
		VALUES ($1, $2, 'forum', 'done', 'a1', 20, $3, now())`, closedForum.String(), space.String(), alex.String())
	e.exec(t, `INSERT INTO channels (id, space_id, kind, name, sort_key, history_backfill_limit, created_by)
		VALUES ($1, $2, 'forum', 'old', 'a0', 20, $3)`, gone.String(), archivedSpace.String(), alex.String())
	live, inClosed, inGone := ids.New(), ids.New(), ids.New()
	e.exec(t, `INSERT INTO threads (id, channel_id, kind, state, closed_at, operator_agent_id, created_by, last_message_seq, unread_floor_seq, version)
		VALUES ($1, $2, 'thread', 'closed', now(), $3, $4, 7, 2, 5)`, live.String(), forum.String(), robin.String(), alex.String())
	e.exec(t, `INSERT INTO threads (id, channel_id, kind, created_by) VALUES ($1, $2, 'thread', $3)`, inClosed.String(), closedForum.String(), alex.String())
	e.exec(t, `INSERT INTO threads (id, channel_id, kind, created_by) VALUES ($1, $2, 'thread', $3)`, inGone.String(), gone.String(), alex.String())

	got, err := e.repo.LocateThreads(ctx, []ids.ID{inGone, live, ids.New(), inClosed})
	if err != nil || len(got) != 3 {
		t.Fatalf("locate %+v, %v", got, err)
	}
	for i := 1; i < len(got); i++ {
		if got[i-1].ID.String() >= got[i].ID.String() {
			t.Fatalf("not ascending: %+v", got)
		}
	}
	byID := map[ids.ID]chat.ThreadInfo{}
	for _, li := range got {
		byID[li.ID] = li
	}
	want := chat.ThreadInfo{ID: live, ChannelID: forum, SpaceID: space, ChannelKind: "forum", Kind: "thread", State: "closed",
		OperatorAgentID: robin, CreatedBy: alex, Version: 5, LastMessageSeq: 7, UnreadFloorSeq: 2}
	if byID[live] != want {
		t.Fatalf("live %+v, want %+v", byID[live], want)
	}
	if c := byID[inClosed]; !c.ChannelArchived || c.SpaceID != space || c.State != "open" || !c.OperatorAgentID.IsZero() {
		t.Fatalf("archived channel %+v", c)
	}
	if g := byID[inGone]; !g.ChannelArchived || g.SpaceID != archivedSpace {
		t.Fatalf("archived space %+v", g)
	}
	if none, err := e.repo.LocateThreads(ctx, nil); err != nil || len(none) != 0 {
		t.Fatalf("no threads: %+v %v", none, err)
	}

	// Inside a unit of work the read runs in its transaction and sees its uncommitted thread.
	fresh := ids.New()
	if err := e.write(t, func(ctx context.Context) error {
		if _, err := e.repo.InsertThread(ctx, domain.Thread{ID: fresh, ChannelID: forum, Kind: domain.ThreadKindThread, Title: "t",
			TitleSource: domain.TitleAuto, State: domain.ThreadOpen, CreatedBy: alex, CreatedAt: t0, LastActivityAt: t0, Version: 1,
			SearchLanguage: "simple"}); err != nil {
			return err
		}
		in, err := e.repo.LocateThreads(ctx, []ids.ID{fresh})
		if err != nil || len(in) != 1 || in[0].ChannelID != forum {
			t.Errorf("ambient transaction %+v, %v", in, err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
