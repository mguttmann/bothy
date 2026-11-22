// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/files"
	"github.com/OWNER/bothy/services/server/internal/files/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// relay runs Relay of source to the copy m in Eskalation's thread copyThread in its own unit.
func (w *world) relay(source, m, copyThread ids.ID) ([]files.Attachment, error) {
	var out []files.Attachment
	err := w.uow.Do(context.Background(), func(ctx context.Context) error {
		var err error
		out, err = w.svc.Relay(ctx, files.RelayCmd{SourceMessageID: source, MessageID: m, ThreadID: copyThread, ChannelID: w.eskalation})
		return err
	})
	return out, err
}

// copyIn inserts a message of Alex into the thread th of Eskalation and returns it.
func (w *world) copyIn(th ids.ID, seq int) ids.ID {
	w.t.Helper()
	m := ids.New()
	w.exec(`INSERT INTO messages (id, thread_id, channel_id, thread_seq, author_principal_id, client_msg_id, client_payload_sha256, origin,
		content_md, has_attachments) VALUES ($1, $2, $3, $4, $5, $6, '\x00', 'human', 'copy', true)`, m, th, w.eskalation, seq, w.alex, ids.New())
	return m
}

// TestRepository_Relays holds FIX-TEAM-CHANNEL-ATTACHMENTS D-TCAT-1 to D-TCAT-3 against PostgreSQL:
// Relay writes message_attachment_relays at the source's positions with the copy's place and
// skips a deleted file; ForMessages and Pending read a copy's relays like attachments; the copy
// of a copy relays the same files; message_attachments keeps the one original row (MessageOf,
// CountFilesOfThreads); RelaysOf lists the copies' places; a thread move takes the relays along;
// deleting a copy keeps the file, deleting the source removes every relay and marks the file
// deleted.
func TestRepository_Relays(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	ctx := context.Background()
	f1 := w.file(w.alex, w.allgemein, "attachment", "completed", "complete", "clean")
	gone := w.file(w.alex, w.allgemein, "attachment", "completed", "complete", "clean")
	f3 := w.attachable()
	if _, err := w.attach(w.m1, f1, gone, f3); err != nil {
		t.Fatal(err)
	}
	w.exec(`UPDATE files SET deleted_at = now() WHERE id = $1`, gone)
	work := ids.New()
	w.exec(`INSERT INTO threads (id, channel_id, kind, created_by) VALUES ($1, $2, 'thread', $3)`, work, w.eskalation, w.alex)
	c1, c2 := w.copyIn(work, 1), w.copyIn(work, 2)

	got, err := w.relay(w.m1, c1, work)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].File.ID != f1 || got[0].Position != 1 || got[1].File.ID != f3 || got[1].Position != 3 ||
		!got[0].File.ScopeChannelID.IsZero() {
		t.Fatalf("Relay = %+v", got)
	}
	if n := w.count(`SELECT count(*) FROM message_attachment_relays WHERE message_id = $1 AND thread_id = $2 AND channel_id = $3`,
		c1, work, w.eskalation); n != 2 {
		t.Fatalf("%d relays with the copy's place", n)
	}
	// The input made from a mirror: a copy of the copy relays the same files.
	if got, err := w.relay(c1, c2, work); err != nil || len(got) != 2 || got[0].File.ID != f1 || got[1].File.ID != f3 {
		t.Fatalf("Relay of a copy = %+v %v", got, err)
	}
	list, err := w.svc.ForMessages(ctx, []ids.ID{w.m1, c1})
	if err != nil || len(list[w.m1]) != 3 || len(list[c1]) != 2 || list[c1][1].File.ID != f3 {
		t.Fatalf("ForMessages = %+v %v", list, err)
	}
	if n := w.count(`SELECT count(*) FROM message_attachments`); n != 3 {
		t.Errorf("message_attachments has %d rows, want the 3 originals", n)
	}
	if ref, ok, err := w.svc.MessageOf(ctx, f1); err != nil || !ok || ref.MessageID != w.m1 {
		t.Errorf("MessageOf = %+v %v %v, want the source", ref, ok, err)
	}
	repo := w.repo
	relays, err := repo.RelaysOf(ctx, f3)
	if err != nil || len(relays) != 2 || relays[0].ThreadID != work || relays[0].ChannelID != w.eskalation {
		t.Fatalf("RelaysOf = %+v %v", relays, err)
	}
	reason := func(m ids.ID) files.PendingReason {
		t.Helper()
		var p files.PendingReason
		if err := w.uow.Do(ctx, func(ctx context.Context) error {
			var err error
			p, err = w.svc.Pending(ctx, m)
			return err
		}); err != nil {
			t.Fatal(err)
		}
		return p
	}
	if got := reason(c1); got != files.PendingFiles {
		t.Errorf("a copy with a relayed pending file: %v, want PendingFiles", got)
	}
	w.exec(`UPDATE files SET upload_state = 'complete', scan_status = 'clean' WHERE id = $1`, f3)
	if got := reason(c1); got != files.PendingNone {
		t.Errorf("a copy whose files are final: %v, want PendingNone", got)
	}

	subs, err := app.NewSubscribers(repo)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.uow.Do(ctx, func(ctx context.Context) error {
		return subs.OnThreadMoved(ctx, chat.ThreadMoved{ThreadID: work, FromChannelID: w.eskalation, ToChannelID: w.allgemein})
	}); err != nil {
		t.Fatal(err)
	}
	if n := w.count(`SELECT count(*) FROM message_attachment_relays WHERE channel_id = $1`, w.allgemein); n != 4 {
		t.Errorf("%d relays moved with the thread, want 4", n)
	}

	del := func(m ids.ID) {
		t.Helper()
		if err := w.uow.Do(ctx, func(ctx context.Context) error { return w.svc.DeleteForMessage(ctx, m) }); err != nil {
			t.Fatal(err)
		}
	}
	del(c2)
	if n := w.count(`SELECT count(*) FROM message_attachment_relays WHERE message_id = $1`, c2); n != 0 {
		t.Errorf("%d relays of a deleted copy left", n)
	}
	if n := w.count(`SELECT count(*) FROM files WHERE id = ANY($1::uuid[]) AND deleted_at IS NULL`, []string{f1.String(), f3.String()}); n != 2 {
		t.Errorf("deleting a copy deleted %d files", 2-n)
	}
	del(w.m1)
	if n := w.count(`SELECT count(*) FROM message_attachment_relays`); n != 0 {
		t.Errorf("%d relays left after the source was deleted", n)
	}
	if n := w.count(`SELECT count(*) FROM files WHERE id = ANY($1::uuid[]) AND deleted_at IS NOT NULL`, []string{f1.String(), f3.String()}); n != 2 {
		t.Errorf("deleting the source marked %d of 2 files deleted", n)
	}
}
