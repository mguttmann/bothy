// SPDX-License-Identifier: Apache-2.0

// Package messagingworld is the HTTP test world of sending (M1-56): PostgreSQL 18 through pgtest
// with the real chat, messaging, identity, authz, audit and sync modules behind the HTTP chain with
// response validation, a fake clock, a test subscriber of MessagePosted and the helpers of the
// tests of the brief. Only tests import it; it lives under services/server/test (test code, 02
// "Code-Layout"). Each test package calls pgtest.Main in its TestMain.
package messagingworld

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
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
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/searchlang"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/messaging"
	messagingapp "github.com/OWNER/bothy/services/server/internal/messaging/app"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
	syncpg "github.com/OWNER/bothy/services/server/internal/sync/adapters/postgres"
	"github.com/OWNER/bothy/services/server/internal/sync/adapters/telemetry"
	syncapp "github.com/OWNER/bothy/services/server/internal/sync/app"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

// start is the clock of the world.
var start = time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC)

// chatConfig is the chat configuration of the world.
var chatConfig = chatapp.Config{
	SpaceNameMaxChars: 40, ChannelNameMaxChars: 40, ChannelTopicMaxChars: 40, TagNameMaxChars: 20,
	HistoryBackfillLimitDefault: 20, HistoryBackfillLimitMax: 200, ThreadTitleMaxChars: 80, ThreadMaxTags: 3,
	PlaceholderTitles: []string{"New chat"}, PageSizeDefault: 20, PageSizeMax: 50,
}

// Limits of the world, small enough for the tests to reach them.
const (
	MaxContent  = 4096
	MaxMentions = 4
	PayloadMax  = 2048
	// TemplateNameMax and TemplateMax are template.name_max_chars and template.max_per_scope.
	TemplateNameMax = 20
	TemplateMax     = 5
	// ListContentMax is message.list_content_max_bytes of the world: no cut unless a test sets it.
	ListContentMax = MaxContent
	// PageSizeDefault and PageSizeMax are api.page_size_default and api.page_size_max.
	PageSizeDefault = 20
	PageSizeMax     = 50
)

// Option changes the messaging configuration of a world (brief M1-57: small test values).
type Option func(*messagingapp.Config)

// testKEK is the key-encryption key of the audit recorder in these tests.
type testKEK struct{}

func (testKEK) KEK(context.Context, string) ([]byte, error) {
	return []byte(hex.EncodeToString(make([]byte, 32))), nil
}

// errVeto is the error a vetoing MessagePosted subscriber returns.
var errVeto = errors.New("veto")

// Recorder collects the MessagePosted events a test subscriber saw inside the transactions and
// can refuse them. The subscriber itself must be a method of a type in the test package: the txn
// registry derives the module of a handler from its Go package (internal/<module>/...).
type Recorder struct {
	t      *testing.T
	mu     sync.Mutex
	posted []messaging.MessagePosted
	veto   bool
}

// On records ev, or refuses it while the veto is set; it must run inside a unit of work.
func (r *Recorder) On(ctx context.Context, ev messaging.MessagePosted) error {
	if _, ok := txn.UnitFrom(ctx); !ok {
		r.t.Error("MessagePosted outside a unit of work")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.veto {
		return errVeto
	}
	r.posted = append(r.posted, ev)
	return nil
}

// Subscribe registers the test subscriber of MessagePosted that feeds rec.
type Subscribe func(reg *txn.Registry, rec *Recorder) error

// World is PostgreSQL 18 with the real chat and messaging modules, authz, identity, sync and audit
// behind the full HTTP chain with response validation against openapi.yaml, the test
// authenticator of httpservertest and a controllable clock.
type World struct {
	t *testing.T
	// Pool is the pool of bothy_app; Owner the pool of the database owner for fixtures.
	Pool, Owner *pg.Pool
	// Clock is the clock of the world.
	Clock    *clock.Fake
	creds    *fake.Credentials
	actors   *httpservertest.Actors
	policies *fake.Policies
	// H is the HTTP harness.
	H *httpservertest.Harness
	// OwnerID is the person "owner", an active owner.
	OwnerID ids.ID
	// Svc is the messaging use cases; Threads the threads of chat.
	Svc     *messagingapp.Service
	Threads chat.Threads
	// Authorizer is the real Authorizer of the world (Scope for CountUnread, M1-60).
	Authorizer authorize.Authorizer
	// Logs holds every record the modules logged (the chain logs into H.Logs).
	Logs *httpservertest.LogCapture

	rec *Recorder
}

// New builds a world on a new database with the owner "owner"; subscribe registers the test
// subscriber of MessagePosted of the calling package; opts change the messaging configuration.
func New(t *testing.T, subscribe Subscribe, opts ...Option) *World {
	t.Helper()
	db := pgtest.New(t)
	cfg := db.Config(pg.RoleApp)
	cfg.MaxConns = 32
	pool := db.OpenPool(t, cfg)
	capture := &httpservertest.LogCapture{}
	log := slog.New(capture)
	w := &World{t: t, Logs: capture, Pool: pool, Owner: db.Pool(t, pg.RoleOwner), Clock: clock.NewFake(start), creds: fake.NewCredentials(),
		actors: httpservertest.NewActors(), policies: fake.NewPolicies(), rec: &Recorder{t: t}}
	// The installations of apps decide whether an app acts (M2-24), as in the process.
	principals := identityapp.NewPrincipalSource(identitypg.NewRepository(pool)).WithApps(wire.NewAppInstallations(pool))
	str := wire.NewStructureSource(pool)
	hook, err := wire.NewRightsHook(pool, wire.RightsSources{Principals: principals, Structure: str, Policies: w.policies}, w.Clock, log)
	if err != nil {
		t.Fatal(err)
	}
	reg := registry(t, hook, wire.NewModules(pool, log), subscribe, w.rec)
	uow := pg.NewUnitOfWork(pool, pg.UnitConfig{RetryMax: 3, StatementTimeout: 30 * time.Second, LockTimeout: 20 * time.Second, IdleInTxTimeout: 60 * time.Second}, reg)
	rec, err := auditapp.NewRecorder(auditapp.Config{
		DenyAggregateWindow: time.Hour, DenyAggregateMaxKeys: 100, IPKeyPeriod: 30 * 24 * time.Hour,
		IPKeyRetention: 90 * 24 * time.Hour, IPKEKRef: "file:<secrets.dir>/audit_ip_kek",
	}, auditapp.Deps{UoW: uow, Store: auditpg.NewRepository(pool), KEK: testKEK{}, Clock: w.Clock, Reporter: logs.New(log)})
	if err != nil {
		t.Fatal(err)
	}
	appender, err := syncapp.NewAppender(syncapp.AppenderConfig{PayloadMaxBytes: PayloadMax}, telemetry.Correlator{})
	if err != nil {
		t.Fatal(err)
	}
	if err := hook.Bind(authzapp.RightsBindings{Events: appender, Audit: rec}); err != nil {
		t.Fatal(err)
	}
	maint, err := syncapp.NewPartitionMaintenance(syncapp.PartitionConfig{Size: 1000, Ahead: 2, Retention: 30 * 24 * time.Hour, LockTimeout: 2 * time.Second},
		uow, syncpg.NewRepository(pool, log), w.Clock)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := maint.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	az, err := authzapp.NewAuthorizer(authzapp.AuthorizerConfig{SnapshotCacheMaxEntries: 1000, StepUpMaxAge: 5 * time.Minute}, authzapp.AuthorizerDeps{
		Catalog: authzapp.NewCatalog(log), Store: authzpg.NewRepository(pool), Principals: principals,
		Credentials: w.creds, Structure: str, Policies: w.policies,
		Versions: instanceapp.NewVersions(instancepg.NewRepository(pool)), Recorder: rec, Clock: w.Clock, Log: log,
	})
	if err != nil {
		t.Fatal(err)
	}
	settings, err := wire.NewInstanceSettings(wire.DefaultInstanceConfig(), true, pool, uow, az, wire.SettingsWrites{Audit: rec, Events: appender}, w.Clock)
	if err != nil {
		t.Fatal(err)
	}
	dir := wire.NewSearchDirectory(pool, SearchLanguages(t), settings.Admin)
	c, err := wire.NewChat(chatConfig, pool, uow, az, az, wire.ChatWrites{Audit: rec, Events: appender, Rights: hook, Directory: dir}, w.policies, w.Clock)
	if err != nil {
		t.Fatal(err)
	}
	mcfg := messagingapp.Config{MaxContentBytes: MaxContent, MaxMentions: MaxMentions, EventPayloadMaxBytes: PayloadMax,
		TemplateNameMaxChars: TemplateNameMax, TemplateMaxPerScope: TemplateMax,
		ListContentMaxBytes: ListContentMax, PageSizeDefault: PageSizeDefault, PageSizeMax: PageSizeMax}
	for _, o := range opts {
		o(&mcfg)
	}
	m, err := wire.NewMessaging(mcfg, pool, uow, az, appender, c.Service, dir, w.Clock, log)
	if err != nil {
		t.Fatal(err)
	}
	w.Svc, w.Threads, w.Authorizer = m.Service, c.Service, az
	w.H = harness(t, pool, uow, w.actors, wire.Handlers{Spaces: c.Handlers, Channels: c.ChannelHandlers, Threads: c.ThreadHandlers, Messages: m.Handlers}, w.Clock)
	w.OwnerID = w.Person("owner", true)
	return w
}

// SearchLanguages returns the language configuration of the chapter defaults (search.languages
// german and english, search.locale_map de and en; docs/spec/14-suche.md section 15).
func SearchLanguages(t testing.TB) searchlang.Map {
	t.Helper()
	m, err := searchlang.New([]string{"german", "english"}, map[string]string{"de": "german", "en": "english"})
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// registry returns the sealed txn registry with the rights hooks, the sync and audit hooks and
// the test subscriber of MessagePosted.
func registry(t *testing.T, hook *authzapp.RightsHook, mods *wire.Modules, subscribe Subscribe, rec *Recorder) *txn.Registry {
	t.Helper()
	reg := txn.NewRegistry()
	for _, h := range []struct {
		p txn.Phase
		f txn.Hook
	}{{txn.PhaseRightsBegin, hook.Begin}, {txn.PhaseRightsCheck, hook.BeforeCommit}, {txn.PhaseSync, mods.Sync.Run}, {txn.PhaseAudit, mods.Audit.Hook}} {
		if err := reg.SetHook(h.p, h.f); err != nil {
			t.Fatal(err)
		}
	}
	// The test subscriber of MessagePosted, the way runs, sync and notify subscribe later.
	if err := subscribe(reg, rec); err != nil {
		t.Fatal(err)
	}
	reg.Seal()
	return reg
}

// harness returns the HTTP chain over the routes of handlers with response validation and the
// Idempotency-Key stage.
func harness(t *testing.T, pool *pg.Pool, uow *pg.UnitOfWork, actors *httpservertest.Actors, handlers wire.Handlers, clk *clock.Fake) *httpservertest.Harness {
	t.Helper()
	built, err := routes.Build(handlers, wire.RouteOptions(), routes.SpecCatalog())
	if err != nil {
		t.Fatal(err)
	}
	var rs []httpservertest.Route
	for _, r := range built {
		rs = append(rs, httpservertest.Route{Op: wire.HTTPOperation(r.Operation), Handler: r.Handler.ServeHTTP})
	}
	stage, err := httpserver.NewIdempotencyStage(httpserver.IdempotencyConfig{TTL: time.Hour},
		syncapp.NewIdempotencyStore(syncpg.NewIdempotencyRepository(pool), uow), uow, clk)
	if err != nil {
		t.Fatal(err)
	}
	return httpservertest.New(t, httpservertest.Options{
		Routes: rs, Contract: rest.Spec, Authenticator: actors, ValidateResponses: true,
		Stages: map[httpserver.Stage]func(http.Handler) http.Handler{httpserver.StageIdempotency: stage},
	})
}

// SetVeto makes the test subscriber of MessagePosted refuse (true) or accept (false) the next
// messages.
func (w *World) SetVeto(veto bool) {
	w.rec.mu.Lock()
	defer w.rec.mu.Unlock()
	w.rec.veto = veto
}

// Posted returns a copy of the MessagePosted events seen so far.
func (w *World) Posted() []messaging.MessagePosted {
	w.rec.mu.Lock()
	defer w.rec.mu.Unlock()
	return append([]messaging.MessagePosted(nil), w.rec.posted...)
}

// SQL runs stmt as the database owner (fixtures outside the use cases).
func (w *World) SQL(stmt string, args ...any) {
	w.t.Helper()
	if _, err := w.Owner.Exec(context.Background(), stmt, args...); err != nil {
		w.t.Fatalf("%s: %v", stmt, err)
	}
}

// Count returns the integer result of query.
func (w *World) Count(query string, args ...any) int {
	w.t.Helper()
	var n int
	if err := w.Owner.QueryRow(context.Background(), query, args...).Scan(&n); err != nil {
		w.t.Fatalf("%s: %v", query, err)
	}
	return n
}

// Person inserts an active person with handle name and registers the token name for it in a
// session on an active device; owner adds an active owners row. It returns the person.
func (w *World) Person(name string, owner bool) ids.ID {
	w.t.Helper()
	id := ids.New()
	w.SQL(`INSERT INTO principals (id, kind, status, handle, display_name) VALUES ($1, 'user', 'active', $2, $2)`, id.String(), name)
	w.SQL(`INSERT INTO users (principal_id) VALUES ($1)`, id.String())
	if owner {
		w.SQL(`INSERT INTO owners (id, user_id, reason) VALUES ($1, $2, 'first owner')`, ids.New().String(), id.String())
	}
	s, d := ids.New(), ids.New()
	w.SQL(`INSERT INTO devices (id, user_id, name, platform, state) VALUES ($1, $2, 'Laptop', 'web', 'active')`, d.String(), id.String())
	w.creds.PutDevice(d, fake.Device{PrincipalID: id, State: authz.DeviceActive})
	w.creds.PutSession(s, fake.Session{PrincipalID: id, DeviceID: d, ExpiresAt: start.Add(365 * 24 * time.Hour)})
	w.actors.Set(name, httpservertest.InSession(id, s, d, w.Clock.Now()))
	return id
}

// Agent inserts an active agent with handle name, an agents row and an organization policy, and
// registers token name as its API token.
func (w *World) Agent(name string) ids.ID {
	w.t.Helper()
	id, tok := ids.New(), ids.New()
	w.SQL(`INSERT INTO principals (id, kind, status, handle, display_name) VALUES ($1, 'agent', 'active', $2, $2)`, id.String(), name)
	w.SQL(`INSERT INTO agents (principal_id, adapter_kind, billing_mode) VALUES ($1, 'hermes', 'organization')`, id.String())
	w.policies.Put(id, authz.AgentPolicy{BillingMode: "organization", ToolClass: "chat"})
	w.creds.PutToken(tok, fake.Token{PrincipalID: id})
	w.actors.Set(name, httpservertest.WithToken(id, actor.KindAgent, tok))
	return id
}

// App inserts an active app with handle name and its active installation with the granted
// scopes (M2-24), and registers token name for it with the same scopes.
func (w *World) App(name string, scopes ...string) ids.ID {
	w.t.Helper()
	id, tok, inst := ids.New(), ids.New(), ids.New()
	w.SQL(`INSERT INTO principals (id, kind, status, handle, display_name) VALUES ($1, 'app', 'active', $2, $2)`, id.String(), name)
	w.SQL(`INSERT INTO apps (principal_id, slug, manifest, manifest_version, manifest_sha256) VALUES ($1, $2, '{}', '1', '\x00')`,
		id.String(), name)
	w.SQL(`INSERT INTO app_installations (id, app_id, granted_scopes, installed_by) VALUES ($1, $2, $3, $4)`,
		inst.String(), id.String(), scopes, w.OwnerID.String())
	w.creds.PutToken(tok, fake.Token{PrincipalID: id})
	a := httpservertest.WithToken(id, actor.KindApp, tok)
	a.InstallationID, a.GrantedScopes, a.Scopes = inst, scopes, scopes
	w.actors.Set(name, a)
	return id
}

// Grant inserts a grant of level to subject of kind on the space, the fixture of a grant that
// existed before (it must precede the first request of the subject).
func (w *World) Grant(kind string, subject, space ids.ID, level string) {
	w.t.Helper()
	w.SQL(`INSERT INTO grants (id, subject_kind, subject_principal_id, target_kind, target_space_id, level, capabilities, reason, granted_by)
		VALUES ($1, $2, $3, 'space', $4, $5, '{}', 'fixture', $6)`, ids.New().String(), kind, subject.String(), space.String(), level, w.OwnerID.String())
}

// SetOperator sets the own operator of the channel ch directly.
func (w *World) SetOperator(ch, agent ids.ID) {
	w.t.Helper()
	w.SQL(`UPDATE channels SET operator_inherit = false, operator_agent_id = $2, version = version + 1 WHERE id = $1`, ch.String(), agent.String())
}

// Do sends a request with token name ("" for anonymous) and the extra headers.
func (w *World) Do(name, method, path, body string, headers ...string) *httpservertest.Response {
	w.t.Helper()
	if name != "" {
		headers = append(headers, "Authorization", "Bearer "+name)
	}
	return w.H.Do(w.H.NewRequest(method, path, body, headers...))
}

// Want checks status and, for a problem, its code; it returns the decoded body. A success body
// never holds JSON null (ADR 0053); the harness validated it against openapi.yaml.
func (w *World) Want(r *httpservertest.Response, status int, code string) map[string]any {
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
		if strings.Contains(string(r.Body), ":null") {
			w.t.Errorf("JSON null in %s", r.Body)
		}
		if err := json.Unmarshal(r.Body, &m); err != nil {
			w.t.Fatalf("body: %v: %s", err, r.Body)
		}
	}
	return m
}

// Field returns pointer and code of the first field error of a problem.
func Field(p map[string]any) (string, string) {
	list, _ := p["errors"].([]any)
	if len(list) == 0 {
		return "", ""
	}
	f, _ := list[0].(map[string]any)
	ptr, _ := f["pointer"].(string)
	code, _ := f["code"].(string)
	return ptr, code
}

// Key returns a fresh Idempotency-Key header.
func Key() []string { return []string{"Idempotency-Key", ids.New().String()} }

// Space creates a space as the owner and returns it.
func (w *World) Space(name string) ids.ID {
	w.t.Helper()
	m := w.Want(w.Do("owner", http.MethodPost, "/api/v1/spaces", `{"name":"`+name+`"}`, Key()...), http.StatusCreated, "")
	return ids.MustParse(m["id"].(string))
}

// Channel creates a channel of kind in space as the owner and returns it.
func (w *World) Channel(space ids.ID, kind, name string) ids.ID {
	w.t.Helper()
	m := w.Want(w.Do("owner", http.MethodPost, "/api/v1/spaces/"+space.String()+"/channels", `{"kind":"`+kind+`","name":"`+name+`"}`, Key()...),
		http.StatusCreated, "")
	return ids.MustParse(m["id"].(string))
}

// MainThread returns the main thread of the text channel ch.
func (w *World) MainThread(ch ids.ID) ids.ID {
	w.t.Helper()
	var id string
	if err := w.Owner.QueryRow(context.Background(), `SELECT id::text FROM threads WHERE channel_id = $1 AND kind = 'main'`, ch.String()).Scan(&id); err != nil {
		w.t.Fatal(err)
	}
	return ids.MustParse(id)
}

// TextChannel creates a space with a text channel and returns the space, the channel and its
// main thread.
func (w *World) TextChannel() (space, ch, main ids.ID) {
	w.t.Helper()
	space = w.Space("Support " + ids.New().String()[:8])
	ch = w.Channel(space, "text", "general")
	return space, ch, w.MainThread(ch)
}

// MessagesPath is the path of createMessage.
func MessagesPath(thread ids.ID) string { return "/api/v1/threads/" + thread.String() + "/messages" }

// ThreadsPath is the path of createThread.
func ThreadsPath(ch ids.ID) string { return "/api/v1/channels/" + ch.String() + "/threads" }

// Body returns a MessageCreate body with clientMsgID, content and the extra members.
func Body(clientMsgID ids.ID, content string, extra ...string) string {
	c, err := json.Marshal(content)
	if err != nil {
		panic(err)
	}
	b := `{"client_msg_id":"` + clientMsgID.String() + `","content_md":` + string(c)
	for _, e := range extra {
		b += "," + e
	}
	return b + "}"
}

// Send posts content as token name into thread and checks the status; it returns the body.
func (w *World) Send(name string, thread ids.ID, content string, status int, extra ...string) map[string]any {
	w.t.Helper()
	return w.Want(w.Do(name, http.MethodPost, MessagesPath(thread), Body(ids.New(), content, extra...)), status, "")
}

// Event is one row of event_log.
type Event struct {
	Seq                                     int64
	Type, ResourceID, ClientMsgID, ThreadID string
	Truncated                               bool
	Payload                                 map[string]any
	Raw                                     string
}

// Events returns the durable events of the types (all types when none), oldest first.
func (w *World) Events(types ...string) []Event {
	w.t.Helper()
	if types == nil {
		types = []string{}
	}
	rs, err := w.Owner.Query(context.Background(), `SELECT seq, type, coalesce(resource_id::text, ''), coalesce(client_msg_id::text, ''),
		coalesce(thread_id::text, ''), truncated, payload::text FROM event_log
		WHERE cardinality($1::text[]) = 0 OR type = ANY ($1::text[]) ORDER BY seq`, types)
	if err != nil {
		w.t.Fatal(err)
	}
	defer rs.Close()
	var out []Event
	for rs.Next() {
		var e Event
		if err := rs.Scan(&e.Seq, &e.Type, &e.ResourceID, &e.ClientMsgID, &e.ThreadID, &e.Truncated, &e.Raw); err != nil {
			w.t.Fatal(err)
		}
		if strings.Contains(e.Raw, ": null") || strings.Contains(e.Raw, ":null") {
			w.t.Errorf("Event payload with null: %s", e.Raw)
		}
		if err := json.Unmarshal([]byte(e.Raw), &e.Payload); err != nil {
			w.t.Fatal(err)
		}
		out = append(out, e)
	}
	return out
}

// MessageEvents returns the message.created events of the thread.
func (w *World) MessageEvents(thread ids.ID) []Event {
	var out []Event
	for _, e := range w.Events("message.created") {
		if e.ThreadID == thread.String() {
			out = append(out, e)
		}
	}
	return out
}

// Seqs returns the committed thread_seq values of the thread in order.
func (w *World) Seqs(thread ids.ID) []int64 {
	w.t.Helper()
	rs, err := w.Owner.Query(context.Background(), `SELECT thread_seq FROM messages WHERE thread_id = $1 ORDER BY thread_seq`, thread.String())
	if err != nil {
		w.t.Fatal(err)
	}
	defer rs.Close()
	var out []int64
	for rs.Next() {
		var n int64
		if err := rs.Scan(&n); err != nil {
			w.t.Fatal(err)
		}
		out = append(out, n)
	}
	return out
}

// JSONList renders a decoded JSON list of strings as [a,b].
func JSONList(v any) string {
	var parts []string
	for _, x := range v.([]any) {
		parts = append(parts, x.(string))
	}
	return "[" + strings.Join(parts, ",") + "]"
}

// JSONErrors renders the errors member of a problem.
func JSONErrors(p map[string]any) string {
	b, _ := json.Marshal(p["errors"])
	return string(b)
}

// Same404 checks that two answers are the same 404 apart from correlation_id and instance in the
// body and Correlation-Id and Date in the headers (brief M1-57 note 9; Date only differs when the
// second turns between the two requests).
func Same404(t *testing.T, a, b *httpservertest.Response) {
	t.Helper()
	if a.StatusCode != http.StatusNotFound || b.StatusCode != http.StatusNotFound {
		t.Fatalf("statuses %d %d", a.StatusCode, b.StatusCode)
	}
	ha, hb := a.Header.Clone(), b.Header.Clone()
	for _, k := range []string{"Correlation-Id", "Date"} {
		ha.Del(k)
		hb.Del(k)
	}
	if fa, fb := fmt.Sprint(ha), fmt.Sprint(hb); fa != fb {
		t.Fatalf("404 headers differ:\n%s\n%s", fa, fb)
	}
	pa, pb := a.Problem(t), b.Problem(t)
	for _, k := range []string{"correlation_id", "instance"} {
		delete(pa, k)
		delete(pb, k)
	}
	ja, _ := json.Marshal(pa)
	jb, _ := json.Marshal(pb)
	if string(ja) != string(jb) {
		t.Fatalf("404 bodies differ:\n%s\n%s", ja, jb)
	}
}

// ForumTag creates a tag in the forum as the owner and returns it.
func (w *World) ForumTag(forum ids.ID, name string) ids.ID {
	w.t.Helper()
	m := w.Want(w.Do("owner", http.MethodPost, "/api/v1/channels/"+forum.String()+"/tags", `{"name":"`+name+`"}`, Key()...), http.StatusCreated, "")
	return ids.MustParse(m["id"].(string))
}

// Tombstone turns the message id into a tombstone the way M1-58 deletes (brief M1-57 note 6):
// empty content, deleted_at and deleted_by set, version + 1.
func (w *World) Tombstone(id ids.ID) {
	w.t.Helper()
	w.SQL(`UPDATE messages SET content_md = '', deleted_at = now(), deleted_by = $2, version = version + 1 WHERE id = $1`, id.String(), w.OwnerID.String())
}

// MessagePath is the path of getMessage.
func MessagePath(id ids.ID) string { return "/api/v1/messages/" + id.String() }
