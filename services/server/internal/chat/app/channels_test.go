// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"errors"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/chat/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// channel creates a channel of kind in space as the owner.
func (e *env) channel(t *testing.T, space ids.ID, kind, name string) chat.Channel {
	t.Helper()
	c, err := e.svc.CreateChannel(context.Background(), e.owner, space, chat.CreateChannelCmd{Kind: kind, Name: name})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func boolPtr(b bool) *bool { return &b }

// TestNewChannelConfig: the channel limits and the backfill default are checked.
func TestNewChannelConfig(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	deps := app.Deps{Store: e.store, UoW: e.uow, Authorizer: e.az, Rights: e.rights, Audit: e.rec, Events: e.events}
	for _, c := range []app.Config{
		{SpaceNameMaxChars: 1, ChannelNameMaxChars: 0, ChannelTopicMaxChars: 1, TagNameMaxChars: 1, HistoryBackfillLimitMax: 1},
		{SpaceNameMaxChars: 1, ChannelNameMaxChars: 1, ChannelTopicMaxChars: 1, TagNameMaxChars: 1, HistoryBackfillLimitDefault: 2, HistoryBackfillLimitMax: 1},
		{SpaceNameMaxChars: 1, ChannelNameMaxChars: 1, ChannelTopicMaxChars: 1, TagNameMaxChars: 1, HistoryBackfillLimitDefault: -1},
	} {
		if _, err := app.New(c, deps); err == nil {
			t.Errorf("config %+v accepted", c)
		}
	}
}

// TestChannelUseCases: create (text with main thread, forum without), order, read, list with
// the scope, update, move and archive over the fakes, with the events and audit entries.
func TestChannelUseCases(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ctx := context.Background()
	sp := e.create(t, "Support", ids.ID{})
	a := e.channel(t, sp.ID, "text", "alpha")
	if a.MainThreadID.IsZero() || a.Operator.Mode != "inherit" || a.Version != 1 {
		t.Fatalf("text channel %+v", a)
	}
	f := e.channel(t, sp.ID, "forum", "forum")
	if !f.MainThreadID.IsZero() {
		t.Fatalf("forum %+v", f)
	}
	first, err := e.svc.CreateChannel(ctx, e.owner, sp.ID, chat.CreateChannelCmd{Kind: "text", Name: "first", Topic: ptr("x")})
	if err != nil {
		t.Fatal(err)
	}
	_, err = e.svc.CreateChannel(ctx, e.owner, sp.ID, chat.CreateChannelCmd{Kind: "text", Name: "mid", AfterChannelID: a.ID})
	if err != nil {
		t.Fatal(err)
	}
	_, err = e.svc.CreateChannel(ctx, e.owner, sp.ID, chat.CreateChannelCmd{Kind: "text", Name: "bad", AfterChannelID: ids.New()})
	wantCode(t, err, errs.CodeValidationFailed)
	_, err = e.svc.CreateChannel(ctx, e.owner, sp.ID, chat.CreateChannelCmd{Kind: "text", Name: "ALPHA"})
	wantCode(t, err, errs.CodeConflict)
	_, err = e.svc.CreateChannel(ctx, e.owner, sp.ID, chat.CreateChannelCmd{Kind: "text", Name: "toolongname"})
	wantCode(t, err, errs.CodeValidationFailed)
	_, err = e.svc.CreateChannel(ctx, e.owner, sp.ID, chat.CreateChannelCmd{Kind: "text", Name: "x", Topic: ptr("waytoolongtopic")})
	wantCode(t, err, errs.CodeValidationFailed)
	_, err = e.svc.CreateChannel(ctx, e.owner, ids.New(), chat.CreateChannelCmd{Kind: "text", Name: "x"})
	wantCode(t, err, errs.CodeNotFound)

	v, err := e.svc.GetChannel(ctx, e.owner, a.ID)
	if err != nil || v.Level != "trigger" || len(v.Capabilities) != 1 {
		t.Fatalf("view %+v, %v", v, err)
	}
	_, err = e.svc.GetChannel(ctx, e.owner, ids.New())
	wantCode(t, err, errs.CodeNotFound)
	e.az.scope = authorize.Scope{ChannelIDs: []ids.ID{a.ID}}
	list, err := e.svc.ListSpaceChannels(ctx, e.owner, sp.ID)
	if err != nil || len(list) != 1 || list[0].Channel.ID != a.ID {
		t.Fatalf("list %+v, %v", list, err)
	}
	e.az.scope = authorize.Scope{FullyVisibleSpaceIDs: []ids.ID{sp.ID}}
	e.az.rules[authorize.ActionChannelRead] = func(_ authorize.Principal, r authorize.Resource) authorize.Decision {
		if r.ID == f.ID {
			return authorize.Decision{Outcome: authorize.OutcomeNotFound}
		}
		return allow
	}
	if list, err = e.svc.ListSpaceChannels(ctx, e.owner, sp.ID); err != nil || len(list) != 3 || list[0].Channel.ID != a.ID || list[2].Channel.ID != first.ID {
		t.Fatalf("list after filter %+v, %v", list, err)
	}
	delete(e.az.rules, authorize.ActionChannelRead)

	_, err = e.svc.UpdateChannel(ctx, e.owner, a.ID, chat.UpdateChannelCmd{Name: ptr("beta")})
	wantCode(t, err, errs.CodePreconditionRequired)
	_, err = e.svc.UpdateChannel(ctx, e.owner, a.ID, chat.UpdateChannelCmd{Name: ptr("beta"), IfMatch: chat.Precondition{Present: true, Valid: true, Version: 9}})
	wantCode(t, err, errs.CodeVersionConflict)
	match := chat.Precondition{Present: true, Valid: true, Version: 1}
	_, err = e.svc.UpdateChannel(ctx, e.owner, a.ID, chat.UpdateChannelCmd{IfMatch: match})
	wantCode(t, err, errs.CodeValidationFailed)
	_, err = e.svc.UpdateChannel(ctx, e.owner, a.ID, chat.UpdateChannelCmd{Name: ptr(" "), IfMatch: match})
	wantCode(t, err, errs.CodeValidationFailed)
	_, err = e.svc.UpdateChannel(ctx, e.owner, a.ID, chat.UpdateChannelCmd{Topic: ptr("waytoolongtopic"), IfMatch: match})
	wantCode(t, err, errs.CodeValidationFailed)
	same, err := e.svc.UpdateChannel(ctx, e.owner, a.ID, chat.UpdateChannelCmd{Name: ptr("alpha"), IfMatch: match})
	if err != nil || same.Version != 1 {
		t.Fatalf("no change %+v, %v", same, err)
	}
	up, err := e.svc.UpdateChannel(ctx, e.owner, a.ID, chat.UpdateChannelCmd{Name: ptr("beta"), Topic: ptr("t"), IfMatch: match})
	if err != nil || up.Version != 2 || up.Name != "beta" {
		t.Fatalf("update %+v, %v", up, err)
	}

	moved, err := e.svc.MoveChannel(ctx, e.owner, a.ID, chat.MovePosition{})
	if err != nil || moved.Version != 3 {
		t.Fatalf("move %+v, %v", moved, err)
	}
	_, err = e.svc.MoveChannel(ctx, e.owner, a.ID, chat.MovePosition{AfterID: f.ID, BeforeID: first.ID})
	wantCode(t, err, errs.CodeValidationFailed)
	_, err = e.svc.MoveChannel(ctx, e.owner, a.ID, chat.MovePosition{AfterID: ids.New()})
	wantCode(t, err, errs.CodeValidationFailed)
	_, err = e.svc.MoveChannel(ctx, e.owner, ids.New(), chat.MovePosition{})
	wantCode(t, err, errs.CodeNotFound)
	e.deny(authorize.ActionChannelReorder, authorize.Decision{Outcome: authorize.OutcomeForbidden, Reason: "capability_missing"})
	_, err = e.svc.MoveChannel(ctx, e.owner, a.ID, chat.MovePosition{})
	wantCode(t, err, errs.CodeForbidden)
	delete(e.az.rules, authorize.ActionChannelReorder)

	_, err = e.svc.ArchiveChannel(ctx, e.owner, a.ID, " ")
	wantCode(t, err, errs.CodeValidationFailed)
	arch, err := e.svc.ArchiveChannel(ctx, e.owner, a.ID, "Old")
	if err != nil || arch.ArchivedAt.IsZero() || arch.Version != 4 {
		t.Fatalf("archive %+v, %v", arch, err)
	}
	again, err := e.svc.ArchiveChannel(ctx, e.owner, a.ID, "Again")
	if err != nil || again.Version != 4 {
		t.Fatalf("archive again %+v, %v", again, err)
	}
	_, err = e.svc.UpdateChannel(ctx, e.owner, a.ID, chat.UpdateChannelCmd{Name: ptr("x"), IfMatch: chat.Precondition{Present: true, Valid: true, Version: 4}})
	wantCode(t, err, errs.CodeConflict)
	_, err = e.svc.MoveChannel(ctx, e.owner, a.ID, chat.MovePosition{})
	wantCode(t, err, errs.CodeConflict)
	var created, thread, updated, archived int
	for _, ev := range e.events.evs {
		switch ev.Type {
		case "channel.created":
			created++
		case "thread.created":
			thread++
		case "channel.updated":
			updated++
		case "channel.archived":
			archived++
		}
	}
	if created != 4 || thread != 3 || updated != 2 || archived != 1 {
		t.Fatalf("events created %d thread %d updated %d archived %d", created, thread, updated, archived)
	}
	if _, err := e.svc.ArchiveSpace(ctx, e.owner, sp.ID, "Done"); err != nil {
		t.Fatal(err)
	}
	_, err = e.svc.CreateChannel(ctx, e.owner, sp.ID, chat.CreateChannelCmd{Kind: "text", Name: "late"})
	wantCode(t, err, errs.CodeConflict)
}

// TestForumTagUseCases: create, list, update, move and delete over the fakes, with the not_a_forum,
// not_found, conflict and archived branches.
func TestForumTagUseCases(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ctx := context.Background()
	sp := e.create(t, "Support", ids.ID{})
	text := e.channel(t, sp.ID, "text", "tickets")
	f := e.channel(t, sp.ID, "forum", "bugs")
	_, err := e.svc.CreateForumTag(ctx, e.owner, text.ID, chat.CreateForumTagCmd{Name: "bug"})
	wantCode(t, err, errs.CodeValidationFailed)
	_, err = e.svc.CreateForumTag(ctx, e.owner, f.ID, chat.CreateForumTagCmd{Name: "bug", Color: ptr("#fff")})
	wantCode(t, err, errs.CodeValidationFailed)
	_, err = e.svc.CreateForumTag(ctx, e.owner, f.ID, chat.CreateForumTagCmd{Name: "toolongname"})
	wantCode(t, err, errs.CodeValidationFailed)
	bug, err := e.svc.CreateForumTag(ctx, e.owner, f.ID, chat.CreateForumTagCmd{Name: "bug", Color: ptr("moss"), Restricted: boolPtr(true)})
	if err != nil || !bug.Restricted || bug.Color != "moss" {
		t.Fatalf("tag %+v, %v", bug, err)
	}
	idea, err := e.svc.CreateForumTag(ctx, e.owner, f.ID, chat.CreateForumTagCmd{Name: "idea", AfterTagID: bug.ID})
	if err != nil {
		t.Fatal(err)
	}
	_, err = e.svc.CreateForumTag(ctx, e.owner, f.ID, chat.CreateForumTagCmd{Name: "x", AfterTagID: ids.New()})
	wantCode(t, err, errs.CodeValidationFailed)
	_, err = e.svc.CreateForumTag(ctx, e.owner, f.ID, chat.CreateForumTagCmd{Name: "BUG"})
	wantCode(t, err, errs.CodeConflict)
	if tags, err := e.svc.ListForumTags(ctx, e.owner, f.ID); err != nil || len(tags) != 2 || tags[0].ID != bug.ID {
		t.Fatalf("tags %+v, %v", tags, err)
	}
	_, err = e.svc.ListForumTags(ctx, e.owner, ids.New())
	wantCode(t, err, errs.CodeNotFound)
	_, err = e.svc.UpdateForumTag(ctx, e.owner, f.ID, bug.ID, chat.UpdateForumTagCmd{})
	wantCode(t, err, errs.CodeValidationFailed)
	_, err = e.svc.UpdateForumTag(ctx, e.owner, f.ID, bug.ID, chat.UpdateForumTagCmd{Name: ptr(" ")})
	wantCode(t, err, errs.CodeValidationFailed)
	_, err = e.svc.UpdateForumTag(ctx, e.owner, f.ID, ids.New(), chat.UpdateForumTagCmd{Name: ptr("x")})
	wantCode(t, err, errs.CodeNotFound)
	_, err = e.svc.UpdateForumTag(ctx, e.owner, text.ID, bug.ID, chat.UpdateForumTagCmd{Name: ptr("x")})
	wantCode(t, err, errs.CodeValidationFailed)
	if got, err := e.svc.UpdateForumTag(ctx, e.owner, f.ID, bug.ID, chat.UpdateForumTagCmd{Name: ptr("bugs"), ClearColor: true}); err != nil || got.Color != "" {
		t.Fatalf("update %+v, %v", got, err)
	}
	if _, err := e.svc.UpdateForumTag(ctx, e.owner, f.ID, bug.ID, chat.UpdateForumTagCmd{Name: ptr("bugs")}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.MoveForumTag(ctx, e.owner, f.ID, idea.ID, chat.MovePosition{}); err != nil {
		t.Fatal(err)
	}
	_, err = e.svc.MoveForumTag(ctx, e.owner, f.ID, idea.ID, chat.MovePosition{AfterID: idea.ID})
	wantCode(t, err, errs.CodeValidationFailed)
	_, err = e.svc.MoveForumTag(ctx, e.owner, f.ID, ids.New(), chat.MovePosition{})
	wantCode(t, err, errs.CodeNotFound)
	if err := e.svc.DeleteForumTag(ctx, e.owner, f.ID, idea.ID); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.DeleteForumTag(ctx, e.owner, f.ID, idea.ID); err != nil {
		t.Fatal(err)
	}
	wantCode(t, e.svc.DeleteForumTag(ctx, e.owner, f.ID, ids.New()), errs.CodeNotFound)
	e.deny(authorize.ActionForumTagManage, authorize.Decision{Outcome: authorize.OutcomeForbidden, Reason: "capability_missing"})
	wantCode(t, e.svc.DeleteForumTag(ctx, e.owner, f.ID, bug.ID), errs.CodeForbidden)
	delete(e.az.rules, authorize.ActionForumTagManage)
	if _, err := e.svc.ArchiveChannel(ctx, e.owner, f.ID, "Old"); err != nil {
		t.Fatal(err)
	}
	wantCode(t, e.svc.DeleteForumTag(ctx, e.owner, f.ID, bug.ID), errs.CodeConflict)
}

// TestChannelStoreErrorsPass: a store error reaches the caller unchanged.
func TestChannelStoreErrorsPass(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ctx := context.Background()
	sp := e.create(t, "Support", ids.ID{})
	c := e.channel(t, sp.ID, "forum", "bugs")
	boom := errs.New(errs.CodeInternalError, "boom")
	e.store.fail = boom
	checks := []error{
		func() error { _, err := e.svc.GetChannel(ctx, e.owner, c.ID); return err }(),
		func() error { _, err := e.svc.ListSpaceChannels(ctx, e.owner, sp.ID); return err }(),
		func() error {
			_, err := e.svc.CreateChannel(ctx, e.owner, sp.ID, chat.CreateChannelCmd{Kind: "text", Name: "x"})
			return err
		}(),
		func() error { _, err := e.svc.ArchiveChannel(ctx, e.owner, c.ID, "Old"); return err }(),
		func() error { _, err := e.svc.ListForumTags(ctx, e.owner, c.ID); return err }(),
	}
	for i, err := range checks {
		if !errors.Is(err, boom) {
			t.Errorf("call %d: %v", i, err)
		}
	}
	e.store.fail = nil
	e.az.err = boom
	if _, err := e.svc.ListSpaceChannels(ctx, e.owner, sp.ID); !errors.Is(err, boom) {
		t.Fatalf("authorize error %v", err)
	}
	e.az.err = nil
	e.rights.err = boom
	if _, err := e.svc.GetChannel(ctx, e.owner, c.ID); !errors.Is(err, boom) {
		t.Fatalf("rights error %v", err)
	}
}
