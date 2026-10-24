// SPDX-License-Identifier: Apache-2.0

package leak

import (
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/test/harness"
)

// p5Config is the configuration of an AZ-P5 server: raised rate limits (23 R-QA-11) and an expiry
// sweep that does not run during a test, so the reports of expired grants (access.changed) never
// land between two collections of property 2; expiry itself is checked on every call (07 8.3).
func p5Config() map[string]string {
	cfg := harness.RaisedRateLimits()
	cfg["authz.expiry_sweep_interval"] = "24h"
	// Few candidates, so noise in unreadable channels would flip candidates_capped of a search
	// without the rights filter (M3-14).
	cfg["search.max_candidates"] = "3"
	return cfg
}

// newRandomWorld starts a server and builds the world of seed on it.
func newRandomWorld(t *testing.T, seed uint64) *RandomWorld {
	t.Helper()
	t.Logf("AZ-P5 seed %d (replay: %s=%d)", seed, SeedEnv, seed)
	s := harness.Start(t, harness.Options{EgressDenyAll: true, Config: p5Config()})
	return BuildWorld(t, s, NewPlan(seed))
}

// seeds returns the seeds of this run or fails t.
func seeds(t *testing.T) []uint64 {
	t.Helper()
	s, err := Seeds()
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// truth is what the observed principal may read, from per-item reads (07 "Query pattern" rule 3:
// authorize is the truth): kind and visibility per known ID.
type truth struct {
	kind    map[string]string // id -> space, channel, thread, agent, message
	visible map[string]bool
	channel map[string]string // message or thread id -> channel id
}

// readTruth reads every space, channel, thread and agent of w as c (getSpace, getChannel,
// getThread, getAgent): 200 is visible, 404 is not; anything else fails t.
func readTruth(t *testing.T, w *RandomWorld, c *harness.Client) truth {
	t.Helper()
	tr := truth{kind: map[string]string{}, visible: map[string]bool{}, channel: map[string]string{}}
	probe := func(kind, path, id string) {
		r := c.Do(t.Context(), harness.Request{Method: http.MethodGet, Path: path + id})
		switch r.Status {
		case http.StatusOK:
			tr.visible[id] = true
		case http.StatusNotFound:
		default:
			t.Fatalf("truth %s %s: %d %s", kind, id, r.Status, r.Body)
		}
		tr.kind[id] = kind
	}
	for _, id := range w.Spaces {
		probe("space", "/spaces/", id)
	}
	for _, id := range w.AllChannels() {
		probe("channel", "/channels/", id)
	}
	for id, ch := range w.Threads {
		probe("thread", "/threads/", id)
		tr.channel[id] = ch
	}
	for _, a := range []*harness.Client{w.Agent, w.Other} {
		probe("agent", "/agents/", a.PrincipalID)
	}
	for _, m := range w.Messages {
		tr.kind[m.ID] = "message"
		tr.channel[m.ID] = m.Channel
		tr.visible[m.ID] = tr.visible[m.Channel]
	}
	// A thread is visible exactly when its channel is (no thread-level read in M1).
	for id, ch := range tr.channel {
		if tr.kind[id] == "thread" && tr.visible[id] != tr.visible[ch] {
			t.Errorf("truth: thread %s visible %v, its channel %v", id, tr.visible[id], tr.visible[ch])
		}
	}
	return tr
}

// surface is one answer: status and decoded body.
type surface struct {
	Status int
	Body   any
}

// collect queries every surface of AZ-P5 design note 3 as c: getSnapshot, the gateway ready
// snapshot, listSpaces, listSpaceChannels per visible space, listChannelThreads per visible
// channel, listAgents, getMySidebar, listMyMentions, listMyReadStates, listMyDrafts and
// listEvents from cursor 0 to the head (all pages joined).
func collect(t *testing.T, w *RandomWorld, c *harness.Client, tr truth) map[string]surface {
	t.Helper()
	out := map[string]surface{}
	getJSON := func(name, path string) {
		r := c.Do(t.Context(), harness.Request{Method: http.MethodGet, Path: path})
		var b any
		if len(r.Body) > 0 {
			if err := json.Unmarshal(r.Body, &b); err != nil {
				t.Fatalf("%s: %d %s", name, r.Status, r.Body)
			}
		}
		out[name] = surface{Status: r.Status, Body: b}
	}
	getJSON("getSnapshot", "/sync/snapshot")
	g, err := c.Gateway(t.Context())
	if err != nil {
		t.Fatalf("gateway: %v", err)
	}
	raw, err := json.Marshal(g.Ready.Snapshot)
	g.Close()
	if err != nil {
		t.Fatal(err)
	}
	var ready any
	if err := json.Unmarshal(raw, &ready); err != nil {
		t.Fatal(err)
	}
	out["gateway.ready"] = surface{Status: http.StatusOK, Body: ready}
	getJSON("listSpaces", "/spaces")
	for _, id := range w.Spaces {
		if tr.visible[id] {
			getJSON("listSpaceChannels/"+id, "/spaces/"+id+"/channels")
		}
	}
	for _, id := range w.AllChannels() {
		if tr.visible[id] {
			getJSON("listChannelThreads/"+id, "/channels/"+id+"/threads?limit=100")
		}
	}
	getJSON("listAgents", "/agents?limit=100")
	getJSON("getMySidebar", "/me/sidebar")
	getJSON("listMyMentions", "/me/mentions?limit=100")
	for _, id := range w.AllChannels() {
		getJSON("listMyReadStates/"+id, "/me/read-states?channel_id="+id)
	}
	getJSON("listMyDrafts", "/me/drafts")
	for _, typ := range []string{"messages", "threads", "files"} {
		term := "hello"
		if typ == "threads" {
			term = "topic"
		}
		name := "search/" + typ
		getJSON(name, "/search?type="+typ+"&q="+term+"&sort=newest&limit=50")
		if m, ok := out[name].Body.(map[string]any); ok {
			_, has := m["next_cursor"]
			m["has_next_cursor"] = has
		}
	}
	getJSON("searchThreadTitles", "/search/thread-titles?q=topic&limit=50")
	out["listEvents"] = events(t, c)
	return out
}

// events reads listEvents from cursor 0 until next_cursor reaches head_seq and returns the
// events of every page with the status of the last page.
func events(t *testing.T, c *harness.Client) surface {
	t.Helper()
	var all []any
	after := int64(0)
	for range 1000 {
		r := c.Do(t.Context(), harness.Request{Method: http.MethodGet, Path: "/events?limit=500&after=" + strconv.FormatInt(after, 10)})
		if r.Status != http.StatusOK {
			return surface{Status: r.Status, Body: string(r.Body)}
		}
		var page struct {
			Events     []any `json:"events"`
			NextCursor int64 `json:"next_cursor"`
			HeadSeq    int64 `json:"head_seq"`
		}
		r.JSON(t, &page)
		all = append(all, page.Events...)
		if page.NextCursor >= page.HeadSeq || page.NextCursor <= after {
			return surface{Status: http.StatusOK, Body: map[string]any{"events": all}}
		}
		after = page.NextCursor
	}
	t.Fatal("listEvents did not reach the head")
	return surface{}
}

// strs calls f with the JSON path and value of every string of v, recursively.
func strs(v any, f func(path, s string), path string) {
	switch x := v.(type) {
	case map[string]any:
		for k, y := range x {
			strs(y, f, path+"/"+k)
		}
	case []any:
		for i, y := range x {
			strs(y, f, path+"/"+strconv.Itoa(i))
		}
	case string:
		f(path, x)
	}
}

// hasKey reports the paths of every member named key in v.
func hasKey(v any, key, path string) []string {
	var out []string
	switch x := v.(type) {
	case map[string]any:
		for k, y := range x {
			if k == key {
				out = append(out, path+"/"+k)
			}
			out = append(out, hasKey(y, key, path+"/"+k)...)
		}
	case []any:
		for i, y := range x {
			out = append(out, hasKey(y, key, path+"/"+strconv.Itoa(i))...)
		}
	}
	return out
}

// checkProperty1 holds AZ-P5 property 1 for the observed principal o of w.
func checkProperty1(t *testing.T, w *RandomWorld, o Observed) {
	t.Helper()
	c := w.Principal(o)
	tr := readTruth(t, w, c)
	got := collect(t, w, c, tr)
	for name, s := range got {
		// Every space, channel, thread, agent and message the surface names is visible.
		if name == "listEvents" {
			continue
		}
		strs(s.Body, func(path, v string) {
			if k, ok := tr.kind[v]; ok && !tr.visible[v] && !operatorRef(k, path) {
				t.Errorf("%s: %s%s names the %s %s, which the %s cannot read", name, name, path, k, v, o)
			}
		}, "")
		for _, p := range hasKey(s.Body, "total", "") {
			t.Errorf("%s: a total at %s (06 Pagination)", name, p)
		}
	}
	checkEvents(t, got["listEvents"], tr, o)
	for _, name := range []string{"getSnapshot", "gateway.ready"} {
		checkUnread(t, w, o, tr, name, got[name])
	}
	if o == ObservedHuman {
		checkPersonal(t, w, tr, got)
	}
}

// operatorRef reports an agent ID in an operator member of a space or channel (the default or
// effective operator, also in the payload of an event of that time): header data of a resource
// the principal reads (05 space.read, channel.read), not the agent card that agent.read protects
// (ruling M1-70/5). Agents as items (listAgents, the agents of the snapshot) stay checked.
func operatorRef(kind, path string) bool {
	return kind == "agent" && strings.Contains(path, "operator")
}

// checkEvents: every event of listEvents has a visible scope or audience self, and an event
// that is not personal names nothing the principal cannot read.
func checkEvents(t *testing.T, s surface, tr truth, o Observed) {
	t.Helper()
	if s.Status != http.StatusOK {
		t.Fatalf("listEvents: %d %v", s.Status, s.Body)
	}
	evs, _ := s.Body.(map[string]any)["events"].([]any)
	for i, e := range evs {
		ev, _ := e.(map[string]any)
		if ev["audience"] == "self" {
			continue
		}
		scope, _ := ev["scope"].(map[string]any)
		for _, k := range []string{"thread_id", "channel_id", "space_id"} {
			if id, ok := scope[k].(string); ok && !tr.visible[id] {
				t.Errorf("listEvents: event %d (%v) has the scope %s %s, which the %s cannot read", i, ev["type"], k, id, o)
			}
		}
		strs(ev, func(path, v string) {
			if k, ok := tr.kind[v]; ok && !tr.visible[v] && !operatorRef(k, path) {
				t.Errorf("listEvents: event %d (%v) names the %s %s at %s, which the %s cannot read", i, ev["type"], k, v, path, o)
			}
		}, "")
	}
}

// ownerSeq returns last_message_seq of thread as the owner reads it.
func ownerSeq(t *testing.T, w *RandomWorld, thread string) int64 {
	t.Helper()
	var th struct {
		LastMessageSeq int64 `json:"last_message_seq"`
	}
	harness.MustDo(t, w.Owner, harness.Request{Method: http.MethodGet, Path: "/threads/" + thread}, http.StatusOK).JSON(t, &th)
	return th.LastMessageSeq
}

// mentionsAfter counts the messages of thread that mention the person after seq.
func mentionsAfter(w *RandomWorld, thread string, seq int64) int {
	n := 0
	for _, m := range w.Messages {
		if m.Thread == thread && m.MentionsHuman && m.Seq > seq {
			n++
		}
	}
	return n
}

// checkUnread: the unread threads of a snapshot are exactly the visible threads with messages
// after the read position, with the model's counts (03 read_states: messages after last_read_seq;
// mentions of the person in visible channels only).
func checkUnread(t *testing.T, w *RandomWorld, o Observed, tr truth, name string, s surface) {
	t.Helper()
	if s.Status != http.StatusOK {
		t.Fatalf("%s: %d %v", name, s.Status, s.Body)
	}
	var snap struct {
		UnreadThreads []struct {
			ThreadID     string `json:"thread_id"`
			UnreadCount  int64  `json:"unread_count"`
			MentionCount int    `json:"mention_count"`
			LastReadSeq  int64  `json:"last_read_seq"`
		} `json:"unread_threads"`
	}
	raw, _ := json.Marshal(s.Body)
	if err := json.Unmarshal(raw, &snap); err != nil {
		t.Fatal(err)
	}
	if o == ObservedAgent {
		// Unread threads are personal data of a person; an agent has none (sync snapshot).
		if len(snap.UnreadThreads) != 0 {
			t.Errorf("%s: an agent has unread threads %+v", name, snap.UnreadThreads)
		}
		return
	}
	listed := map[string]bool{}
	for _, u := range snap.UnreadThreads {
		listed[u.ThreadID] = true
		if !tr.visible[u.ThreadID] {
			continue // reported by the reference check
		}
		read := w.ReadStates[u.ThreadID]
		want := ownerSeq(t, w, u.ThreadID) - read
		if u.UnreadCount != want || u.LastReadSeq != read {
			t.Errorf("%s: thread %s unread %d last_read %d, want %d and %d", name, u.ThreadID, u.UnreadCount, u.LastReadSeq, want, read)
		}
		if wantMentions := mentionsAfter(w, u.ThreadID, read); u.MentionCount != wantMentions {
			t.Errorf("%s: thread %s mention_count %d, want %d", name, u.ThreadID, u.MentionCount, wantMentions)
		}
	}
	for th := range w.Threads {
		if !tr.visible[th] || listed[th] {
			continue
		}
		if n := ownerSeq(t, w, th) - w.ReadStates[th]; n > 0 {
			t.Errorf("%s: visible thread %s with %d unread messages is missing", name, th, n)
		}
	}
}

// checkPersonal: mentions, read states and drafts of the person are exactly the ones in visible
// channels.
func checkPersonal(t *testing.T, w *RandomWorld, tr truth, got map[string]surface) {
	t.Helper()
	ids := func(name, member string) []string {
		s := got[name]
		if s.Status != http.StatusOK {
			t.Fatalf("%s: %d %v", name, s.Status, s.Body)
		}
		var out []string
		items, _ := s.Body.(map[string]any)["items"].([]any)
		for _, it := range items {
			v := it.(map[string]any)
			for _, part := range strings.Split(member, ".") {
				if m, ok := v[part].(map[string]any); ok {
					v = m
					continue
				}
				if id, ok := v[part].(string); ok {
					out = append(out, id)
				}
			}
		}
		slices.Sort(out)
		return out
	}
	var wantMentions []string
	for _, m := range w.Messages {
		if m.MentionsHuman && tr.visible[m.Channel] {
			wantMentions = append(wantMentions, m.ID)
		}
	}
	slices.Sort(wantMentions)
	if got := ids("listMyMentions", "message.id"); !slices.Equal(got, wantMentions) {
		t.Errorf("listMyMentions %v, want %v", got, wantMentions)
	}
	var wantReads []string
	for th, seq := range w.ReadStates {
		// A position of 0 is the default and stores no row.
		if tr.visible[th] && seq > 0 {
			wantReads = append(wantReads, th)
		}
	}
	slices.Sort(wantReads)
	var gotReads []string
	for _, ch := range w.AllChannels() {
		name := "listMyReadStates/" + ch
		if !tr.visible[ch] {
			// An unreadable channel never shows read states (it answers like an unknown one).
			if s := got[name]; s.Status == http.StatusOK && len(ids(name, "thread_id")) != 0 {
				t.Errorf("%s of an unreadable channel: %v", name, s.Body)
			}
			continue
		}
		gotReads = append(gotReads, ids(name, "thread_id")...)
	}
	slices.Sort(gotReads)
	if !slices.Equal(gotReads, wantReads) {
		t.Errorf("listMyReadStates threads %v, want %v", gotReads, wantReads)
	}
	var wantDrafts []string
	for _, ch := range w.Drafts {
		if tr.visible[ch] {
			wantDrafts = append(wantDrafts, ch)
		}
	}
	slices.Sort(wantDrafts)
	if got := ids("listMyDrafts", "channel_id"); !slices.Equal(got, wantDrafts) {
		t.Errorf("listMyDrafts channels %v, want %v", got, wantDrafts)
	}
}

// TestAZ_P5_ListsAndCountersMatchAuthorize (AZ-P5 property 1): in random worlds, every space,
// channel, thread, agent and message a surface names is readable by the observed person and the
// observed agent by the truth of per-item reads, every event has a visible scope or audience self,
// unread and mention counts equal the model, and no surface has a total.
func TestAZ_P5_ListsAndCountersMatchAuthorize(t *testing.T) {
	t.Parallel()
	for _, seed := range seeds(t) {
		t.Run("AZ-P5/seed-"+strconv.FormatUint(seed, 10), func(t *testing.T) {
			t.Parallel()
			w := newRandomWorld(t, seed)
			for _, o := range []Observed{ObservedHuman, ObservedAgent} {
				t.Run(o.String(), func(t *testing.T) { checkProperty1(t, w, o) })
			}
		})
	}
}

// TestAZ_P5_NoiseInvariance (AZ-P5 property 2): noise only in channels the observed principal
// cannot read (threads, messages, mentions of the person, reactions, pins) leaves every surface
// equal after dropping head_seq, next_cursor and cursor frames.
func TestAZ_P5_NoiseInvariance(t *testing.T) {
	t.Parallel()
	for _, seed := range seeds(t) {
		t.Run("AZ-P5/seed-"+strconv.FormatUint(seed, 10), func(t *testing.T) {
			t.Parallel()
			w := newRandomWorld(t, seed)
			for _, o := range []Observed{ObservedHuman, ObservedAgent} {
				t.Run(o.String(), func(t *testing.T) {
					c := w.Principal(o)
					tr := readTruth(t, w, c)
					before := collect(t, w, c, tr)
					dark := 0
					for _, ch := range w.AllChannels() {
						if !tr.visible[ch] {
							addNoise(t, w, ch)
							dark++
						}
					}
					if dark == 0 {
						t.Logf("the %s reads every channel of seed %d; no noise possible", o, seed)
						return
					}
					after := collect(t, w, c, tr)
					for _, name := range slices.Sorted(maps.Keys(before)) {
						a, b := strip(before[name]), strip(after[name])
						if !reflect.DeepEqual(a, b) {
							ja, _ := json.Marshal(a)
							jb, _ := json.Marshal(b)
							t.Errorf("%s changed by noise in %d unreadable channels:\nbefore %s\nafter  %s", name, dark, ja, jb)
						}
					}
				})
			}
		})
	}
}

// addNoise writes, as the owner, into channel ch: a new thread (a post in a forum) and messages
// in every existing thread, each mentioning the person, a reaction and a pin on the first new
// message.
func addNoise(t *testing.T, w *RandomWorld, ch string) {
	t.Helper()
	w.buildChannel(t, ch, ChannelPlan{Forum: w.Forum[ch], Threads: [][]bool{{true}}})
	var first string
	for _, th := range w.threadsOf(ch) {
		for range worldNoiseMessages {
			id := w.Post(t, th, true)
			if first == "" {
				first = id
			}
		}
	}
	harness.MustDo(t, w.Owner, harness.Request{Method: http.MethodPut, Path: "/messages/" + first + "/reactions/" + "%F0%9F%91%8D"}, http.StatusNoContent)
	harness.MustDo(t, w.Owner, harness.Request{Method: http.MethodPut, Path: "/messages/" + first + "/pin"}, http.StatusNoContent)
}

// strip returns s without head_seq, next_cursor and cursor frames (04 "Cursor": the checked
// cursor advances by design) and without the correlation_id of a problem body (per request).
func strip(s surface) surface {
	return surface{Status: s.Status, Body: stripValue(s.Body)}
}

// stripValue removes the members head_seq, next_cursor and correlation_id and the elements of
// type cursor.
func stripValue(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := map[string]any{}
		for k, y := range x {
			if k == "head_seq" || k == "next_cursor" || k == "correlation_id" {
				continue
			}
			out[k] = stripValue(y)
		}
		return out
	case []any:
		out := []any{}
		for _, y := range x {
			if m, ok := y.(map[string]any); ok && m["type"] == "cursor" {
				continue
			}
			out = append(out, stripValue(y))
		}
		return out
	default:
		return v
	}
}

// TestWorldPlanReproducible: a seed always gives the same plan, and different seeds give
// different plans; Seeds honours LEAK_SEED.
func TestWorldPlanReproducible(t *testing.T) {
	a, b := NewPlan(42), NewPlan(42)
	if !reflect.DeepEqual(a, b) {
		t.Fatal("seed 42 gave two plans")
	}
	differ := 0
	for s := range uint64(20) {
		if !reflect.DeepEqual(NewPlan(s).Spaces, NewPlan(s+100).Spaces) {
			differ++
		}
	}
	if differ == 0 {
		t.Fatal("every seed gave the same plan")
	}
	for s := range uint64(50) {
		p := NewPlan(s)
		if len(p.Spaces) != worldSpaces || p.Lost.Kind != "channel" {
			t.Fatalf("plan %d: %+v", s, p)
		}
		for _, g := range p.Grants {
			if g.Subject == ObservedAgent && !g.ToGroup && g.Level == "trigger" {
				t.Fatalf("plan %d: agent with trigger", s)
			}
		}
	}
	t.Setenv(SeedEnv, "7")
	got, err := Seeds()
	if err != nil || len(got) != WorldsPerRun || got[0] != 7 {
		t.Fatalf("seeds %v %v", got, err)
	}
	t.Setenv(SeedEnv, "x")
	if _, err := Seeds(); err == nil {
		t.Fatal("a bad LEAK_SEED gave seeds")
	}
	if fmt.Sprint(ObservedHuman, ObservedAgent) != "human agent" {
		t.Fatal("names")
	}
}
