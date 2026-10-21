// SPDX-License-Identifier: Apache-2.0

package runs_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pb33f/libopenapi"
	"github.com/pb33f/libopenapi-validator/schema_validation"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
	"github.com/OWNER/bothy/services/server/internal/runs"
)

// expiryJob is one scheduled runs.component_expiry.
type expiryJob struct {
	Message ids.ID
	At      time.Time
}

// expiryJobs records the scheduled jobs runs.component_expiry (runsapp.ComponentExpiryScheduler);
// the world has no job runtime, so the tests call ExpireComponents directly.
type expiryJobs struct {
	mu   sync.Mutex
	list []expiryJob
}

// ScheduleComponentExpiry implements runsapp.ComponentExpiryScheduler.
func (e *expiryJobs) ScheduleComponentExpiry(_ context.Context, message ids.ID, at time.Time) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.list = append(e.list, expiryJob{Message: message, At: at})
	return nil
}

// of returns the jobs of the card message.
func (e *expiryJobs) of(message string) []expiryJob {
	e.mu.Lock()
	defer e.mu.Unlock()
	var out []expiryJob
	for _, j := range e.list {
		if j.Message.String() == message {
			out = append(out, j)
		}
	}
	return out
}

// startedRun opens a run of Lotse for Alex in a new thread of Support/Allgemein, delivers it and
// starts it; it returns the run and the start token.
func (w *world) startedRun() (runRow, string) {
	w.t.Helper()
	r, _ := w.openRun()
	tok := w.deliver(r.ID)
	w.wantOK(w.start("lotse", r.ID, tok))
	return r, tok
}

// waitingRun is startedRun with awaitRunInput; it returns the run.
func (w *world) waitingRun() runRow {
	w.t.Helper()
	r, _ := w.startedRun()
	w.wantOK(w.await("lotse", r.ID, "Waiting for approval"))
	return r
}

// cardBody is the body of a card message: kind card, the content, run_id when not "" and the
// components JSON array.
func cardBody(content, run, components string) string {
	c, _ := json.Marshal(content)
	body := `{"client_msg_id":"` + ids.New().String() + `","kind":"card","content_md":` + string(c) + `,"components":` + components
	if run != "" {
		body += `,"run_id":"` + run + `"`
	}
	return body + "}"
}

// postCard sends a card as handle into thread.
func (w *world) postCard(handle string, thread ids.ID, run, components string) *httpservertest.Response {
	return w.do(handle, http.MethodPost, "/api/v1/threads/"+thread.String()+"/messages", cardBody("Approve the refund?", run, components), key()...)
}

// card sends a card and expects 201; it returns the message and its components by custom_id.
func (w *world) card(handle string, thread ids.ID, run, components string) (map[string]any, map[string]map[string]any) {
	w.t.Helper()
	m := w.want(w.postCard(handle, thread, run, components), http.StatusCreated, "")
	return m, componentsByID(m)
}

// componentsByID indexes the components of a message body by custom_id.
func componentsByID(m map[string]any) map[string]map[string]any {
	out := map[string]map[string]any{}
	list, _ := m["components"].([]any)
	for _, c := range list {
		cm := c.(map[string]any)
		out[cm["custom_id"].(string)] = cm
	}
	return out
}

// threadOf reads the thread of a run.
func (w *world) threadOf(run string) ids.ID {
	return ids.MustParse(w.text(`SELECT thread_id::text FROM runs WHERE id = $1`, run))
}

// The components of the tests.
const (
	twoButtons = `[{"custom_id":"approve","type":"button","definition":{"label":"Approve","value":"approve","style":"success"},"restricted_to":"any"},` +
		`{"custom_id":"deny","type":"button","definition":{"label":"Deny","value":"deny","style":"danger"},"restricted_to":"any"}]`
	threeKinds = `[{"custom_id":"go","type":"button","definition":{"label":"Go","value":"go"}},` +
		`{"custom_id":"pick","type":"select","definition":{"placeholder":"Choose","options":[{"label":"One","value":"one"},{"label":"Two","value":"two"}]}},` +
		`{"custom_id":"ask","type":"form","definition":{"title":"Details","fields":[{"name":"reason","label":"Reason","type":"text","required":true}]}}]`
	// approvalCard holds the buttons of 09 11.5.
	approvalCard = `[{"custom_id":"once","type":"button","definition":{"label":"Allow once","value":"once","style":"primary"},"required_capability":"approve_actions"},` +
		`{"custom_id":"session","type":"button","definition":{"label":"Allow for this session","value":"session"},"required_capability":"approve_actions"},` +
		`{"custom_id":"always","type":"button","definition":{"label":"Always allow","value":"always"},"required_capability":"approve_actions","restricted_to":"owner"},` +
		`{"custom_id":"deny","type":"button","definition":{"label":"Deny","value":"deny","style":"danger"},"required_capability":"approve_actions"}]`
)

// click sends createInteraction as name on component with the values JSON ("" for none) and a
// fresh client_interaction_id.
func (w *world) click(name, component, values string) *httpservertest.Response {
	return w.clickWith(name, component, ids.New().String(), values)
}

// clickWith sends createInteraction with client.
func (w *world) clickWith(name, component, client, values string) *httpservertest.Response {
	body := `{"client_interaction_id":"` + client + `"`
	if values != "" {
		body += `,"values":` + values
	}
	return w.do(name, http.MethodPost, "/api/v1/components/"+component+"/interactions", body+"}")
}

// componentRow is the part of a message_components row the tests read.
type componentRow struct {
	ID, CustomID, State, UsedBy, Run string
	Version                          int64
	Position                         int
	SingleUse                        bool
	Restricted, Capability           string
	ExpiresAt                        time.Time
}

// componentsOf reads the components of a card message by position.
func (w *world) componentsOf(message string) []componentRow {
	w.t.Helper()
	rs, err := w.db.Query(context.Background(), `SELECT id::text, custom_id, state, coalesce(used_by::text, ''), coalesce(run_id::text, ''),
		version, position, single_use, restricted_to, coalesce(required_capability, ''), coalesce(expires_at, 'epoch'::timestamptz)
		FROM message_components WHERE message_id = $1 ORDER BY position`, message)
	must(w.t, err)
	defer rs.Close()
	var out []componentRow
	for rs.Next() {
		var c componentRow
		must(w.t, rs.Scan(&c.ID, &c.CustomID, &c.State, &c.UsedBy, &c.Run, &c.Version, &c.Position, &c.SingleUse, &c.Restricted, &c.Capability, &c.ExpiresAt))
		out = append(out, c)
	}
	return out
}

// statesOf returns custom_id -> state of the components of message.
func (w *world) statesOf(message string) map[string]string {
	out := map[string]string{}
	for _, c := range w.componentsOf(message) {
		out[c.CustomID] = c.State
	}
	return out
}

// messageVersion reads messages.version.
func (w *world) messageVersion(message string) int64 {
	return w.count(`SELECT version FROM messages WHERE id = $1`, message)
}

// interactionState reads interactions.state.
func (w *world) interactionState(interaction string) string {
	return w.text(`SELECT state FROM interactions WHERE id = $1`, interaction)
}

// eventPayloadOf reads the payload of the last event of typ on resource.
func (w *world) eventPayloadOf(typ, resource string) json.RawMessage {
	w.t.Helper()
	var raw string
	must(w.t, w.db.QueryRow(context.Background(), `SELECT payload::text FROM event_log WHERE type = $1 AND resource_id = $2 ORDER BY seq DESC LIMIT 1`,
		typ, resource).Scan(&raw))
	return json.RawMessage(raw)
}

// deliverInteraction prepares the delivery of interaction.created of interaction to Lotse
// through runs.Delivery with the stored payload, the way the gateway does.
func (w *world) deliverInteraction(interaction string) runs.DeliveryOutcome {
	w.t.Helper()
	p := w.eventPayloadOf("interaction.created", interaction)
	agent := w.text(`SELECT audience_principal_id::text FROM event_log WHERE type = 'interaction.created' AND resource_id = $1`, interaction)
	out, err := w.runs.Prepare(context.Background(), runs.DeliveryInput{AgentID: ids.MustParse(agent), EventID: ids.New(),
		Type: "interaction.created", ResourceKind: "interaction", ResourceID: ids.MustParse(interaction), Payload: p})
	must(w.t, err)
	return out
}

// version returns the version member of a body as an int64.
func version(m map[string]any) int64 { return int64(m["version"].(float64)) }

// ifMatchOnly is the If-Match header of v and a fresh Idempotency-Key.
func ifMatchOnly(v int64) []string {
	return []string{"If-Match", `"` + strconv.FormatInt(v, 10) + `"`, "Idempotency-Key", ids.New().String()}
}

// validateSchema checks one payload against the schema name of openapi.yaml; it carries no JSON
// null (ADR 0053).
func validateSchema(t *testing.T, name string, p []byte) {
	t.Helper()
	if strings.Contains(string(p), "null") {
		t.Fatalf("null in %s", p)
	}
	doc, err := libopenapi.NewDocument(rest.Spec)
	must(t, err)
	model, err := doc.BuildV3Model()
	must(t, err)
	proxy, ok := model.Model.Components.Schemas.Get(name)
	if !ok {
		t.Fatalf("no schema %s", name)
	}
	if valid, problems := schema_validation.NewSchemaValidator().ValidateSchemaString(proxy.Schema(), string(p)); !valid {
		t.Fatalf("%s does not match %s: %v", p, name, problems)
	}
}

// deliveryOf is the delivery input of agent.invocation of run with its stored payload.
func deliveryOf(w *world, run string) runs.DeliveryInput {
	agent := w.text(`SELECT agent_id::text FROM runs WHERE id = $1`, run)
	return runs.DeliveryInput{AgentID: ids.MustParse(agent), EventID: ids.New(), Type: "agent.invocation", ResourceKind: "run",
		ResourceID: ids.MustParse(run), Payload: w.eventPayloadOf("agent.invocation", run)}
}
