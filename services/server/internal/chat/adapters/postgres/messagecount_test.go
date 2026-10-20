// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// TestThreads_DecrementMessageCountSQL (M1-58): the UPDATE lowers message_count by one and raises
// version by one; at zero and for an unknown thread it changes nothing and fails, outside a unit
// of work it refuses.
func TestThreads_DecrementMessageCountSQL(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ctx := context.Background()
	alex, _ := e.person(t)
	space, ch, th := ids.New(), ids.New(), ids.New()
	e.exec(t, `INSERT INTO spaces (id, name, sort_key, created_by) VALUES ($1, 'Support', 'a0', $2)`, space.String(), alex.String())
	e.exec(t, `INSERT INTO channels (id, space_id, kind, name, sort_key, history_backfill_limit, created_by)
		VALUES ($1, $2, 'forum', 'bugs', 'a0', 20, $3)`, ch.String(), space.String(), alex.String())
	e.exec(t, `INSERT INTO threads (id, channel_id, kind, created_by, message_count, last_message_seq, version)
		VALUES ($1, $2, 'thread', $3, 1, 3, 5)`, th.String(), ch.String(), alex.String())
	if err := e.repo.DecrementMessageCount(ctx, th); err == nil {
		t.Fatal("decrement outside a unit of work")
	}
	dec := func(id ids.ID) error {
		return e.write(t, func(ctx context.Context) error { return e.repo.DecrementMessageCount(ctx, id) })
	}
	if err := dec(th); err != nil {
		t.Fatal(err)
	}
	got, _, err := e.repo.Thread(ctx, th)
	if err != nil || got.MessageCount != 0 || got.Version != 6 || got.LastMessageSeq != 3 {
		t.Fatalf("thread %+v, %v", got, err)
	}
	for _, id := range []ids.ID{th, ids.New()} {
		if ee, ok := errs.As(dec(id)); !ok || ee.Code != errs.CodeInternalError {
			t.Fatalf("decrement %s: %v", id, ee)
		}
	}
	if got, _, _ := e.repo.Thread(ctx, th); got.MessageCount != 0 || got.Version != 6 {
		t.Fatalf("a failed decrement changed %+v", got)
	}
}
