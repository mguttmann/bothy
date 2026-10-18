// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/audit/adapters/logs"
	auditpg "github.com/OWNER/bothy/services/server/internal/audit/adapters/postgres"
	auditapp "github.com/OWNER/bothy/services/server/internal/audit/app"
	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/authz/adapters/fake"
	authzhttp "github.com/OWNER/bothy/services/server/internal/authz/adapters/httpapi"
	authzpg "github.com/OWNER/bothy/services/server/internal/authz/adapters/postgres"
	"github.com/OWNER/bothy/services/server/internal/authz/app"
	"github.com/OWNER/bothy/services/server/internal/authz/domain"
	identitypg "github.com/OWNER/bothy/services/server/internal/identity/adapters/postgres"
	identityapp "github.com/OWNER/bothy/services/server/internal/identity/app"
	instancepg "github.com/OWNER/bothy/services/server/internal/instance/adapters/postgres"
	instanceapp "github.com/OWNER/bothy/services/server/internal/instance/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
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

// pgStart is the clock of the PostgreSQL world.
var pgStart = time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)

// testKEK is the key-encryption key of the audit recorder in these tests.
type testKEK struct{}

func (testKEK) KEK(context.Context, string) ([]byte, error) {
	return []byte(hex.EncodeToString(make([]byte, 32))), nil
}

// pgWorld is the world of 07 section 11.1 on PostgreSQL 18: people, agents and the app as rows of
// identity (read through the real PrincipalSource), groups, memberships and grants of authz, the
// real unit of work with the rights hooks, sync and audit, and fakes for structure, agent policy
// and credentials (their modules come later).
type pgWorld struct {
	db         *pgtest.DB
	pool       *pg.Pool
	uow        *pg.UnitOfWork
	hook       *app.RightsHook
	rec        *auditapp.Recorder
	az         *app.Authorizer
	admin      *app.AdminService
	store      *authzpg.Repository
	principals authz.PrincipalSource
	structure  *fake.Structure
	policies   *fake.Policies
	creds      *fake.Credentials
	clk        *clock.Fake
	cfg        app.AuthorizerConfig
	appender   *syncapp.Appender

	subMu sync.Mutex
	seen  []authorize.AccessChanged
	veto  error

	owner, alex, sam, robin                ids.ID
	lotse, texter, runner, assistent       ids.ID
	tracker                                ids.ID
	support, marketing, engineering, resea ids.ID
	allgemein, eskalation, kampagnen       ids.ID
	projektA, werkzeuge, notizen           ids.ID
	leser                                  ids.ID
	g1, g2, g3, g4, g5, g6                 ids.ID

	sessions, devices, tokens map[ids.ID]ids.ID
}

// onAccessChanged is the test subscriber of authorize.AccessChanged: it keeps every event and
// returns the configured veto. It checks that it runs inside the transaction in phase 3.1.
func (w *pgWorld) onAccessChanged(ctx context.Context, ev authorize.AccessChanged) error {
	w.subMu.Lock()
	defer w.subMu.Unlock()
	if txn.CurrentPhase(ctx) != txn.PhaseRightsCheck {
		return errors.New("AccessChanged outside the rights check phase")
	}
	if _, err := pg.Tx(ctx); err != nil {
		return errors.New("AccessChanged outside the transaction")
	}
	w.seen = append(w.seen, ev)
	return w.veto
}

// setVeto makes the subscriber veto with err (nil for none).
func (w *pgWorld) setVeto(err error) {
	w.subMu.Lock()
	w.veto = err
	w.subMu.Unlock()
}

// published returns the AccessChanged events seen so far.
func (w *pgWorld) published() []authorize.AccessChanged {
	w.subMu.Lock()
	defer w.subMu.Unlock()
	return slices.Clone(w.seen)
}

func newPGWorld(t *testing.T) *pgWorld {
	t.Helper()
	db := pgtest.New(t)
	cfg := db.Config(pg.RoleApp)
	cfg.MaxConns = 8
	pool := db.OpenPool(t, cfg)
	log := slog.New(slog.DiscardHandler)
	w := &pgWorld{
		db: db, pool: pool, clk: clock.NewFake(pgStart), structure: fake.NewStructure(), policies: fake.NewPolicies(),
		creds: fake.NewCredentials(), store: authzpg.NewRepository(pool),
		principals: identityapp.NewPrincipalSource(identitypg.NewRepository(pool)),
		cfg:        app.AuthorizerConfig{SnapshotCacheMaxEntries: 1000, StepUpMaxAge: 5 * time.Minute, ViewAsMaxDuration: viewAsMax},
		sessions:   map[ids.ID]ids.ID{}, devices: map[ids.ID]ids.ID{}, tokens: map[ids.ID]ids.ID{},
	}
	hook, err := wire.NewRightsHook(pool, wire.RightsSources{Principals: w.principals, Structure: w.structure, Policies: w.policies}, w.clk, log)
	if err != nil {
		t.Fatal(err)
	}
	w.hook = hook
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
	if err := txn.Subscribe(reg, w.onAccessChanged); err != nil {
		t.Fatal(err)
	}
	reg.Seal()
	w.uow = pg.NewUnitOfWork(pool, pg.UnitConfig{RetryMax: 3, StatementTimeout: 30 * time.Second, LockTimeout: 20 * time.Second, IdleInTxTimeout: 60 * time.Second}, reg)
	w.rec, err = auditapp.NewRecorder(auditapp.Config{
		DenyAggregateWindow: time.Hour, DenyAggregateMaxKeys: 100, IPKeyPeriod: 30 * 24 * time.Hour,
		IPKeyRetention: 90 * 24 * time.Hour, IPKEKRef: "file:<secrets.dir>/audit_ip_kek",
	}, auditapp.Deps{UoW: w.uow, Store: auditpg.NewRepository(pool), KEK: testKEK{}, Clock: w.clk, Reporter: logs.New(log)})
	if err != nil {
		t.Fatal(err)
	}
	appender, err := syncapp.NewAppender(syncapp.AppenderConfig{PayloadMaxBytes: 65536}, telemetry.Correlator{})
	if err != nil {
		t.Fatal(err)
	}
	w.appender = appender
	if err := hook.Bind(app.RightsBindings{Events: appender, Audit: w.rec}); err != nil {
		t.Fatal(err)
	}
	maint, err := syncapp.NewPartitionMaintenance(syncapp.PartitionConfig{Size: 1000, Ahead: 2, Retention: 30 * 24 * time.Hour, LockTimeout: 2 * time.Second},
		w.uow, syncpg.NewRepository(pool, log), w.clk)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := maint.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	w.az = w.newAuthorizer(t)
	w.admin, err = app.NewAdminService(app.AdminDeps{
		UoW: w.uow, Authorizer: w.az, Store: w.store, Principals: w.principals, Structure: w.structure, Rights: hook,
		Audit: w.rec, Clock: w.clk, Events: w.appender, Scopes: w.az, Cursors: authzhttp.Cursors{},
		Config: app.AdminConfig{GroupNameMaxChars: 100, GroupDescriptionMaxChars: 500, PageSizeDefault: 2, PageSizeMax: 3},
	})
	if err != nil {
		t.Fatal(err)
	}
	w.seed(t)
	return w
}

// newAuthorizer returns an Authorizer with its own cache over the world.
func (w *pgWorld) newAuthorizer(t *testing.T) *app.Authorizer {
	t.Helper()
	az, err := app.NewAuthorizer(w.cfg, app.AuthorizerDeps{
		Catalog: app.NewCatalog(slog.New(slog.DiscardHandler)), Store: w.store, Principals: w.principals,
		Credentials: w.creds, Structure: w.structure, Policies: w.policies,
		Versions: instanceapp.NewVersions(instancepg.NewRepository(w.pool)), Recorder: w.rec, Clock: w.clk,
		Log: slog.New(slog.DiscardHandler),
	})
	if err != nil {
		t.Fatal(err)
	}
	return az
}

// sql runs a statement as bothy_app outside a unit of work (fixtures).
func (w *pgWorld) sql(t *testing.T, stmt string, args ...any) {
	t.Helper()
	if _, err := w.pool.Exec(context.Background(), stmt, args...); err != nil {
		t.Fatalf("%s: %v", stmt, err)
	}
}

// txSQL runs a statement in the ambient transaction of ctx (test use cases).
func txSQL(ctx context.Context, stmt string, args ...any) error {
	tx, err := pg.Tx(ctx)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, stmt, args...)
	return err
}

// person inserts an active person with handle and a session with device.
func (w *pgWorld) person(t *testing.T, handle string) ids.ID {
	t.Helper()
	id := ids.New()
	w.sql(t, `INSERT INTO principals (id, kind, handle, display_name) VALUES ($1, 'user', $2, $2)`, id.String(), handle)
	w.sql(t, `INSERT INTO users (principal_id) VALUES ($1)`, id.String())
	s, d := ids.New(), ids.New()
	w.sessions[id], w.devices[id] = s, d
	w.creds.PutDevice(d, fake.Device{PrincipalID: id, State: authz.DeviceActive})
	w.creds.PutSession(s, fake.Session{PrincipalID: id, DeviceID: d, ExpiresAt: pgStart.Add(365 * 24 * time.Hour)})
	return id
}

// bot inserts an active agent or app with a token.
func (w *pgWorld) bot(t *testing.T, kind, handle string) ids.ID {
	t.Helper()
	id := ids.New()
	w.sql(t, `INSERT INTO principals (id, kind, handle, display_name) VALUES ($1, $2, $3, $3)`, id.String(), kind, handle)
	tok := ids.New()
	w.tokens[id] = tok
	w.creds.PutToken(tok, fake.Token{PrincipalID: id})
	return id
}

// makeOwner inserts an active owners row for person.
func (w *pgWorld) makeOwner(t *testing.T, person ids.ID) {
	t.Helper()
	w.sql(t, `INSERT INTO owners (id, user_id, reason) VALUES ($1, $2, 'setup')`, ids.New().String(), person.String())
}

// change runs fn in a unit of work declared as a rights change of action with principals.
func (w *pgWorld) change(ctx context.Context, action string, res txn.Ref, principals []ids.ID, fn func(ctx context.Context) error) error {
	return w.uow.Do(ctx, fn, txn.DeclareRightsChange(txn.RightsChangeSpec{Action: action, Resource: res, Principals: principals}))
}

// grantRow inserts a grant through the store in a rights change (the real path, hooks included).
func (w *pgWorld) grantRow(t *testing.T, s domain.Subject, tg domain.Target, l domain.Level, exp time.Time, caps ...domain.Capability) ids.ID {
	t.Helper()
	g, err := domain.NewGrant(ids.New(), s, tg, l, domain.CapsOf(caps...), exp, "setup", w.owner, w.clk.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := w.change(context.Background(), "grant.create", txn.Ref{Kind: "grant", ID: g.ID}, []ids.ID{s.PrincipalID}, func(ctx context.Context) error {
		return w.store.InsertGrant(ctx, g)
	}); err != nil {
		t.Fatalf("seed grant: %v", err)
	}
	return g.ID
}

// mirrorChannel writes the spaces and channels rows of a fake channel, the targets of the foreign
// keys of grants (M1-52, M1-53); authorize still reads the fake.
func (w *pgWorld) mirrorChannel(t *testing.T, c authz.ChannelFacts) {
	t.Helper()
	var space any
	kind, inherit := string(c.Kind), true
	if c.Kind == authz.ChannelDM {
		inherit = false
	} else {
		space = c.SpaceID.String()
		w.sql(t, `INSERT INTO spaces (id, name, sort_key, created_by) VALUES ($1::uuid, $1::text, 'a0', $2) ON CONFLICT (id) DO NOTHING`, c.SpaceID.String(), w.owner.String())
	}
	w.sql(t, `INSERT INTO channels (id, space_id, kind, name, sort_key, operator_inherit, history_backfill_limit, created_by)
		VALUES ($1::uuid, $2, $3, $1::text, 'a0', $4, 20, $5) ON CONFLICT (id) DO NOTHING`, c.ID.String(), space, kind, inherit, w.owner.String())
}

// seed builds the world of 07 sections 10 and 11.1.
func (w *pgWorld) seed(t *testing.T) {
	t.Helper()
	w.owner, w.alex, w.sam, w.robin = w.person(t, "owner"), w.person(t, "alex"), w.person(t, "sam"), w.person(t, "robin")
	w.makeOwner(t, w.owner)
	w.structure.OnPutChannel(func(c authz.ChannelFacts) { w.mirrorChannel(t, c) })
	w.lotse, w.texter, w.runner, w.assistent = w.bot(t, "agent", "lotse"), w.bot(t, "agent", "texter"), w.bot(t, "agent", "runner"), w.bot(t, "agent", "assistent")
	w.tracker = w.bot(t, "app", "tracker")
	w.policies.Put(w.lotse, authz.AgentPolicy{BillingMode: domain.BillingOrganization, ToolClass: domain.ToolChat})
	w.policies.Put(w.texter, authz.AgentPolicy{BillingMode: domain.BillingOrganization, ToolClass: domain.ToolScoped})
	w.policies.Put(w.runner, authz.AgentPolicy{BillingMode: domain.BillingOrganization, ToolClass: domain.ToolHost})
	w.policies.Put(w.assistent, authz.AgentPolicy{BillingMode: domain.BillingPersonal, CredentialHolderID: w.owner, ToolClass: domain.ToolScoped})
	for i, s := range []*ids.ID{&w.support, &w.marketing, &w.engineering, &w.resea} {
		*s = ids.New()
		w.structure.PutSpace(*s)
		// The spaces row behind the foreign key of grants.target_space_id (M1-52).
		w.sql(t, `INSERT INTO spaces (id, name, sort_key, created_by) VALUES ($1, $2, $3, $4)`, s.String(), s.String(), "a"+string(rune('0'+i)), w.owner.String())
	}
	ch := func(id *ids.ID, space ids.ID, kind authz.ChannelKind, op ids.ID) {
		*id = ids.New()
		w.structure.PutChannel(authz.ChannelFacts{ID: *id, SpaceID: space, Kind: kind, EffectiveOperatorID: op})
	}
	ch(&w.allgemein, w.support, authz.ChannelText, w.lotse)
	ch(&w.eskalation, w.support, authz.ChannelText, w.lotse)
	ch(&w.kampagnen, w.marketing, authz.ChannelText, w.texter)
	ch(&w.projektA, w.engineering, authz.ChannelForum, w.runner)
	ch(&w.werkzeuge, w.engineering, authz.ChannelText, w.runner)
	ch(&w.notizen, w.resea, authz.ChannelText, w.assistent)
	g, err := domain.NewGroup(ids.New(), "Leser", "", domain.GroupLimits{}, w.owner, pgStart)
	if err != nil {
		t.Fatal(err)
	}
	w.leser = g.ID
	m, err := domain.NewMembership(ids.New(), g.ID, w.sam, w.owner, pgStart.Add(48*time.Hour), pgStart)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.change(context.Background(), "group.member.add", txn.Ref{Kind: "group", ID: g.ID}, []ids.ID{w.sam}, func(ctx context.Context) error {
		if err := w.store.InsertGroup(ctx, g); err != nil {
			return err
		}
		return w.store.InsertMembership(ctx, m)
	}); err != nil {
		t.Fatal(err)
	}
	user := func(id ids.ID) domain.Subject { return domain.Subject{Kind: domain.SubjectUser, PrincipalID: id} }
	w.g1 = w.grantRow(t, user(w.alex), domain.Target{Kind: domain.TargetSpace, SpaceID: w.support}, domain.LevelTrigger, time.Time{}, domain.CapUpload, domain.CapApproveActions)
	w.g2 = w.grantRow(t, user(w.sam), domain.Target{Kind: domain.TargetSpace, SpaceID: w.marketing}, domain.LevelTrigger, time.Time{}, domain.CapUpload)
	w.g3 = w.grantRow(t, user(w.robin), domain.Target{Kind: domain.TargetChannel, ChannelID: w.projektA}, domain.LevelTrigger, time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC), domain.CapUpload)
	w.g4 = w.grantRow(t, domain.Subject{Kind: domain.SubjectGroup, GroupID: w.leser}, domain.Target{Kind: domain.TargetChannel, ChannelID: w.allgemein}, domain.LevelRead, time.Time{})
	w.g5 = w.grantRow(t, domain.Subject{Kind: domain.SubjectAgent, PrincipalID: w.lotse}, domain.Target{Kind: domain.TargetSpace, SpaceID: w.support}, domain.LevelWrite, time.Time{}, domain.CapUpload)
	w.g6 = w.grantRow(t, domain.Subject{Kind: domain.SubjectApp, PrincipalID: w.tracker}, domain.Target{Kind: domain.TargetChannel, ChannelID: w.eskalation}, domain.LevelWrite, time.Time{})
	w.subMu.Lock()
	w.seen = nil
	w.subMu.Unlock()
}

// user is a person in its session with a fresh step-up.
func (w *pgWorld) user(id ids.ID) actor.Actor {
	return actor.Actor{ID: id, Kind: actor.KindUser, SessionID: w.sessions[id], DeviceID: w.devices[id], StepUpAt: w.clk.Now()}
}

// ctxOf is a context with p as the request principal.
func ctxOf(p actor.Actor) context.Context { return actor.WithActor(context.Background(), p) }

// counts are the rows and counters a rejected change must leave unchanged (brief M1-29 test 6).
type counts struct {
	groups, memberships, grants, events, allows, denies int64
	version                                             int64
	groupsHash, membershipsHash, grantsHash             string
}

// snapshot reads the counts, and a hash of the content of the three authz tables.
func (w *pgWorld) snapshot(t *testing.T) counts {
	t.Helper()
	conn := w.db.SuperConn(t)
	ctx := context.Background()
	var c counts
	q := func(stmt string, dst any) {
		t.Helper()
		if err := conn.QueryRow(ctx, stmt).Scan(dst); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	q(`SELECT count(*) FROM groups`, &c.groups)
	q(`SELECT count(*) FROM group_memberships`, &c.memberships)
	q(`SELECT count(*) FROM grants`, &c.grants)
	q(`SELECT count(*) FROM event_log`, &c.events)
	q(`SELECT count(*) FROM audit_events WHERE decision = 'allow'`, &c.allows)
	q(`SELECT count(*) FROM audit_events WHERE decision = 'deny'`, &c.denies)
	q(`SELECT value FROM sequence_counters WHERE name = 'permission_version'`, &c.version)
	q(`SELECT coalesce(md5(string_agg(g::text, ',' ORDER BY id)), '') FROM groups g`, &c.groupsHash)
	q(`SELECT coalesce(md5(string_agg(m::text, ',' ORDER BY id)), '') FROM group_memberships m`, &c.membershipsHash)
	q(`SELECT coalesce(md5(string_agg(g::text, ',' ORDER BY id)), '') FROM grants g`, &c.grantsHash)
	return c
}

// version reads the committed permission_version.
func (w *pgWorld) version(t *testing.T) int64 {
	t.Helper()
	return w.snapshot(t).version
}

// denyRow is the latest deny row of audit_events.
type denyRow struct {
	Action, Reason, ResourceKind string
	ResourceID, Actor            ids.ID
	Details                      map[string]any
}

// expectRejected checks err for code and that nothing changed against before except exactly one
// new deny row with action, reason code and resource (brief M1-29 test 6 and "Deny audit entry").
func (w *pgWorld) expectRejected(t *testing.T, before counts, err error, code errs.Code, action string, res txn.Ref, by ids.ID) denyRow {
	t.Helper()
	if e, ok := errs.As(err); !ok || e.Code != code {
		t.Fatalf("error %v, want %s", err, code)
	}
	if err := w.rec.Aggregator().Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
	after := w.snapshot(t)
	want := before
	want.denies++
	if after != want {
		t.Fatalf("a rejected change left something behind:\nbefore %+v\nafter  %+v", before, after)
	}
	conn := w.db.SuperConn(t)
	var r denyRow
	var rid, actorID *string
	var details []byte
	if err := conn.QueryRow(context.Background(), `SELECT action, coalesce(deny_reason, ''), coalesce(resource_kind, ''), resource_id::text,
		actor_principal_id::text, details FROM audit_events WHERE decision = 'deny' ORDER BY seq DESC LIMIT 1`).
		Scan(&r.Action, &r.Reason, &r.ResourceKind, &rid, &actorID, &details); err != nil {
		t.Fatal(err)
	}
	if rid != nil {
		r.ResourceID = ids.MustParse(*rid)
	}
	if actorID != nil {
		r.Actor = ids.MustParse(*actorID)
	}
	if err := json.Unmarshal(details, &r.Details); err != nil {
		t.Fatal(err)
	}
	if r.Action != action || r.Reason != string(code) || r.ResourceKind != res.Kind || r.ResourceID != res.ID || r.Actor != by {
		t.Fatalf("deny row %+v, want action %s reason %s resource %+v actor %v", r, action, code, res, by)
	}
	return r
}

// events returns the access.changed rows after seq, ascending.
func (w *pgWorld) events(t *testing.T, afterSeq int64) []eventRow {
	t.Helper()
	rows, err := w.db.SuperConn(t).Query(context.Background(), `SELECT seq, audience_principal_id::text, resource_id::text, actor_kind,
		receive_action, payload::text, permission_version, coalesce(space_id::text, ''), coalesce(channel_id::text, '')
		FROM event_log WHERE type = 'access.changed' AND seq > $1 ORDER BY seq`, afterSeq)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []eventRow
	for rows.Next() {
		var e eventRow
		var aud, res string
		var actorKind *string
		if err := rows.Scan(&e.Seq, &aud, &res, &actorKind, &e.Receive, &e.Payload, &e.Version, &e.Space, &e.Channel); err != nil {
			t.Fatal(err)
		}
		e.Audience, e.Resource = ids.MustParse(aud), ids.MustParse(res)
		if actorKind != nil {
			e.ActorKind = *actorKind
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

// eventRow is one access.changed row of event_log.
type eventRow struct {
	Seq                         int64
	Audience, Resource          ids.ID
	ActorKind, Receive, Payload string
	Version                     int64
	Space, Channel              string
}

// lastSeq returns the highest seq of event_log.
func (w *pgWorld) lastSeq(t *testing.T) int64 {
	t.Helper()
	var n int64
	if err := w.db.SuperConn(t).QueryRow(context.Background(), `SELECT coalesce(max(seq), 0) FROM event_log`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// payload decodes an access.changed payload.
type payload struct {
	PermissionVersion int64         `json:"permission_version"`
	Added             []targetEntry `json:"added"`
	Removed           []targetEntry `json:"removed"`
	Changed           []targetEntry `json:"changed"`
}

// targetEntry is one AccessTarget of a payload.
type targetEntry struct {
	Kind          string    `json:"kind"`
	ID            string    `json:"id"`
	Level         string    `json:"level"`
	PreviousLevel string    `json:"previous_level"`
	Capabilities  *[]string `json:"capabilities"`
}

// decodePayload decodes p.
func decodePayload(t *testing.T, p string) payload {
	t.Helper()
	var out payload
	if err := json.Unmarshal([]byte(p), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

// mustOK fails t for err.
func mustOK(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
