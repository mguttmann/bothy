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

	"github.com/pb33f/libopenapi"
	"github.com/pb33f/libopenapi-validator/schema_validation"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/gateway/adapters/wstest"
	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/adapters/identitytest"
	"github.com/OWNER/bothy/services/server/internal/identity/domain/dpop"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/runs/adapters/runstest"
)

// The limits of the command world: low enough to reach in a test, never the defaults.
const (
	cmdMaxPerPrincipal = 8
	cmdPayloadMax      = 8192
)

// cmdWorld is the world of the slash command tests (M2-16) after 07 "Examples" in the shape of the
// delivery world: the Owner, Sam with trigger on Support, Robin with write on Eskalation and an
// outsider; Support with Allgemein and Eskalation operated by Lotse, Marketing with Kampagnen
// operated by Texter.
type cmdWorld struct {
	*invocationWorld
	robin, outsider       gwPerson
	marketing             string
	allgemein, eskalation channel
	kampagnen             channel
	lotse, texter         consumerAgent
	lotseHandle           string
	samGrant              string
}

// newCmdWorld starts the command world.
func newCmdWorld(t *testing.T) *cmdWorld {
	t.Helper()
	cw := newConsumerWorldWith(t, "BOTHY_COMMAND__MAX_PER_PRINCIPAL="+strconv.Itoa(cmdMaxPerPrincipal),
		"BOTHY_SYNC__EVENT_PAYLOAD_MAX_BYTES="+strconv.Itoa(cmdPayloadMax),
		"BOTHY_RATELIMIT__RL__TRIGGER__BURST=1000", "BOTHY_RATELIMIT__RL__READ__BURST=1000")
	iw := &invocationWorld{consumerWorld: cw, sam: cw.newPerson("sam", identity.DeviceStateActive)}
	w := &cmdWorld{invocationWorld: iw, robin: cw.newPerson("robin", identity.DeviceStateActive),
		outsider: cw.newPerson("robin", identity.DeviceStateActive)}
	w.marketing = w.newSpace("Marketing")
	w.lotse, w.texter = w.agent(), w.agent()
	w.lotseHandle = w.handleOf(w.lotse.id)
	w.allgemein, w.eskalation = w.operatedBy(w.space, "allgemein", w.lotse), w.operatedBy(w.space, "eskalation", w.lotse)
	w.kampagnen = w.operatedBy(w.marketing, "kampagnen", w.texter)
	w.sql(`UPDATE spaces SET default_operator_agent_id = $1 WHERE id = $2`, w.lotse.id.String(), w.space)
	w.samGrant = w.must(w.owner, http.MethodPost, "/api/v1/grants", map[string]any{
		"subject_kind": "user", "subject_id": w.sam.user.String(), "target_kind": "space", "target_id": w.space, "level": "trigger", "reason": "test"})
	w.must(w.owner, http.MethodPost, "/api/v1/grants", map[string]any{
		"subject_kind": "user", "subject_id": w.robin.user.String(), "target_kind": "channel", "target_id": w.eskalation.id, "level": "write", "reason": "test"})
	return w
}

// operatedBy creates a text channel in space operated by a without auto_thread.
func (w *cmdWorld) operatedBy(space, name string, a consumerAgent) channel {
	w.t.Helper()
	ch := w.newChannel(space, name)
	w.sql(`UPDATE channels SET operator_inherit = false, operator_agent_id = $1, auto_thread = false WHERE id = $2`, a.id.String(), ch.id)
	return ch
}

// handleOf reads the handle of a principal.
func (w *cmdWorld) handleOf(id ids.ID) string {
	w.t.Helper()
	var h string
	if err := w.queryRow(context.Background(), `SELECT handle FROM principals WHERE id = $1`, id.String()).Scan(&h); err != nil {
		w.t.Fatal(err)
	}
	return h
}

// agentGrant gives agent a the level on the channel ch and returns the grant id.
func (w *cmdWorld) agentGrant(a consumerAgent, ch channel, level string) string {
	return w.must(w.owner, http.MethodPost, "/api/v1/grants", map[string]any{
		"subject_kind": "agent", "subject_id": a.id.String(), "target_kind": "channel", "target_id": ch.id, "level": level, "reason": "test"})
}

// send is a REST request with the extra headers; the principal is the agent a or the person p.
type send struct {
	agent   *consumerAgent
	person  *gwPerson
	method  string
	path    string
	body    any
	headers map[string]string
}

// do sends s and returns status, body and the response headers.
func (w *cmdWorld) do(s send) (int, []byte, http.Header) {
	w.t.Helper()
	var rd io.Reader
	if s.body != nil {
		b, _ := json.Marshal(s.body)
		rd = bytes.NewReader(b)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, s.method, w.base+s.path, rd)
	if err != nil {
		w.t.Fatal(err)
	}
	tok, key := "", (*identitytest.Key)(nil)
	switch {
	case s.agent != nil:
		tok, key = s.agent.tok, s.agent.key
	case s.person != nil:
		w.sql(`UPDATE auth_sessions SET step_up_at = now(), step_up_method = 'passkey', step_up_consumed_at = NULL WHERE id = $1`,
			s.person.set.Session.ID.String())
		tok, key = s.person.access, s.person.key
	}
	req.Header.Set("Authorization", "DPoP "+tok)
	req.Header.Set("DPoP", identitytest.SignDPoPProof(key, identitytest.ProofClaims{HTM: s.method, HTU: gwPublicURL + s.path,
		IAT: time.Now(), Nonce: w.id.DPoP.CurrentNonce(), ATH: dpop.ATH(tok)}))
	if s.body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range s.headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		w.t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	b, _ := io.ReadAll(resp.Body)
	if bytes.Contains(b, []byte("null")) {
		w.t.Fatalf("%s %s answered JSON null: %s", s.method, s.path, b)
	}
	return resp.StatusCode, b, resp.Header
}

// register posts the definition def as a.
func (w *cmdWorld) register(a consumerAgent, def map[string]any) (int, []byte) {
	code, b, _ := w.do(send{agent: &a, method: http.MethodPost, path: "/api/v1/commands", body: def,
		headers: map[string]string{"Idempotency-Key": ids.New().String()}})
	return code, b
}

// mustRegister registers def as a and returns the command.
func (w *cmdWorld) mustRegister(a consumerAgent, def map[string]any) commandBody {
	w.t.Helper()
	code, b := w.register(a, def)
	if code != http.StatusCreated {
		w.t.Fatalf("registerCommand = %d %s", code, b)
	}
	var c commandBody
	_ = json.Unmarshal(b, &c)
	return c
}

// commandBody is the schema Command as the tests read it.
type commandBody struct {
	ID            string `json:"id"`
	Owner         string `json:"owner_principal_id"`
	Name          string `json:"name"`
	ScopeKind     string `json:"scope_kind"`
	ScopeID       string `json:"scope_id"`
	RequiredLevel string `json:"required_level"`
	Control       bool   `json:"control"`
	Version       int64  `json:"version"`
}

// problem is a problem body as the tests read it.
type problem struct {
	Status int    `json:"status"`
	Code   string `json:"code"`
	Reason string `json:"reason"`
	Errors []struct {
		Pointer string `json:"pointer"`
		Code    string `json:"code"`
	} `json:"errors"`
}

// wantProblem checks status, code, reason and, when pointer or field is set, the one field error.
func wantProblem(t *testing.T, code int, b []byte, status int, pcode, reason, pointer, field string) {
	t.Helper()
	var p problem
	_ = json.Unmarshal(b, &p)
	if code != status || p.Code != pcode || p.Reason != reason {
		t.Fatalf("got %d %s, want %d %s reason %q", code, b, status, pcode, reason)
	}
	if field != "" && (len(p.Errors) != 1 || p.Errors[0].Pointer != pointer || p.Errors[0].Code != field) {
		t.Fatalf("errors %s, want %q %s", b, pointer, field)
	}
}

// sameProblem reports whether two problem bodies are equal apart from correlation_id and instance.
func sameProblem(a, b []byte) bool {
	var x, y map[string]any
	_ = json.Unmarshal(a, &x)
	_ = json.Unmarshal(b, &y)
	for _, m := range []map[string]any{x, y} {
		delete(m, "correlation_id")
		delete(m, "instance")
	}
	xa, _ := json.Marshal(x)
	ya, _ := json.Marshal(y)
	return bytes.Equal(xa, ya)
}

// def returns a definition with name and description.
func def(name string, extra ...any) map[string]any {
	d := map[string]any{"name": name, "description": "Escalate the ticket"}
	for i := 0; i+1 < len(extra); i += 2 {
		d[extra[i].(string)] = extra[i+1]
	}
	return d
}

// events counts the command.changed events of the command id with change.
func (w *cmdWorld) changed(id, change string) int {
	return w.count(`SELECT count(*) FROM event_log WHERE type = 'command.changed' AND resource_id = $1 AND payload->>'change' = $2`, id, change)
}

// audits counts the allow entries of action on the command id.
func (w *cmdWorld) audits(action, id string) int {
	return w.count(`SELECT count(*) FROM audit_events WHERE action = $1 AND resource_id = $2 AND decision = 'allow'`, action, id)
}

// invoke invokes the command id in channel ch as p with body and returns status and body.
func (w *cmdWorld) invoke(p gwPerson, ch channel, id string, body map[string]any) (int, []byte) {
	code, b, _ := w.do(send{person: &p, method: http.MethodPost, path: "/api/v1/channels/" + ch.id + "/commands/" + id + "/invoke", body: body,
		headers: map[string]string{"Idempotency-Key": ids.New().String()}})
	return code, b
}

// invocationBody is the schema CommandInvocation.
type invocationBody struct {
	InvocationID string `json:"invocation_id"`
	CommandID    string `json:"command_id"`
	ChannelID    string `json:"channel_id"`
	ThreadID     string `json:"thread_id"`
	RunID        string `json:"run_id"`
	Accepted     bool   `json:"accepted"`
}

// mustInvoke invokes and requires 202.
func (w *cmdWorld) mustInvoke(p gwPerson, ch channel, id string, body map[string]any) invocationBody {
	w.t.Helper()
	code, b := w.invoke(p, ch, id, body)
	if code != http.StatusAccepted {
		w.t.Fatalf("invokeCommand = %d %s", code, b)
	}
	var inv invocationBody
	_ = json.Unmarshal(b, &inv)
	return inv
}

// thread creates a thread from a new message of the Owner in the main thread of ch and returns it.
func (w *cmdWorld) thread(ch channel) string {
	w.t.Helper()
	code, b := w.rest(w.base, w.owner, http.MethodPost, "/api/v1/threads/"+ch.thread+"/messages",
		map[string]any{"client_msg_id": ids.New().String(), "content_md": "a case"})
	if code != http.StatusCreated && code != http.StatusOK {
		w.t.Fatalf("createMessage = %d %s", code, b)
	}
	var m struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(b, &m)
	code, b = w.rest(w.base, w.owner, http.MethodPost, "/api/v1/channels/"+ch.id+"/threads", map[string]any{"parent_message_id": m.ID})
	if code != http.StatusCreated && code != http.StatusOK {
		w.t.Fatalf("createThread = %d %s", code, b)
	}
	var out struct {
		Thread struct {
			ID string `json:"id"`
		} `json:"thread"`
	}
	_ = json.Unmarshal(b, &out)
	return out.Thread.ID
}

// dispatchFrame is a dispatch of an invocation-bearing event as the command tests read it.
type dispatchFrame struct {
	Seq             int64           `json:"seq"`
	Type            string          `json:"type"`
	InvocationToken string          `json:"invocation_token"`
	Audience        string          `json:"audience"`
	Resource        json.RawMessage `json:"resource"`
	Payload         json.RawMessage `json:"payload"`
}

// nextOf reads frames of c until a dispatch of typ arrives or d passed, acknowledging every other
// dispatch; ok is false when none came.
func (w *cmdWorld) nextOf(c *wstest.Client, typ string, d time.Duration) (dispatchFrame, bool) {
	w.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), d)
	defer cancel()
	for {
		op, raw, err := c.Next(ctx)
		if errors.Is(err, context.DeadlineExceeded) {
			return dispatchFrame{}, false
		}
		if err != nil {
			w.t.Fatal(err)
		}
		if op != "dispatch" {
			continue
		}
		var f dispatchFrame
		if err := json.Unmarshal(raw, &f); err != nil {
			w.t.Fatal(err)
		}
		if f.InvocationToken != "" {
			w.remember(f.InvocationToken)
		}
		if f.Type != typ {
			sendAck(w.t, c, f.Seq)
			continue
		}
		return f, true
	}
}

// fresh creates a new agent operating a new text channel of Support without auto_thread.
func (w *cmdWorld) fresh(name string) (consumerAgent, channel) {
	w.t.Helper()
	a := w.agent()
	return a, w.operatedBy(w.space, name+"-"+ids.New().String()[30:], a)
}

// connect opens the default consumer of a and reads until the catch-up is complete.
func (w *cmdWorld) connect(t *testing.T, a consumerAgent) *wstest.Client {
	t.Helper()
	c, _ := w.ready(t, w.agentParams(a, ""))
	readUntil(t, c, func(f frame) bool { return f.op == "catchup.complete" })
	return c
}

// mention posts a message of p mentioning a in thread and returns the run it opened.
func (w *cmdWorld) mention(p gwPerson, thread string, a consumerAgent) runRow {
	w.t.Helper()
	m, _ := w.say(p, thread, "@"+w.handleOf(a.id)+" please look")
	r, ok := w.runOf(m)
	if !ok || r.state != "queued" {
		w.t.Fatalf("no active run of the mention: %+v", r)
	}
	if n := w.count(`SELECT count(*) FROM runs WHERE id = $1 AND agent_id = $2`, r.id, a.id.String()); n != 1 {
		var ment, agent, thop string
		_ = w.queryRow(context.Background(), `SELECT coalesce(string_agg(mentioned_principal_id::text, ','), '') FROM mentions WHERE message_id = $1`, m).Scan(&ment)
		_ = w.queryRow(context.Background(), `SELECT agent_id::text FROM runs WHERE id = $1`, r.id).Scan(&agent)
		_ = w.queryRow(context.Background(), `SELECT coalesce(operator_agent_id::text, '') FROM threads WHERE id = $1`, thread).Scan(&thop)
		w.t.Fatalf("the run of the mention is not the run of the mentioned agent: mentions %s agent %s want %s thread op %s", ment, agent, a.id, thop)
	}
	return r
}

// escalate is the model command of the invocation tests: a required string and a choice.
func escalate(name string) map[string]any {
	return def(name, "options", []any{
		map[string]any{"name": "ticket", "description": "Ticket number", "type": "string", "required": true},
		map[string]any{"name": "priority", "description": "Priority", "type": "choice", "required": false,
			"choices": []any{map[string]any{"name": "High", "value": "high"}, map[string]any{"name": "Low", "value": "low"}}},
	})
}

// opts are option values.
func opts(kv ...string) []any {
	out := []any{}
	for i := 0; i+1 < len(kv); i += 2 {
		out = append(out, map[string]any{"name": kv[i], "value": kv[i+1]})
	}
	return out
}

// TestSlashCommands is the integration suite of M2-16 against a started api process: the command
// register, the list rule of 09 10.3 and the execution of control and model commands with the
// delivery re-check.
func TestSlashCommands(t *testing.T) {
	t.Parallel()
	w := newCmdWorld(t)

	t.Run("TestAG24_ReservedNamesRefused", func(t *testing.T) {
		before := w.count(`SELECT count(*) FROM commands`)
		for _, name := range []string{"stop", "status", "new", "resume", "title", "close"} {
			code, b := w.register(w.lotse, def(name))
			wantProblem(t, code, b, http.StatusConflict, "conflict", "command_name_reserved", "", "")
		}
		if n := w.count(`SELECT count(*) FROM commands`); n != before {
			t.Fatalf("%d rows written", n-before)
		}
		if n := w.count(`SELECT count(*) FROM audit_events WHERE action = 'command.register' AND decision = 'allow' AND actor_principal_id = $1`,
			w.lotse.id.String()); n != 0 {
			t.Fatalf("%d allow entries", n)
		}
		c := w.mustRegister(w.lotse, def("ag24"))
		code, b, _ := w.do(send{agent: &w.lotse, method: http.MethodPut, path: "/api/v1/commands/" + c.ID, body: def("stop"),
			headers: map[string]string{"If-Match": `"1"`}})
		wantProblem(t, code, b, http.StatusConflict, "conflict", "command_name_reserved", "", "")
		if n := w.count(`SELECT count(*) FROM commands WHERE id = $1 AND name = 'ag24' AND version = 1`, c.ID); n != 1 {
			t.Fatal("the update changed the command")
		}
		if n := w.count(`SELECT count(*) FROM event_log WHERE type = 'command.changed' AND payload->'command'->>'name' = 'stop'`); n != 0 {
			t.Fatal("a reserved name was announced")
		}
		w.removeAll(w.lotse)
	})

	t.Run("TestRegisterCommand_Created", func(t *testing.T) {
		a := w.agent()
		c := w.mustRegister(a, def("escalate", "options", []any{
			map[string]any{"name": "ticket", "description": "Ticket number", "type": "string", "required": true},
			map[string]any{"name": "priority", "description": "Priority", "type": "choice", "required": false,
				"choices": []any{map[string]any{"name": "High", "value": "high"}, map[string]any{"name": "Low", "value": "low"}}},
		}))
		if c.Version != 1 || c.ScopeKind != "instance" || c.ScopeID != "" || c.RequiredLevel != "trigger" || c.Control || c.Owner != a.id.String() {
			t.Fatalf("command %+v", c)
		}
		var space, channelID *string
		var version int64
		var receive string
		var payload []byte
		if err := w.queryRow(context.Background(), `SELECT space_id::text, channel_id::text, resource_version, receive_action, payload
			FROM event_log WHERE type = 'command.changed' AND resource_id = $1`, c.ID).Scan(&space, &channelID, &version, &receive, &payload); err != nil {
			t.Fatal(err)
		}
		var pl struct {
			Change string `json:"change"`
		}
		_ = json.Unmarshal(payload, &pl)
		if space != nil || channelID != nil || version != 1 || receive != "command.read" || pl.Change != "registered" {
			t.Fatalf("event %v %v %d %s %s", space, channelID, version, receive, payload)
		}
		if w.changed(c.ID, "registered") != 1 || w.audits("command.register", c.ID) != 1 {
			t.Fatal("event or audit missing")
		}
		var details []byte
		if err := w.queryRow(context.Background(), `SELECT details FROM audit_events WHERE action = 'command.register' AND resource_id = $1`, c.ID).
			Scan(&details); err != nil {
			t.Fatal(err)
		}
		var d map[string]any
		_ = json.Unmarshal(details, &d)
		for _, k := range []string{"change", "scope_kind", "control", "required_level", "version"} {
			if _, ok := d[k]; !ok {
				t.Fatalf("details %s without %s", details, k)
			}
		}
		if n := w.count(`SELECT count(*) FROM audit_events WHERE details::text LIKE '%escalate%' OR details::text LIKE '%Escalate the ticket%'`); n != 0 {
			t.Fatal("the name or description is in audit_events")
		}
		wantSchema(t, "CommandChangedPayload", payload)
	})

	t.Run("TestRegisterCommand_Validation", func(t *testing.T) {
		a := w.agent()
		opt := func(extra ...any) map[string]any {
			o := map[string]any{"name": "o", "description": "An option", "type": "string", "required": false}
			for i := 0; i+1 < len(extra); i += 2 {
				o[extra[i].(string)] = extra[i+1]
			}
			return o
		}
		many := make([]any, 0, 26)
		for i := range 26 {
			many = append(many, map[string]any{"name": "C" + strconv.Itoa(i), "value": "v" + strconv.Itoa(i)})
		}
		big := make([]any, 0, 80)
		for i := range 80 {
			big = append(big, opt("name", "o"+strconv.Itoa(i), "description", strings.Repeat("d", 100)))
		}
		cases := []struct {
			name, pointer, code string
			body                map[string]any
		}{
			{"description too long", "/description", "too_long", def("v1", "description", strings.Repeat("x", 101))},
			{"description bidi", "/description", "invalid", def("v2", "description", "ab\u202ecd")},
			{"scope host", "/scope_kind", "invalid_enum", def("v3", "scope_kind", "host", "scope_id", ids.New().String())},
			{"scope_id missing", "/scope_id", "required", def("v4", "scope_kind", "channel")},
			{"scope_id superfluous", "/scope_id", "invalid", def("v5", "scope_kind", "instance", "scope_id", ids.New().String())},
			{"options twice", "/options/1/name", "invalid", def("v6", "options", []any{opt(), opt()})},
			{"choices on boolean", "/options/0/choices", "invalid", def("v7", "options", []any{opt("type", "boolean",
				"choices", []any{map[string]any{"name": "Yes", "value": "true"}})})},
			{"choice without choices", "/options/0/choices", "required", def("v8", "options", []any{opt("type", "choice")})},
			{"too many choices", "/options/0/choices", "too_many_items", def("v9", "options", []any{opt("type", "choice", "choices", many)})},
			{"too large", "", "too_large", def("v10", "options", big)},
		}
		for _, tc := range cases {
			code, b := w.register(a, tc.body)
			if code != http.StatusBadRequest {
				t.Fatalf("%s: %d %s", tc.name, code, b)
			}
			wantProblem(t, code, b, http.StatusBadRequest, "validation_failed", "", tc.pointer, tc.code)
		}
		if n := w.count(`SELECT count(*) FROM commands WHERE owner_principal_id = $1`, a.id.String()); n != 0 {
			t.Fatalf("%d rows written", n)
		}
	})

	t.Run("TestRegisterCommand_DuplicateAndNulls", func(t *testing.T) {
		a := w.agent()
		w.sql(`UPDATE channels SET operator_agent_id = $1 WHERE id = $2`, a.id.String(), w.allgemein.id)
		defer w.sql(`UPDATE channels SET operator_agent_id = $1 WHERE id = $2`, w.lotse.id.String(), w.allgemein.id)
		c := w.mustRegister(a, def("escalate"))
		code, b := w.register(a, def("escalate"))
		wantProblem(t, code, b, http.StatusConflict, "conflict", "", "", "")
		w.mustRegister(a, def("escalate", "scope_kind", "channel", "scope_id", w.allgemein.id))
		if code, b, _ := w.do(send{agent: &a, method: http.MethodDelete, path: "/api/v1/commands/" + c.ID}); code != http.StatusNoContent {
			t.Fatalf("remove = %d %s", code, b)
		}
		w.mustRegister(a, def("escalate"))
	})

	t.Run("TestRegisterCommand_LimitPerPrincipal", func(t *testing.T) {
		a := w.agent()
		for i := range cmdMaxPerPrincipal - 1 {
			w.mustRegister(a, def("limit"+strconv.Itoa(i)))
		}
		var wg sync.WaitGroup
		codes := make([]int, 2)
		for i := range 2 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				codes[i], _ = w.register(a, def("race"+strconv.Itoa(i)))
			}()
		}
		wg.Wait()
		slices.Sort(codes)
		if codes[0] != http.StatusCreated || codes[1] != http.StatusBadRequest {
			t.Fatalf("parallel registrations at the limit: %v", codes)
		}
		code, b := w.register(a, def("over"))
		wantProblem(t, code, b, http.StatusBadRequest, "validation_failed", "", "", "limit_reached")
		var one string
		_ = w.queryRow(context.Background(), `SELECT id::text FROM commands WHERE owner_principal_id = $1 AND name = 'limit0'`, a.id.String()).Scan(&one)
		if code, _, _ := w.do(send{agent: &a, method: http.MethodDelete, path: "/api/v1/commands/" + one}); code != http.StatusNoContent {
			t.Fatal("remove")
		}
		w.mustRegister(a, def("over"))
	})

	t.Run("TestRegisterCommand_TargetRule", func(t *testing.T) {
		w.mustRegister(w.lotse, def("target1", "scope_kind", "channel", "scope_id", w.allgemein.id))
		code, b := w.register(w.lotse, def("target2", "scope_kind", "channel", "scope_id", w.kampagnen.id))
		_, unknown := w.register(w.lotse, def("target2", "scope_kind", "channel", "scope_id", ids.New().String()))
		if code != http.StatusNotFound || !sameProblem(b, unknown) {
			t.Fatalf("foreign channel %d %s, unknown %s", code, b, unknown)
		}
		w.mustRegister(w.lotse, def("target3", "scope_kind", "space", "scope_id", w.space))
		code, b = w.register(w.lotse, def("target4", "scope_kind", "space", "scope_id", w.marketing))
		wantProblem(t, code, b, http.StatusNotFound, "not_found", "", "", "")
		w.removeAll(w.lotse)
		tx := w.agent()
		w.agentGrant(tx, w.eskalation, "write")
		w.agentGrant(tx, w.allgemein, "read")
		w.mustRegister(tx, def("t5", "scope_kind", "channel", "scope_id", w.eskalation.id))
		code, b = w.register(tx, def("t6", "scope_kind", "channel", "scope_id", w.allgemein.id))
		wantProblem(t, code, b, http.StatusForbidden, "forbidden", "level_insufficient", "", "")
		w.mustRegister(tx, def("t7"))
	})

	t.Run("TestUpdateCommand", func(t *testing.T) {
		c := w.mustRegister(w.lotse, def("upd"))
		path := "/api/v1/commands/" + c.ID
		put := func(a consumerAgent, p string, body map[string]any, ifMatch string) (int, []byte, http.Header) {
			h := map[string]string{}
			if ifMatch != "" {
				h["If-Match"] = ifMatch
			}
			return w.do(send{agent: &a, method: http.MethodPut, path: p, body: body, headers: h})
		}
		code, b, _ := put(w.lotse, path, def("upd", "description", "New text"), "")
		wantProblem(t, code, b, http.StatusPreconditionRequired, "precondition_required", "", "", "")
		code, b, _ = put(w.lotse, path, def("upd", "description", "New text"), `"7"`)
		wantProblem(t, code, b, http.StatusPreconditionFailed, "version_conflict", "", "", "")
		if !strings.Contains(string(b), `"current"`) {
			t.Fatalf("412 without current: %s", b)
		}
		code, b, h := put(w.lotse, path, def("upd", "description", "New text"), `"1"`)
		var got commandBody
		_ = json.Unmarshal(b, &got)
		if code != http.StatusOK || got.Version != 2 || h.Get("ETag") != `"2"` {
			t.Fatalf("update = %d %s etag %s", code, b, h.Get("ETag"))
		}
		if w.changed(c.ID, "updated") != 1 || w.audits("command.register", c.ID) != 2 {
			t.Fatal("event or audit of the update")
		}
		code, b, _ = put(w.lotse, path, def("upd", "description", "New text"), `"2"`)
		if _ = json.Unmarshal(b, &got); code != http.StatusOK || got.Version != 2 || w.changed(c.ID, "updated") != 1 ||
			w.audits("command.register", c.ID) != 2 {
			t.Fatalf("identical update = %d %s", code, b)
		}
		code, b, _ = put(w.texter, path, def("upd"), `"2"`)
		wantProblem(t, code, b, http.StatusForbidden, "forbidden", "not_self", "", "")
		allg := w.mustRegister(w.lotse, def("upd2", "scope_kind", "channel", "scope_id", w.allgemein.id))
		code, hidden, hh := put(w.texter, "/api/v1/commands/"+allg.ID, def("upd2", "scope_kind", "channel", "scope_id", w.kampagnen.id), `"1"`)
		ucode, unknown, uh := put(w.texter, "/api/v1/commands/"+ids.New().String(), def("upd2", "scope_kind", "channel", "scope_id",
			w.kampagnen.id), `"1"`)
		if code != http.StatusNotFound || ucode != code || !sameProblem(hidden, unknown) || hh.Get("ETag") != uh.Get("ETag") {
			t.Fatalf("hidden %d %s, unknown %d %s", code, hidden, ucode, unknown)
		}
		code, hidden, _ = put(w.texter, "/api/v1/commands/"+allg.ID, def("upd2"), `"1"`)
		if code != http.StatusNotFound || !sameProblem(hidden, unknown) {
			t.Fatalf("hidden without scope %d %s", code, hidden)
		}
		tx := w.agent()
		g := w.agentGrant(tx, w.eskalation, "write")
		esk := w.mustRegister(tx, def("mine", "scope_kind", "channel", "scope_id", w.eskalation.id))
		w.must(w.owner, http.MethodPost, "/api/v1/grants/"+g+"/revoke", map[string]any{"reason": "moved"})
		if code, b, _ := put(tx, "/api/v1/commands/"+esk.ID, def("mine"), `"1"`); code != http.StatusOK {
			t.Fatalf("move to instance after revoke = %d %s", code, b)
		}
		w.removeAll(w.lotse)
	})

	t.Run("TestRemoveCommand", func(t *testing.T) {
		c := w.mustRegister(w.lotse, def("rm"))
		path := "/api/v1/commands/" + c.ID
		inv := w.mustInvoke(w.sam, w.allgemein, c.ID, map[string]any{"thread_id": w.thread(w.allgemein)})
		if code, b, _ := w.do(send{agent: &w.lotse, method: http.MethodDelete, path: path}); code != http.StatusNoContent {
			t.Fatalf("remove = %d %s", code, b)
		}
		if n := w.count(`SELECT count(*) FROM commands WHERE id = $1 AND removed_at IS NOT NULL AND version = 2`, c.ID); n != 1 {
			t.Fatal("not removed")
		}
		if w.changed(c.ID, "removed") != 1 || w.audits("command.remove", c.ID) != 1 {
			t.Fatal("event or audit of the removal")
		}
		if code, _, _ := w.do(send{agent: &w.lotse, method: http.MethodDelete, path: path}); code != http.StatusNoContent ||
			w.changed(c.ID, "removed") != 1 || w.audits("command.remove", c.ID) != 1 {
			t.Fatalf("second remove = %d", code)
		}
		if code, _, _ := w.do(send{agent: &w.lotse, method: http.MethodGet, path: path}); code != http.StatusNotFound {
			t.Fatalf("get after remove = %d", code)
		}
		if n := w.count(`SELECT count(*) FROM runs WHERE id = $1 AND triggering_command_id = $2`, inv.RunID, c.ID); n != 1 {
			t.Fatal("the run lost triggering_command_id")
		}
		allg := w.mustRegister(w.lotse, def("rm2", "scope_kind", "channel", "scope_id", w.allgemein.id))
		code, hidden, _ := w.do(send{agent: &w.texter, method: http.MethodDelete, path: "/api/v1/commands/" + allg.ID})
		_, unknown, _ := w.do(send{agent: &w.texter, method: http.MethodDelete, path: "/api/v1/commands/" + ids.New().String()})
		if code != http.StatusNotFound || !sameProblem(hidden, unknown) {
			t.Fatalf("hidden %d %s", code, hidden)
		}
		inst := w.mustRegister(w.lotse, def("rm3"))
		code, b, _ := w.do(send{agent: &w.texter, method: http.MethodDelete, path: "/api/v1/commands/" + inst.ID})
		wantProblem(t, code, b, http.StatusForbidden, "forbidden", "not_self", "", "")
		w.removeAll(w.lotse)
	})

	t.Run("TestGetCommand_Visibility", func(t *testing.T) {
		inst := w.mustRegister(w.lotse, def("vis1"))
		allg := w.mustRegister(w.lotse, def("vis2", "scope_kind", "channel", "scope_id", w.allgemein.id))
		kamp := w.mustRegister(w.texter, def("vis3", "scope_kind", "channel", "scope_id", w.kampagnen.id))
		code, _, h := w.do(send{agent: &w.lotse, method: http.MethodGet, path: "/api/v1/commands/" + allg.ID})
		if code != http.StatusOK || h.Get("ETag") != `"1"` {
			t.Fatalf("owner get = %d %s", code, h.Get("ETag"))
		}
		for _, id := range []string{inst.ID, allg.ID} {
			if code, b, _ := w.do(send{person: &w.sam, method: http.MethodGet, path: "/api/v1/commands/" + id}); code != http.StatusOK {
				t.Fatalf("sam get = %d %s", code, b)
			}
		}
		code, hidden, hh := w.do(send{person: &w.sam, method: http.MethodGet, path: "/api/v1/commands/" + kamp.ID})
		ucode, unknown, uh := w.do(send{person: &w.sam, method: http.MethodGet, path: "/api/v1/commands/" + ids.New().String()})
		if code != http.StatusNotFound || ucode != code || !sameProblem(hidden, unknown) || hh.Get("Content-Type") != uh.Get("Content-Type") {
			t.Fatalf("hidden %d %s unknown %d %s", code, hidden, ucode, unknown)
		}
		w.removeAll(w.lotse)
		w.removeAll(w.texter)
	})

	t.Run("TestCommandChanged_Delivery", func(t *testing.T) {
		var after int64
		_ = w.queryRow(context.Background(), `SELECT coalesce(max(seq), 0) FROM event_log`).Scan(&after)
		inst := w.mustRegister(w.lotse, def("ev1"))
		sup := w.mustRegister(w.lotse, def("ev2", "scope_kind", "space", "scope_id", w.space))
		events := func(code int, b []byte) string {
			if code != http.StatusOK {
				t.Fatalf("listEvents = %d %s", code, b)
			}
			return string(b)
		}
		q := "/api/v1/events?after=" + strconv.FormatInt(after, 10)
		sam := events(w.rest(w.base, w.sam, http.MethodGet, q, nil))
		if !strings.Contains(sam, inst.ID) || !strings.Contains(sam, sup.ID) {
			t.Fatalf("sam misses command.changed: %s", sam)
		}
		out := events(w.rest(w.base, w.outsider, http.MethodGet, q, nil))
		if !strings.Contains(out, inst.ID) || strings.Contains(out, sup.ID) {
			t.Fatalf("outsider: %s", out)
		}
		lot := events(w.call(w.base, w.lotse, http.MethodGet, q, nil))
		if !strings.Contains(lot, inst.ID) || !strings.Contains(lot, sup.ID) {
			t.Fatalf("lotse: %s", lot)
		}
		w.removeAll(w.lotse)
	})

	t.Run("TestListChannelCommands_Rule10_3", func(t *testing.T) {
		list := func(p gwPerson, ch channel) []commandBody {
			code, b, _ := w.do(send{person: &p, method: http.MethodGet, path: "/api/v1/channels/" + ch.id + "/commands"})
			if code != http.StatusOK {
				t.Fatalf("list = %d %s", code, b)
			}
			var out struct {
				Items []commandBody `json:"items"`
			}
			_ = json.Unmarshal(b, &out)
			return out.Items
		}
		names := func(items []commandBody) []string {
			var out []string
			for _, c := range items {
				out = append(out, c.Name+"@"+c.Owner[len(c.Owner)-4:]+"/"+c.ScopeKind)
			}
			return out
		}
		guest := w.agent()
		l, tx := w.lotse.id.String(), guest.id.String()
		w.mustRegister(w.lotse, def("l-inst"))
		w.mustRegister(w.lotse, def("l-space", "scope_kind", "space", "scope_id", w.space))
		w.mustRegister(w.lotse, def("l-allg", "scope_kind", "channel", "scope_id", w.allgemein.id))
		w.mustRegister(w.lotse, def("l-esk", "scope_kind", "channel", "scope_id", w.eskalation.id))
		w.mustRegister(w.lotse, def("deploy"))
		dep := w.mustRegister(w.lotse, def("deploy", "scope_kind", "channel", "scope_id", w.allgemein.id))
		w.mustRegister(w.lotse, def("shared"))
		w.mustRegister(guest, def("shared"))
		w.mustRegister(w.texter, def("shared"))
		want := []string{"deploy@" + l[len(l)-4:] + "/channel", "l-allg@" + l[len(l)-4:] + "/channel", "l-inst@" + l[len(l)-4:] + "/instance",
			"l-space@" + l[len(l)-4:] + "/space", "shared@" + l[len(l)-4:] + "/instance"}
		got := list(w.sam, w.allgemein)
		if !slices.Equal(names(got), want) || got[0].ID != dep.ID {
			t.Fatalf("list %v, want %v", names(got), want)
		}
		w.sql(`UPDATE channels SET allow_mentioned_agents = true WHERE id = $1`, w.allgemein.id)
		defer w.sql(`UPDATE channels SET allow_mentioned_agents = false WHERE id = $1`, w.allgemein.id)
		if got := names(list(w.sam, w.allgemein)); !slices.Equal(got, want) {
			t.Fatalf("a guest without write appears: %v", got)
		}
		w.agentGrant(guest, w.allgemein, "write")
		withGuest := append(slices.Clone(want), "shared@"+tx[len(tx)-4:]+"/instance")
		if got := names(list(w.sam, w.allgemein)); !slices.Equal(got, withGuest) {
			t.Fatalf("list with guest %v, want %v", got, withGuest)
		}
		asst := w.agent()
		w.agentGrant(asst, w.allgemein, "write")
		w.mustRegister(asst, def("personal"))
		w.sql(`UPDATE agents SET billing_mode = 'personal', credential_holder_id = $1 WHERE principal_id = $2`, w.owner.user.String(), asst.id.String())
		if got := names(list(w.sam, w.allgemein)); !slices.Equal(got, withGuest) {
			t.Fatalf("a personal agent appears as guest: %v", got)
		}
		w.must(w.owner, http.MethodPost, "/api/v1/agents/"+tx+"/disable", map[string]any{"reason": "test"})
		if got := names(list(w.sam, w.allgemein)); !slices.Equal(got, want) {
			t.Fatalf("a disabled owner appears: %v", got)
		}
		code, hidden, _ := w.do(send{person: &w.outsider, method: http.MethodGet, path: "/api/v1/channels/" + w.allgemein.id + "/commands"})
		_, unknown, _ := w.do(send{person: &w.outsider, method: http.MethodGet, path: "/api/v1/channels/" + ids.New().String() + "/commands"})
		if code != http.StatusNotFound || !sameProblem(hidden, unknown) {
			t.Fatalf("outsider %d %s", code, hidden)
		}
		w.removeAll(w.lotse)
		w.removeAll(w.texter)
	})

	t.Run("TestInvokeModelCommand_CreatesRun", func(t *testing.T) {
		a, ch := w.fresh("model")
		c := w.connect(t, a)
		cmd := w.mustRegister(a, escalate("escalate"))
		th := w.thread(ch)
		inv := w.mustInvoke(w.sam, ch, cmd.ID, map[string]any{"thread_id": th, "options": opts("priority", "high", "ticket", "T-1")})
		if inv.InvocationID == "" || inv.RunID == "" || inv.ThreadID != th || !inv.Accepted || inv.CommandID != cmd.ID || inv.ChannelID != ch.id {
			t.Fatalf("invocation %+v", inv)
		}
		if n := w.count(`SELECT count(*) FROM runs WHERE id = $1 AND state = 'queued' AND trigger_kind = 'command' AND triggering_command_id = $2
			AND triggered_by = $3 AND triggering_message_id IS NULL AND thread_id = $4`, inv.RunID, cmd.ID, w.sam.user.String(), th); n != 1 {
			t.Fatal("the run of the command")
		}
		if n := w.count(`SELECT count(*) FROM messages WHERE run_id = $1 AND thread_id = $2 AND kind = 'run'`, inv.RunID, th); n != 1 {
			t.Fatal("no anchor")
		}
		if n := w.count(`SELECT count(*) FROM event_log WHERE type = 'run.state_changed' AND resource_id = $1`, inv.RunID); n != 1 {
			t.Fatalf("run.state_changed %d", n)
		}
		if n := w.count(`SELECT count(*) FROM event_log WHERE type = 'command.invoked' AND resource_id = $1`, cmd.ID); n != 0 {
			t.Fatal("a model command sent command.invoked")
		}
		if n := w.count(`SELECT count(*) FROM messages WHERE thread_id = $1 AND trigger_state IS NOT NULL`, th); n != 0 {
			t.Fatal("a trigger_state changed")
		}
		if n := w.count(`SELECT count(*) FROM jobs.river_job WHERE kind = 'runs.queued_expiry' AND args->>'run_id' = $1::text`, inv.RunID); n != 1 {
			t.Fatal("no runs.queued_expiry job")
		}
		var payload []byte
		if err := w.queryRow(context.Background(), `SELECT payload FROM event_log WHERE type = 'agent.invocation' AND resource_id = $1`, inv.RunID).
			Scan(&payload); err != nil {
			t.Fatal(err)
		}
		wantSchema(t, "AgentInvocationPayload", payload)
		var p struct {
			TriggerKind  string `json:"trigger_kind"`
			InvocationID string `json:"command_invocation_id"`
			CommandID    string `json:"command_id"`
			CommandName  string `json:"command_name"`
			MessageID    string `json:"message_id"`
			Options      []struct {
				Name string `json:"name"`
			} `json:"options"`
		}
		_ = json.Unmarshal(payload, &p)
		if p.TriggerKind != "command" || p.InvocationID != inv.InvocationID || p.CommandID != cmd.ID || p.CommandName != "escalate" ||
			p.MessageID != "" || len(p.Options) != 2 || p.Options[0].Name != "ticket" || p.Options[1].Name != "priority" {
			t.Fatalf("payload %s", payload)
		}
		f, ok := w.nextOf(c, "agent.invocation", 15*time.Second)
		if !ok || f.InvocationToken == "" {
			t.Fatal("no agent.invocation")
		}
		cl, err := w.verifier(a).Check(context.Background(), f.InvocationToken, runstest.ReferenceExpectation{RunID: ids.MustParse(inv.RunID),
			ThreadID: ids.MustParse(th), CommandID: ids.MustParse(cmd.ID), ActualBillingMode: "organization", ActualToolClass: "chat"})
		var jti string
		_ = w.queryRow(context.Background(), `SELECT invocation_jti::text FROM runs WHERE id = $1`, inv.RunID).Scan(&jti)
		if err != nil || cl.TriggerKind != "command" || cl.CommandID != cmd.ID || cl.RunID != inv.RunID || cl.Sub != w.sam.user.String() ||
			cl.MessageID != "" || cl.Jti != jti {
			t.Fatalf("claims %+v %v jti %s", cl, err, jti)
		}
	})

	t.Run("TestInvokeModelCommand_RunActive", func(t *testing.T) {
		a, ch := w.fresh("busy")
		cmd := w.mustRegister(a, escalate("busy"))
		th := w.thread(ch)
		w.mention(w.sam, th, a)
		runsBefore := w.count(`SELECT count(*) FROM runs WHERE thread_id = $1`, th)
		anchors := w.count(`SELECT count(*) FROM messages WHERE thread_id = $1 AND kind = 'run'`, th)
		code, b := w.invoke(w.sam, ch, cmd.ID, map[string]any{"thread_id": th, "options": opts("ticket", "T-2")})
		wantProblem(t, code, b, http.StatusForbidden, "agent_invocation_denied", "run_active", "", "")
		// The active run of a guest refuses the same way.
		guest := w.guestIn(ch)
		th2 := w.thread(ch)
		w.mention(w.sam, th2, guest)
		code, b = w.invoke(w.sam, ch, cmd.ID, map[string]any{"thread_id": th2, "options": opts("ticket", "T-2")})
		wantProblem(t, code, b, http.StatusForbidden, "agent_invocation_denied", "run_active", "", "")
		if w.count(`SELECT count(*) FROM runs WHERE thread_id = $1`, th) != runsBefore ||
			w.count(`SELECT count(*) FROM messages WHERE thread_id = $1 AND kind = 'run'`, th) != anchors ||
			w.count(`SELECT count(*) FROM runs WHERE trigger_kind = 'command' AND triggering_command_id = $1`, cmd.ID) != 0 ||
			w.count(`SELECT count(*) FROM event_log WHERE type = 'agent.invocation' AND payload->>'command_id' = $1`, cmd.ID) != 0 {
			t.Fatal("a refused model command wrote")
		}
		w.denials(t, `action = 'command.invoke' AND deny_reason = 'run_active' AND resource_id = $1`, th)
	})

	t.Run("TestInvokeModelCommand_ConcurrentOneRun", func(t *testing.T) {
		a, ch := w.fresh("race")
		cmd := w.mustRegister(a, escalate("race"))
		th := w.thread(ch)
		var wg sync.WaitGroup
		codes := make([]int, 2)
		bodies := make([][]byte, 2)
		for i := range 2 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				codes[i], bodies[i] = w.invoke(w.sam, ch, cmd.ID, map[string]any{"thread_id": th, "options": opts("ticket", "T-3")})
			}()
		}
		wg.Wait()
		ok, refused := 0, 0
		for i, c := range codes {
			switch {
			case c == http.StatusAccepted:
				ok++
			case c == http.StatusForbidden && strings.Contains(string(bodies[i]), `"reason":"run_active"`):
				refused++
			}
		}
		if ok != 1 || refused != 1 {
			t.Fatalf("codes %v %s %s", codes, bodies[0], bodies[1])
		}
		if n := w.count(`SELECT count(*) FROM runs WHERE thread_id = $1 AND state IN ('queued', 'running', 'waiting_input')`, th); n != 1 {
			t.Fatalf("%d active runs", n)
		}
	})

	t.Run("TestInvokeModelCommand_ThreadResolution", func(t *testing.T) {
		a, ch := w.fresh("resolve")
		model := w.mustRegister(a, escalate("resolve"))
		ctl := w.mustRegister(a, def("usage", "control", true))
		w.sql(`UPDATE channels SET auto_thread = true WHERE id = $1`, ch.id)
		threads := func() int { return w.count(`SELECT count(*) FROM threads WHERE channel_id = $1`, ch.id) }
		before := threads()
		code, b := w.invoke(w.sam, ch, model.ID, map[string]any{"options": opts("ticket", "T-4")})
		wantProblem(t, code, b, http.StatusBadRequest, "validation_failed", "", "/thread_id", "required")
		if threads() != before || w.count(`SELECT count(*) FROM runs WHERE triggering_command_id = $1`, model.ID) != 0 {
			t.Fatal("a refused invocation wrote")
		}
		inv := w.mustInvoke(w.sam, ch, ctl.ID, map[string]any{})
		if inv.ThreadID != ch.thread || inv.RunID != "" || w.count(`SELECT count(*) FROM event_log WHERE type = 'command.invoked'
			AND resource_id = $1 AND thread_id = $2`, ctl.ID, ch.thread) != 1 {
			t.Fatalf("control without thread %+v", inv)
		}
		w.sql(`UPDATE channels SET auto_thread = false WHERE id = $1`, ch.id)
		inv = w.mustInvoke(w.sam, ch, model.ID, map[string]any{"options": opts("ticket", "T-4")})
		if inv.ThreadID != ch.thread || w.count(`SELECT count(*) FROM runs WHERE id = $1 AND thread_id = $2`, inv.RunID, ch.thread) != 1 ||
			threads() != before {
			t.Fatalf("model without thread %+v", inv)
		}
		forum := w.must(w.owner, http.MethodPost, "/api/v1/spaces/"+w.space+"/channels", map[string]any{"name": "forum", "kind": "forum"})
		w.sql(`UPDATE channels SET operator_inherit = false, operator_agent_id = $1 WHERE id = $2`, a.id.String(), forum)
		code, b = w.invoke(w.sam, channel{id: forum}, ctl.ID, map[string]any{})
		wantProblem(t, code, b, http.StatusBadRequest, "validation_failed", "", "/thread_id", "required")
		code, b = w.invoke(w.sam, ch, ctl.ID, map[string]any{"thread_id": w.allgemein.thread})
		wantProblem(t, code, b, http.StatusNotFound, "not_found", "", "", "")
	})

	t.Run("TestInvokeControlCommand_NeverToModel", func(t *testing.T) {
		a, ch := w.fresh("control")
		c := w.connect(t, a)
		ctl := w.mustRegister(a, def("usage", "control", true, "options", []any{
			map[string]any{"name": "ticket", "description": "Ticket", "type": "string", "required": false}}))
		th := w.thread(ch)
		check := func(inv invocationBody, wantRun string) {
			t.Helper()
			var payload []byte
			var audience, receive, kind string
			var version *int64
			if err := w.queryRow(context.Background(), `SELECT payload, audience_principal_id::text, receive_action, resource_kind, resource_version
				FROM event_log WHERE type = 'command.invoked' AND payload->>'invocation_id' = $1`, inv.InvocationID).
				Scan(&payload, &audience, &receive, &kind, &version); err != nil {
				t.Fatal(err)
			}
			wantSchema(t, "CommandInvokedPayload", payload)
			var p struct {
				Control bool   `json:"control"`
				RunID   string `json:"run_id"`
			}
			_ = json.Unmarshal(payload, &p)
			if audience != a.id.String() || receive != "agent.events.consume" || kind != "command" || (version != nil && *version != 0) ||
				!p.Control || p.RunID != wantRun || inv.RunID != "" {
				t.Fatalf("command.invoked %s %s %s %s %v", payload, audience, receive, kind, version)
			}
			f, ok := w.nextOf(c, "command.invoked", 15*time.Second)
			if !ok || f.InvocationToken == "" {
				t.Fatal("no command.invoked to the owner")
			}
			e := runstest.ReferenceExpectation{CommandID: ids.MustParse(ctl.ID), ThreadID: ids.MustParse(inv.ThreadID)}
			if wantRun != "" {
				e.RunID = ids.MustParse(wantRun)
			}
			cl, err := w.verifier(a).Check(context.Background(), f.InvocationToken, e)
			if err != nil || cl.TriggerKind != "command" || cl.CommandID != ctl.ID || cl.RunID != wantRun || cl.MessageID != "" {
				t.Fatalf("claims %+v %v", cl, err)
			}
			sendAck(t, c, f.Seq)
		}
		inv := w.mustInvoke(w.sam, ch, ctl.ID, map[string]any{"thread_id": th, "options": opts("ticket", "T-5")})
		check(inv, "")
		if w.count(`SELECT count(*) FROM runs WHERE thread_id = $1`, th) != 0 ||
			w.count(`SELECT count(*) FROM messages WHERE thread_id = $1 AND kind = 'run'`, th) != 0 {
			t.Fatal("a control command opened a run")
		}
		r := w.mention(w.sam, th, a)
		// The delivery of the run's own invocation records its jti first.
		if f, ok := w.nextOf(c, "agent.invocation", 15*time.Second); !ok {
			t.Fatal("no invocation of the mention")
		} else {
			sendAck(t, c, f.Seq)
		}
		r, _ = w.runOf(w.lastTrigger(r.id))
		inv = w.mustInvoke(w.sam, ch, ctl.ID, map[string]any{"thread_id": th})
		check(inv, r.id)
		if r2, _ := w.runOf(w.lastTrigger(r.id)); r2.state != r.state || r2.jti != r.jti || r2.version != r.version {
			t.Fatalf("the control command changed the run %+v, was %+v", r2, r)
		}
		guest := w.guestIn(ch)
		th2 := w.thread(ch)
		w.mention(w.sam, th2, guest)
		inv = w.mustInvoke(w.sam, ch, ctl.ID, map[string]any{"thread_id": th2})
		check(inv, "")
		if n := w.count(`SELECT count(*) FROM event_log WHERE type = 'agent.invocation' AND payload->>'command_id' = $1`, ctl.ID); n != 0 {
			t.Fatal("a control command reached the model")
		}
	})

	t.Run("TestInvokeCommand_Rights", func(t *testing.T) {
		a, ch := w.fresh("rights")
		trig := w.mustRegister(a, escalate("rights"))
		write := w.mustRegister(a, escalate("rights-write"))
		w.sql(`UPDATE commands SET required_level = 'write' WHERE id = $1`, write.ID)
		th := w.thread(ch)
		body := map[string]any{"thread_id": th, "options": opts("ticket", "T-6")}
		code, hidden := w.invoke(w.outsider, ch, trig.ID, body)
		_, unknown := w.invoke(w.outsider, ch, ids.New().String(), body)
		if code != http.StatusNotFound || !sameProblem(hidden, unknown) {
			t.Fatalf("outsider %d %s", code, hidden)
		}
		code, _ = w.invoke(w.sam, w.kampagnen, trig.ID, map[string]any{"options": opts("ticket", "T-6")})
		if code != http.StatusNotFound {
			t.Fatalf("Sam in Kampagnen = %d", code)
		}
		w.must(w.owner, http.MethodPost, "/api/v1/grants", map[string]any{
			"subject_kind": "user", "subject_id": w.robin.user.String(), "target_kind": "channel", "target_id": ch.id, "level": "write", "reason": "test"})
		for _, c := range []commandBody{trig, write} {
			code, b := w.invoke(w.robin, ch, c.ID, body)
			wantProblem(t, code, b, http.StatusForbidden, "agent_invocation_denied", "level_insufficient", "", "")
		}
		w.sql(`UPDATE channels SET read_only = true WHERE id = $1`, ch.id)
		code, b := w.invoke(w.sam, ch, trig.ID, body)
		wantProblem(t, code, b, http.StatusForbidden, "channel_read_only", "channel_read_only", "", "")
		w.sql(`UPDATE channels SET read_only = false WHERE id = $1`, ch.id)
		b2, ch2 := w.fresh("archived")
		arch := w.mustRegister(b2, def("usage", "control", true))
		w.must(w.owner, http.MethodPost, "/api/v1/channels/"+ch2.id+"/archive", map[string]any{"reason": "done"})
		code, b = w.invoke(w.sam, ch2, arch.ID, map[string]any{})
		wantProblem(t, code, b, http.StatusForbidden, "channel_read_only", "channel_archived", "", "")
		guest := w.agent()
		gc := w.mustRegister(guest, def("guestcmd", "control", true))
		code, b = w.invoke(w.sam, ch, gc.ID, map[string]any{"thread_id": th})
		wantProblem(t, code, b, http.StatusForbidden, "agent_invocation_denied", "agent_not_reachable", "", "")
		esk := w.mustRegister(w.lotse, def("only-esk", "control", true, "scope_kind", "channel", "scope_id", w.eskalation.id))
		code, _ = w.invoke(w.sam, w.allgemein, esk.ID, map[string]any{})
		if code != http.StatusNotFound {
			t.Fatalf("not covered = %d", code)
		}
		wide := w.mustRegister(w.lotse, def("shadowed", "control", true))
		w.mustRegister(w.lotse, def("shadowed", "control", true, "scope_kind", "channel", "scope_id", w.allgemein.id))
		code, _ = w.invoke(w.sam, w.allgemein, wide.ID, map[string]any{})
		if code != http.StatusNotFound {
			t.Fatalf("shadowed = %d", code)
		}
		w.denials(t, `action = 'command.invoke' AND actor_principal_id = $1 AND resource_id = $2`, w.outsider.user.String(), th)
		w.denials(t, `action = 'command.invoke' AND actor_principal_id = $1 AND deny_reason = 'level_insufficient'`, w.robin.user.String())
		w.denials(t, `action = 'agent.invoke' AND actor_principal_id = $1 AND deny_reason = 'level_insufficient' AND resource_id = $2`,
			w.robin.user.String(), th)
		w.denials(t, `action = 'agent.invoke' AND actor_principal_id = $1 AND deny_reason = 'agent_not_reachable' AND resource_id = $2`,
			w.sam.user.String(), th)
		w.removeAll(w.lotse)
	})

	t.Run("TestInvokeCommand_I2", func(t *testing.T) {
		a, ch := w.fresh("personal")
		cmd := w.mustRegister(a, def("usage", "control", true))
		w.sql(`UPDATE agents SET billing_mode = 'personal', credential_holder_id = $1 WHERE principal_id = $2`, w.owner.user.String(), a.id.String())
		defer w.sql(`UPDATE agents SET billing_mode = 'organization', credential_holder_id = NULL WHERE principal_id = $1`, a.id.String())
		code, b := w.invoke(w.sam, ch, cmd.ID, map[string]any{})
		wantProblem(t, code, b, http.StatusForbidden, "agent_invocation_denied", "billing_mode_personal_not_holder", "", "")
		w.denials(t, `action = 'agent.invoke' AND deny_reason = 'billing_mode_personal_not_holder' AND actor_principal_id = $1`, w.sam.user.String())
		w.mustInvoke(w.owner, ch, cmd.ID, map[string]any{})
	})

	t.Run("TestInvokeCommand_SR2", func(t *testing.T) {
		a, ch := w.fresh("werkzeuge")
		cmd := w.mustRegister(a, def("build", "control", true))
		w.sql(`UPDATE agents SET tool_class = 'host' WHERE principal_id = $1`, a.id.String())
		code, b := w.invoke(w.sam, ch, cmd.ID, map[string]any{})
		wantProblem(t, code, b, http.StatusForbidden, "agent_invocation_denied", "tool_class_owner_only", "", "")
		w.mustInvoke(w.owner, ch, cmd.ID, map[string]any{})
	})

	t.Run("TestInvokeCommand_SR4", func(t *testing.T) {
		a, ch := w.fresh("selfcall")
		cmd := w.mustRegister(a, def("usage", "control", true))
		w.sql(`UPDATE commands SET required_level = 'write' WHERE id = $1`, cmd.ID)
		code, b, _ := w.do(send{agent: &a, method: http.MethodPost, path: "/api/v1/channels/" + ch.id + "/commands/" + cmd.ID + "/invoke",
			body: map[string]any{}, headers: map[string]string{"Idempotency-Key": ids.New().String()}})
		// agent.invoke needs trigger, which an agent never holds (07 2.1), before its origin rule.
		wantProblem(t, code, b, http.StatusForbidden, "agent_invocation_denied", "level_insufficient", "", "")
		if w.count(`SELECT count(*) FROM event_log WHERE type = 'command.invoked' AND resource_id = $1`, cmd.ID) != 0 {
			t.Fatal("an agent invoked a command")
		}
	})

	t.Run("TestInvokeCommand_OptionValidation", func(t *testing.T) {
		a, ch := w.fresh("options")
		secret := "OPTSECRET" + ids.New().String()[24:]
		cmd := w.mustRegister(a, def("usage", "control", true, "options", []any{
			map[string]any{"name": "ticket", "description": "Ticket", "type": "string", "required": true},
			map[string]any{"name": "count", "description": "Count", "type": "integer", "required": false},
			map[string]any{"name": "urgent", "description": "Urgent", "type": "boolean", "required": false},
			map[string]any{"name": "prio", "description": "Priority", "type": "choice", "required": false,
				"choices": []any{map[string]any{"name": "High", "value": "high"}}},
			map[string]any{"name": "owner", "description": "Owner", "type": "user", "required": false},
			map[string]any{"name": "file", "description": "File", "type": "file", "required": false},
		}))
		cases := []struct {
			values        []any
			pointer, code string
		}{
			{opts("count", "1"), "/options", "required"},
			{opts("ticket", "a", "nope", secret), "/options/1/name", "unknown_member"},
			{opts("ticket", "a", "ticket", secret), "/options/1/name", "invalid"},
			{opts("ticket", secret+"\u202e"), "/options/0/value", "invalid"},
			{opts("ticket", "a", "count", "007"), "/options/1/value", "invalid_format"},
			{opts("ticket", "a", "urgent", secret), "/options/1/value", "invalid_enum"},
			{opts("ticket", "a", "prio", secret), "/options/1/value", "invalid_enum"},
			{opts("ticket", "a", "owner", secret), "/options/1/value", "invalid_format"},
			{opts("ticket", "a", "file", ids.New().String()), "/options/1/value", "not_enabled"},
			{opts("ticket", strings.Repeat("x", cmdPayloadMax)), "/options", "too_large"},
		}
		for _, tc := range cases {
			code, b := w.invoke(w.sam, ch, cmd.ID, map[string]any{"options": tc.values})
			wantProblem(t, code, b, http.StatusBadRequest, "validation_failed", "", tc.pointer, tc.code)
			if strings.Contains(string(b), secret) {
				t.Fatalf("the error body repeats a value: %s", b)
			}
		}
		if w.count(`SELECT count(*) FROM event_log WHERE type = 'command.invoked' AND resource_id = $1`, cmd.ID) != 0 {
			t.Fatal("a refused invocation wrote an event")
		}
	})

	t.Run("TestInvokeCommand_Idempotency", func(t *testing.T) {
		a, ch := w.fresh("idem")
		cmd := w.mustRegister(a, escalate("idem"))
		th := w.thread(ch)
		key := ids.New().String()
		path := "/api/v1/channels/" + ch.id + "/commands/" + cmd.ID + "/invoke"
		body := map[string]any{"thread_id": th, "options": opts("ticket", "T-7")}
		code, first, _ := w.do(send{person: &w.sam, method: http.MethodPost, path: path, body: body, headers: map[string]string{"Idempotency-Key": key}})
		if code != http.StatusAccepted {
			t.Fatalf("first = %d %s", code, first)
		}
		code, again, h := w.do(send{person: &w.sam, method: http.MethodPost, path: path, body: body, headers: map[string]string{"Idempotency-Key": key}})
		if code != http.StatusAccepted || !sameProblem(first, again) || h.Get("Idempotent-Replay") != "true" {
			t.Fatalf("replay = %d %s %v", code, again, h)
		}
		if w.count(`SELECT count(*) FROM runs WHERE triggering_command_id = $1`, cmd.ID) != 1 ||
			w.count(`SELECT count(*) FROM event_log WHERE type = 'agent.invocation' AND payload->>'command_id' = $1`, cmd.ID) != 1 {
			t.Fatal("the replay wrote again")
		}
		code, b, _ := w.do(send{person: &w.sam, method: http.MethodPost, path: path, body: map[string]any{"thread_id": th, "options": opts("ticket", "T-8")},
			headers: map[string]string{"Idempotency-Key": key}})
		wantProblem(t, code, b, http.StatusUnprocessableEntity, "idempotency_key_reuse", "", "", "")
	})

	t.Run("TestInvokeCommand_RecheckAtDelivery", func(t *testing.T) {
		a, ch := w.fresh("recheck")
		first := w.connect(t, a)
		first.Close()
		p := w.newPerson("sam", identity.DeviceStateActive)
		g := w.triggerGrant(p, ch)
		ctl := w.mustRegister(a, def("usage", "control", true))
		model := w.mustRegister(a, escalate("recheck"))
		ci := w.mustInvoke(p, ch, ctl.ID, map[string]any{"thread_id": w.thread(ch)})
		mi := w.mustInvoke(p, ch, model.ID, map[string]any{"thread_id": w.thread(ch), "options": opts("ticket", "T-9")})
		w.must(w.owner, http.MethodPost, "/api/v1/grants/"+g+"/revoke", map[string]any{"reason": "left the team"})
		c, _ := w.ready(t, w.agentParams(a, ""))
		if f, ok := w.nextOf(c, "command.invoked", 5*time.Second); ok {
			t.Fatalf("a revoked control command was delivered: %+v", f)
		}
		eventually(t, 10*time.Second, "the failed command run", func() bool {
			return w.count(`SELECT count(*) FROM runs WHERE id = $1 AND state = 'failed' AND state_reason = 'access_revoked'`, mi.RunID) == 1
		})
		if n := w.count(`SELECT count(*) FROM audit_events WHERE action = 'agent.invoke' AND decision = 'deny' AND actor_principal_id = $1
			AND details->>'stage' = 'delivery' AND details->>'event_type' = 'command.invoked' AND details->>'command_id' = $2
			AND details->>'trigger_kind' = 'command'`, p.user.String(), ctl.ID); n != 1 {
			t.Fatalf("deny entries of command.invoked: %d (%s)", n, ci.InvocationID)
		}
		if n := w.count(`SELECT count(*) FROM audit_events WHERE action = 'agent.invoke' AND decision = 'deny' AND actor_principal_id = $1
			AND details->>'stage' = 'delivery' AND details->>'event_type' = 'agent.invocation' AND details->>'run_id' = $2
			AND details->>'trigger_kind' = 'command'`, p.user.String(), mi.RunID); n != 1 {
			t.Fatalf("deny entries of agent.invocation: %d", n)
		}
	})

	t.Run("TestCommandsNeverLeakValues", func(t *testing.T) {
		a, ch := w.fresh("leak")
		desc, value := "DESCMARK"+ids.New().String()[24:], "VALUEMARK"+ids.New().String()[24:]
		cmd := w.mustRegister(a, def("leak", "description", desc, "control", true, "options", []any{
			map[string]any{"name": "ticket", "description": "Ticket", "type": "string", "required": true}}))
		w.mustInvoke(w.sam, ch, cmd.ID, map[string]any{"options": opts("ticket", value)})
		code, b := w.invoke(w.sam, ch, cmd.ID, map[string]any{"options": opts("ticket", value+"\u202e")})
		if code != http.StatusBadRequest || strings.Contains(string(b), value) {
			t.Fatalf("error body %d %s", code, b)
		}
		for _, m := range []string{desc, value} {
			if n := w.count(`SELECT count(*) FROM audit_events WHERE details::text LIKE '%' || $1 || '%'`, m); n != 0 {
				t.Fatal("a marker is in audit_events")
			}
			if strings.Contains(w.p.logs.String(), m) {
				t.Fatal("a marker is in the log")
			}
		}
		if n := w.count(`SELECT count(*) FROM event_log WHERE payload::text LIKE '%' || $1 || '%' AND type NOT IN ('command.changed', 'command.invoked')`,
			value); n != 0 {
			t.Fatal("the value is in another event")
		}
	})

	t.Run("TestRegisterCommand_HumansRefused", func(t *testing.T) {
		for _, p := range []gwPerson{w.sam, w.owner} {
			code, b, _ := w.do(send{person: &p, method: http.MethodPost, path: "/api/v1/commands", body: def("human"),
				headers: map[string]string{"Idempotency-Key": ids.New().String()}})
			wantProblem(t, code, b, http.StatusForbidden, "forbidden", "subject_kind_not_allowed", "", "")
			w.denials(t, `action = 'command.register' AND actor_principal_id = $1`, p.user.String())
		}
	})
}

// denials waits until exactly one deny entry matches cond (the authz layer writes denials outside
// the business transaction, aggregated per audit.deny_aggregate_window).
func (w *cmdWorld) denials(t *testing.T, cond string, args ...any) {
	t.Helper()
	q := `SELECT count(*) FROM audit_events WHERE decision = 'deny' AND ` + cond
	eventually(t, 10*time.Second, "the deny entry", func() bool { return w.count(q, args...) >= 1 })
	time.Sleep(1500 * time.Millisecond)
	if n := w.count(q, args...); n != 1 {
		t.Fatalf("%d deny entries for %s", n, cond)
	}
}

// wantSchema validates p against the schema name of the contract and refuses null.
func wantSchema(t *testing.T, name string, p []byte) {
	t.Helper()
	if err := validateSchema(name, p); err != nil {
		t.Fatalf("%s: %v: %s", name, err, p)
	}
}

// validateSchema validates p against the schema name of docs/api/openapi.yaml; null is refused.
func validateSchema(name string, p []byte) error {
	if bytes.Contains(p, []byte("null")) {
		return errors.New("null in the payload")
	}
	doc, err := libopenapi.NewDocument(rest.Spec)
	if err != nil {
		return err
	}
	model, err := doc.BuildV3Model()
	if err != nil {
		return err
	}
	proxy, ok := model.Model.Components.Schemas.Get(name)
	if !ok {
		return errors.New("no schema " + name)
	}
	if valid, problems := schema_validation.NewSchemaValidator().ValidateSchemaString(proxy.Schema(), string(p)); !valid {
		var msgs []string
		for _, pr := range problems {
			msgs = append(msgs, pr.Message+" "+pr.Reason)
		}
		return errors.New(strings.Join(msgs, "; "))
	}
	return nil
}

// guestIn creates an agent that may be mentioned as a guest in ch (allow_mentioned_agents, its
// own write on ch) and that Sam can mention: it operates a channel of Support of its own (07 4.6,
// agent.read).
func (w *cmdWorld) guestIn(ch channel) consumerAgent {
	w.t.Helper()
	g, _ := w.fresh("guest-home")
	w.sql(`UPDATE channels SET allow_mentioned_agents = true WHERE id = $1`, ch.id)
	w.agentGrant(g, ch, "write")
	return g
}

// lastTrigger returns the triggering message of run.
func (w *cmdWorld) lastTrigger(run string) string {
	w.t.Helper()
	var m string
	if err := w.queryRow(context.Background(), `SELECT triggering_message_id::text FROM runs WHERE id = $1`, run).Scan(&m); err != nil {
		w.t.Fatal(err)
	}
	return m
}

// removeAll removes every live command of a, so the next subtest starts below the limit.
func (w *cmdWorld) removeAll(a consumerAgent) {
	w.t.Helper()
	rows, err := w.gwWorld.owner.Query(context.Background(), `SELECT id::text FROM commands WHERE owner_principal_id = $1 AND removed_at IS NULL`,
		a.id.String())
	if err != nil {
		w.t.Fatal(err)
	}
	var list []string
	for rows.Next() {
		var id string
		_ = rows.Scan(&id)
		list = append(list, id)
	}
	rows.Close()
	for _, id := range list {
		if code, b, _ := w.do(send{agent: &a, method: http.MethodDelete, path: "/api/v1/commands/" + id}); code != http.StatusNoContent {
			w.t.Fatalf("removeCommand = %d %s", code, b)
		}
	}
}

// Keep the helpers of later subtests referenced while the suite grows.
var (
	_ = slices.Contains[[]string]
	_ = strings.Contains
	_ = sync.Mutex{}
	_ = runstest.ReferenceExpectation{}
)
