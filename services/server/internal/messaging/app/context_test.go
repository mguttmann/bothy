// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/messaging"
	"github.com/OWNER/bothy/services/server/internal/messaging/app"
	"github.com/OWNER/bothy/services/server/internal/messaging/domain"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
)

// contextStore returns fixed context rows and records the read; Message answers from all.
type contextStore struct {
	bareStore
	rows app.ContextRows
	read *app.ContextRead
	all  map[ids.ID]domain.Message
}

func (s contextStore) ContextWindow(_ context.Context, q app.ContextRead) (app.ContextRows, error) {
	*s.read = q
	return s.rows, nil
}

func (s contextStore) Message(_ context.Context, id ids.ID) (domain.Message, bool, error) {
	m, ok := s.all[id]
	return m, ok, nil
}

func (contextStore) Mentions(context.Context, ids.ID) ([]ids.ID, error) { return nil, nil }

// TestContextWindow (M2-13 design note 3): the query reaches the store unchanged, the items keep
// their ascending order and the flag, content above message.list_content_max_bytes stays whole,
// every item is checked with message.read in filter mode, and a refused item is dropped, logged
// with the action only and counted as filter_mismatch. ContextMessage answers ok false for an
// unknown and for a refused message alike and returns a tombstone as stored.
func TestContextWindow(t *testing.T) {
	t.Parallel()
	thread, ch, author := ids.New(), ids.New(), ids.New()
	long := strings.Repeat("y", 300)
	var rows []domain.Message
	for seq := int64(4); seq <= 6; seq++ {
		rows = append(rows, domain.Message{ID: ids.New(), ThreadID: thread, ChannelID: ch, ThreadSeq: seq, AuthorID: author,
			Kind: domain.KindText, Origin: domain.OriginHuman, ContentMD: long, Version: 1})
	}
	gone := domain.Message{ID: ids.New(), ThreadID: thread, ChannelID: ch, ThreadSeq: 9, AuthorID: author, Kind: domain.KindText,
		Origin: domain.OriginHuman, Version: 2, DeletedAt: rows[0].CreatedAt.AddDate(2026, 0, 0)}
	read := &app.ContextRead{}
	store := contextStore{rows: app.ContextRows{Items: rows, HasOlder: true}, read: read,
		all: map[ids.ID]domain.Message{rows[0].ID: rows[0], rows[1].ID: rows[1], gone.ID: gone}}
	az := &mismatchAuthz{deny: rows[1].ID, filtered: true}
	logs := &httpservertest.LogCapture{}
	cnt := &counter{}
	svc, err := app.New(app.Config{MaxContentBytes: 1000, MaxMentions: 1, EventPayloadMaxBytes: 100, TemplateNameMaxChars: 10, TemplateMaxPerScope: 5,
		ListContentMaxBytes: 10, PageSizeDefault: 10, PageSizeMax: 10, EmojiMaxBytes: 64, MaxDistinctReactions: 20, MaxPinsPerThread: 50},
		app.Deps{Store: store, UoW: noUoW{}, Authorizer: az, Events: noEvents{},
			Threads: oneThread{info: chat.ThreadInfo{ID: thread, ChannelID: ch}}, Directory: anyone{}, Log: slog.New(logs), Errors: cnt,
			Cursors: noCursors{}, Redactor: struct{ eventlog.Redactor }{}, Audit: struct{ auditing.Recorder }{}, Symbols: anySymbol{}})
	if err != nil {
		t.Fatal(err)
	}
	exclude := ids.New()
	q := messaging.ContextQuery{ThreadID: thread, AfterSeq: 3, BeforeSeq: 7, Limit: 3, ExcludeAuthorID: exclude}
	page, err := svc.ContextWindow(context.Background(), q)
	if err != nil {
		t.Fatal(err)
	}
	if *read != (app.ContextRead{ThreadID: thread, AfterSeq: 3, BeforeSeq: 7, Limit: 3, ExcludeAuthorID: exclude}) {
		t.Errorf("store read %+v", *read)
	}
	if !page.HasOlder || len(page.Items) != 2 || page.Items[0].ThreadSeq != 4 || page.Items[1].ThreadSeq != 6 {
		t.Fatalf("page %+v", page)
	}
	for _, m := range page.Items {
		if m.ContentMD != long || m.ContentTruncated || m.Author.Handle != "sam" {
			t.Errorf("item %d: content of %d bytes, truncated %v, author %q", m.ThreadSeq, len(m.ContentMD), m.ContentTruncated, m.Author.Handle)
		}
	}
	if !az.filtered {
		t.Error("message.read ran outside filter mode")
	}
	if len(cnt.reasons) != 1 || cnt.reasons[0] != "filter_mismatch" {
		t.Errorf("counted %v", cnt.reasons)
	}
	recs := logs.Records()
	if len(recs) != 1 || recs[0].Level != slog.LevelError || recs[0].Attrs["bothy.reason"] != "filter_mismatch" {
		t.Fatalf("records %+v", recs)
	}
	for _, v := range recs[0].Attrs {
		if v == rows[1].ID.String() || v == long {
			t.Errorf("record carries %q", v)
		}
	}

	// ContextMessage: whole content, ok false for unknown and refused, a tombstone as stored.
	ctx := context.Background()
	if m, ok, err := svc.ContextMessage(ctx, rows[0].ID); err != nil || !ok || m.ContentMD != long {
		t.Errorf("ContextMessage = %d bytes %v %v", len(m.ContentMD), ok, err)
	}
	if _, ok, err := svc.ContextMessage(ctx, rows[1].ID); err != nil || ok {
		t.Errorf("refused: %v %v", ok, err)
	}
	if _, ok, err := svc.ContextMessage(ctx, ids.New()); err != nil || ok {
		t.Errorf("unknown: %v %v", ok, err)
	}
	if m, ok, err := svc.ContextMessage(ctx, gone.ID); err != nil || !ok || m.ContentMD != "" || m.DeletedAt.IsZero() {
		t.Errorf("tombstone = %+v %v %v", m, ok, err)
	}
	if !az.filtered {
		t.Error("ContextMessage ran message.read outside filter mode")
	}
	_ = authorize.ActionMessageRead
}
