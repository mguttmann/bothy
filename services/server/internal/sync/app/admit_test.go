// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"encoding/json"
	"slices"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/sync"
)

// revokedAuthz answers every receive action with unauthenticated session_revoked.
type revokedAuthz struct{ catchAuthz }

func (*revokedAuthz) Authorize(context.Context, authorize.Principal, authorize.Action, authorize.Resource) (authorize.Decision, error) {
	return authorize.Decision{Outcome: authorize.OutcomeUnauthenticated, Reason: "session_revoked"}, nil
}

// TestAdmit_SingleRuleForGateway: Admit is the admission rule of the page (M1-65): a personal
// event only to its audience, any other by its receive action; an unauthenticated decision is an
// error of kind unauthenticated, so the gateway closes the connection with 4012.
func TestAdmit_SingleRuleForGateway(t *testing.T) {
	t.Parallel()
	alex, sam, support, marketing := ids.New(), ids.New(), ids.New(), ids.New()
	a := actor.Actor{ID: alex, Kind: actor.KindUser}
	file := func(channel ids.ID) sync.Event {
		return sync.Event{Seq: 1, ID: ids.New(), Type: "file.updated", SchemaVersion: 1, ChannelID: channel, ResourceKind: "file",
			ResourceID: ids.New(), ReceiveAction: "file.read", Payload: json.RawMessage(`{}`)}
	}
	personal := func(audience ids.ID) sync.Event {
		th := ids.New()
		return sync.Event{Seq: 2, ID: ids.New(), Type: "read_state.updated", SchemaVersion: 1, ThreadID: th, ResourceKind: "thread",
			ResourceID: th, ReceiveAction: "self", AudiencePrincipalID: audience, Payload: json.RawMessage(`{}`)}
	}
	s := newCatchUp(t, &catchLog{}, &catchAuthz{allowed: map[ids.ID]bool{support: true}}, 5, 10)
	ctx := context.Background()
	for _, c := range []struct {
		name string
		ev   sync.Event
		want bool
	}{
		{"readable channel", file(support), true},
		{"unreadable channel", file(marketing), false},
		{"own personal event", personal(alex), true},
		{"personal event of another", personal(sam), false},
	} {
		got, err := s.Admit(ctx, a, c.ev)
		if err != nil || got != c.want {
			t.Errorf("%s: %v %v, want %v", c.name, got, err, c.want)
		}
	}
	revoked := newCatchUp(t, &catchLog{}, &revokedAuthz{}, 5, 10)
	if ok, err := revoked.Admit(ctx, a, file(support)); ok || errs.KindOf(err) != errs.KindUnauthenticated {
		t.Fatalf("revoked credential: %v %v, want unauthenticated", ok, err)
	}
	// A personal event is checked against the credential too: a revoked API token without
	// access.changed must not receive its own events (red team M1-65, blocker 2).
	if ok, err := revoked.Admit(ctx, a, personal(alex)); ok || errs.KindOf(err) != errs.KindUnauthenticated {
		t.Fatalf("own personal event of a revoked credential: %v %v, want unauthenticated", ok, err)
	}
}

// TestPageEvents_AdmittedEventsAndCursor: PageEvents returns the admitted events themselves (the
// gateway needs their scope) and the highest checked seq, like Page.
func TestPageEvents_AdmittedEventsAndCursor(t *testing.T) {
	t.Parallel()
	alex, support, marketing := ids.New(), ids.New(), ids.New()
	ev := func(seq int64, channel ids.ID) sync.Event {
		return sync.Event{Seq: seq, ID: ids.New(), Type: "file.updated", SchemaVersion: 1, ChannelID: channel, ResourceKind: "file",
			ResourceID: ids.New(), ReceiveAction: "file.read", Payload: json.RawMessage(`{}`)}
	}
	log := &catchLog{min: 1, events: []sync.Event{ev(1, support), ev(2, marketing), ev(3, support), ev(4, marketing)}}
	s := newCatchUp(t, log, &catchAuthz{allowed: map[ids.ID]bool{support: true}}, 4, 10)
	ctx := actor.WithActor(context.Background(), actor.Actor{ID: alex, Kind: actor.KindUser})
	page, err := s.PageEvents(ctx, sync.PageInput{After: 0, Head: 4})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Events) != 2 || page.Events[0].Seq != 1 || page.Events[1].Seq != 3 || page.NextCursor != 4 {
		t.Fatalf("page %+v", page)
	}
}

// operatorAuthz allows agent.config.read on a channel only for its operator in operators, and
// every other action (gateway.connect of the credential check) for everyone.
type operatorAuthz struct {
	catchAuthz
	operators map[ids.ID]ids.ID // channel -> operator
	asked     []authorize.Action
}

func (f *operatorAuthz) Authorize(_ context.Context, p authorize.Principal, a authorize.Action, r authorize.Resource) (authorize.Decision, error) {
	f.asked = append(f.asked, a)
	if a != authorize.ActionAgentConfigRead {
		return authorize.Decision{Outcome: authorize.OutcomeAllow}, nil
	}
	if r.Kind == authorize.ResourceChannel && f.operators[r.ID] == p.PrincipalID() {
		return authorize.Decision{Outcome: authorize.OutcomeAllow}, nil
	}
	return authorize.Decision{Outcome: authorize.OutcomeForbidden, Reason: "subject_kind_not_allowed"}, nil
}

// TestAdmit_PersonalEventWithReceiveAction: a personal event whose receive action is not self
// (agent.config_changed, M2-02 design note 11a) needs both the audience and the receive action at
// delivery time: admitted for Lotse while it operates the channel, not after the operator
// changed, never for another principal; a personal event with self is admitted for its audience
// without a receive action check.
func TestAdmit_PersonalEventWithReceiveAction(t *testing.T) {
	t.Parallel()
	lotse, runner, channel, space := ids.New(), ids.New(), ids.New(), ids.New()
	cfg := sync.Event{Seq: 1, ID: ids.New(), Type: "agent.config_changed", SchemaVersion: 1, SpaceID: space, ChannelID: channel,
		ResourceKind: "channel", ResourceID: channel, ReceiveAction: "agent.config.read", AudiencePrincipalID: lotse,
		Payload: json.RawMessage(`{}`)}
	az := &operatorAuthz{operators: map[ids.ID]ids.ID{channel: lotse}}
	s := newCatchUp(t, &catchLog{}, az, 5, 10)
	ctx := context.Background()
	asLotse := actor.Actor{ID: lotse, Kind: actor.KindAgent}
	if ok, err := s.Admit(ctx, asLotse, cfg); err != nil || !ok {
		t.Fatalf("Lotse as operator: %v %v, want admitted", ok, err)
	}
	if ok, err := s.Admit(ctx, actor.Actor{ID: runner, Kind: actor.KindAgent}, cfg); err != nil || ok {
		t.Fatalf("another principal: %v %v, want not admitted", ok, err)
	}
	az.operators[channel] = runner
	if ok, err := s.Admit(ctx, asLotse, cfg); err != nil || ok {
		t.Fatalf("Lotse after the operator changed: %v %v, want not admitted", ok, err)
	}
	az.asked = nil
	self := sync.Event{Seq: 2, ID: ids.New(), Type: "access.changed", SchemaVersion: 1, ResourceKind: "principal", ResourceID: lotse,
		ReceiveAction: "self", AudiencePrincipalID: lotse, Payload: json.RawMessage(`{}`)}
	if ok, err := s.Admit(ctx, asLotse, self); err != nil || !ok {
		t.Fatalf("own personal event with self: %v %v, want admitted", ok, err)
	}
	if slices.Contains(az.asked, authorize.ActionAgentConfigRead) {
		t.Fatalf("a self event asked for a receive action: %v", az.asked)
	}
}

// hintAuthz allows push.deliver on the threads of readable for the people of readers and
// records every action asked.
type hintAuthz struct {
	catchAuthz
	readers  map[ids.ID]bool
	readable map[ids.ID]bool
	asked    []authorize.Action
}

func (f *hintAuthz) Authorize(_ context.Context, p authorize.Principal, a authorize.Action, r authorize.Resource) (authorize.Decision, error) {
	f.asked = append(f.asked, a)
	if a != authorize.ActionPushDeliver {
		return authorize.Decision{Outcome: authorize.OutcomeAllow}, nil
	}
	if r.Kind == authorize.ResourceThread && f.readable[r.ID] && f.readers[p.PrincipalID()] {
		return authorize.Decision{Outcome: authorize.OutcomeAllow}, nil
	}
	return authorize.Decision{Outcome: authorize.OutcomeNotFound, Reason: "no_grant"}, nil
}

// TestAdmit_NotificationCreatedWithoutThreadSelfOnly: notification.created with resource user and
// audience Alex is admitted for Alex's connection and refused for Sam's and for an agent
// connection, without a push.deliver check (05 section 6, R-AZ-30, M3-23 design note 6); with
// resource thread it still needs push.deliver and is refused for a connection without read right.
func TestAdmit_NotificationCreatedWithoutThreadSelfOnly(t *testing.T) {
	t.Parallel()
	alex, sam, lotse, thread, channel, space := ids.New(), ids.New(), ids.New(), ids.New(), ids.New(), ids.New()
	az := &hintAuthz{readers: map[ids.ID]bool{alex: true}, readable: map[ids.ID]bool{thread: true}}
	s := newCatchUp(t, &catchLog{}, az, 5, 10)
	ctx := context.Background()
	asAlex, asSam := actor.Actor{ID: alex, Kind: actor.KindUser}, actor.Actor{ID: sam, Kind: actor.KindUser}
	asAgent := actor.Actor{ID: lotse, Kind: actor.KindAgent}
	admin := sync.Event{Type: "notification.created", SchemaVersion: 1, ResourceKind: "user", ResourceID: alex, ReceiveAction: "push.deliver",
		AudiencePrincipalID: alex, Payload: json.RawMessage(`{}`)}
	if ok, err := s.Admit(ctx, asAlex, admin); err != nil || !ok {
		t.Fatalf("Alex: %v %v, want admitted", ok, err)
	}
	if slices.Contains(az.asked, authorize.ActionPushDeliver) {
		t.Fatalf("a hint without a thread asked push.deliver: %v", az.asked)
	}
	for name, a := range map[string]actor.Actor{"Sam": asSam, "agent": asAgent} {
		if ok, err := s.Admit(ctx, a, admin); err != nil || ok {
			t.Errorf("%s: %v %v, want refused", name, ok, err)
		}
	}
	// A resource user that names another person than the audience is refused as well.
	foreign := admin
	foreign.ResourceID = sam
	if ok, err := s.Admit(ctx, asAlex, foreign); err != nil || ok {
		t.Errorf("resource of another person: %v %v, want refused", ok, err)
	}
	hint := func(audience ids.ID) sync.Event {
		return sync.Event{Type: "notification.created", SchemaVersion: 1, SpaceID: space, ChannelID: channel, ThreadID: thread,
			ResourceKind: "thread", ResourceID: thread, ReceiveAction: "push.deliver", AudiencePrincipalID: audience, Payload: json.RawMessage(`{}`)}
	}
	az.asked = nil
	if ok, err := s.Admit(ctx, asAlex, hint(alex)); err != nil || !ok {
		t.Fatalf("thread hint to Alex: %v %v, want admitted", ok, err)
	}
	if !slices.Contains(az.asked, authorize.ActionPushDeliver) {
		t.Fatalf("a thread hint did not ask push.deliver: %v", az.asked)
	}
	if ok, err := s.Admit(ctx, asSam, hint(sam)); err != nil || ok {
		t.Fatalf("thread hint to Sam without read right: %v %v, want refused", ok, err)
	}
}

// statsAuthz allows stats.read only for the manager on the space support and records the actions
// and resources asked.
type statsAuthz struct {
	catchAuthz
	manager, support ids.ID
	asked            []authorize.Resource
}

func (f *statsAuthz) Authorize(_ context.Context, p authorize.Principal, a authorize.Action, r authorize.Resource) (authorize.Decision, error) {
	if a != authorize.ActionStatsRead {
		return authorize.Decision{Outcome: authorize.OutcomeAllow}, nil
	}
	f.asked = append(f.asked, r)
	if p.PrincipalID() == f.manager && r.Kind == authorize.ResourceSpace && r.ID == f.support {
		return authorize.Decision{Outcome: authorize.OutcomeAllow}, nil
	}
	return authorize.Decision{Outcome: authorize.OutcomeForbidden, Reason: "capability_missing"}, nil
}

// TestAdmit_StatsLive (M6-15 design note 5): the generic rule of a personal ephemeral event with
// a receive action decides stats.live, so sync needs no case of its own: the frame for the
// manager is admitted with stats.read on its scope resource (the space), refused on the instance
// and for any other connection, and every decision names the frame's resource.
func TestAdmit_StatsLive(t *testing.T) {
	t.Parallel()
	manager, support := ids.New(), ids.New()
	az := &statsAuthz{manager: manager, support: support}
	s := newCatchUp(t, &catchLog{}, az, 5, 10)
	ctx := context.Background()
	asManager := actor.Actor{ID: manager, Kind: actor.KindUser}
	space := sync.Event{Type: "stats.live", SchemaVersion: 1, SpaceID: support, ResourceKind: "space", ResourceID: support,
		ReceiveAction: "stats.read", AudiencePrincipalID: manager, Payload: json.RawMessage(`{}`)}
	if ok, err := s.Admit(ctx, asManager, space); err != nil || !ok {
		t.Fatalf("space frame for the manager: %v %v, want admitted", ok, err)
	}
	instance := sync.Event{Type: "stats.live", SchemaVersion: 1, ResourceKind: "instance", ReceiveAction: "stats.read",
		AudiencePrincipalID: manager, Payload: json.RawMessage(`{}`)}
	if ok, err := s.Admit(ctx, asManager, instance); err != nil || ok {
		t.Fatalf("instance frame for the manager: %v %v, want refused", ok, err)
	}
	if ok, err := s.Admit(ctx, actor.Actor{ID: ids.New(), Kind: actor.KindUser}, space); err != nil || ok {
		t.Fatalf("frame of another receiver: %v %v, want refused", ok, err)
	}
	want := []authorize.ResourceKind{authorize.ResourceSpace, authorize.ResourceInstance}
	if len(az.asked) != 2 || az.asked[0].Kind != want[0] || az.asked[0].ID != support || az.asked[1].Kind != want[1] {
		t.Fatalf("resources asked %+v", az.asked)
	}
}
