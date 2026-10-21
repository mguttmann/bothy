// SPDX-License-Identifier: Apache-2.0

package leak

import (
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"net/http"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/test/harness"
)

// The sizes of a random world of AZ-P5 and the number of worlds per test run (design note 1).
const (
	// WorldsPerRun is the number of seeds each AZ-P5 test checks.
	WorldsPerRun = 2
	// worldSpaces is the number of spaces of a world.
	worldSpaces = 3
	// worldChannelsMax is the largest number of channels per space (at least one).
	worldChannelsMax = 3
	// worldThreadsMax is the largest number of extra threads per channel (a text channel also
	// has its main thread).
	worldThreadsMax = 2
	// worldMessagesMax is the largest number of messages per thread (at least one).
	worldMessagesMax = 3
	// worldGrantsMax is the largest number of grants per observed principal and per group.
	worldGrantsMax = 4
	// worldGroups is the number of groups the observed person may belong to.
	worldGroups = 2
	// worldNoiseMessages is the number of noise messages per channel the principal cannot read.
	worldNoiseMessages = 2
	// expiryLead is how far ahead of the harness clock an expiring grant or membership ends; the
	// build writes what needs access before it and waits for it after.
	expiryLead = 4 * time.Second
)

// SeedEnv is the environment variable that fixes the seed of AZ-P5, so a failure can be
// replayed: LEAK_SEED=<seed> go test -run TestAZ_P5 ./test/leak/
const SeedEnv = "LEAK_SEED"

// Seeds returns WorldsPerRun seeds: from LEAK_SEED (the first, the following ones added by one)
// or from the current time. Errors: LEAK_SEED is no unsigned integer.
func Seeds() ([]uint64, error) {
	base := uint64(time.Now().UnixNano()) // #nosec G115 -- a positive time as seed
	if v := os.Getenv(SeedEnv); v != "" { //nolint:forbidigo // test input for replaying a seed, not server configuration
		s, err := strconv.ParseUint(v, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("%s=%q: %w", SeedEnv, v, err)
		}
		base = s
	}
	out := make([]uint64, WorldsPerRun)
	for i := range out {
		out[i] = base + uint64(i) // #nosec G115 -- a small index
	}
	return out, nil
}

// Observed names the principal whose view a property checks.
type Observed int

// The observed principals of a world: the person P (no owner) and the agent P with the operator
// rule of 07 "Agents as principals".
const (
	ObservedHuman Observed = iota
	ObservedAgent
)

// String returns human or agent.
func (o Observed) String() string {
	if o == ObservedAgent {
		return "agent"
	}
	return "human"
}

// Target is a grant or operator target of a plan: the instance, a space or a channel by index.
type Target struct {
	// Kind is instance, space or channel.
	Kind string
	// Space and Channel are indices into Plan.Spaces and its channels.
	Space, Channel int
}

// GrantPlan is one grant of a plan. Subject is the observed person, the observed agent or a group
// (Group is then its index). Expires makes it end expiryLead after its creation, before the checks.
type GrantPlan struct {
	Subject Observed
	Group   int
	ToGroup bool
	Target  Target
	Level   string
	Expires bool
}

// OperatorPlan makes the observed agent (Observed true) or the other agent the operator of a
// space or channel.
type OperatorPlan struct {
	Target   Target
	Observed bool
}

// ChannelPlan is one channel: forum or text, with threads of messages; Mentions marks the
// messages that mention the observed person; Draft asks the person for a draft there.
type ChannelPlan struct {
	Forum    bool
	Threads  [][]bool
	Draft    bool
	ReadUpTo []int
}

// Plan is a random world as data, derived from its seed only: the same seed gives the same plan
// (TestWorldPlanReproducible).
type Plan struct {
	Seed   uint64
	Spaces [][]ChannelPlan
	Grants []GrantPlan
	// Members marks per group whether the person is a member; Expiring whether that membership
	// ends expiryLead after it starts.
	Members, Expiring []bool
	Operators         []OperatorPlan
	// Lost is a channel on which the person gets write, writes a draft and read states, and loses
	// the grant again before the checks (revokeGrant).
	Lost Target
}

// NewPlan derives the plan of seed.
func NewPlan(seed uint64) *Plan {
	r := rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15)) // #nosec G404 -- a reproducible test world, no secret
	p := &Plan{Seed: seed}
	for range worldSpaces {
		var chans []ChannelPlan
		for range 1 + r.IntN(worldChannelsMax) {
			c := ChannelPlan{Forum: r.IntN(4) == 0, Draft: r.IntN(2) == 0}
			n := r.IntN(worldThreadsMax + 1)
			if c.Forum && n == 0 {
				n = 1
			}
			for range n {
				msgs := make([]bool, 1+r.IntN(worldMessagesMax))
				for i := range msgs {
					msgs[i] = r.IntN(3) == 0
				}
				c.Threads = append(c.Threads, msgs)
			}
			c.ReadUpTo = make([]int, len(c.Threads)+1) // index 0: the main thread of a text channel
			for i := range c.ReadUpTo {
				c.ReadUpTo[i] = r.IntN(worldMessagesMax+2) - 1
			}
			chans = append(chans, c)
		}
		p.Spaces = append(p.Spaces, chans)
	}
	target := func() Target {
		s := r.IntN(len(p.Spaces))
		switch k := r.IntN(10); {
		case k == 0:
			return Target{Kind: "instance"}
		case k < 4:
			return Target{Kind: "space", Space: s}
		default:
			return Target{Kind: "channel", Space: s, Channel: r.IntN(len(p.Spaces[s]))}
		}
	}
	levels := []string{"read", "write", "trigger"}
	for _, subject := range []Observed{ObservedHuman, ObservedAgent} {
		for range r.IntN(worldGrantsMax + 1) {
			lv := levels[r.IntN(len(levels))]
			if subject == ObservedAgent && lv == "trigger" {
				lv = "write" // agents receive at most write (07 2.9 item 1)
			}
			p.Grants = append(p.Grants, GrantPlan{Subject: subject, Target: target(), Level: lv, Expires: r.IntN(3) == 0})
		}
	}
	for g := range worldGroups {
		p.Members = append(p.Members, r.IntN(3) != 0)
		p.Expiring = append(p.Expiring, r.IntN(3) == 0)
		for range r.IntN(3) {
			t := target()
			if t.Kind == "instance" {
				t = Target{Kind: "space", Space: r.IntN(len(p.Spaces))}
			}
			p.Grants = append(p.Grants, GrantPlan{ToGroup: true, Group: g, Target: t, Level: "read", Expires: r.IntN(4) == 0})
		}
	}
	for range 1 + r.IntN(3) {
		t := target()
		if t.Kind == "instance" {
			t.Kind = "space"
		}
		p.Operators = append(p.Operators, OperatorPlan{Target: t, Observed: r.IntN(2) == 0})
	}
	s := r.IntN(len(p.Spaces))
	p.Lost = Target{Kind: "channel", Space: s, Channel: r.IntN(len(p.Spaces[s]))}
	return p
}

// MessageRef is a message of a built world.
type MessageRef struct {
	ID, Thread, Channel string
	Seq                 int64
	MentionsHuman       bool
}

// RandomWorld is a Plan built on a harness server through the public API.
type RandomWorld struct {
	Plan *Plan
	// Owner builds the world; Human and Agent are the observed principals; Other is a second
	// agent that operates targets the plan gives it.
	Owner, Human, Agent, Other *harness.Client
	// Spaces holds the space IDs; Channels the channel IDs per space; Forum marks forum channels.
	Spaces   []string
	Channels [][]string
	Forum    map[string]bool
	// Threads maps a thread ID to its channel ID; Main maps a text channel to its main thread.
	Threads map[string]string
	Main    map[string]string
	// Messages are every message the build or the noise wrote.
	Messages []MessageRef
	// ReadStates are the positions the person set (thread -> last_read_seq), Drafts the channels
	// or threads with a draft of the person (ID -> channel).
	ReadStates map[string]int64
	Drafts     map[string]string

	humanHandle string
}

// Principal returns the client of o.
func (w *RandomWorld) Principal(o Observed) *harness.Client {
	if o == ObservedAgent {
		return w.Agent
	}
	return w.Human
}

// BuildWorld builds p on s: spaces and channels, the observed person and agent and the other
// agent, operators, groups with memberships, grants (the expiring ones end expiryLead after
// creation), threads and messages by the owner with mentions of the person, read states and
// drafts of the person, the lost channel; then it waits until every expiry has passed.
func BuildWorld(tb testing.TB, s *harness.Server, p *Plan) *RandomWorld {
	tb.Helper()
	tag := strconv.FormatUint(p.Seed%1_000_000, 36)
	w := &RandomWorld{
		Plan: p, Owner: s.Owner, Forum: map[string]bool{}, Threads: map[string]string{}, Main: map[string]string{},
		ReadStates: map[string]int64{}, Drafts: map[string]string{},
	}
	o := s.Owner
	w.humanHandle = "p" + tag
	w.Human = s.NewUser(tb, harness.UserSpec{Handle: w.humanHandle, DisplayName: "P " + tag, Email: "p" + tag + "@example.com"})
	w.Agent = s.NewAgent(tb, harness.AgentSpec{Handle: "pa" + tag, DisplayName: "PA " + tag})
	w.Other = s.NewAgent(tb, harness.AgentSpec{Handle: "oa" + tag, DisplayName: "OA " + tag})
	for si, chans := range p.Spaces {
		var sid struct {
			ID string `json:"id"`
		}
		harness.MustDo(tb, o, harness.Request{Method: http.MethodPost, Path: "/spaces", Body: map[string]any{"name": fmt.Sprintf("S%d %s", si, tag)}},
			http.StatusCreated).JSON(tb, &sid)
		w.Spaces = append(w.Spaces, sid.ID)
		var cids []string
		for ci, c := range chans {
			kind := "text"
			if c.Forum {
				kind = "forum"
			}
			var ch struct {
				ID           string `json:"id"`
				MainThreadID string `json:"main_thread_id"`
			}
			harness.MustDo(tb, o, harness.Request{Method: http.MethodPost, Path: "/spaces/" + sid.ID + "/channels",
				Body: map[string]any{"kind": kind, "name": fmt.Sprintf("c%d-%d", si, ci)}}, http.StatusCreated).JSON(tb, &ch)
			cids = append(cids, ch.ID)
			w.Forum[ch.ID] = c.Forum
			if ch.MainThreadID != "" {
				w.Main[ch.ID] = ch.MainThreadID
				w.Threads[ch.MainThreadID] = ch.ID
			}
		}
		w.Channels = append(w.Channels, cids)
	}
	for _, op := range p.Operators {
		agent := w.Other
		if op.Observed {
			agent = w.Agent
		}
		path := "/spaces/" + w.Spaces[op.Target.Space]
		if op.Target.Kind == "channel" {
			path = "/channels/" + w.Channels[op.Target.Space][op.Target.Channel]
		}
		setOperator(tb, o, path, agent.PrincipalID)
	}
	for si, chans := range p.Spaces {
		for ci, c := range chans {
			w.buildChannel(tb, w.Channels[si][ci], c)
		}
	}
	// Each expiry is taken at its own request: one moment for all of them failed on a loaded
	// runner, where the build of a world took longer than expiryLead and the server refused a
	// later grant whose expiry was already before its creation time.
	exp := &expiries{now: s.Clock().Now}
	groups := make([]string, len(p.Members))
	for g := range p.Members {
		groups[g] = idOf(tb, harness.MustDo(tb, o, harness.Request{Method: http.MethodPost, Path: "/groups",
			Body: map[string]any{"name": fmt.Sprintf("G%d %s", g, tag)}}, http.StatusCreated))
		if p.Members[g] {
			body := map[string]any{"user_id": w.Human.PrincipalID}
			if p.Expiring[g] {
				body["expires_at"] = stamp(exp.next())
			}
			harness.MustDo(tb, o, harness.Request{Method: http.MethodPost, Path: "/groups/" + groups[g] + "/members", Body: body}, http.StatusCreated)
		}
	}
	for _, g := range p.Grants {
		kind, subject := "user", w.Human.PrincipalID
		switch {
		case g.ToGroup:
			kind, subject = "group", groups[g.Group]
		case g.Subject == ObservedAgent:
			kind, subject = "agent", w.Agent.PrincipalID
		}
		w.grant(tb, kind, subject, g.Target, g.Level, g.Expires, exp)
	}
	// The person writes where it can read: before the expiring grants end and before the lost
	// channel is revoked, so drafts and read states exist in channels it no longer reads.
	lost := w.grant(tb, "user", w.Human.PrincipalID, p.Lost, "write", false, exp)
	w.writePersonal(tb)
	harness.MustDo(tb, o, harness.Request{Method: http.MethodPost, Path: "/grants/" + lost + "/revoke", Body: map[string]any{"reason": "leak test"}}, http.StatusOK)
	if d := time.Until(exp.last); d > 0 {
		time.Sleep(d + time.Second)
	}
	return w
}

// buildChannel writes the threads and messages of c into channel id as the owner: a text channel
// gets one message per planned thread in its main thread, each thread hangs on it; a forum gets
// posts.
func (w *RandomWorld) buildChannel(tb testing.TB, id string, c ChannelPlan) {
	tb.Helper()
	for _, msgs := range c.Threads {
		body := map[string]any{"title": "Topic", "first_message": w.messageBody(msgs[0])}
		if main := w.Main[id]; main != "" {
			body["parent_message_id"] = w.Post(tb, main, false)
		}
		var th struct {
			Thread struct {
				ID string `json:"id"`
			} `json:"thread"`
			FirstMessage struct {
				ID        string `json:"id"`
				ThreadSeq int64  `json:"thread_seq"`
			} `json:"first_message"`
		}
		harness.MustDo(tb, w.Owner, harness.Request{Method: http.MethodPost, Path: "/channels/" + id + "/threads", Body: body}, http.StatusCreated).JSON(tb, &th)
		w.Threads[th.Thread.ID] = id
		w.Messages = append(w.Messages, MessageRef{ID: th.FirstMessage.ID, Thread: th.Thread.ID, Channel: id, Seq: th.FirstMessage.ThreadSeq, MentionsHuman: msgs[0]})
		for _, m := range msgs[1:] {
			w.Post(tb, th.Thread.ID, m)
		}
	}
}

// messageBody is the createMessage body of a message, with a mention token of the person when
// mention (03 messages, content format).
func (w *RandomWorld) messageBody(mention bool) map[string]any {
	text := "Hello"
	if mention {
		text += " @" + w.humanHandle
	}
	return map[string]any{"client_msg_id": ids.New().String(), "content_md": text}
}

// Post writes a message of the owner into thread (with a mention of the person when mention) and
// returns its ID.
func (w *RandomWorld) Post(tb testing.TB, thread string, mention bool) string {
	tb.Helper()
	var m struct {
		ID        string `json:"id"`
		ThreadSeq int64  `json:"thread_seq"`
		ChannelID string `json:"channel_id"`
	}
	harness.MustDo(tb, w.Owner, harness.Request{Method: http.MethodPost, Path: "/threads/" + thread + "/messages", Body: w.messageBody(mention)},
		http.StatusCreated).JSON(tb, &m)
	w.Messages = append(w.Messages, MessageRef{ID: m.ID, Thread: thread, Channel: m.ChannelID, Seq: m.ThreadSeq, MentionsHuman: mention})
	return m.ID
}

// expiries hands out the expiries of a world build: each ends expiryLead after the moment it is
// taken (now is the harness clock), and last is the latest one handed out, the moment after which
// every expiring grant and membership has ended.
type expiries struct {
	now  func() time.Time
	last time.Time
}

// next returns an expiry expiryLead after now and records it in last when it is the latest.
func (e *expiries) next() time.Time {
	t := e.now().Add(expiryLead)
	if t.After(e.last) {
		e.last = t
	}
	return t
}

// grant creates a grant of the owner and returns its ID.
func (w *RandomWorld) grant(tb testing.TB, kind, subject string, t Target, level string, expires bool, exp *expiries) string {
	tb.Helper()
	body := map[string]any{"subject_kind": kind, "subject_id": subject, "target_kind": t.Kind, "level": level, "reason": "leak test"}
	switch t.Kind {
	case "space":
		body["target_id"] = w.Spaces[t.Space]
	case "channel":
		body["target_id"] = w.Channels[t.Space][t.Channel]
	}
	if expires {
		body["expires_at"] = stamp(exp.next())
	}
	return idOf(tb, harness.MustDo(tb, w.Owner, harness.Request{Method: http.MethodPost, Path: "/grants", Body: body}, http.StatusCreated))
}

// writePersonal writes the read states and drafts of the plan as the person, wherever it can
// read at this moment (a 404 means it cannot).
func (w *RandomWorld) writePersonal(tb testing.TB) {
	tb.Helper()
	h := w.Human
	for si, chans := range w.Plan.Spaces {
		for ci, c := range chans {
			cid := w.Channels[si][ci]
			// The lost channel always gets a draft and read states, the data it must hide later.
			lost := w.Plan.Lost.Space == si && w.Plan.Lost.Channel == ci
			threads := w.threadsOf(cid)
			for i, th := range threads {
				up := int64(c.ReadUpTo[min(i, len(c.ReadUpTo)-1)])
				if lost {
					up = max(up, 1)
				}
				if up < 0 {
					continue
				}
				up = min(up, w.lastSeq(th))
				r := h.Do(tb.Context(), harness.Request{Method: http.MethodPut, Path: "/threads/" + th + "/read-state", Body: map[string]any{"last_read_seq": up}})
				switch r.Status {
				case http.StatusOK:
					w.ReadStates[th] = up
				case http.StatusNotFound:
				default:
					tb.Fatalf("putReadState %d %s", r.Status, r.Body)
				}
			}
			if !c.Draft && !lost {
				continue
			}
			// A forum drafts a new post on the channel; a text channel drafts on its main thread.
			path, key := "/channels/"+cid+"/draft", cid
			if main := w.Main[cid]; main != "" {
				path, key = "/threads/"+main+"/draft", main
			}
			r := h.Do(tb.Context(), harness.Request{Method: http.MethodPut, Path: path, Body: map[string]any{"content_md": "Draft"}})
			switch r.Status {
			case http.StatusOK, http.StatusCreated:
				w.Drafts[key] = cid
			case http.StatusNotFound, http.StatusForbidden:
			default:
				tb.Fatalf("put draft %s %d %s", path, r.Status, r.Body)
			}
		}
	}
}

// threadsOf returns the threads of channel cid in creation order (the main thread first).
func (w *RandomWorld) threadsOf(cid string) []string {
	var out []string
	if m := w.Main[cid]; m != "" {
		out = append(out, m)
	}
	seen := map[string]bool{}
	for _, m := range w.Messages {
		if m.Channel == cid && m.Thread != w.Main[cid] && !seen[m.Thread] {
			seen[m.Thread] = true
			out = append(out, m.Thread)
		}
	}
	return out
}

// lastSeq returns the highest thread_seq of the messages written into thread.
func (w *RandomWorld) lastSeq(thread string) int64 {
	var n int64
	for _, m := range w.Messages {
		if m.Thread == thread && m.Seq > n {
			n = m.Seq
		}
	}
	return n
}

// AllChannels returns every channel ID of the world.
func (w *RandomWorld) AllChannels() []string {
	var out []string
	for _, cs := range w.Channels {
		out = append(out, cs...)
	}
	return out
}

// setOperator makes agent the operator of the space or channel at path as o.
func setOperator(tb testing.TB, o *harness.Client, path, agent string) {
	tb.Helper()
	cur := harness.MustDo(tb, o, harness.Request{Method: http.MethodGet, Path: path}, http.StatusOK)
	h := http.Header{}
	h.Set("If-Match", cur.Header.Get("ETag"))
	harness.MustDo(tb, o, harness.Request{Method: http.MethodPut, Path: path + "/operator", Header: h,
		Body: map[string]any{"mode": "agent", "agent_id": agent, "reason": "leak test"}}, http.StatusOK)
}

// idOf returns the member id of a JSON body.
func idOf(tb testing.TB, r harness.Response) string {
	tb.Helper()
	var b struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(r.Body, &b); err != nil || b.ID == "" {
		tb.Fatalf("leak: no id in %s", r.Body)
	}
	return b.ID
}

// stamp formats t as RFC 3339 in UTC with milliseconds (06 "Basic rules").
func stamp(t time.Time) string { return t.UTC().Format("2006-01-02T15:04:05.000Z") }
