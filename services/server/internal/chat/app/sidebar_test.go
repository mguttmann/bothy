// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// wantField fails unless err is validation_failed with code on pointer.
func wantField(t *testing.T, err error, pointer, code string) {
	t.Helper()
	e, ok := errs.As(err)
	if !ok || e.Code != errs.CodeValidationFailed || len(e.Fields) != 1 || e.Fields[0].Pointer != pointer || e.Fields[0].Code != code {
		t.Fatalf("error %v, want validation_failed %s on %s", err, code, pointer)
	}
}

// TestSidebarChannelOrder (M4-51): putMySidebarSpace keeps the stored channel order when the
// member is absent, replaces it with a list and stores the default order (nil) for an empty list;
// the form rules (duplicate, sidebar.order_max_items) answer 400 on /channel_order; the IDs are
// not checked against existing rows (D-M4-51-8); sidebar.updated carries the list.
func TestSidebarChannelOrder(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ctx := context.Background()
	sp := e.create(t, "Dev", ids.ID{})
	a, b, unknown := ids.New(), ids.New(), ids.New()
	order := []ids.ID{b, a, unknown}
	st, err := e.svc.PutSidebarSpace(ctx, e.owner, sp.ID, chat.SidebarPutCmd{SortKey: ptr("a0V"), ChannelOrder: &order})
	if err != nil || !slices.Equal(st.ChannelOrder, order) || st.SortKey != "a0V" {
		t.Fatalf("set %+v, %v", st, err)
	}
	var payload struct {
		State struct {
			ChannelOrder []string `json:"channel_order"`
		} `json:"state"`
	}
	if err := json.Unmarshal(e.events.evs[len(e.events.evs)-1].Payload, &payload); err != nil ||
		!slices.Equal(payload.State.ChannelOrder, []string{b.String(), a.String(), unknown.String()}) {
		t.Fatalf("payload %+v, %v", payload, err)
	}
	st, err = e.svc.PutSidebarSpace(ctx, e.owner, sp.ID, chat.SidebarPutCmd{Collapsed: true})
	if err != nil || !slices.Equal(st.ChannelOrder, order) || st.SortKey != "a0V" || st.Version != 2 {
		t.Fatalf("keep %+v, %v", st, err)
	}
	empty := []ids.ID{}
	st, err = e.svc.PutSidebarSpace(ctx, e.owner, sp.ID, chat.SidebarPutCmd{ChannelOrder: &empty})
	if err != nil || st.ChannelOrder != nil || st.SortKey != "a0V" {
		t.Fatalf("clear %+v, %v", st, err)
	}
	payload.State.ChannelOrder = nil
	if err := json.Unmarshal(e.events.evs[len(e.events.evs)-1].Payload, &payload); err != nil || payload.State.ChannelOrder != nil {
		t.Fatalf("cleared payload %+v, %v", payload, err)
	}
	dup := []ids.ID{a, a}
	_, err = e.svc.PutSidebarSpace(ctx, e.owner, sp.ID, chat.SidebarPutCmd{ChannelOrder: &dup})
	wantField(t, err, "/channel_order", "duplicate")
	many := []ids.ID{ids.New(), ids.New(), ids.New(), ids.New()}
	_, err = e.svc.PutSidebarSpace(ctx, e.owner, sp.ID, chat.SidebarPutCmd{ChannelOrder: &many})
	wantField(t, err, "/channel_order", "limit_reached")
}

// TestSidebarChannel (M4-51): putMySidebarChannel is for people only (agents and apps 403
// subject_kind_not_allowed before any lookup), a channel the caller cannot read and an unknown
// one are 404, the list is stored with version + 1 and the device, the form rules answer 400 on
// /thread_order, and sidebar_channel.updated goes to the person without a resource version.
func TestSidebarChannel(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ctx := context.Background()
	sp := e.create(t, "Dev", ids.ID{})
	forum := e.channel(t, sp.ID, "forum", "Projects")
	for _, kind := range []actor.Kind{actor.KindAgent, actor.KindApp} {
		_, err := e.svc.PutSidebarChannel(ctx, actor.Actor{ID: ids.New(), Kind: kind}, forum.ID, chat.SidebarChannelPutCmd{})
		wantCode(t, err, errs.CodeForbidden)
	}
	p1, p2 := ids.New(), ids.New()
	st, err := e.svc.PutSidebarChannel(ctx, e.owner, forum.ID, chat.SidebarChannelPutCmd{ThreadOrder: []ids.ID{p2, p1}})
	if err != nil || st.ChannelID != forum.ID || !slices.Equal(st.ThreadOrder, []ids.ID{p2, p1}) || st.Version != 1 || st.UpdatedByDeviceID != e.owner.DeviceID {
		t.Fatalf("put %+v, %v", st, err)
	}
	ev := e.events.evs[len(e.events.evs)-1]
	if ev.Type != "sidebar_channel.updated" || ev.AudiencePrincipalID != e.owner.ID || ev.ResourceKind != "channel" || ev.ResourceID != forum.ID ||
		ev.ResourceVersion != 0 || ev.ReceiveAction != "self" || ev.SpaceID != sp.ID || ev.ChannelID != forum.ID {
		t.Fatalf("event %+v", ev)
	}
	var payload struct {
		State struct {
			ChannelID   string   `json:"channel_id"`
			ThreadOrder []string `json:"thread_order"`
			Version     int64    `json:"version"`
			Device      string   `json:"updated_by_device_id"`
		} `json:"state"`
	}
	if err := json.Unmarshal(ev.Payload, &payload); err != nil || payload.State.ChannelID != forum.ID.String() ||
		!slices.Equal(payload.State.ThreadOrder, []string{p2.String(), p1.String()}) || payload.State.Version != 1 || payload.State.Device != e.owner.DeviceID.String() {
		t.Fatalf("payload %+v, %v", payload, err)
	}
	st, err = e.svc.PutSidebarChannel(ctx, e.owner, forum.ID, chat.SidebarChannelPutCmd{})
	if err != nil || len(st.ThreadOrder) != 0 || st.ThreadOrder == nil || st.Version != 2 {
		t.Fatalf("empty %+v, %v", st, err)
	}
	payload.State.ThreadOrder = nil
	if err := json.Unmarshal(e.events.evs[len(e.events.evs)-1].Payload, &payload); err != nil || payload.State.ThreadOrder == nil {
		t.Fatalf("empty payload must carry []: %+v, %v", payload, err)
	}
	_, err = e.svc.PutSidebarChannel(ctx, e.owner, forum.ID, chat.SidebarChannelPutCmd{ThreadOrder: []ids.ID{p1, p1}})
	wantField(t, err, "/thread_order", "duplicate")
	_, err = e.svc.PutSidebarChannel(ctx, e.owner, forum.ID, chat.SidebarChannelPutCmd{ThreadOrder: []ids.ID{ids.New(), ids.New(), ids.New(), ids.New()}})
	wantField(t, err, "/thread_order", "limit_reached")
	_, err = e.svc.PutSidebarChannel(ctx, e.owner, ids.New(), chat.SidebarChannelPutCmd{})
	wantCode(t, err, errs.CodeNotFound)
	n := len(e.events.evs)
	e.deny(authorize.ActionSidebarUpdate, authorize.Decision{Outcome: authorize.OutcomeNotFound, Reason: "no_grant"})
	_, err = e.svc.PutSidebarChannel(ctx, e.owner, forum.ID, chat.SidebarChannelPutCmd{})
	wantCode(t, err, errs.CodeNotFound)
	if len(e.events.evs) != n {
		t.Fatal("a refused write appended an event")
	}
	_, err = e.svc.PutSidebarChannel(actor.WithActor(ctx, actor.Actor{ID: ids.New(), Kind: actor.KindUser}), e.owner, forum.ID, chat.SidebarChannelPutCmd{})
	wantCode(t, err, errs.CodeInternalError)
}

// TestSidebarChannelList (M4-51): the channel rows are read with the scope of channel.read and
// each row is kept only when the caller may read the channel; an archived channel and a channel
// of an archived space are left out; agents and apps get no rows; errors reach the caller.
func TestSidebarChannelList(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ctx := context.Background()
	sp, other := e.create(t, "Dev", ids.ID{}), e.create(t, "Ops", ids.ID{})
	f1, f2, f3 := e.channel(t, sp.ID, "forum", "One"), e.channel(t, sp.ID, "forum", "Two"), e.channel(t, other.ID, "forum", "Three")
	for _, c := range []ids.ID{f1.ID, f2.ID, f3.ID} {
		if _, err := e.svc.PutSidebarChannel(ctx, e.owner, c, chat.SidebarChannelPutCmd{ThreadOrder: []ids.ID{ids.New()}}); err != nil {
			t.Fatal(err)
		}
	}
	e.az.scope = authorize.Scope{InstanceWide: true}
	list, err := e.svc.ListSidebarChannelStates(ctx, e.owner)
	if err != nil || len(list) != 3 {
		t.Fatalf("all %+v, %v", list, err)
	}
	e.az.scope = authorize.Scope{ChannelIDs: []ids.ID{f1.ID, f3.ID}}
	if list, err := e.svc.ListSidebarChannelStates(ctx, e.owner); err != nil || len(list) != 2 {
		t.Fatalf("scoped %+v, %v", list, err)
	}
	e.az.scope = authorize.Scope{FullyVisibleSpaceIDs: []ids.ID{sp.ID}}
	if list, err := e.svc.ListSidebarChannelStates(ctx, e.owner); err != nil || len(list) != 2 {
		t.Fatalf("fully visible space %+v, %v", list, err)
	}
	e.az.scope = authorize.Scope{InstanceWide: true}
	e.az.rules[authorize.ActionChannelRead] = func(_ authorize.Principal, r authorize.Resource) authorize.Decision {
		if r.ID == f2.ID {
			return authorize.Decision{Outcome: authorize.OutcomeNotFound, Reason: "no_grant"}
		}
		return allow
	}
	list, err = e.svc.ListSidebarChannelStates(ctx, e.owner)
	if err != nil || len(list) != 2 || slices.ContainsFunc(list, func(s chat.SidebarChannelState) bool { return s.ChannelID == f2.ID }) {
		t.Fatalf("filtered %+v, %v", list, err)
	}
	delete(e.az.rules, authorize.ActionChannelRead)
	if _, err := e.svc.ArchiveSpace(ctx, e.owner, other.ID, "Done"); err != nil {
		t.Fatal(err)
	}
	if list, err := e.svc.ListSidebarChannelStates(ctx, e.owner); err != nil || len(list) != 2 {
		t.Fatalf("archived space %+v, %v", list, err)
	}
	for _, kind := range []actor.Kind{actor.KindAgent, actor.KindApp} {
		if list, err := e.svc.ListSidebarChannelStates(ctx, actor.Actor{ID: ids.New(), Kind: kind}); err != nil || len(list) != 0 {
			t.Fatalf("%s %+v, %v", kind, list, err)
		}
	}
	e.store.fail = errors.New("database down")
	if _, err := e.svc.ListSidebarChannelStates(ctx, e.owner); err == nil {
		t.Fatal("store error swallowed")
	}
	e.store.fail = nil
	e.az.err = errors.New("rights down")
	if _, err := e.svc.ListSidebarChannelStates(ctx, e.owner); err == nil {
		t.Fatal("scope error swallowed")
	}
}
