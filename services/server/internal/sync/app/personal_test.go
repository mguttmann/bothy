// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
	syncmod "github.com/OWNER/bothy/services/server/internal/sync"
	"github.com/OWNER/bothy/services/server/internal/sync/app"
	mw "github.com/OWNER/bothy/services/server/test/messagingworld"
)

// TestMain starts the PostgreSQL containers of pgtest for the tests over test/messagingworld.
func TestMain(m *testing.M) { os.Exit(pgtest.Main(m)) }

// noSubscriber registers no test subscriber of MessagePosted.
func noSubscriber(*txn.Registry, *mw.Recorder) error { return nil }

// wantCode fails unless err is an errs error with code.
func wantCode(t *testing.T, err error, code errs.Code) {
	t.Helper()
	if e, ok := errs.As(err); !ok || e.Code != code {
		t.Fatalf("error %v, want %s", err, code)
	}
}

// TestNewPersonal_Refuses: a missing collaborator and a limit below its minimum are refused.
func TestNewPersonal_Refuses(t *testing.T) {
	t.Parallel()
	if _, err := app.NewPersonal(app.PersonalConfig{MaxContentBytes: 1, EventPayloadMaxBytes: 1}, app.PersonalDeps{}); err == nil {
		t.Fatal("no collaborators accepted")
	}
}

// TestPersonal_UseCases (M1-61): the use cases called directly with the caller in the context:
// read states, drafts, settings and the sidebar of a person, and the refusals of the ports.
func TestPersonal_UseCases(t *testing.T) {
	t.Parallel()
	w := mw.NewPersonal(t, noSubscriber)
	space, ch, main := w.TextChannel()
	forum := w.Channel(space, "forum", "ideas")
	alex := w.Person("alex", false)
	w.Grant("user", alex, space, "write")
	w.Send("owner", main, "one", http.StatusCreated)
	w.Send("owner", main, "two @alex", http.StatusCreated)
	ctx := actor.WithActor(context.Background(), w.Actor("alex"))
	p := w.Personal

	// Read states.
	rs, err := p.Advance(ctx, syncmod.AdvanceReadState{ThreadID: main, LastReadSeq: 1})
	if err != nil || rs.Version != 1 || rs.MentionCount != 1 || rs.ChannelID != ch {
		t.Fatalf("advance %+v %v", rs, err)
	}
	_, err = p.Advance(ctx, syncmod.AdvanceReadState{ThreadID: main, LastReadSeq: 3})
	wantCode(t, err, errs.CodeValidationFailed)
	_, err = p.Advance(ctx, syncmod.AdvanceReadState{ThreadID: ids.New(), LastReadSeq: 1})
	wantCode(t, err, errs.CodeNotFound)
	_, err = p.MarkUnread(ctx, syncmod.MarkUnread{ThreadID: main})
	wantCode(t, err, errs.CodePreconditionRequired)
	_, err = p.MarkUnread(ctx, syncmod.MarkUnread{ThreadID: main, IfMatch: syncmod.Precondition{Present: true}})
	wantCode(t, err, errs.CodeVersionConflict)
	if rs, err = p.MarkUnread(ctx, syncmod.MarkUnread{ThreadID: main, IfMatch: syncmod.Precondition{Present: true, Valid: true, Version: 1}}); err != nil ||
		rs.LastReadSeq != 0 || rs.Version != 2 || rs.MentionCount != 1 {
		t.Fatalf("mark unread %+v %v", rs, err)
	}
	list, err := p.ListForChannel(ctx, ch)
	if err != nil || len(list) != 1 || list[0].MentionCount != 1 {
		t.Fatalf("list %+v %v", list, err)
	}
	if list, err := p.ListForChannel(ctx, ids.New()); err != nil || len(list) != 0 {
		t.Fatalf("unknown channel %+v %v", list, err)
	}

	// Drafts.
	d := p.Drafts()
	base := int64(0)
	res, err := d.PutThreadDraft(ctx, syncmod.PutDraft{ThreadID: main, ContentMD: "x", BaseVersion: &base})
	if err != nil || res.Draft.Version != 1 || res.OverwroteNewer {
		t.Fatalf("put %+v %v", res, err)
	}
	// The same device again without a base: its own write is never overwrote_newer
	// (FIX-draft-conflict-banner).
	if res, err = d.PutThreadDraft(ctx, syncmod.PutDraft{ThreadID: main, ContentMD: "y"}); err != nil || res.OverwroteNewer || res.Draft.Version != 2 {
		t.Fatalf("put without base %+v %v", res, err)
	}
	bad := ids.New()
	_, err = d.PutThreadDraft(ctx, syncmod.PutDraft{ThreadID: main, ContentMD: "y", ReplyToID: &bad})
	wantCode(t, err, errs.CodeValidationFailed)
	if res, err = d.PutChannelDraft(ctx, syncmod.PutDraft{ChannelID: forum, ContentMD: "post"}); err != nil || !res.Draft.ThreadID.IsZero() {
		t.Fatalf("channel draft %+v %v", res, err)
	}
	_, err = d.PutChannelDraft(ctx, syncmod.PutDraft{ChannelID: ch, ContentMD: "post"})
	wantCode(t, err, errs.CodeValidationFailed)
	if drafts, err := d.List(ctx); err != nil || len(drafts) != 2 {
		t.Fatalf("drafts %+v %v", drafts, err)
	}
	if res, err = d.PutChannelDraft(ctx, syncmod.PutDraft{ChannelID: forum, ContentMD: " "}); err != nil || res.Draft.Version != 2 {
		t.Fatalf("empty channel draft %+v %v", res, err)
	}
	if err := d.DeleteThreadDraft(ctx, main); err != nil {
		t.Fatal(err)
	}
	if err := d.DeleteChannelDraft(ctx, forum); err != nil {
		t.Fatal(err)
	}
	wantCode(t, d.DeleteThreadDraft(ctx, ids.New()), errs.CodeNotFound)
	wantCode(t, d.DeleteChannelDraft(ctx, ids.New()), errs.CodeNotFound)

	// Settings and sidebar.
	s := p.Settings()
	if st, err := s.Put(ctx, "appearance.theme", json.RawMessage(`"dark"`)); err != nil || st.Version != 1 {
		t.Fatalf("put %+v %v", st, err)
	}
	if st, err := s.Put(ctx, "appearance.theme", json.RawMessage(`"dark"`)); err != nil || st.Version != 1 {
		t.Fatalf("same value %+v %v", st, err)
	}
	_, err = s.Put(ctx, "appearance.font", json.RawMessage(`"x"`))
	wantCode(t, err, errs.CodeNotFound)
	if list, err := s.List(ctx); err != nil || len(list) != 1 {
		t.Fatalf("settings %+v %v", list, err)
	}
	if err := s.Delete(ctx, "appearance.theme"); err != nil {
		t.Fatal(err)
	}
	wantCode(t, s.Delete(ctx, "appearance.font"), errs.CodeNotFound)
	if side, err := s.ListSidebar(ctx); err != nil || len(side) != 0 {
		t.Fatalf("sidebar %+v %v", side, err)
	}

	// Without a caller authorize refuses every operation.
	anon := context.Background()
	if _, err := p.ListForChannel(anon, ch); err == nil {
		t.Fatal("anonymous read states")
	}
	if _, err := d.List(anon); err == nil {
		t.Fatal("anonymous drafts")
	}
	if _, err := s.List(anon); err == nil {
		t.Fatal("anonymous settings")
	}
	if _, err := s.ListSidebar(anon); err == nil {
		t.Fatal("anonymous sidebar")
	}
}
