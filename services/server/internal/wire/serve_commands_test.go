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
	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/adapters/identitytest"
	"github.com/OWNER/bothy/services/server/internal/identity/domain/dpop"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/runs/adapters/runstest"
)

// The limits of the command world: low enough to reach in a test, never the defaults.
const (
	cmdMaxPerPrincipal = 4
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

// audits counts the audit entries of action on the command id with decision.
func (w *cmdWorld) audits(action, id, decision string) int {
	return w.count(`SELECT count(*) FROM audit_events WHERE action = $1 AND resource_id = $2 AND decision = $3`, action, id, decision)
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

// thread creates a thread of a message of the Owner in the main thread of ch and returns it.
func (w *cmdWorld) thread(ch channel) string {
	w.t.Helper()
	code, b := w.rest(w.base, w.owner, http.MethodPost, "/api/v1/channels/"+ch.id+"/threads", map[string]any{
		"title": "Case", "message": map[string]any{"client_msg_id": ids.New().String(), "content_md": "a case"}})
	if code != http.StatusCreated && code != http.StatusOK {
		w.t.Fatalf("createThread = %d %s", code, b)
	}
	var out struct {
		Thread struct {
			ID string `json:"id"`
		} `json:"thread"`
		ID string `json:"id"`
	}
	_ = json.Unmarshal(b, &out)
	if out.Thread.ID != "" {
		return out.Thread.ID
	}
	return out.ID
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
		if w.changed(c.ID, "registered") != 1 || w.audits("command.register", c.ID, "allow") != 1 {
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
