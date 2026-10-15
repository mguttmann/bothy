// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"errors"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/messaging"
	"github.com/OWNER/bothy/services/server/internal/messaging/adapters/postgres"
	"github.com/OWNER/bothy/services/server/internal/messaging/app"
	"github.com/OWNER/bothy/services/server/internal/messaging/domain"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
	mw "github.com/OWNER/bothy/services/server/test/messagingworld"
)

func TestMain(m *testing.M) { os.Exit(pgtest.Main(m)) }

// posted is the test subscriber of MessagePosted (the txn registry derives its module from here).
type posted struct{ rec *mw.Recorder }

func (p posted) onPosted(ctx context.Context, ev messaging.MessagePosted) error {
	return p.rec.On(ctx, ev)
}

// TestRepository_MessagesAndMentions: insert and read back a message with every column of M1,
// the idempotency lookup, the mentions in one statement sorted, a second client_msg_id of the
// author as ErrClientMsgIDTaken, and writes outside a unit of work refused.
func TestRepository_MessagesAndMentions(t *testing.T) {
	t.Parallel()
	w := mw.New(t, func(reg *txn.Registry, rec *mw.Recorder) error { return txn.Subscribe(reg, posted{rec}.onPosted) })
	_, ch, main := w.TextChannel()
	sam := w.Person("sam", false)
	repo := postgres.NewRepository(w.Pool)
	reg := txn.NewRegistry()
	reg.Seal()
	uow := pg.NewUnitOfWork(w.Pool, pg.UnitConfig{StatementTimeout: 30 * time.Second, LockTimeout: 10 * time.Second, IdleInTxTimeout: time.Minute}, reg)
	ctx := context.Background()
	at := time.Date(2026, 10, 14, 9, 0, 0, 123000, time.UTC)
	m := domain.Message{
		ID: ids.New(), ThreadID: main, ChannelID: ch, ThreadSeq: 1, AuthorID: w.OwnerID, ClientMsgID: ids.New(),
		PayloadSHA256: []byte{1, 2, 3}, Origin: domain.OriginHuman, Kind: domain.KindText, ContentMD: "hello",
		ContentFormat: domain.ContentFormat, Silent: true, Version: 1, CreatedAt: at,
	}
	if _, err := repo.InsertMessage(ctx, m); err == nil {
		t.Fatal("insert outside a unit of work")
	}
	err := uow.Do(ctx, func(ctx context.Context) error {
		if _, err := repo.InsertMessage(ctx, m); err != nil {
			return err
		}
		reply := m
		reply.ID, reply.ThreadSeq, reply.ClientMsgID, reply.ReplyToID = ids.New(), 2, ids.New(), m.ID
		if _, err := repo.InsertMessage(ctx, reply); err != nil {
			return err
		}
		return repo.InsertMentions(ctx, m.ID, main, []ids.ID{sam, w.OwnerID}, at)
	})
	if err != nil {
		t.Fatal(err)
	}
	got, found, err := repo.Message(ctx, m.ID)
	if err != nil || !found || got.ContentMD != "hello" || !got.Silent || got.Origin != domain.OriginHuman || !got.CreatedAt.Equal(at) ||
		got.Version != 1 || got.ContentFormat != 1 || !got.ReplyToID.IsZero() || got.SystemCode != "" {
		t.Fatalf("Message = %+v, %v, %v", got, found, err)
	}
	byClient, found, err := repo.MessageByClientID(ctx, w.OwnerID, m.ClientMsgID)
	if err != nil || !found || byClient.ID != m.ID {
		t.Fatalf("MessageByClientID = %+v, %v, %v", byClient, found, err)
	}
	if _, found, err := repo.MessageByClientID(ctx, sam, m.ClientMsgID); found || err != nil {
		t.Fatalf("another author: %v %v", found, err)
	}
	if _, found, err := repo.Message(ctx, ids.New()); found || err != nil {
		t.Fatalf("unknown message: %v %v", found, err)
	}
	mentions, err := repo.Mentions(ctx, m.ID)
	want := []ids.ID{sam, w.OwnerID}
	slices.SortFunc(want, func(a, b ids.ID) int { return slices.Compare(a[:], b[:]) })
	if err != nil || !slices.Equal(mentions, want) {
		t.Fatalf("Mentions = %v, %v", mentions, err)
	}
	err = uow.Do(ctx, func(ctx context.Context) error {
		dup := m
		dup.ID, dup.ThreadSeq = ids.New(), 3
		_, err := repo.InsertMessage(ctx, dup)
		return err
	})
	if !errors.Is(err, app.ErrClientMsgIDTaken) {
		t.Fatalf("second client_msg_id: %v", err)
	}
	if err := uow.Do(ctx, func(ctx context.Context) error { return repo.InsertMentions(ctx, m.ID, main, nil, at) }); err != nil {
		t.Fatalf("no mentions: %v", err)
	}
}
