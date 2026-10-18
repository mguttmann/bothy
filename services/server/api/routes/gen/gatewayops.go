// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"
)

// The gateway part of the generator (M1-64): the section gateway_ops of the enable list names
// every op a client, agent or app sends on the channel gateway of docs/api/asyncapi.yaml, either
// as enabled (the dispatcher registers a handler) or as later (with the milestone that builds it).
// The terminal channels are not gateway ops and stay out of the list.

// gatewayChannelRef is the channel reference of a gateway operation in the AsyncAPI document.
const gatewayChannelRef = "#/channels/gateway"

// gatewayLaterKeys are the only keys of a later entry of gateway_ops.
var gatewayLaterKeys = []string{"op", "milestone", "ticket"}

// gatewayOpsKeys are the only keys of the section gateway_ops.
var gatewayOpsKeys = []string{sectionEnabled, sectionLater}

// gatewayOp is one enabled client-to-server op of the gateway with the action of its
// x-authz-action and the name of the schema of its body d in openapi.yaml.
type gatewayOp struct {
	Op, Action, Schema string
}

// gatewayLater is one later entry of gateway_ops.
type gatewayLater struct {
	Op        string `yaml:"op"`
	Milestone string `yaml:"milestone"`
	Ticket    string `yaml:"ticket"`
}

// gatewayOpsList is the decoded section gateway_ops.
type gatewayOpsList struct {
	Enabled []string       `yaml:"enabled"`
	Later   []gatewayLater `yaml:"later"`
}

// loadGatewayOps reads the client-to-server ops of the channel gateway from the AsyncAPI document
// at asyncPath and checks the section gateway_ops of the list at listPath against them: the
// section exists, both parts are sorted and free of duplicates, every entry names such an op,
// none is in both parts, every op is in one part, and the later entries carry a valid milestone and
// ticket. It also checks that the body schema of every client op refuses unknown members (W7,
// SEC-IN-1, as checkRequestSchemasClosed does for request bodies). in supplies nothing yet but
// keeps the signature next to loadInputs; specPath is the OpenAPI document of the bodies.
//
// It returns the enabled ops sorted by op; the error is a *problemsError naming every problem.
func loadGatewayOps(_ *inputs, specPath, listPath, asyncPath string) ([]gatewayOp, error) {
	p := &problemsError{}
	ops := readAsyncGatewayOps(asyncPath, p)
	list := readGatewaySection(listPath, p)
	specDoc, _, err := readYAML(specPath)
	if err != nil {
		p.add(specPath, "%v", err)
	}
	if list == nil || ops == nil || specDoc == nil {
		return nil, p.err()
	}
	byOp := map[string]gatewayOp{}
	var schemas []string
	for _, op := range ops {
		byOp[op.Op] = op
		schemas = append(schemas, op.Schema)
	}
	checkGatewayList(listPath, asyncPath, list, byOp, p)
	checkFrameSchemasClosed(specPath, specDoc, schemas, p)
	if err := p.err(); err != nil {
		return nil, err
	}
	out := make([]gatewayOp, 0, len(list.Enabled))
	for _, name := range list.Enabled {
		out = append(out, byOp[name])
	}
	slices.SortFunc(out, func(a, b gatewayOp) int { return strings.Compare(a.Op, b.Op) })
	return out, nil
}

// readGatewaySection reads the section gateway_ops of the list at path; a list without it is a
// problem, because the generator runs with an AsyncAPI document.
func readGatewaySection(path string, p *problemsError) *gatewayOpsList {
	doc, _, err := readYAML(path)
	if err != nil {
		p.add(path, "%v", err)
		return nil
	}
	n := value(doc.Content[0], sectionGatewayOps)
	if n == nil {
		p.add(path, "missing key %s: every client op of the channel gateway in the AsyncAPI document must be listed", sectionGatewayOps)
		return nil
	}
	knownKeys(path, sectionGatewayOps, n, gatewayOpsKeys, p)
	for _, k := range gatewayOpsKeys {
		if value(n, k) == nil {
			p.add(path, "%s: missing key %s", sectionGatewayOps, k)
		}
	}
	for i, e := range items(value(n, sectionLater)) {
		knownKeys(path, fmt.Sprintf("%s.later[%d]", sectionGatewayOps, i), e, gatewayLaterKeys, p)
	}
	var list gatewayOpsList
	if err := n.Decode(&list); err != nil {
		p.add(path, "%s: %v", sectionGatewayOps, err)
		return nil
	}
	return &list
}

// checkGatewayList checks list against the client ops byOp of the AsyncAPI document.
func checkGatewayList(listPath, asyncPath string, list *gatewayOpsList, byOp map[string]gatewayOp, p *problemsError) {
	enabledSection := sectionGatewayOps + "." + sectionEnabled
	laterSection := sectionGatewayOps + "." + sectionLater
	later := make([]string, len(list.Later))
	for i, e := range list.Later {
		later[i] = e.Op
		checkGatewayLater(listPath, i, e, p)
	}
	checkSorted(listPath, enabledSection, list.Enabled, p)
	checkSorted(listPath, laterSection, later, p)
	checkUnique(listPath, enabledSection, list.Enabled, p)
	checkUnique(listPath, laterSection, later, p)
	listed := map[string]bool{}
	for _, name := range list.Enabled {
		listed[name] = true
		if _, ok := byOp[name]; !ok {
			p.add(listPath, "%s: %s is not a client-to-server op of the channel gateway of %s", enabledSection, name, asyncPath)
		}
	}
	for _, name := range later {
		if listed[name] {
			p.add(listPath, "%s: %s is in enabled and in later", sectionGatewayOps, name)
		}
		listed[name] = true
		if _, ok := byOp[name]; !ok && name != "" {
			p.add(listPath, "%s: %s is not a client-to-server op of the channel gateway of %s", laterSection, name, asyncPath)
		}
	}
	for _, name := range slices.Sorted(maps.Keys(byOp)) {
		if !listed[name] {
			p.add(listPath, "%s: the gateway op %s is in neither enabled nor later", sectionGatewayOps, name)
		}
	}
}

// checkGatewayLater checks the fields of one later entry of gateway_ops, with the rules of the
// later entries of operations.
func checkGatewayLater(path string, i int, e gatewayLater, p *problemsError) {
	if e.Op == "" {
		p.add(path, "%s.later[%d]: missing op", sectionGatewayOps, i)
		return
	}
	checkLaterEntry(path, i, laterEntry{OperationID: e.Op, Milestone: e.Milestone, Ticket: e.Ticket}, p)
}

// readAsyncGatewayOps returns the client-to-server ops of the channel gateway of the AsyncAPI
// document at path: every operation with action receive on #/channels/gateway, with its message
// name, its x-authz-action and the schema name of its body d. A malformed operation is a problem.
func readAsyncGatewayOps(path string, p *problemsError) []gatewayOp {
	doc, _, err := readYAML(path)
	if err != nil {
		p.add(path, "%v", err)
		return nil
	}
	root := doc.Content[0]
	channelMessages := value(value(value(root, "channels"), "gateway"), "messages")
	components := value(value(root, "components"), "messages")
	operations := value(root, "operations")
	var out []gatewayOp
	for _, id := range keys(operations) {
		op := value(operations, id)
		if scalar(value(op, "action")) != "receive" || scalar(value(value(op, "channel"), "$ref")) != gatewayChannelRef {
			continue
		}
		msgs := items(value(op, "messages"))
		if len(msgs) != 1 {
			p.add(path, "operations.%s: a gateway op needs exactly one message", id)
			continue
		}
		key := strings.TrimPrefix(scalar(value(msgs[0], "$ref")), gatewayChannelRef+"/messages/")
		ref := strings.TrimPrefix(scalar(value(value(channelMessages, key), "$ref")), "#/components/messages/")
		msg := value(components, ref)
		g := gatewayOp{
			Op:     scalar(value(msg, "name")),
			Action: scalar(value(op, "x-authz-action")),
			Schema: frameSchema(value(value(value(msg, "payload"), "properties"), "d")),
		}
		if g.Op == "" || g.Action == "" || g.Schema == "" {
			p.add(path, "operations.%s: the message needs a name, the operation an x-authz-action and the body d a reference to a schema of openapi.yaml", id)
			continue
		}
		out = append(out, g)
	}
	if len(out) == 0 {
		p.add(path, "the channel gateway has no client-to-server op")
		return nil
	}
	return out
}

// frameSchema returns the schema name of the body node d: "X" for a reference
// "./openapi.yaml#/components/schemas/X", "" otherwise.
func frameSchema(d *yaml.Node) string {
	_, name, ok := strings.Cut(scalar(value(d, "$ref")), schemaRefPrefix)
	if !ok {
		return ""
	}
	return name
}

// checkFrameSchemasClosed reports every object schema reachable from the named body schemas of
// client frames that accepts unknown members, with the rule of checkRequestSchemasClosed (W7,
// SEC-IN-1): the gateway closes with 4002 on an unknown member, so the contract must say so.
func checkFrameSchemasClosed(specPath string, doc *yaml.Node, names []string, p *problemsError) {
	root := doc.Content[0]
	w := &closureWalk{schemas: map[string]*yaml.Node{}, visited: map[visit]bool{}}
	all := value(value(root, "components"), "schemas")
	for _, name := range keys(all) {
		w.schemas[name] = value(all, name)
	}
	for _, name := range names {
		target := w.schemas[name]
		if target == nil {
			p.add(specPath, "the gateway frame body %s is not a schema of components.schemas", name)
			continue
		}
		w.schemaNamed(target, name, false)
	}
	for _, o := range w.open {
		p.add(specPath, "%s (line %d) is reachable from a gateway client frame but accepts unknown members; add additionalProperties: false (W7, SEC-IN-1)", o.where, o.line)
	}
}
