// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"encoding/hex"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/agents"
	"github.com/OWNER/bothy/services/server/internal/agents/adapters/authzsource"
	agentshttp "github.com/OWNER/bothy/services/server/internal/agents/adapters/httpapi"
	agentspg "github.com/OWNER/bothy/services/server/internal/agents/adapters/postgres"
	"github.com/OWNER/bothy/services/server/internal/agents/app"
	"github.com/OWNER/bothy/services/server/internal/audit/adapters/logs"
	auditpg "github.com/OWNER/bothy/services/server/internal/audit/adapters/postgres"
	auditapp "github.com/OWNER/bothy/services/server/internal/audit/app"
	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/authz/adapters/fake"
	authzpg "github.com/OWNER/bothy/services/server/internal/authz/adapters/postgres"
	authzapp "github.com/OWNER/bothy/services/server/internal/authz/app"
	identitypg "github.com/OWNER/bothy/services/server/internal/identity/adapters/postgres"
	identityapp "github.com/OWNER/bothy/services/server/internal/identity/app"
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

func TestMain(m *testing.M) { os.Exit(pgtest.Main(m)) }

// start is the clock of the use case world.
var start = time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)

// testKEK is the key-encryption key of the audit recorder in these tests.
type testKEK struct{}

func (testKEK) KEK(context.Context, string) ([]byte, error) {
	return []byte(hex.EncodeToString(make([]byte, 32))), nil
}

// world is PostgreSQL 18 with the use cases of agents over the real identity, authz (rights hooks
// with I1), sync and audit, fake credentials and the fake structure of authz (the fake-source
// harness of the brief: Assistent serves Research and Notizen).
type world struct {
	t         *testing.T
	owner     *pg.Pool
	creds     *fake.Credentials
	structure *fake.Structure
	svc       *app.Service
	id        *wire.Identity
	// uow is the unit of work of the use cases; the tests of the guided flow (M2-21) run
	// SwitchToOrganization inside it the way authz does.
	uow txn.UnitOfWork
}

// newWorld builds a world on a new database.
func newWorld(t *testing.T) *world {
	t.Helper()
	db := pgtest.New(t)
	cfg := db.Config(pg.RoleApp)
	cfg.MaxConns = 8
	pool := db.OpenPool(t, cfg)
	log := slog.New(slog.DiscardHandler)
	clk := clock.NewFake(start)
	w := &world{t: t, owner: db.Pool(t, pg.RoleOwner), creds: fake.NewCredentials(), structure: fake.NewStructure()}
	w.structure.OnPutChannel(w.mirrorChannel)
	repo := identitypg.NewRepository(pool)
	principals := identityapp.NewPrincipalSource(repo)
	source := authzsource.New(agentspg.NewRepository(pool), identityapp.NewDirectory(repo))
	hook, err := wire.NewRightsHook(pool, wire.RightsSources{Principals: principals, Structure: w.structure, Policies: source}, clk, log)
	if err != nil {
		t.Fatal(err)
	}
	mods := wire.NewModules(pool, log)
	reg := txn.NewRegistry()
	for _, h := range []struct {
		p txn.Phase
		f txn.Hook
	}{{txn.PhaseRightsBegin, hook.Begin}, {txn.PhaseRightsCheck, hook.BeforeCommit}, {txn.PhaseSync, mods.Sync.Run}, {txn.PhaseAudit, mods.Audit.Hook}} {
		if err := reg.SetHook(h.p, h.f); err != nil {
			t.Fatal(err)
		}
	}
	reg.Seal()
	uow := pg.NewUnitOfWork(pool, pg.UnitConfig{RetryMax: 3, StatementTimeout: 30 * time.Second, LockTimeout: 20 * time.Second, IdleInTxTimeout: 60 * time.Second}, reg)
	w.uow = uow
	rec, err := auditapp.NewRecorder(auditapp.Config{
		DenyAggregateWindow: time.Hour, DenyAggregateMaxKeys: 100, IPKeyPeriod: 30 * 24 * time.Hour,
		IPKeyRetention: 90 * 24 * time.Hour, IPKEKRef: "file:<secrets.dir>/audit_ip_kek",
	}, auditapp.Deps{UoW: uow, Store: auditpg.NewRepository(pool), KEK: testKEK{}, Clock: clk, Reporter: logs.New(log)})
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
		uow, syncpg.NewRepository(pool, log), clk)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := maint.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	az, err := authzapp.NewAuthorizer(authzapp.AuthorizerConfig{SnapshotCacheMaxEntries: 1000, StepUpMaxAge: 5 * time.Minute}, authzapp.AuthorizerDeps{
		Catalog: authzapp.NewCatalog(log), Store: authzpg.NewRepository(pool), Principals: principals, Credentials: w.creds,
		Structure: w.structure, Policies: source, Versions: instanceapp.NewVersions(instancepg.NewRepository(pool)), Recorder: rec, Clock: clk, Log: log,
	})
	if err != nil {
		t.Fatal(err)
	}
	idCfg := identityapp.Config{
		HandleMaxChars: 12, DisplayNameMaxChars: 20, DPoPNonceTTL: time.Minute, DPoPProofMaxAge: time.Minute, ClockLeeway: time.Second,
		PublicURL: "https://bothy.example.com", PageSizeDefault: 2, PageSizeMax: 3, StepUpMaxAge: 5 * time.Minute,
		APITokenMaxTTL: 24 * time.Hour, PersonalTokenMaxTTL: 24 * time.Hour,
		// The keys of M1-37, M1-38, M1-40, M1-41 and M1-45, which NewIdentity requires (INT-W14/8).
		EnrollmentCodeTTL: 72 * time.Hour, EnrollmentCodeMaxAttempts: 5, LoginFlowTTL: 10 * time.Minute,
		DevicePendingTTL: 24 * time.Hour, DeviceVerificationCodeDigits: 6, PairingCodeTTL: 5 * time.Minute, PairingCodeChars: 8,
		ServerCodeTTL: time.Minute, OIDCJWKSCacheTTL: time.Hour, OIDCClientAssertionTTL: 2 * time.Minute,
		IDPRecheckInterval: 30 * 24 * time.Hour, DeviceInactiveRevokeAfter: 180 * 24 * time.Hour,
	}
	// Identity takes the instance settings (M1-46) and the feature flags (M1-47) (INT-W14/8).
	settings, err := wire.NewInstanceSettings(wire.DefaultInstanceConfig(), true, pool, uow, az, wire.SettingsWrites{Audit: rec, Events: appender}, clk)
	if err != nil {
		t.Fatal(err)
	}
	writes := wire.IdentityWrites{Audit: rec, Events: appender, Settings: settings.Admin, Flags: settings.Admin}
	if w.id, err = wire.NewIdentity(idCfg, pool, uow, az, writes, []byte("0123456789abcdef0123456789abcdef"), clk, log); err != nil {
		t.Fatal(err)
	}
	w.svc, err = app.New(app.Config{PageSizeDefault: 2, PageSizeMax: 3, ReasonMaxChars: 40}, app.Deps{
		Store: agentspg.NewRepository(pool), Principals: w.id.Principals, Directory: identityapp.NewDirectory(repo), Tokens: w.id.Tokens,
		Rights: hook, UoW: uow, Authorizer: az, Audit: rec, Events: appender, Cursors: agentshttp.Cursors{}, Clock: clk,
	})
	if err != nil {
		t.Fatal(err)
	}
	return w
}

// sql runs stmt as the database owner.
func (w *world) sql(stmt string, args ...any) {
	w.t.Helper()
	if _, err := w.owner.Exec(context.Background(), stmt, args...); err != nil {
		w.t.Fatalf("%s: %v", stmt, err)
	}
}

// person inserts an active person, optionally an owner, and returns it in a session with a fresh
// step-up.
func (w *world) person(handle string, owner bool) actor.Actor {
	w.t.Helper()
	id := ids.New()
	w.sql(`INSERT INTO principals (id, kind, handle, display_name) VALUES ($1, 'user', $2, $2)`, id.String(), handle)
	w.sql(`INSERT INTO users (principal_id) VALUES ($1)`, id.String())
	if owner {
		w.sql(`INSERT INTO owners (id, user_id, reason) VALUES ($1, $2, 'first owner')`, ids.New().String(), id.String())
	}
	s, d := ids.New(), ids.New()
	w.creds.PutDevice(d, fake.Device{PrincipalID: id, State: authz.DeviceActive})
	w.creds.PutSession(s, fake.Session{PrincipalID: id, DeviceID: d, ExpiresAt: start.Add(time.Hour)})
	return actor.Actor{ID: id, Kind: actor.KindUser, SessionID: s, DeviceID: d, StepUpAt: start}
}

// create creates an agent as o and returns it.
func (w *world) create(o actor.Actor, handle string) agents.Agent {
	w.t.Helper()
	a, err := w.svc.Create(context.Background(), o, agents.CreateCmd{Handle: handle, DisplayName: handle, AdapterKind: "hermes"})
	if err != nil {
		w.t.Fatal(err)
	}
	return a
}

// match is the precondition of version v.
func match(v int64) agents.Precondition {
	return agents.Precondition{Present: true, Valid: true, Version: v}
}

// wantCode checks the public code of err.
func wantCode(t *testing.T, err error, code errs.Code) {
	t.Helper()
	e, ok := errs.As(err)
	if !ok || e.PublicCode() != code {
		t.Fatalf("error %v, want %s", err, code)
	}
}

// mirrorChannel writes the spaces and channels rows of a fake channel, created by the app
// principal structure-fixture, the targets of the foreign keys of grants (M1-52, M1-53);
// authorize still reads the fake.
func (w *world) mirrorChannel(c authz.ChannelFacts) {
	w.t.Helper()
	w.sql(`INSERT INTO principals (id, kind, handle, display_name) SELECT gen_random_uuid(), 'app', 'structure-fixture', 'Structure fixture'
		WHERE NOT EXISTS (SELECT 1 FROM principals WHERE handle = 'structure-fixture')`)
	const by = `(SELECT id FROM principals WHERE handle = 'structure-fixture')`
	var space any
	inherit := c.Kind != authz.ChannelDM
	if inherit {
		space = c.SpaceID.String()
		w.sql(`INSERT INTO spaces (id, name, sort_key, created_by) VALUES ($1::uuid, $1::text, 'a0', `+by+`) ON CONFLICT (id) DO NOTHING`, c.SpaceID.String())
	}
	w.sql(`INSERT INTO channels (id, space_id, kind, name, sort_key, operator_inherit, history_backfill_limit, created_by)
		VALUES ($1::uuid, $2, $3, $1::text, 'a0', $4, 20, `+by+`) ON CONFLICT (id) DO NOTHING`, c.ID.String(), space, string(c.Kind), inherit)
}
