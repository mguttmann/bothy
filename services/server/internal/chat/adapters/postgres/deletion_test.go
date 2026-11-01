// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/chat/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/searchkit"
)

// TestRepositoryDeletedThreadsLeaveTheLists (D-DEL-3): a deleted thread is absent from the active
// threads of the snapshot (also when pinned), the unread counters and candidates, the served
// targets of its fixed operator (I1), the title search and the thread filter of search; the live
// thread next to it stays.
func TestRepositoryDeletedThreadsLeaveTheLists(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ctx := context.Background()
	alex, _ := e.person(t)
	agent := ids.New()
	e.exec(t, `INSERT INTO principals (id, kind, handle, display_name) VALUES ($1, 'agent', 'lotse', 'Lotse')`, agent.String())
	e.exec(t, `INSERT INTO agents (principal_id, adapter_kind) VALUES ($1, 'custom')`, agent.String())
	space, forum := ids.New(), ids.New()
	e.exec(t, `INSERT INTO spaces (id, name, sort_key, created_by) VALUES ($1, 'Support', 'a0', $2)`, space.String(), alex.String())
	e.exec(t, `INSERT INTO channels (id, space_id, kind, name, sort_key, history_backfill_limit, created_by, operator_inherit)
		VALUES ($1, $2, 'forum', 'bugs', 'a0', 20, $3, false)`, forum.String(), space.String(), alex.String())
	thread := func(title string) ids.ID {
		id := ids.New()
		e.exec(t, `INSERT INTO threads (id, channel_id, kind, title, operator_agent_id, created_by, last_message_seq, last_activity_at)
			VALUES ($1, $2, 'thread', $3, $4, $5, 3, now())`, id.String(), forum.String(), title, agent.String(), alex.String())
		return id
	}
	live, gone := thread("invoice export"), thread("invoice import")
	e.exec(t, `UPDATE threads SET deleted_at = now(), deleted_by = $2 WHERE id = $1`, gone.String(), alex.String())

	active, err := e.repo.ActiveThreads(ctx, app.ActiveThreadQuery{ChannelIDs: []ids.ID{forum}, PinnedIDs: []ids.ID{gone},
		Since: time.Now().Add(-time.Hour), PerChannel: 10})
	if err != nil || len(active) != 1 || active[0].ID != live {
		t.Fatalf("active threads %+v, %v", active, err)
	}
	counters, err := e.repo.ThreadCounters(ctx, []ids.ID{forum})
	if err != nil || len(counters) != 1 || counters[0].ThreadID != live {
		t.Fatalf("counters %+v, %v", counters, err)
	}
	unread, err := e.repo.UnreadCandidates(ctx, []ids.ID{forum})
	if err != nil || len(unread) != 1 || unread[0].ThreadID != live {
		t.Fatalf("unread candidates %+v, %v", unread, err)
	}
	e.exec(t, `UPDATE threads SET deleted_at = now(), deleted_by = $2 WHERE id = $1`, live.String(), alex.String())
	served, err := e.repo.ServedTargets(ctx, agent)
	if err != nil || len(served) != 0 {
		t.Fatalf("served targets of an agent whose threads are deleted %+v, %v", served, err)
	}
	e.exec(t, `UPDATE threads SET deleted_at = NULL, deleted_by = NULL WHERE id = $1`, live.String())
	if served, err := e.repo.ServedTargets(ctx, agent); err != nil || !slices.ContainsFunc(served, func(s app.ServedTarget) bool { return s.ChannelID == forum }) {
		t.Fatalf("served targets %+v, %v", served, err)
	}
	hits, err := e.repo.PrefixThreadTitles(ctx, searchkit.PrefixQuery{TSQuery: "invoice:*", ChannelIDs: []ids.ID{forum}, Limit: 10})
	if err != nil || len(hits) != 1 || hits[0].ThreadID != live {
		t.Fatalf("title hits %+v, %v", hits, err)
	}
	matching, err := e.repo.ThreadsMatching(ctx, []ids.ID{forum}, searchkit.ThreadFilter{})
	if err != nil || !slices.Equal(matching, []ids.ID{live}) {
		t.Fatalf("matching %v, %v", matching, err)
	}
}

// TestRepositoryFinishPurge (D-DEL-1): finishing the purge of a channel clears its topic and the
// titles of its threads and sets purged_at once; a second call changes nothing; DuePurges lists
// only items deleted before the given time and not purged.
func TestRepositoryFinishPurge(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ctx := context.Background()
	alex, _ := e.person(t)
	space, ch, th := ids.New(), ids.New(), ids.New()
	e.exec(t, `INSERT INTO spaces (id, name, sort_key, created_by) VALUES ($1, 'Support', 'a0', $2)`, space.String(), alex.String())
	e.exec(t, `INSERT INTO channels (id, space_id, kind, name, topic, sort_key, history_backfill_limit, created_by, archived_at, deleted_at, deleted_by)
		VALUES ($1, $2, 'forum', 'bugs', 'secret topic', 'a0', 20, $3, $4, $4, $3)`, ch.String(), space.String(), alex.String(), t0)
	e.exec(t, `INSERT INTO threads (id, channel_id, kind, title, title_source, created_by) VALUES ($1, $2, 'thread', 'secret title', 'manual', $3)`,
		th.String(), ch.String(), alex.String())

	if due, err := e.repo.DuePurges(ctx, t0, 10); err != nil || len(due) != 0 {
		t.Fatalf("due before the deletion time %+v, %v", due, err)
	}
	due, err := e.repo.DuePurges(ctx, t0.Add(time.Minute), 10)
	if err != nil || len(due) != 1 || due[0] != (app.PurgeItem{Kind: chat.TargetChannel, ID: ch}) {
		t.Fatalf("due %+v, %v", due, err)
	}
	for range 2 {
		if err := e.write(t, func(ctx context.Context) error {
			return e.repo.FinishPurge(ctx, chat.TargetChannel, ch, t0.Add(time.Minute))
		}); err != nil {
			t.Fatal(err)
		}
	}
	var topic, title, source string
	var version int64
	if err := e.pool.QueryRow(ctx, `SELECT c.topic, t.title, t.title_source, t.version FROM channels c JOIN threads t ON t.channel_id = c.id
		WHERE c.id = $1 AND c.purged_at IS NOT NULL`, ch.String()).Scan(&topic, &title, &source, &version); err != nil {
		t.Fatal(err)
	}
	if topic != "" || title != "" || source != "manual" || version != 2 {
		t.Fatalf("after the purge: topic %q title %q source %s version %d", topic, title, source, version)
	}
	if due, err := e.repo.DuePurges(ctx, t0.Add(time.Hour), 10); err != nil || len(due) != 0 {
		t.Fatalf("due after the purge %+v, %v", due, err)
	}
}
