// SPDX-License-Identifier: Apache-2.0

// Package agentworld builds the world of the load tests of the agent path (brief M2-36 design
// note 5) through the public API of a harness server, which the caller starts with raised rate
// limits (the seeding rule of M1-72): one space, per agent ChannelsPerAgent text channels with
// the default switches of 09 section 3.3 and the agent as their operator, ThreadsPerChannel
// background threads per channel, the agents with an API token each, trigger persons with the
// level trigger, background senders with write and viewers with read on the space, every person
// with one device. The handles are generated (agent-01, trigger-001, sender-001, viewer-001) and
// the addresses lie at example.com. It also sends the human messages of both tests.
//
// Example:
//
//	w, err := agentworld.Seed(t, p.Server, plan)
//	sent := w.Trigger(ctx, j, due)
//	bg := w.Background(ctx, i, due)
package agentworld

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/perf/internal/agentpath"
	"github.com/OWNER/bothy/services/server/test/harness"
)

// Agent is one seeded agent: its client (API token bound to a DPoP key), ID and handle.
type Agent struct {
	Client *harness.Client
	ID     string
	Handle string
}

// Channel is one text channel with its main thread and the index of its operator agent.
type Channel struct {
	ID, Thread string
	Agent      int
}

// World is the seeded world of a plan.
type World struct {
	// InstanceID is instance_id of getDiscovery, the iss of every invocation token.
	InstanceID string
	Space      string
	Agents     []Agent
	Channels   []Channel
	// Threads are the background threads, ThreadsPerChannel per channel.
	Threads                          []string
	TriggerPersons, Senders, Viewers []*harness.Client
}

// seedParallel is how many seeding requests run at once.
const seedParallel = 32

// clients numbers the simulated clients of a test binary, so each gets its own address.
var clients atomic.Int64

// clientAddress returns the documentation address of simulated client i: its own /64 of
// 2001:db8::/32 (RFC 3849), because the server counts IPv6 clients by /64; the prefix 2001:db8:a
// keeps it apart from the addresses of perfkit.
func clientAddress(i int64) string { return fmt.Sprintf("2001:db8:a%03x:%x::1", i>>16, i&0xffff) }

// Seed builds the world of plan on srv (design note 5). The background threads are created before
// the agents become operators, so seeding starts no run. The error reports the first failed
// request; tb only fails for the agent creation of harness.NewAgent.
func Seed(tb testing.TB, srv *harness.Server, plan agentpath.Plan) (*World, error) {
	tb.Helper()
	ctx := tb.Context()
	w := &World{}
	disc := srv.Owner.DoAnonymous(ctx, harness.Request{Method: http.MethodGet, Path: "/instance/discovery"})
	var d struct {
		InstanceID string `json:"instance_id"`
	}
	if disc.Status != http.StatusOK || json.Unmarshal(disc.Body, &d) != nil || d.InstanceID == "" {
		return nil, fmt.Errorf("getDiscovery = %d", disc.Status)
	}
	w.InstanceID = d.InstanceID
	space, err := create(ctx, srv.Owner, "/spaces", map[string]any{"name": "Agent path"})
	if err != nil {
		return nil, err
	}
	w.Space = space
	w.Channels = make([]Channel, plan.Channels)
	if err := parallel(ctx, plan.Channels, func(ctx context.Context, i int) error {
		b, err := do(ctx, srv.Owner, harness.Request{Method: http.MethodPost, Path: "/spaces/" + space + "/channels",
			Body: map[string]any{"name": fmt.Sprintf("agents-%02d", i+1), "kind": "text"}}, http.StatusCreated)
		if err != nil {
			return err
		}
		var c struct {
			ID           string `json:"id"`
			MainThreadID string `json:"main_thread_id"`
		}
		if err := json.Unmarshal(b, &c); err != nil {
			return err
		}
		w.Channels[i] = Channel{ID: c.ID, Thread: c.MainThreadID, Agent: i / agentpath.ChannelsPerAgent}
		return nil
	}); err != nil {
		return nil, err
	}
	w.Threads = make([]string, plan.Threads)
	if err := parallel(ctx, plan.Threads, func(ctx context.Context, i int) error {
		ch := w.Channels[i/agentpath.ThreadsPerChannel]
		id, err := thread(ctx, srv.Owner, ch, i)
		w.Threads[i] = id
		return err
	}); err != nil {
		return nil, err
	}
	for i := range plan.Agents {
		a, err := agent(tb, srv, i)
		if err != nil {
			return nil, err
		}
		w.Agents = append(w.Agents, a)
	}
	if err := parallel(ctx, len(w.Channels), func(ctx context.Context, i int) error {
		return setOperator(ctx, srv.Owner, w.Channels[i].ID, w.Agents[w.Channels[i].Agent].ID)
	}); err != nil {
		return nil, err
	}
	for _, g := range []struct {
		prefix, level string
		n             int
		into          *[]*harness.Client
	}{{"trigger", "trigger", plan.TriggerPersons, &w.TriggerPersons}, {"sender", "write", plan.Senders, &w.Senders},
		{"viewer", "read", plan.Viewers, &w.Viewers}} {
		people, err := persons(ctx, srv, space, g.prefix, g.level, g.n)
		if err != nil {
			return nil, err
		}
		*g.into = people
	}
	return w, nil
}

// agent creates agent i through createAgent and createToken (harness.NewAgent) and checks that
// the createAgent defaults give billing mode organization and tool class chat, which the measured
// path needs (getAgent).
func agent(tb testing.TB, srv *harness.Server, i int) (Agent, error) {
	handle := fmt.Sprintf("agent-%02d", i+1)
	c := srv.NewAgent(tb, harness.AgentSpec{Handle: handle, DisplayName: fmt.Sprintf("Agent %02d", i+1)})
	c.ClientIP = clientAddress(clients.Add(1))
	b, err := do(tb.Context(), srv.Owner, harness.Request{Method: http.MethodGet, Path: "/agents/" + c.PrincipalID}, http.StatusOK)
	if err != nil {
		return Agent{}, err
	}
	var a struct {
		BillingMode string `json:"billing_mode"`
		ToolClass   string `json:"tool_class"`
	}
	if err := json.Unmarshal(b, &a); err != nil {
		return Agent{}, err
	}
	if a.BillingMode != "organization" || a.ToolClass != "chat" {
		return Agent{}, fmt.Errorf("agent %s has billing mode %q and tool class %q, want organization and chat", handle, a.BillingMode, a.ToolClass)
	}
	return Agent{Client: c, ID: c.PrincipalID, Handle: handle}, nil
}

// thread creates background thread i of ch: a message of the owner in the main thread and a
// thread on it with a first message (createThread).
func thread(ctx context.Context, owner *harness.Client, ch Channel, i int) (string, error) {
	parent, err := create(ctx, owner, "/threads/"+ch.Thread+"/messages", map[string]any{"client_msg_id": ids.New().String(), "content_md": fmt.Sprintf("Topic %d", i+1)})
	if err != nil {
		return "", err
	}
	b, err := do(ctx, owner, harness.Request{Method: http.MethodPost, Path: "/channels/" + ch.ID + "/threads", Body: map[string]any{
		"title": fmt.Sprintf("Topic %d", i+1), "parent_message_id": parent,
		"first_message": map[string]any{"client_msg_id": ids.New().String(), "content_md": "Start of the topic"},
	}}, http.StatusCreated)
	if err != nil {
		return "", err
	}
	var th struct {
		Thread struct {
			ID string `json:"id"`
		} `json:"thread"`
	}
	if err := json.Unmarshal(b, &th); err != nil || th.Thread.ID == "" {
		return "", fmt.Errorf("createThread: no thread id (%v)", err)
	}
	return th.Thread.ID, nil
}

// setOperator makes agent the operator of the channel (setChannelOperator) with the ETag of the
// channel as If-Match; the channel keeps its default switches.
func setOperator(ctx context.Context, owner *harness.Client, channel, agent string) error {
	cur := owner.Do(ctx, harness.Request{Method: http.MethodGet, Path: "/channels/" + channel})
	if cur.Status != http.StatusOK {
		return fmt.Errorf("getChannel = %d", cur.Status)
	}
	h := http.Header{}
	h.Set("If-Match", cur.Header.Get("ETag"))
	_, err := do(ctx, owner, harness.Request{Method: http.MethodPut, Path: "/channels/" + channel + "/operator", Header: h,
		Body: map[string]any{"mode": "agent", "agent_id": agent, "reason": "load test"}}, http.StatusOK)
	return err
}

// persons creates n people prefix-001 ... with one device each and a grant of level on the space.
func persons(ctx context.Context, srv *harness.Server, space, prefix, level string, n int) ([]*harness.Client, error) {
	out := make([]*harness.Client, n)
	err := parallel(ctx, n, func(ctx context.Context, i int) error {
		handle := fmt.Sprintf("%s-%03d", prefix, i+1)
		c, err := srv.CreateUser(ctx, harness.UserSpec{Handle: handle, DisplayName: handle, Email: handle + "@example.com"})
		if err != nil {
			return err
		}
		c.ClientIP = clientAddress(clients.Add(1))
		out[i] = c
		_, err = do(ctx, srv.Owner, harness.Request{Method: http.MethodPost, Path: "/grants", Body: map[string]any{
			"subject_kind": "user", "subject_id": c.PrincipalID, "target_kind": "space", "target_id": space, "level": level, "reason": "load test",
		}}, http.StatusCreated)
		return err
	})
	return out, err
}

// Sent is one human message of a load: its kind, the created message, when it was due and the
// status of its answer.
type Sent struct {
	Trigger   bool
	MessageID string
	Scheduled time.Time
	Status    int
	// TriggerState is trigger_state of the answer.
	TriggerState string
	Code         string
}

// OK reports whether the message was created as expected: 201, and for a trigger trigger_state
// received (04 section 15.3 step 1).
func (s Sent) OK() bool {
	return s.Status == http.StatusCreated && (!s.Trigger || s.TriggerState == "received")
}

// Trigger posts trigger j into the main thread of channel j mod channels as trigger person j mod
// persons, mentioning the channel's operator agent by its handle, with a fresh client_msg_id.
func (w *World) Trigger(ctx context.Context, j int, due time.Time) Sent {
	ch := w.Channels[j%len(w.Channels)]
	who := w.TriggerPersons[j%len(w.TriggerPersons)]
	return post(ctx, who, ch.Thread, fmt.Sprintf("@%s please look at item %d", w.Agents[ch.Agent].Handle, j), true, due)
}

// TriggerIn posts trigger j into the main thread of channel as trigger person j mod persons,
// mentioning the channel's operator agent by its handle (the setup triggers of perf/run_patch).
func (w *World) TriggerIn(ctx context.Context, channel, j int, due time.Time) Sent {
	ch := w.Channels[channel]
	who := w.TriggerPersons[j%len(w.TriggerPersons)]
	return post(ctx, who, ch.Thread, fmt.Sprintf("@%s please work on task %d", w.Agents[ch.Agent].Handle, j), true, due)
}

// Background posts background message i into thread i mod threads as sender i mod senders.
func (w *World) Background(ctx context.Context, i int, due time.Time) Sent {
	return post(ctx, w.Senders[i%len(w.Senders)], w.Threads[i%len(w.Threads)], fmt.Sprintf("Note %d", i), false, due)
}

// post sends one createMessage and records it.
func post(ctx context.Context, c *harness.Client, thread, text string, trigger bool, due time.Time) Sent {
	r := c.Do(ctx, harness.Request{Method: http.MethodPost, Path: "/threads/" + thread + "/messages",
		Body: map[string]any{"client_msg_id": ids.New().String(), "content_md": text}})
	s := Sent{Trigger: trigger, Scheduled: due, Status: r.Status}
	var m struct {
		ID           string `json:"id"`
		TriggerState string `json:"trigger_state"`
		Code         string `json:"code"`
	}
	_ = json.Unmarshal(r.Body, &m)
	if r.Status == http.StatusCreated {
		s.MessageID, s.TriggerState = m.ID, m.TriggerState
	} else {
		s.Code = m.Code
	}
	return s
}

// create posts body to path as c, expects 201 and returns the id member.
func create(ctx context.Context, c *harness.Client, path string, body any) (string, error) {
	b, err := do(ctx, c, harness.Request{Method: http.MethodPost, Path: path, Body: body}, http.StatusCreated)
	if err != nil {
		return "", err
	}
	var v struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(b, &v); err != nil || v.ID == "" {
		return "", fmt.Errorf("POST %s: no id (%v)", path, err)
	}
	return v.ID, nil
}

// do sends r as c with a step-up when asked and returns the body; a status other than want is an
// error naming the problem code.
func do(ctx context.Context, c *harness.Client, r harness.Request, want int) ([]byte, error) {
	resp, err := harness.DoWithStepUp(ctx, c, r)
	if err != nil {
		return nil, err
	}
	if resp.Status != want {
		var p struct {
			Code string `json:"code"`
		}
		_ = json.Unmarshal(resp.Body, &p)
		return nil, fmt.Errorf("%s %s = %d %s", r.Method, r.Path, resp.Status, p.Code)
	}
	return resp.Body, nil
}

// parallel runs f for 0..n-1 on seedParallel goroutines and returns the first error.
func parallel(parent context.Context, n int, f func(ctx context.Context, i int) error) error {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	var next atomic.Int64
	var once sync.Once
	var first error
	var wg sync.WaitGroup
	for range min(seedParallel, max(n, 1)) {
		wg.Go(func() {
			for {
				i := int(next.Add(1) - 1)
				if i >= n || ctx.Err() != nil {
					return
				}
				if err := f(ctx, i); err != nil {
					once.Do(func() { first = err; cancel() })
					return
				}
			}
		})
	}
	wg.Wait()
	if first == nil {
		return parent.Err()
	}
	return first
}

// Querier is the superuser pool of the measured database (harness AdminDB), for the read-only
// checks after a measurement.
type Querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// RunStates returns how many runs are in each state (read-only query on runs, as the invariant
// queries of M1-72 read their database).
func RunStates(ctx context.Context, q Querier) (map[string]int, error) {
	rows, err := q.Query(ctx, `SELECT state::text, count(*) FROM runs GROUP BY state`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var state string
		var n int
		if err := rows.Scan(&state, &n); err != nil {
			return nil, err
		}
		out[state] = n
	}
	return out, rows.Err()
}

// SendProblems returns one sentence per kind of failed send (any status other than 201, among
// them 429 rate_limited, and a trigger whose trigger_state is not received), with its count and
// first example; none for a clean load.
func SendProblems(sent []Sent) []string {
	var first string
	n := 0
	for _, s := range sent {
		if s.OK() {
			continue
		}
		if n == 0 {
			first = fmt.Sprintf("status %d %s, trigger %v, trigger_state %q", s.Status, s.Code, s.Trigger, s.TriggerState)
		}
		n++
	}
	if n == 0 {
		return nil
	}
	return []string{fmt.Sprintf("%d of %d messages not created as expected, first: %s", n, len(sent), first)}
}
