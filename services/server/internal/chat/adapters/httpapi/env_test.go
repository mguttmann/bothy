// SPDX-License-Identifier: Apache-2.0

package httpapi_test

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
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
	"github.com/OWNER/bothy/services/server/internal/chat"
	chatapp "github.com/OWNER/bothy/services/server/internal/chat/app"
	identitypg "github.com/OWNER/bothy/services/server/internal/identity/adapters/postgres"
	identityapp "github.com/OWNER/bothy/services/server/internal/identity/app"
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
	syncpg "github.com/OWNER/bothy/services/server/internal/sync/adapters/postgres"
	"github.com/OWNER/bothy/services/server/internal/sync/adapters/telemetry"
	syncapp "github.com/OWNER/bothy/services/server/internal/sync/app"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

func TestMain(m *testing.M) { os.Exit(pgtest.Main(m)) }

// start is the clock of the chat world.
var start = time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC)

// nameMax is space.name_max_chars of the world, small enough to test the limit.
const nameMax = 12

// chatConfig is the configuration of the world: small text limits so the tests reach them, and
// the backfill defaults of 03.
var chatConfig = chatapp.Config{
	SpaceNameMaxChars: nameMax, ChannelNameMaxChars: 10, ChannelTopicMaxChars: 16, TagNameMaxChars: 6,
	HistoryBackfillLimitDefault: 20, HistoryBackfillLimitMax: 200,
}

// testKEK is the key-encryption key of the audit recorder in these tests.
type testKEK struct{}

func (testKEK) KEK(context.Context, string) ([]byte, error) {
	return []byte(hex.EncodeToString(make([]byte, 32))), nil
}

// world is PostgreSQL 18 with the real chat module, authz (Authorizer, rights hooks), identity
// (principal source), sync and audit behind the full HTTP chain with response validation against
// openapi.yaml, the test authenticator of httpservertest and a controllable clock.
type world struct {
	t       *testing.T
	pool    *pg.Pool
	owner   *pg.Pool
	rec     *auditapp.Recorder
	clk     *clock.Fake
	creds   *fake.Credentials
	actors  *httpservertest.Actors
	h       *httpservertest.Harness
	svc     *chatapp.Service
	az      *authzapp.Authorizer
	ownerID ids.ID
	// archived collects the SpaceArchived events a test subscriber saw inside the transactions.
	archived *[]chat.SpaceArchived
	// channelsArchived collects the ChannelArchived events a test subscriber saw (M1-53).
	channelsArchived *[]chat.ChannelArchived
	mu               *sync.Mutex
}

// newWorld builds a world on a new database with the owner "owner" (session with a fresh
// step-up).
func newWorld(t *testing.T) *world {
	t.Helper()
	db := pgtest.New(t)
	cfg := db.Config(pg.RoleApp)
	cfg.MaxConns = 16
	pool := db.OpenPool(t, cfg)
	log := slog.New(&httpservertest.LogCapture{})
	w := &world{t: t, pool: pool, owner: db.Pool(t, pg.RoleOwner), clk: clock.NewFake(start), creds: fake.NewCredentials(),
		actors: httpservertest.NewActors(), archived: &[]chat.SpaceArchived{}, channelsArchived: &[]chat.ChannelArchived{}, mu: &sync.Mutex{}}
	repo := identitypg.NewRepository(pool)
	principals := identityapp.NewPrincipalSource(repo)
	// The structure source of the process: spaces and channels of chat, no threads yet (M1-53).
	str := wire.NewStructureSource(pool)
	policies := fake.NewPolicies()
	hook, err := wire.NewRightsHook(pool, wire.RightsSources{Principals: principals, Structure: str, Policies: policies}, w.clk, log)
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
	// A subscriber of SpaceArchived the way notify subscribes from M3 on.
	if err := txn.Subscribe(reg, w.onArchived); err != nil {
		t.Fatal(err)
	}
	if err := txn.Subscribe(reg, w.onChannelArchived); err != nil {
		t.Fatal(err)
	}
	reg.Seal()
	uow := pg.NewUnitOfWork(pool, pg.UnitConfig{RetryMax: 3, StatementTimeout: 30 * time.Second, LockTimeout: 20 * time.Second, IdleInTxTimeout: 60 * time.Second}, reg)
	w.rec, err = auditapp.NewRecorder(auditapp.Config{
		DenyAggregateWindow: time.Hour, DenyAggregateMaxKeys: 100, IPKeyPeriod: 30 * 24 * time.Hour,
		IPKeyRetention: 90 * 24 * time.Hour, IPKEKRef: "file:<secrets.dir>/audit_ip_kek",
	}, auditapp.Deps{UoW: uow, Store: auditpg.NewRepository(pool), KEK: testKEK{}, Clock: w.clk, Reporter: logs.New(log)})
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
		uow, syncpg.NewRepository(pool, log), w.clk)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := maint.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	az, err := authzapp.NewAuthorizer(authzapp.AuthorizerConfig{SnapshotCacheMaxEntries: 1000, StepUpMaxAge: 5 * time.Minute}, authzapp.AuthorizerDeps{
		Catalog: authzapp.NewCatalog(log), Store: authzpg.NewRepository(pool), Principals: principals,
		Credentials: w.creds, Structure: str, Policies: policies,
		Versions: instanceapp.NewVersions(instancepg.NewRepository(pool)), Recorder: w.rec, Clock: w.clk, Log: log,
	})
	if err != nil {
		t.Fatal(err)
	}
	c, err := wire.NewChat(chatConfig, pool, uow, az, az, wire.ChatWrites{Audit: w.rec, Events: appender}, w.clk)
	if err != nil {
		t.Fatal(err)
	}
	w.svc, w.az = c.Service, az
	built, err := routes.Build(wire.Handlers{Spaces: c.Handlers, Channels: c.ChannelHandlers}, wire.RouteOptions(), routes.SpecCatalog())
	if err != nil {
		t.Fatal(err)
	}
	var rs []httpservertest.Route
	for _, r := range built {
		rs = append(rs, httpservertest.Route{Op: wire.HTTPOperation(r.Operation), Handler: r.Handler.ServeHTTP})
	}
	stage, err := httpserver.NewIdempotencyStage(httpserver.IdempotencyConfig{TTL: time.Hour},
		syncapp.NewIdempotencyStore(syncpg.NewIdempotencyRepository(pool), uow), uow, w.clk)
	if err != nil {
		t.Fatal(err)
	}
	w.h = httpservertest.New(t, httpservertest.Options{
		Routes: rs, Contract: rest.Spec, Authenticator: w.actors, ValidateResponses: true,
		Stages: map[httpserver.Stage]func(http.Handler) http.Handler{httpserver.StageIdempotency: stage},
	})
	w.ownerID = w.person("owner", true)
	return w
}

// onChannelArchived is the test subscriber of ChannelArchived; it runs inside the unit of work.
func (w *world) onChannelArchived(ctx context.Context, ev chat.ChannelArchived) error {
	if _, ok := txn.UnitFrom(ctx); !ok {
		w.t.Error("ChannelArchived outside a unit of work")
	}
	w.mu.Lock()
	*w.channelsArchived = append(*w.channelsArchived, ev)
	w.mu.Unlock()
	return nil
}

// onArchived is the test subscriber of SpaceArchived; it runs inside the unit of work.
func (w *world) onArchived(ctx context.Context, ev chat.SpaceArchived) error {
	if _, ok := txn.UnitFrom(ctx); !ok {
		w.t.Error("SpaceArchived outside a unit of work")
	}
	w.mu.Lock()
	*w.archived = append(*w.archived, ev)
	w.mu.Unlock()
	return nil
}

// sql runs stmt as the database owner (fixtures outside the use cases).
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

// person inserts an active person and registers the token name for it in a session on an active
// device (a devices row) with a fresh step-up, and with owner an active owners row.
func (w *world) person(name string, owner bool) ids.ID {
	w.t.Helper()
	id := ids.New()
	w.sql(`INSERT INTO principals (id, kind, status, handle, display_name) VALUES ($1, 'user', 'active', $2, $2)`, id.String(), name)
	w.sql(`INSERT INTO users (principal_id) VALUES ($1)`, id.String())
	if owner {
		w.sql(`INSERT INTO owners (id, user_id, reason) VALUES ($1, $2, 'first owner')`, ids.New().String(), id.String())
	}
	w.session(name, id, w.clk.Now())
	return id
}

// session registers token name as id in a new session on a new active device, with step-up at
// stepUp (zero for none), and returns the device.
func (w *world) session(name string, id ids.ID, stepUp time.Time) ids.ID {
	w.t.Helper()
	s, d := ids.New(), ids.New()
	w.sql(`INSERT INTO devices (id, user_id, name, platform, state) VALUES ($1, $2, 'Laptop', 'web', 'active')`, d.String(), id.String())
	w.creds.PutDevice(d, fake.Device{PrincipalID: id, State: authz.DeviceActive})
	w.creds.PutSession(s, fake.Session{PrincipalID: id, DeviceID: d, ExpiresAt: start.Add(365 * 24 * time.Hour)})
	w.actors.Set(name, httpservertest.InSession(id, s, d, stepUp))
	return d
}

// agent inserts an active agent principal and registers token name as its API token.
func (w *world) agent(name string) ids.ID {
	w.t.Helper()
	id, tok := ids.New(), ids.New()
	w.sql(`INSERT INTO principals (id, kind, status, handle, display_name) VALUES ($1, 'agent', 'active', $2, $2)`, id.String(), name)
	w.creds.PutToken(tok, fake.Token{PrincipalID: id})
	w.actors.Set(name, httpservertest.WithToken(id, actor.KindAgent, tok))
	return id
}

// grant inserts a grant of level to the person subject on the space (zero: the instance)
// directly, the fixture of a grant that existed before; it must precede the first request of the
// subject, because it raises no rights version.
func (w *world) grant(subject, space ids.ID, level string) {
	w.t.Helper()
	if space.IsZero() {
		w.sql(`INSERT INTO grants (id, subject_kind, subject_principal_id, target_kind, level, capabilities, reason, granted_by)
			VALUES ($1, 'user', $2, 'instance', $3, '{}', 'fixture', $2)`, ids.New().String(), subject.String(), level)
		return
	}
	w.sql(`INSERT INTO grants (id, subject_kind, subject_principal_id, target_kind, target_space_id, level, capabilities, reason, granted_by)
		VALUES ($1, 'user', $2, 'space', $3, $4, '{upload}', 'fixture', $2)`, ids.New().String(), subject.String(), space.String(), level)
}

// manage inserts a grant of read with the capability manage on space for subject.
func (w *world) manage(subject, space ids.ID) {
	w.t.Helper()
	w.sql(`INSERT INTO grants (id, subject_kind, subject_principal_id, target_kind, target_space_id, level, capabilities, reason, granted_by)
		VALUES ($1, 'user', $2, 'space', $3, 'read', '{manage}', 'fixture', $2)`, ids.New().String(), subject.String(), space.String())
}

// do sends a request with token name ("" for anonymous) and the extra headers.
func (w *world) do(name, method, path, body string, headers ...string) *httpservertest.Response {
	w.t.Helper()
	if name != "" {
		headers = append(headers, "Authorization", "Bearer "+name)
	}
	return w.h.Do(w.h.NewRequest(method, path, body, headers...))
}

// want checks status and, for a problem, its code; it returns the decoded body. A success body
// never holds JSON null (ADR 0053); the harness validated it against openapi.yaml.
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
		if strings.Contains(string(r.Body), "null") {
			w.t.Errorf("JSON null in %s", r.Body)
		}
		if err := json.Unmarshal(r.Body, &m); err != nil {
			w.t.Fatalf("body: %v: %s", err, r.Body)
		}
	}
	return m
}

// pointer returns the pointer of the first field error of a problem.
func pointer(p map[string]any) string {
	list, _ := p["errors"].([]any)
	if len(list) == 0 {
		return ""
	}
	f, _ := list[0].(map[string]any)
	s, _ := f["pointer"].(string)
	return s
}

// paths of the operations.
const spacesPath = "/api/v1/spaces"

func spacePath(id ids.ID) string   { return spacesPath + "/" + id.String() }
func sidebarPath(id ids.ID) string { return "/api/v1/me/sidebar/spaces/" + id.String() }

// ifMatch returns the If-Match header of version.
func ifMatch(version int64) []string {
	return []string{"If-Match", `"` + strconv.FormatInt(version, 10) + `"`}
}

// key returns a fresh Idempotency-Key header.
func key() []string { return []string{"Idempotency-Key", ids.New().String()} }

// create creates a space through the API as the owner with the extra JSON members and returns
// its id.
func (w *world) create(name string, extra ...string) ids.ID {
	w.t.Helper()
	body := `{"name":"` + name + `"`
	for _, e := range extra {
		body += "," + e
	}
	m := w.want(w.do("owner", http.MethodPost, spacesPath, body+"}", key()...), http.StatusCreated, "")
	return ids.MustParse(m["id"].(string))
}

// move moves id as the owner with body and returns the response.
func (w *world) move(id ids.ID, body string) *httpservertest.Response {
	return w.do("owner", http.MethodPost, spacePath(id)+"/move", body, key()...)
}

// spaceRow is one row of spaces as the database holds it.
type spaceRow struct {
	ID, Name, SortKey string
	Version           int64
	Archived          bool
}

// rows returns every space ordered by id.
func (w *world) rows() []spaceRow {
	w.t.Helper()
	rs, err := w.owner.Query(context.Background(), `SELECT id::text, name, sort_key, version, archived_at IS NOT NULL FROM spaces ORDER BY id`)
	if err != nil {
		w.t.Fatal(err)
	}
	defer rs.Close()
	var out []spaceRow
	for rs.Next() {
		var r spaceRow
		if err := rs.Scan(&r.ID, &r.Name, &r.SortKey, &r.Version, &r.Archived); err != nil {
			w.t.Fatal(err)
		}
		out = append(out, r)
	}
	return out
}

// order returns the names of the non-archived spaces in the global order of the database.
func (w *world) order() []string {
	w.t.Helper()
	rs, err := w.owner.Query(context.Background(), `SELECT name FROM spaces WHERE archived_at IS NULL ORDER BY sort_key, id`)
	if err != nil {
		w.t.Fatal(err)
	}
	defer rs.Close()
	var out []string
	for rs.Next() {
		var n string
		if err := rs.Scan(&n); err != nil {
			w.t.Fatal(err)
		}
		out = append(out, n)
	}
	return out
}

// listed returns the names of listSpaces for token name.
func (w *world) listed(name string) []string {
	w.t.Helper()
	m := w.want(w.do(name, http.MethodGet, spacesPath, ""), http.StatusOK, "")
	var out []string
	for _, it := range m["items"].([]any) {
		out = append(out, it.(map[string]any)["space"].(map[string]any)["name"].(string))
	}
	return out
}

// permissionVersion returns the committed rights version.
func (w *world) permissionVersion() int {
	w.t.Helper()
	return w.count(`SELECT value::int FROM sequence_counters WHERE name = 'permission_version'`)
}

// event is one row of event_log.
type event struct {
	Type, ResourceKind, ResourceID, ReceiveAction, Audience, SpaceID string
	Version                                                          int64
	HasVersion                                                       bool
	Payload                                                          map[string]any
}

// events returns the durable events of typ, oldest first.
func (w *world) events(typ string) []event {
	w.t.Helper()
	rs, err := w.owner.Query(context.Background(), `SELECT type, resource_kind, coalesce(resource_id::text, ''), receive_action,
		coalesce(audience_principal_id::text, ''), coalesce(space_id::text, ''), coalesce(resource_version, 0),
		resource_version IS NOT NULL, payload::text FROM event_log WHERE type = $1 ORDER BY seq`, typ)
	if err != nil {
		w.t.Fatal(err)
	}
	defer rs.Close()
	var out []event
	for rs.Next() {
		var e event
		var raw string
		if err := rs.Scan(&e.Type, &e.ResourceKind, &e.ResourceID, &e.ReceiveAction, &e.Audience, &e.SpaceID, &e.Version, &e.HasVersion, &raw); err != nil {
			w.t.Fatal(err)
		}
		if strings.Contains(raw, "null") {
			w.t.Errorf("event payload with null: %s", raw)
		}
		if err := json.Unmarshal([]byte(raw), &e.Payload); err != nil {
			w.t.Fatal(err)
		}
		out = append(out, e)
	}
	return out
}

// auditRow is one row of audit_events.
type auditRow struct {
	Decision, ResourceID string
	Details              map[string]any
}

// allows returns the allow rows of action, oldest first.
func (w *world) allows(action string) []auditRow {
	w.t.Helper()
	if err := w.rec.Aggregator().Flush(context.Background()); err != nil {
		w.t.Fatal(err)
	}
	rs, err := w.owner.Query(context.Background(), `SELECT decision, coalesce(resource_id::text, ''), details::text
		FROM audit_events WHERE action = $1 AND decision = 'allow' ORDER BY seq`, action)
	if err != nil {
		w.t.Fatal(err)
	}
	defer rs.Close()
	var out []auditRow
	for rs.Next() {
		var r auditRow
		var raw string
		if err := rs.Scan(&r.Decision, &r.ResourceID, &raw); err != nil {
			w.t.Fatal(err)
		}
		if err := json.Unmarshal([]byte(raw), &r.Details); err != nil {
			w.t.Fatal(err)
		}
		out = append(out, r)
	}
	return out
}
