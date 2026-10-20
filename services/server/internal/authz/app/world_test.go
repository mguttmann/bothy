// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/authz/adapters/fake"
	"github.com/OWNER/bothy/services/server/internal/authz/app"
	"github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// worldStart is the clock of the test world (07 section 11.1: the clock is controllable).
var worldStart = time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)

// recorder is a fake auditing.Recorder that keeps every call.
type recorder struct {
	mu       sync.Mutex
	denials  []auditing.Entry
	detached []auditing.Entry
	records  []auditing.Entry
	fail     bool
}

var _ auditing.Recorder = (*recorder)(nil)

func (r *recorder) Record(_ context.Context, e auditing.Entry) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.records = append(r.records, e)
	return nil
}

func (r *recorder) RecordDenial(_ context.Context, e auditing.Entry) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.denials = append(r.denials, e)
	if r.fail {
		return fake.ErrInjected
	}
	return nil
}

func (r *recorder) RecordDetached(_ context.Context, e auditing.Entry) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.detached = append(r.detached, e)
	if r.fail {
		return fake.ErrInjected
	}
	return nil
}

func (r *recorder) counts() (denials, detached int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.denials), len(r.detached)
}

// world is the test world of 07 sections 10 and 11.1, built with the fakes.
type world struct {
	clk        *clock.Fake
	principals *fake.Principals
	structure  *fake.Structure
	policies   *fake.Policies
	creds      *fake.Credentials
	store      *fake.Store
	versions   *fake.Versions
	rec        *recorder
	logs       *bytes.Buffer
	az         *app.Authorizer
	cfg        app.AuthorizerConfig

	owner, alex, sam, robin                ids.ID
	lotse, texter, runner, assistent       ids.ID
	tracker                                ids.ID
	support, marketing, engineering, resea ids.ID
	allgemein, eskalation, kampagnen       ids.ID
	intern, projektA, werkzeuge, notizen   ids.ID
	dm                                     ids.ID
	mainThread                             map[ids.ID]ids.ID
	leser                                  ids.ID
	samMembership                          ids.ID
	membershipExpiry                       time.Time
	g1, g2, g3, g4, g5, g6, gAlexEsk       ids.ID
	g3Expiry                               time.Time

	sessions map[ids.ID]ids.ID // principal -> session
	devices  map[ids.ID]ids.ID // principal -> device
	tokens   map[ids.ID]ids.ID // principal -> token
	install  ids.ID
}

func newWorld(t testing.TB) *world {
	return newWorldWith(t, app.AuthorizerConfig{SnapshotCacheMaxEntries: 1000, StepUpMaxAge: 5 * time.Minute, ViewAsMaxDuration: viewAsMax})
}

// viewAsMax is authz.view_as_max_duration of the test worlds.
const viewAsMax = 15 * time.Minute

func newWorldWith(t testing.TB, cfg app.AuthorizerConfig) *world {
	t.Helper()
	w := &world{
		clk: clock.NewFake(worldStart), principals: fake.NewPrincipals(), structure: fake.NewStructure(),
		policies: fake.NewPolicies(), creds: fake.NewCredentials(), store: fake.NewStore(), versions: &fake.Versions{},
		rec: &recorder{}, logs: &bytes.Buffer{}, cfg: cfg,
		mainThread: map[ids.ID]ids.ID{}, sessions: map[ids.ID]ids.ID{}, devices: map[ids.ID]ids.ID{}, tokens: map[ids.ID]ids.ID{},
	}
	w.versions.Set(1)
	for _, p := range []*ids.ID{&w.owner, &w.alex, &w.sam, &w.robin} {
		*p = ids.New()
		w.principals.Put(authz.PrincipalFacts{ID: *p, Kind: actor.KindUser, Status: authz.PrincipalActive, IsOwner: *p == w.owner})
		w.addSession(*p)
	}
	for _, p := range []*ids.ID{&w.lotse, &w.texter, &w.runner, &w.assistent} {
		*p = ids.New()
		w.principals.Put(authz.PrincipalFacts{ID: *p, Kind: actor.KindAgent, Status: authz.PrincipalActive})
		w.addToken(*p)
	}
	w.tracker = ids.New()
	w.principals.Put(authz.PrincipalFacts{ID: w.tracker, Kind: actor.KindApp, Status: authz.PrincipalActive, AppInstallationActive: true})
	w.addToken(w.tracker)
	w.install = ids.New()

	w.policies.Put(w.lotse, authz.AgentPolicy{BillingMode: domain.BillingOrganization, ToolClass: domain.ToolChat})
	w.policies.Put(w.texter, authz.AgentPolicy{BillingMode: domain.BillingOrganization, ToolClass: domain.ToolScoped})
	w.policies.Put(w.runner, authz.AgentPolicy{BillingMode: domain.BillingOrganization, ToolClass: domain.ToolHost})
	w.policies.Put(w.assistent, authz.AgentPolicy{BillingMode: domain.BillingPersonal, CredentialHolderID: w.owner, ToolClass: domain.ToolScoped})

	for _, s := range []*ids.ID{&w.support, &w.marketing, &w.engineering, &w.resea} {
		*s = ids.New()
		w.structure.PutSpace(*s)
	}
	ch := func(id *ids.ID, space ids.ID, kind authz.ChannelKind, op ids.ID) {
		*id = ids.New()
		w.structure.PutChannel(authz.ChannelFacts{ID: *id, SpaceID: space, Kind: kind, EffectiveOperatorID: op})
		w.addMainThread(*id)
	}
	ch(&w.allgemein, w.support, authz.ChannelText, w.lotse)
	ch(&w.eskalation, w.support, authz.ChannelText, w.lotse)
	ch(&w.kampagnen, w.marketing, authz.ChannelText, w.texter)
	ch(&w.intern, w.marketing, authz.ChannelText, w.texter)
	ch(&w.projektA, w.engineering, authz.ChannelForum, w.runner)
	ch(&w.werkzeuge, w.engineering, authz.ChannelText, w.runner)
	ch(&w.notizen, w.resea, authz.ChannelText, w.assistent)
	pa, _, _ := w.structure.Channel(context.Background(), w.projektA)
	pa.ProjectBound, pa.ProjectReady = true, true
	w.structure.PutChannel(pa)
	w.dm = ids.New()
	w.structure.PutChannel(authz.ChannelFacts{ID: w.dm, Kind: authz.ChannelDM, DMParticipants: []ids.ID{w.alex, w.sam}})
	w.addMainThread(w.dm)

	w.leser = ids.New()
	mustNoErr(t, w.store.InsertGroup(context.Background(), domain.Group{ID: w.leser, Name: "Leser", Source: domain.GroupSourceLocal, CreatedAt: worldStart}))
	w.samMembership = ids.New()
	w.membershipExpiry = worldStart.Add(48 * time.Hour)
	mustNoErr(t, w.store.InsertMembership(context.Background(), domain.Membership{ID: w.samMembership, GroupID: w.leser, UserID: w.sam,
		AddedAt: worldStart, ExpiresAt: w.membershipExpiry}))

	user := func(id ids.ID) domain.Subject { return domain.Subject{Kind: domain.SubjectUser, PrincipalID: id} }
	w.g1 = w.grant(t, user(w.alex), domain.Target{Kind: domain.TargetSpace, SpaceID: w.support}, domain.LevelTrigger, time.Time{}, domain.CapUpload, domain.CapApproveActions)
	w.g2 = w.grant(t, user(w.sam), domain.Target{Kind: domain.TargetSpace, SpaceID: w.marketing}, domain.LevelTrigger, time.Time{}, domain.CapUpload)
	w.g3Expiry = time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)
	w.g3 = w.grant(t, user(w.robin), domain.Target{Kind: domain.TargetChannel, ChannelID: w.projektA}, domain.LevelTrigger, w.g3Expiry, domain.CapUpload)
	w.g4 = w.grant(t, domain.Subject{Kind: domain.SubjectGroup, GroupID: w.leser}, domain.Target{Kind: domain.TargetChannel, ChannelID: w.allgemein}, domain.LevelRead, time.Time{})
	w.g5 = w.grant(t, domain.Subject{Kind: domain.SubjectAgent, PrincipalID: w.lotse}, domain.Target{Kind: domain.TargetSpace, SpaceID: w.support}, domain.LevelWrite, time.Time{}, domain.CapUpload)
	w.g6 = w.grant(t, domain.Subject{Kind: domain.SubjectApp, PrincipalID: w.tracker}, domain.Target{Kind: domain.TargetChannel, ChannelID: w.eskalation}, domain.LevelWrite, time.Time{})
	w.build(t)
	return w
}

// build (re)creates the authorizer over the fakes.
func (w *world) build(t testing.TB) {
	t.Helper()
	az, err := app.NewAuthorizer(w.cfg, app.AuthorizerDeps{
		Catalog: app.NewCatalog(slog.New(slog.NewTextHandler(w.logs, nil))), Store: w.store, Principals: w.principals,
		Credentials: w.creds, Structure: w.structure, Policies: w.policies, Versions: w.versions, Recorder: w.rec,
		Clock: w.clk, Log: slog.New(slog.NewTextHandler(w.logs, nil)),
	})
	if err != nil {
		t.Fatal(err)
	}
	w.az = az
}

func mustNoErr(t testing.TB, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func (w *world) grant(t testing.TB, s domain.Subject, tg domain.Target, l domain.Level, exp time.Time, caps ...domain.Capability) ids.ID {
	t.Helper()
	id := ids.New()
	mustNoErr(t, w.store.InsertGrant(context.Background(), domain.Grant{ID: id, Subject: s, Target: tg, Level: l,
		Caps: domain.CapsOf(caps...), ExpiresAt: exp, Reason: "test", GrantedAt: worldStart}))
	return id
}

func (w *world) addSession(p ids.ID) {
	s, d := ids.New(), ids.New()
	w.sessions[p], w.devices[p] = s, d
	w.creds.PutDevice(d, fake.Device{PrincipalID: p, State: authz.DeviceActive})
	w.creds.PutSession(s, fake.Session{PrincipalID: p, DeviceID: d, ExpiresAt: worldStart.Add(365 * 24 * time.Hour)})
}

func (w *world) addToken(p ids.ID) {
	tok := ids.New()
	w.tokens[p] = tok
	w.creds.PutToken(tok, fake.Token{PrincipalID: p})
}

func (w *world) addMainThread(channel ids.ID) {
	th := ids.New()
	w.mainThread[channel] = th
	w.structure.PutThread(authz.ThreadFacts{ID: th, ChannelID: channel, Main: true})
}

// user returns the principal of a person in its session with a fresh step-up.
func (w *world) user(id ids.ID) actor.Actor {
	return actor.Actor{ID: id, Kind: actor.KindUser, SessionID: w.sessions[id], DeviceID: w.devices[id], StepUpAt: w.clk.Now()}
}

// agent returns the principal of an agent with its token.
func (w *world) agent(id ids.ID) actor.Actor {
	return actor.Actor{ID: id, Kind: actor.KindAgent, TokenID: w.tokens[id]}
}

// app returns the principal of the app Tracker with scope messages:write in its installation and
// in its token (M2-24: an app needs the scope in both).
func (w *world) app() actor.Actor {
	return actor.Actor{ID: w.tracker, Kind: actor.KindApp, TokenID: w.tokens[w.tracker], InstallationID: w.install,
		GrantedScopes: []string{domain.ScopeMessagesWrite}, Scopes: []string{domain.ScopeMessagesWrite}}
}

// msg is a message in the main thread of channel, written by author.
func (w *world) msg(channel, author ids.ID) authorize.Resource {
	return authorize.Resource{Kind: authorize.ResourceMessage, ID: ids.New(), ThreadID: w.mainThread[channel], OwnerID: author}
}

// thread is the main thread of channel.
func (w *world) thread(channel ids.ID) authorize.Resource {
	return authorize.Resource{Kind: authorize.ResourceThread, ID: w.mainThread[channel]}
}

// channel is the channel resource.
func (w *world) channel(id ids.ID) authorize.Resource {
	return authorize.Resource{Kind: authorize.ResourceChannel, ID: id}
}

// invoke is agent.invoke of agent in the main thread of channel by a human message.
func (w *world) invoke(channel, agent ids.ID, mention bool) authorize.Resource {
	r := w.thread(channel)
	r.Invoke = &authorize.InvokeFacts{AgentID: agent, TriggerOrigin: "human", MentionsAgent: mention}
	return r
}

func (w *world) decide(t testing.TB, ctx context.Context, p authorize.Principal, a authorize.Action, r authorize.Resource) authorize.Decision {
	t.Helper()
	d, err := w.az.Authorize(ctx, p, a, r)
	if err != nil {
		t.Fatalf("Authorize(%s): %v", a, err)
	}
	return d
}

// expect checks outcome and reason of one call.
func (w *world) expect(t testing.TB, p authorize.Principal, a authorize.Action, r authorize.Resource, o authorize.Outcome, reason string) {
	t.Helper()
	d := w.decide(t, context.Background(), p, a, r)
	if d.Outcome != o || string(d.Reason) != reason {
		t.Fatalf("%s: got %s/%s, want %s/%s", a, d.Outcome, d.Reason, o, reason)
	}
}

func (w *world) allow(t testing.TB, p authorize.Principal, a authorize.Action, r authorize.Resource) {
	t.Helper()
	w.expect(t, p, a, r, authorize.OutcomeAllow, "")
}

// errorLines counts the ERROR records in the log.
func (w *world) errorLines() int {
	return strings.Count(w.logs.String(), "level=ERROR")
}

func fakeSessionRevoked(p, device ids.ID) fake.Session {
	return fake.Session{PrincipalID: p, DeviceID: device, Revoked: true, ExpiresAt: worldStart.Add(365 * 24 * time.Hour)}
}

func fakeDevice(p ids.ID, s authz.DeviceState) fake.Device {
	return fake.Device{PrincipalID: p, State: s}
}
