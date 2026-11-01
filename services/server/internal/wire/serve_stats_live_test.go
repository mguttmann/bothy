// SPDX-License-Identifier: Apache-2.0

package wire_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"slices"
	gosync "sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/api/routes"
	"github.com/OWNER/bothy/services/server/internal/gateway/adapters/wstest"
	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
	statsapp "github.com/OWNER/bothy/services/server/internal/stats/app"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

// slCanary is the canary of VZ-T07: it is the text of messages, the title of the run's thread,
// the step title and the step detail, and must appear in no frame and no getStatsLive answer, except
// as the thread title of the run's chat for a person who may read the thread (25 4 rule 6).
const slCanary = "canary-live-5d1c"

// slWorld is the live-box world (Support with the Owner Alex, Sam, Robin) of a process in role
// all with the live stream of M6-15: stats.live_interval of an hour, so only the test builds
// frames (Live.Tick), and stats.flush_interval of an hour, so stats_minutes changes only when the
// test flushes.
type slWorld struct {
	*lbWorld
	live    *statsapp.Live
	measure *wire.Measure
	// frames are the payloads of every stats.live frame and getStatsLive answer of the world, for
	// the canary check of VZ-T07.
	mu     gosync.Mutex
	frames [][]byte
}

// newSLWorld starts the world with the environment entries extra.
func newSLWorld(t *testing.T, extra ...string) *slWorld {
	t.Helper()
	lives := make(chan *statsapp.Live, 1)
	measures := make(chan *wire.Measure, 1)
	env := append(lbBaseEnv(), "BOTHY_STATS__LIVE_INTERVAL=1h", "BOTHY_STATS__FLUSH_INTERVAL=1h")
	gw := newGatewayWorldRole(t, wire.RoleAll, wire.Deps{
		StatsLive: func(l *statsapp.Live) { lives <- l }, StatsMeasure: func(m *wire.Measure) { measures <- m },
	}, append(env, extra...)...)
	gw.partitions(t)
	owner := gw.newPerson("alex", identity.DeviceStateActive)
	gw.sql(`INSERT INTO owners (id, user_id, reason) VALUES ($1, $2, 'test')`, ids.New().String(), owner.user.String())
	dl := &dlWorld{gwWorld: gw, owner: owner}
	cw := &consumerWorld{dlWorld: dl, space: dl.newSpace("Support")}
	iw := &invocationWorld{consumerWorld: cw, sam: dl.newPerson("sam", identity.DeviceStateActive)}
	return &slWorld{lbWorld: &lbWorld{invocationWorld: iw, robin: dl.newPerson("robin", identity.DeviceStateActive)},
		live: testkit.Receive(t, "the live stream", lives), measure: testkit.Receive(t, "the own measurement", measures)}
}

// slFrame is one server frame as the live tests read it: the op, the payload of a stats.live
// dispatch, and op and code of an error.
type slFrame struct {
	op      string
	typ     string
	payload json.RawMessage
	errOp   string
	code    string
}

// next reads the next frame of c.
func slNext(t *testing.T, c *wstest.Client) slFrame {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), testkit.Patience)
	defer cancel()
	op, d, err := c.Next(ctx)
	if err != nil {
		t.Fatalf("next frame: %v", err)
	}
	f := slFrame{op: op}
	switch op {
	case "dispatch":
		var ev struct {
			Type    string          `json:"type"`
			Payload json.RawMessage `json:"payload"`
		}
		if err := json.Unmarshal(d, &ev); err != nil {
			t.Fatal(err)
		}
		f.typ, f.payload = ev.Type, ev.Payload
	case "error":
		var e struct {
			Op   string `json:"op"`
			Code string `json:"code"`
		}
		if err := json.Unmarshal(d, &e); err != nil {
			t.Fatal(err)
		}
		f.errOp, f.code = e.Op, e.Code
	}
	return f
}

// probe sends a focus over sync.max_focused_threads, which every principal gets answered with
// error validation_failed and which changes nothing, and returns the frames before that answer:
// once it arrives, every earlier op of the connection is handled and every frame queued before
// it was read.
func (w *slWorld) probe(t *testing.T, c *wstest.Client) []slFrame {
	t.Helper()
	threads := make([]string, w.cfg.MaxFocusedThreads+1)
	for i := range threads {
		threads[i] = ids.New().String()
	}
	if err := c.Send("focus", map[string]any{"thread_ids": threads}); err != nil {
		t.Fatal(err)
	}
	var out []slFrame
	for {
		f := slNext(t, c)
		if f.op == "error" && f.errOp == "focus" && f.code == "validation_failed" {
			return out
		}
		if f.typ == "stats.live" {
			w.mu.Lock()
			w.frames = append(w.frames, f.payload)
			w.mu.Unlock()
		}
		out = append(out, f)
	}
}

// lives returns the stats.live payloads of fs.
func lives(fs []slFrame) []json.RawMessage {
	var out []json.RawMessage
	for _, f := range fs {
		if f.typ == "stats.live" {
			out = append(out, f.payload)
		}
	}
	return out
}

// subscribeErrors returns the codes of the error frames of stats.subscribe among fs.
func subscribeErrors(fs []slFrame) []string {
	var out []string
	for _, f := range fs {
		if f.op == "error" && f.errOp == "stats.subscribe" {
			out = append(out, f.code)
		}
	}
	return out
}

// subscribe sends stats.subscribe for space ("" for the instance) and returns the frames up to
// the probe that follows it.
func (w *slWorld) subscribe(t *testing.T, c *wstest.Client, space string) []slFrame {
	t.Helper()
	d := map[string]any{}
	if space != "" {
		d["space_id"] = space
	}
	if err := c.Send("stats.subscribe", d); err != nil {
		t.Fatal(err)
	}
	return w.probe(t, c)
}

// tick drives one interval of the builder and fails when the node was idle.
func (w *slWorld) tick(t *testing.T) {
	t.Helper()
	if !w.live.Tick(context.Background()) {
		t.Fatal("the builder was idle with a subscription")
	}
}

// frameOf ticks and probes c until a stats.live frame arrives and returns it: a connection goes
// live only after its catch-up, so the frames of the first ticks may be dropped.
func (w *slWorld) frameOf(t *testing.T, c *wstest.Client) json.RawMessage {
	t.Helper()
	deadline := testkit.Deadline()
	for time.Now().Before(deadline) {
		w.tick(t)
		if got := lives(w.probe(t, c)); len(got) > 0 {
			return got[len(got)-1]
		}
	}
	t.Fatal("no stats.live frame within testkit.Patience")
	return nil
}

// liveDoc is a frame as the client reads it.
type liveDoc struct {
	At    string `json:"at"`
	Scope struct {
		Kind    string `json:"kind"`
		SpaceID string `json:"space_id"`
	} `json:"scope"`
	Runs []struct {
		RunID           string `json:"run_id"`
		SpaceID         string `json:"space_id"`
		ChannelID       string `json:"channel_id"`
		ApprovalPending bool   `json:"approval_pending"`
		CostMicros      *int64 `json:"cost_micros"`
		Step            *struct {
			Kind string `json:"kind"`
			Name string `json:"name"`
		} `json:"step"`
	} `json:"runs"`
	Agents []struct {
		AgentID string `json:"agent_id"`
	} `json:"agents"`
	Rates []struct {
		Metric    string `json:"metric"`
		Dimension string `json:"dimension"`
		Currency  string `json:"currency"`
		Last      int64  `json:"last_minute"`
		Current   int64  `json:"current_minute"`
	} `json:"rates"`
	SLI         []json.RawMessage `json:"sli"`
	Connections *json.RawMessage  `json:"connections"`
}

// decodeLive decodes a frame.
func decodeLive(t *testing.T, b []byte) liveDoc {
	t.Helper()
	var d liveDoc
	if err := json.Unmarshal(b, &d); err != nil {
		t.Fatalf("frame %s: %v", b, err)
	}
	return d
}

// messages returns last plus current minute of the summed messages_created row of d.
func (d liveDoc) messages() int64 {
	for _, r := range d.Rates {
		if r.Metric == "messages_created" && r.Dimension == "" {
			return r.Last + r.Current
		}
	}
	return 0
}

// expectedMessages sums messages_created in stats_minutes over the two minutes of the frame at
// in the channels chans (every channel when nil).
func (w *slWorld) expectedMessages(t *testing.T, at string, chans []string) int64 {
	t.Helper()
	ts, err := time.Parse(time.RFC3339Nano, at)
	if err != nil {
		t.Fatal(err)
	}
	cur := ts.UTC().Truncate(time.Minute)
	var n int64
	if err := w.queryRow(context.Background(), `SELECT coalesce(sum(count), 0) FROM stats_minutes
		WHERE metric = 'messages_created' AND bucket_start IN ($1, $2) AND ($3::uuid[] IS NULL OR channel_id = ANY ($3::uuid[]))`,
		cur.Add(-time.Minute), cur, chans).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// startRunInThread creates the text channel name in space operated by a new agent, a thread with
// title from a message of its main thread, lets the Owner trigger the agent there with text and
// starts the run over the agent's gateway connection (M2-09).
func (w *slWorld) startRunInThread(t *testing.T, space, name, title, text string) lbRun {
	t.Helper()
	ch := w.newChannel(space, name)
	a := w.agent()
	w.sql(`UPDATE channels SET operator_inherit = false, operator_agent_id = $1 WHERE id = $2`, a.id.String(), ch.id)
	var handle string
	if err := w.queryRow(context.Background(), `SELECT handle FROM principals WHERE id = $1`, a.id.String()).Scan(&handle); err != nil {
		t.Fatal(err)
	}
	c, _ := w.ready(t, w.agentParams(a, ""))
	readUntil(t, c, func(f frame) bool { return f.op == "catchup.complete" })
	parent, _ := w.say(w.owner, ch.thread, text)
	code, b := w.rest(w.base, w.owner, http.MethodPost, "/api/v1/channels/"+ch.id+"/threads", map[string]any{"parent_message_id": parent, "title": title})
	var created struct {
		Thread struct {
			ID string `json:"id"`
		} `json:"thread"`
	}
	if code != http.StatusCreated || json.Unmarshal(b, &created) != nil || created.Thread.ID == "" {
		t.Fatalf("createThread = %d %s", code, b)
	}
	m, _ := w.say(w.owner, created.Thread.ID, "@"+handle+" "+text)
	f := w.nextInvocation(c, true)
	sendAck(t, c, f.Seq)
	r, ok := w.runOf(m)
	if !ok {
		t.Fatal("no run for the trigger")
	}
	code, b = w.call(w.base, a, http.MethodPost, "/api/v1/runs/"+r.id+"/start",
		map[string]any{"invocation_token": f.InvocationToken, "billing_mode_used": "organization", "tool_class_used": "chat"})
	if code != http.StatusOK {
		t.Fatalf("startRun = %d %s", code, b)
	}
	return lbRun{id: r.id, thread: created.Thread.ID, channel: ch.id, agent: a, conn: c}
}

// liveGet calls getStatsLive as p (space "" for the instance) and returns status and body; a 200
// body joins the frames of the canary check.
func (w *slWorld) liveGet(p gwPerson, space string) (int, []byte) {
	path := "/api/v1/stats/live"
	if space != "" {
		path += "?space_id=" + space
	}
	code, b := w.rest(w.base, p, http.MethodGet, path, nil)
	if code == http.StatusOK {
		w.mu.Lock()
		w.frames = append(w.frames, b)
		w.mu.Unlock()
	}
	return code, b
}

// TestStatsLive is the integration suite of M6-15 against a process in role all (PostgreSQL 18
// via pgtest, the wstest client on 127.0.0.1:0): Support with a run of support-agent in a
// thread titled with the canary, a running canary step, a cost so far and a pending approval
// card; Engineering with a run of dev-runner; Sam with manage on Support, read on Support and on
// dev; privacy.per_person_stats_enabled on (DP-T12); the subtests share the world in order.
func TestStatsLive(t *testing.T) {
	t.Parallel()
	// DP-T12: statistics per person switched on change nothing in the live frame.
	w := newSLWorld(t, "BOTHY_PRIVACY__PER_PERSON_STATS_ENABLED=true")
	ctx := context.Background()
	support := w.startRunInThread(t, w.space, "support", slCanary, slCanary)
	step := ids.New().String()
	v := w.version(support.id)
	v = w.patchOK(t, support.agent, support.id, v, op("replace", "/cost", map[string]any{"amount_micros": 4200, "currency": "USD", "is_estimate": true}))
	w.patchOK(t, support.agent, support.id, v, op("add", "/steps/"+step, map[string]any{"kind": "tool_call", "name": "Bash", "title": slCanary,
		"detail": map[string]any{"input": slCanary}}))
	code, b := w.call(w.base, support.agent, http.MethodPost, "/api/v1/threads/"+support.thread+"/messages", map[string]any{
		"client_msg_id": ids.New().String(), "content_md": "Approve the refund?", "kind": "card", "run_id": support.id,
		"components": []any{map[string]any{"custom_id": "approve", "type": "button", "definition": map[string]any{"label": "Approve", "value": "yes"},
			"required_capability": "approve_actions"}}})
	if code != http.StatusCreated {
		t.Fatalf("approval card = %d %s", code, b)
	}
	engineering := w.newSpace("Engineering")
	dev := w.startRunInThread(t, engineering, "dev", "Build", "please build")
	manage := w.must(w.owner, http.MethodPost, "/api/v1/grants", map[string]any{"subject_kind": "user", "subject_id": w.sam.user.String(),
		"target_kind": "space", "target_id": w.space, "level": "read", "capabilities": []string{"manage"}, "reason": "test"})
	w.grant(w.sam.user, "user", "space", w.space)
	w.grant(w.sam.user, "user", "channel", dev.channel)
	if err := w.measure.Flush.Flush(ctx); err != nil {
		t.Fatal(err)
	}
	var supportChannels []string
	if err := w.queryRow(ctx, `SELECT coalesce(array_agg(id::text), '{}') FROM channels WHERE space_id = $1`, w.space).Scan(&supportChannels); err != nil {
		t.Fatal(err)
	}

	sam, _ := w.ready(t, w.params(w.sam))
	alex, _ := w.ready(t, w.params(w.owner))

	t.Run("TestLive_SpaceManager", func(t *testing.T) {
		if fs := w.subscribe(t, sam, w.space); len(subscribeErrors(fs)) > 0 {
			t.Fatalf("Sam's subscription to Support: %v", subscribeErrors(fs))
		}
		if fs := w.subscribe(t, alex, ""); len(subscribeErrors(fs)) > 0 {
			t.Fatalf("Alex's subscription: %v", subscribeErrors(fs))
		}
		got := decodeLive(t, w.frameOf(t, sam))
		if got.Scope.Kind != "space" || got.Scope.SpaceID != w.space || got.SLI != nil || got.Connections != nil {
			t.Fatalf("Sam's frame scope %+v, sli %v, connections %v", got.Scope, got.SLI, got.Connections)
		}
		if len(got.Runs) != 1 || got.Runs[0].RunID != support.id || got.Runs[0].SpaceID != w.space {
			t.Fatalf("Sam's runs %+v, want only the run of Support", got.Runs)
		}
		r := got.Runs[0]
		if r.CostMicros == nil || *r.CostMicros != 4200 || !r.ApprovalPending || r.Step == nil || r.Step.Kind != "tool_call" || r.Step.Name != "Bash" {
			t.Fatalf("support run %+v", r)
		}
		for _, a := range got.Agents {
			if a.AgentID == dev.agent.id.String() {
				t.Fatal("dev-runner of Engineering in Sam's frame")
			}
		}
		for _, rt := range got.Rates {
			if rt.Metric == "catchups" || rt.Metric == "resyncs" || rt.Metric == "slow_consumer_closes" || rt.Metric == "billing_mode_switches" {
				t.Fatalf("a row without a channel in Sam's frame: %+v", rt)
			}
		}
		if want := w.expectedMessages(t, got.At, supportChannels); got.messages() != want {
			t.Fatalf("Sam's messages_created %d, stats_minutes of Support %d", got.messages(), want)
		}
		owner := decodeLive(t, w.frameOf(t, alex))
		if owner.Scope.Kind != "instance" || owner.SLI == nil || owner.Connections == nil || len(owner.Runs) != 2 {
			t.Fatalf("Alex's frame scope %+v, sli %v, connections %v, %d runs", owner.Scope, owner.SLI, owner.Connections, len(owner.Runs))
		}
		for _, run := range owner.Runs {
			if run.CostMicros == nil {
				t.Fatalf("Alex's run %s without cost", run.RunID)
			}
		}
		if want := w.expectedMessages(t, owner.At, nil); owner.messages() != want || want <= w.expectedMessages(t, owner.At, supportChannels) {
			t.Fatalf("Alex's messages_created %d, stats_minutes of every channel %d", owner.messages(), want)
		}
		w.unsubscribe(t, sam)
		w.unsubscribe(t, alex)
	})

	t.Run("TestLive_NoBuildWithoutSubscription", func(t *testing.T) {
		testkit.Until(t, "no subscription on the node", func() bool { return w.live.Subscribers() == 0 })
		before := w.live.Builds()
		if w.live.Tick(ctx) || w.live.Builds() != before {
			t.Fatalf("a tick without a subscription built: %d builds, %d before", w.live.Builds(), before)
		}
		other := w.addSession(w.owner)
		third, _ := w.ready(t, w.params(other))
		conns := []*wstest.Client{alex, sam, third}
		spaces := []string{"", w.space, ""}
		for i, c := range conns {
			w.subscribe(t, c, spaces[i])
		}
		w.tick(t)
		if w.live.Builds() != before+1 {
			t.Fatalf("one tick with three subscribers built %d frames", w.live.Builds()-before)
		}
		for i, c := range conns {
			if n := len(lives(w.probe(t, c))); n != 1 {
				t.Fatalf("subscriber %d got %d frames of one build", i, n)
			}
		}
		for _, c := range conns {
			w.unsubscribe(t, c)
		}
		if w.live.Tick(ctx) || w.live.Builds() != before+1 {
			t.Fatalf("a tick after the last unsubscribe built: %d builds", w.live.Builds()-before)
		}
		third.Close()
	})

	t.Run("TestLive_ResumeRestores", func(t *testing.T) {
		second := w.addSession(w.owner)
		c, r := w.ready(t, w.params(second))
		w.subscribe(t, c, "")
		w.frameOf(t, c)
		c.Abort()
		n := w.dial(t)
		rctx, cancel := context.WithTimeout(ctx, testkit.Patience)
		defer cancel()
		if _, err := n.Resume(rctx, wstest.ResumeParams{SessionID: r.GatewaySessionId.String(), AccessToken: second.access,
			Proof: w.proofFor(second.key, second.access, gwHTU), Cursor: r.HeadSeq}); err != nil {
			t.Fatalf("resume: %v", err)
		}
		if errs := subscribeErrors(w.probe(t, n)); len(errs) > 0 {
			t.Fatalf("the restored subscription was refused: %v", errs)
		}
		w.frameOf(t, n)
		third := w.addSession(w.owner)
		fresh, _ := w.ready(t, w.params(third))
		w.probe(t, fresh)
		w.tick(t)
		if got := lives(w.probe(t, fresh)); len(got) != 0 {
			t.Fatalf("identify without resume received %d frames", len(got))
		}
		w.unsubscribe(t, n)
		n.Close()
		fresh.Close()
	})

	t.Run("TestLive_AgentCannotSubscribe", func(t *testing.T) {
		codes := subscribeErrors(w.subscribe(t, support.conn, ""))
		if !slices.Equal(codes, []string{"forbidden"}) {
			t.Fatalf("an agent's stats.subscribe answered %v, want forbidden", codes)
		}
	})

	t.Run("TestLive_Revocation", func(t *testing.T) {
		w.subscribe(t, sam, w.space)
		w.frameOf(t, sam)
		w.must(w.owner, http.MethodPost, "/api/v1/grants/"+manage+"/revoke", map[string]any{"reason": "test"})
		// The log tail applied the revocation when Sam's connection received access.changed.
		for {
			if f := slNext(t, sam); f.typ == "access.changed" {
				break
			}
		}
		w.tick(t)
		fs := w.probe(t, sam)
		if codes := subscribeErrors(fs); !slices.Equal(codes, []string{"forbidden"}) {
			t.Fatalf("errors after the revocation %v, want one forbidden", codes)
		}
		if n := len(lives(fs)); n != 0 {
			t.Fatalf("%d frames after the revocation", n)
		}
		testkit.Until(t, "the slot of the ended subscription", func() bool { return w.live.Subscribers() == 0 })
		w.subscribe(t, alex, "")
		w.tick(t)
		w.tick(t)
		if fs := w.probe(t, sam); len(lives(fs)) != 0 || len(subscribeErrors(fs)) != 0 {
			t.Fatalf("Sam received %v after the end of the subscription", fs)
		}
		w.unsubscribe(t, alex)
	})

	t.Run("TestLive_NoContentNoPeople", func(t *testing.T) {
		for _, space := range []string{"", w.space} {
			if code, b := w.liveGet(w.owner, space); code != http.StatusOK {
				t.Fatalf("getStatsLive %q = %d %s", space, code, b)
			}
		}
		if code, _ := w.liveGet(w.sam, w.space); code != http.StatusForbidden {
			t.Fatalf("getStatsLive of Sam after the revocation = %d, want 403", code)
		}
		w.mu.Lock()
		defer w.mu.Unlock()
		if len(w.frames) < 4 {
			t.Fatalf("only %d frames collected", len(w.frames))
		}
		people := []string{w.owner.user.String(), w.sam.user.String(), w.robin.user.String()}
		named := false
		for _, f := range w.frames {
			rest, titles := withoutChatTitles(t, f)
			named = named || slices.Contains(titles, slCanary)
			if bytes.Contains(rest, []byte(slCanary)) {
				t.Fatalf("the canary outside the chat of a run in a frame: %s", f)
			}
			for _, p := range people {
				if bytes.Contains(f, []byte(p)) {
					t.Fatalf("the id of a person in a frame: %s", f)
				}
			}
		}
		if !named {
			t.Fatal("no frame names the chat of the support run for a person who may read it")
		}
	})
}

// unsubscribe sends stats.unsubscribe and waits for the probe after it.
func (w *slWorld) unsubscribe(t *testing.T, c *wstest.Client) {
	t.Helper()
	if err := c.Send("stats.unsubscribe", map[string]any{}); err != nil {
		t.Fatal(err)
	}
	w.probe(t, c)
}

// TestStatsLiveLimits: getStatsLive on an idle node builds once for ten parallel polls, filtered
// per caller (R-VZ-06); with stats.live_max_subscribers 2 the third subscription is rate_limited
// and its connection stays.
func TestStatsLiveLimits(t *testing.T) {
	t.Parallel()
	w := newSLWorld(t, "BOTHY_STATS__LIVE_MAX_SUBSCRIBERS=2")
	w.must(w.owner, http.MethodPost, "/api/v1/grants", map[string]any{"subject_kind": "user", "subject_id": w.sam.user.String(),
		"target_kind": "space", "target_id": w.space, "level": "read", "capabilities": []string{"manage"}, "reason": "test"})
	engineering := w.newSpace("Engineering")
	w.newChannel(engineering, "dev")

	t.Run("TestGetStatsLive_NoRebuildPerRequest", func(t *testing.T) {
		before := w.live.Builds()
		var wg gosync.WaitGroup
		bodies := make([][]byte, 10)
		codes := make([]int, 10)
		for i := range 10 {
			wg.Go(func() {
				p, space := w.owner, ""
				if i%2 == 1 {
					p, space = w.sam, w.space
				}
				codes[i], bodies[i] = w.liveGet(p, space)
			})
		}
		wg.Wait()
		for i, code := range codes {
			if code != http.StatusOK {
				t.Fatalf("poll %d = %d %s", i, code, bodies[i])
			}
		}
		if n := w.live.Builds() - before; n != 1 {
			t.Fatalf("ten polls built %d frames", n)
		}
		owner, sam := decodeLive(t, bodies[0]), decodeLive(t, bodies[1])
		if owner.At != sam.At || owner.Scope.Kind != "instance" || sam.Scope.Kind != "space" || sam.SLI != nil || owner.SLI == nil {
			t.Fatalf("owner %+v, Sam %+v", owner, sam)
		}
	})

	t.Run("TestLive_MaxSubscribers", func(t *testing.T) {
		var conns []*wstest.Client
		for i := range 3 {
			p := w.addSession(w.owner)
			c, _ := w.ready(t, w.params(p))
			conns = append(conns, c)
			codes := subscribeErrors(w.subscribe(t, c, ""))
			if want := []string(nil); i == 2 {
				want = []string{"rate_limited"}
				if !slices.Equal(codes, want) {
					t.Fatalf("third subscription answered %v, want rate_limited", codes)
				}
			} else if len(codes) != 0 {
				t.Fatalf("subscription %d answered %v", i, codes)
			}
		}
		// The refused connection stays open and is served: its probe is answered.
		w.probe(t, conns[2])
		if w.live.Subscribers() != 2 {
			t.Fatalf("%d subscriptions on the node", w.live.Subscribers())
		}
		// A node without an active run still sends frames, with an empty list of runs (no null).
		var raw map[string]json.RawMessage
		if err := json.Unmarshal(w.frameOf(t, conns[0]), &raw); err != nil || string(raw["runs"]) != "[]" {
			t.Fatalf("frame without runs: runs %s, %v", raw["runs"], err)
		}
	})
}

// TestStatsLiveDisabled (VZ-T10): with stats.live_enabled false stats.subscribe answers error
// feature_disabled and keeps the connection, getStatsLive answers 403 feature_disabled.
func TestStatsLiveDisabled(t *testing.T) {
	t.Parallel()
	w := newSLWorld(t, "BOTHY_STATS__LIVE_ENABLED=false")
	t.Run("TestLive_Disabled", func(t *testing.T) {
		c, _ := w.ready(t, w.params(w.owner))
		if codes := subscribeErrors(w.subscribe(t, c, "")); !slices.Equal(codes, []string{"feature_disabled"}) {
			t.Fatalf("stats.subscribe answered %v, want feature_disabled", codes)
		}
		code, b := w.liveGet(w.owner, "")
		var p struct {
			Code string `json:"code"`
		}
		if code != http.StatusForbidden || json.Unmarshal(b, &p) != nil || p.Code != "feature_disabled" {
			t.Fatalf("getStatsLive = %d %s", code, b)
		}
		if w.live.Builds() != 0 || w.live.Subscribers() != 0 {
			t.Fatalf("%d builds, %d subscriptions with the stream off", w.live.Builds(), w.live.Subscribers())
		}
	})
}

// TestGatewayOpsEnabled: stats.subscribe and stats.unsubscribe are in gateway_ops.enabled with
// stats.read and the schemas of asyncapi, and the gateway counts both in rl.read.
func TestGatewayOpsEnabled(t *testing.T) {
	t.Parallel()
	want := map[string]string{"stats.subscribe": "GatewayStatsSubscribe", "stats.unsubscribe": "GatewayStatsUnsubscribe"}
	for _, op := range routes.GatewayOps() {
		if schema, ok := want[op.Op]; ok {
			if op.Action != "stats.read" || op.Schema != schema {
				t.Errorf("%s: action %s schema %s", op.Op, op.Action, op.Schema)
			}
			delete(want, op.Op)
		}
	}
	if len(want) != 0 {
		t.Fatalf("not enabled: %v", want)
	}
	classes := wire.GatewayOpClasses()
	for _, op := range []string{"stats.subscribe", "stats.unsubscribe"} {
		if classes[op] != "rl.read" {
			t.Errorf("%s counts in %q, want rl.read", op, classes[op])
		}
	}
}
