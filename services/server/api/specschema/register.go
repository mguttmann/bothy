// SPDX-License-Identifier: Apache-2.0

package specschema

import (
	"fmt"
	"slices"
	"strings"
)

// CatalogueEvent is one event type of docs/spec/actions.yaml.
type CatalogueEvent struct {
	Kind            string // durable or ephemeral
	ReceiveAction   string // receive_action, for example message.read or self
	ResourceVersion bool   // resource_version
}

// CatalogueOp is one WebSocket op of docs/spec/actions.yaml (ws_ops).
type CatalogueOp struct {
	Direction string // for example client_to_server
	Action    string
}

// Inbound reports whether a client, an agent or an app sends the op.
func (o CatalogueOp) Inbound() bool { return strings.HasSuffix(o.Direction, "_to_server") }

// Catalogue is the part of docs/spec/actions.yaml that the register test compares.
type Catalogue struct {
	Events map[string]CatalogueEvent // by event type
	Ops    map[string]CatalogueOp    // by op
}

// readCatalogue reads events and ws_ops of actions.yaml.
func readCatalogue(doc map[string]any) (Catalogue, error) {
	cat := Catalogue{Events: map[string]CatalogueEvent{}, Ops: map[string]CatalogueOp{}}
	events, _ := doc["events"].([]any)
	for _, e := range events {
		m, _ := e.(map[string]any)
		t, _ := m["type"].(string)
		kind, _ := m["kind"].(string)
		recv, _ := m["receive_action"].(string)
		rv, _ := m["resource_version"].(bool)
		if t == "" {
			return Catalogue{}, fmt.Errorf("specschema: actions.yaml event without type: %v", e)
		}
		cat.Events[t] = CatalogueEvent{Kind: kind, ReceiveAction: recv, ResourceVersion: rv}
	}
	ops, _ := doc["ws_ops"].([]any)
	for _, e := range ops {
		m, _ := e.(map[string]any)
		op, _ := m["op"].(string)
		dir, _ := m["direction"].(string)
		action, _ := m["action"].(string)
		if op == "" {
			return Catalogue{}, fmt.Errorf("specschema: actions.yaml ws_op without op: %v", e)
		}
		cat.Ops[op] = CatalogueOp{Direction: dir, Action: action}
	}
	if len(cat.Events) == 0 || len(cat.Ops) == 0 {
		return Catalogue{}, fmt.Errorf("specschema: actions.yaml has no events or ws_ops")
	}
	return cat, nil
}

// AsyncAPI is the gateway register of asyncapi.yaml: its operations, its event messages and the
// schema names of openapi.yaml the messages point to.
type AsyncAPI struct {
	Operations []Operation
	Events     map[string]EventMessage
	Schemas    map[string]bool
}

// RegisteredOp is one client-to-server op the server registers, with the action its dispatcher
// requires and the rate-limit class it counts.
type RegisteredOp struct {
	Op, Action, RateLimitClass string
}

// RegisteredEvent is one event type a module registers, with the kind and receive action the
// server's catalogue gives it.
type RegisteredEvent struct {
	Type, Kind, ReceiveAction string
}

// LaterOp is one entry of gateway_ops.later of services/server/api/operations.yaml.
type LaterOp struct {
	Op, Milestone string
}

// Register is the runtime register of the gateway: the registered ops and event types and the
// enable list gateway_ops (PLAN P-04).
type Register struct {
	Ops     []RegisteredOp
	Events  []RegisteredEvent
	Enabled []string
	Later   []LaterOp
}

// Problem is one difference Compare found.
type Problem struct {
	// Subject is the op or event type, for example "event test.unlisted".
	Subject string
	// Text says what differs.
	Text string
}

// String returns subject and text.
func (p Problem) String() string { return p.Subject + ": " + p.Text }

// SchemaName returns the openapi schema of an event type: PascalCase of its parts plus Event, for
// example read_state.updated -> ReadStateUpdatedEvent.
func SchemaName(eventType string) string {
	var b strings.Builder
	for _, part := range strings.FieldsFunc(eventType, func(r rune) bool { return r == '.' || r == '_' }) {
		b.WriteString(strings.ToUpper(part[:1]) + part[1:])
	}
	return b.String() + "Event"
}

// Compare holds the runtime register against the catalogue and asyncapi.yaml (gate G9, 05 "Tests
// that hold this catalogue" item 2, AZ-P2) and returns every difference, sorted:
//
//   - each registered op exists as receive.<op> with the same x-authz-action and
//     x-rate-limit-class, and as an inbound ws_op of actions.yaml with the same action; a wrong
//     action is one problem, whichever source it differs from;
//   - each registered event type exists in actions.yaml with the same kind and receive action, in
//     asyncapi.yaml as event.<type> with the same x-receive-action and x-event-kind and the
//     schema <PascalCase>Event of openapi.yaml;
//   - every enabled op is registered, no later op is registered, and together they are exactly the
//     client-to-server ops of asyncapi.yaml, each later op with a milestone.
func Compare(reg Register, actions Catalogue, async AsyncAPI) []Problem {
	var p problems
	clientOps := map[string]Operation{}
	for _, o := range async.Operations {
		if !o.Send {
			clientOps[o.Op] = o
		}
	}
	registered := map[string]bool{}
	for _, r := range reg.Ops {
		registered[r.Op] = true
		p.compareOp(r, actions, clientOps)
	}
	for _, e := range reg.Events {
		p.compareEvent(e, actions, async)
	}
	p.compareEnableList(reg, registered, clientOps)
	out := p.list
	slices.SortFunc(out, func(a, b Problem) int { return strings.Compare(a.String(), b.String()) })
	return out
}

// problems collects the differences of one Compare.
type problems struct{ list []Problem }

// add appends one problem.
func (p *problems) add(subject, format string, args ...any) {
	p.list = append(p.list, Problem{Subject: subject, Text: fmt.Sprintf(format, args...)})
}

// compareOp holds one registered op against asyncapi.yaml and actions.yaml.
func (p *problems) compareOp(r RegisteredOp, actions Catalogue, clientOps map[string]Operation) {
	subject := "op " + r.Op
	a, inAsync := clientOps[r.Op]
	c, inCat := actions.Ops[r.Op]
	inCat = inCat && c.Inbound()
	if !inAsync {
		p.add(subject, "registered, but asyncapi.yaml has no receive.%s", r.Op)
	}
	if !inCat {
		p.add(subject, "registered, but actions.yaml has no client-to-server ws_op %s", r.Op)
	}
	if (inAsync && a.AuthzAction != r.Action) || (inCat && c.Action != r.Action) {
		p.add(subject, "registered with action %s; asyncapi.yaml says %q, actions.yaml says %q", r.Action, a.AuthzAction, c.Action)
	}
	if inAsync && a.RateLimitClass != r.RateLimitClass {
		p.add(subject, "counts rate-limit class %q; asyncapi.yaml says %q", r.RateLimitClass, a.RateLimitClass)
	}
}

// compareEvent holds one registered event type against actions.yaml, asyncapi.yaml and the
// schema names of openapi.yaml.
func (p *problems) compareEvent(e RegisteredEvent, actions Catalogue, async AsyncAPI) {
	subject := "event " + e.Type
	c, inCat := actions.Events[e.Type]
	m, inAsync := async.Events[e.Type]
	switch {
	case !inCat:
		p.add(subject, "registered, but actions.yaml has no such event type")
	case c.Kind != e.Kind || c.ReceiveAction != e.ReceiveAction:
		p.add(subject, "registered as %s with receive action %s; actions.yaml says %s with %s", e.Kind, e.ReceiveAction, c.Kind, c.ReceiveAction)
	}
	if !inAsync {
		p.add(subject, "registered, but asyncapi.yaml has no message event.%s in send.dispatch", e.Type)
		return
	}
	if m.Kind != e.Kind || m.ReceiveAction != e.ReceiveAction {
		p.add(subject, "registered as %s with receive action %s; asyncapi.yaml says x-event-kind %q, x-receive-action %q",
			e.Kind, e.ReceiveAction, m.Kind, m.ReceiveAction)
	}
	if want := SchemaName(e.Type); m.Schema != want || !async.Schemas[want] {
		p.add(subject, "message body is %q, want the openapi.yaml schema %s", m.Schema, want)
	}
}

// compareEnableList holds gateway_ops against the registered ops and the client ops of
// asyncapi.yaml.
func (p *problems) compareEnableList(reg Register, registered map[string]bool, clientOps map[string]Operation) {
	enabled := map[string]bool{}
	for _, op := range reg.Enabled {
		enabled[op] = true
		if !registered[op] {
			p.add("op "+op, "in gateway_ops.enabled, but not registered")
		}
	}
	later := p.compareLater(reg.Later, registered, enabled)
	for op := range registered {
		if !enabled[op] {
			p.add("op "+op, "registered, but not in gateway_ops.enabled")
		}
	}
	for op := range clientOps {
		if !enabled[op] && !later[op] {
			p.add("op "+op, "a client op of asyncapi.yaml in neither gateway_ops.enabled nor gateway_ops.later")
		}
	}
	p.notClientOps("enabled", enabled, clientOps)
	p.notClientOps("later", later, clientOps)
}

// compareLater checks gateway_ops.later and returns its ops.
func (p *problems) compareLater(list []LaterOp, registered, enabled map[string]bool) map[string]bool {
	later := map[string]bool{}
	for _, l := range list {
		later[l.Op] = true
		if registered[l.Op] {
			p.add("op "+l.Op, "in gateway_ops.later, but registered")
		}
		if enabled[l.Op] {
			p.add("op "+l.Op, "in gateway_ops.enabled and gateway_ops.later")
		}
		if l.Milestone == "" {
			p.add("op "+l.Op, "in gateway_ops.later without a milestone")
		}
	}
	return later
}

// notClientOps reports every op of the list section that is no client op of asyncapi.yaml.
func (p *problems) notClientOps(section string, ops map[string]bool, clientOps map[string]Operation) {
	for op := range ops {
		if _, ok := clientOps[op]; !ok {
			p.add("op "+op, "in gateway_ops.%s, but no client op of asyncapi.yaml", section)
		}
	}
}
