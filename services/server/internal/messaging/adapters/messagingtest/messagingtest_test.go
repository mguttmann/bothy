// SPDX-License-Identifier: Apache-2.0

package messagingtest_test

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/messaging/adapters/messagingtest"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
)

func TestMain(m *testing.M) { os.Exit(pgtest.Main(m)) }

// TestMessagingtest_NotInBinary: the server binary never depends on the test helpers.
func TestMessagingtest_NotInBinary(t *testing.T) {
	t.Parallel()
	out, err := exec.CommandContext(t.Context(), "go", "list", "-deps", "../../../../cmd/bothy").CombinedOutput()
	if err != nil {
		t.Fatalf("go list: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "/internal/messaging/adapters/postgres") {
		t.Fatalf("go list output does not look like the dependencies of cmd/bothy:\n%.400s", out)
	}
	if strings.Contains(string(out), "/internal/messaging/adapters/messagingtest") {
		t.Fatal("cmd/bothy depends on internal/messaging/adapters/messagingtest")
	}
}

// TestInsertImportedMessage stores two imported messages after one ordinary row and reads them
// back: origin import, kind text, silent, no trigger_state, the next thread_seq each, the thread's
// counters raised; an unknown thread fails the test.
func TestInsertImportedMessage(t *testing.T) {
	t.Parallel()
	db := pgtest.New(t)
	owner := db.Pool(t, pg.RoleOwner)
	pool := db.Pool(t, pg.RoleApp)
	ctx := context.Background()
	sam, space, channel, thread := ids.New(), ids.New(), ids.New(), ids.New()
	for _, stmt := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO principals (id, kind, handle, display_name) VALUES ($1, 'user', 'sam', 'Sam')`, []any{sam.String()}},
		{`INSERT INTO users (principal_id) VALUES ($1)`, []any{sam.String()}},
		{`INSERT INTO spaces (id, name, sort_key, created_by) VALUES ($1, 'Support', 'a0', $2)`, []any{space.String(), sam.String()}},
		{`INSERT INTO channels (id, space_id, kind, name, sort_key, history_backfill_limit, created_by)
			VALUES ($1, $2, 'text', 'general', 'a0', 20, $3)`, []any{channel.String(), space.String(), sam.String()}},
		{`INSERT INTO threads (id, channel_id, kind, created_by) VALUES ($1, $2, 'main', $3)`, []any{thread.String(), channel.String(), sam.String()}},
	} {
		if _, err := owner.Exec(ctx, stmt.sql, stmt.args...); err != nil {
			t.Fatal(err)
		}
	}
	first := messagingtest.InsertImportedMessage(t, pool, messagingtest.ImportedMessage{ThreadID: thread, AuthorID: sam, Content: "Imported note"})
	second := messagingtest.InsertImportedMessage(t, owner, messagingtest.ImportedMessage{ThreadID: thread, AuthorID: sam, Content: "Second note"})
	for i, id := range []ids.ID{first, second} {
		var origin, kind, content string
		var seq int64
		var silent, hasTrigger bool
		if err := owner.QueryRow(ctx, `SELECT origin, kind, content_md, thread_seq, silent, trigger_state IS NOT NULL FROM messages WHERE id = $1`,
			id.String()).Scan(&origin, &kind, &content, &seq, &silent, &hasTrigger); err != nil {
			t.Fatal(err)
		}
		if origin != "import" || kind != "text" || !silent || hasTrigger || seq != int64(i+1) {
			t.Errorf("message %d: origin %s kind %s silent %v trigger %v seq %d", i+1, origin, kind, silent, hasTrigger, seq)
		}
	}
	var last, count int64
	if err := owner.QueryRow(ctx, `SELECT last_message_seq, message_count FROM threads WHERE id = $1`, thread.String()).Scan(&last, &count); err != nil {
		t.Fatal(err)
	}
	if last != 2 || count != 2 {
		t.Fatalf("thread counters %d %d", last, count)
	}
	ft := &fatalRecorder{TB: t}
	func() {
		defer func() { _ = recover() }()
		messagingtest.InsertImportedMessage(ft, pool, messagingtest.ImportedMessage{ThreadID: ids.New(), AuthorID: sam, Content: "lost"})
	}()
	if !ft.failed {
		t.Fatal("an unknown thread did not fail the test")
	}
}

// fatalRecorder records Fatalf instead of ending the test.
type fatalRecorder struct {
	testing.TB
	failed bool
}

func (f *fatalRecorder) Helper() {}

func (f *fatalRecorder) Fatalf(string, ...any) {
	f.failed = true
	panic("fatal")
}
