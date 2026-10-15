// SPDX-License-Identifier: Apache-2.0

package sqlcgen_test

import (
	"context"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/messaging"
	"github.com/OWNER/bothy/services/server/internal/messaging/adapters/postgres/sqlcgen"
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

func uuid(id ids.ID) pgtype.UUID { return pgtype.UUID{Bytes: id, Valid: true} }

// TestQueries_RunAgainstTheSchema (M1-56, M1-60): every generated query of messaging runs as
// bothy_app against the migrated schema of PostgreSQL 18 and returns what it promises, the
// two-array unnest of CountUnreadMentions and the keyset of ListMyMentions included.
func TestQueries_RunAgainstTheSchema(t *testing.T) {
	t.Parallel()
	w := mw.New(t, func(reg *txn.Registry, rec *mw.Recorder) error { return txn.Subscribe(reg, posted{rec}.onPosted) })
	ch := w.Channel(w.Space("Support"), "text", "general")
	main := w.MainThread(ch)
	sam := w.Person("sam", false)
	sent := w.Send("owner", main, "@sam hello", http.StatusCreated)
	msgID := ids.MustParse(sent["id"].(string))
	ctx := context.Background()
	q := sqlcgen.New(pg.Querier(ctx, w.Pool))

	m, err := q.GetMessage(ctx, uuid(msgID))
	if err != nil || m.ContentMd != "@sam hello" {
		t.Fatalf("GetMessage %+v %v", m, err)
	}
	if byClient, err := q.GetMessageByClientID(ctx, sqlcgen.GetMessageByClientIDParams{AuthorPrincipalID: m.AuthorPrincipalID, ClientMsgID: m.ClientMsgID}); err != nil || byClient.ID != m.ID {
		t.Fatalf("GetMessageByClientID %v", err)
	}
	at := pgtype.Timestamptz{Time: time.Date(2026, 10, 14, 10, 0, 0, 0, time.UTC), Valid: true}
	second, err := q.InsertMessage(ctx, sqlcgen.InsertMessageParams{
		ID: uuid(ids.New()), ThreadID: uuid(main), ChannelID: uuid(ch), ThreadSeq: 100, AuthorPrincipalID: m.AuthorPrincipalID,
		ClientMsgID: uuid(ids.New()), ClientPayloadSha256: []byte{1}, Origin: "human", Kind: "text", ContentMd: "@sam again",
		ContentFormat: 1, CreatedAt: at, SearchLanguage: "simple",
	})
	if err != nil || second.Version != 1 {
		t.Fatalf("InsertMessage %+v %v", second, err)
	}
	if err := q.InsertMentions(ctx, sqlcgen.InsertMentionsParams{MessageID: second.ID, ThreadID: uuid(main), CreatedAt: at,
		PrincipalIds: []pgtype.UUID{uuid(sam)}}); err != nil {
		t.Fatal(err)
	}
	if list, err := q.ListMentions(ctx, second.ID); err != nil || len(list) != 1 {
		t.Fatalf("ListMentions %v %v", list, err)
	}
	visible := []pgtype.UUID{uuid(ch)}
	page, err := q.ListMyMentions(ctx, sqlcgen.ListMyMentionsParams{PrincipalID: uuid(sam), VisibleChannelIds: visible, RowLimit: 1,
		AfterAt: pgtype.Timestamptz{Valid: true}, AfterID: uuid(ids.ID{})})
	if err != nil || len(page) != 1 || page[0].ID != second.ID {
		t.Fatalf("ListMyMentions %+v %v", page, err)
	}
	rest, err := q.ListMyMentions(ctx, sqlcgen.ListMyMentionsParams{PrincipalID: uuid(sam), VisibleChannelIds: visible, RowLimit: 5,
		HasAfter: true, AfterAt: page[0].MentionedAt, AfterID: page[0].ID})
	if err != nil || len(rest) != 1 || rest[0].ID != m.ID {
		t.Fatalf("ListMyMentions after %+v %v", rest, err)
	}
	counts, err := q.CountUnreadMentions(ctx, sqlcgen.CountUnreadMentionsParams{ReadThreadIds: []pgtype.UUID{uuid(main)}, ReadSeqs: []int64{1},
		PrincipalID: uuid(sam), VisibleChannelIds: visible})
	if err != nil || len(counts) != 1 || counts[0].MentionCount != 1 {
		t.Fatalf("CountUnreadMentions %+v %v", counts, err)
	}

	tpl, err := q.InsertTemplate(ctx, sqlcgen.InsertTemplateParams{ID: uuid(ids.New()), OwnerUserID: uuid(sam), Name: "Mine", ContentMd: "x",
		CreatedBy: uuid(sam), CreatedAt: at})
	if err != nil {
		t.Fatal(err)
	}
	chTpl, err := q.InsertTemplate(ctx, sqlcgen.InsertTemplateParams{ID: uuid(ids.New()), ChannelID: uuid(ch), Name: "Channel", ContentMd: "y",
		CreatedBy: uuid(w.OwnerID), CreatedAt: at})
	if err != nil {
		t.Fatal(err)
	}
	if n, err := q.CountPersonalTemplates(ctx, uuid(sam)); err != nil || n != 1 {
		t.Fatalf("CountPersonalTemplates %d %v", n, err)
	}
	if n, err := q.CountChannelTemplates(ctx, uuid(ch)); err != nil || n != 1 {
		t.Fatalf("CountChannelTemplates %d %v", n, err)
	}
	if list, err := q.ListPersonalTemplates(ctx, uuid(sam)); err != nil || len(list) != 1 {
		t.Fatalf("ListPersonalTemplates %v %v", list, err)
	}
	if list, err := q.ListChannelTemplates(ctx, sqlcgen.ListChannelTemplatesParams{CallerID: uuid(sam), ChannelID: uuid(ch), VisibleChannelIds: visible}); err != nil || len(list) != 2 {
		t.Fatalf("ListChannelTemplates %v %v", list, err)
	}
	got, err := q.GetVisibleTemplate(ctx, sqlcgen.GetVisibleTemplateParams{ID: chTpl.ID, CallerID: uuid(sam), VisibleChannelIds: visible})
	if err != nil || got.ID != chTpl.ID {
		t.Fatalf("GetVisibleTemplate %v", err)
	}
	if up, err := q.UpdateTemplate(ctx, sqlcgen.UpdateTemplateParams{Name: "New", ContentMd: "z", ID: tpl.ID}); err != nil || up.Version != 2 {
		t.Fatalf("UpdateTemplate %+v %v", up, err)
	}
	if del, err := q.DeleteTemplate(ctx, sqlcgen.DeleteTemplateParams{DeletedAt: at, ID: tpl.ID}); err != nil || del.Version != 3 || !del.DeletedAt.Valid {
		t.Fatalf("DeleteTemplate %+v %v", del, err)
	}
}
