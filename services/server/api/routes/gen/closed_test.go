// SPDX-License-Identifier: Apache-2.0

package main

import (
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

// realSpec is docs/api/openapi.yaml, relative to this package.
const realSpec = "../../../../../docs/api/openapi.yaml"

// TestRealSpec_RequestSchemasClosed holds W7 on the contract: every object schema reachable from
// a request body refuses unknown members, except JsonObject.
func TestRealSpec_RequestSchemasClosed(t *testing.T) {
	t.Parallel()
	doc, _, err := readYAML(realSpec)
	if err != nil {
		t.Fatal(err)
	}
	for _, o := range findOpenRequestSchemas(doc) {
		t.Errorf("open request schema: %s (line %d)", o.where, o.line)
	}
}

// TestGenerate_RejectsOpenRequestSchema: the negative fixture opens ThingCreate, which the
// request body of createThing uses; the generator names it and writes nothing.
func TestGenerate_RejectsOpenRequestSchema(t *testing.T) {
	expectProblems(t, "open-request-schema", 1, "schema ThingCreate", "additionalProperties: false")
}

func TestClosedCheck_Forms(t *testing.T) {
	t.Parallel()
	spec := `openapi: 3.1.0
paths:
  /a:
    post:
      operationId: a
      requestBody:
        $ref: '#/components/requestBodies/A'
components:
  requestBodies:
    A:
      content:
        application/json:
          schema:
            type: object
            additionalProperties: false
            properties:
              free: {$ref: '#/components/schemas/JsonObject'}
              labels: {type: object, additionalProperties: {type: string}}
              composed: {$ref: '#/components/schemas/Composed'}
              open_composed: {allOf: [{$ref: '#/components/schemas/Base'}]}
              list: {type: array, items: {type: object, properties: {x: {type: string}}}}
              union: {oneOf: [{$ref: '#/components/schemas/Base'}]}
  schemas:
    JsonObject: {type: object, additionalProperties: true}
    Base: {type: object, properties: {id: {type: string}}}
    Composed:
      allOf: [{$ref: '#/components/schemas/Base'}]
      unevaluatedProperties: false
`
	doc := parseYAMLText(t, spec)
	var got []string
	for _, o := range findOpenRequestSchemas(doc) {
		got = append(got, o.where)
	}
	want := []string{"a request body application/json/properties/open_composed", "a request body application/json/properties/list/items", "schema Base"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("open schemas %q\nwant %q", got, want)
	}
}

// parseYAMLText parses YAML text into a document node.
func parseYAMLText(t *testing.T, text string) *yaml.Node {
	t.Helper()
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(text), &doc); err != nil {
		t.Fatal(err)
	}
	return &doc
}
