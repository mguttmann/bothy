// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/messaging"
	"github.com/OWNER/bothy/services/server/internal/messaging/app"
	"github.com/OWNER/bothy/services/server/internal/messaging/domain"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
)

// mismatchAuthz allows everything except message.read on the message deny, so the visible
// predicate and the per-item check disagree (07 section 7.4 rule 3).
type mismatchAuthz struct {
	deny ids.ID
	// filtered records whether every message.read ran in filter mode.
	filtered bool
}

func (m *mismatchAuthz) Authorize(ctx context.Context, _ authorize.Principal, a authorize.Action, r authorize.Resource) (authorize.Decision, error) {
	if a == authorize.ActionMessageRead {
		m.filtered = m.filtered && authorize.IsFilter(ctx)
		if r.ID == m.deny {
			return authorize.Decision{Outcome: authorize.OutcomeNotFound}, nil
		}
	}
	return authorize.Decision{Outcome: authorize.OutcomeAllow}, nil
}

func (*mismatchAuthz) Scope(context.Context, authorize.Principal, authorize.Action) (authorize.Scope, error) {
	return authorize.Scope{}, nil
}

// rowsStore returns fixed history rows.
type rowsStore struct {
	app.Store
	rows app.HistoryRows
}

func (s rowsStore) History(context.Context, app.HistoryRead) (app.HistoryRows, error) {
	return s.rows, nil
}
func (rowsStore) Mentions(context.Context, ids.ID) ([]ids.ID, error) { return nil, nil }

// oneThread knows one thread.
type oneThread struct {
	chat.Threads
	info chat.ThreadInfo
}

func (o oneThread) Info(context.Context, ids.ID) (chat.ThreadInfo, error) { return o.info, nil }

// anyone resolves every principal.
type anyone struct{ identity.Directory }

func (anyone) Principal(_ context.Context, id ids.ID) (identity.PrincipalView, error) {
	return identity.PrincipalView{ID: id, Kind: "user", Handle: "sam", DisplayName: "Sam"}, nil
}

// counter (mentions_test.go) records CountError.

// noUoW and noEvents are never used by History.
type (
	noUoW    struct{ txn.UnitOfWork }
	noEvents struct{ eventlog.Appender }
)

// TestHistory_FilterMismatchDropsLogsAndCounts (brief M1-57 note 2 step 4): an item the visible
// predicate let through but message.read refuses is dropped, logged at error level with the
// action only and counted as filter_mismatch; every item check runs in filter mode.
func TestHistory_FilterMismatchDropsLogsAndCounts(t *testing.T) {
	t.Parallel()
	thread, ch, author := ids.New(), ids.New(), ids.New()
	var below []domain.Message
	for seq := int64(3); seq >= 1; seq-- {
		below = append(below, domain.Message{ID: ids.New(), ThreadID: thread, ChannelID: ch, ThreadSeq: seq, AuthorID: author,
			Kind: domain.KindText, Origin: domain.OriginHuman, ContentMD: "private words", Version: 1})
	}
	az := &mismatchAuthz{deny: below[1].ID, filtered: true}
	logs := &httpservertest.LogCapture{}
	cnt := &counter{}
	svc, err := app.New(app.Config{MaxContentBytes: 100, MaxMentions: 1, EventPayloadMaxBytes: 100, ListContentMaxBytes: 100, PageSizeDefault: 10, PageSizeMax: 10},
		app.Deps{Store: rowsStore{rows: app.HistoryRows{Below: below}}, UoW: noUoW{}, Authorizer: az, Events: noEvents{},
			Threads: oneThread{info: chat.ThreadInfo{ID: thread, ChannelID: ch}}, Directory: anyone{}, Log: slog.New(logs), Errors: cnt})
	if err != nil {
		t.Fatal(err)
	}
	page, err := svc.History(context.Background(), messaging.HistoryQuery{ThreadID: thread})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 2 || page.Items[0].ThreadSeq != 1 || page.Items[1].ThreadSeq != 3 {
		t.Fatalf("items %+v", page.Items)
	}
	if !az.filtered {
		t.Error("message.read ran outside filter mode")
	}
	if len(cnt.reasons) != 1 || cnt.reasons[0] != "filter_mismatch" {
		t.Errorf("counted %v", cnt.reasons)
	}
	recs := logs.Records()
	if len(recs) != 1 || recs[0].Level != slog.LevelError || recs[0].Attrs["bothy.action"] != "message.read" || recs[0].Attrs["bothy.reason"] != "filter_mismatch" {
		t.Fatalf("records %+v", recs)
	}
	for _, v := range recs[0].Attrs {
		if v == below[1].ID.String() || v == thread.String() || v == "private words" {
			t.Errorf("record carries %q", v)
		}
	}
}

// decideAuthz answers message.read with the outcome per message id (allow when absent) and
// records the resources it saw.
type decideAuthz struct {
	outcomes map[ids.ID]authorize.Outcome
	seen     []ids.ID
}

func (d *decideAuthz) Authorize(_ context.Context, _ authorize.Principal, a authorize.Action, r authorize.Resource) (authorize.Decision, error) {
	if a == authorize.ActionMessageRead {
		d.seen = append(d.seen, r.ID)
		if o, ok := d.outcomes[r.ID]; ok {
			return authorize.Decision{Outcome: o, Reason: "scope_missing"}, nil
		}
	}
	return authorize.Decision{Outcome: authorize.OutcomeAllow}, nil
}

func (*decideAuthz) Scope(context.Context, authorize.Principal, authorize.Action) (authorize.Scope, error) {
	return authorize.Scope{ChannelIDs: []ids.ID{}}, nil
}

// getStore holds the visible and the existing messages of Get.
type getStore struct {
	app.Store
	visible, all map[ids.ID]domain.Message
}

func (s getStore) VisibleMessage(_ context.Context, id ids.ID, _ []ids.ID) (domain.Message, bool, error) {
	m, ok := s.visible[id]
	return m, ok, nil
}

func (s getStore) Message(_ context.Context, id ids.ID) (domain.Message, bool, error) {
	m, ok := s.all[id]
	return m, ok, nil
}

func (getStore) Mentions(context.Context, ids.ID) ([]ids.ID, error) { return []ids.ID{ids.New()}, nil }

// TestGet_Paths (brief note 7, M1-57/9): a visible message is returned whole; a message outside
// the visible channels is decided by message.read (forbidden for an app with a grant, not_found
// otherwise, and not_found when message.read allows what the visible read missed); an unknown id
// is not_found without a check; a tombstone is returned without content and mentions.
func TestGet_Paths(t *testing.T) {
	t.Parallel()
	thread, ch := ids.New(), ids.New()
	msg := func(content string, deleted bool) domain.Message {
		m := domain.Message{ID: ids.New(), ThreadID: thread, ChannelID: ch, ThreadSeq: 1, AuthorID: ids.New(), Kind: domain.KindText,
			Origin: domain.OriginHuman, ContentMD: content, Version: 1}
		if deleted {
			m.ContentMD, m.Version, m.DeletedAt = "", 2, time.Date(2026, 10, 15, 9, 0, 0, 0, time.UTC)
		}
		return m
	}
	visible, gone, scoped, foreign, contradicted := msg("whole", false), msg("", true), msg("x", false), msg("y", false), msg("z", false)
	store := getStore{
		visible: map[ids.ID]domain.Message{visible.ID: visible, gone.ID: gone},
		all:     map[ids.ID]domain.Message{visible.ID: visible, gone.ID: gone, scoped.ID: scoped, foreign.ID: foreign, contradicted.ID: contradicted},
	}
	az := &decideAuthz{outcomes: map[ids.ID]authorize.Outcome{scoped.ID: authorize.OutcomeForbidden, foreign.ID: authorize.OutcomeNotFound}}
	svc, err := app.New(app.Config{MaxContentBytes: 100, MaxMentions: 1, EventPayloadMaxBytes: 100, ListContentMaxBytes: 2, PageSizeDefault: 10, PageSizeMax: 10},
		app.Deps{Store: store, UoW: noUoW{}, Authorizer: az, Events: noEvents{}, Threads: oneThread{}, Directory: anyone{}})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	got, err := svc.Get(ctx, visible.ID)
	if err != nil || got.ContentMD != "whole" || got.ContentTruncated || len(got.MentionIDs) != 1 {
		t.Fatalf("visible: %+v %v", got, err)
	}
	got, err = svc.Get(ctx, gone.ID)
	if err != nil || got.ContentMD != "" || len(got.MentionIDs) != 0 || got.DeletedAt.IsZero() || got.Version != 2 {
		t.Fatalf("tombstone: %+v %v", got, err)
	}
	for id, want := range map[ids.ID]errs.Code{scoped.ID: errs.CodeScopeMissing, foreign.ID: errs.CodeNotFound, contradicted.ID: errs.CodeNotFound, ids.New(): errs.CodeNotFound} {
		_, err := svc.Get(ctx, id)
		if e, ok := errs.As(err); !ok || e.Code != want {
			t.Errorf("Get: %v, want %s", err, want)
		}
	}
}
