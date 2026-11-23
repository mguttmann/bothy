// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"log/slog"
	"slices"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/messaging"
	"github.com/OWNER/bothy/services/server/internal/messaging/app"
	"github.com/OWNER/bothy/services/server/internal/messaging/domain"
)

// batchStore returns fixed history rows and counts the mention reads.
type batchStore struct {
	bareStore
	rows     app.HistoryRows
	mentions map[ids.ID][]ids.ID
	single   *int
	batches  *[][]ids.ID
	err      error // fails MentionsOfMessages when set
}

func (s batchStore) ContextWindow(context.Context, app.ContextRead) (app.ContextRows, error) {
	return app.ContextRows{Items: s.rows.Below}, nil
}

func (s batchStore) History(context.Context, app.HistoryRead) (app.HistoryRows, error) {
	return s.rows, nil
}

func (s batchStore) Mentions(_ context.Context, id ids.ID) ([]ids.ID, error) {
	*s.single++
	return s.mentions[id], nil
}

func (s batchStore) MentionsOfMessages(_ context.Context, list []ids.ID) (map[ids.ID][]ids.ID, error) {
	if s.err != nil {
		return nil, s.err
	}
	*s.batches = append(*s.batches, slices.Clone(list))
	out := map[ids.ID][]ids.ID{}
	for _, id := range list {
		if m, ok := s.mentions[id]; ok {
			out[id] = m
		}
	}
	return out, nil
}

// TestHistory_MentionsInOneRead (audit data-layer-2): a page reads the mentions of its live
// messages in one statement, never one per message, and a tombstone is not asked for.
func TestHistory_MentionsInOneRead(t *testing.T) {
	t.Parallel()
	thread, ch, author, sam := ids.New(), ids.New(), ids.New(), ids.New()
	var below []domain.Message
	for seq := int64(3); seq >= 1; seq-- {
		below = append(below, domain.Message{ID: ids.New(), ThreadID: thread, ChannelID: ch, ThreadSeq: seq, AuthorID: author,
			Kind: domain.KindText, Origin: domain.OriginHuman, ContentMD: "hello @sam", Version: 1})
	}
	below[1].DeletedAt = time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	single, batches := 0, [][]ids.ID{}
	store := batchStore{rows: app.HistoryRows{Below: below}, mentions: map[ids.ID][]ids.ID{below[0].ID: {sam}}, single: &single, batches: &batches}
	svc, err := app.New(app.Config{MaxContentBytes: 100, MaxMentions: 1, EventPayloadMaxBytes: 100, TemplateNameMaxChars: 10, TemplateMaxPerScope: 5, ListContentMaxBytes: 100, PageSizeDefault: 10, PageSizeMax: 10, EmojiMaxBytes: 64, MaxDistinctReactions: 20, HandleMaxChars: 32, DisplayNameMaxChars: 64, MaxPinsPerThread: 50},
		app.Deps{Store: store, UoW: noUoW{}, Authorizer: &mismatchAuthz{}, Events: noEvents{},
			Threads: oneThread{info: chat.ThreadInfo{ID: thread, ChannelID: ch}}, Directory: anyone{}, Log: slog.New(slog.DiscardHandler),
			Cursors: noCursors{}, Redactor: struct{ eventlog.Redactor }{}, Audit: struct{ auditing.Recorder }{}, Symbols: anySymbol{}})
	if err != nil {
		t.Fatal(err)
	}
	page, err := svc.History(context.Background(), messaging.HistoryQuery{ThreadID: thread})
	if err != nil {
		t.Fatal(err)
	}
	if single != 0 || len(batches) != 1 {
		t.Fatalf("single reads %d, batch reads %d", single, len(batches))
	}
	if !sameIDs(batches[0], []ids.ID{below[2].ID, below[0].ID}) {
		t.Errorf("batch asked for %v", batches[0])
	}
	if len(page.Items) != 3 || !slices.Equal(page.Items[2].MentionIDs, []ids.ID{sam}) || len(page.Items[0].MentionIDs) != 0 ||
		page.Items[1].MentionIDs == nil || len(page.Items[1].MentionIDs) != 0 {
		t.Fatalf("items %+v", page.Items)
	}
}

// noDirectory fails every principal read.
type noDirectory struct{ identity.Directory }

func (noDirectory) Principal(context.Context, ids.ID) (identity.PrincipalView, error) {
	return identity.PrincipalView{}, errs.New(errs.CodeInternalError, "directory: down")
}

// TestViews_ReadErrors: a failing mention or author read ends the history page and the context
// window with that error instead of a page with missing mentions or authors; without a failure
// both answer.
func TestViews_ReadErrors(t *testing.T) {
	t.Parallel()
	thread, ch := ids.New(), ids.New()
	below := []domain.Message{{ID: ids.New(), ThreadID: thread, ChannelID: ch, ThreadSeq: 1, AuthorID: ids.New(),
		Kind: domain.KindText, Origin: domain.OriginHuman, ContentMD: "hello", Version: 1}}
	for name, tc := range map[string]struct {
		mentionsErr error
		dir         identity.Directory
	}{
		"mentions": {mentionsErr: errs.New(errs.CodeInternalError, "store: down"), dir: anyone{}},
		"author":   {dir: noDirectory{}},
		"none":     {dir: anyone{}},
	} {
		single, batches := 0, [][]ids.ID{}
		store := batchStore{rows: app.HistoryRows{Below: below}, single: &single, batches: &batches, err: tc.mentionsErr}
		svc, err := app.New(app.Config{MaxContentBytes: 100, MaxMentions: 1, EventPayloadMaxBytes: 100, TemplateNameMaxChars: 10, TemplateMaxPerScope: 5, ListContentMaxBytes: 100, PageSizeDefault: 10, PageSizeMax: 10, EmojiMaxBytes: 64, MaxDistinctReactions: 20, HandleMaxChars: 32, DisplayNameMaxChars: 64, MaxPinsPerThread: 50},
			app.Deps{Store: store, UoW: noUoW{}, Authorizer: &mismatchAuthz{}, Events: noEvents{},
				Threads: oneThread{info: chat.ThreadInfo{ID: thread, ChannelID: ch}}, Directory: tc.dir, Log: slog.New(slog.DiscardHandler),
				Cursors: noCursors{}, Redactor: struct{ eventlog.Redactor }{}, Audit: struct{ auditing.Recorder }{}, Symbols: anySymbol{}})
		if err != nil {
			t.Fatal(err)
		}
		_, herr := svc.History(context.Background(), messaging.HistoryQuery{ThreadID: thread})
		_, cerr := svc.ContextWindow(context.Background(), messaging.ContextQuery{ThreadID: thread, BeforeSeq: 2, Limit: 5})
		if wantErr := name != "none"; (herr != nil) != wantErr || (cerr != nil) != wantErr {
			t.Errorf("%s: history err %v, context err %v, want an error %v", name, herr, cerr, wantErr)
		}
	}
}

// sameIDs reports whether a and b hold the same ids in any order.
func sameIDs(a, b []ids.ID) bool {
	if len(a) != len(b) {
		return false
	}
	for _, id := range a {
		if !slices.Contains(b, id) {
			return false
		}
	}
	return true
}
