// SPDX-License-Identifier: Apache-2.0

package migrations_test

import (
	"context"
	"testing"
	"time"

	"github.com/pressly/goose/v3"

	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
	"github.com/OWNER/bothy/services/server/migrations"
)

// beforeTeamHandle is the last version before messages.team_handle (00110).
const beforeTeamHandle = 109

// TestMigration_TeamHandle holds messages.team_handle (FIX-TEAM-MEMBER-ATTRIBUTION, 03 section
// 4.6): a nullable text column without a default; the up step fills it for the team copies written
// before it with the handle of the source member (a mirror through team_mirrors.member_id, a
// member's input through team_inputs.source_member_id) and leaves an owner's input, a mirror whose
// member is gone and every other message NULL. One agent that is two members gets the handle of
// the member each copy came from. Down drops the column; a second up changes nothing.
func TestMigration_TeamHandle(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := pgtest.New(t)
	pool := db.OpenPool(t, db.Config(pg.RoleOwner).ForMigrate(5*time.Second))
	sqlDB := pool.OpenSQL()
	t.Cleanup(func() { _ = sqlDB.Close() })
	provider, err := goose.NewProvider(goose.DialectPostgres, sqlDB, migrations.FS, goose.WithDisableGlobalRegistry(true))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.DownTo(ctx, beforeTeamHandle); err != nil {
		t.Fatalf("down to %d: %v", beforeTeamHandle, err)
	}
	app := db.Pool(t, pg.RoleApp)
	w := seedWakeWorld(t, app)
	const (
		home2   = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7822"
		thread2 = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7832"
		coding  = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7811"
		helper  = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7812"
		owner   = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7801"
		ownerIn = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7802"
		reply   = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7803"
		mirror  = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7804"
		input   = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7805"
		reply2  = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7806"
		mirror2 = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7807"
		orphan  = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7808"
	)
	mustExec(t, app, `INSERT INTO channels (id, space_id, kind, name, sort_key, history_backfill_limit, created_by)
		SELECT $1::uuid, space_id, 'text', 'second', 'a1', 20, created_by FROM channels WHERE id = $2`, home2, w.channel)
	mustExec(t, app, `INSERT INTO threads (id, channel_id, kind, created_by) VALUES ($1, $2, 'main', $3)`, thread2, home2, w.person)
	mustExec(t, app, `INSERT INTO team_channels (channel_id, sponsor_id) VALUES ($1, $2)`, w.channel, w.person)
	addMember := `INSERT INTO team_members (id, channel_id, agent_id, home_channel_id, work_thread_id, handle, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`
	mustExec(t, app, addMember, coding, w.channel, w.agent, w.channel, w.thread, "coding", w.person)
	mustExec(t, app, addMember, helper, w.channel, w.agent, home2, thread2, "helper", w.person)
	addMsg := `INSERT INTO messages (id, thread_id, channel_id, thread_seq, author_principal_id, client_msg_id,
		client_payload_sha256, origin, content_md) VALUES ($1, $2, $3, $4, $5, $1, '\x00', $6, 'x')`
	for i, m := range []struct{ id, thread, channel, author, origin string }{
		{owner, w.thread, w.channel, w.person, "human"}, {ownerIn, thread2, home2, w.person, "human"},
		{reply, w.thread, w.channel, w.agent, "agent"}, {mirror, w.thread, w.channel, w.agent, "agent"},
		{input, thread2, home2, w.agent, "agent"}, {reply2, thread2, home2, w.agent, "agent"},
		{mirror2, w.thread, w.channel, w.agent, "agent"}, {orphan, w.thread, w.channel, w.agent, "agent"},
	} {
		mustExec(t, app, addMsg, m.id, m.thread, m.channel, i+1, m.author, m.origin)
	}
	addInput := `INSERT INTO team_inputs (message_id, member_id, source_message_id, source, source_member_id, authority_id)
		VALUES ($1, $2, $3, $4, $5, $6)`
	mustExec(t, app, addInput, ownerIn, helper, owner, "owner", nil, w.person)
	mustExec(t, app, addInput, input, helper, mirror, "member", coding, w.person)
	addMirror := `INSERT INTO team_mirrors (source_message_id, mirror_message_id, member_id) VALUES ($1, $2, $3)`
	mustExec(t, app, addMirror, reply, mirror, coding)
	mustExec(t, app, addMirror, reply2, mirror2, helper)
	// A mirror whose member was removed since (ON DELETE SET NULL).
	mustExec(t, app, addMirror, owner, orphan, nil)

	if _, err := provider.UpTo(ctx, beforeTeamHandle+1); err != nil {
		t.Fatalf("up to %d: %v", beforeTeamHandle+1, err)
	}
	want := map[string]string{
		owner: "", ownerIn: "", reply: "", mirror: "coding", input: "coding", reply2: "", mirror2: "helper", orphan: "",
	}
	for id, handle := range want {
		var got string
		if err := app.QueryRow(ctx, `SELECT coalesce(team_handle, '') FROM messages WHERE id = $1`, id).Scan(&got); err != nil || got != handle {
			t.Errorf("team_handle of %s: %q %v, want %q", id, got, err, handle)
		}
	}
	var typ, dflt string
	var nullable bool
	if err := app.QueryRow(ctx, `SELECT data_type, is_nullable = 'YES', coalesce(column_default, '')
		FROM information_schema.columns WHERE table_name = 'messages' AND column_name = 'team_handle'`).Scan(&typ, &nullable, &dflt); err != nil {
		t.Fatal(err)
	}
	if typ != "text" || !nullable || dflt != "" {
		t.Errorf("team_handle: %s nullable %v default %q", typ, nullable, dflt)
	}
	if _, err := provider.DownTo(ctx, beforeTeamHandle); err != nil {
		t.Fatalf("down: %v", err)
	}
	var cols int
	if err := app.QueryRow(ctx, `SELECT count(*) FROM information_schema.columns WHERE table_name = 'messages' AND column_name = 'team_handle'`).
		Scan(&cols); err != nil || cols != 0 {
		t.Fatalf("team_handle after down: %d %v", cols, err)
	}
	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("up: %v", err)
	}
	if res, err := provider.Up(ctx); err != nil || len(res) != 0 {
		t.Fatalf("second up: %d results, %v", len(res), err)
	}
}
