// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/messaging/adapters/postgres"
	"github.com/OWNER/bothy/services/server/internal/messaging/app"
	"github.com/OWNER/bothy/services/server/internal/messaging/domain"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	mw "github.com/OWNER/bothy/services/server/test/messagingworld"
)

// attachWorld is a world with the repository, a unit of work and one text channel.
type attachWorld struct {
	w        *mw.World
	repo     *postgres.Repository
	uow      *pg.UnitOfWork
	ch, main ids.ID
}

func newAttachWorld(t *testing.T) attachWorld {
	t.Helper()
	w := mw.New(t, func(reg *txn.Registry, rec *mw.Recorder) error { return txn.Subscribe(reg, posted{rec}.onPosted) })
	_, ch, main := w.TextChannel()
	reg := txn.NewRegistry()
	reg.Seal()
	uow := pg.NewUnitOfWork(w.Pool, pg.UnitConfig{StatementTimeout: 30 * time.Second, LockTimeout: 10 * time.Second, IdleInTxTimeout: time.Minute}, reg)
	return attachWorld{w: w, repo: postgres.NewRepository(w.Pool), uow: uow, ch: ch, main: main}
}

// insert writes a text message of the owner at seq with the flag and returns it.
func (a attachWorld) insert(t *testing.T, seq int64, flag bool, at time.Time) domain.Message {
	t.Helper()
	m := domain.Message{
		ID: ids.New(), ThreadID: a.main, ChannelID: a.ch, ThreadSeq: seq, AuthorID: a.w.OwnerID, ClientMsgID: ids.New(),
		PayloadSHA256: []byte{1}, Origin: domain.OriginHuman, Kind: domain.KindText, ContentMD: "with or without files",
		ContentFormat: domain.ContentFormat, HasAttachments: flag, Version: 1, CreatedAt: at, SearchLanguage: "english",
	}
	var got domain.Message
	if err := a.uow.Do(context.Background(), func(ctx context.Context) error {
		var err error
		got, err = a.repo.InsertMessage(ctx, m)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if got.HasAttachments != flag {
		t.Fatalf("InsertMessage returned has_attachments %v, want %v", got.HasAttachments, flag)
	}
	if n := a.w.Count(`SELECT count(*)::int FROM messages WHERE id = $1 AND has_attachments = $2`, m.ID.String(), flag); n != 1 {
		t.Fatalf("stored has_attachments differs from %v", flag)
	}
	return got
}

// TestRepository_HasAttachments (brief M3-07 design note 7 and requirement 14): the INSERT writes
// has_attachments, every read that builds domain.Message returns it (by id, by client_msg_id,
// visible, locked, history, mentions, export, the trigger state write), and the tombstone resets
// it together with content_md.
func TestRepository_HasAttachments(t *testing.T) {
	t.Parallel()
	a := newAttachWorld(t)
	ctx := context.Background()
	at := time.Date(2026, 10, 21, 9, 0, 0, 0, time.UTC)
	flagged := a.insert(t, 1, true, at)
	plain := a.insert(t, 2, false, at)
	a.w.SQL(`UPDATE threads SET last_message_seq = 2 WHERE id = $1`, a.main.String())
	visible := []ids.ID{a.ch}

	want := func(what string, m domain.Message, found bool, err error, flag bool) {
		t.Helper()
		if err != nil || !found || m.HasAttachments != flag {
			t.Errorf("%s: has_attachments %v (found %v, %v), want %v", what, m.HasAttachments, found, err, flag)
		}
	}
	for _, c := range []struct {
		m    domain.Message
		flag bool
	}{{flagged, true}, {plain, false}} {
		m, found, err := a.repo.Message(ctx, c.m.ID)
		want("Message", m, found, err, c.flag)
		m, found, err = a.repo.MessageByClientID(ctx, a.w.OwnerID, c.m.ClientMsgID)
		want("MessageByClientID", m, found, err, c.flag)
		m, found, err = a.repo.VisibleMessage(ctx, c.m.ID, visible)
		want("VisibleMessage", m, found, err, c.flag)
		if err := a.uow.Do(ctx, func(ctx context.Context) error {
			m, found, err = a.repo.LockMessage(ctx, c.m.ID, visible)
			return err
		}); err != nil {
			t.Fatal(err)
		}
		want("LockMessage", m, found, err, c.flag)
	}

	rows, err := a.repo.History(ctx, app.HistoryRead{ThreadID: a.main, Visible: visible, Mode: domain.HistoryNewest, RowLimit: 10})
	if err != nil || len(rows.Below) != 2 {
		t.Fatalf("History: %d rows, %v", len(rows.Below), err)
	}
	for _, m := range rows.Below {
		want("History", m, true, nil, m.ID == flagged.ID)
	}
	exported, err := a.repo.ExportMessages(ctx, app.ExportRead{ChannelID: a.ch, Visible: visible, From: at.Add(-time.Hour), To: at.Add(time.Hour), Limit: 10},
		domain.ExportCursor{ThreadID: a.main})
	if err != nil || len(exported) != 2 {
		t.Fatalf("ExportMessages: %d rows, %v", len(exported), err)
	}
	for _, m := range exported {
		want("ExportMessages", m, true, nil, m.ID == flagged.ID)
	}
	if err := a.uow.Do(ctx, func(ctx context.Context) error {
		return a.repo.InsertMentions(ctx, flagged.ID, a.main, []ids.ID{a.w.OwnerID}, at)
	}); err != nil {
		t.Fatal(err)
	}
	mentions, err := a.repo.MyMentions(ctx, app.MentionQuery{PrincipalID: a.w.OwnerID, Visible: visible, Limit: 10})
	if err != nil || len(mentions) != 1 {
		t.Fatalf("MyMentions: %d rows, %v", len(mentions), err)
	}
	want("MyMentions", mentions[0].Message, true, nil, true)
	var triggered domain.Message
	if err := a.uow.Do(ctx, func(ctx context.Context) error {
		var err error
		triggered, _, err = a.repo.SetTriggerState(ctx, flagged.ID, "received", "", at)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	want("SetTriggerState", triggered, true, nil, true)

	var tomb domain.Message
	if err := a.uow.Do(ctx, func(ctx context.Context) error {
		if _, _, err := a.repo.LockMessage(ctx, flagged.ID, visible); err != nil {
			return err
		}
		var err error
		tomb, err = a.repo.Tombstone(ctx, flagged.ID, visible, a.w.OwnerID, at.Add(time.Minute))
		return err
	}); err != nil {
		t.Fatalf("Tombstone of a message with attachments: %v", err)
	}
	if tomb.HasAttachments || tomb.ContentMD != "" || tomb.DeletedAt.IsZero() {
		t.Fatalf("tombstone %+v", tomb)
	}
	m, found, err := a.repo.Message(ctx, flagged.ID)
	want("Message after the tombstone", m, found, err, false)
}

// TestHasAttachments_CheckRejectsDeletedWithFlag (03 messages, requirement 14): the CHECK
// messages_has_attachments_deleted_check refuses a direct UPDATE that deletes a flagged message
// without resetting the flag; with the flag reset the same UPDATE passes.
func TestHasAttachments_CheckRejectsDeletedWithFlag(t *testing.T) {
	t.Parallel()
	a := newAttachWorld(t)
	m := a.insert(t, 1, true, time.Date(2026, 10, 21, 9, 0, 0, 0, time.UTC))
	_, err := a.w.Owner.Exec(context.Background(), `UPDATE messages SET deleted_at = now(), content_md = '' WHERE id = $1`, m.ID.String())
	if err == nil || !strings.Contains(err.Error(), "messages_has_attachments_deleted_check") {
		t.Fatalf("deleting a flagged message: %v, want the CHECK violation", err)
	}
	if n := a.w.Count(`SELECT count(*)::int FROM messages WHERE id = $1 AND deleted_at IS NULL AND has_attachments`, m.ID.String()); n != 1 {
		t.Fatal("the refused UPDATE changed the row")
	}
	a.w.SQL(`UPDATE messages SET deleted_at = now(), content_md = '', has_attachments = false WHERE id = $1`, m.ID.String())
}
