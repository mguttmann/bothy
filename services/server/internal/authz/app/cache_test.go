// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/authz/app"
	"github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

func TestCache_VersionChangeInvalidates(t *testing.T) {
	w := newWorld(t)
	alex := w.user(w.alex)
	w.allow(t, alex, authorize.ActionMessageRead, w.msg(w.allgemein, w.sam))
	// A revocation without a new version is not seen: the version is the only invalidation.
	if _, _, err := w.store.RevokeGrant(context.Background(), w.g1, w.owner, "t", w.clk.Now()); err != nil {
		t.Fatal(err)
	}
	w.allow(t, alex, authorize.ActionMessageRead, w.msg(w.allgemein, w.sam))
	w.versions.Bump()
	w.expect(t, alex, authorize.ActionMessageRead, w.msg(w.allgemein, w.sam), notFound, "no_grant")
}

func TestCache_ValidUntilBoundary(t *testing.T) {
	w := newWorld(t)
	sam := w.user(w.sam)
	w.clk.Set(w.membershipExpiry.Add(-time.Nanosecond))
	w.allow(t, sam, authorize.ActionMessageRead, w.msg(w.allgemein, w.alex))
	reads := w.store.Reads()
	w.allow(t, sam, authorize.ActionMessageRead, w.msg(w.allgemein, w.alex))
	if w.store.Reads() != reads {
		t.Fatal("snapshot rebuilt before ValidUntil")
	}
	w.clk.Set(w.membershipExpiry)
	w.expect(t, sam, authorize.ActionMessageRead, w.msg(w.allgemein, w.alex), notFound, "no_grant")
	if w.store.Reads() != reads+1 {
		t.Fatalf("reads %d, want a rebuild at ValidUntil", w.store.Reads())
	}
}

func TestCache_LRUBound(t *testing.T) {
	w := newWorldWith(t, app.AuthorizerConfig{SnapshotCacheMaxEntries: 2, StepUpMaxAge: 5 * time.Minute})
	for _, p := range []ids.ID{w.owner, w.alex, w.sam, w.robin, w.owner, w.alex} {
		w.decide(t, context.Background(), w.user(p), authorize.ActionMessageRead, w.msg(w.allgemein, w.alex))
		if n := w.az.CacheLen(); n > 2 {
			t.Fatalf("cache holds %d entries", n)
		}
	}
	if w.az.CacheLen() != 2 {
		t.Fatalf("cache holds %d entries, want 2", w.az.CacheLen())
	}
}

func TestVersion_OneReadPerRequestScope(t *testing.T) {
	w := newWorld(t)
	ctx := authorize.WithRequestScope(context.Background())
	for range 5 {
		w.decide(t, ctx, w.user(w.alex), authorize.ActionMessageRead, w.msg(w.allgemein, w.sam))
	}
	if _, err := w.az.Scope(ctx, w.user(w.alex), authorize.ActionMessageRead); err != nil {
		t.Fatal(err)
	}
	if w.versions.Reads() != 1 {
		t.Fatalf("version read %d times in one request", w.versions.Reads())
	}
	for range 3 {
		w.decide(t, context.Background(), w.user(w.alex), authorize.ActionMessageRead, w.msg(w.allgemein, w.sam))
	}
	if w.versions.Reads() != 4 {
		t.Fatalf("without a request scope every call reads: %d", w.versions.Reads())
	}
}

func TestVersion_PinnedWins(t *testing.T) {
	w := newWorld(t)
	w.versions.Fail(true)
	ctx := authorize.WithRequestScope(authorize.WithPermissionVersion(context.Background(), 99))
	if d, err := w.az.Authorize(ctx, w.user(w.alex), authorize.ActionMessageRead, w.msg(w.allgemein, w.sam)); err != nil || !d.Allowed() {
		t.Fatalf("pinned: %+v %v", d, err)
	}
	sc, err := w.az.Scope(ctx, w.user(w.alex), authorize.ActionMessageRead)
	if err != nil || sc.PermissionVersion != 99 {
		t.Fatalf("scope %+v %v", sc, err)
	}
}

func TestVersion_ReadFailureIsUnavailable(t *testing.T) {
	w := newWorld(t)
	w.versions.Fail(true)
	d, err := w.az.Authorize(context.Background(), w.user(w.alex), authorize.ActionMessageRead, w.msg(w.allgemein, w.sam))
	if d.Allowed() || errs.KindOf(err) != errs.KindUnavailable {
		t.Fatalf("got %+v, kind %s", d, errs.KindOf(err))
	}
}

// TestFailClosed_SourceErrors: no source error ever produces an allow (07 section 1 point 5).
func TestFailClosed_SourceErrors(t *testing.T) {
	type failing interface{ Fail(bool) }
	for name, pick := range map[string]func(*world) failing{
		"principals":  func(w *world) failing { return w.principals },
		"credentials": func(w *world) failing { return w.creds },
		"structure":   func(w *world) failing { return w.structure },
		"store":       func(w *world) failing { return w.store },
	} {
		t.Run(name, func(t *testing.T) {
			w := newWorld(t)
			pick(w).Fail(true)
			d, err := w.az.Authorize(context.Background(), w.user(w.alex), authorize.ActionMessageRead, w.msg(w.allgemein, w.sam))
			if err == nil || d.Allowed() {
				t.Fatalf("got %+v, %v", d, err)
			}
			if _, err := w.az.Scope(context.Background(), w.user(w.alex), authorize.ActionMessageRead); err == nil {
				t.Fatal("scope without error")
			}
		})
	}
	t.Run("policies", func(t *testing.T) {
		w := newWorld(t)
		w.policies.Fail(true)
		d, err := w.az.Authorize(context.Background(), w.user(w.alex), authorize.ActionAgentInvoke, w.invoke(w.allgemein, w.lotse, true))
		if err == nil || d.Allowed() {
			t.Fatalf("got %+v, %v", d, err)
		}
	})
}

func TestFailClosed_UnknownResourceKindAndHints(t *testing.T) {
	w := newWorld(t)
	alex := w.user(w.alex)
	for name, r := range map[string]authorize.Resource{
		"kind not of the action": {Kind: authorize.ResourceDevice, ID: w.devices[w.alex], OwnerID: w.alex},
		"terminal session":       {Kind: authorize.ResourceTerminalSession, ID: ids.New()},
		"contradictory channel":  {Kind: authorize.ResourceMessage, ID: ids.New(), ThreadID: w.mainThread[w.allgemein], ChannelID: w.kampagnen},
		"no path":                {Kind: authorize.ResourceMessage, ID: ids.New()},
	} {
		if d, err := w.az.Authorize(context.Background(), alex, authorize.ActionMessageRead, r); err == nil || d.Allowed() {
			t.Errorf("%s: got %+v, %v", name, d, err)
		}
	}
	// A missing thread is simply not found.
	w.expect(t, alex, authorize.ActionMessageRead, authorize.Resource{Kind: authorize.ResourceMessage, ID: ids.New(), ThreadID: ids.New()}, notFound, "no_grant")
}

// unknownAction is an action outside the catalogue; only a test may build one.
var unknownAction = authorize.Action("no.such.action")

func TestAZP3_UnknownActionFailsClosedThroughAuthorize(t *testing.T) {
	w := newWorld(t)
	before := w.errorLines()
	d, err := w.az.Authorize(context.Background(), w.user(w.owner), unknownAction, authorize.Resource{Kind: authorize.ResourceInstance})
	if d.Allowed() || errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("got %+v, %v", d, err)
	}
	if e, _ := errs.As(err); e.Cause != errs.CauseUnknownAction {
		t.Fatalf("cause %q", e.Cause)
	}
	if n := w.errorLines() - before; n != 1 {
		t.Fatalf("%d ERROR records, want 1:\n%s", n, w.logs)
	}
	_, detached := w.rec.counts()
	if detached != 1 || w.rec.detached[0].Decision != auditing.DecisionError || w.rec.detached[0].Reason != auditing.ReasonUnknownAction {
		t.Fatalf("recorder %+v", w.rec.detached)
	}
	// A deferred action fails the same way (D3).
	for _, a := range []authorize.Action{authorize.ActionRunFollowupCreate, authorize.ActionFileTranscriptSubmit, authorize.ActionImportApply} {
		d, err := w.az.Authorize(context.Background(), w.user(w.owner), a, authorize.Resource{Kind: authorize.ResourceInstance})
		if e, ok := errs.As(err); !ok || d.Allowed() || e.Cause != errs.CauseUnknownAction {
			t.Errorf("%s: %+v %v", a, d, err)
		}
	}
	// A system action is never reachable.
	if d, err := w.az.Authorize(context.Background(), w.user(w.owner), authorize.ActionGrantExpire, authorize.Resource{Kind: authorize.ResourceGrant, ID: w.g1}); err == nil || d.Allowed() {
		t.Fatalf("system action: %+v %v", d, err)
	}
	if _, err := w.az.Scope(context.Background(), w.user(w.owner), unknownAction); errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("scope: %v", err)
	}
}

func TestAZP10_ExpiryWithoutJob(t *testing.T) {
	w := newWorld(t)
	robin, sam := w.user(w.robin), w.user(w.sam)
	w.allow(t, robin, authorize.ActionThreadRead, w.thread(w.projektA))
	w.allow(t, sam, authorize.ActionMessageRead, w.msg(w.allgemein, w.alex))
	reads := w.versions.Reads()
	w.clk.Set(w.g3Expiry)
	w.expect(t, robin, authorize.ActionThreadRead, w.thread(w.projektA), notFound, "no_grant")
	w.expect(t, sam, authorize.ActionMessageRead, w.msg(w.allgemein, w.alex), notFound, "no_grant")
	if v, _ := w.versions.PermissionVersion(context.Background()); v != 1 || w.versions.Reads() <= reads {
		t.Fatalf("the version must not change: %d", v)
	}
}

func TestChannelCreatedAfterSnapshotVisibleWithoutNewVersion(t *testing.T) {
	w := newWorld(t)
	alex := w.user(w.alex)
	w.allow(t, alex, authorize.ActionChannelRead, w.channel(w.allgemein))
	reads := w.store.Reads()
	fresh := ids.New()
	w.structure.PutChannel(authz.ChannelFacts{ID: fresh, SpaceID: w.support, Kind: authz.ChannelText})
	w.allow(t, alex, authorize.ActionChannelRead, w.channel(fresh))
	sc, err := w.az.Scope(context.Background(), alex, authorize.ActionChannelRead)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(sc.ChannelIDs, fresh) || !slices.Contains(sc.FullyVisibleSpaceIDs, w.support) {
		t.Fatalf("scope %+v", sc)
	}
	if w.store.Reads() != reads {
		t.Fatal("the snapshot was rebuilt")
	}
	// A new space with an instance grant appears as well, and a new dm of the person.
	space := ids.New()
	w.structure.PutSpace(space)
	dm := ids.New()
	w.structure.PutChannel(authz.ChannelFacts{ID: dm, Kind: authz.ChannelDM, DMParticipants: []ids.ID{w.alex, w.robin}})
	sc, _ = w.az.Scope(context.Background(), alex, authorize.ActionChannelRead)
	if !slices.Contains(sc.ChannelIDs, dm) || slices.Contains(sc.SpaceIDs, space) {
		t.Fatalf("scope %+v", sc)
	}
	sc, _ = w.az.Scope(context.Background(), w.user(w.owner), authorize.ActionSpaceRead)
	if !slices.Contains(sc.FullyVisibleSpaceIDs, space) || !sc.InstanceWide || slices.Contains(sc.ChannelIDs, dm) {
		t.Fatalf("owner scope %+v", sc)
	}
}

func TestAudit_DenialsOnlyAndFilterMode(t *testing.T) {
	w := newWorld(t)
	alex := w.user(w.alex)
	w.allow(t, alex, authorize.ActionMessageRead, w.msg(w.allgemein, w.sam))
	w.expect(t, alex, authorize.ActionMessageRead, w.msg(w.kampagnen, w.sam), notFound, "no_grant")
	w.expect(t, w.user(w.sam), authorize.ActionMessageCreate, w.thread(w.allgemein), forbidden, "level_insufficient")
	denials, _ := w.rec.counts()
	if denials != 2 || w.rec.denials[0].Reason != auditing.ReasonNoGrant || w.rec.denials[0].Decision != auditing.DecisionDeny ||
		w.rec.denials[0].Action != "message.read" || w.rec.denials[0].ResourceKind != "message" {
		t.Fatalf("denials %+v", w.rec.denials)
	}
	fctx := authorize.Filter(context.Background())
	if d := w.decide(t, fctx, alex, authorize.ActionMessageRead, w.msg(w.kampagnen, w.sam)); d.Outcome != notFound {
		t.Fatalf("filter changed the decision: %+v", d)
	}
	if d := w.decide(t, fctx, alex, authorize.ActionMessageRead, w.msg(w.allgemein, w.sam)); !d.Allowed() {
		t.Fatalf("filter changed the decision: %+v", d)
	}
	if n, _ := w.rec.counts(); n != 2 {
		t.Fatalf("filter mode wrote a denial: %d", n)
	}
	// A recorder failure is logged and never changes the decision.
	w.rec.fail = true
	before := w.errorLines()
	w.expect(t, alex, authorize.ActionMessageRead, w.msg(w.kampagnen, w.sam), notFound, "no_grant")
	if w.errorLines() != before+1 || !strings.Contains(w.logs.String(), "audit of an authorization decision failed") {
		t.Fatalf("no log line for the failed audit:\n%s", w.logs)
	}
}

func TestCheckLog_AuthorizeAndScope(t *testing.T) {
	w := newWorld(t)
	ctx := authorize.WithCheckLog(context.Background())
	w.decide(t, ctx, w.user(w.alex), authorize.ActionMessageRead, w.msg(w.allgemein, w.sam))
	w.decide(t, ctx, w.user(w.alex), authorize.ActionMessageRead, w.msg(w.kampagnen, w.sam))
	if _, err := w.az.Scope(ctx, w.user(w.alex), authorize.ActionThreadRead); err != nil {
		t.Fatal(err)
	}
	got := authorize.Checks(ctx)
	want := []authorize.Check{
		{Action: "message.read", Kind: authorize.CheckAuthorize, Allowed: true},
		{Action: "message.read", Kind: authorize.CheckAuthorize, Allowed: false},
		{Action: "thread.read", Kind: authorize.CheckScope, Allowed: true},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("checks %+v", got)
	}
}

func TestAnonymous(t *testing.T) {
	w := newWorld(t)
	anon := actor.Anonymous()
	w.allow(t, anon, authorize.ActionInstanceDiscoveryRead, authorize.Resource{Kind: authorize.ResourceInstance})
	w.expect(t, anon, authorize.ActionSelfRead, authorize.Resource{Kind: authorize.ResourcePrincipal}, unauthentic, "unauthenticated")
	if _, err := w.az.Scope(context.Background(), anon, authorize.ActionMessageRead); errs.KindOf(err) != errs.KindUnauthenticated {
		t.Fatalf("scope: %v", err)
	}
	// A host principal has no action but host.connect (M2-43).
	w.expect(t, actor.Actor{ID: ids.New(), Kind: actor.KindHost}, authorize.ActionSelfRead, authorize.Resource{Kind: authorize.ResourcePrincipal}, unauthentic, "unauthenticated")
}

func TestLiveness_TokenAndApp(t *testing.T) {
	w := newWorld(t)
	tracker := w.app()
	tracker.InstallationID = ids.ID{}
	w.expect(t, tracker, authorize.ActionMessageCreate, w.thread(w.eskalation), unauthentic, "app_not_installed")
	lotse := w.agent(w.lotse)
	lotse.TokenID = ids.New()
	w.expect(t, lotse, authorize.ActionMessageCreate, w.thread(w.allgemein), unauthentic, "token_revoked")
	// A kind that differs from the principal row fails closed.
	w.expect(t, actor.Actor{ID: w.lotse, Kind: actor.KindUser, TokenID: w.tokens[w.lotse]}, authorize.ActionMessageRead, w.msg(w.allgemein, w.sam),
		unauthentic, "unauthenticated")
	// An unknown principal is unauthenticated.
	w.expect(t, actor.Actor{ID: ids.New(), Kind: actor.KindAgent}, authorize.ActionMessageRead, w.msg(w.allgemein, w.sam), unauthentic, "unauthenticated")
	// A revoked device.
	w.creds.PutDevice(w.devices[w.sam], fakeDevice(w.sam, authz.DeviceRevoked))
	w.versions.Bump()
	w.expect(t, w.user(w.sam), authorize.ActionMessageRead, w.msg(w.allgemein, w.alex), unauthentic, "device_inactive")
	if _, err := w.az.Scope(context.Background(), w.user(w.sam), authorize.ActionMessageRead); errs.KindOf(err) != errs.KindUnauthenticated {
		t.Fatalf("scope of a revoked device: %v", err)
	}
}

func TestScope_Owner(t *testing.T) {
	w := newWorld(t)
	own := ids.New()
	w.structure.PutChannel(authz.ChannelFacts{ID: own, Kind: authz.ChannelDM, DMParticipants: []ids.ID{w.owner, w.alex}})
	sc, err := w.az.Scope(context.Background(), w.user(w.owner), authorize.ActionMessageRead)
	if err != nil {
		t.Fatal(err)
	}
	if slices.Contains(sc.ChannelIDs, w.dm) || !slices.Contains(sc.ChannelIDs, own) || len(sc.ChannelIDs) != 8 || !sc.InstanceWide ||
		len(sc.FullyVisibleSpaceIDs) != 4 {
		t.Fatalf("owner scope %+v", sc)
	}
	if !slices.IsSortedFunc(sc.ChannelIDs, func(a, b ids.ID) int { return strings.Compare(a.String(), b.String()) }) ||
		!slices.IsSortedFunc(sc.SpaceIDs, func(a, b ids.ID) int { return strings.Compare(a.String(), b.String()) }) {
		t.Fatal("not ascending")
	}
}

func TestScope_AppNeedsReadScope(t *testing.T) {
	w := newWorld(t)
	sc, err := w.az.Scope(context.Background(), w.app(), authorize.ActionMessageRead)
	if err != nil || len(sc.ChannelIDs) != 0 || len(sc.SpaceIDs) != 0 {
		t.Fatalf("scope %+v %v", sc, err)
	}
	reader := w.app()
	reader.GrantedScopes = []string{domain.ScopeMessagesRead}
	sc, _ = w.az.Scope(context.Background(), reader, authorize.ActionMessageRead)
	if len(sc.ChannelIDs) != 0 {
		t.Fatalf("a token without messages:read lists channels: %+v", sc)
	}
	reader.Scopes = []string{domain.ScopeMessagesRead}
	sc, _ = w.az.Scope(context.Background(), reader, authorize.ActionMessageRead)
	if !slices.Equal(sc.ChannelIDs, []ids.ID{w.eskalation}) || !slices.Equal(sc.SpaceIDs, []ids.ID{w.support}) {
		t.Fatalf("scope %+v", sc)
	}
}

func TestScope_ViewAsAndActingFor(t *testing.T) {
	w := newWorld(t)
	asSam := w.user(w.owner)
	asSam.ViewAs = &actor.ViewAs{UserID: w.sam, StartedAt: w.clk.Now()}
	sc, _ := w.az.Scope(context.Background(), asSam, authorize.ActionMessageRead)
	want := domain.SortIDs([]ids.ID{w.allgemein, w.kampagnen, w.intern})
	if !slices.Equal(sc.ChannelIDs, want) || sc.InstanceWide {
		t.Fatalf("view as Sam %+v", sc)
	}
	w.grant(t, domain.Subject{Kind: domain.SubjectAgent, PrincipalID: w.lotse}, domain.Target{Kind: domain.TargetInstance}, domain.LevelRead, time.Time{})
	w.versions.Bump()
	lotse := w.agent(w.lotse)
	sc, _ = w.az.Scope(context.Background(), lotse, authorize.ActionMessageRead)
	if len(sc.ChannelIDs) != 7 || !sc.InstanceWide {
		t.Fatalf("lotse %+v", sc)
	}
	lotse.ActingFor = actor.ActingFor{UserID: w.alex}
	sc, _ = w.az.Scope(context.Background(), lotse, authorize.ActionMessageRead)
	if !slices.Equal(sc.ChannelIDs, domain.SortIDs([]ids.ID{w.allgemein, w.eskalation})) || sc.InstanceWide {
		t.Fatalf("lotse acting for Alex %+v", sc)
	}
}

func TestScope_OnlyListActions(t *testing.T) {
	w := newWorld(t)
	for _, a := range []authorize.Action{authorize.ActionMessageCreate, authorize.ActionSearchQuery, authorize.ActionUserCreate} {
		if _, err := w.az.Scope(context.Background(), w.user(w.owner), a); errs.KindOf(err) != errs.KindInternal {
			t.Errorf("%s: %v", a, err)
		}
	}
}

func TestScope_MemoPerRequest(t *testing.T) {
	w := newWorld(t)
	ctx := authorize.WithRequestScope(context.Background())
	a, _ := w.az.Scope(ctx, w.user(w.alex), authorize.ActionMessageRead)
	a.ChannelIDs[0] = ids.ID{}
	w.structure.PutChannel(authz.ChannelFacts{ID: ids.New(), SpaceID: w.support, Kind: authz.ChannelText})
	b, _ := w.az.Scope(ctx, w.user(w.alex), authorize.ActionMessageRead)
	if len(b.ChannelIDs) != 3 || b.ChannelIDs[0].IsZero() {
		t.Fatalf("memo not kept or shared: %+v", b)
	}
	c, _ := w.az.Scope(context.Background(), w.user(w.alex), authorize.ActionMessageRead)
	if len(c.ChannelIDs) != 4 {
		t.Fatalf("next request must see the new channel: %+v", c)
	}
}

func TestNewAuthorizer_Validation(t *testing.T) {
	if _, err := app.NewAuthorizer(app.AuthorizerConfig{SnapshotCacheMaxEntries: 1}, app.AuthorizerDeps{}); err == nil {
		t.Fatal("missing deps accepted")
	}
	w := newWorld(t)
	w.cfg.SnapshotCacheMaxEntries = 0
	_, err := app.NewAuthorizer(w.cfg, app.AuthorizerDeps{Catalog: app.NewCatalog(nil), Store: w.store, Principals: w.principals,
		Credentials: w.creds, Structure: w.structure, Policies: w.policies, Versions: w.versions, Recorder: w.rec, Clock: w.clk})
	if err == nil || errors.Is(err, nil) {
		t.Fatal("cache bound 0 accepted")
	}
}
