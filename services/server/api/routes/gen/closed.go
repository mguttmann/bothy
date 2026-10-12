// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"
)

// freeFormSchemas are the named schemas that stay open on purpose: JsonObject is free form by
// design (its content is described by the member that uses it; W7, docs/spec/06-api.md row
// "Unbekanntes").
var freeFormSchemas = []string{"JsonObject"}

// yamlFalse is the YAML scalar false of additionalProperties and unevaluatedProperties.
const yamlFalse = "false"

// schemaRefPrefix is the prefix of a reference to a named schema.
const schemaRefPrefix = "#/components/schemas/"

// openSchema is one object schema in the closure of a request body that accepts unknown members.
type openSchema struct {
	where string // "schema X" or "<operationId> request body <pointer>"
	line  int    // line of the schema node in the spec
	col   int    // column of the schema node
	allOf bool   // composed with allOf: close it with unevaluatedProperties
}

// visit is one node reached either directly or as an allOf member; a schema reached both ways is
// checked once for each way.
type visit struct {
	node   *yaml.Node
	member bool
}

// closureWalk walks the schemas reachable from the request bodies of the operations under paths.
type closureWalk struct {
	schemas map[string]*yaml.Node
	visited map[visit]bool
	open    []openSchema
}

// checkRequestSchemasClosed reports every object schema reachable from a request body of an
// operation under paths that does not refuse unknown members (W7, SEC-IN-1, R-API-04): it must
// have additionalProperties: false, or unevaluatedProperties: false where it is composed with
// allOf. A map (additionalProperties with a schema) is closed in this sense, and the schemas in
// freeFormSchemas are exempt. Webhooks are not walked: the server sends them, and responses and
// events stay additive.
func checkRequestSchemasClosed(specPath string, doc *yaml.Node, p *problemsError) {
	for _, o := range findOpenRequestSchemas(doc) {
		p.add(specPath, "%s (line %d) is reachable from a request body but accepts unknown members; add additionalProperties: false, or unevaluatedProperties: false on an allOf composition (W7, SEC-IN-1)", o.where, o.line)
	}
}

// findOpenRequestSchemas returns the open schemas of the request closure, each once, in walk order.
func findOpenRequestSchemas(doc *yaml.Node) []openSchema {
	root := doc.Content[0]
	w := &closureWalk{schemas: map[string]*yaml.Node{}, visited: map[visit]bool{}}
	all := value(value(root, "components"), "schemas")
	for _, name := range keys(all) {
		w.schemas[name] = value(all, name)
	}
	bodies := value(value(root, "components"), "requestBodies")
	paths := value(root, "paths")
	for _, path := range keys(paths) {
		item := value(paths, path)
		for _, method := range keys(item) {
			op := value(item, method)
			body := value(op, "requestBody")
			if ref := scalar(value(body, "$ref")); strings.HasPrefix(ref, "#/components/requestBodies/") {
				body = value(bodies, strings.TrimPrefix(ref, "#/components/requestBodies/"))
			}
			content := value(body, "content")
			for _, media := range keys(content) {
				w.schema(value(value(content, media), "schema"), scalar(value(op, "operationId"))+" request body "+media, false)
			}
		}
	}
	return w.open
}

// schema walks one schema node; where names it in a problem. composedMember is true for a
// member of allOf, which the composing schema closes with unevaluatedProperties.
func (w *closureWalk) schema(n *yaml.Node, where string, composedMember bool) {
	if n == nil || n.Kind != yaml.MappingNode || w.visited[visit{n, composedMember}] {
		return
	}
	if ref := scalar(value(n, "$ref")); ref != "" {
		name := strings.TrimPrefix(ref, schemaRefPrefix)
		if target := w.schemas[name]; target != nil && !slices.Contains(freeFormSchemas, name) {
			w.schemaNamed(target, name, composedMember)
		}
		return
	}
	w.visited[visit{n, composedMember}] = true
	w.check(n, where, composedMember)
	w.children(n, where)
}

// schemaNamed walks a named schema; a schema used both directly and as an allOf member is
// checked as used directly the first time it is reached directly.
func (w *closureWalk) schemaNamed(n *yaml.Node, name string, composedMember bool) {
	if w.visited[visit{n, composedMember}] {
		return
	}
	w.visited[visit{n, composedMember}] = true
	w.check(n, "schema "+name, composedMember)
	w.children(n, "schema "+name)
}

// check records n when it is an object schema that accepts unknown members.
func (w *closureWalk) check(n *yaml.Node, where string, composedMember bool) {
	if composedMember || !isObjectSchema(n) {
		return
	}
	if value(n, "allOf") != nil {
		if scalar(value(n, "unevaluatedProperties")) != yamlFalse {
			w.open = append(w.open, openSchema{where: where, line: n.Line, col: n.Column, allOf: true})
		}
		return
	}
	ap := value(n, "additionalProperties")
	if ap != nil && ap.Kind == yaml.MappingNode {
		return // a map with typed values
	}
	if scalar(ap) != yamlFalse && scalar(value(n, "unevaluatedProperties")) != yamlFalse {
		w.open = append(w.open, openSchema{where: where, line: n.Line, col: n.Column})
	}
}

// isObjectSchema reports whether n describes an object: type object, properties or allOf.
func isObjectSchema(n *yaml.Node) bool {
	return scalar(value(n, "type")) == "object" || value(n, "properties") != nil || value(n, "allOf") != nil
}

// children walks the subschemas of n.
func (w *closureWalk) children(n *yaml.Node, where string) {
	props := value(n, "properties")
	for _, name := range keys(props) {
		w.schema(value(props, name), fmt.Sprintf("%s/properties/%s", where, name), false)
	}
	for _, key := range []string{"items", "additionalProperties", "not"} {
		w.schema(value(n, key), where+"/"+key, false)
	}
	for _, key := range []string{"oneOf", "anyOf", "prefixItems"} {
		for i, c := range items(value(n, key)) {
			w.schema(c, fmt.Sprintf("%s/%s/%d", where, key, i), false)
		}
	}
	for i, c := range items(value(n, "allOf")) {
		w.schema(c, fmt.Sprintf("%s/allOf/%d", where, i), true)
	}
}
