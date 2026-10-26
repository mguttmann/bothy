// SPDX-License-Identifier: Apache-2.0

package wire_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"slices"
	"strconv"
	"syscall"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/gateway/adapters/wstest"
	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/adapters/identitytest"
	"github.com/OWNER/bothy/services/server/internal/identity/domain/dpop"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/jose"
	"github.com/OWNER/bothy/services/server/internal/platform/lifecycle"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

// The windows of the consumer world: short enough for the tests, never the defaults.
const (
	consumerAckTimeout = 2 * time.Second
	consumerPersist    = 300 * time.Millisecond
	consumerInflight   = 3
)

// consumerAgent is an agent with a DPoP-bound API token.
type consumerAgent struct {
	id  ids.ID
	tok string
	key *identitytest.Key
}

// consumerWorld is the delivery world of M1-65 with the windows of the agent consumers (M2-07).
type consumerWorld struct {
	*dlWorld
	space string
}

// newConsumerWorld starts the world with short agent windows and a deny aggregation of a second.
func newConsumerWorld(t *testing.T) *consumerWorld {
	t.Helper()
	return newConsumerWorldWith(t)
}

// newConsumerWorldWith is newConsumerWorld with the further environment entries extra (M2-16).
func newConsumerWorldWith(t *testing.T, extra ...string) *consumerWorld {
	t.Helper()
	return newConsumerWorldDeps(t, wire.Deps{}, extra...)
}

// newConsumerWorldDeps is newConsumerWorldWith with the dependencies deps of the process; a clock
// of deps is the clock of the side identity and of the proofs of call too.
func newConsumerWorldDeps(t *testing.T, deps wire.Deps, extra ...string) *consumerWorld {
	t.Helper()
	env := append([]string{"BOTHY_AGENT__ACK_TIMEOUT=" + consumerAckTimeout.String(),
		"BOTHY_AGENT__ACK_PERSIST_INTERVAL=" + consumerPersist.String(), "BOTHY_AGENT__MAX_INFLIGHT_EVENTS=" + strconv.Itoa(consumerInflight),
		"BOTHY_AUDIT__DENY_AGGREGATE_WINDOW=1s", "BOTHY_SYNC__CURSOR_ADVANCE_EVERY=5",
		// The Owner builds and posts for every subtest; the suite must not depend on its speed.
		"BOTHY_RATELIMIT__RL__MESSAGE__BURST=1000", "BOTHY_RATELIMIT__RL__ADMIN__BURST=1000", "BOTHY_RATELIMIT__RL__WRITE__BURST=1000"},
		extra...)
	w := newDeliveryWorldRole(t, wire.RoleAPI, deps, env...)
	return &consumerWorld{dlWorld: w, space: w.newSpace("Support")}
}

// agent creates support-agent style agent with its record and a DPoP-bound token.
func (w *consumerWorld) agent() consumerAgent {
	w.t.Helper()
	id := w.newAgent()
	key, _ := identitytest.NewDeviceKey(jose.AlgES256)
	return consumerAgent{id: id, tok: w.agentToken(id, key, time.Now().Add(time.Hour)), key: key}
}

// params returns the identify parameters of a for consumer ("" sends none).
func (w *consumerWorld) agentParams(a consumerAgent, consumer string) wstest.IdentifyParams {
	p := wstest.IdentifyParams{APIToken: a.tok, Proof: w.proofFor(a.key, a.tok, gwHTU)}
	if consumer != "" {
		p.Consumer = &consumer
	}
	return p
}

// call sends a REST request as a at base and returns status and body.
func (w *consumerWorld) call(base string, a consumerAgent, method, path string, body any) (int, []byte) {
	w.t.Helper()
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	ctx, cancel := context.WithTimeout(context.Background(), testkit.Patience)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, method, base+path, rd)
	if err != nil {
		w.t.Fatal(err)
	}
	req.Header.Set("Authorization", "DPoP "+a.tok)
	req.Header.Set("DPoP", identitytest.SignDPoPProof(a.key, identitytest.ProofClaims{HTM: method, HTU: gwPublicURL + path,
		IAT: w.clk.Now(), Nonce: w.id.DPoP.CurrentNonce(), ATH: dpop.ATH(a.tok)}))
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		w.t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	b, _ := io.ReadAll(resp.Body)
	if bytes.Contains(b, []byte("null")) {
		w.t.Fatalf("%s %s answered JSON null: %s", method, path, b)
	}
	return resp.StatusCode, b
}

// restPage is the body of pollAgentEvents.
type restPage struct {
	Events     []json.RawMessage `json:"events"`
	NextCursor int64             `json:"next_cursor"`
	HeadSeq    int64             `json:"head_seq"`
}

// poll calls pollAgentEvents of consumer at base and requires 200.
//
// A poll reads up to the hub position, which the log tail raises after a post has returned; so
// for a consumer that exists, poll first waits until the tail has passed the newest event.
func (w *consumerWorld) poll(base string, a consumerAgent, consumer, query string) (restPage, []event) {
	w.t.Helper()
	if code, _ := w.call(base, a, http.MethodGet, "/api/v1/agents/me/consumers/"+consumer, nil); code == http.StatusOK {
		w.tailed(a, consumer)
	}
	code, b := w.call(base, a, http.MethodGet, "/api/v1/agents/me/consumers/"+consumer+"/events"+query, nil)
	if code != http.StatusOK {
		w.t.Fatalf("poll = %d %s", code, b)
	}
	var p restPage
	var page rest.EventPage
	if err := wstest.Strict(b, &page); err != nil {
		w.t.Fatalf("EventPage %s: %v", b, err)
	}
	if err := json.Unmarshal(b, &p); err != nil {
		w.t.Fatal(err)
	}
	evs := make([]event, len(p.Events))
	for i, raw := range p.Events {
		if err := json.Unmarshal(raw, &evs[i]); err != nil {
			w.t.Fatal(err)
		}
	}
	return p, evs
}

// state is the body of getConsumerState and ackAgentEvents.
type state struct {
	Consumer string `json:"consumer"`
	AckedSeq int64  `json:"acked_seq"`
	HeadSeq  int64  `json:"head_seq"`
}

// state reads the stored cursor of consumer through getConsumerState and requires 200. The
// gateway sends ready before it takes the consumer over and writes its row (the hub registers
// first, 04 section 6.3), so a 404 right after a ready is waited out; a consumer that never gets
// a row still fails after Patience.
func (w *consumerWorld) state(a consumerAgent, consumer string) state {
	w.t.Helper()
	var code int
	var b []byte
	testkit.Until(w.t, "the cursor row of "+consumer, func() bool {
		code, b = w.call(w.base, a, http.MethodGet, "/api/v1/agents/me/consumers/"+consumer, nil)
		return code != http.StatusNotFound
	})
	if code != http.StatusOK {
		w.t.Fatalf("getConsumerState = %d %s", code, b)
	}
	var s state
	var strict rest.ConsumerState
	if err := wstest.Strict(b, &strict); err != nil {
		w.t.Fatalf("ConsumerState %s: %v", b, err)
	}
	_ = json.Unmarshal(b, &s)
	return s
}

// row reads the cursor row of a and the consumer default: acked_seq and holder_session_id ("" for
// NULL); ok false without a row.
func (w *consumerWorld) row(a ids.ID) (acked int64, holder string, ok bool) {
	w.t.Helper()
	err := w.queryRow(context.Background(), `SELECT acked_seq, coalesce(holder_session_id::text, '') FROM agent_consumer_cursors
		WHERE agent_id = $1 AND consumer_name = 'default'`, a.String()).Scan(&acked, &holder)
	if err != nil {
		return 0, "", false
	}
	return acked, holder, true
}

// queryRow runs a query as the schema owner.
func (w *consumerWorld) queryRow(_ context.Context, query string, args ...any) pgx.Row {
	w.sql(`SELECT 1`) // opens the owner pool once
	return w.gwWorld.owner.QueryRow(context.Background(), query, args...)
}

// retain removes every event the log tail of the world's process has passed, as the partition
// maintenance of M1-16 drops old partitions after sync.event_retention; an event the tail has not
// read yet stays, so the tail never sees a hole.
func (w *consumerWorld) retain(a consumerAgent) {
	w.t.Helper()
	w.sql(`DELETE FROM event_log WHERE seq <= $1`, w.tailed(a, "default"))
}

// tailed waits until the log tail of the world's process has passed the newest event of the log
// (the head_seq of the existing consumer of a reaches it) and returns that event's seq.
func (w *consumerWorld) tailed(a consumerAgent, consumer string) int64 {
	w.t.Helper()
	var last int64
	if err := w.queryRow(context.Background(), `SELECT coalesce(max(seq), 0) FROM event_log`).Scan(&last); err != nil {
		w.t.Fatal(err)
	}
	testkit.Until(w.t, "the log tail at the newest event", func() bool { return w.state(a, consumer).HeadSeq >= last })
	return last
}

// quiet reads frames of c for d and returns them; the window must stay closed meanwhile.
func quiet(t *testing.T, c *wstest.Client, d time.Duration) []frame {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), d)
	defer cancel()
	var out []frame
	for {
		op, raw, err := c.Next(ctx)
		if errors.Is(err, context.DeadlineExceeded) {
			return out
		}
		if err != nil {
			t.Fatalf("after %v: %v", ops(out), err)
		}
		f := frame{op: op}
		if op == "dispatch" {
			_ = json.Unmarshal(raw, &f.ev)
			f.seq = f.ev.Seq
		}
		out = append(out, f)
	}
}

// fresh returns the frames of fs whose seq is not in seen and adds their seqs to seen, so a
// redelivery of an already dispatched event does not count as a new dispatch.
func fresh(fs []frame, seen map[int64]bool) []frame {
	var out []frame
	for _, f := range fs {
		if !seen[f.seq] {
			seen[f.seq] = true
			out = append(out, f)
		}
	}
	return out
}

// dispatches returns the dispatch frames of fs.
func dispatches(fs []frame) []frame {
	var out []frame
	for _, f := range fs {
		if f.op == "dispatch" {
			out = append(out, f)
		}
	}
	return out
}

// sendAck sends ack {seq} on c.
func sendAck(t *testing.T, c *wstest.Client, seq int64) {
	t.Helper()
	if err := c.Send("ack", map[string]any{"seq": seq}); err != nil {
		t.Fatal(err)
	}
}

// redeliveredFrame reads the flag of a dispatch frame.
type redeliveredFrame struct {
	Seq         int64 `json:"seq"`
	Redelivered bool  `json:"redelivered"`
}

// TestAgentConsumers is the integration suite of M2-07 against a started api process
// (PostgreSQL 18 via pgtest, the agent's DPoP-bound API token, the wstest client and plain HTTP):
// the first connection at head_seq, the continuation at acked_seq + 1, the inflight bound, the
// redelivery after agent.ack_timeout, the takeover with 4015 within one process and across two,
// the save interval and the shutdown flush, the rights filter, the REST operations and the
// unchanged delivery of people.
func TestAgentConsumers(t *testing.T) {
	t.Parallel()
	w := newConsumerWorld(t)

	t.Run("TestConsumerFirstConnectionStartsAtHead", func(t *testing.T) {
		ch := w.newChannel(w.space, "first")
		a := w.agent()
		w.grant(a.id, "agent", "channel", ch.id)
		before := w.post(ch.thread, "before the first connection")
		c, r := w.ready(t, w.agentParams(a, ""))
		fs := readUntil(t, c, func(f frame) bool { return f.op == "catchup.complete" })
		if got := fs[len(fs)-1].seq; got != r.HeadSeq {
			t.Fatalf("catchup.complete %d, ready head %d", got, r.HeadSeq)
		}
		if acked, holder, ok := w.row(a.id); !ok || acked != r.HeadSeq || holder != r.GatewaySessionId.String() {
			t.Fatalf("row %d %q %v, ready %d %s", acked, holder, ok, r.HeadSeq, r.GatewaySessionId)
		}
		after := w.post(ch.thread, "after")
		fs = readUntil(t, c, message(after))
		for _, f := range dispatches(fs) {
			if f.ev.ClientMsgID == before || f.seq <= r.HeadSeq {
				t.Fatalf("an event from before the first connection: %v", ops(fs))
			}
		}
		p, evs := w.poll(w.base, a, "rest-first", "")
		if len(evs) != 0 || p.NextCursor != p.HeadSeq {
			t.Fatalf("first poll %+v", p)
		}
		if s := w.state(a, "rest-first"); s.AckedSeq != p.HeadSeq {
			t.Fatalf("state after the first poll %+v, head %d", s, p.HeadSeq)
		}
	})

	t.Run("TestConsumerContinuesFromAckedSeq", func(t *testing.T) {
		ch := w.newChannel(w.space, "continue")
		a := w.agent()
		w.grant(a.id, "agent", "channel", ch.id)
		c, _ := w.ready(t, w.agentParams(a, "worker"))
		m1, m2, m3 := w.post(ch.thread, "one"), w.post(ch.thread, "two"), w.post(ch.thread, "three")
		fs := dispatches(readUntil(t, c, message(m3)))
		var seq2 int64
		for _, f := range fs {
			if f.ev.ClientMsgID == m2 {
				seq2 = f.seq
			}
		}
		sendAck(t, c, seq2)
		// TestConsumerAckPersistedWithinInterval: visible through getConsumerState within
		// agent.ack_persist_interval plus test slack.
		testkit.Until(t, "the save of the acknowledgement", func() bool { return w.state(a, "worker").AckedSeq == seq2 })
		c.Close()
		m4 := w.post(ch.thread, "four")
		c2, _ := w.ready(t, w.agentParams(a, "worker"))
		got := dispatches(readUntil(t, c2, message(m4)))
		var names []string
		for _, f := range got {
			if f.ev.Type == "message.created" {
				names = append(names, f.ev.ClientMsgID)
			}
		}
		if !slices.Equal(names, []string{m3, m4}) || slices.Contains(names, m1) {
			t.Fatalf("after the reconnect %v, want %v", names, []string{m3, m4})
		}
	})

	t.Run("TestConsumerMaxInflightEvents", func(t *testing.T) {
		ch := w.newChannel(w.space, "inflight")
		a := w.agent()
		w.grant(a.id, "agent", "channel", ch.id)
		c, _ := w.ready(t, w.agentParams(a, ""))
		readUntil(t, c, func(f frame) bool { return f.op == "catchup.complete" })
		var cms []string
		for i := range consumerInflight + 2 {
			cms = append(cms, w.post(ch.thread, "m"+strconv.Itoa(i)))
		}
		// A slow host can pass agent.ack_timeout inside the windows below, and the gateway then
		// redelivers unacknowledged events; only a dispatch of a seq not seen before counts.
		seen := map[int64]bool{}
		fs := fresh(dispatches(readUntil(t, c, message(cms[consumerInflight-1]))), seen)
		more := fresh(dispatches(quiet(t, c, consumerAckTimeout/4)), seen)
		if len(fs) != consumerInflight || len(more) != 0 {
			t.Fatalf("%d dispatches, then %d more while nothing was acknowledged", len(fs), len(more))
		}
		sendAck(t, c, fs[0].seq)
		next := fresh(dispatches(readUntil(t, c, message(cms[consumerInflight]))), seen)
		if extra := fresh(dispatches(quiet(t, c, consumerAckTimeout/8)), seen); len(next) != 1 || len(extra) != 0 {
			t.Fatalf("one acknowledgement released %d and then %d", len(next), len(extra))
		}
		w.poll(w.base, a, "rest-inflight", "")
		for i := range consumerInflight + 2 {
			w.post(ch.thread, "r"+strconv.Itoa(i))
		}
		_, evs := w.poll(w.base, a, "rest-inflight", "?limit=50")
		if len(evs) != consumerInflight {
			t.Fatalf("a poll with a larger limit returned %d events", len(evs))
		}
	})

	t.Run("TestConsumerRedeliveryAfterAckTimeout", func(t *testing.T) {
		ch := w.newChannel(w.space, "redeliver")
		a := w.agent()
		w.grant(a.id, "agent", "channel", ch.id)
		c, _ := w.ready(t, w.agentParams(a, ""))
		readUntil(t, c, func(f frame) bool { return f.op == "catchup.complete" })
		first := w.post(ch.thread, "first")
		var flags []redeliveredFrame
		ctx, cancel := context.WithDeadline(context.Background(), testkit.Deadline())
		defer cancel()
		for len(flags) < 2 {
			op, raw, err := c.Next(ctx)
			if err != nil {
				t.Fatalf("frames %v: %v", flags, err)
			}
			switch op {
			case "dispatch":
				var f redeliveredFrame
				_ = json.Unmarshal(raw, &f)
				var ev rest.Event
				if err := ev.UnmarshalJSON(raw); err != nil || bytes.Contains(raw, []byte("null")) {
					t.Fatalf("dispatch %s: %v", raw, err)
				}
				flags = append(flags, f)
			case "cursor":
				if err := wstest.Strict(raw, &rest.GatewayCursor{}); err != nil {
					t.Fatalf("cursor %s: %v", raw, err)
				}
			}
		}
		if flags[0].Redelivered || !flags[1].Redelivered || flags[0].Seq != flags[1].Seq {
			t.Fatalf("delivery then redelivery %+v of %s", flags, first)
		}
		sendAck(t, c, flags[1].Seq)
		if again := dispatches(quiet(t, c, consumerAckTimeout+time.Second)); len(again) != 0 {
			t.Fatalf("after the acknowledgement %v", ops(again))
		}
		fresh := w.post(ch.thread, "fresh")
		if f := dispatches(readUntil(t, c, message(fresh))); len(f) != 1 {
			t.Fatalf("fresh %v", ops(f))
		}
		var stored int
		if err := w.queryRow(context.Background(), `SELECT count(*) FROM event_log WHERE payload::text LIKE '%redelivered%'`).Scan(&stored); err != nil || stored != 0 {
			t.Fatalf("event_log rows with redelivered: %d %v", stored, err)
		}
	})

	t.Run("TestConsumerSecondIdentifyCloses4015", func(t *testing.T) {
		ch := w.newChannel(w.space, "takeover")
		a := w.agent()
		w.grant(a.id, "agent", "channel", ch.id)
		c1, r1 := w.ready(t, w.agentParams(a, ""))
		other, _ := w.ready(t, w.agentParams(a, "other"))
		m := w.post(ch.thread, "to the first")
		fs := dispatches(readUntil(t, c1, message(m)))
		readUntil(t, other, message(m))
		sendAck(t, c1, fs[len(fs)-1].seq)
		// The engine applies the ack asynchronously; the flush on takeover itself is a unit test
		// (TestConsumerTakeover_FlushTakeCloseOrder).
		testkit.Until(t, "the save of the acknowledgement", func() bool { return w.state(a, "default").AckedSeq == fs[len(fs)-1].seq })
		c2, r2 := w.ready(t, w.agentParams(a, ""))
		if reason := w.expectClose(t, c1, 4015); reason != "consumer_replaced" {
			t.Fatalf("reason %q", reason)
		}
		acked, holder, _ := w.row(a.id)
		if acked != fs[len(fs)-1].seq || holder != r2.GatewaySessionId.String() {
			t.Fatalf("row %d %q, want %d %s", acked, holder, fs[len(fs)-1].seq, r2.GatewaySessionId)
		}
		later := w.post(ch.thread, "to the second")
		readUntil(t, c2, message(later))
		readUntil(t, other, message(later))
		c3 := w.dial(t)
		ctx, cancel := context.WithTimeout(context.Background(), testkit.Patience)
		defer cancel()
		_, err := c3.Resume(ctx, wstest.ResumeParams{SessionID: r1.GatewaySessionId.String(), APIToken: a.tok,
			Proof: w.proofFor(a.key, a.tok, gwHTU), Cursor: r1.HeadSeq})
		var ce *wstest.CloseError
		if !errors.As(err, &ce) || ce.Code != 4006 {
			t.Fatalf("resume of the replaced session: %v", err)
		}
	})

	t.Run("TestConsumerAckValidation", func(t *testing.T) {
		ch := w.newChannel(w.space, "validation")
		a := w.agent()
		w.grant(a.id, "agent", "channel", ch.id)
		c, r := w.ready(t, w.agentParams(a, ""))
		readUntil(t, c, func(f frame) bool { return f.op == "catchup.complete" })
		sendAck(t, c, r.HeadSeq+1_000_000)
		fs := readUntil(t, c, func(f frame) bool { return f.op == "error" })
		_ = fs
		sendAck(t, c, 0)
		m := w.post(ch.thread, "still open")
		readUntil(t, c, message(m))
		code, b := w.call(w.base, a, http.MethodPost, "/api/v1/agents/me/consumers/default/ack", map[string]any{"seq": r.HeadSeq + 1_000_000})
		if code != http.StatusBadRequest || !bytes.Contains(b, []byte(`"/seq"`)) {
			t.Fatalf("ack above the head = %d %s", code, b)
		}
		if code, b = w.call(w.base, a, http.MethodPost, "/api/v1/agents/me/consumers/default/ack", map[string]any{"seq": -1}); code != http.StatusBadRequest {
			t.Fatalf("negative ack = %d %s", code, b)
		}
		if code, b = w.call(w.base, a, http.MethodPost, "/api/v1/agents/me/consumers/never/ack", map[string]any{"seq": 1}); code != http.StatusNotFound {
			t.Fatalf("unknown consumer = %d %s", code, b)
		}
		code, b = w.call(w.base, a, http.MethodPost, "/api/v1/agents/me/consumers/default/ack", map[string]any{"seq": r.HeadSeq})
		var s rest.ConsumerState
		if err := wstest.Strict(b, &s); code != http.StatusOK || err != nil || s.AckedSeq != r.HeadSeq || s.Consumer != "default" {
			t.Fatalf("valid ack = %d %s", code, b)
		}
	})

	t.Run("TestPollTakesOverGatewayConsumer", func(t *testing.T) {
		ch := w.newChannel(w.space, "polltakeover")
		a := w.agent()
		w.grant(a.id, "agent", "channel", ch.id)
		c, _ := w.ready(t, w.agentParams(a, ""))
		keep, _ := w.ready(t, w.agentParams(a, "keep"))
		readUntil(t, c, func(f frame) bool { return f.op == "catchup.complete" })
		w.state(a, "keep")
		if code, b := w.call(w.base, a, http.MethodPost, "/api/v1/agents/me/consumers/keep/ack", map[string]any{"seq": 0}); code != http.StatusOK {
			t.Fatalf("ack = %d %s", code, b)
		}
		m := w.post(ch.thread, "before the poll")
		readUntil(t, keep, message(m))
		w.poll(w.base, a, "default", "")
		w.expectClose(t, c, 4015)
		readUntil(t, keep, message(w.post(ch.thread, "keep is open")))
	})

	t.Run("TestConsumerNameRules", func(t *testing.T) {
		a := w.agent()
		// An invalid consumer name breaks the identify schema on purpose (INT-W42/2).
		w.identifyErr(t, w.agentParams(a, "Not Valid"), 4002, wstest.WithRawClientFrames())
		code, b := w.call(w.base, a, http.MethodGet, "/api/v1/agents/me/consumers/Not_Valid", nil)
		if code != http.StatusBadRequest || !bytes.Contains(b, []byte("validation_failed")) {
			t.Fatalf("invalid path = %d %s", code, b)
		}
		w.ready(t, w.agentParams(a, ""))
		// ready is sent before the consumer is taken over (the hub registers first, 04 section 6.3).
		testkit.Until(t, "the row of default", func() bool {
			_, _, ok := w.row(a.id)
			return ok
		})
	})

	t.Run("TestPeopleAndAppsUnchanged", func(t *testing.T) {
		ch := w.newChannel(w.space, "people")
		sam := w.newPerson("sam", identity.DeviceStateActive)
		w.grant(sam.user, "user", "channel", ch.id)
		c, _ := w.ready(t, w.params(sam))
		readUntil(t, c, message(w.post(ch.thread, "to sam")))
		if err := c.Send("ack", map[string]any{"seq": 1}); err != nil {
			t.Fatal(err)
		}
		readUntil(t, c, func(f frame) bool { return f.op == "error" })
		readUntil(t, c, message(w.post(ch.thread, "sam still open")))
		var n int
		if err := w.queryRow(context.Background(), `SELECT count(*) FROM agent_consumer_cursors WHERE agent_id = $1`, sam.user.String()).Scan(&n); err != nil || n != 0 {
			t.Fatalf("rows of a person %d %v", n, err)
		}
		for _, call := range []struct{ method, path string }{
			{http.MethodGet, "/api/v1/agents/me/consumers/default"},
			{http.MethodGet, "/api/v1/agents/me/consumers/default/events"},
			{http.MethodPost, "/api/v1/agents/me/consumers/default/ack"},
		} {
			var body any
			if call.method == http.MethodPost {
				body = map[string]any{"seq": 1}
			}
			// The resource is the agent itself: a person who is no owner does not see it (404
			// before 403, R-AZ-01); the owner sees it and is refused by agent-self (D-M2-07-12).
			code, b := w.rest(w.base, sam, call.method, call.path, body)
			if code != http.StatusNotFound {
				t.Fatalf("%s %s as a person = %d %s", call.method, call.path, code, b)
			}
			code, b = w.rest(w.base, w.owner, call.method, call.path, body)
			if code != http.StatusForbidden || !bytes.Contains(b, []byte("subject_kind_not_allowed")) {
				t.Fatalf("%s %s as the owner = %d %s", call.method, call.path, code, b)
			}
		}
		// TestConsumerDenialsAudited (AZ-P13): the denied calls write an aggregated deny entry.
		testkit.Until(t, "the aggregated deny entry", func() bool {
			var n int
			_ = w.queryRow(context.Background(), `SELECT count(*) FROM audit_events WHERE action = 'agent.events.consume'
				AND decision = 'deny' AND actor_principal_id = $1`, sam.user.String()).Scan(&n)
			return n > 0
		})
	})

	t.Run("TestConsumerStreamIsFilteredByRights", func(t *testing.T) {
		allgemein := w.newChannel(w.space, "allgemein")
		marketing := w.newChannel(w.newSpace("Marketing"), "kampagnen")
		a := w.agent()
		g := w.grant(a.id, "agent", "channel", allgemein.id)
		c, _ := w.ready(t, w.agentParams(a, ""))
		readUntil(t, c, func(f frame) bool { return f.op == "catchup.complete" })
		hidden := w.post(marketing.thread, "invisible")
		code, b, etag := w.restIfMatch(w.owner, http.MethodGet, "/api/v1/agents/"+a.id.String(), "", nil)
		if code != http.StatusOK {
			t.Fatalf("getAgent = %d %s", code, b)
		}
		if code, b, _ = w.restIfMatch(w.owner, http.MethodPatch, "/api/v1/agents/"+a.id.String(), etag, map[string]any{"description": "Support"}); code != http.StatusOK {
			t.Fatalf("updateAgent = %d %s", code, b)
		}
		visible := w.post(allgemein.thread, "visible")
		fs := readUntil(t, c, message(visible))
		if inChannel(fs, marketing.id) || !slices.ContainsFunc(fs, func(f frame) bool { return f.op == "dispatch" && f.ev.Type == "agent.updated" }) {
			t.Fatalf("frames %v (hidden %s)", ops(fs), hidden)
		}
		var acked int64
		for _, f := range fs {
			if f.op == "dispatch" || f.op == "cursor" {
				acked = max(acked, f.seq)
			}
		}
		sendAck(t, c, acked)
		// Revoked through process B while the agent reads on A (AZ-P9 for consumers); an
		// unacknowledged message of Allgemein is not repeated by the redelivery.
		pending := w.post(allgemein.thread, "unacknowledged")
		readUntil(t, c, message(pending))
		b2 := start(t, wire.RoleAPI, w.env, wire.Deps{})
		waitReadyz(t, b2.waitInternal(t), http.StatusOK)
		bBase := "http://127.0.0.1:" + strconv.Itoa(b2.logs.port("public listener started"))
		if code, body := w.rest(bBase, w.owner, http.MethodPost, "/api/v1/grants/"+g+"/revoke", map[string]any{"reason": "test"}); code != http.StatusOK {
			t.Fatalf("revoke on B = %d %s", code, body)
		}
		readUntil(t, c, func(f frame) bool { return f.op == "dispatch" && f.ev.Type == "access.changed" })
		after := w.post(allgemein.thread, "after the revocation")
		rest := quiet(t, c, consumerAckTimeout+time.Second)
		for _, f := range dispatches(rest) {
			if f.ev.ClientMsgID == after || f.ev.ClientMsgID == pending {
				t.Fatalf("delivered after the revocation: %v", ops(rest))
			}
		}
		b2.sigs <- syscall.SIGTERM
		if exit := b2.exit(t); exit != lifecycle.ExitOK {
			t.Fatalf("exit of B = %d", exit)
		}
		// AZ-P13: the delivery filter of the runner (the Marketing message, the revoked channel)
		// writes no deny entry; the deny aggregation window of the world has passed by now.
		var denied int
		if err := w.queryRow(context.Background(), `SELECT count(*) FROM audit_events WHERE actor_principal_id = $1 AND decision = 'deny'`,
			a.id.String()).Scan(&denied); err != nil || denied != 0 {
			t.Fatalf("deny entries of the agent's delivery: %d %v", denied, err)
		}
	})

	t.Run("TestConsumerAgentDisabledCloses4012", func(t *testing.T) {
		ch := w.newChannel(w.space, "disable")
		a := w.agent()
		w.grant(a.id, "agent", "channel", ch.id)
		c, _ := w.ready(t, w.agentParams(a, ""))
		fs := dispatches(readUntil(t, c, message(w.post(ch.thread, "before"))))
		seq := fs[len(fs)-1].seq
		sendAck(t, c, seq)
		w.must(w.owner, http.MethodPost, "/api/v1/agents/"+a.id.String()+"/disable", map[string]any{"reason": "test"})
		w.expectClose(t, c, 4012)
		testkit.Until(t, "the saved acknowledgement", func() bool {
			acked, _, _ := w.row(a.id)
			return acked == seq
		})
	})

	t.Run("TestPollRetentionGivesGone", func(t *testing.T) {
		ch := w.newChannel(w.space, "retention")
		a := w.agent()
		w.grant(a.id, "agent", "channel", ch.id)
		w.poll(w.base, a, "default", "")
		w.post(ch.thread, "removed by retention")
		w.retain(a)
		code, b := w.call(w.base, a, http.MethodGet, "/api/v1/agents/me/consumers/default/events", nil)
		if code != http.StatusGone || !bytes.Contains(b, []byte(`"retention"`)) {
			t.Fatalf("poll after retention = %d %s", code, b)
		}
		s := w.state(a, "default")
		if s.AckedSeq < s.HeadSeq-1 {
			t.Fatalf("cursor not moved to the head: %+v", s)
		}
		w.poll(w.base, a, "default", "")
		w.post(ch.thread, "gateway retention")
		w.retain(a)
		c, _ := w.ready(t, w.agentParams(a, ""))
		fs := readUntil(t, c, func(f frame) bool { return f.op == "resync_required" })
		_ = fs
		readUntil(t, c, message(w.post(ch.thread, "after resync")))
	})

	// TestConsumerFencedSaveAcrossInstances and TestConsumerShutdownFlushesAck run against a second
	// process B on the same database.
	// B saves only at its shutdown, so the shutdown test proves the flush and not the ticker.
	b := start(t, wire.RoleAPI, withEnv(w.env, "BOTHY_AGENT__ACK_PERSIST_INTERVAL=1h"), wire.Deps{})
	waitReadyz(t, b.waitInternal(t), http.StatusOK)
	bBase := "http://127.0.0.1:" + strconv.Itoa(b.logs.port("public listener started"))
	dialB := func(t *testing.T, p wstest.IdentifyParams) (*wstest.Client, rest.GatewayReady) {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), testkit.Patience)
		defer cancel()
		c, err := wstest.Dial(t, ctx, bBase, wstest.WithOrigin(gwPublicURL), wstest.WithMaxFrameBytes(w.cfg.MaxFrameBytes))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(c.Abort)
		r, err := c.Identify(ctx, p)
		if err != nil {
			t.Fatalf("identify on B: %v", err)
		}
		return c, r
	}

	t.Run("TestConsumerFencedSaveAcrossInstances", func(t *testing.T) {
		ch := w.newChannel(w.space, "fenced")
		a := w.agent()
		w.grant(a.id, "agent", "channel", ch.id)
		onA, _ := w.ready(t, w.agentParams(a, ""))
		m := w.post(ch.thread, "to A")
		fs := dispatches(readUntil(t, onA, message(m)))
		onB, rb := dialB(t, w.agentParams(a, ""))
		readUntil(t, onB, func(f frame) bool { return f.op == "catchup.complete" })
		before, holder, _ := w.row(a.id)
		if holder != rb.GatewaySessionId.String() {
			t.Fatalf("holder %q, want B's %s", holder, rb.GatewaySessionId)
		}
		sendAck(t, onA, fs[len(fs)-1].seq)
		ctx, cancel := context.WithTimeout(context.Background(), testkit.Patience)
		defer cancel()
		code, _, err := onA.ExpectClose(ctx)
		if err != nil || code != 4015 {
			t.Fatalf("A closed with %d %v, want 4015", code, err)
		}
		if acked, _, _ := w.row(a.id); acked != before {
			t.Fatalf("A's late save changed acked_seq from %d to %d", before, acked)
		}
	})

	t.Run("TestConsumerShutdownFlushesAck", func(t *testing.T) {
		ch := w.newChannel(w.space, "shutdown")
		a := w.agent()
		w.grant(a.id, "agent", "channel", ch.id)
		c, _ := dialB(t, w.agentParams(a, ""))
		fs := dispatches(readUntil(t, c, message(w.post(ch.thread, "to B"))))
		seq := fs[len(fs)-1].seq
		sendAck(t, c, seq)
		time.Sleep(200 * time.Millisecond)
		if acked, _, _ := w.row(a.id); acked == seq {
			t.Fatal("B saved before its shutdown; the test would not prove the flush")
		}
		b.sigs <- syscall.SIGTERM
		w.expectClose(t, c, 4013)
		if exit := b.exit(t); exit != lifecycle.ExitOK {
			t.Fatalf("exit = %d; logs:\n%s", exit, b.logs)
		}
		if acked, _, _ := w.row(a.id); acked != seq {
			t.Fatalf("acked_seq %d after the shutdown, want %d", acked, seq)
		}
	})
}

// TestConsumerRESTRedelivery: over REST a poll inside agent.ack_timeout repeats nothing, and a
// poll after it repeats the event with redelivered. The process runs on a shiftClock with a
// timeout (restAckTimeout) beyond testkit.Patience, so the real time between two polls never
// reaches it on a loaded machine; only the move of the clock does. sync.resume_window lies far
// beyond the move, so the REST window survives it.
func TestConsumerRESTRedelivery(t *testing.T) {
	t.Parallel()
	const restAckTimeout = 4 * time.Minute // BOTHY_AGENT__ACK_TIMEOUT below
	clk := &shiftClock{}
	w := newConsumerWorldDeps(t, wire.Deps{Clock: clk}, "BOTHY_AGENT__ACK_TIMEOUT=4m",
		"BOTHY_SYNC__RESUME_WINDOW=1h") // nosemgrep: bothy-go-test-gateway-window -- the REST window must outlive the clock move of the test
	ch := w.newChannel(w.space, "restredeliver")
	a := w.agent()
	w.grant(a.id, "agent", "channel", ch.id)
	w.poll(w.base, a, "rest", "")
	first := w.post(ch.thread, "rest one")
	p, evs := w.poll(w.base, a, "rest", "")
	if len(evs) != 1 || evs[0].ClientMsgID != first || bytes.Contains(p.Events[0], []byte(`"redelivered"`)) {
		t.Fatalf("first poll %s", p.Events)
	}
	if p, again := w.poll(w.base, a, "rest", ""); len(again) != 0 {
		t.Fatalf("a poll inside the timeout repeated %s", p.Events)
	}
	clk.Advance(restAckTimeout)
	p, again := w.poll(w.base, a, "rest", "")
	if len(again) != 1 || again[0].Seq != evs[0].Seq || !bytes.Contains(p.Events[0], []byte(`"redelivered":true`)) {
		t.Fatalf("a poll after the timeout %s", p.Events)
	}
}
