// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"encoding/json"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/chat/adapters/postgres"
	"github.com/OWNER/bothy/services/server/internal/chat/app"
	"github.com/OWNER/bothy/services/server/internal/chat/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
)

// activeThreadsSQL returns the statement ActiveThreads of queries/threads.sql with its named
// arguments numbered as args lists them, so EXPLAIN runs exactly the text sqlc generates from.
func activeThreadsSQL(t *testing.T, args []string) string {
	t.Helper()
	b, err := os.ReadFile("queries/threads.sql")
	if err != nil {
		t.Fatal(err)
	}
	text := string(b)
	start := strings.Index(text, "-- name: ActiveThreads :many")
	if start < 0 {
		t.Fatal("no query ActiveThreads in queries/threads.sql")
	}
	text = text[start+len("-- name: ActiveThreads :many"):]
	if end := strings.Index(text, "-- name:"); end >= 0 {
		text = text[:end]
	}
	var lines []string
	for _, l := range strings.Split(text, "\n") {
		if !strings.HasPrefix(strings.TrimSpace(l), "--") {
			lines = append(lines, l)
		}
	}
	text = strings.TrimSuffix(strings.TrimSpace(strings.Join(lines, "\n")), ";")
	return regexp.MustCompile(`sqlc\.arg\((\w+)\)`).ReplaceAllStringFunc(text, func(m string) string {
		name := m[len("sqlc.arg(") : len(m)-1]
		i := slices.Index(args, name)
		if i < 0 {
			t.Fatalf("argument %s not listed", name)
		}
		return "$" + strconv.Itoa(i+1)
	})
}

// planNodes walks an EXPLAIN (FORMAT JSON) plan and returns every node.
func planNodes(n map[string]any) []map[string]any {
	out := []map[string]any{n}
	children, _ := n["Plans"].([]any)
	for _, c := range children {
		if m, ok := c.(map[string]any); ok {
			out = append(out, planNodes(m)...)
		}
	}
	return out
}

// TestActiveThreads_QueryPlan (M2-52 design note 1): on a seeded database (eight forums with
// four thousand threads each, activity spread over two months, some done) the statement of the
// active threads reads threads only through indexes, among them
// threads_channel_state_activity_idx for the LATERAL read per channel, never by a sequential scan,
// and returns the bound per channel plus the pinned threads.
func TestActiveThreads_QueryPlan(t *testing.T) {
	t.Parallel()
	db := pgtest.New(t)
	owner := db.Pool(t, pg.RoleOwner)
	ctx := context.Background()
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := owner.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
	}
	alex, space := ids.New(), ids.New()
	exec(`INSERT INTO principals (id, kind, handle, display_name) VALUES ($1, 'user', 'alex', 'alex')`, alex.String())
	exec(`INSERT INTO users (principal_id) VALUES ($1)`, alex.String())
	exec(`INSERT INTO spaces (id, name, sort_key, created_by) VALUES ($1, 'Support', 'a0', $2)`, space.String(), alex.String())
	const forums, perForum, bound = 8, 4000, 20
	var channels []ids.ID
	for i := range forums {
		ch := ids.New()
		channels = append(channels, ch)
		exec(`INSERT INTO channels (id, space_id, kind, name, sort_key, history_backfill_limit, created_by)
			VALUES ($1, $2, 'forum', $3, $4, 20, $5)`, ch.String(), space.String(), "forum"+strconv.Itoa(i), "a"+strconv.Itoa(i), alex.String())
		// Every 25th thread is done; activity steps back 24 minutes per thread from t0, so a
		// forum spans about 66 days and about 420 threads per forum lie within the window.
		exec(`INSERT INTO threads (id, channel_id, kind, title, state, closed_at, closed_by, last_activity_at, created_by, created_at)
			SELECT gen_random_uuid(), $1, 'thread', 'post ' || g,
			       CASE WHEN g % 25 = 0 THEN 'closed' ELSE 'open' END,
			       CASE WHEN g % 25 = 0 THEN $2::timestamptz END,
			       CASE WHEN g % 25 = 0 THEN $3::uuid END,
			       $2::timestamptz - g * interval '1 minute' * 24, $3, $2::timestamptz - g * interval '1 minute' * 24
			FROM generate_series(1, $4::int) AS g`, ch.String(), t0, alex.String(), perForum)
	}
	exec(`ANALYZE threads`)
	// Three old open threads of the first forum are pinned (an active run or unread).
	var pinned []ids.ID
	rows, err := owner.Query(ctx, `SELECT id FROM threads WHERE channel_id = $1 AND state = 'open' ORDER BY last_activity_at LIMIT 3`, channels[0].String())
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var id ids.ID
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		pinned = append(pinned, id)
	}
	rows.Close()
	if len(pinned) != 3 {
		t.Fatalf("pinned %v", pinned)
	}
	since := t0.Add(-7 * 24 * time.Hour)
	order := []string{"channel_ids", "pinned_ids", "since", "per_channel"}
	sql := activeThreadsSQL(t, order)
	args := []any{idStrings(channels), idStrings(pinned), since, bound}

	var raw []byte
	if err := owner.QueryRow(ctx, "EXPLAIN (FORMAT JSON) "+sql, args...).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var plan []struct {
		Plan map[string]any `json:"Plan"`
	}
	if err := json.Unmarshal(raw, &plan); err != nil || len(plan) != 1 {
		t.Fatalf("plan %s: %v", raw, err)
	}
	usedActivity := false
	for _, n := range planNodes(plan[0].Plan) {
		if n["Relation Name"] == "threads" && n["Node Type"] == "Seq Scan" {
			t.Fatalf("sequential scan of threads in the plan:\n%s", raw)
		}
		if n["Index Name"] == "threads_channel_state_activity_idx" {
			usedActivity = true
		}
	}
	if !usedActivity {
		t.Fatalf("threads_channel_state_activity_idx unused:\n%s", raw)
	}
	t.Logf("plan of ActiveThreads on %d threads: %s", forums*perForum, raw)

	repo := postgres.NewRepository(db.Pool(t, pg.RoleApp))
	got, err := repo.ActiveThreads(ctx, app.ActiveThreadQuery{ChannelIDs: channels, PinnedIDs: pinned, Since: since, PerChannel: bound})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != forums*bound {
		t.Fatalf("%d active threads, want %d (the bound per forum, the pinned ones counted)", len(got), forums*bound)
	}
	for i, th := range got {
		if !slices.Contains(channels, th.ChannelID) || th.State != "open" {
			t.Fatalf("thread %d: %+v", i, th)
		}
	}
	for _, p := range pinned {
		if !slices.ContainsFunc(got, func(th domain.Thread) bool { return th.ID == p }) {
			t.Fatalf("pinned thread %s missing", p)
		}
	}
}

// idStrings converts ids for an array parameter of a raw statement.
func idStrings(list []ids.ID) []string {
	out := make([]string, 0, len(list))
	for _, id := range list {
		out = append(out, id.String())
	}
	return out
}
