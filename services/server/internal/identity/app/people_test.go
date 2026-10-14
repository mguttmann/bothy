// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"encoding/hex"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/audit/adapters/logs"
	auditpg "github.com/OWNER/bothy/services/server/internal/audit/adapters/postgres"
	auditapp "github.com/OWNER/bothy/services/server/internal/audit/app"
	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/authz/adapters/fake"
	authzpg "github.com/OWNER/bothy/services/server/internal/authz/adapters/postgres"
	authzapp "github.com/OWNER/bothy/services/server/internal/authz/app"
	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/adapters/httpapi"
	"github.com/OWNER/bothy/services/server/internal/identity/adapters/locale"
	"github.com/OWNER/bothy/services/server/internal/identity/adapters/postgres"
	"github.com/OWNER/bothy/services/server/internal/identity/app"
	instancepg "github.com/OWNER/bothy/services/server/internal/instance/adapters/postgres"
	instanceapp "github.com/OWNER/bothy/services/server/internal/instance/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
	syncpg "github.com/OWNER/bothy/services/server/internal/sync/adapters/postgres"
	"github.com/OWNER/bothy/services/server/internal/sync/adapters/telemetry"
	syncapp "github.com/OWNER/bothy/services/server/internal/sync/app"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

var peopleStart = time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)

type kek struct{}

func (kek) KEK(context.Context, string) ([]byte, error) {
	return []byte(hex.EncodeToString(make([]byte, 32))), nil
}

var peopleCfg = app.Config{HandleMaxChars: 12, DisplayNameMaxChars: 20, PageSizeDefault: 2, PageSizeMax: 3, StepUpMaxAge: 5 * time.Minute}

// peopleWorld drives the people and owner use cases directly over PostgreSQL 18 with the rights
// hooks, sync and audit (the HTTP tests in adapters/httpapi run the same through the chain).
type peopleWorld struct {
	t        *testing.T
	owner    *pg.Pool
	p        *app.People
	clk      *clock.Fake
	creds    *fake.Credentials
	policies *fake.Policies
	str      *fake.Structure
	// pool, uow, rec, az and events are the parts the login service of login_test.go reuses.
	pool   *pg.Pool
	uow    *pg.UnitOfWork
	rec    *auditapp.Recorder
	az     *authzapp.Authorizer
	events *syncapp.Appender
	// repo and appender serve the worlds built on this one (lifecycle, M1-45).
	repo     *postgres.Repository
	appender *syncapp.Appender
}

func newPeopleWorld(t *testing.T) *peopleWorld {
	t.Helper()
	db := pgtest.New(t)
	cfg := db.Config(pg.RoleApp)
	cfg.MaxConns = 8
	pool := db.OpenPool(t, cfg)
	log := slog.New(slog.DiscardHandler)
	w := &peopleWorld{t: t, owner: db.Pool(t, pg.RoleOwner), clk: clock.NewFake(peopleStart), creds: fake.NewCredentials(),
		policies: fake.NewPolicies(), str: fake.NewStructure()}
	repo := postgres.NewRepository(pool)
	principals := app.NewPrincipalSource(repo)
	hook, err := wire.NewRightsHook(pool, wire.RightsSources{Principals: principals, Structure: w.str, Policies: w.policies}, w.clk, log)
	if err != nil {
		t.Fatal(err)
	}
	mods := wire.NewModules(pool, log)
	reg := txn.NewRegistry()
	for p, h := range map[txn.Phase]txn.Hook{txn.PhaseRightsBegin: hook.Begin, txn.PhaseRightsCheck: hook.BeforeCommit, txn.PhaseSync: mods.Sync.Run, txn.PhaseAudit: mods.Audit.Hook} {
		if err := reg.SetHook(p, h); err != nil {
			t.Fatal(err)
		}
	}
	reg.Seal()
	uow := pg.NewUnitOfWork(pool, pg.UnitConfig{RetryMax: 3, StatementTimeout: 30 * time.Second, LockTimeout: 20 * time.Second, IdleInTxTimeout: 60 * time.Second}, reg)
	rec, err := auditapp.NewRecorder(auditapp.Config{
		DenyAggregateWindow: time.Hour, DenyAggregateMaxKeys: 100, IPKeyPeriod: 30 * 24 * time.Hour,
		IPKeyRetention: 90 * 24 * time.Hour, IPKEKRef: "file:<secrets.dir>/audit_ip_kek",
	}, auditapp.Deps{UoW: uow, Store: auditpg.NewRepository(pool), KEK: kek{}, Clock: w.clk, Reporter: logs.New(log)})
	if err != nil {
		t.Fatal(err)
	}
	appender, err := syncapp.NewAppender(syncapp.AppenderConfig{PayloadMaxBytes: 65536}, telemetry.Correlator{})
	if err != nil {
		t.Fatal(err)
	}
	if err := hook.Bind(authzapp.RightsBindings{Events: appender, Audit: rec}); err != nil {
		t.Fatal(err)
	}
	maint, err := syncapp.NewPartitionMaintenance(syncapp.PartitionConfig{Size: 1000, Ahead: 2, Retention: 30 * 24 * time.Hour, LockTimeout: 2 * time.Second},
		uow, syncpg.NewRepository(pool, log), w.clk)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := maint.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	versions := instanceapp.NewVersions(instancepg.NewRepository(pool))
	az, err := authzapp.NewAuthorizer(authzapp.AuthorizerConfig{SnapshotCacheMaxEntries: 100, StepUpMaxAge: peopleCfg.StepUpMaxAge}, authzapp.AuthorizerDeps{
		Catalog: authzapp.NewCatalog(log), Store: authzpg.NewRepository(pool), Principals: principals, Credentials: w.creds,
		Structure: w.str, Policies: w.policies, Versions: versions, Recorder: rec, Clock: w.clk, Log: log,
	})
	if err != nil {
		t.Fatal(err)
	}
	codes, err := app.NewEnrollmentCodes(app.Config{EnrollmentCodeTTL: time.Hour, EnrollmentCodeMaxAttempts: 3}, []byte("0123456789abcdef0123456789abcdef"),
		app.EnrollmentDeps{Codes: repo, People: repo, UoW: uow, Clock: w.clk})
	if err != nil {
		t.Fatal(err)
	}
	st, err := wire.NewInstanceSettings(wire.DefaultInstanceConfig(), true, pool, uow, az, wire.SettingsWrites{Audit: rec, Events: appender}, w.clk)
	if err != nil {
		t.Fatal(err)
	}
	w.pool, w.uow, w.rec, w.az, w.events = pool, uow, rec, az, appender
	w.repo, w.appender = repo, appender
	w.p, err = app.NewPeople(peopleCfg, app.PeopleDeps{Store: repo, UoW: uow, Authorizer: az, Audit: rec, Events: appender,
		Cursors: httpapi.Cursors{}, Versions: versions, Locales: locale.Parser{}, Providers: repo, Identities: repo, Enrollment: codes,
		Settings: st.Admin, Clock: w.clk})
	if err != nil {
		t.Fatal(err)
	}
	return w
}

func (w *peopleWorld) sql(stmt string, args ...any) {
	w.t.Helper()
	if _, err := w.owner.Exec(context.Background(), stmt, args...); err != nil {
		w.t.Fatalf("%s: %v", stmt, err)
	}
}

// person inserts a person and returns it as an actor in a session with a fresh step-up.
func (w *peopleWorld) person(handle, status string, owner bool) actor.Actor {
	w.t.Helper()
	id := ids.New()
	w.sql(`INSERT INTO principals (id, kind, status, handle, display_name) VALUES ($1, 'user', $2, $3, $3)`, id.String(), status, handle)
	w.sql(`INSERT INTO users (principal_id, email) VALUES ($1, $2)`, id.String(), handle+"@example.com")
	if owner {
		w.sql(`INSERT INTO owners (id, user_id, reason) VALUES ($1, $2, 'first')`, ids.New().String(), id.String())
	}
	s, d := ids.New(), ids.New()
	w.creds.PutDevice(d, fake.Device{PrincipalID: id, State: authz.DeviceActive})
	w.creds.PutSession(s, fake.Session{PrincipalID: id, DeviceID: d, ExpiresAt: peopleStart.Add(time.Hour)})
	return actor.Actor{ID: id, Kind: actor.KindUser, SessionID: s, DeviceID: d, StepUpAt: w.clk.Now()}
}

func (w *peopleWorld) agent(handle string) actor.Actor {
	w.t.Helper()
	id, tok := ids.New(), ids.New()
	w.sql(`INSERT INTO principals (id, kind, handle, display_name) VALUES ($1, 'agent', $2, $2)`, id.String(), handle)
	w.creds.PutToken(tok, fake.Token{PrincipalID: id})
	return actor.Actor{ID: id, Kind: actor.KindAgent, TokenID: tok}
}

func ctxOf(a actor.Actor) context.Context { return actor.WithActor(context.Background(), a) }

func wantCode(t *testing.T, err error, code errs.Code) {
	t.Helper()
	if e, ok := errs.As(err); !ok || e.Code != code {
		t.Fatalf("err %v, want %s", err, code)
	}
}

func ptr(s string) *string { return &s }

func TestNewPeople_RefusesMissingParts(t *testing.T) {
	t.Parallel()
	if _, err := app.NewPeople(peopleCfg, app.PeopleDeps{}); err == nil {
		t.Fatal("NewPeople without collaborators")
	}
}

func TestPeople_Lifecycle(t *testing.T) {
	t.Parallel()
	w := newPeopleWorld(t)
	owner := w.person("owner", "active", true)
	ctx := ctxOf(owner)
	created, err := w.p.CreateUser(ctx, owner, identity.CreateUserCmd{Handle: "kim", DisplayName: "Kim", Email: ptr("kim@example.com"), Locale: ptr("en-us"), TimeZone: ptr("Europe/Berlin")})
	u := created.User
	if err != nil || u.Locale != "en-US" || u.Status != identity.StatusActive || created.Enrollment != nil {
		t.Fatalf("CreateUser = %+v, %v", created, err)
	}
	_, err = w.p.CreateUser(ctx, owner, identity.CreateUserCmd{Handle: "kim", DisplayName: "Kim"})
	wantCode(t, err, errs.CodeConflict)
	// Without an enabled local provider a local account is 409 local_provider_disabled (M1-37).
	_, err = w.p.CreateUser(ctx, owner, identity.CreateUserCmd{Handle: "x", DisplayName: "X", LocalAccount: true})
	wantCode(t, err, errs.CodeConflict)
	if e, _ := errs.As(err); e.Reason != "local_provider_disabled" {
		t.Fatalf("reason %q, want local_provider_disabled", e.Reason)
	}
	for _, cmd := range []identity.CreateUserCmd{
		{Handle: "Bad", DisplayName: "X"}, {Handle: "y", DisplayName: " "}, {Handle: "y", DisplayName: "Y", Email: ptr("no")},
		{Handle: "y", DisplayName: "Y", Locale: ptr("--")}, {Handle: "y", DisplayName: "Y", TimeZone: ptr("Local")},
		{Handle: "y", DisplayName: "Y", Locale: ptr(strings.Repeat("a", 40))},
	} {
		_, err := w.p.CreateUser(ctx, owner, cmd)
		wantCode(t, err, errs.CodeValidationFailed)
	}
	// Update with and without change.
	got, err := w.p.UpdateUser(ctx, owner, u.ID, identity.UpdateUserCmd{DisplayName: ptr("Kim K"), TimeZone: ptr("UTC")})
	if err != nil || got.DisplayName != "Kim K" || got.TimeZone != "UTC" {
		t.Fatalf("UpdateUser = %+v, %v", got, err)
	}
	if _, err := w.p.UpdateUser(ctx, owner, u.ID, identity.UpdateUserCmd{DisplayName: ptr("Kim K")}); err != nil {
		t.Fatal(err)
	}
	_, err = w.p.UpdateUser(ctx, owner, ids.New(), identity.UpdateUserCmd{})
	wantCode(t, err, errs.CodeNotFound)
	_, err = w.p.UpdateUser(ctx, owner, u.ID, identity.UpdateUserCmd{Email: ptr("owner@example.com")})
	wantCode(t, err, errs.CodeConflict)
	// Read.
	if g, err := w.p.GetUser(ctx, owner, u.ID); err != nil || g.Handle != "kim" {
		t.Fatalf("GetUser = %+v, %v", g, err)
	}
	_, err = w.p.GetUser(ctx, owner, ids.New())
	wantCode(t, err, errs.CodeNotFound)
	// Enable: pending, disabled, active (no effect), deleted.
	pend := w.person("pia", "pending", false)
	if g, err := w.p.EnableUser(ctx, owner, pend.ID, "ok"); err != nil || g.Status != identity.StatusActive {
		t.Fatalf("EnableUser pending = %+v, %v", g, err)
	}
	if g, err := w.p.EnableUser(ctx, owner, pend.ID, "again"); err != nil || g.Status != identity.StatusActive {
		t.Fatalf("EnableUser active = %+v, %v", g, err)
	}
	del := w.person("del", "deleted", false)
	_, err = w.p.EnableUser(ctx, owner, del.ID, "x")
	wantCode(t, err, errs.CodeInvalidStateTransition)
	_, err = w.p.EnableUser(ctx, owner, pend.ID, " ")
	wantCode(t, err, errs.CodeValidationFailed)
	_, err = w.p.EnableUser(ctx, owner, ids.New(), "x")
	wantCode(t, err, errs.CodeNotFound)
}

func TestOwners_AddRevokeAndInvariants(t *testing.T) {
	t.Parallel()
	w := newPeopleWorld(t)
	owner := w.person("owner", "active", true)
	alex := w.person("alex", "active", false)
	ctx := ctxOf(owner)
	o, err := w.p.AddOwner(ctx, owner, alex.ID, "Deputy")
	if err != nil || o.User.ID != alex.ID || o.GrantedBy != owner.ID {
		t.Fatalf("AddOwner = %+v, %v", o, err)
	}
	_, err = w.p.AddOwner(ctx, owner, alex.ID, "again")
	wantCode(t, err, errs.CodeConflict)
	_, err = w.p.AddOwner(ctx, owner, ids.New(), "x")
	wantCode(t, err, errs.CodeNotFound)
	dora := w.person("dora", "disabled", false)
	_, err = w.p.AddOwner(ctx, owner, dora.ID, "x")
	wantCode(t, err, errs.CodeConflict)
	_, err = w.p.AddOwner(ctx, owner, alex.ID, "")
	wantCode(t, err, errs.CodeValidationFailed)
	list, err := w.p.ListOwners(ctx, owner)
	if err != nil || len(list) != 2 {
		t.Fatalf("ListOwners = %v, %v", list, err)
	}
	if err := w.p.RevokeOwner(ctx, owner, alex.ID, "Done"); err != nil {
		t.Fatal(err)
	}
	wantCode(t, w.p.RevokeOwner(ctx, owner, alex.ID, "Done"), errs.CodeNotFound)
	wantCode(t, w.p.RevokeOwner(ctx, owner, owner.ID, "Leaving"), errs.CodeLastOwner)
	wantCode(t, w.p.RevokeOwner(ctx, owner, owner.ID, ""), errs.CodeValidationFailed)
	// I1: a personal agent of the owner on a served channel.
	bot := w.agent("assistent")
	w.policies.Put(bot.ID, authz.AgentPolicy{BillingMode: "personal", CredentialHolderID: owner.ID, ToolClass: "scoped"})
	space, ch := ids.New(), ids.New()
	w.str.PutSpace(space)
	w.str.PutChannel(authz.ChannelFacts{ID: ch, SpaceID: space, Kind: authz.ChannelText, EffectiveOperatorID: bot.ID})
	_, err = w.p.AddOwner(ctx, owner, alex.ID, "Deputy")
	var inv *authz.InvariantError
	if !errors.As(err, &inv) || inv.Code != errs.CodeI1Violation {
		t.Fatalf("AddOwner with a personal agent = %v", err)
	}
	// A caller other than the request principal is a programming error.
	_, err = w.p.AddOwner(ctxOf(alex), owner, alex.ID, "x")
	wantCode(t, err, errs.CodeInternalError)
}

func TestMeAndProfile(t *testing.T) {
	t.Parallel()
	w := newPeopleWorld(t)
	owner := w.person("owner", "active", true)
	me, err := w.p.GetMe(ctxOf(owner), owner)
	if err != nil || !me.IsOwner || me.Email != "owner@example.com" || me.StepUpValidUntil.IsZero() || me.SessionID != owner.SessionID {
		t.Fatalf("GetMe = %+v, %v", me, err)
	}
	stale := owner
	stale.StepUpAt = w.clk.Now().Add(-time.Hour)
	if me, err := w.p.GetMe(ctxOf(stale), stale); err != nil || !me.StepUpValidUntil.IsZero() {
		t.Fatalf("GetMe stale = %+v, %v", me, err)
	}
	bot := w.agent("lotse")
	if me, err := w.p.GetMe(ctxOf(bot), bot); err != nil || me.Email != "" || me.Principal.Kind != actor.KindAgent {
		t.Fatalf("GetMe agent = %+v, %v", me, err)
	}
	_, err = w.p.UpdateMyProfile(ctxOf(bot), bot, identity.ProfileCmd{DisplayName: ptr("Bot")})
	wantCode(t, err, errs.CodeForbidden)
	u, err := w.p.UpdateMyProfile(ctxOf(owner), owner, identity.ProfileCmd{DisplayName: ptr("Own"), Locale: ptr("fr"), TimeZone: ptr("UTC")})
	if err != nil || u.DisplayName != "Own" || u.Locale != "fr" {
		t.Fatalf("UpdateMyProfile = %+v, %v", u, err)
	}
	if _, err := w.p.UpdateMyProfile(ctxOf(owner), owner, identity.ProfileCmd{DisplayName: ptr("Own")}); err != nil {
		t.Fatal(err)
	}
	_, err = w.p.UpdateMyProfile(ctxOf(owner), owner, identity.ProfileCmd{TimeZone: ptr("Mars/Base")})
	wantCode(t, err, errs.CodeValidationFailed)
}

func TestListUsers_Paging(t *testing.T) {
	t.Parallel()
	w := newPeopleWorld(t)
	anna := w.person("anna", "active", false)
	for _, h := range []string{"ben", "carl"} {
		w.person(h, "active", false)
	}
	w.person("dora", "disabled", false)
	ctx := ctxOf(anna)
	p1, err := w.p.ListUsers(ctx, anna, identity.ListUsersQuery{})
	if err != nil || len(p1.Items) != 2 || p1.NextCursor == "" {
		t.Fatalf("page 1 = %+v, %v", p1, err)
	}
	p2, err := w.p.ListUsers(ctx, anna, identity.ListUsersQuery{Cursor: p1.NextCursor, Limit: 10})
	if err != nil || len(p2.Items) != 2 || p2.NextCursor != "" {
		t.Fatalf("page 2 = %+v, %v", p2, err)
	}
	_, err = w.p.ListUsers(ctx, anna, identity.ListUsersQuery{Cursor: p1.NextCursor, Status: identity.StatusActive})
	wantCode(t, err, errs.CodeCursorInvalid)
	_, err = w.p.ListUsers(ctx, anna, identity.ListUsersQuery{Status: "gone"})
	wantCode(t, err, errs.CodeValidationFailed)
	_, err = w.p.ListUsers(ctx, anna, identity.ListUsersQuery{Q: "a\x00"})
	wantCode(t, err, errs.CodeValidationFailed)
	if p, err := w.p.ListUsers(ctx, anna, identity.ListUsersQuery{Status: identity.StatusDisabled}); err != nil || len(p.Items) != 1 {
		t.Fatalf("status filter = %+v, %v", p, err)
	}
	bot := w.agent("lotse")
	_, err = w.p.ListUsers(ctxOf(bot), bot, identity.ListUsersQuery{})
	wantCode(t, err, errs.CodeForbidden)
	_, err = w.p.ListOwners(ctxOf(bot), bot)
	wantCode(t, err, errs.CodeForbidden)
}
