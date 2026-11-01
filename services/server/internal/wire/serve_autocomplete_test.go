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
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/gateway/adapters/wstest"
	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/adapters/identitytest"
	"github.com/OWNER/bothy/services/server/internal/identity/domain/dpop"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
	"github.com/OWNER/bothy/services/server/internal/runs/adapters/runstest"
)

// acWait is command.autocomplete_timeout of the autocomplete world: longer than testkit.Patience,
// so a request that waits for the timeout fails the test instead of passing late. Every answer
// that must come "at once" therefore comes within the patience of the suite, on any machine, and
// no test bounds a time itself.
const acWait = testkit.Patience + time.Minute

// acRequestTimeout is http.request_timeout of the autocomplete worlds, above every acWait
// (cross-check autocomplete_below_request_timeout).
const acRequestTimeout = acWait + time.Minute

// acShortWait is command.autocomplete_timeout of the world of AG-23, the only test that waits for
// the timeout: short, because the test waits for it, and never the default.
const acShortWait = 1500 * time.Millisecond

// acWorld is the command world of M2-16 with the commands of the autocomplete tests (brief M2-17
// "Tests"): Lotse /ticket in the space Support (option ticket with autocomplete, priority with
// static choices), Runner /deploy in the space Engineering (forum projekt-a and text channel
// werkzeuge, tool class host), Assistent /note in Research (personal, holder the Owner). Sam has
// trigger on Support and on werkzeuge; Robin write on Eskalation; the outsider reads nothing.
type acWorld struct {
	*cmdWorld
	ticket, deploy, note string
	runner, assistent    consumerAgent
	werkzeuge, notizen   channel
	forum                string
	forumThread          string
}

// newACWorld starts the autocomplete world with command.autocomplete_timeout acWait.
func newACWorld(t *testing.T, extra ...string) *acWorld {
	t.Helper()
	return newACWorldWait(t, acWait, extra...)
}

// newACWorldWait starts the autocomplete world with command.autocomplete_timeout wait.
func newACWorldWait(t *testing.T, wait time.Duration, extra ...string) *acWorld {
	t.Helper()
	env := []string{"BOTHY_COMMAND__AUTOCOMPLETE_TIMEOUT=" + strconv.FormatInt(wait.Milliseconds(), 10) + "ms",
		"BOTHY_HTTP__REQUEST_TIMEOUT=" + strconv.FormatInt(acRequestTimeout.Milliseconds(), 10) + "ms",
		"BOTHY_SYNC__EVENT_PAYLOAD_MAX_BYTES=" + strconv.Itoa(cmdPayloadMax), "BOTHY_RATELIMIT__RL__READ__BURST=1000",
		"BOTHY_RATELIMIT__RL__GATEWAY_CONNECT__BURST=1000"}
	// The suite must not depend on its own speed; the rate-limit test sets the two classes low.
	for _, k := range []string{"BOTHY_RATELIMIT__RL__TRIGGER__BURST", "BOTHY_RATELIMIT__RL__AGENT_RUNTIME__BURST"} {
		if !slices.ContainsFunc(extra, func(e string) bool { return strings.HasPrefix(e, k+"=") }) {
			env = append(env, k+"=1000")
		}
	}
	cw := newConsumerWorldWith(t, append(env, extra...)...)
	iw := &invocationWorld{consumerWorld: cw, sam: cw.newPerson("sam", identity.DeviceStateActive)}
	w := &cmdWorld{invocationWorld: iw, seen: map[*wstest.Client]int64{}, robin: cw.newPerson("robin", identity.DeviceStateActive),
		outsider: cw.newPerson("robin", identity.DeviceStateActive)}
	w.lotse = w.agent()
	w.allgemein, w.eskalation = w.operatedBy(w.space, "allgemein", w.lotse), w.operatedBy(w.space, "eskalation", w.lotse)
	w.sql(`UPDATE spaces SET default_operator_agent_id = $1 WHERE id = $2`, w.lotse.id.String(), w.space)
	w.samGrant = w.must(w.owner, http.MethodPost, "/api/v1/grants", map[string]any{
		"subject_kind": "user", "subject_id": w.sam.user.String(), "target_kind": "space", "target_id": w.space, "level": "trigger", "reason": "test"})
	w.must(w.owner, http.MethodPost, "/api/v1/grants", map[string]any{
		"subject_kind": "user", "subject_id": w.robin.user.String(), "target_kind": "channel", "target_id": w.eskalation.id, "level": "write", "reason": "test"})
	aw := &acWorld{cmdWorld: w}
	aw.ticket = w.mustRegister(w.lotse, def("ticket", "scope_kind", "space", "scope_id", w.space, "options", []any{
		map[string]any{"name": "ticket", "description": "Ticket number", "type": "string", "required": false, "autocomplete": true},
		map[string]any{"name": "priority", "description": "Priority", "type": "choice", "required": false,
			"choices": []any{map[string]any{"name": "High", "value": "high"}, map[string]any{"name": "Low", "value": "low"}}},
	})).ID
	engineering := w.newSpace("Engineering")
	aw.runner = w.agent()
	aw.werkzeuge = w.operatedBy(engineering, "werkzeuge", aw.runner)
	aw.forum = w.must(w.owner, http.MethodPost, "/api/v1/spaces/"+engineering+"/channels", map[string]any{"name": "projekt-a", "kind": "forum"})
	w.sql(`UPDATE channels SET operator_inherit = false, operator_agent_id = $1 WHERE id = $2`, aw.runner.id.String(), aw.forum)
	w.sql(`UPDATE agents SET tool_class = 'host' WHERE principal_id = $1`, aw.runner.id.String())
	w.sql(`UPDATE spaces SET default_operator_agent_id = $1 WHERE id = $2`, aw.runner.id.String(), engineering)
	w.must(w.owner, http.MethodPost, "/api/v1/grants", map[string]any{
		"subject_kind": "user", "subject_id": w.sam.user.String(), "target_kind": "channel", "target_id": aw.werkzeuge.id, "level": "trigger", "reason": "test"})
	aw.deploy = w.mustRegister(aw.runner, def("deploy", "scope_kind", "space", "scope_id", engineering, "options", []any{
		map[string]any{"name": "target", "description": "Target", "type": "string", "required": false, "autocomplete": true}})).ID
	aw.forumThread = aw.forumPost()
	research := w.newSpace("Research")
	aw.assistent = w.agent()
	aw.notizen = w.operatedBy(research, "notizen", aw.assistent)
	w.sql(`UPDATE spaces SET default_operator_agent_id = $1 WHERE id = $2`, aw.assistent.id.String(), research)
	aw.note = w.mustRegister(aw.assistent, def("note", "scope_kind", "space", "scope_id", research, "options", []any{
		map[string]any{"name": "topic", "description": "Topic", "type": "string", "required": false, "autocomplete": true}})).ID
	w.sql(`UPDATE agents SET billing_mode = 'personal', credential_holder_id = $1 WHERE principal_id = $2`, w.owner.user.String(),
		aw.assistent.id.String())
	return aw
}

// forumPost creates a post in the forum as the Owner and returns its thread.
func (w *acWorld) forumPost() string {
	w.t.Helper()
	code, b := w.rest(w.base, w.owner, http.MethodPost, "/api/v1/channels/"+w.forum+"/threads",
		map[string]any{"title": "Release", "first_message": map[string]any{"client_msg_id": ids.New().String(), "content_md": "a post"}})
	if code != http.StatusCreated && code != http.StatusOK {
		w.t.Fatalf("createThread in the forum = %d %s", code, b)
	}
	var out struct {
		Thread struct {
			ID string `json:"id"`
		} `json:"thread"`
	}
	_ = json.Unmarshal(b, &out)
	if out.Thread.ID == "" {
		w.t.Fatalf("no thread in %s", b)
	}
	return out.Thread.ID
}

// unconnected creates an agent that never opened a gateway connection, operating a new text
// channel of Support, with the command /lookup (option ticket with autocomplete) scoped to that
// channel. Sam may trigger there through his grant on the space. Tests that need "no live
// connection" use it: a connection closed by an earlier test may still be attached for a moment,
// and nothing outside the process tells when it is gone.
func (w *acWorld) unconnected() (consumerAgent, channel, string) {
	w.t.Helper()
	a, ch := w.fresh("lookup")
	id := w.mustRegister(a, def("lookup", "scope_kind", "channel", "scope_id", ch.id, "options", []any{
		map[string]any{"name": "ticket", "description": "Ticket number", "type": "string", "required": false, "autocomplete": true}})).ID
	return a, ch, id
}

// acReply is the answer of one autocompleteCommand.
type acReply struct {
	code int
	body []byte
	hdr  http.Header
	took time.Duration
	err  error
}

// choicesOf decodes the choices of a 200 answer; nil for a body without the member.
func (r acReply) choices() []map[string]string {
	var b struct {
		Choices []map[string]string `json:"choices"`
	}
	if json.Unmarshal(r.body, &b) != nil {
		return nil
	}
	return b.Choices
}

// rawRequest sends POST path with body as the person p or the agent a without failing the test,
// so it may run in a goroutine. No Idempotency-Key: both operations are exempt.
func (w *acWorld) rawRequest(p *gwPerson, a *consumerAgent, path string, body any) acReply {
	const method = http.MethodPost
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	ctx, cancel := context.WithTimeout(context.Background(), testkit.Patience)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, method, w.base+path, rd)
	if err != nil {
		return acReply{err: err}
	}
	tok, key := "", (*identitytest.Key)(nil)
	if a != nil {
		tok, key = a.tok, a.key
	} else {
		tok, key = p.access, p.key
	}
	req.Header.Set("Authorization", "DPoP "+tok)
	req.Header.Set("DPoP", identitytest.SignDPoPProof(key, identitytest.ProofClaims{HTM: method, HTU: gwPublicURL + path,
		IAT: time.Now(), Nonce: w.id.DPoP.CurrentNonce(), ATH: dpop.ATH(tok)}))
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	start := time.Now()
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return acReply{err: err}
	}
	defer func() { _ = resp.Body.Close() }()
	b, _ := io.ReadAll(resp.Body)
	return acReply{code: resp.StatusCode, body: b, hdr: resp.Header, took: time.Since(start)}
}

// ask starts autocompleteCommand of cmd in ch as p in a goroutine.
func (w *acWorld) ask(p gwPerson, ch, cmd string, body map[string]any) <-chan acReply {
	out := make(chan acReply, 1)
	go func() {
		out <- w.rawRequest(&p, nil, "/api/v1/channels/"+ch+"/commands/"+cmd+"/autocomplete", body)
	}()
	return out
}

// askNow runs autocompleteCommand and waits for its answer.
func (w *acWorld) askNow(p gwPerson, ch, cmd string, body map[string]any) acReply {
	w.t.Helper()
	r := <-w.ask(p, ch, cmd, body)
	if r.err != nil {
		w.t.Fatal(r.err)
	}
	if testkit.HasJSONNull(r.body) {
		w.t.Fatalf("autocompleteCommand answered JSON null: %s", r.body)
	}
	return r
}

// answer posts answerAutocomplete for request as a.
func (w *acWorld) answer(a consumerAgent, request string, choices []any) acReply {
	w.t.Helper()
	r := w.rawRequest(nil, &a, "/api/v1/command-autocompletions/"+request+"/answer", map[string]any{"choices": choices})
	if r.err != nil {
		w.t.Fatal(r.err)
	}
	return r
}

// answerAs posts answerAutocomplete for request as the person p.
func (w *acWorld) answerAs(p gwPerson, request string, choices []any) acReply {
	w.t.Helper()
	r := w.rawRequest(&p, nil, "/api/v1/command-autocompletions/"+request+"/answer", map[string]any{"choices": choices})
	if r.err != nil {
		w.t.Fatal(r.err)
	}
	return r
}

// got waits for the answer of out, at most testkit.Patience.
func (w *acWorld) got(out <-chan acReply) acReply {
	w.t.Helper()
	r := testkit.Receive(w.t, "the answer of autocompleteCommand", out)
	if r.err != nil {
		w.t.Fatal(r.err)
	}
	return r
}

// acFrame is a dispatch of command.autocomplete_requested as the tests read it.
type acFrame struct {
	raw               json.RawMessage
	Seq               *int64 `json:"seq"`
	PermissionVersion *int64 `json:"permission_version"`
	Type              string `json:"type"`
	Audience          string `json:"audience"`
	CorrelationID     string `json:"correlation_id"`
	InvocationToken   string `json:"invocation_token"`
	Resource          struct {
		Kind    string `json:"kind"`
		ID      string `json:"id"`
		Version *int64 `json:"version"`
	} `json:"resource"`
	Scope struct {
		SpaceID   string `json:"space_id"`
		ChannelID string `json:"channel_id"`
		ThreadID  string `json:"thread_id"`
	} `json:"scope"`
	Payload struct {
		RequestID string              `json:"request_id"`
		CommandID string              `json:"command_id"`
		ChannelID string              `json:"channel_id"`
		ThreadID  string              `json:"thread_id"`
		UserID    string              `json:"user_id"`
		Option    string              `json:"option"`
		Value     string              `json:"value"`
		Options   []map[string]string `json:"options"`
		ExpiresAt string              `json:"expires_at"`
	} `json:"payload"`
}

// opError is the body of an error frame.
type opError struct {
	Op     string `json:"op"`
	Code   string `json:"code"`
	Detail string `json:"detail"`
	Ref    string `json:"ref"`
}

// nextAC reads frames of c until a dispatch of command.autocomplete_requested or an error frame
// arrives, at most testkit.Patience; durable dispatches are acknowledged on the way. ok is false
// when none came.
func (w *acWorld) nextAC(c *wstest.Client) (acFrame, *opError, bool) {
	w.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), testkit.Patience)
	defer cancel()
	for {
		op, raw, err := c.Next(ctx)
		if errors.Is(err, context.DeadlineExceeded) {
			return acFrame{}, nil, false
		}
		if err != nil {
			w.t.Fatal(err)
		}
		switch op {
		case "error":
			var e opError
			_ = json.Unmarshal(raw, &e)
			return acFrame{}, &e, true
		case "dispatch":
		default:
			continue
		}
		var f acFrame
		if err := json.Unmarshal(raw, &f); err != nil {
			w.t.Fatal(err)
		}
		if f.Type != "command.autocomplete_requested" {
			if f.Seq != nil && *f.Seq > 0 {
				sendAck(w.t, c, *f.Seq)
			}
			continue
		}
		f.raw = raw
		w.remember(f.InvocationToken)
		return f, nil, true
	}
}

// mustAC requires a dispatch of command.autocomplete_requested on c.
func (w *acWorld) mustAC(c *wstest.Client) acFrame {
	w.t.Helper()
	f, e, ok := w.nextAC(c)
	if !ok || e != nil {
		w.t.Fatalf("no command.autocomplete_requested (error %+v)", e)
	}
	return f
}

// noAC requires that the agent connection c got no command.autocomplete_requested and no error
// frame so far. It is proven by order, without a quiet window: c sends command.autocomplete_result
// for an unknown request with a fresh ref, and its error not_found (answered before any rights
// check, D-M2-17-8) must be the next of these frames. The connection's outgoing queue is first in,
// first out, and the gateway handles the frames of one connection in order, so a frame queued by
// an earlier request or op would arrive before the marker.
func (w *acWorld) noAC(c *wstest.Client) {
	w.t.Helper()
	ref := "marker-" + ids.New().String()
	sendResult(w.t, c, ids.New().String(), []any{}, ref)
	f, e, ok := w.nextAC(c)
	if !ok || e == nil || e.Ref != ref || e.Code != "not_found" {
		w.t.Fatalf("frame before the marker: %s %+v", f.raw, e)
	}
}

// noACPerson is noAC for the connection of a person, which answers no autocomplete: the marker is
// typing.start on an unknown thread, answered with error not_found.
func (w *acWorld) noACPerson(c *wstest.Client) {
	w.t.Helper()
	if err := c.Send("typing.start", map[string]any{"thread_id": ids.New().String()}); err != nil {
		w.t.Fatal(err)
	}
	// A person acknowledges nothing (ack is an agent op), so the frames are read here.
	ctx, cancel := context.WithTimeout(context.Background(), testkit.Patience)
	defer cancel()
	for {
		op, raw, err := c.Next(ctx)
		if err != nil {
			w.t.Fatalf("no marker: %v", err)
		}
		switch op {
		case "error":
			var e opError
			_ = json.Unmarshal(raw, &e)
			if e.Op != "typing.start" || e.Code != "not_found" {
				w.t.Fatalf("error frame before the marker: %+v", e)
			}
			return
		case "dispatch":
			var f acFrame
			_ = json.Unmarshal(raw, &f)
			if f.Type == "command.autocomplete_requested" {
				w.t.Fatalf("a person got %s", raw)
			}
		}
	}
}

// connectAs opens consumer of a and reads until catchup.complete, acknowledging the backlog.
func (w *acWorld) connectAs(t *testing.T, a consumerAgent, consumer string) *wstest.Client {
	t.Helper()
	c, _ := w.ready(t, w.agentParams(a, consumer))
	w.catchUp(t, c)
	return c
}

// catchUp reads c until catchup.complete and acknowledges every dispatch on the way.
func (w *acWorld) catchUp(t *testing.T, c *wstest.Client) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), testkit.Patience)
	defer cancel()
	for {
		op, raw, err := c.Next(ctx)
		if err != nil {
			t.Fatal(err)
		}
		switch op {
		case "catchup.complete":
			return
		case "dispatch":
			var f struct {
				Seq int64 `json:"seq"`
			}
			_ = json.Unmarshal(raw, &f)
			if f.Seq > 0 {
				sendAck(t, c, f.Seq)
			}
		}
	}
}

// writerOn creates a person with write, not trigger, on ch: autocomplete there is denied with
// level_insufficient and audited.
func (w *acWorld) writerOn(ch channel) gwPerson {
	w.t.Helper()
	p := w.newPerson("robin", identity.DeviceStateActive)
	w.must(w.owner, http.MethodPost, "/api/v1/grants", map[string]any{"subject_kind": "user", "subject_id": p.user.String(),
		"target_kind": "channel", "target_id": ch.id, "level": "write", "reason": "test"})
	return p
}

// sendResult sends command.autocomplete_result on c.
func sendResult(t *testing.T, c *wstest.Client, request string, choices []any, ref string) {
	t.Helper()
	d := map[string]any{"request_id": request, "choices": choices}
	if ref != "" {
		d["ref"] = ref
	}
	if err := c.Send("command.autocomplete_result", d); err != nil {
		t.Fatal(err)
	}
}

// choice is one CommandChoice.
func choice(name, value string) map[string]any { return map[string]any{"name": name, "value": value} }

// noViolations requires that every frame c saw matched its schema (G9).
func noViolations(t *testing.T, cs ...*wstest.Client) {
	t.Helper()
	for _, c := range cs {
		if v := c.Violations(); len(v) != 0 {
			t.Fatalf("frames outside the contract: %v", v)
		}
	}
}

// wantEmpty requires 200 with an empty list.
func wantEmpty(t *testing.T, r acReply) {
	t.Helper()
	if r.code != http.StatusOK || string(bytes.TrimSpace(r.body)) != `{"choices":[]}` {
		t.Fatalf("got %d %s, want 200 with an empty list", r.code, r.body)
	}
	wantSchema(t, "AutocompleteResult", r.body)
}

// TestCommandAutocomplete is the integration suite of M2-17 against a started api process: the
// relay of command.autocomplete_requested to one live connection of the owner, the answers over the
// op and REST, the rights of the request (I2, SR2, SR4, read-only, archived) and of the answer, and
// that nothing is stored.
func TestCommandAutocomplete(t *testing.T) {
	t.Parallel()
	w := newACWorld(t)
	q47 := map[string]any{"option": "ticket", "value": "47"}

	t.Run("TestAutocomplete_AnswerOverGateway", func(t *testing.T) {
		c := w.connectAs(t, w.lotse, "")
		defer c.Close()
		out := w.ask(w.sam, w.allgemein.id, w.ticket, q47)
		f := w.mustAC(c)
		if f.Seq != nil || f.PermissionVersion != nil || f.Audience != "self" || f.Resource.Kind != "command" || f.Resource.ID != w.ticket ||
			f.Resource.Version != nil || f.Scope.SpaceID != w.space || f.Scope.ChannelID != w.allgemein.id ||
			f.Scope.ThreadID != w.allgemein.thread || f.CorrelationID == "" {
			t.Fatalf("envelope %s", f.raw)
		}
		p := f.Payload
		if p.RequestID == "" || p.CommandID != w.ticket || p.ChannelID != w.allgemein.id || p.ThreadID != w.allgemein.thread ||
			p.UserID != w.sam.user.String() || p.Option != "ticket" || p.Value != "47" || p.ExpiresAt == "" || p.Options != nil {
			t.Fatalf("payload %s", f.raw)
		}
		cl, err := w.verifier(w.lotse).Check(context.Background(), f.InvocationToken, runstest.ReferenceExpectation{
			CommandID: ids.MustParse(w.ticket), ThreadID: ids.MustParse(w.allgemein.thread)})
		if err != nil || cl.TriggerKind != "autocomplete" || cl.RunID != "" || cl.CommandID != w.ticket || cl.ThreadID != p.ThreadID ||
			cl.ChannelID != p.ChannelID || cl.SpaceID != w.space || cl.Level != "trigger" || cl.BillingMode != "organization" ||
			cl.ToolClass != "chat" || cl.Aud != w.lotse.id.String() || cl.Sub != w.sam.user.String() {
			t.Fatalf("claims %+v %v", cl, err)
		}
		sendResult(t, c, p.RequestID, []any{choice("Ticket 47", "47"), choice("Ticket 470", "470")}, "")
		r := w.got(out)
		if r.code != http.StatusOK || r.took >= acWait {
			t.Fatalf("got %d %s after %s", r.code, r.body, r.took)
		}
		wantSchema(t, "AutocompleteResult", r.body)
		if ch := r.choices(); len(ch) != 2 || ch[0]["name"] != "Ticket 47" || ch[1]["value"] != "470" {
			t.Fatalf("choices %s", r.body)
		}
		w.noAC(c)
		noViolations(t, c)
	})

	t.Run("TestAutocomplete_AnswerOverREST", func(t *testing.T) {
		c := w.connectAs(t, w.lotse, "")
		defer c.Close()
		out := w.ask(w.sam, w.allgemein.id, w.ticket, map[string]any{"option": "ticket", "value": "4", "options": opts("priority", "hi")})
		f := w.mustAC(c)
		if len(f.Payload.Options) != 1 || f.Payload.Options[0]["name"] != "priority" || f.Payload.Options[0]["value"] != "hi" {
			t.Fatalf("options %s", f.raw)
		}
		if a := w.answer(w.lotse, f.Payload.RequestID, []any{choice("Four", "4")}); a.code != http.StatusNoContent {
			t.Fatalf("answerAutocomplete = %d %s", a.code, a.body)
		}
		r := w.got(out)
		if ch := r.choices(); r.code != http.StatusOK || len(ch) != 1 || ch[0]["value"] != "4" {
			t.Fatalf("got %d %s", r.code, r.body)
		}
		noViolations(t, c)
	})

	t.Run("TestAutocomplete_NoLiveConnectionEmptyAtOnce", func(t *testing.T) {
		// acWait is longer than testkit.Patience: an answer within the patience did not wait.
		a, ch, cmd := w.unconnected()
		r := w.askNow(w.sam, ch.id, cmd, q47)
		wantEmpty(t, r)
		if r.took >= acWait {
			t.Fatalf("without a connection the answer took %s", r.took)
		}
		// Connected but still catching up: a backlog above agent.max_inflight_events that the
		// client does not acknowledge keeps the consumer window before catchup.complete. A first
		// connection creates the cursor of consumer default; the second takes the consumer over
		// (fencing), so the first is no longer streaming even while it is still attached.
		first := w.connectAs(t, a, "")
		first.Close()
		for range 3 * consumerInflight {
			w.post(ch.thread, "filler")
		}
		c, _ := w.ready(t, w.agentParams(a, ""))
		defer c.Close()
		r = w.askNow(w.sam, ch.id, cmd, q47)
		wantEmpty(t, r)
		if r.took >= acWait {
			t.Fatalf("while catching up the answer took %s", r.took)
		}
		w.catchUp(t, c)
	})

	t.Run("TestAutocomplete_ExactlyOneConnection", func(t *testing.T) {
		dflt := w.connectAs(t, w.lotse, "")
		audit := w.connectAs(t, w.lotse, "audit")
		samC, _ := w.ready(t, w.params(w.sam))
		ownerC, _ := w.ready(t, w.params(w.owner))
		defer func() {
			for _, c := range []*wstest.Client{dflt, audit, samC, ownerC} {
				c.Close()
			}
		}()
		out := w.ask(w.sam, w.allgemein.id, w.ticket, q47)
		f := w.mustAC(dflt)
		sendResult(t, dflt, f.Payload.RequestID, []any{}, "")
		w.got(out)
		w.noAC(audit)
		w.noACPerson(samC)
		w.noACPerson(ownerC)
		// Without default only the newer of two other consumers gets it. An agent that never had a
		// default connection, so no closed connection can still be attached.
		a, ch, cmd := w.unconnected()
		older := w.connectAs(t, a, "older")
		defer older.Close()
		newer := w.connectAs(t, a, "newer")
		defer newer.Close()
		out = w.ask(w.sam, ch.id, cmd, q47)
		f = w.mustAC(newer)
		sendResult(t, newer, f.Payload.RequestID, []any{}, "")
		w.got(out)
		w.noAC(older)
		noViolations(t, newer, older, audit)
	})

	t.Run("TestAutocomplete_I2_PersonalAgent", func(t *testing.T) {
		c := w.connectAs(t, w.assistent, "")
		defer c.Close()
		out := w.ask(w.owner, w.notizen.id, w.note, map[string]any{"option": "topic", "value": "pl"})
		f := w.mustAC(c)
		cl, err := w.verifier(w.assistent).Check(context.Background(), f.InvocationToken, runstest.ReferenceExpectation{
			CommandID: ids.MustParse(w.note), ThreadID: ids.MustParse(f.Payload.ThreadID)})
		if err != nil || cl.BillingMode != "personal" || cl.Sub != w.owner.user.String() {
			t.Fatalf("claims %+v %v", cl, err)
		}
		sendResult(t, c, f.Payload.RequestID, []any{}, "")
		w.got(out)
		// An I1 defect simulated in the database: Lotse personal for the Owner while Sam keeps his grant.
		lc := w.connectAs(t, w.lotse, "")
		defer lc.Close()
		w.sql(`UPDATE agents SET billing_mode = 'personal', credential_holder_id = $1 WHERE principal_id = $2`, w.owner.user.String(),
			w.lotse.id.String())
		defer w.sql(`UPDATE agents SET billing_mode = 'organization', credential_holder_id = NULL WHERE principal_id = $1`, w.lotse.id.String())
		r := w.askNow(w.sam, w.allgemein.id, w.ticket, q47)
		wantProblem(t, r.code, r.body, http.StatusForbidden, "agent_invocation_denied", "billing_mode_personal_not_holder", "", "")
		w.noAC(lc)
		w.denials(t, `action = 'command.autocomplete' AND deny_reason = 'billing_mode_personal_not_holder' AND actor_principal_id = $1`,
			w.sam.user.String())
	})

	t.Run("TestAutocomplete_ActiveRunDoesNotBlock", func(t *testing.T) {
		th := w.thread(w.allgemein)
		run := w.mention(w.sam, th, w.lotse)
		c := w.connectAs(t, w.lotse, "")
		defer c.Close()
		out := w.ask(w.sam, w.allgemein.id, w.ticket, map[string]any{"option": "ticket", "value": "4", "thread_id": th})
		f := w.mustAC(c)
		// The delivery of the run's invocation happened on the way; the run as it stands now.
		run, _ = w.runOf(w.lastTrigger(run.id))
		cl, err := w.verifier(w.lotse).Check(context.Background(), f.InvocationToken, runstest.ReferenceExpectation{
			CommandID: ids.MustParse(w.ticket), ThreadID: ids.MustParse(th)})
		if err != nil || f.Payload.ThreadID != th || cl.ThreadID != th || cl.RunID != "" {
			t.Fatalf("thread %s claims %+v %v", f.raw, cl, err)
		}
		sendResult(t, c, f.Payload.RequestID, []any{}, "")
		w.got(out)
		if r2, _ := w.runOf(w.lastTrigger(run.id)); r2.id != run.id || r2.version != run.version || r2.state != run.state || r2.jti != run.jti {
			t.Fatalf("the run changed: %+v, was %+v", r2, run)
		}
	})

	t.Run("TestAutocomplete_SR2_HostRunner", func(t *testing.T) {
		c := w.connectAs(t, w.runner, "")
		defer c.Close()
		r := w.askNow(w.sam, w.werkzeuge.id, w.deploy, map[string]any{"option": "target", "value": "pr"})
		wantProblem(t, r.code, r.body, http.StatusForbidden, "agent_invocation_denied", "tool_class_owner_only", "", "")
		w.noAC(c)
		out := w.ask(w.owner, w.werkzeuge.id, w.deploy, map[string]any{"option": "target", "value": "pr"})
		f := w.mustAC(c)
		cl, err := w.verifier(w.runner).Check(context.Background(), f.InvocationToken, runstest.ReferenceExpectation{
			CommandID: ids.MustParse(w.deploy), ThreadID: ids.MustParse(w.werkzeuge.thread)})
		if err != nil || cl.ToolClass != "host" {
			t.Fatalf("claims %+v %v", cl, err)
		}
		sendResult(t, c, f.Payload.RequestID, []any{}, "")
		w.got(out)
	})

	t.Run("TestAutocomplete_ForumWithoutThread", func(t *testing.T) {
		c := w.connectAs(t, w.runner, "")
		defer c.Close()
		r := w.askNow(w.owner, w.forum, w.deploy, map[string]any{"option": "target", "value": "pr"})
		wantEmpty(t, r)
		if r.took >= acWait {
			t.Fatalf("the forum answer took %s", r.took)
		}
		w.noAC(c)
		unknown := w.askNow(w.outsider, ids.New().String(), w.deploy, map[string]any{"option": "target", "value": "pr"})
		denied := w.askNow(w.outsider, w.forum, w.deploy, map[string]any{"option": "target", "value": "pr"})
		if denied.code != http.StatusNotFound || !sameProblem(denied.body, unknown.body) {
			t.Fatalf("outsider in the forum %d %s, unknown channel %d %s", denied.code, denied.body, unknown.code, unknown.body)
		}
		out := w.ask(w.owner, w.forum, w.deploy, map[string]any{"option": "target", "value": "pr", "thread_id": w.forumThread})
		f := w.mustAC(c)
		if f.Payload.ThreadID != w.forumThread || f.Scope.ThreadID != w.forumThread {
			t.Fatalf("thread %s", f.raw)
		}
		sendResult(t, c, f.Payload.RequestID, []any{}, "")
		w.got(out)
	})

	t.Run("TestAutocomplete_Rights", func(t *testing.T) {
		outsider := w.askNow(w.outsider, w.allgemein.id, w.ticket, q47)
		unknownChannel := w.askNow(w.outsider, ids.New().String(), w.ticket, q47)
		unknownCommand := w.askNow(w.sam, w.allgemein.id, ids.New().String(), q47)
		for _, r := range []acReply{outsider, unknownChannel, unknownCommand} {
			if r.code != http.StatusNotFound || !sameProblem(r.body, outsider.body) || r.hdr.Get("Content-Type") != outsider.hdr.Get("Content-Type") {
				t.Fatalf("404 differs: %d %s against %s", r.code, r.body, outsider.body)
			}
		}
		// The times of AZ-P4 are compared in TestCommandAutocompleteNotFoundTiming.
		r := w.askNow(w.robin, w.eskalation.id, w.ticket, q47)
		wantProblem(t, r.code, r.body, http.StatusForbidden, "agent_invocation_denied", "level_insufficient", "", "")
		// An agent never triggers (SR4).
		ar := w.rawRequest(nil, &w.lotse, "/api/v1/channels/"+w.allgemein.id+"/commands/"+w.ticket+"/autocomplete", q47)
		wantProblem(t, ar.code, ar.body, http.StatusForbidden, "agent_invocation_denied", "level_insufficient", "", "")
		w.sql(`UPDATE channels SET read_only = true WHERE id = $1`, w.allgemein.id)
		r = w.askNow(w.owner, w.allgemein.id, w.ticket, q47)
		w.sql(`UPDATE channels SET read_only = false WHERE id = $1`, w.allgemein.id)
		wantProblem(t, r.code, r.body, http.StatusForbidden, "channel_read_only", "channel_read_only", "", "")
		arch := w.operatedBy(w.space, "archiv", w.lotse)
		w.must(w.owner, http.MethodPost, "/api/v1/channels/"+arch.id+"/archive", map[string]any{"reason": "test"})
		r = w.askNow(w.owner, arch.id, w.ticket, q47)
		wantProblem(t, r.code, r.body, http.StatusForbidden, "channel_read_only", "channel_archived", "", "")
	})

	t.Run("TestAutocomplete_UnusableCommand", func(t *testing.T) {
		unknown := w.askNow(w.sam, w.allgemein.id, ids.New().String(), q47)
		gone := w.mustRegister(w.lotse, def("gone", "options", []any{map[string]any{"name": "ticket", "description": "T", "type": "string", "required": false,
			"autocomplete": true}}))
		if code, b, _ := w.do(send{agent: &w.lotse, method: http.MethodDelete, path: "/api/v1/commands/" + gone.ID}); code != http.StatusNoContent {
			t.Fatalf("removeCommand = %d %s", code, b)
		}
		removed := w.askNow(w.sam, w.allgemein.id, gone.ID, q47)
		other := w.askNow(w.sam, w.allgemein.id, w.deploy, map[string]any{"option": "target", "value": "x"})
		foreign := w.askNow(w.sam, w.allgemein.id, w.ticket, map[string]any{"option": "ticket", "value": "4", "thread_id": w.eskalation.thread})
		for _, r := range []acReply{removed, other, foreign} {
			if r.code != http.StatusNotFound || !sameProblem(r.body, unknown.body) {
				t.Fatalf("got %d %s, want %s", r.code, r.body, unknown.body)
			}
		}
	})

	t.Run("TestAutocomplete_Validation", func(t *testing.T) {
		for _, c := range []struct {
			body          map[string]any
			pointer, code string
		}{
			{map[string]any{"option": "nope", "value": "x"}, "/option", "unknown_option"},
			{map[string]any{"option": "priority", "value": "x"}, "/option", "autocomplete_not_enabled"},
			{map[string]any{"option": "ticket", "value": "x", "options": opts("nope", "x")}, "/options/0/name", "unknown_option"},
			{map[string]any{"option": "ticket", "value": "x", "options": opts("priority", "a", "priority", "b")}, "/options/1/name", "duplicate"},
			{map[string]any{"option": "ticket", "value": "x", "options": opts("ticket", "a")}, "/options/0/name", "duplicate"},
			{map[string]any{"option": "ticket", "value": "x", "extra": true}, "/extra", "unknown_member"},
			{map[string]any{"option": "ticket", "value": strings.Repeat("a", cmdPayloadMax)}, "", "too_long"},
		} {
			r := w.askNow(w.sam, w.allgemein.id, w.ticket, c.body)
			wantProblem(t, r.code, r.body, http.StatusBadRequest, "validation_failed", "", c.pointer, c.code)
		}
	})

	t.Run("TestAnswerAutocomplete_Rules", func(t *testing.T) {
		if a := w.answer(w.lotse, ids.New().String(), []any{}); a.code != http.StatusNotFound {
			t.Fatalf("unknown request = %d %s", a.code, a.body)
		}
		c := w.connectAs(t, w.lotse, "")
		defer c.Close()
		out := w.ask(w.sam, w.allgemein.id, w.ticket, q47)
		f := w.mustAC(c)
		req := f.Payload.RequestID
		texter := w.agent()
		if a := w.answer(texter, req, []any{}); a.code != http.StatusNotFound {
			t.Fatalf("Texter without read = %d %s", a.code, a.body)
		}
		w.agentGrant(texter, w.allgemein, "read")
		a := w.answer(texter, req, []any{})
		wantProblem(t, a.code, a.body, http.StatusForbidden, "forbidden", "not_self", "", "")
		a = w.answerAs(w.sam, req, []any{})
		wantProblem(t, a.code, a.body, http.StatusForbidden, "forbidden", "subject_kind_not_allowed", "", "")
		many := make([]any, 26)
		for i := range many {
			many[i] = choice("c"+strconv.Itoa(i), "v"+strconv.Itoa(i))
		}
		a = w.answer(w.lotse, req, many)
		if a.code != http.StatusBadRequest || !bytes.Contains(a.body, []byte(`"too_many_items"`)) {
			t.Fatalf("too many = %d %s", a.code, a.body)
		}
		a = w.answer(w.lotse, req, []any{choice("bad\u0007name", "x")})
		wantProblem(t, a.code, a.body, http.StatusBadRequest, "validation_failed", "", "/choices/0/name", "control_character")
		// The op variants answer error frames with the same codes and keep the connection.
		sendResult(t, c, req, []any{choice("bad\u0007name", "x")}, "r1")
		if _, e, ok := w.nextAC(c); !ok || e == nil || e.Code != "validation_failed" || e.Ref != "r1" ||
			e.Detail != "/choices/0/name control_character" {
			t.Fatalf("op error %+v", e)
		}
		if a := w.answer(w.lotse, req, []any{choice("First", "1")}); a.code != http.StatusNoContent {
			t.Fatalf("a corrected answer = %d %s", a.code, a.body)
		}
		if a := w.answer(w.lotse, req, []any{choice("Second", "2")}); a.code != http.StatusNoContent {
			t.Fatalf("a second answer = %d %s", a.code, a.body)
		}
		sendResult(t, c, req, []any{choice("Third", "3")}, "r2")
		w.noAC(c)
		r := w.got(out)
		if ch := r.choices(); len(ch) != 1 || ch[0]["value"] != "1" {
			t.Fatalf("Sam got %s", r.body)
		}
		w.denials(t, `action = 'command.autocomplete.respond' AND deny_reason = 'not_self' AND actor_principal_id = $1`, texter.id.String())
		noViolations(t, c)
	})

	t.Run("TestAutocomplete_ConcurrentAnswers", func(t *testing.T) {
		c := w.connectAs(t, w.lotse, "")
		defer c.Close()
		out := w.ask(w.sam, w.allgemein.id, w.ticket, q47)
		f := w.mustAC(c)
		var wg sync.WaitGroup
		var rest acReply
		wg.Add(2)
		go func() { defer wg.Done(); sendResult(t, c, f.Payload.RequestID, []any{choice("Op", "op")}, "") }()
		go func() {
			defer wg.Done()
			rest = w.rawRequest(nil, &w.lotse, "/api/v1/command-autocompletions/"+f.Payload.RequestID+"/answer",
				map[string]any{"choices": []any{choice("Rest", "rest")}})
		}()
		wg.Wait()
		r := w.got(out)
		ch := r.choices()
		if rest.code != http.StatusNoContent || len(ch) != 1 || (ch[0]["value"] != "op" && ch[0]["value"] != "rest") {
			t.Fatalf("REST %d %s, Sam %s", rest.code, rest.body, r.body)
		}
		w.noAC(c)
	})

	t.Run("TestAutocomplete_NothingStored", func(t *testing.T) {
		secret := "ACSECRET" + ids.New().String()[24:]
		choiceSecret := "ACCHOICE" + ids.New().String()[24:]
		// marker's denial is written last; once it is there, the entries of the requests before it
		// would be too (instead of a quiet window).
		marker := w.writerOn(w.eskalation)
		before := map[string]int{
			"runs": w.count(`SELECT count(*) FROM runs`), "steps": w.count(`SELECT count(*) FROM run_steps`),
			"events": w.count(`SELECT count(*) FROM event_log`),
			"allows": w.count(`SELECT count(*) FROM audit_events WHERE action LIKE 'command.autocomplete%' AND decision = 'allow'`),
		}
		c := w.connectAs(t, w.lotse, "")
		defer c.Close()
		body := map[string]any{"option": "ticket", "value": secret, "options": opts("priority", secret)}
		out := w.ask(w.sam, w.allgemein.id, w.ticket, body)
		f := w.mustAC(c)
		sendResult(t, c, f.Payload.RequestID, []any{choice(choiceSecret, choiceSecret)}, "")
		w.got(out)
		out = w.ask(w.sam, w.allgemein.id, w.ticket, body)
		f2 := w.mustAC(c)
		if a := w.answer(w.lotse, f2.Payload.RequestID, []any{}); a.code != http.StatusNoContent {
			t.Fatalf("answerAutocomplete = %d %s", a.code, a.body)
		}
		wantEmpty(t, w.got(out))
		_ = w.askNow(w.robin, w.eskalation.id, w.ticket, body)
		_ = w.askNow(marker, w.eskalation.id, w.ticket, body)
		w.denials(t, `action = 'command.autocomplete' AND actor_principal_id = $1`, marker.user.String())
		after := map[string]int{
			"runs": w.count(`SELECT count(*) FROM runs`), "steps": w.count(`SELECT count(*) FROM run_steps`),
			"events": w.count(`SELECT count(*) FROM event_log`),
			"allows": w.count(`SELECT count(*) FROM audit_events WHERE action LIKE 'command.autocomplete%' AND decision = 'allow'`),
		}
		for k, v := range before {
			if after[k] != v {
				t.Errorf("%s: %d before, %d after", k, v, after[k])
			}
		}
		for _, s := range []string{secret, choiceSecret, f.InvocationToken, f2.InvocationToken} {
			if strings.Contains(w.p.logs.String(), s) {
				t.Fatal("a partial text, a choice or a token reached the log")
			}
			if w.count(`SELECT count(*) FROM audit_events WHERE details::text LIKE '%' || $1 || '%'`, s) != 0 ||
				w.count(`SELECT count(*) FROM event_log WHERE payload::text LIKE '%' || $1 || '%'`, s) != 0 {
				t.Fatal("a partial text, a choice or a token was stored")
			}
		}
	})

	t.Run("TestAutocomplete_Registered", func(t *testing.T) {
		// Without Idempotency-Key both operations pass the chain (x-idempotency-exempt).
		_, ch, cmd := w.unconnected()
		r := w.askNow(w.sam, ch.id, cmd, q47)
		wantEmpty(t, r)
		if a := w.answer(w.lotse, ids.New().String(), []any{}); a.code != http.StatusNotFound {
			t.Fatalf("answer without Idempotency-Key = %d %s", a.code, a.body)
		}
	})

	t.Run("TestAutocomplete_DenialsAudited", func(t *testing.T) {
		writer := w.writerOn(w.eskalation)
		before := w.count(`SELECT count(*) FROM audit_events WHERE action LIKE 'command.autocomplete%'`)
		c := w.connectAs(t, w.lotse, "")
		defer c.Close()
		out := w.ask(w.sam, w.allgemein.id, w.ticket, q47)
		f := w.mustAC(c)
		sendResult(t, c, f.Payload.RequestID, []any{}, "")
		w.got(out)
		// The denial is written after the allowed request and its answer; once it is there, an entry
		// of the allowed request would be too (instead of a quiet window).
		r := w.askNow(writer, w.eskalation.id, w.ticket, q47)
		wantProblem(t, r.code, r.body, http.StatusForbidden, "agent_invocation_denied", "level_insufficient", "", "")
		w.denials(t, `action = 'command.autocomplete' AND deny_reason = 'level_insufficient' AND actor_principal_id = $1`, writer.user.String())
		if n := w.count(`SELECT count(*) FROM audit_events WHERE action LIKE 'command.autocomplete%'`); n != before+1 {
			t.Fatalf("an allowed request or the relay wrote %d audit entries", n-before-1)
		}
	})
}

// TestCommandAutocompleteTimeout is AG-23 against a started api process with a short
// command.autocomplete_timeout: nobody answers, the person gets an empty list not before the
// timeout, a late answer over REST and over the op learns that it came too late, and the
// connection stays.
func TestCommandAutocompleteTimeout(t *testing.T) {
	t.Parallel()
	w := newACWorldWait(t, acShortWait)
	q47 := map[string]any{"option": "ticket", "value": "47"}
	c := w.connectAs(t, w.lotse, "")
	defer c.Close()
	out := w.ask(w.sam, w.allgemein.id, w.ticket, q47)
	f := w.mustAC(c)
	r := w.got(out)
	wantEmpty(t, r)
	// Not earlier than the timeout; how much later depends on the machine and is not bounded.
	if r.took < acShortWait {
		t.Fatalf("empty list after %s, before command.autocomplete_timeout (%s)", r.took, acShortWait)
	}
	// The request is remembered until expires_at plus command.autocomplete_timeout: a late answer
	// within that window gets 410; after it the request is forgotten and the answer gets 404
	// (D-M2-17-7). A slow machine may push the answer past the window, so the expected code
	// follows the time of the answer.
	expires, err := time.Parse(time.RFC3339Nano, f.Payload.ExpiresAt)
	if err != nil {
		t.Fatal(err)
	}
	// lateOK reports whether code fits an answer sent at sent: 410 while the window is open,
	// 404 once it closed before the server handled the answer.
	lateOK := func(sent time.Time, gone, notFound bool) bool {
		end := expires.Add(acShortWait)
		return gone && sent.Before(end) || notFound && !time.Now().Before(end)
	}
	start := time.Now()
	a := w.answer(w.lotse, f.Payload.RequestID, []any{})
	if !lateOK(start, a.code == http.StatusGone, a.code == http.StatusNotFound) {
		t.Fatalf("late answer = %d %s", a.code, a.body)
	}
	start = time.Now()
	sendResult(t, c, f.Payload.RequestID, []any{}, "late-1")
	_, e, ok := w.nextAC(c)
	if !ok || e == nil || e.Op != "command.autocomplete_result" || e.Ref != "late-1" || !lateOK(start, e.Code == "gone", e.Code == "not_found") {
		t.Fatalf("op after the end: %+v", e)
	}
	// The connection stays: a new request still reaches it (the frame arriving is the proof). The
	// answer may come before or after the short timeout, so both outcomes are accepted.
	out = w.ask(w.sam, w.allgemein.id, w.ticket, q47)
	f = w.mustAC(c)
	sendResult(t, c, f.Payload.RequestID, []any{choice("A", "a")}, "")
	if r := w.got(out); r.code != http.StatusOK || len(r.choices()) > 1 {
		t.Fatalf("after the error frame: %d %s", r.code, r.body)
	}
	noViolations(t, c)
}

// TestCommandAutocompleteNotFoundTiming is the time part of AZ-P4 for autocompleteCommand: the
// invisible and the unknown channel take the same time within authz.not_found_timing_tolerance (its
// documented default; the key is not registered yet), compared by testkit.TimingSame. The test
// does not call t.Parallel, so the parallel tests of the package wait until it is done: their
// load lengthens every request many times over, and the invisible case, which also decides and
// audits, then differed beyond the bound in every round (Semgrep rule
// bothy-go-test-timing-parallel).
func TestCommandAutocompleteNotFoundTiming(t *testing.T) {
	w := newACWorld(t)
	q47 := map[string]any{"option": "ticket", "value": "47"}
	invisible := func() time.Duration { return w.askNow(w.outsider, w.allgemein.id, w.ticket, q47).took }
	unknown := func() time.Duration { return w.askNow(w.outsider, ids.New().String(), w.ticket, q47).took }
	if d, same := testkit.TimingSame(notFoundTimingTolerance, invisible, unknown); !same {
		t.Fatalf("the times differ by %s in every round", d)
	}
}

// notFoundTimingTolerance is the documented default of authz.not_found_timing_tolerance (07
// section 11.2 AZ-P4).
const notFoundTimingTolerance = 20 * time.Millisecond

// TestCommandAutocompleteRateLimits: rl.trigger on autocompleteCommand, rl.agent_runtime on
// answerAutocomplete and on the op (brief M2-17 "TestAutocomplete_RateLimits").
func TestCommandAutocompleteRateLimits(t *testing.T) {
	t.Parallel()
	w := newACWorld(t, "BOTHY_RATELIMIT__RL__TRIGGER__BURST=2", "BOTHY_RATELIMIT__RL__TRIGGER__PER_MINUTE=1",
		"BOTHY_RATELIMIT__RL__AGENT_RUNTIME__BURST=2", "BOTHY_RATELIMIT__RL__AGENT_RUNTIME__PER_MINUTE=1")
	q := map[string]any{"option": "ticket", "value": "1"}
	var last acReply
	for range 3 {
		last = w.askNow(w.sam, w.allgemein.id, w.ticket, q)
	}
	if last.code != http.StatusTooManyRequests || last.hdr.Get("Retry-After") == "" {
		t.Fatalf("rl.trigger: %d %s", last.code, last.body)
	}
	for range 3 {
		last = w.answer(w.lotse, ids.New().String(), []any{})
	}
	if last.code != http.StatusTooManyRequests {
		t.Fatalf("rl.agent_runtime on REST: %d %s", last.code, last.body)
	}
	c := w.connectAs(t, w.lotse, "")
	defer c.Close()
	sendResult(t, c, ids.New().String(), []any{}, "rl")
	if _, e, ok := w.nextAC(c); !ok || e == nil || e.Code != "rate_limited" || e.Ref != "rl" {
		t.Fatalf("rl.agent_runtime on the op: %+v", e)
	}
}
