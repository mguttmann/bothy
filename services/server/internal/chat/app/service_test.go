// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/chat/app"
	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// env is the use cases over in-memory fakes: the rules of authz are stubbed, so these tests cover
// the order of the steps, the errors and the writes; adapters/httpapi tests the same over
// PostgreSQL with the real authz.
type env struct {
	svc      *app.Service
	store    *store
	az       *authorizer
	rights   *rights
	rec      *recorder
	events   *appender
	uow      *uow
	archived *[]chat.SpaceArchived
	owner    actor.Actor
	affected *affected
	dir      directory
	pol      policies
}

func newEnv(t *testing.T) *env {
	t.Helper()
	e := &env{store: newStore(), az: &authorizer{rules: map[authorize.Action]func(authorize.Principal, authorize.Resource) authorize.Decision{}},
		rights: &rights{r: authz.EffectiveRights{Level: "trigger", Capabilities: []string{"manage"}}}, rec: &recorder{}, events: &appender{},
		archived: &[]chat.SpaceArchived{}, owner: actor.Actor{ID: ids.New(), Kind: actor.KindUser, SessionID: ids.New(), DeviceID: ids.New()},
		affected: &affected{}, dir: directory{principals: map[ids.ID]identity.PrincipalView{}}, pol: policies{rows: map[ids.ID]bool{}}}
	reg := txn.NewRegistry()
	if err := txn.Subscribe(reg, func(_ context.Context, ev chat.SpaceArchived) error {
		*e.archived = append(*e.archived, ev)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	reg.Seal()
	e.uow = &uow{reg: reg}
	svc, err := app.New(testConfig(12), app.Deps{
		Store: e.store, UoW: e.uow, Authorizer: e.az, Rights: rightsPort{e.rights}, Audit: e.rec, Events: e.events,
		Clock:         clock.NewFake(time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC)),
		RightsChanges: e.affected, Directory: e.dir, Policies: e.pol,
	})
	if err != nil {
		t.Fatal(err)
	}
	e.svc = svc
	return e
}

// rightsPort reads the current answer of the rights fake.
type rightsPort struct{ r *rights }

func (p rightsPort) EffectiveRights(ctx context.Context, a authorize.Principal, act authorize.Action, r authorize.Resource) (authz.EffectiveRights, error) {
	return p.r.EffectiveRights(ctx, a, act, r)
}

// deny makes act answer d.
func (e *env) deny(act authorize.Action, d authorize.Decision) {
	e.az.rules[act] = func(authorize.Principal, authorize.Resource) authorize.Decision { return d }
}

// create creates a space as the owner.
func (e *env) create(t *testing.T, name string, after ids.ID) chat.Space {
	t.Helper()
	sp, err := e.svc.CreateSpace(context.Background(), e.owner, chat.CreateSpaceCmd{Name: name, AfterSpaceID: after})
	if err != nil {
		t.Fatal(err)
	}
	return sp
}

// names returns the names of the spaces that are not archived in the global order.
func (e *env) names() []string {
	var out []string
	for _, sp := range e.store.sorted() {
		out = append(out, sp.Name)
	}
	return out
}

func wantCode(t *testing.T, err error, code errs.Code) {
	t.Helper()
	if e, ok := errs.As(err); !ok || e.Code != code {
		t.Fatalf("error %v, want %s", err, code)
	}
}

func ptr(s string) *string { return &s }

func TestNew(t *testing.T) {
	t.Parallel()
	if _, err := app.New(testConfig(1), app.Deps{}); err == nil {
		t.Fatal("missing collaborators accepted")
	}
	e := newEnv(t)
	if _, err := app.New(app.Config{}, app.Deps{Store: e.store, UoW: e.uow, Authorizer: e.az, Rights: e.rights, Audit: e.rec, Events: e.events}); err == nil {
		t.Fatal("name limit 0 accepted")
	}
	if _, err := app.New(testConfig(1), app.Deps{Store: e.store, UoW: e.uow, Authorizer: e.az, Rights: e.rights, Audit: e.rec, Events: e.events}); err == nil {
		t.Fatal("missing operator collaborators accepted")
	}
	if _, err := app.New(testConfig(1), app.Deps{Store: e.store, UoW: e.uow, Authorizer: e.az, Rights: e.rights, Audit: e.rec, Events: e.events,
		RightsChanges: e.affected, Directory: e.dir, Policies: e.pol}); err != nil {
		t.Fatal(err)
	}
}

// TestCreateAndOrder: create at the end and after a space, the checks of the body and the
// refusal of authorize.
func TestCreateAndOrder(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ctx := context.Background()
	support := e.create(t, "Support", ids.ID{})
	e.create(t, "Marketing", ids.ID{})
	e.create(t, "Research", support.ID)
	if got := e.names(); !slices.Equal(got, []string{"Support", "Research", "Marketing"}) {
		t.Fatalf("order %v", got)
	}
	if support.Version != 1 || support.CreatedAt.IsZero() || len(e.events.evs) != 3 || len(e.rec.entries) != 3 {
		t.Fatalf("create %+v, %d events, %d audits", support, len(e.events.evs), len(e.rec.entries))
	}
	_, err := e.svc.CreateSpace(ctx, e.owner, chat.CreateSpaceCmd{Name: "Team", AfterSpaceID: ids.New()})
	wantCode(t, err, errs.CodeValidationFailed)
	_, err = e.svc.CreateSpace(ctx, e.owner, chat.CreateSpaceCmd{Name: "Team", Icon: ptr("rocket")})
	wantCode(t, err, errs.CodeValidationFailed)
	_, err = e.svc.CreateSpace(ctx, e.owner, chat.CreateSpaceCmd{Name: "Team", Color: ptr("#00ff00")})
	wantCode(t, err, errs.CodeValidationFailed)
	_, err = e.svc.CreateSpace(ctx, e.owner, chat.CreateSpaceCmd{Name: "support"})
	wantCode(t, err, errs.CodeConflict)
	sp, err := e.svc.CreateSpace(ctx, e.owner, chat.CreateSpaceCmd{Name: " Team ", Icon: ptr("rocket-launch"), Color: ptr("moss")})
	if err != nil || sp.Name != "Team" || sp.Icon != "rocket-launch" || sp.Color != "moss" {
		t.Fatalf("create %+v, %v", sp, err)
	}
	e.deny(authorize.ActionSpaceCreate, authorize.Decision{Outcome: authorize.OutcomeForbidden, Reason: "owner_only"})
	_, err = e.svc.CreateSpace(ctx, e.owner, chat.CreateSpaceCmd{Name: "Other"})
	wantCode(t, err, errs.CodeForbidden)
	_, err = e.svc.CreateSpace(actor.WithActor(ctx, actor.Actor{ID: ids.New(), Kind: actor.KindUser}), e.owner, chat.CreateSpaceCmd{Name: "Other"})
	wantCode(t, err, errs.CodeInternalError)
}

// TestReads: list with the scope, filter and view rules; get with 404 for an unknown space and the
// view of a space seen through a channel.
func TestReads(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ctx := context.Background()
	support, marketing := e.create(t, "Support", ids.ID{}), e.create(t, "Marketing", ids.ID{})
	e.az.scope = authorize.Scope{InstanceWide: true}
	list, err := e.svc.ListSpaces(ctx, e.owner)
	if err != nil || len(list) != 2 || list[0].Level != "trigger" {
		t.Fatalf("list %+v, %v", list, err)
	}
	e.az.scope = authorize.Scope{SpaceIDs: []ids.ID{marketing.ID}}
	e.az.rules[authorize.ActionSpaceRead] = func(_ authorize.Principal, r authorize.Resource) authorize.Decision {
		if r.ID == marketing.ID {
			return allow
		}
		return authorize.Decision{Outcome: authorize.OutcomeNotFound, Reason: "no_grant"}
	}
	e.rights.r = authz.EffectiveRights{}
	list, err = e.svc.ListSpaces(ctx, e.owner)
	if err != nil || len(list) != 1 || list[0].Space.ID != marketing.ID || list[0].Level != "read" || len(list[0].Capabilities) != 0 {
		t.Fatalf("filtered list %+v, %v", list, err)
	}
	_, err = e.svc.GetSpace(ctx, e.owner, support.ID)
	wantCode(t, err, errs.CodeNotFound)
	e.az.rules[authorize.ActionSpaceRead] = nil
	_, err = e.svc.GetSpace(ctx, e.owner, ids.New())
	wantCode(t, err, errs.CodeNotFound)
	v, err := e.svc.GetSpace(ctx, e.owner, support.ID)
	if err != nil || v.Space.Name != "Support" || v.Level != "read" {
		t.Fatalf("get %+v, %v", v, err)
	}
	e.rights.err = errors.New("source down")
	if _, err := e.svc.GetSpace(ctx, e.owner, support.ID); err == nil {
		t.Fatal("rights error swallowed")
	}
	e.az.err = errors.New("authz down")
	if _, err := e.svc.ListSpaces(ctx, e.owner); err == nil {
		t.Fatal("scope error swallowed")
	}
}

// TestUpdate: If-Match, body checks, no change, conflict and the archived space.
func TestUpdate(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ctx := context.Background()
	sp := e.create(t, "Support", ids.ID{})
	e.create(t, "Research", ids.ID{})
	fresh := chat.Precondition{Present: true, Valid: true, Version: 1}
	_, err := e.svc.UpdateSpace(ctx, e.owner, sp.ID, chat.UpdateSpaceCmd{Name: ptr("Help")})
	wantCode(t, err, errs.CodePreconditionRequired)
	_, err = e.svc.UpdateSpace(ctx, e.owner, sp.ID, chat.UpdateSpaceCmd{Name: ptr("Help"), IfMatch: chat.Precondition{Present: true, Valid: true, Version: 9}})
	wantCode(t, err, errs.CodeVersionConflict)
	_, err = e.svc.UpdateSpace(ctx, e.owner, sp.ID, chat.UpdateSpaceCmd{Name: ptr("x\u0007"), IfMatch: fresh})
	wantCode(t, err, errs.CodeValidationFailed)
	_, err = e.svc.UpdateSpace(ctx, e.owner, sp.ID, chat.UpdateSpaceCmd{Name: ptr("research"), IfMatch: fresh})
	wantCode(t, err, errs.CodeConflict)
	same, err := e.svc.UpdateSpace(ctx, e.owner, sp.ID, chat.UpdateSpaceCmd{Name: ptr(" Support "), IfMatch: fresh})
	if err != nil || same.Version != 1 {
		t.Fatalf("no change %+v, %v", same, err)
	}
	up, err := e.svc.UpdateSpace(ctx, e.owner, sp.ID, chat.UpdateSpaceCmd{Name: ptr("Help"), Color: ptr("loch"), IfMatch: fresh})
	if err != nil || up.Version != 2 || up.Name != "Help" || up.Color != "loch" {
		t.Fatalf("update %+v, %v", up, err)
	}
	_, err = e.svc.UpdateSpace(ctx, e.owner, ids.New(), chat.UpdateSpaceCmd{IfMatch: fresh})
	wantCode(t, err, errs.CodeNotFound)
	_, err = e.svc.ArchiveSpace(ctx, e.owner, sp.ID, "Done")
	if err != nil {
		t.Fatal(err)
	}
	_, err = e.svc.UpdateSpace(ctx, e.owner, sp.ID, chat.UpdateSpaceCmd{Name: ptr("Desk"), IfMatch: chat.Precondition{Present: true, Valid: true, Version: 3}})
	wantCode(t, err, errs.CodeConflict)
	_, err = e.svc.UpdateSpace(actor.WithActor(ctx, actor.Actor{ID: ids.New(), Kind: actor.KindUser}), e.owner, sp.ID, chat.UpdateSpaceCmd{})
	wantCode(t, err, errs.CodeInternalError)
}

// TestMove: the positions, the field errors, the archived space and the refusal of authorize.
func TestMove(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ctx := context.Background()
	a, b, c := e.create(t, "A", ids.ID{}), e.create(t, "B", ids.ID{}), e.create(t, "C", ids.ID{})
	for _, step := range []struct {
		id   ids.ID
		cmd  chat.MoveSpaceCmd
		want []string
	}{
		{a.ID, chat.MoveSpaceCmd{AfterID: c.ID}, []string{"B", "C", "A"}},
		{a.ID, chat.MoveSpaceCmd{BeforeID: b.ID}, []string{"A", "B", "C"}},
		{c.ID, chat.MoveSpaceCmd{}, []string{"C", "A", "B"}},
		{c.ID, chat.MoveSpaceCmd{AfterID: b.ID}, []string{"A", "B", "C"}},
	} {
		if _, err := e.svc.MoveSpace(ctx, e.owner, step.id, step.cmd); err != nil {
			t.Fatal(err)
		}
		if got := e.names(); !slices.Equal(got, step.want) {
			t.Fatalf("after %+v: %v", step.cmd, got)
		}
	}
	for _, cmd := range []chat.MoveSpaceCmd{{AfterID: a.ID, BeforeID: b.ID}, {AfterID: a.ID}, {BeforeID: a.ID}, {AfterID: ids.New()}, {BeforeID: ids.New()}} {
		_, err := e.svc.MoveSpace(ctx, e.owner, a.ID, cmd)
		wantCode(t, err, errs.CodeValidationFailed)
	}
	_, err := e.svc.MoveSpace(ctx, e.owner, ids.New(), chat.MoveSpaceCmd{})
	wantCode(t, err, errs.CodeNotFound)
	if _, err := e.svc.ArchiveSpace(ctx, e.owner, b.ID, "Done"); err != nil {
		t.Fatal(err)
	}
	_, err = e.svc.MoveSpace(ctx, e.owner, b.ID, chat.MoveSpaceCmd{})
	wantCode(t, err, errs.CodeConflict)
	e.deny(authorize.ActionSpaceReorder, authorize.Decision{Outcome: authorize.OutcomeForbidden, Reason: "owner_only"})
	_, err = e.svc.MoveSpace(ctx, e.owner, a.ID, chat.MoveSpaceCmd{})
	wantCode(t, err, errs.CodeForbidden)
	_, err = e.svc.MoveSpace(actor.WithActor(ctx, actor.Actor{ID: ids.New(), Kind: actor.KindUser}), e.owner, a.ID, chat.MoveSpaceCmd{})
	wantCode(t, err, errs.CodeInternalError)
}

// TestArchive: the reason, the rights change declaration with the space as target, the in-tx
// event, a second archive without effect and the refusal of authorize.
func TestArchive(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ctx := context.Background()
	sp := e.create(t, "Support", ids.ID{})
	_, err := e.svc.ArchiveSpace(ctx, e.owner, sp.ID, " ")
	wantCode(t, err, errs.CodeValidationFailed)
	out, err := e.svc.ArchiveSpace(ctx, e.owner, sp.ID, "Team left")
	if err != nil || out.ArchivedAt.IsZero() || out.Version != 2 {
		t.Fatalf("archive %+v, %v", out, err)
	}
	last := e.uow.opts[len(e.uow.opts)-1]
	if !last.RightsChange || last.Action != "space.archive" || len(last.Targets) != 1 || last.Targets[0].ID != sp.ID {
		t.Fatalf("rights change %+v", last)
	}
	if len(*e.archived) != 1 || (*e.archived)[0].SpaceID != sp.ID {
		t.Fatalf("SpaceArchived %+v", *e.archived)
	}
	n := len(e.uow.opts)
	again, err := e.svc.ArchiveSpace(ctx, e.owner, sp.ID, "Again")
	if err != nil || again.Version != 2 || len(e.uow.opts) != n+1 {
		t.Fatalf("again %+v, %v, %d units", again, err, len(e.uow.opts)-n)
	}
	_, err = e.svc.ArchiveSpace(ctx, e.owner, ids.New(), "Done")
	wantCode(t, err, errs.CodeNotFound)
	e.deny(authorize.ActionSpaceArchive, authorize.Decision{Outcome: authorize.OutcomeUnauthenticated, Reason: "step_up_required"})
	_, err = e.svc.ArchiveSpace(ctx, e.owner, sp.ID, "Done")
	wantCode(t, err, errs.CodeUnauthenticated)
	_, err = e.svc.ArchiveSpace(actor.WithActor(ctx, actor.Actor{ID: ids.New(), Kind: actor.KindUser}), e.owner, sp.ID, "Done")
	wantCode(t, err, errs.CodeInternalError)
}

// TestSidebar: people only, the key rules, keep and replace, the device, the event to the
// person, and the port of M1-61.
func TestSidebar(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ctx := context.Background()
	support, research := e.create(t, "Support", ids.ID{}), e.create(t, "Research", ids.ID{})
	for _, kind := range []actor.Kind{actor.KindAgent, actor.KindApp} {
		_, err := e.svc.PutSidebarSpace(ctx, actor.Actor{ID: ids.New(), Kind: kind}, support.ID, chat.SidebarPutCmd{})
		wantCode(t, err, errs.CodeForbidden)
	}
	st, err := e.svc.PutSidebarSpace(ctx, e.owner, support.ID, chat.SidebarPutCmd{Collapsed: true, SortKey: ptr("a0V")})
	if err != nil || st.SortKey != "a0V" || st.Version != 1 || st.UpdatedByDeviceID != e.owner.DeviceID {
		t.Fatalf("put %+v, %v", st, err)
	}
	st, err = e.svc.PutSidebarSpace(ctx, e.owner, support.ID, chat.SidebarPutCmd{})
	if err != nil || st.SortKey != "a0V" || st.Collapsed || st.Version != 2 {
		t.Fatalf("keep %+v, %v", st, err)
	}
	st, err = e.svc.PutSidebarSpace(ctx, e.owner, support.ID, chat.SidebarPutCmd{UseGlobalOrder: true})
	if err != nil || st.SortKey != "" {
		t.Fatalf("global %+v, %v", st, err)
	}
	_, err = e.svc.PutSidebarSpace(ctx, e.owner, support.ID, chat.SidebarPutCmd{SortKey: ptr("a1"), UseGlobalOrder: true})
	wantCode(t, err, errs.CodeValidationFailed)
	_, err = e.svc.PutSidebarSpace(ctx, e.owner, support.ID, chat.SidebarPutCmd{SortKey: ptr("zz")})
	wantCode(t, err, errs.CodeValidationFailed)
	_, err = e.svc.PutSidebarSpace(ctx, e.owner, ids.New(), chat.SidebarPutCmd{})
	wantCode(t, err, errs.CodeNotFound)
	ev := e.events.evs[len(e.events.evs)-1]
	if ev.Type != "sidebar.updated" || ev.AudiencePrincipalID != e.owner.ID || ev.ResourceVersion != 0 || ev.ReceiveAction != "self" {
		t.Fatalf("event %+v", ev)
	}
	if _, err := e.svc.PutSidebarSpace(ctx, e.owner, research.ID, chat.SidebarPutCmd{Collapsed: true}); err != nil {
		t.Fatal(err)
	}
	list, err := e.svc.ListSidebarStates(ctx, e.owner)
	if err != nil || len(list) != 2 {
		t.Fatalf("list %+v, %v", list, err)
	}
	e.az.rules[authorize.ActionSpaceRead] = func(_ authorize.Principal, r authorize.Resource) authorize.Decision {
		if r.ID == research.ID {
			return allow
		}
		return authorize.Decision{Outcome: authorize.OutcomeNotFound, Reason: "no_grant"}
	}
	list, err = e.svc.ListSidebarStates(ctx, e.owner)
	if err != nil || len(list) != 1 || list[0].SpaceID != research.ID {
		t.Fatalf("filtered %+v, %v", list, err)
	}
	if list, err := e.svc.ListSidebarStates(ctx, actor.Actor{ID: ids.New(), Kind: actor.KindAgent}); err != nil || len(list) != 0 {
		t.Fatalf("agent %+v, %v", list, err)
	}
	e.deny(authorize.ActionSidebarUpdate, authorize.Decision{Outcome: authorize.OutcomeNotFound, Reason: "no_grant"})
	_, err = e.svc.PutSidebarSpace(ctx, e.owner, support.ID, chat.SidebarPutCmd{})
	wantCode(t, err, errs.CodeNotFound)
	e.store.fail = errors.New("database down")
	if _, err := e.svc.ListSidebarStates(ctx, e.owner); err == nil {
		t.Fatal("store error swallowed")
	}
	_, err = e.svc.PutSidebarSpace(actor.WithActor(ctx, actor.Actor{ID: ids.New(), Kind: actor.KindUser}), e.owner, support.ID, chat.SidebarPutCmd{})
	wantCode(t, err, errs.CodeInternalError)
}

// TestErrorsPass: errors of the store and of authorize reach the caller unchanged, and events
// name agents and apps as actors.
func TestErrorsPass(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ctx := context.Background()
	sp := e.create(t, "Support", ids.ID{})
	for _, kind := range []actor.Kind{actor.KindAgent, actor.KindApp} {
		if _, err := e.svc.CreateSpace(ctx, actor.Actor{ID: ids.New(), Kind: kind}, chat.CreateSpaceCmd{Name: "By " + string(kind)}); err != nil {
			t.Fatal(err)
		}
		if got := e.events.evs[len(e.events.evs)-1].ActorKind; string(got) != string(kind) {
			t.Fatalf("actor kind %s, want %s", got, kind)
		}
	}
	fresh := chat.Precondition{Present: true, Valid: true, Version: 1}
	e.store.fail = errors.New("database down")
	calls := map[string]func() error{
		"list": func() error { _, err := e.svc.ListSpaces(ctx, e.owner); return err },
		"get":  func() error { _, err := e.svc.GetSpace(ctx, e.owner, sp.ID); return err },
		"create": func() error {
			_, err := e.svc.CreateSpace(ctx, e.owner, chat.CreateSpaceCmd{Name: "Other"})
			return err
		},
		"update": func() error {
			_, err := e.svc.UpdateSpace(ctx, e.owner, sp.ID, chat.UpdateSpaceCmd{Name: ptr("Help"), IfMatch: fresh})
			return err
		},
		"move":    func() error { _, err := e.svc.MoveSpace(ctx, e.owner, sp.ID, chat.MoveSpaceCmd{}); return err },
		"archive": func() error { _, err := e.svc.ArchiveSpace(ctx, e.owner, sp.ID, "Done"); return err },
		"sidebar": func() error { _, err := e.svc.PutSidebarSpace(ctx, e.owner, sp.ID, chat.SidebarPutCmd{}); return err },
	}
	for name, call := range calls {
		if err := call(); err == nil {
			t.Errorf("%s: store error swallowed", name)
		}
	}
	e.store.fail = nil
	e.az.err = errors.New("authz down")
	for name, call := range calls {
		if err := call(); err == nil {
			t.Errorf("%s: authorize error swallowed", name)
		}
	}
}

// testConfig returns a valid configuration with the space name limit n.
func testConfig(n int) app.Config {
	return app.Config{SpaceNameMaxChars: n, ChannelNameMaxChars: 8, ChannelTopicMaxChars: 8, TagNameMaxChars: 8,
		HistoryBackfillLimitDefault: 20, HistoryBackfillLimitMax: 200}
}
