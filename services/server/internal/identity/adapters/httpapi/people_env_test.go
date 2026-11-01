// SPDX-License-Identifier: Apache-2.0

package httpapi_test

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/api/routes"
	"github.com/OWNER/bothy/services/server/internal/audit/adapters/logs"
	auditpg "github.com/OWNER/bothy/services/server/internal/audit/adapters/postgres"
	auditapp "github.com/OWNER/bothy/services/server/internal/audit/app"
	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/authz/adapters/fake"
	authzpg "github.com/OWNER/bothy/services/server/internal/authz/adapters/postgres"
	authzapp "github.com/OWNER/bothy/services/server/internal/authz/app"
	"github.com/OWNER/bothy/services/server/internal/identity/adapters/httpapi"
	"github.com/OWNER/bothy/services/server/internal/identity/adapters/locale"
	"github.com/OWNER/bothy/services/server/internal/identity/adapters/postgres"
	"github.com/OWNER/bothy/services/server/internal/identity/app"
	instancepg "github.com/OWNER/bothy/services/server/internal/instance/adapters/postgres"
	instanceapp "github.com/OWNER/bothy/services/server/internal/instance/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
	syncpg "github.com/OWNER/bothy/services/server/internal/sync/adapters/postgres"
	"github.com/OWNER/bothy/services/server/internal/sync/adapters/telemetry"
	syncapp "github.com/OWNER/bothy/services/server/internal/sync/app"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

// peopleStart is the clock of the people world.
var peopleStart = time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)

// testKEK is the key-encryption key of the audit recorder in these tests.
type testKEK struct{}

func (testKEK) KEK(context.Context, string) ([]byte, error) {
	return []byte(hex.EncodeToString(make([]byte, 32))), nil
}

// peopleCfg are the limits of the people world.
var peopleCfg = app.Config{HandleMaxChars: 12, DisplayNameMaxChars: 20, PageSizeDefault: 2, PageSizeMax: 3, StepUpMaxAge: 5 * time.Minute}

// world is PostgreSQL 18 with the real identity, authz (rights hooks, I1, SR8), sync (event log,
// Idempotency-Key stage) and audit behind the full HTTP chain, the test authenticator of
// httpservertest, a controllable clock and fakes of credentials, structure and agent policy
// (brief M1-31 "Tests").
type world struct {
	t         *testing.T
	pool      *pg.Pool
	owner     *pg.Pool
	uow       *pg.UnitOfWork
	rec       *auditapp.Recorder
	clk       *clock.Fake
	creds     *fake.Credentials
	structure *fake.Structure
	policies  *fake.Policies
	actors    *httpservertest.Actors
	h         *httpservertest.Harness
	// logs captures the records of authz, audit and sync; h.Logs those of the chain.
	logs *httpservertest.LogCapture
}

// newWorld returns the people world; instance changes the configuration of the instance
// settings (wire.DefaultInstanceConfig otherwise), for example telemetry.dashboard_url (M6-06).
func newWorld(t *testing.T, instance ...func(*instanceapp.Config)) *world {
	t.Helper()
	db := pgtest.New(t)
	cfg := db.Config(pg.RoleApp)
	cfg.MaxConns = 8
	pool := db.OpenPool(t, cfg)
	capture := &httpservertest.LogCapture{}
	log := slog.New(capture)
	w := &world{logs: capture,
		t: t, pool: pool, owner: db.Pool(t, pg.RoleOwner), clk: clock.NewFake(peopleStart), creds: fake.NewCredentials(),
		structure: fake.NewStructure(), policies: fake.NewPolicies(), actors: httpservertest.NewActors(),
	}
	w.structure.OnPutChannel(w.mirrorChannel)
	repo := postgres.NewRepository(pool)
	principals := app.NewPrincipalSource(repo)
	hook, err := wire.NewRightsHook(pool, wire.RightsSources{Principals: principals, Structure: w.structure, Policies: w.policies}, w.clk, log)
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
	if err := hook.Bind(authzapp.RightsBindings{Events: appender, Audit: w.rec}); err != nil {
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
	versions := instanceapp.NewVersions(instancepg.NewRepository(pool))
	az, err := authzapp.NewAuthorizer(authzapp.AuthorizerConfig{SnapshotCacheMaxEntries: 1000, StepUpMaxAge: peopleCfg.StepUpMaxAge}, authzapp.AuthorizerDeps{
		Catalog: authzapp.NewCatalog(log), Store: authzpg.NewRepository(pool), Principals: principals,
		Credentials: w.creds, Structure: w.structure, Policies: w.policies, Versions: versions, Recorder: w.rec, Clock: w.clk, Log: log,
	})
	if err != nil {
		t.Fatal(err)
	}
	codes, err := app.NewEnrollmentCodes(app.Config{EnrollmentCodeTTL: time.Hour, EnrollmentCodeMaxAttempts: 3}, testPepper,
		app.EnrollmentDeps{Codes: repo, People: repo, UoW: w.uow, Clock: w.clk})
	if err != nil {
		t.Fatal(err)
	}
	instCfg := wire.DefaultInstanceConfig()
	for _, change := range instance {
		change(&instCfg)
	}
	st, err := wire.NewInstanceSettings(instCfg, true, pool, w.uow, az, wire.SettingsWrites{Audit: w.rec, Events: appender}, w.clk)
	if err != nil {
		t.Fatal(err)
	}
	people, err := app.NewPeople(peopleCfg, app.PeopleDeps{
		Store: repo, UoW: w.uow, Authorizer: az, Audit: w.rec, Events: appender, Cursors: httpapi.Cursors{},
		Versions: versions, Locales: locale.Parser{}, Providers: repo, Identities: repo, Enrollment: codes, Settings: st.Admin, Clock: w.clk,
	})
	if err != nil {
		t.Fatal(err)
	}
	built, err := routes.Build(wire.Handlers{People: httpapi.NewPeople(people, people), Admin: st.Handlers}, wire.RouteOptions(), routes.SpecCatalog())
	if err != nil {
		t.Fatal(err)
	}
	var rs []httpservertest.Route
	for _, r := range built {
		rs = append(rs, httpservertest.Route{Op: wire.HTTPOperation(r.Operation), Handler: r.Handler.ServeHTTP})
	}
	stage, err := httpserver.NewIdempotencyStage(httpserver.IdempotencyConfig{TTL: time.Hour},
		syncapp.NewIdempotencyStore(syncpg.NewIdempotencyRepository(pool), w.uow), w.uow, w.clk)
	if err != nil {
		t.Fatal(err)
	}
	w.h = httpservertest.New(t, httpservertest.Options{
		Routes: rs, Contract: rest.Spec, Authenticator: w.actors, ValidateResponses: true,
		Stages: map[httpserver.Stage]func(http.Handler) http.Handler{httpserver.StageIdempotency: stage},
	})
	return w
}

// sql runs stmt as the database owner (test fixtures outside the use cases).
func (w *world) sql(stmt string, args ...any) {
	w.t.Helper()
	if _, err := w.owner.Exec(context.Background(), stmt, args...); err != nil {
		w.t.Fatalf("%s: %v", stmt, err)
	}
}

// count returns the integer result of query.
func (w *world) count(query string, args ...any) int {
	w.t.Helper()
	var n int
	if err := w.owner.QueryRow(context.Background(), query, args...).Scan(&n); err != nil {
		w.t.Fatalf("%s: %v", query, err)
	}
	return n
}

// person inserts a person with status and registers the token name for it in a session on an
// active device with a fresh step-up; owner adds an active owners row.
func (w *world) person(name, status string, owner bool) ids.ID {
	w.t.Helper()
	id := ids.New()
	w.sql(`INSERT INTO principals (id, kind, status, handle, display_name) VALUES ($1, 'user', $2, $3, $3)`, id.String(), status, name)
	w.sql(`INSERT INTO users (principal_id, email) VALUES ($1, $2)`, id.String(), name+"@example.com")
	if owner {
		w.sql(`INSERT INTO owners (id, user_id, reason) VALUES ($1, $2, 'first owner')`, ids.New().String(), id.String())
	}
	w.session(name, id, authz.DeviceActive, w.clk.Now())
	return id
}

// session registers token name as id in a new session on a device in state, with step-up at
// stepUp (zero for none).
func (w *world) session(name string, id ids.ID, state authz.DeviceState, stepUp time.Time) {
	s, d := ids.New(), ids.New()
	w.creds.PutDevice(d, fake.Device{PrincipalID: id, State: state})
	w.creds.PutSession(s, fake.Session{PrincipalID: id, DeviceID: d, ExpiresAt: peopleStart.Add(365 * 24 * time.Hour)})
	w.actors.Set(name, httpservertest.InSession(id, s, d, stepUp))
}

// token registers token name as an API token of id with kind.
func (w *world) token(name string, id ids.ID, kind actor.Kind) {
	tok := ids.New()
	w.creds.PutToken(tok, fake.Token{PrincipalID: id})
	w.actors.Set(name, httpservertest.WithToken(id, kind, tok))
}

// agent inserts an active agent and registers its token under name.
func (w *world) agent(name string) ids.ID {
	w.t.Helper()
	id := ids.New()
	w.sql(`INSERT INTO principals (id, kind, handle, display_name) VALUES ($1, 'agent', $2, $2)`, id.String(), name)
	w.token(name, id, actor.KindAgent)
	return id
}

// personalAgent makes agent a personal agent of holder (fake agent policy source).
func (w *world) personalAgent(agent, holder ids.ID) {
	w.policies.Put(agent, authz.AgentPolicy{BillingMode: "personal", CredentialHolderID: holder, ToolClass: "scoped"})
}

// channel adds a space with a text channel operated by operator (zero for none).
func (w *world) channel(operator ids.ID) ids.ID {
	space, ch := ids.New(), ids.New()
	w.structure.PutSpace(space)
	w.structure.PutChannel(authz.ChannelFacts{ID: ch, SpaceID: space, Kind: authz.ChannelText, EffectiveOperatorID: operator})
	return ch
}

// do sends a request with token name ("" for anonymous) and optional Idempotency-Key.
func (w *world) do(name, method, path, body, key string) *httpservertest.Response {
	w.t.Helper()
	var hdr []string
	if name != "" {
		hdr = append(hdr, "Authorization", "Bearer "+name)
	}
	if key != "" {
		hdr = append(hdr, "Idempotency-Key", key)
	}
	return w.h.Do(w.h.NewRequest(method, path, body, hdr...))
}

// want checks status and, for a problem, its code; it returns the decoded body.
func (w *world) want(r *httpservertest.Response, status int, code string) map[string]any {
	w.t.Helper()
	if r.StatusCode != status {
		w.t.Fatalf("status %d, want %d: %s", r.StatusCode, status, r.Body)
	}
	if code != "" {
		p := r.Problem(w.t)
		if p["code"] != code {
			w.t.Fatalf("code %v, want %s: %s", p["code"], code, r.Body)
		}
		return p
	}
	var m map[string]any
	if len(r.Body) > 0 {
		if err := json.Unmarshal(r.Body, &m); err != nil {
			w.t.Fatalf("body: %v: %s", err, r.Body)
		}
	}
	return m
}

// auditRow is one row of audit_events.
type auditRow struct {
	Action, Decision, Reason string
	ResourceID               string
	Details                  map[string]any
	Raw                      string
}

// audits returns the audit rows of action ("" for all), oldest first, after flushing the
// aggregated denials.
func (w *world) audits(action string) []auditRow {
	w.t.Helper()
	if err := w.rec.Aggregator().Flush(context.Background()); err != nil {
		w.t.Fatal(err)
	}
	rows, err := w.owner.Query(context.Background(), `SELECT action, decision, coalesce(deny_reason, ''), coalesce(resource_id::text, ''), details::text
		FROM audit_events WHERE $1 = '' OR action = $1 ORDER BY seq`, action)
	if err != nil {
		w.t.Fatal(err)
	}
	defer rows.Close()
	var out []auditRow
	for rows.Next() {
		var r auditRow
		if err := rows.Scan(&r.Action, &r.Decision, &r.Reason, &r.ResourceID, &r.Raw); err != nil {
			w.t.Fatal(err)
		}
		if err := json.Unmarshal([]byte(r.Raw), &r.Details); err != nil {
			w.t.Fatal(err)
		}
		out = append(out, r)
	}
	return out
}

// event is one row of event_log.
type event struct {
	Type, ResourceKind, ResourceID, ReceiveAction, Actor, Audience string
	Payload                                                        map[string]any
	HasScope                                                       bool
}

// events returns the durable events of type typ, oldest first.
func (w *world) events(typ string) []event {
	w.t.Helper()
	rows, err := w.owner.Query(context.Background(), `SELECT type, resource_kind, coalesce(resource_id::text, ''), receive_action,
		coalesce(actor_principal_id::text, ''), coalesce(audience_principal_id::text, ''), payload::text,
		space_id IS NOT NULL OR channel_id IS NOT NULL OR thread_id IS NOT NULL OR resource_version IS NOT NULL
		FROM event_log WHERE type = $1 ORDER BY seq`, typ)
	if err != nil {
		w.t.Fatal(err)
	}
	defer rows.Close()
	var out []event
	for rows.Next() {
		var e event
		var raw string
		if err := rows.Scan(&e.Type, &e.ResourceKind, &e.ResourceID, &e.ReceiveAction, &e.Actor, &e.Audience, &raw, &e.HasScope); err != nil {
			w.t.Fatal(err)
		}
		if testkit.HasJSONNull([]byte(raw)) {
			w.t.Errorf("event payload with null: %s", raw)
		}
		if err := json.Unmarshal([]byte(raw), &e.Payload); err != nil {
			w.t.Fatal(err)
		}
		out = append(out, e)
	}
	return out
}

// permissionVersion returns the committed rights version.
func (w *world) permissionVersion() int {
	w.t.Helper()
	return w.count(`SELECT value::int FROM sequence_counters WHERE name = 'permission_version'`)
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
