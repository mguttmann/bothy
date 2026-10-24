// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/files"
	"github.com/OWNER/bothy/services/server/internal/files/adapters/postgres"
	"github.com/OWNER/bothy/services/server/internal/files/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
)

func TestMain(m *testing.M) { os.Exit(pgtest.Main(m)) }

// The stubs of the collaborators app.New needs and the attachment use cases never call.
type (
	allow     struct{}
	noEvents  struct{}
	oneSpace  struct{ space ids.ID }
	asIs      struct{}
	noTypes   struct{}
	noQueue   struct{}
	noStorage struct{ app.ObjectStore }
)

func (allow) Authorize(context.Context, authorize.Principal, authorize.Action, authorize.Resource) (authorize.Decision, error) {
	return authorize.Decision{Outcome: authorize.OutcomeAllow}, nil
}
func (allow) Scope(context.Context, authorize.Principal, authorize.Action) (authorize.Scope, error) {
	return authorize.Scope{}, nil
}
func (allow) Observe(context.Context, authorize.LogEvent)                   {}
func (noEvents) Append(context.Context, ...eventlog.Event) error            { return nil }
func (o oneSpace) SpaceOf(context.Context, ids.ID) (ids.ID, error)          { return o.space, nil }
func (asIs) NFC(s string) string                                            { return s }
func (noTypes) Extensions(string) []string                                  { return nil }
func (noQueue) EnqueueScan(context.Context, ids.ID, int64, time.Time) error { return nil }
func (noQueue) EnqueueThumbnail(context.Context, ids.ID) error              { return nil }

// Enqueue implements app.PurgeQueue: files.purge_objects is not run here (M3-08).
func (noQueue) Enqueue(context.Context, ids.ID) error { return nil }

// noRedact is the redactor of these tests: the event log is not under test here (M3-08).
type noRedact struct{}

func (noRedact) Redact(context.Context, string, ids.ID, eventlog.RedactFunc) (int, error) {
	return 0, nil
}

// world is a database with the place of the attachment tests: Alex and Sam, the space Support with
// the channels Allgemein and Eskalation, a thread of Allgemein and two messages of Alex in it.
type world struct {
	t          *testing.T
	conn       *pgx.Conn
	repo       *postgres.Repository
	uow        *pg.UnitOfWork
	svc        *app.Service
	clk        *clock.Fake
	alex, sam  ids.ID
	space      ids.ID
	allgemein  ids.ID
	eskalation ids.ID
	thread     ids.ID
	m1, m2     ids.ID
	// seq is the last thread_seq of the thread.
	seq int
}

func newWorld(t *testing.T) *world {
	t.Helper()
	db := pgtest.New(t)
	pool := db.Pool(t, pg.RoleApp)
	reg := txn.NewRegistry()
	reg.Seal()
	w := &world{t: t, conn: db.SuperConn(t), repo: postgres.NewRepository(pool),
		uow:  pg.NewUnitOfWork(pool, pg.UnitConfig{StatementTimeout: 30 * time.Second, LockTimeout: 10 * time.Second, IdleInTxTimeout: time.Minute}, reg),
		clk:  clock.NewFake(time.Date(2026, 10, 21, 9, 0, 0, 0, time.UTC)),
		alex: ids.New(), sam: ids.New(), space: ids.New(), allgemein: ids.New(), eskalation: ids.New(), thread: ids.New(), m1: ids.New(), m2: ids.New()}
	svc, err := app.New(app.Config{MaxUploadBytes: 64, PartBytes: 4, SessionTTL: time.Hour, MaxOpenSessions: 3, NameMaxBytes: 64},
		app.Deps{Repo: w.repo, Parts: app.NewFSPartStore(noStorage{}), UoW: w.uow, Authorizer: allow{}, Events: noEvents{},
			Channels: oneSpace{w.space}, Names: asIs{}, Types: noTypes{}, ScanQueue: noQueue{}, Purge: noQueue{},
			Redactor: noRedact{}, Cleanup: w.repo, Clock: w.clk})
	if err != nil {
		t.Fatal(err)
	}
	w.svc = svc
	for _, p := range []struct {
		id     ids.ID
		handle string
	}{{w.alex, "alex"}, {w.sam, "sam"}} {
		w.exec(`INSERT INTO principals (id, kind, status, handle, display_name) VALUES ($1, 'user', 'active', $2, $2)`, p.id, p.handle)
	}
	w.exec(`INSERT INTO spaces (id, name, sort_key, created_by) VALUES ($1, 'Support', 'a', $2)`, w.space, w.alex)
	for i, ch := range []ids.ID{w.allgemein, w.eskalation} {
		w.exec(`INSERT INTO channels (id, space_id, kind, name, sort_key, history_backfill_limit, created_by)
			VALUES ($1, $2, 'text', $3, $3, 20, $4)`, ch, w.space, []string{"Allgemein", "Eskalation"}[i], w.alex)
	}
	w.exec(`INSERT INTO threads (id, channel_id, kind, created_by) VALUES ($1, $2, 'thread', $3)`, w.thread, w.allgemein, w.alex)
	for i, m := range []ids.ID{w.m1, w.m2} {
		w.exec(`INSERT INTO messages (id, thread_id, channel_id, thread_seq, author_principal_id, client_msg_id, client_payload_sha256, origin, content_md)
			VALUES ($1, $2, $3, $4, $5, $6, '\x00', 'human', 'hello')`, m, w.thread, w.allgemein, i+1, w.alex, ids.New())
	}
	w.seq = 2
	return w
}

// message inserts another message of Alex into the thread and returns it.
func (w *world) message() ids.ID {
	w.t.Helper()
	w.seq++
	m := ids.New()
	w.exec(`INSERT INTO messages (id, thread_id, channel_id, thread_seq, author_principal_id, client_msg_id, client_payload_sha256, origin, content_md)
		VALUES ($1, $2, $3, $4, $5, $6, '\x00', 'human', 'hello')`, m, w.thread, w.allgemein, w.seq, w.alex, ids.New())
	return m
}

// exec runs a fixture statement as the superuser; ids.ID arguments are passed as text.
func (w *world) exec(stmt string, args ...any) {
	w.t.Helper()
	for i, a := range args {
		if id, ok := a.(ids.ID); ok {
			args[i] = id.String()
		}
	}
	if _, err := w.conn.Exec(context.Background(), stmt, args...); err != nil {
		w.t.Fatalf("%s: %v", stmt, err)
	}
}

// file inserts a file of uploader for channel (zero: no scope channel) with purpose, session state
// (empty: no session), upload state and scan status, and returns its id.
func (w *world) file(uploader, channel ids.ID, purpose, session, upload, scan string) ids.ID {
	w.t.Helper()
	id := ids.New()
	var scope any
	if !channel.IsZero() {
		scope = channel.String()
	}
	w.exec(`INSERT INTO files (id, object_key, sha256, size_bytes, mime_type, original_name, uploaded_by, purpose, scope_channel_id,
		upload_state, scan_status) VALUES ($1, $2, '\x01', 10, 'text/plain', 'notes.txt', $3, $4, $5, $6, $7)`,
		id, "f/"+id.String(), uploader, purpose, scope, upload, scan)
	if session != "" {
		w.exec(`INSERT INTO file_upload_sessions (file_id, state, declared_size_bytes, declared_sha256, part_size_bytes, part_count, expires_at)
			VALUES ($1, $2, 10, '\x01', 4, 3, now() + interval '1 hour')`, id, session)
	}
	return id
}

// attachable inserts a completed, pending file of Alex for Allgemein.
func (w *world) attachable() ids.ID {
	return w.file(w.alex, w.allgemein, "attachment", "completed", "pending", "pending")
}

// attach runs Attach of fileIDs to message m of the thread in Allgemein as Alex in its own unit.
func (w *world) attach(m ids.ID, fileIDs ...ids.ID) ([]files.Attachment, error) {
	var out []files.Attachment
	err := w.uow.Do(context.Background(), func(ctx context.Context) error {
		var err error
		out, err = w.svc.Attach(ctx, files.AttachCmd{MessageID: m, ThreadID: w.thread, ChannelID: w.allgemein, AuthorID: w.alex,
			FileIDs: fileIDs, Pointer: "/first_message/attachment_file_ids"})
		return err
	})
	return out, err
}

// count returns the integer result of query.
func (w *world) count(query string, args ...any) int {
	w.t.Helper()
	for i, a := range args {
		if id, ok := a.(ids.ID); ok {
			args[i] = id.String()
		}
	}
	var n int
	if err := w.conn.QueryRow(context.Background(), query, args...).Scan(&n); err != nil {
		w.t.Fatalf("%s: %v", query, err)
	}
	return n
}

// TestRepository_AttachRows holds brief M3-07 design note 4 against PostgreSQL: the rows in request
// order with position = index + 1 and the copies of the command, the attached form of the result
// with the thumbnails of the batch read; ForMessages, MessageOf and Pending read them back.
func TestRepository_AttachRows(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	f1, f2 := w.attachable(), w.file(w.alex, w.allgemein, "attachment", "completed", "complete", "clean")
	w.exec(`INSERT INTO file_thumbnails (file_id, variant, object_key, mime_type, width, height, size_bytes)
		VALUES ($1, 'small', $2, 'image/jpeg', 2, 1, 3)`, f2, "t/"+f2.String())
	got, err := w.attach(w.m1, f2, f1)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].File.ID != f2 || got[1].File.ID != f1 || got[0].Position != 1 || got[1].Position != 2 ||
		!got[0].File.ScopeChannelID.IsZero() || got[0].File.SHA256 == "" || got[1].File.SHA256 != "" || len(got[0].File.Thumbnails) != 1 {
		t.Fatalf("Attach = %+v", got)
	}
	if n := w.count(`SELECT count(*) FROM message_attachments WHERE message_id = $1 AND thread_id = $2 AND channel_id = $3`,
		w.m1, w.thread, w.allgemein); n != 2 {
		t.Fatalf("%d rows with the copies", n)
	}
	if n := w.count(`SELECT count(*) FROM message_attachments WHERE file_id = $1 AND position = 1`, f2); n != 1 {
		t.Fatal("position of the first entry")
	}
	ctx := context.Background()
	list, err := w.svc.ForMessages(ctx, []ids.ID{w.m1, w.m2})
	if err != nil || len(list) != 1 || len(list[w.m1]) != 2 || list[w.m1][0].File.ID != f2 || len(list[w.m1][0].File.Thumbnails) != 1 ||
		list[w.m1][0].File.Thumbnails[0].Variant != "small" || !list[w.m1][1].File.ScopeChannelID.IsZero() {
		t.Fatalf("ForMessages = %+v, %v", list, err)
	}
	ref, ok, err := w.svc.MessageOf(ctx, f1)
	if err != nil || !ok || ref != (files.AttachmentRef{MessageID: w.m1, ThreadID: w.thread, ChannelID: w.allgemein, Position: 2}) {
		t.Fatalf("MessageOf = %+v %v %v", ref, ok, err)
	}
	if _, ok, err := w.svc.MessageOf(ctx, w.attachable()); ok || err != nil {
		t.Fatalf("MessageOf of an unattached file: %v %v", ok, err)
	}
	reason := func() files.PendingReason {
		var p files.PendingReason
		if err := w.uow.Do(ctx, func(ctx context.Context) error {
			var err error
			p, err = w.svc.Pending(ctx, w.m1)
			return err
		}); err != nil {
			t.Fatal(err)
		}
		return p
	}
	pending := func() bool { return reason() != files.PendingNone }
	if !pending() {
		t.Error("a pending attachment: Pending false")
	}
	w.exec(`UPDATE files SET upload_state = 'complete', scan_status = 'infected' WHERE id = $1`, f1)
	if pending() {
		t.Error("an infected and a clean attachment: Pending true")
	}
	// M3-09 design note 3: the transcript status is read with the states.
	w.exec(`UPDATE files SET transcript_status = 'pending' WHERE id = $1`, f1)
	if got := reason(); got != files.PendingTranscript {
		t.Errorf("a pending transcript: %v, want PendingTranscript", got)
	}
}

// TestRepository_AttachRefusals holds 12 10 point 1 and SEC-UP-9 at the use case over PostgreSQL:
// every cause answers the same 409 conflict file_not_attachable with the entry in detail, and
// neither a row nor a lock stays.
func TestRepository_AttachRefusals(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	ok := w.attachable()
	elsewhere := w.attachable()
	if _, err := w.attach(w.m2, elsewhere); err != nil {
		t.Fatal(err)
	}
	deleted := w.attachable()
	w.exec(`UPDATE files SET deleted_at = now() WHERE id = $1`, deleted)
	for _, c := range []struct {
		name  string
		file  ids.ID
		index string
	}{
		{"unknown id", ids.New(), "1"},
		{"foreign uploader", w.file(w.sam, w.allgemein, "attachment", "completed", "complete", "clean"), "1"},
		{"other channel (SEC-UP-9)", w.file(w.alex, w.eskalation, "attachment", "completed", "complete", "clean"), "1"},
		{"open session", w.file(w.alex, w.allgemein, "attachment", "open", "pending", "pending"), "1"},
		{"aborted", w.file(w.alex, w.allgemein, "attachment", "aborted", "failed", "pending"), "1"},
		{"expired", w.file(w.alex, w.allgemein, "attachment", "expired", "pending", "pending"), "1"},
		{"checksum failure", w.file(w.alex, w.allgemein, "attachment", "completed", "failed", "pending"), "1"},
		{"purpose avatar", w.file(w.alex, ids.ID{}, "avatar", "completed", "complete", "clean"), "1"},
		{"deleted", deleted, "1"},
		{"attached to another message", elsewhere, "1"},
		{"the same id twice", ok, "1"},
	} {
		_, err := w.attach(w.m1, ok, c.file)
		e, isErr := errs.As(err)
		if !isErr || e.Code != errs.CodeConflict || e.Reason != "file_not_attachable" || e.Detail != "/first_message/attachment_file_ids/"+c.index ||
			len(e.Headers) != 0 || e.RetryAfter != 0 {
			t.Errorf("%s: %v", c.name, err)
		}
	}
	if n := w.count(`SELECT count(*) FROM message_attachments WHERE message_id = $1`, w.m1); n != 0 {
		t.Errorf("%d rows after the refusals", n)
	}
	if _, err := w.attach(w.m1, ok); err != nil {
		t.Fatalf("the attachable file after the refusals: %v", err)
	}
}

// TestRepository_AttachConcurrentSameFile holds design note 4 a and d: two attaches of the same file
// to two messages at the same time: exactly one wins, the other answers the 409 and leaves no row.
func TestRepository_AttachConcurrentSameFile(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	for range 5 {
		f := w.attachable()
		var wg sync.WaitGroup
		results := make([]error, 2)
		for i, m := range []ids.ID{w.message(), w.message()} {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, results[i] = w.attach(m, f)
			}()
		}
		wg.Wait()
		wins := 0
		for _, err := range results {
			switch e, ok := errs.As(err); {
			case err == nil:
				wins++
			case !ok || e.Code != errs.CodeConflict || e.Reason != "file_not_attachable":
				t.Fatalf("loser: %v (cause %v)", err, errors.Unwrap(err))
			}
		}
		if wins != 1 || w.count(`SELECT count(*) FROM message_attachments WHERE file_id = $1`, f) != 1 {
			t.Fatalf("%d winners", wins)
		}
	}
}

// TestRepository_AttachUniqueViolation holds design note 4 d: a row that appears between the read
// of message_attachments and the insert (here a fixture that bypasses the lock) maps the unique
// violation on file_id to app.ErrAlreadyAttached.
func TestRepository_AttachUniqueViolation(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	f := w.attachable()
	w.exec(`INSERT INTO message_attachments (message_id, file_id, position, thread_id, channel_id) VALUES ($1, $2, 1, $3, $4)`,
		w.m2, f, w.thread, w.allgemein)
	err := w.uow.Do(context.Background(), func(ctx context.Context) error {
		return w.repo.InsertAttachments(ctx, []app.AttachmentRecord{{MessageID: w.m1, FileID: f, ThreadID: w.thread, ChannelID: w.allgemein, Position: 1}})
	})
	if err != app.ErrAlreadyAttached { //nolint:errorlint // the repository returns the sentinel itself
		t.Fatalf("InsertAttachments = %v, want ErrAlreadyAttached", err)
	}
	if err := w.repo.InsertAttachments(context.Background(), nil); err == nil {
		t.Fatal("InsertAttachments outside a unit of work")
	}
}

// TestRepository_DeleteForMessage holds design note 7 b: in one unit the rows of the message go
// and its files get deleted_at at the clock time; the attachments of another message stay; a
// second call writes nothing.
func TestRepository_DeleteForMessage(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	f1, f2, keep := w.attachable(), w.attachable(), w.attachable()
	if _, err := w.attach(w.m1, f1, f2); err != nil {
		t.Fatal(err)
	}
	if _, err := w.attach(w.m2, keep); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	del := func() error {
		return w.uow.Do(ctx, func(ctx context.Context) error { return w.svc.DeleteForMessage(ctx, w.m1) })
	}
	if err := del(); err != nil {
		t.Fatal(err)
	}
	if n := w.count(`SELECT count(*) FROM message_attachments WHERE message_id = $1`, w.m1); n != 0 {
		t.Errorf("%d rows of the deleted message", n)
	}
	at := w.clk.Now()
	if n := w.count(`SELECT count(*) FROM files WHERE id = ANY($1::uuid[]) AND deleted_at = $2`, []string{f1.String(), f2.String()}, at); n != 2 {
		t.Errorf("%d files with deleted_at at the clock time", n)
	}
	if n := w.count(`SELECT count(*) FROM files f JOIN message_attachments a ON a.file_id = f.id WHERE f.id = $1 AND f.deleted_at IS NULL`, keep); n != 1 {
		t.Error("the attachment of the other message changed")
	}
	w.clk.Advance(time.Hour)
	if err := del(); err != nil {
		t.Fatal(err)
	}
	if n := w.count(`SELECT count(*) FROM files WHERE id = $1 AND deleted_at = $2`, f1, at); n != 1 {
		t.Error("a second delete wrote deleted_at again")
	}
	if _, found, err := w.repo.File(ctx, f1); found || err != nil {
		t.Errorf("File of a deleted attachment: %v %v", found, err)
	}
}

// TestRepository_MarkFilesDeleted holds the one deleted_at write: purpose and uploader limit it
// when set (M3-10), a deleted file is not marked again.
func TestRepository_MarkFilesDeleted(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	avatar := w.file(w.alex, ids.ID{}, "avatar", "completed", "complete", "clean")
	ctx := context.Background()
	mark := func(d app.Deletion) int {
		var n int
		if err := w.uow.Do(ctx, func(ctx context.Context) error {
			marked, err := w.repo.MarkFilesDeleted(ctx, d, w.clk.Now())
			n = len(marked)
			return err
		}); err != nil {
			t.Fatal(err)
		}
		return n
	}
	if n := mark(app.Deletion{FileIDs: []ids.ID{avatar}, Purpose: "branding"}); n != 0 {
		t.Error("another purpose marked")
	}
	if n := mark(app.Deletion{FileIDs: []ids.ID{avatar}, Purpose: "avatar", UploadedBy: w.sam}); n != 0 {
		t.Error("another uploader marked")
	}
	if n := mark(app.Deletion{FileIDs: []ids.ID{avatar}, Purpose: "avatar", UploadedBy: w.alex}); n != 1 {
		t.Error("not marked")
	}
	if n := mark(app.Deletion{FileIDs: []ids.ID{avatar}}); n != 0 {
		t.Error("marked twice")
	}
}

// TestRepository_OnThreadMoved holds design note 8: the subscriber built over a repository
// without a pool moves the copy channel_id of every row of the thread in the transaction of the
// move; rows of other threads and files.scope_channel_id stay; a rollback keeps the copies.
func TestRepository_OnThreadMoved(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	f1, f2 := w.attachable(), w.attachable()
	if _, err := w.attach(w.m1, f1); err != nil {
		t.Fatal(err)
	}
	if _, err := w.attach(w.m2, f2); err != nil {
		t.Fatal(err)
	}
	other := ids.New()
	w.exec(`INSERT INTO threads (id, channel_id, kind, created_by) VALUES ($1, $2, 'thread', $3)`, other, w.allgemein, w.alex)
	m3, f3 := ids.New(), w.attachable()
	w.exec(`INSERT INTO messages (id, thread_id, channel_id, thread_seq, author_principal_id, client_msg_id, client_payload_sha256, origin, content_md)
		VALUES ($1, $2, $3, 1, $4, $5, '\x00', 'human', 'hello')`, m3, other, w.allgemein, w.alex, ids.New())
	w.exec(`INSERT INTO message_attachments (message_id, file_id, position, thread_id, channel_id) VALUES ($1, $2, 1, $3, $4)`,
		m3, f3, other, w.allgemein)
	subs, err := app.NewSubscribers(postgres.NewRepository(nil))
	if err != nil {
		t.Fatal(err)
	}
	ev := chat.ThreadMoved{ThreadID: w.thread, FromChannelID: w.allgemein, ToChannelID: w.eskalation}
	ctx := context.Background()
	rollback := context.Canceled
	if err := w.uow.Do(ctx, func(ctx context.Context) error {
		if err := subs.OnThreadMoved(ctx, ev); err != nil {
			return err
		}
		return rollback
	}); err != rollback { //nolint:errorlint // the unit returns the error of fn unchanged
		t.Fatalf("vetoed move: %v", err)
	}
	if n := w.count(`SELECT count(*) FROM message_attachments WHERE thread_id = $1 AND channel_id = $2`, w.thread, w.allgemein); n != 2 {
		t.Fatalf("a rolled back move changed the copies: %d rows left", n)
	}
	if err := w.uow.Do(ctx, func(ctx context.Context) error { return subs.OnThreadMoved(ctx, ev) }); err != nil {
		t.Fatal(err)
	}
	if n := w.count(`SELECT count(*) FROM message_attachments WHERE thread_id = $1 AND channel_id = $2`, w.thread, w.eskalation); n != 2 {
		t.Errorf("%d rows moved, want 2", n)
	}
	if n := w.count(`SELECT count(*) FROM message_attachments WHERE thread_id = $1 AND channel_id = $2`, other, w.allgemein); n != 1 {
		t.Error("a row of another thread moved")
	}
	if n := w.count(`SELECT count(*) FROM files WHERE id = ANY($1::uuid[]) AND scope_channel_id = $2`, []string{f1.String(), f2.String()}, w.allgemein); n != 2 {
		t.Error("files.scope_channel_id changed")
	}
	if err := subs.OnThreadMoved(ctx, ev); err == nil {
		t.Error("OnThreadMoved outside a unit of work")
	}
}
