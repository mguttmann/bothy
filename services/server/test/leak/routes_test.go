// SPDX-License-Identifier: Apache-2.0

package leak

import (
	"slices"
	"strings"
	"testing"
)

// TestSelectRoutes: only enabled GET operations with a uuid path parameter are selected, with
// parameters resolved through components/parameters and components/schemas, in template order.
func TestSelectRoutes(t *testing.T) {
	list := []byte("version: 1\nenabled: [getThing, listParts, getByCode, createThing, getMe]\nlater: []\n")
	contract := []byte(`
paths:
  /things/{thing_id}:
    parameters: [{$ref: "#/components/parameters/ThingId"}]
    get: {operationId: getThing}
    post: {operationId: createThing}
  /things/{thing_id}/parts/{part_id}:
    get:
      operationId: listParts
      parameters:
        - {$ref: "#/components/parameters/ThingId"}
        - {name: part_id, in: path, schema: {type: string, format: uuid}}
  /codes/{code}:
    get:
      operationId: getByCode
      parameters: [{name: code, in: path, schema: {type: string}}]
  /me:
    get: {operationId: getMe}
  /hidden/{thing_id}:
    get: {operationId: getHidden, parameters: [{$ref: "#/components/parameters/ThingId"}]}
components:
  parameters:
    ThingId: {name: thing_id, in: path, schema: {$ref: "#/components/schemas/Uuid"}}
  schemas:
    Uuid: {type: string, format: uuid}
`)
	got, err := selectRoutes(list, contract)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].OperationID != "getThing" || got[1].OperationID != "listParts" ||
		!slices.Equal(got[1].Params, []string{"thing_id", "part_id"}) {
		t.Fatalf("routes %+v", got)
	}
	if p := got[1].Build(map[string]string{"thing_id": "a", "part_id": "b"}); p != "/things/a/parts/b" {
		t.Fatalf("path %s", p)
	}
	for _, bad := range [][2]string{
		{"version: 2\n", "paths: {}\n"},
		{"version: 1\nenabled: [x]\n", "paths: {/x/{a}: {get: {operationId: x, parameters: [{$ref: '#/components/parameters/Nope'}]}}}\n"},
		{"version: 1\nenabled: [x]\n", "paths: {/x/{a}: {get: {operationId: x, parameters: [{$ref: 'other.yaml#/A'}]}}}\n"},
		{"[", "paths: {}\n"},
		{"version: 1\n", "["},
	} {
		if _, err := selectRoutes([]byte(bad[0]), []byte(bad[1])); err == nil {
			t.Errorf("no error for %q %q", bad[0], bad[1])
		}
	}
}

// TestSelectRoutesOfTheRepository: the real selection contains the routes of the brief.
func TestSelectRoutesOfTheRepository(t *testing.T) {
	got, err := SelectRoutes()
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, len(got))
	for i, r := range got {
		names[i] = r.OperationID
		if !strings.HasPrefix(r.Path, "/") || len(r.Params) == 0 {
			t.Errorf("route %+v", r)
		}
	}
	for _, want := range []string{"getSpace", "getChannel", "getThread", "getMessage", "getAgent", "getUser", "listTokens", "explainChannelAccess"} {
		if !slices.Contains(names, want) {
			t.Errorf("%s not selected: %v", want, names)
		}
	}
	for _, not := range []string{"getSnapshot", "listEvents", "getMe", "getProblemType"} {
		if slices.Contains(names, not) {
			t.Errorf("%s selected", not)
		}
	}
}
