// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// TestRepositoryThreadCounters (M1-63): the counters of the threads in the given channels with
// last_message_seq above unread_floor_seq; other channels and threads without an unread message
// for anybody are absent.
func TestRepositoryThreadCounters(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ctx := context.Background()
	alex, _ := e.person(t)
	space, a, b, c := ids.New(), ids.New(), ids.New(), ids.New()
	e.exec(t, `INSERT INTO spaces (id, name, sort_key, created_by) VALUES ($1, 'Support', 'a0', $2)`, space.String(), alex.String())
	for i, ch := range []ids.ID{a, b, c} {
		e.exec(t, `INSERT INTO channels (id, space_id, kind, name, sort_key, history_backfill_limit, created_by)
			VALUES ($1, $2, 'forum', $3, 'a0', 20, $4)`, ch.String(), space.String(), "f"+string(rune('a'+i)), alex.String())
	}
	thread := func(ch ids.ID, last, floor int) ids.ID {
		id := ids.New()
		e.exec(t, `INSERT INTO threads (id, channel_id, kind, created_by, last_message_seq, unread_floor_seq) VALUES ($1, $2, 'thread', $3, $4, $5)`,
			id.String(), ch.String(), alex.String(), last, floor)
		return id
	}
	inA, inB := thread(a, 5, 2), thread(b, 3, 0)
	thread(a, 4, 4) // nothing unread for anybody
	thread(a, 0, 0) // no message
	thread(c, 9, 0) // channel not asked
	got, err := e.repo.ThreadCounters(ctx, []ids.ID{a, b})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("counters %+v", got)
	}
	want := map[ids.ID][3]int64{inA: {5, 2, 0}, inB: {3, 0, 0}}
	for _, tc := range got {
		w, ok := want[tc.ThreadID]
		if !ok || tc.LastMessageSeq != w[0] || tc.UnreadFloorSeq != w[1] {
			t.Errorf("counter %+v", tc)
		}
		if (tc.ThreadID == inA && tc.ChannelID != a) || (tc.ThreadID == inB && tc.ChannelID != b) {
			t.Errorf("channel of %+v", tc)
		}
	}
	if none, err := e.repo.ThreadCounters(ctx, nil); err != nil || len(none) != 0 {
		t.Fatalf("no channels: %+v %v", none, err)
	}
}
