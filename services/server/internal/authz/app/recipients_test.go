// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"slices"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/authz/app"
	"github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Compile-time check: the Authorizer answers the candidates of hints (M3-19 design note 6).
var _ authz.Recipients = (*app.Authorizer)(nil)

// ascendingIDs returns list ascending, the order Readers promises.
func ascendingIDs(list ...ids.ID) []ids.ID { return domain.SortIDs(slices.Clone(list)) }

// TestReaders_Channel: the readers of Support/Allgemein are Alex (G-1 on Support), Sam (through
// the group Leser, G-4) and the Owner; Robin has no grant there, and the agent Lotse (G-5) and the
// app Tracker are no humans (13 section 12 point 2).
func TestReaders_Channel(t *testing.T) {
	w := newWorld(t)
	got, err := w.az.Readers(context.Background(), w.allgemein)
	if err != nil {
		t.Fatal(err)
	}
	if want := ascendingIDs(w.alex, w.sam, w.owner); !slices.Equal(got, want) {
		t.Fatalf("Readers(Allgemein) = %v, want %v", got, want)
	}
	// Eskalation has no grant of the group Leser: Alex and the Owner only, the app G-6 dropped.
	got, err = w.az.Readers(context.Background(), w.eskalation)
	if err != nil {
		t.Fatal(err)
	}
	if want := ascendingIDs(w.alex, w.owner); !slices.Equal(got, want) {
		t.Fatalf("Readers(Eskalation) = %v, want %v", got, want)
	}
}

// TestReaders_OnlyActiveHumans: a disabled person and a pending one are no recipients, even with a
// grant; an owner who is disabled is dropped too.
func TestReaders_OnlyActiveHumans(t *testing.T) {
	w := newWorld(t)
	w.principals.Put(authz.PrincipalFacts{ID: w.sam, Kind: actor.KindUser, Status: authz.PrincipalDisabled})
	w.principals.Put(authz.PrincipalFacts{ID: w.alex, Kind: actor.KindUser, Status: authz.PrincipalPending})
	w.principals.Put(authz.PrincipalFacts{ID: w.owner, Kind: actor.KindUser, Status: authz.PrincipalDisabled, IsOwner: true})
	w.versions.Bump()
	got, err := w.az.Readers(context.Background(), w.allgemein)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("Readers(Allgemein) = %v, want none", got)
	}
}

// TestReaders_DM: a dm channel gives its two participants and no owner (SR6).
func TestReaders_DM(t *testing.T) {
	w := newWorld(t)
	got, err := w.az.Readers(context.Background(), w.dm)
	if err != nil {
		t.Fatal(err)
	}
	if want := ascendingIDs(w.alex, w.sam); !slices.Equal(got, want) {
		t.Fatalf("Readers(dm) = %v, want %v", got, want)
	}
}

// TestReaders_UnknownChannel: an unknown channel is not_found.
func TestReaders_UnknownChannel(t *testing.T) {
	w := newWorld(t)
	if _, err := w.az.Readers(context.Background(), ids.New()); errs.KindOf(err) != errs.KindNotFound {
		t.Fatalf("unknown channel: err %v, want not_found", err)
	}
}

// TestRecheckPrincipal_PushDeliver: push.deliver is a check without a request (R-AZ-31), so the
// re-check principal is accepted with it: Sam reads Allgemein through Leser, Robin does not, a
// disabled person is denied, and the Owner reads as owner without a session (R-AZ-29).
func TestRecheckPrincipal_PushDeliver(t *testing.T) {
	w := newWorld(t)
	ctx := authorize.Filter(context.Background())
	if d := w.decide(t, ctx, recheck(w.sam), authorize.ActionPushDeliver, w.thread(w.allgemein)); !d.Allowed() {
		t.Fatalf("Sam on Allgemein: %+v", d)
	}
	if d := w.decide(t, ctx, recheck(w.robin), authorize.ActionPushDeliver, w.thread(w.allgemein)); d.Allowed() {
		t.Fatalf("Robin on Allgemein: %+v", d)
	}
	if d := w.decide(t, ctx, recheck(w.owner), authorize.ActionPushDeliver, w.thread(w.allgemein)); !d.Allowed() {
		t.Fatalf("Owner on Allgemein: %+v", d)
	}
	if d := w.decide(t, ctx, recheck(w.owner), authorize.ActionPushDeliver, w.thread(w.dm)); d.Allowed() {
		t.Fatalf("Owner on the dm (SR6): %+v", d)
	}
	w.principals.Put(authz.PrincipalFacts{ID: w.sam, Kind: actor.KindUser, Status: authz.PrincipalDisabled})
	w.versions.Bump()
	if d := w.decide(t, ctx, recheck(w.sam), authorize.ActionPushDeliver, w.thread(w.allgemein)); d.Allowed() {
		t.Fatalf("disabled Sam on Allgemein: %+v", d)
	}
	// The marker with a credential stays a programming error with push.deliver as well.
	p := recheck(w.owner)
	p.SessionID = w.sessions[w.owner]
	if _, err := w.az.Authorize(ctx, p, authorize.ActionPushDeliver, w.thread(w.allgemein)); errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("re-check principal with a session: err %v, want internal", err)
	}
}

// TestRecheckPrincipal_ScopePushDeliver: Scope with push.deliver for the re-check principal lists
// the channels the person reads (the badge of M3-19 design note 16).
func TestRecheckPrincipal_ScopePushDeliver(t *testing.T) {
	w := newWorld(t)
	ctx := authorize.Filter(context.Background())
	sc, err := w.az.Scope(ctx, recheck(w.sam), authorize.ActionPushDeliver)
	if err != nil {
		t.Fatal(err)
	}
	if want := ascendingIDs(w.allgemein, w.kampagnen, w.intern, w.dm); !slices.Equal(ascendingIDs(sc.ChannelIDs...), want) {
		t.Fatalf("Scope(Sam, push.deliver).ChannelIDs = %v, want %v", sc.ChannelIDs, want)
	}
	sc, err = w.az.Scope(ctx, recheck(w.robin), authorize.ActionPushDeliver)
	if err != nil {
		t.Fatal(err)
	}
	if want := []ids.ID{w.projektA}; !slices.Equal(sc.ChannelIDs, want) {
		t.Fatalf("Scope(Robin, push.deliver).ChannelIDs = %v, want %v", sc.ChannelIDs, want)
	}
	w.principals.Put(authz.PrincipalFacts{ID: w.robin, Kind: actor.KindUser, Status: authz.PrincipalDisabled})
	w.versions.Bump()
	if _, err := w.az.Scope(ctx, recheck(w.robin), authorize.ActionPushDeliver); err == nil {
		t.Fatal("Scope for a disabled person succeeded")
	}
}
