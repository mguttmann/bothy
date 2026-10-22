// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/chat/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// The service is the channel facts port of the trigger decision (M2-05).
var _ chat.TriggerFacts = (*app.Service)(nil)

// TestThreadsLocate (M2-05): Locate answers the routing data Info answers for every known thread
// of the list, ascending by id, leaves unknown ids out, asks no store for an empty list and
// authorizes nothing.
func TestThreadsLocate(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ctx := context.Background()
	sp := e.create(t, "Locate", ids.ID{})
	text := e.channel(t, sp.ID, "text", "general")
	forum := e.channel(t, sp.ID, "forum", "ideas")
	post, err := e.newThread(e.owner, chat.CreateThreadCmd{ChannelID: forum.ID, Title: ptr("Plan")})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.appendTo(post.ID, false); err != nil {
		t.Fatal(err)
	}
	e.store.mu.Lock()
	c := e.store.channels[text.ID]
	c.ArchivedAt = time.Date(2026, 10, 15, 9, 0, 0, 0, time.UTC)
	e.store.channels[text.ID] = c
	e.store.mu.Unlock()
	// Locate authorizes nothing: a refusing authorizer changes no answer.
	e.az.err = errs.New(errs.CodeForbidden, "authorize called")

	none, err := e.svc.Locate(ctx, nil)
	if err != nil || none == nil || len(none) != 0 {
		t.Fatalf("no threads: %v %v", none, err)
	}
	got, err := e.svc.Locate(ctx, []ids.ID{post.ID, ids.New(), text.MainThreadID})
	if err != nil || len(got) != 2 {
		t.Fatalf("locate %+v, %v", got, err)
	}
	if got[0].ID.String() > got[1].ID.String() {
		t.Fatalf("not ascending: %+v", got)
	}
	e.az.err = nil
	for _, li := range got {
		want, err := e.svc.Info(ctx, li.ID)
		if err != nil || li != want {
			t.Fatalf("locate %+v, info %+v, %v", li, want, err)
		}
	}
	byID := map[ids.ID]chat.ThreadInfo{got[0].ID: got[0], got[1].ID: got[1]}
	if p := byID[post.ID]; p.ChannelKind != "forum" || p.Kind != "thread" || p.SpaceID != sp.ID || p.LastMessageSeq != 1 || p.ChannelArchived {
		t.Fatalf("post %+v", p)
	}
	if m := byID[text.MainThreadID]; m.Kind != "main" || !m.ChannelArchived {
		t.Fatalf("main %+v", m)
	}
	e.store.fail = errs.New(errs.CodeInternalError, "boom")
	_, err = e.svc.Locate(ctx, []ids.ID{post.ID})
	wantCode(t, err, errs.CodeInternalError)
}

// TestTriggerChannel (M2-05): the channel facts carry kind, space, the five switches and the
// effective operator after inheritance from the space; an unknown channel is not_found; nothing is
// authorized.
func TestTriggerChannel(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ctx := context.Background()
	sp := e.create(t, "Trigger", ids.ID{})
	text := e.channel(t, sp.ID, "text", "general")
	forum := e.channel(t, sp.ID, "forum", "ideas")
	spaceOp, ownOp := ids.New(), ids.New()
	e.store.mu.Lock()
	s := e.store.spaces[sp.ID]
	s.DefaultOperatorAgentID = spaceOp
	e.store.spaces[sp.ID] = s
	f := e.store.channels[forum.ID]
	f.OperatorInherit, f.OperatorAgentID = false, ownOp
	f.RequireMention, f.ThreadRequireMention, f.FreeResponse, f.AutoThread, f.AllowMentionedAgents = false, true, true, false, true
	e.store.channels[forum.ID] = f
	e.store.mu.Unlock()
	e.az.err = errs.New(errs.CodeForbidden, "authorize called")

	got, err := e.svc.TriggerChannel(ctx, text.ID)
	want := chat.TriggerChannel{ChannelID: text.ID, SpaceID: sp.ID, Kind: "text", EffectiveOperatorID: spaceOp, MainThreadID: text.MainThreadID,
		RequireMention: true, AutoThread: true, SettingsVersion: e.store.channels[text.ID].SettingsVersion}
	if err != nil || got != want {
		t.Fatalf("text %+v, %v; want %+v", got, err, want)
	}
	got, err = e.svc.TriggerChannel(ctx, forum.ID)
	want = chat.TriggerChannel{ChannelID: forum.ID, SpaceID: sp.ID, Kind: "forum", EffectiveOperatorID: ownOp,
		ThreadRequireMention: true, FreeResponse: true, AllowMentionedAgents: true, SettingsVersion: f.SettingsVersion}
	if err != nil || got != want {
		t.Fatalf("forum %+v, %v; want %+v", got, err, want)
	}
	_, err = e.svc.TriggerChannel(ctx, ids.New())
	wantCode(t, err, errs.CodeNotFound)
	e.az.err = nil
	e.store.fail = errs.New(errs.CodeInternalError, "boom")
	_, err = e.svc.TriggerChannel(ctx, text.ID)
	wantCode(t, err, errs.CodeInternalError)
}
