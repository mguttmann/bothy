// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"encoding/json"
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
