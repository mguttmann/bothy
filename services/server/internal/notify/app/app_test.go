// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/notify"
	"github.com/OWNER/bothy/services/server/internal/notify/app"
	"github.com/OWNER/bothy/services/server/internal/notify/domain"
)

// fakeStore records the calls of the push path reads and the cleanup.
type fakeStore struct {
	app.Store
	mu         sync.Mutex
	levelCalls int
	levelArgs  struct{ thread, channel, space ids.ID }
	levelRows  []app.PreferenceRow
	rules      map[ids.ID][]notify.QuietHoursRule
	deleted    []app.PreferenceRow
	delArgs    struct {
		space            ids.ID
		channels, thread []ids.ID
	}
}

func (s *fakeStore) LevelRows(_ context.Context, users []ids.ID, thread, channel, space ids.ID) ([]app.PreferenceRow, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.levelCalls++
	s.levelArgs.thread, s.levelArgs.channel, s.levelArgs.space = thread, channel, space
	var out []app.PreferenceRow
	for _, r := range s.levelRows {
		if slices.Contains(users, r.UserID) {
			out = append(out, r)
		}
	}
	return out, nil
}

func (s *fakeStore) Preferences(_ context.Context, user ids.ID) ([]notify.Preference, error) {
	var out []notify.Preference
	for _, r := range s.levelRows {
		if r.UserID == user {
			out = append(out, r.Preference)
		}
	}
	return out, nil
}

func (s *fakeStore) RulesOfUsers(_ context.Context, users []ids.ID) (map[ids.ID][]notify.QuietHoursRule, error) {
	out := map[ids.ID][]notify.QuietHoursRule{}
	for _, u := range users {
		if r, ok := s.rules[u]; ok {
			out[u] = r
		}
	}
	return out, nil
}

func (s *fakeStore) DeletePreferencesOfTargets(_ context.Context, space ids.ID, channels, threads []ids.ID) ([]app.PreferenceRow, error) {
	s.delArgs.space, s.delArgs.channels, s.delArgs.thread = space, channels, threads
	return s.deleted, nil
}

// fakePaths is the chat side.
type fakePaths struct {
	app.PathSource
	calls    int
	path     chat.NotificationPath
	channels map[ids.ID][]ids.ID
	threads  map[ids.ID][]ids.ID
	// channelPaths are the paths of channels (kind channel); threadCalls counts the reads of a
	// thread path.
	channelPaths map[ids.ID]chat.NotificationPath
	threadCalls  int
}

func (p *fakePaths) NotificationPath(_ context.Context, kind string, id ids.ID) (chat.NotificationPath, error) {
	p.calls++
	if kind == chat.TargetChannel {
		if path, ok := p.channelPaths[id]; ok {
			return path, nil
		}
		return chat.NotificationPath{}, errs.New(errs.CodeNotFound, "")
	}
	p.threadCalls++
	if kind != chat.TargetThread || id != p.path.ThreadID {
		return chat.NotificationPath{}, errs.New(errs.CodeNotFound, "")
	}
	return p.path, nil
}

func (p *fakePaths) ChannelIDsOfSpace(_ context.Context, space ids.ID) ([]ids.ID, error) {
	return p.channels[space], nil
}

func (p *fakePaths) ThreadIDsOfChannels(_ context.Context, chs []ids.ID) ([]ids.ID, error) {
	var out []ids.ID
	for _, c := range chs {
		out = append(out, p.threads[c]...)
	}
	return out, nil
}

// fakeSettings is the sync side.
type fakeSettings struct {
	calls int
	vals  map[ids.ID]map[string]json.RawMessage
}

func (s *fakeSettings) ValuesFor(_ context.Context, users []ids.ID, keys []string) (map[ids.ID]map[string]json.RawMessage, error) {
	s.calls++
	out := map[ids.ID]map[string]json.RawMessage{}
	for _, u := range users {
		if v, ok := s.vals[u]; ok {
			out[u] = v
		}
	}
	return out, nil
}

// fakeUsers is the identity side.
type fakeUsers struct {
	calls int
	m     map[ids.ID]identity.UserView
}

func (u *fakeUsers) Users(_ context.Context, list []ids.ID) (map[ids.ID]identity.UserView, error) {
	u.calls++
	out := map[ids.ID]identity.UserView{}
	for _, id := range list {
		if v, ok := u.m[id]; ok {
			out[id] = v
		}
	}
	return out, nil
}

// appender collects the appended events.
type appender struct{ events []eventlog.Event }

func (a *appender) Append(_ context.Context, es ...eventlog.Event) error {
	a.events = append(a.events, es...)
	return nil
}

// noUoW, noAuthz satisfy New; the tests here never write through a use case.
type noUoW struct{ txn.UnitOfWork }
type noAuthz struct{ authorize.Authorizer }

// world is the use cases over the fakes with a captured log.
type world struct {
	svc      *app.Service
	store    *fakeStore
	paths    *fakePaths
	settings *fakeSettings
	users    *fakeUsers
	log      *bytes.Buffer
}

func newWorld(t *testing.T) *world {
	t.Helper()
	w := &world{store: &fakeStore{rules: map[ids.ID][]notify.QuietHoursRule{}}, paths: &fakePaths{}, settings: &fakeSettings{},
		users: &fakeUsers{m: map[ids.ID]identity.UserView{}}, log: &bytes.Buffer{}}
	svc, err := app.New(app.Config{EventPayloadMaxBytes: 4096, UserDefaultLevel: "mentions"}, app.Deps{
		Store: w.store, UoW: noUoW{}, Authorizer: noAuthz{}, Events: &appender{}, Paths: w.paths, Settings: w.settings, Users: w.users,
		Log: slog.New(slog.NewTextHandler(w.log, nil)),
	})
	if err != nil {
		t.Fatal(err)
	}
	w.svc = svc
	return w
}

// TestLevelFacts_OneQueryForAllCandidates (13 section 12 point 2): 50 people cost one preference
// query, one path read and one settings read; a DM path leaves the space out; a stored personal
// default outside the levels counts as unset and logs one Warn line without value and id.
func TestLevelFacts_OneQueryForAllCandidates(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	space, channel, thread := ids.New(), ids.New(), ids.New()
	w.paths.path = chat.NotificationPath{Kind: chat.TargetThread, SpaceID: space, ChannelID: channel, ThreadID: thread, ChannelKind: "text",
		DefaultNotificationLevel: "mute"}
	people := make([]ids.ID, 50)
	for i := range people {
		people[i] = ids.New()
	}
	until := time.Date(2026, 10, 14, 10, 0, 0, 0, time.UTC)
	w.store.levelRows = []app.PreferenceRow{
		{UserID: people[0], Preference: notify.Preference{Target: notify.Target{Kind: "thread", ID: thread}, Level: "all", MutedUntil: until}},
		{UserID: people[0], Preference: notify.Preference{Target: notify.Target{Kind: "space", ID: space}, Level: "mute"}},
		{UserID: people[1], Preference: notify.Preference{Target: notify.Target{Kind: "channel", ID: channel}, Level: "mentions"}},
	}
	w.settings.vals = map[ids.ID]map[string]json.RawMessage{
		people[1]: {"notify.default_level": json.RawMessage(`"all"`)},
		people[2]: {"notify.default_level": json.RawMessage(`"loud"`)},
	}
	facts, err := w.svc.Facts().LevelFacts(context.Background(), thread, people)
	if err != nil {
		t.Fatal(err)
	}
	if w.store.levelCalls != 1 || w.paths.calls != 1 || w.settings.calls != 1 {
		t.Fatalf("calls: preferences %d, path %d, settings %d; want one each", w.store.levelCalls, w.paths.calls, w.settings.calls)
	}
	if a := w.store.levelArgs; a.thread != thread || a.channel != channel || a.space != space {
		t.Fatalf("query targets %+v", a)
	}
	if len(facts) != 50 {
		t.Fatalf("%d facts, want 50", len(facts))
	}
	f0 := facts[people[0]]
	if f0.Thread == nil || f0.Thread.Level != domain.LevelAll || !f0.Thread.MutedUntil.Equal(until) || f0.Space == nil || f0.Channel != nil {
		t.Fatalf("facts of the first person: %+v", f0)
	}
	if f0.ChannelDefault != domain.LevelMute || f0.DirectMessage || f0.InstanceDefault != domain.LevelMentions || f0.PersonalDefault != "" {
		t.Fatalf("shared facts: %+v", f0)
	}
	if f := facts[people[1]]; f.Channel == nil || f.PersonalDefault != domain.LevelAll {
		t.Fatalf("facts of the second person: %+v", f)
	}
	if f := facts[people[2]]; f.PersonalDefault != "" {
		t.Fatalf("a stored default outside the levels is unset: %+v", f)
	}
	logs := w.log.String()
	if strings.Count(logs, "level=WARN") != 1 || strings.Contains(logs, "loud") || strings.Contains(logs, people[2].String()) {
		t.Fatalf("log: %s", logs)
	}

	// A DM channel: no space, the space condition is left out.
	w.paths.path = chat.NotificationPath{Kind: chat.TargetThread, ChannelID: channel, ThreadID: thread, ChannelKind: "dm"}
	facts, err = w.svc.Facts().LevelFacts(context.Background(), thread, people[:1])
	if err != nil {
		t.Fatal(err)
	}
	if !w.store.levelArgs.space.IsZero() || !facts[people[0]].DirectMessage {
		t.Fatalf("dm: space %v, facts %+v", w.store.levelArgs.space, facts[people[0]])
	}
	if _, err := w.svc.Facts().LevelFacts(context.Background(), ids.New(), people); errs.KindOf(err) != errs.KindNotFound {
		t.Fatalf("unknown thread: %v", err)
	}
}

// TestLevelFactsOfUser_OnePathReadPerChannel (findings srv-notify-2 and performance-7,
// docs/decisions.d/AUD-srv-notify-01.md AUD-SN01/4): the level facts of one person for 3000 unread
// threads in three channels cost one path read per channel, one read of the person's rows and one
// settings read, and no read of a thread path. The facts are those of the thread's place: the
// channel default and the DM flag of its channel, the person's own row of the thread, of its
// channel and of its space. The threads of a channel that no longer exists are left out.
func TestLevelFactsOfUser_OnePathReadPerChannel(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	alex, space := ids.New(), ids.New()
	text, muted, dm, gone := ids.New(), ids.New(), ids.New(), ids.New()
	w.paths.channelPaths = map[ids.ID]chat.NotificationPath{
		text:  {Kind: chat.TargetChannel, SpaceID: space, ChannelID: text, ChannelKind: "text", DefaultNotificationLevel: "all"},
		muted: {Kind: chat.TargetChannel, SpaceID: space, ChannelID: muted, ChannelKind: "forum", DefaultNotificationLevel: "mute"},
		dm:    {Kind: chat.TargetChannel, ChannelID: dm, ChannelKind: "dm"},
	}
	var threads []chat.ThreadPosition
	for i := range 3000 {
		ch := []ids.ID{text, muted, dm}[i%3]
		threads = append(threads, chat.ThreadPosition{ThreadID: ids.New(), ChannelID: ch, LastMessageSeq: 5})
	}
	lost := chat.ThreadPosition{ThreadID: ids.New(), ChannelID: gone, LastMessageSeq: 5}
	threads = append(threads, lost, chat.ThreadPosition{ThreadID: ids.New(), ChannelID: gone, LastMessageSeq: 5})
	own, inMuted := threads[0].ThreadID, threads[1].ThreadID
	until := time.Date(2026, 11, 12, 10, 0, 0, 0, time.UTC)
	w.store.levelRows = []app.PreferenceRow{
		{UserID: alex, Preference: notify.Preference{Target: notify.Target{Kind: "thread", ID: own}, Level: "mute", MutedUntil: until}},
		{UserID: alex, Preference: notify.Preference{Target: notify.Target{Kind: "channel", ID: muted}, Level: "mentions"}},
		{UserID: alex, Preference: notify.Preference{Target: notify.Target{Kind: "space", ID: space}, Level: "all"}},
		{UserID: ids.New(), Preference: notify.Preference{Target: notify.Target{Kind: "channel", ID: text}, Level: "mute"}},
	}
	w.settings.vals = map[ids.ID]map[string]json.RawMessage{alex: {"notify.default_level": json.RawMessage(`"all"`)}}
	facts, err := w.svc.Facts().LevelFactsOfUser(context.Background(), alex, threads)
	if err != nil {
		t.Fatal(err)
	}
	if w.paths.calls != 4 || w.paths.threadCalls != 0 || w.settings.calls != 1 {
		t.Fatalf("calls: path %d (of a thread %d), settings %d; want one path read per channel (4), none of a thread, one settings read",
			w.paths.calls, w.paths.threadCalls, w.settings.calls)
	}
	if len(facts) != 3000 {
		t.Fatalf("%d facts, want 3000: the threads of the missing channel are left out", len(facts))
	}
	if _, ok := facts[lost.ThreadID]; ok {
		t.Fatal("a thread of a channel that no longer exists has facts")
	}
	f := facts[own]
	if f.Thread == nil || f.Thread.Level != domain.LevelMute || !f.Thread.MutedUntil.Equal(until) || f.Channel != nil ||
		f.Space == nil || f.Space.Level != domain.LevelAll {
		t.Fatalf("facts of the thread with an own row: %+v", f)
	}
	if f.ChannelDefault != domain.LevelAll || f.DirectMessage || f.PersonalDefault != domain.LevelAll || f.InstanceDefault != domain.LevelMentions {
		t.Fatalf("shared facts of a thread in the text channel: %+v", f)
	}
	if f := facts[inMuted]; f.Thread != nil || f.Channel == nil || f.Channel.Level != domain.LevelMentions || f.Space == nil ||
		f.ChannelDefault != domain.LevelMute || f.DirectMessage {
		t.Fatalf("facts of a thread in the channel with an own row: %+v", f)
	}
	if f := facts[threads[2].ThreadID]; !f.DirectMessage || f.Thread != nil || f.Channel != nil || f.Space != nil || f.ChannelDefault != "" {
		t.Fatalf("facts of a thread in the DM channel: %+v", f)
	}
	// No thread, no read at all.
	before := w.paths.calls
	if facts, err := w.svc.Facts().LevelFactsOfUser(context.Background(), alex, nil); err != nil || len(facts) != 0 || w.paths.calls != before {
		t.Fatalf("no threads: %d facts, %v, %d path reads", len(facts), err, w.paths.calls-before)
	}
}

// TestQuietRules_ZoneSources (13 section 3.4): a rule's zone wins, NULL takes the person's zone,
// an unloadable stored name becomes UTC with one Warn line without the name; each zone loads once
// per call.
func TestQuietRules_ZoneSources(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	alex, sam := ids.New(), ids.New()
	w.users.m[alex] = identity.UserView{TimeZone: "Europe/Berlin"}
	w.users.m[sam] = identity.UserView{TimeZone: "America/New_York"}
	w.store.rules[alex] = []notify.QuietHoursRule{
		{ID: ids.New(), Weekdays: 31, StartLocal: "22:00", EndLocal: "07:00", TimeZone: "Asia/Tokyo", Enabled: true},
		{ID: ids.New(), Weekdays: 96, StartLocal: "00:00", EndLocal: "00:00", Enabled: true},
		{ID: ids.New(), Weekdays: 1, StartLocal: "12:00", EndLocal: "13:00", TimeZone: "Mars/Olympus", Enabled: false},
	}
	w.store.rules[sam] = []notify.QuietHoursRule{{ID: ids.New(), Weekdays: 1, StartLocal: "08:00", EndLocal: "09:00", Enabled: true}}
	got, err := w.svc.Facts().QuietRules(context.Background(), []ids.ID{alex, sam, ids.New()})
	if err != nil {
		t.Fatal(err)
	}
	a := got[alex]
	if len(a) != 3 || a[0].Location.String() != "Asia/Tokyo" || a[1].Location.String() != "Europe/Berlin" || a[2].Location != time.UTC {
		t.Fatalf("alex: %+v", a)
	}
	if a[0].Weekdays != 31 || a[0].Start != 22*60 || a[0].End != 7*60 || !a[0].Enabled || a[2].Enabled {
		t.Fatalf("rule fields: %+v", a[0])
	}
	if s := got[sam]; len(s) != 1 || s[0].Location.String() != "America/New_York" {
		t.Fatalf("sam: %+v", s)
	}
	if w.users.calls != 1 {
		t.Fatalf("identity read %d times, want once", w.users.calls)
	}
	logs := w.log.String()
	if strings.Count(logs, "level=WARN") != 1 || strings.Contains(logs, "Mars") || strings.Contains(logs, alex.String()) {
		t.Fatalf("log: %s", logs)
	}
	on, err := domain.InQuietHours(got[alex], time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)) // Saturday in Berlin
	if err != nil || !on {
		t.Fatalf("saturday all day: %v %v", on, err)
	}
}

// TestOnChannelArchived_DeletesAndEmitsPerRow (R-DM-16): the channel and its threads go to the
// store; one event per returned row in its order, deleted, the version plus one, audience the
// row's person, the actor of the unit.
func TestOnChannelArchived_DeletesAndEmitsPerRow(t *testing.T) {
	t.Parallel()
	store, paths := &fakeStore{}, &fakePaths{threads: map[ids.ID][]ids.ID{}}
	ch, t1, t2, alex, sam, owner := ids.New(), ids.New(), ids.New(), ids.New(), ids.New(), ids.New()
	paths.threads[ch] = []ids.ID{t1, t2}
	store.deleted = []app.PreferenceRow{
		{UserID: alex, Preference: notify.Preference{Target: notify.Target{Kind: "channel", ID: ch}, Level: "all", Version: 2}},
		{UserID: alex, Preference: notify.Preference{Target: notify.Target{Kind: "thread", ID: t1}, Level: "mute", Version: 1}},
		{UserID: sam, Preference: notify.Preference{Target: notify.Target{Kind: "thread", ID: t2}, Level: "mentions", Version: 3}},
	}
	subs, ev := app.NewSubscribers(store), &appender{}
	if err := subs.OnChannelArchived(context.Background(), chat.ChannelArchived{ChannelID: ch}); errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("unbound: %v", err)
	}
	if err := subs.Bind(ev, paths); err != nil {
		t.Fatal(err)
	}
	ctx := actor.WithActor(context.Background(), actor.Actor{ID: owner, Kind: actor.KindUser})
	if err := subs.OnChannelArchived(ctx, chat.ChannelArchived{ChannelID: ch, SpaceID: ids.New()}); err != nil {
		t.Fatal(err)
	}
	if a := store.delArgs; !a.space.IsZero() || !slices.Equal(a.channels, []ids.ID{ch}) || !slices.Equal(a.thread, []ids.ID{t1, t2}) {
		t.Fatalf("targets %+v", a)
	}
	checkCleanupEvents(t, ev.events, store.deleted, owner)
}

// TestOnSpaceArchived_CollectsChannelsAndThreads: the space, every channel of it and every thread
// of those channels.
func TestOnSpaceArchived_CollectsChannelsAndThreads(t *testing.T) {
	t.Parallel()
	store, paths := &fakeStore{}, &fakePaths{channels: map[ids.ID][]ids.ID{}, threads: map[ids.ID][]ids.ID{}}
	sp, c1, c2, t1, t2, alex := ids.New(), ids.New(), ids.New(), ids.New(), ids.New(), ids.New()
	paths.channels[sp] = []ids.ID{c1, c2}
	paths.threads[c1], paths.threads[c2] = []ids.ID{t1}, []ids.ID{t2}
	store.deleted = []app.PreferenceRow{{UserID: alex, Preference: notify.Preference{Target: notify.Target{Kind: "space", ID: sp}, Level: "all", Version: 1}}}
	subs, ev := app.NewSubscribers(store), &appender{}
	if err := subs.Bind(ev, paths); err != nil {
		t.Fatal(err)
	}
	if err := subs.OnSpaceArchived(context.Background(), chat.SpaceArchived{SpaceID: sp}); err != nil {
		t.Fatal(err)
	}
	if a := store.delArgs; a.space != sp || !slices.Equal(a.channels, []ids.ID{c1, c2}) || !slices.Equal(a.thread, []ids.ID{t1, t2}) {
		t.Fatalf("targets %+v", a)
	}
	checkCleanupEvents(t, ev.events, store.deleted, ids.ID{})
}

// checkCleanupEvents checks one notification_pref.updated per row in order.
func checkCleanupEvents(t *testing.T, events []eventlog.Event, rows []app.PreferenceRow, actorID ids.ID) {
	t.Helper()
	if len(events) != len(rows) {
		t.Fatalf("%d events, want %d", len(events), len(rows))
	}
	for i, e := range events {
		r := rows[i]
		var p struct {
			Preference struct {
				TargetKind string `json:"target_kind"`
				TargetID   string `json:"target_id"`
				Version    int64  `json:"version"`
			} `json:"preference"`
			Deleted bool `json:"deleted"`
		}
		if err := json.Unmarshal(e.Payload, &p); err != nil {
			t.Fatal(err)
		}
		if e.Type != "notification_pref.updated" || e.AudiencePrincipalID != r.UserID || e.ResourceKind != "user" || e.ResourceID != r.UserID ||
			!p.Deleted || p.Preference.Version != r.Version+1 || p.Preference.TargetID != r.Target.ID.String() || e.ActorPrincipalID != actorID {
			t.Fatalf("event %d: %+v %s", i, e, e.Payload)
		}
		if actorID.IsZero() && e.ActorKind != eventlog.ActorSystem {
			t.Fatalf("event %d without a request actor: %s", i, e.ActorKind)
		}
	}
}
