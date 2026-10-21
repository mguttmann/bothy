// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"slices"
	"strconv"
	stdsync "sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/presence"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/messaging"
	"github.com/OWNER/bothy/services/server/internal/notify"
	"github.com/OWNER/bothy/services/server/internal/notify/app"
	"github.com/OWNER/bothy/services/server/internal/notify/domain"
	"github.com/OWNER/bothy/services/server/internal/sync"
)

// pushStart is the clock of the push world.
var pushStart = time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)

// pushConfig is the configuration of the push world (fixture values of the test configuration).
func pushConfig() app.PushConfig {
	return app.PushConfig{
		CoalesceWindow: time.Minute, MobileDelay: 30 * time.Second, DefaultAdminEnabled: true,
		DefaultInputNeededBreaksQuietHours: true, BadgeSyncMinInterval: 5 * time.Minute, MaxAge: time.Hour,
		RetryBase: 10 * time.Second, RetryCap: 5 * time.Minute, MaxConcurrentRequests: 4, MaxConsecutiveFailures: 3,
	}
}

// counter counts calls per port name.
type counter struct {
	mu stdsync.Mutex
	m  map[string]int
}

func (c *counter) inc(name string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.m == nil {
		c.m = map[string]int{}
	}
	c.m[name]++
}

func (c *counter) get(name string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.m[name]
}

// txKey marks the context inside the fake unit of work.
type txKey struct{}

// inTx reports whether ctx is inside the fake unit of work.
func inTx(ctx context.Context) bool { return ctx.Value(txKey{}) != nil }

// pushUoW is a unit of work that only marks the context.
type pushUoW struct{}

func (pushUoW) Do(ctx context.Context, fn func(context.Context) error, _ ...txn.Option) error {
	if inTx(ctx) {
		return fn(ctx)
	}
	return fn(context.WithValue(ctx, txKey{}, true))
}

// pushWorld holds the fakes of every port of the push path.
type pushWorld struct {
	t     *testing.T
	calls *counter
	clock *clock.Fake
	cfg   app.PushConfig
	log   bytes.Buffer

	// people and structure
	alex, sam, robin, owner, lotse, textbot ids.ID
	space, channel, thread                  ids.ID
	dmChannel, dmThread                     ids.ID
	principals                              map[ids.ID]identity.PrincipalView
	readers                                 map[ids.ID][]ids.ID
	threads                                 map[ids.ID]chat.ThreadInfo
	denied                                  map[[2]ids.ID]bool // (user, thread) without push.deliver
	scope                                   map[ids.ID][]ids.ID
	levels                                  map[ids.ID]domain.Level // personal level per person on every thread
	settings                                map[ids.ID]map[string]json.RawMessage
	devices                                 map[ids.ID][]identity.DeviceRef
	regs                                    map[ids.ID][]notify.PushRegistration
	conns                                   map[ids.ID][]presence.Connection
	positions                               map[[2]ids.ID]int64
	unread                                  []chat.ThreadPosition
	mentions                                []messaging.ThreadMentionCount

	// messages and history
	anchors  []messaging.AnchorRef
	agentMsg []agentMessage
	newest   map[ids.ID]messaging.MessageRef
	history  []sync.Event

	// recorded effects
	sched     *fakeScheduler
	routes    []domain.Route
	skips     []string
	sent      []string
	store     *fakeRegStore
	providers []app.PushProvider
	presence  presence.View
	liveRoute app.LiveRoutes
}

// agentMessage is one message of an agent in a thread of the fake messaging.
type agentMessage struct {
	id, thread, agent ids.ID
	seq               int64
}

// newPushWorld returns the world: Support/Allgemein with readers Alex, Sam, Robin (disabled),
// the Owner, the agent Lotse and the app Textbot; the DM of Alex and Sam; every human with one
// phone (ios) and one registration (apns).
func newPushWorld(t *testing.T) *pushWorld {
	t.Helper()
	w := &pushWorld{
		t: t, calls: &counter{}, clock: clock.NewFake(pushStart), cfg: pushConfig(),
		alex: ids.New(), sam: ids.New(), robin: ids.New(), owner: ids.New(), lotse: ids.New(), textbot: ids.New(),
		space: ids.New(), channel: ids.New(), thread: ids.New(), dmChannel: ids.New(), dmThread: ids.New(),
		principals: map[ids.ID]identity.PrincipalView{}, readers: map[ids.ID][]ids.ID{}, threads: map[ids.ID]chat.ThreadInfo{},
		denied: map[[2]ids.ID]bool{}, scope: map[ids.ID][]ids.ID{}, levels: map[ids.ID]domain.Level{},
		settings: map[ids.ID]map[string]json.RawMessage{}, devices: map[ids.ID][]identity.DeviceRef{},
		regs: map[ids.ID][]notify.PushRegistration{}, conns: map[ids.ID][]presence.Connection{}, positions: map[[2]ids.ID]int64{},
		newest: map[ids.ID]messaging.MessageRef{}, sched: newFakeScheduler(), store: newFakeRegStore(),
	}
	for _, h := range []ids.ID{w.alex, w.sam, w.owner} {
		w.principals[h] = identity.PrincipalView{ID: h, Kind: actor.KindUser, Status: identity.StatusActive}
		w.addPhone(h, "apns")
	}
	w.principals[w.robin] = identity.PrincipalView{ID: w.robin, Kind: actor.KindUser, Status: identity.StatusDisabled}
	w.addPhone(w.robin, "apns")
	w.principals[w.lotse] = identity.PrincipalView{ID: w.lotse, Kind: actor.KindAgent, Status: identity.StatusActive}
	w.principals[w.textbot] = identity.PrincipalView{ID: w.textbot, Kind: actor.KindApp, Status: identity.StatusActive}
	w.readers[w.channel] = []ids.ID{w.alex, w.sam, w.robin, w.owner, w.lotse, w.textbot}
	w.readers[w.dmChannel] = []ids.ID{w.alex, w.sam}
	w.threads[w.thread] = chat.ThreadInfo{ID: w.thread, ChannelID: w.channel, SpaceID: w.space, ChannelKind: "text", Kind: "thread"}
	w.threads[w.dmThread] = chat.ThreadInfo{ID: w.dmThread, ChannelID: w.dmChannel, ChannelKind: "dm", Kind: "main"}
	return w
}

// addPhone gives person an active ios phone with one registration of provider and returns the
// registration ID.
func (w *pushWorld) addPhone(person ids.ID, provider string) ids.ID {
	return w.addDevice(person, "ios", provider)
}

// addDevice gives person an active device of platform with one registration of provider.
func (w *pushWorld) addDevice(person ids.ID, platform, provider string) ids.ID {
	dev, reg := ids.New(), ids.New()
	w.devices[person] = append(w.devices[person], identity.DeviceRef{ID: dev, Platform: platform})
	r := notify.PushRegistration{ID: reg, UserID: person, DeviceID: dev, Provider: provider, Environment: "production",
		AppID: "com.example.bothy", CreatedAt: pushStart.Add(-24 * time.Hour)}
	w.regs[person] = append(w.regs[person], r)
	w.store.put(app.Registration{ID: reg, UserID: person, DeviceID: dev, Provider: provider, Environment: "production",
		AppID: "com.example.bothy", Token: "token-" + reg.String()[:8], CreatedAt: r.CreatedAt})
	return reg
}

// push builds the push path over the fakes.
func (w *pushWorld) push() *app.Push {
	w.t.Helper()
	routes := w.liveRoute
	if routes == nil {
		routes = &recordRoutes{w: w}
	}
	p, err := app.NewPush(w.cfg, app.PushDeps{
		Facts: pushFacts{w}, Settings: pushSettings{w}, Authorizer: pushAuthz{w}, Readers: pushReaders{w},
		Principals: pushPrincipals{w}, Devices: pushDevices{w}, Registrations: pushRegs{w}, Store: w.store,
		Presence: w.presenceView(), Routes: routes, Scheduler: w.sched, History: pushHistory{w}, Positions: pushPositions{w},
		Threads: pushThreads{w}, Messages: pushMessages{w}, Mentions: pushMentions{w}, Providers: w.providers, UoW: pushUoW{},
		InstanceID: ids.New(), Clock: w.clock, Metrics: pushMetrics{w}, Log: slog.New(slog.NewTextHandler(&w.log, nil)),
		Jitter: func() float64 { return 1 },
	})
	if err != nil {
		w.t.Fatal(err)
	}
	return p
}

func (w *pushWorld) presenceView() presence.View {
	if w.presence != nil {
		return w.presence
	}
	return pushPresence{w}
}

// handle runs one event through the consumer inside the fake unit of work.
func (w *pushWorld) handle(ev sync.Event) error {
	w.t.Helper()
	p := w.push()
	return pushUoW{}.Do(context.Background(), func(ctx context.Context) error {
		return p.Consumer().HandleEvent(ctx, ev)
	})
}

// mustHandle is handle that fails the test on an error.
func (w *pushWorld) mustHandle(ev sync.Event) {
	w.t.Helper()
	if err := w.handle(ev); err != nil {
		w.t.Fatalf("HandleEvent %s: %v", ev.Type, err)
	}
}

// hints returns the kinds of the hints per person (one entry per NotificationID).
func (w *pushWorld) hints() map[ids.ID][]domain.Kind {
	seen := map[ids.ID]bool{}
	out := map[ids.ID][]domain.Kind{}
	for _, s := range w.sched.sends {
		if seen[s.job.NotificationID] {
			continue
		}
		seen[s.job.NotificationID] = true
		out[s.job.RecipientID] = append(out[s.job.RecipientID], s.job.Kind)
	}
	return out
}

// jobsOf returns the push jobs of person.
func (w *pushWorld) jobsOf(person ids.ID) []app.PushJob {
	var out []app.PushJob
	for _, s := range w.sched.sends {
		if s.job.RecipientID == person {
			out = append(out, s.job)
		}
	}
	return out
}

// seq numbers the events of a test.
var seqMu stdsync.Mutex
var seqNext int64 = 100

func nextSeq() int64 {
	seqMu.Lock()
	defer seqMu.Unlock()
	seqNext++
	return seqNext
}

// msg describes a message.created for messageEvent.
type msg struct {
	id, thread, channel, author ids.ID
	authorKind, origin, kind    string
	seq                         int64
	silent                      bool
	mentions                    []ids.ID
}

// messageEvent returns message.created of m at log seq.
func (w *pushWorld) messageEvent(seq int64, m msg) sync.Event {
	if m.id.IsZero() {
		m.id = ids.New()
	}
	if m.authorKind == "" {
		m.authorKind = "user"
	}
	if m.origin == "" {
		m.origin = "human"
	}
	if m.kind == "" {
		m.kind = "text"
	}
	mentions := m.mentions
	if mentions == nil {
		mentions = []ids.ID{}
	}
	payload := map[string]any{"message": map[string]any{
		"id": m.id, "thread_id": m.thread, "channel_id": m.channel, "thread_seq": m.seq,
		"author": map[string]any{"id": m.author, "kind": m.authorKind}, "origin": m.origin, "kind": m.kind,
		"silent": m.silent, "mention_ids": mentions, "content_md": "secret words",
	}}
	return w.event(seq, "message.created", "message", m.id, ids.ID{}, payload)
}

// runEvent returns run.state_changed of run at log seq.
func (w *pushWorld) runEvent(seq int64, run, thread, agent, starter ids.ID, state string) sync.Event {
	payload := map[string]any{"run": map[string]any{
		"id": run, "thread_id": thread, "channel_id": w.threads[thread].ChannelID, "agent_id": agent,
		"triggered_by": starter, "state": state, "summary": "secret summary",
	}}
	return w.event(seq, "run.state_changed", "run", run, ids.ID{}, payload)
}

// event returns one stored event.
func (w *pushWorld) event(seq int64, typ, kind string, id, audience ids.ID, payload any) sync.Event {
	b, err := json.Marshal(payload)
	if err != nil {
		w.t.Fatal(err)
	}
	return sync.Event{Seq: seq, ID: ids.New(), Type: typ, OccurredAt: w.clock.Now(), ResourceKind: kind, ResourceID: id,
		AudiencePrincipalID: audience, Payload: b}
}

// remember stores ev in the fake history.
func (w *pushWorld) remember(ev sync.Event) sync.Event {
	w.history = append(w.history, ev)
	return ev
}

// ---- fakes of the ports ----

type pushFacts struct{ w *pushWorld }

func (f pushFacts) LevelFacts(_ context.Context, thread ids.ID, users []ids.ID) (map[ids.ID]domain.LevelFacts, error) {
	f.w.calls.inc("LevelFacts")
	out := map[ids.ID]domain.LevelFacts{}
	for _, u := range users {
		out[u] = f.w.levelFacts(u, thread)
	}
	return out, nil
}

func (f pushFacts) QuietRules(_ context.Context, _ []ids.ID) (map[ids.ID][]domain.QuietRule, error) {
	f.w.calls.inc("QuietRules")
	return map[ids.ID][]domain.QuietRule{}, nil
}

func (f pushFacts) LevelFactsOfUser(_ context.Context, user ids.ID, threads []ids.ID) (map[ids.ID]domain.LevelFacts, error) {
	f.w.calls.inc("LevelFactsOfUser")
	out := map[ids.ID]domain.LevelFacts{}
	for _, t := range threads {
		out[t] = f.w.levelFacts(user, t)
	}
	return out, nil
}

func (w *pushWorld) levelFacts(user, thread ids.ID) domain.LevelFacts {
	lf := domain.LevelFacts{InstanceDefault: domain.LevelAll, DirectMessage: w.threads[thread].ChannelKind == "dm"}
	if l, ok := w.levels[user]; ok {
		lf.Thread = &domain.Preference{Level: l}
	}
	return lf
}

type pushSettings struct{ w *pushWorld }

func (s pushSettings) ValuesFor(_ context.Context, users []ids.ID, _ []string) (map[ids.ID]map[string]json.RawMessage, error) {
	s.w.calls.inc("ValuesFor")
	out := map[ids.ID]map[string]json.RawMessage{}
	for _, u := range users {
		if v, ok := s.w.settings[u]; ok {
			out[u] = v
		}
	}
	return out, nil
}

type pushAuthz struct{ w *pushWorld }

func (a pushAuthz) Authorize(_ context.Context, p authorize.Principal, act authorize.Action, r authorize.Resource) (authorize.Decision, error) {
	a.w.calls.inc("Authorize")
	if !authorize.IsRecheckPrincipal(p) || act != authorize.ActionPushDeliver || r.Kind != authorize.ResourceThread {
		return authorize.Decision{}, errs.New(errs.CodeInternalError, "unexpected authorize")
	}
	if a.w.denied[[2]ids.ID{p.PrincipalID(), r.ID}] || a.w.principals[p.PrincipalID()].Status != identity.StatusActive {
		return authorize.Decision{Outcome: authorize.OutcomeNotFound}, nil
	}
	return authorize.Decision{Outcome: authorize.OutcomeAllow}, nil
}

func (a pushAuthz) Scope(_ context.Context, p authorize.Principal, _ authorize.Action) (authorize.Scope, error) {
	a.w.calls.inc("Scope")
	return authorize.Scope{ChannelIDs: a.w.scope[p.PrincipalID()]}, nil
}

func (pushAuthz) Observe(context.Context, authorize.LogEvent) {}

type pushReaders struct{ w *pushWorld }

func (r pushReaders) Readers(_ context.Context, channel ids.ID) ([]ids.ID, error) {
	r.w.calls.inc("Readers")
	return slices.Clone(r.w.readers[channel]), nil
}

type pushPrincipals struct{ w *pushWorld }

func (p pushPrincipals) Principals(_ context.Context, list []ids.ID) (map[ids.ID]identity.PrincipalView, error) {
	out := map[ids.ID]identity.PrincipalView{}
	for _, id := range list {
		if v, ok := p.w.principals[id]; ok {
			out[id] = v
		}
	}
	return out, nil
}

type pushDevices struct{ w *pushWorld }

func (d pushDevices) ActiveForUsers(_ context.Context, users []ids.ID) (map[ids.ID][]identity.DeviceRef, error) {
	d.w.calls.inc("ActiveForUsers")
	out := map[ids.ID][]identity.DeviceRef{}
	for _, u := range users {
		if v, ok := d.w.devices[u]; ok {
			out[u] = slices.Clone(v)
		}
	}
	return out, nil
}

type pushRegs struct{ w *pushWorld }

func (r pushRegs) Active(_ context.Context, users []ids.ID) (map[ids.ID][]notify.PushRegistration, error) {
	r.w.calls.inc("Active")
	out := map[ids.ID][]notify.PushRegistration{}
	for _, u := range users {
		for _, reg := range r.w.regs[u] {
			if _, st, ok := r.w.store.get(reg.ID); ok && st.DisabledAt.IsZero() {
				out[u] = append(out[u], reg)
			}
		}
	}
	return out, nil
}

type pushPresence struct{ w *pushWorld }

func (p pushPresence) Connections(_ context.Context, users []ids.ID) (map[ids.ID][]presence.Connection, error) {
	p.w.calls.inc("Connections")
	out := map[ids.ID][]presence.Connection{}
	for _, u := range users {
		if c, ok := p.w.conns[u]; ok {
			out[u] = c
		}
	}
	return out, nil
}

type pushHistory struct{ w *pushWorld }

func (h pushHistory) ResourceEvents(_ context.Context, q sync.ResourceQuery) ([]sync.Event, error) {
	var out []sync.Event
	for _, ev := range h.w.history {
		if ev.ResourceKind == q.ResourceKind && ev.ResourceID == q.ResourceID && slices.Contains(q.Types, ev.Type) && ev.Seq < q.Before {
			out = append(out, ev)
		}
	}
	slices.SortFunc(out, func(a, b sync.Event) int { return int(a.Seq - b.Seq) })
	return out, nil
}

func (h pushHistory) PersonalEvents(_ context.Context, q sync.PersonalQuery) ([]sync.Event, error) {
	var out []sync.Event
	for _, ev := range h.w.history {
		if ev.AudiencePrincipalID == q.AudienceID && slices.Contains(q.Types, ev.Type) && ev.Seq > q.After {
			out = append(out, ev)
		}
	}
	slices.SortFunc(out, func(a, b sync.Event) int { return int(a.Seq - b.Seq) })
	return out, nil
}

type pushPositions struct{ w *pushWorld }

func (p pushPositions) PositionsFor(_ context.Context, thread ids.ID, users []ids.ID) (map[ids.ID]int64, error) {
	out := map[ids.ID]int64{}
	for _, u := range users {
		out[u] = p.w.positions[[2]ids.ID{u, thread}]
	}
	return out, nil
}

func (p pushPositions) PositionsOfUser(_ context.Context, user ids.ID, threads []ids.ID) (map[ids.ID]int64, error) {
	out := map[ids.ID]int64{}
	for _, t := range threads {
		out[t] = p.w.positions[[2]ids.ID{user, t}]
	}
	return out, nil
}

type pushThreads struct{ w *pushWorld }

func (t pushThreads) Info(_ context.Context, thread ids.ID) (chat.ThreadInfo, error) {
	info, ok := t.w.threads[thread]
	if !ok {
		return chat.ThreadInfo{}, errs.New(errs.CodeNotFound, "")
	}
	return info, nil
}

func (t pushThreads) UnreadCandidates(_ context.Context, channels []ids.ID) ([]chat.ThreadPosition, error) {
	var out []chat.ThreadPosition
	for _, c := range t.w.unread {
		if slices.Contains(channels, c.ChannelID) {
			out = append(out, c)
		}
	}
	return out, nil
}

type pushMessages struct{ w *pushWorld }

func (m pushMessages) LastAnchorBefore(_ context.Context, thread ids.ID, before int64) (messaging.AnchorRef, bool, error) {
	var best messaging.AnchorRef
	found := false
	for _, a := range m.w.anchors {
		if m.w.anchorThread(a) == thread && a.ThreadSeq < before && (!found || a.ThreadSeq > best.ThreadSeq) {
			best, found = a, true
		}
	}
	return best, found, nil
}

func (w *pushWorld) anchorThread(messaging.AnchorRef) ids.ID { return w.thread }

func (m pushMessages) AnchorOf(_ context.Context, run ids.ID) (messaging.AnchorRef, bool, error) {
	for _, a := range m.w.anchors {
		if a.RunID == run {
			return a, true, nil
		}
	}
	return messaging.AnchorRef{}, false, nil
}

func (m pushMessages) LastAgentMessageBefore(_ context.Context, thread, agent ids.ID, after, before int64) (messaging.MessageRef, bool, error) {
	var best agentMessage
	found := false
	for _, a := range m.w.agentMsg {
		if a.thread == thread && a.agent == agent && a.seq > after && a.seq < before && (!found || a.seq > best.seq) {
			best, found = a, true
		}
	}
	return messaging.MessageRef{ID: best.id, ThreadSeq: best.seq}, found, nil
}

func (m pushMessages) Newest(_ context.Context, thread ids.ID) (messaging.MessageRef, bool, error) {
	r, ok := m.w.newest[thread]
	return r, ok, nil
}

type pushMentions struct{ w *pushWorld }

func (m pushMentions) CountUnread(_ context.Context, q messaging.UnreadMentionQuery) ([]messaging.ThreadMentionCount, error) {
	var out []messaging.ThreadMentionCount
	for _, c := range m.w.mentions {
		if slices.Contains(q.VisibleChannelIDs, c.ChannelID) {
			out = append(out, c)
		}
	}
	return out, nil
}

type pushMetrics struct{ w *pushWorld }

func (m pushMetrics) Sent(_ context.Context, provider, outcome string) {
	m.w.sent = append(m.w.sent, provider+"/"+outcome)
}

func (pushMetrics) SendDuration(context.Context, string, time.Duration) {}

func (m pushMetrics) Skipped(_ context.Context, reason string) { m.w.skips = append(m.w.skips, reason) }

// recordRoutes records the live routes and accepts them.
type recordRoutes struct{ w *pushWorld }

func (r *recordRoutes) Gateway(_ context.Context, _ app.Hint, route domain.Route) error {
	r.w.routes = append(r.w.routes, route)
	return nil
}

func (r *recordRoutes) Delayed(_ context.Context, _ app.Hint, route domain.Route) error {
	r.w.routes = append(r.w.routes, route)
	return nil
}

// scheduledSend is one inserted push job.
type scheduledSend struct {
	job app.PushJob
	at  time.Time
}

// fakeScheduler keeps the jobs in memory with the uniqueness of the jobs adapter.
type fakeScheduler struct {
	mu      stdsync.Mutex
	windows map[[2]ids.ID]time.Time
	flushes map[string]time.Time
	sends   []scheduledSend
	syncs   map[ids.ID]time.Time
	syncArg map[ids.ID]int64
	holds   map[ids.ID]time.Time
}

func newFakeScheduler() *fakeScheduler {
	return &fakeScheduler{windows: map[[2]ids.ID]time.Time{}, flushes: map[string]time.Time{}, syncs: map[ids.ID]time.Time{},
		syncArg: map[ids.ID]int64{}, holds: map[ids.ID]time.Time{}}
}

func flushKey(u, t ids.ID, end time.Time) string {
	return u.String() + "/" + t.String() + "/" + strconv.FormatInt(end.UnixMicro(), 10)
}

func (s *fakeScheduler) WindowEnd(_ context.Context, u, t ids.ID) (time.Time, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	end, ok := s.windows[[2]ids.ID{u, t}]
	return end, ok, nil
}

func (s *fakeScheduler) OpenWindow(_ context.Context, u, t ids.ID, end time.Time) (bool, time.Time, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if at, ok := s.windows[[2]ids.ID{u, t}]; ok {
		return false, at, nil
	}
	s.windows[[2]ids.ID{u, t}] = end
	return true, end, nil
}

func (s *fakeScheduler) Collect(_ context.Context, u, t ids.ID, end time.Time) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	k := flushKey(u, t, end)
	if _, ok := s.flushes[k]; ok {
		return false, nil
	}
	s.flushes[k] = end
	return true, nil
}

func (s *fakeScheduler) Send(ctx context.Context, job app.PushJob, at time.Time) error {
	if !inTx(ctx) {
		return errs.New(errs.CodeInternalError, "insert outside a unit of work")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sends = append(s.sends, scheduledSend{job: job, at: at})
	return nil
}

func (s *fakeScheduler) BadgeSyncPending(_ context.Context, u ids.ID) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.syncs[u]
	return ok, nil
}

func (s *fakeScheduler) HoldEnd(_ context.Context, u ids.ID) (time.Time, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	end, ok := s.holds[u]
	return end, ok, nil
}

func (s *fakeScheduler) ScheduleBadgeSync(_ context.Context, u ids.ID, after int64, at time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.syncs[u]; !ok {
		s.syncs[u], s.syncArg[u] = at, after
	}
	return nil
}

func (s *fakeScheduler) Hold(_ context.Context, u ids.ID, at time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.holds[u]; !ok {
		s.holds[u] = at
	}
	return nil
}

// endWindow drops the window marker and the flush of (u, t), as the jobs complete at the end.
func (s *fakeScheduler) endWindow(u, t ids.ID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.windows, [2]ids.ID{u, t})
	for k := range s.flushes {
		if len(k) > 0 && k[:len(u.String())+1+len(t.String())] == u.String()+"/"+t.String() {
			delete(s.flushes, k)
		}
	}
}

// fakeRegStore is the registration store in memory.
type fakeRegStore struct {
	mu      stdsync.Mutex
	regs    map[ids.ID]app.Registration
	states  map[ids.ID]domain.RegistrationState
	updates int
}

func newFakeRegStore() *fakeRegStore {
	return &fakeRegStore{regs: map[ids.ID]app.Registration{}, states: map[ids.ID]domain.RegistrationState{}}
}

func (s *fakeRegStore) put(r app.Registration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.regs[r.ID] = r
	s.states[r.ID] = domain.RegistrationState{CreatedAt: r.CreatedAt}
}

func (s *fakeRegStore) get(id ids.ID) (app.Registration, domain.RegistrationState, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.regs[id]
	return r, s.states[id], ok
}

func (s *fakeRegStore) Load(ctx context.Context, id ids.ID) (app.Registration, domain.RegistrationState, bool, error) {
	if !inTx(ctx) {
		return app.Registration{}, domain.RegistrationState{}, false, errs.New(errs.CodeInternalError, "load outside a unit of work")
	}
	r, st, ok := s.get(id)
	return r, st, ok, nil
}

func (s *fakeRegStore) Lock(ctx context.Context, id ids.ID) (domain.RegistrationState, bool, error) {
	if !inTx(ctx) {
		return domain.RegistrationState{}, false, errs.New(errs.CodeInternalError, "lock outside a unit of work")
	}
	_, st, ok := s.get(id)
	return st, ok, nil
}

func (s *fakeRegStore) Update(_ context.Context, id ids.ID, st domain.RegistrationState) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.states[id] = st
	s.updates++
	return nil
}

func (s *fakeRegStore) ActiveCounts(context.Context) (map[string]int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := map[string]int64{}
	for id, r := range s.regs {
		if s.states[id].DisabledAt.IsZero() {
			out[r.Provider]++
		}
	}
	return out, nil
}

// testProvider is a provider that records its calls, asserts it never runs inside a unit of work
// and can block until released.
type testProvider struct {
	name    string
	mu      stdsync.Mutex
	calls   []app.Notification
	result  domain.SendResult
	err     error
	inTx    bool
	block   chan struct{}
	running int
	maxRun  int
}

func (p *testProvider) Name() string { return p.name }

func (p *testProvider) Send(ctx context.Context, _ app.Registration, n app.Notification) (domain.SendResult, error) {
	p.mu.Lock()
	p.calls = append(p.calls, n)
	p.inTx = p.inTx || inTx(ctx)
	p.running++
	p.maxRun = max(p.maxRun, p.running)
	block := p.block
	p.mu.Unlock()
	if block != nil {
		<-block
	}
	p.mu.Lock()
	p.running--
	p.mu.Unlock()
	return p.result, p.err
}
