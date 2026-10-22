// SPDX-License-Identifier: Apache-2.0

package main

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
)

func TestOperationMetadata_FromFixtureSpec(t *testing.T) {
	spec, catalog, list := fixturePaths(t, "")
	in, err := loadInputs(spec, catalog, list)
	if err != nil {
		t.Fatal(err)
	}
	want := []opMeta{
		{ID: "createThing", Method: "POST", Path: "/api/v1/things", Pattern: "POST /api/v1/things",
			Action: "thing.create", RateLimitClass: "rl.write", IdempotencyKey: true,
			IfMatch: ifMatchNone, RequestBody: bodyJSON, BodyRequired: true},
		{ID: "getThing", Method: "GET", Path: "/api/v1/things/{thing_id}", Pattern: "GET /api/v1/things/{thing_id}",
			Action: "thing.read", RateLimitClass: "rl.public_auth", Public: true,
			IfMatch: ifMatchNone, RequestBody: bodyNone},
		{ID: "updateThing", Method: "PATCH", Path: "/api/v1/things/{thing_id}", Pattern: "PATCH /api/v1/things/{thing_id}",
			Action: "thing.update", RateLimitClass: "rl.write",
			IfMatch: ifMatchRequired, RequestBody: bodyJSON, BodyRequired: true, ETagStatuses: []int{200}},
	}
	if !reflect.DeepEqual(in.ops, want) {
		t.Errorf("metadata =\n%+v\nwant\n%+v", in.ops, want)
	}
}

// TestPattern_FollowsOapiCodegen pins the pattern rule of oapi-codegen v2.8.0
// (SwaggerUriToStdHttpUri and the std-http handler template): method, one space, base URL and
// path, every parameter reduced to its sanitised name, and {$} after a trailing slash.
func TestPattern_FollowsOapiCodegen(t *testing.T) {
	cases := []struct{ method, path, want string }{
		{"get", "/threads/{thread_id}", "GET /api/v1/threads/{thread_id}"},
		{"post", "/a/{a-b}/c", "POST /api/v1/a/{a_b}/c"},
		{"get", "/files/{.name*}", "GET /api/v1/files/{name}"},
		{"put", "/x/{;p}", "PUT /api/v1/x/{p}"},
		{"delete", "/dirs/", "DELETE /api/v1/dirs/{$}"},
		{"patch", "/{1st}", "PATCH /api/v1/{_st}"},
	}
	for _, c := range cases {
		if got := pattern(c.method, "/api/v1", c.path); got != c.want {
			t.Errorf("pattern(%q, %q) = %q, want %q", c.method, c.path, got, c.want)
		}
	}
}

// specVariant loads the fixture with its spec text changed by replacing old with new once.
func specVariant(t *testing.T, old, new string) []string {
	t.Helper()
	spec, catalog, list := fixturePaths(t, "")
	data := readText(t, spec)
	if !strings.Contains(data, old) {
		t.Fatalf("fixture spec does not contain %q", old)
	}
	writeFile(t, spec, strings.Replace(data, old, new, 1))
	_, err := loadInputs(spec, catalog, list)
	if err == nil {
		return nil
	}
	return problemsOf(t, err)
}

func TestOperationMetadata_ResolvesResponseRefs(t *testing.T) {
	spec, catalog, list := fixturePaths(t, "")
	data := readText(t, spec)
	data = strings.Replace(data, `        '200':
          description: The changed thing.
          headers:
            ETag:
              $ref: '#/components/headers/ETag'
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/Thing'
`, `        '200':
          $ref: '#/components/responses/Changed'
`, 1)
	data = strings.Replace(data, "  responses:\n    Error:", `  responses:
    Changed:
      description: The changed thing.
      headers:
        etag:
          $ref: '#/components/headers/ETag'
      content:
        application/json:
          schema:
            $ref: '#/components/schemas/Thing'
    Error:`, 1)
	writeFile(t, spec, data)
	in, err := loadInputs(spec, catalog, list)
	if err != nil {
		t.Fatal(err)
	}
	for _, op := range in.ops {
		if op.ID == "updateThing" {
			if !reflect.DeepEqual(op.ETagStatuses, []int{200}) {
				t.Errorf("ETagStatuses = %v, want [200] through the response reference", op.ETagStatuses)
			}
			return
		}
	}
	t.Fatal("updateThing not found")
}

// TestLoadInputs_FailsClosedOnUnsupportedForms covers forms the route table could not describe
// correctly; the generator refuses them instead of emitting wrong metadata.
func TestLoadInputs_FailsClosedOnUnsupportedForms(t *testing.T) {
	cases := []struct {
		name, old, new string
		wants          []string
	}{
		{"both If-Match parameters", "      - $ref: '#/components/parameters/IfMatchRequired'\n",
			"      - $ref: '#/components/parameters/IfMatchRequired'\n      - $ref: '#/components/parameters/IfMatchOptional'\n",
			[]string{"updateThing", "If-Match"}},
		{"two media types", "              $ref: '#/components/schemas/ThingCreate'\n",
			"              $ref: '#/components/schemas/ThingCreate'\n          multipart/form-data:\n            schema:\n              $ref: '#/components/schemas/ThingCreate'\n",
			[]string{"createThing", "one media type"}},
		{"one CSP report media type alone", "          application/json:\n            schema:\n              $ref: '#/components/schemas/ThingCreate'\n",
			"          application/csp-report:\n            schema:\n              $ref: '#/components/schemas/ThingCreate'\n",
			[]string{"createThing", "application/csp-report"}},
		{"unknown media type", "          application/json:\n            schema:\n              $ref: '#/components/schemas/ThingCreate'\n",
			"          text/plain:\n            schema:\n              type: string\n",
			[]string{"createThing", "text/plain"}},
		{"optional security", "      security: []\n", "      security:\n      - {}\n",
			[]string{"getThing", "security"}},
		{"servers", "servers:\n- url: /api/v1\n", "servers:\n- url: /v1\n",
			[]string{"servers", "/api/v1"}},
		{"operation servers", "      operationId: getThing\n", "      operationId: getThing\n      servers:\n      - url: /other\n",
			[]string{"getThing", "servers"}},
		{"path parameters", "  /things:\n", "  /things:\n    parameters:\n    - $ref: '#/components/parameters/IdempotencyKey'\n",
			[]string{"/things", "parameters"}},
		{"connect method", "  /things:\n", "  /things:\n    connect:\n      operationId: connectThing\n      responses:\n        '204':\n          description: Done.\n",
			[]string{"/things", `"connect"`}},
		{"upper-case method key", "  /things:\n", "  /things:\n    GET:\n      operationId: listThings\n      responses:\n        '200':\n          description: Done.\n",
			[]string{"/things", `"GET"`}},
		{"path item reference", "  /things/{thing_id}:\n    get:\n", "  /other:\n    $ref: '#/components/pathItems/Other'\n  /things/{thing_id}:\n    get:\n",
			[]string{"/other", `"$ref"`}},
		{"duplicate operation id", "      operationId: updateThing\n", "      operationId: getThing\n",
			[]string{"getThing", "more than one operation"}},
		{"missing operation id", "      operationId: updateThing\n", "",
			[]string{"PATCH /things/{thing_id}", "operationId"}},
		{"ETag on a status range", "        '201':\n          description: Created.\n",
			"        '2XX':\n          description: Created.\n          headers:\n            ETag:\n              $ref: '#/components/headers/ETag'\n",
			[]string{"createThing", "2XX"}},
		{"health probe", "      operationId: getThing\n", "      operationId: getReadiness\n",
			[]string{"getReadiness", "health probe"}},
		{"unresolvable reference", "      - $ref: '#/components/parameters/ThingIdPath'\n      - $ref: '#/components/parameters/IfMatchRequired'\n",
			"      - $ref: '#/components/parameters/Missing'\n      - $ref: '#/components/parameters/IfMatchRequired'\n",
			[]string{"updateThing", "#/components/parameters/Missing"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			problems := specVariant(t, c.old, c.new)
			joined := strings.Join(problems, "\n")
			if len(problems) == 0 {
				t.Fatal("no problem reported")
			}
			for _, w := range c.wants {
				if !strings.Contains(joined, w) {
					t.Errorf("problems do not mention %q:\n%s", w, joined)
				}
			}
		})
	}
}

// specWith copies the fixture, replaces each old text of the old/new pairs once in its spec and
// returns the paths of the variant.
func specWith(t *testing.T, pairs ...string) (spec, catalog, list string) {
	t.Helper()
	spec, catalog, list = fixturePaths(t, "")
	data := readText(t, spec)
	for i := 0; i+1 < len(pairs); i += 2 {
		if !strings.Contains(data, pairs[i]) {
			t.Fatalf("fixture spec does not contain %q", pairs[i])
		}
		data = strings.Replace(data, pairs[i], pairs[i+1], 1)
	}
	writeFile(t, spec, data)
	return spec, catalog, list
}

// opByID returns the metadata of one operation of in.
func opByID(t *testing.T, in *inputs, id string) opMeta {
	t.Helper()
	for _, op := range in.ops {
		if op.ID == id {
			return op
		}
	}
	t.Fatalf("operation %s not found", id)
	return opMeta{}
}

// TestOperationMetadata_FollowsWhatOapiCodegenSees: the route table describes every form that the
// loader of oapi-codegen (kin-openapi) resolves, so the table and the generated server never
// disagree about a header, a body or a response.
func TestOperationMetadata_FollowsWhatOapiCodegenSees(t *testing.T) {
	cases := []struct {
		name  string
		pairs []string
		id    string
		check func(opMeta) bool
	}{
		{"chained Idempotency-Key component", []string{
			"      - $ref: '#/components/parameters/IdempotencyKey'\n", "      - $ref: '#/components/parameters/IdemAlias'\n",
			"  parameters:\n    ThingIdPath:", "  parameters:\n    IdemAlias:\n      $ref: '#/components/parameters/IdempotencyKey'\n    ThingIdPath:",
		}, "createThing", func(o opMeta) bool { return o.IdempotencyKey }},
		{"chained If-Match component", []string{
			"      - $ref: '#/components/parameters/IfMatchRequired'\n", "      - $ref: '#/components/parameters/IfAlias'\n",
			"  parameters:\n    ThingIdPath:", "  parameters:\n    IfAlias:\n      $ref: '#/components/parameters/IfMatchRequired'\n    ThingIdPath:",
		}, "updateThing", func(o opMeta) bool { return o.IfMatch == ifMatchRequired }},
		{"chained response with ETag", []string{
			`        '200':
          description: The changed thing.
          headers:
            ETag:
              $ref: '#/components/headers/ETag'
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/Thing'
`, "        '200':\n          $ref: '#/components/responses/UpdatedAlias'\n",
			"  responses:\n    Error:", `  responses:
    UpdatedAlias:
      $ref: '#/components/responses/Updated'
    Updated:
      description: The changed thing.
      headers:
        ETag:
          $ref: '#/components/headers/ETag'
      content:
        application/json:
          schema:
            $ref: '#/components/schemas/Thing'
    Error:`,
		}, "updateThing", func(o opMeta) bool { return reflect.DeepEqual(o.ETagStatuses, []int{200}) }},
		{"required: True", []string{
			"      requestBody:\n        required: true\n        content:\n          application/json:\n            schema:\n              $ref: '#/components/schemas/ThingCreate'",
			"      requestBody:\n        required: True\n        content:\n          application/json:\n            schema:\n              $ref: '#/components/schemas/ThingCreate'",
		}, "createThing", func(o opMeta) bool { return o.BodyRequired }},
		{"inline Idempotency-Key header", []string{
			"      - $ref: '#/components/parameters/IdempotencyKey'\n",
			"      - name: idempotency-key\n        in: header\n        schema:\n          type: string\n",
		}, "createThing", func(o opMeta) bool { return o.IdempotencyKey }},
		{"inline optional If-Match header", []string{
			"      - $ref: '#/components/parameters/IfMatchRequired'\n",
			"      - name: If-Match\n        in: header\n        schema:\n          type: string\n",
		}, "updateThing", func(o opMeta) bool { return o.IfMatch == ifMatchOptional }},
		{"JSON success is no download", nil, "getThing", func(o opMeta) bool { return !o.Download }},
		{"ndjson success is a download", []string{
			"          description: The thing.\n          content:\n            application/json:",
			"          description: The thing.\n          content:\n            application/x-ndjson:",
		}, "getThing", func(o opMeta) bool { return o.Download }},
		{"octet-stream success is a download", []string{
			"          description: The thing.\n          content:\n            application/json:",
			"          description: The thing.\n          content:\n            application/octet-stream:",
		}, "getThing", func(o opMeta) bool { return o.Download }},
		{"the pair of CSP report media types is the report kind", []string{
			"      - $ref: '#/components/parameters/IdempotencyKey'\n", "",
			"          application/json:\n            schema:\n              $ref: '#/components/schemas/ThingCreate'\n",
			"          application/reports+json:\n            schema:\n              $ref: '#/components/schemas/ThingCreate'\n          application/csp-report:\n            schema:\n              $ref: '#/components/schemas/ThingCreate'\n",
		}, "createThing", func(o opMeta) bool { return o.RequestBody == bodyReport }},
		{"a +json success is no download", []string{
			"          description: The thing.\n          content:\n            application/json:",
			"          description: The thing.\n          content:\n            application/jwk-set+json:",
		}, "getThing", func(o opMeta) bool { return !o.Download }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			spec, catalog, list := specWith(t, c.pairs...)
			in, err := loadInputs(spec, catalog, list)
			if err != nil {
				t.Fatalf("loadInputs: %v", err)
			}
			if op := opByID(t, in, c.id); !c.check(op) {
				t.Errorf("%s metadata = %+v", c.id, op)
			}
		})
	}
}

// TestLoadInputs_RejectsSecurityOverrideOfEnabledOperation: the route table records only
// "public" or "inherits the root security"; an enabled operation with its own requirements
// would be served as if it inherited them, so the generator refuses it. A later operation may
// have one, as getPushPreview and the downloads of the real spec do.
func TestLoadInputs_RejectsSecurityOverrideOfEnabledOperation(t *testing.T) {
	pairs := []string{
		"      operationId: updateThing\n", "      operationId: updateThing\n      security:\n      - dpopAuth: []\n",
		"  securitySchemes:\n", "  securitySchemes:\n    dpopAuth:\n      type: http\n      scheme: DPoP\n",
	}
	spec, catalog, list := specWith(t, pairs...)
	_, err := loadInputs(spec, catalog, list)
	joined := strings.Join(problemsOf(t, err), "\n")
	for _, w := range []string{"updateThing", "security"} {
		if !strings.Contains(joined, w) {
			t.Errorf("problems do not mention %q:\n%s", w, joined)
		}
	}

	spec, catalog, list = specWith(t, pairs...)
	writeFile(t, list, "version: 1\nenabled: [createThing, getThing]\nlater:\n  - operation_id: updateThing\n    milestone: M3\n")
	if _, err := loadInputs(spec, catalog, list); err != nil {
		t.Errorf("a later operation with its own security is fine until it is enabled: %v", err)
	}

	spec, catalog, list = specWith(t, "      operationId: updateThing\n", "      operationId: updateThing\n      security:\n      - bearerAuth: []\n")
	if _, err := loadInputs(spec, catalog, list); err != nil {
		t.Errorf("security equal to the root security is no override: %v", err)
	}
}

// TestLoadInputs_AcceptsFileTicketBesideRootSecurity: downloadFile and downloadThumbnail accept
// the root security or a fetch ticket in the query (security scheme fileTicket, 12 9.2, ADR 0048).
// The route table records them as root security: the chain authenticates the header as for every
// route and an anonymous request reaches the handler, which checks the ticket (docs/decisions.d/M3-05.md
// D-M3-05-14). Any other extra requirement stays an override.
func TestLoadInputs_AcceptsFileTicketBesideRootSecurity(t *testing.T) {
	schemes := "  securitySchemes:\n    fileTicket:\n      type: apiKey\n      in: query\n      name: ticket\n" +
		"    otherKey:\n      type: apiKey\n      in: query\n      name: key\n"
	spec, catalog, list := specWith(t, "      operationId: updateThing\n", "      operationId: updateThing\n      security:\n      - bearerAuth: []\n      - fileTicket: []\n",
		"  securitySchemes:\n", schemes)
	in, err := loadInputs(spec, catalog, list)
	if err != nil {
		t.Fatalf("root security plus fileTicket is no override: %v", err)
	}
	if op := opByID(t, in, "updateThing"); op.Public {
		t.Errorf("updateThing with a fetch ticket is public: %+v", op)
	}

	spec, catalog, list = specWith(t, "      operationId: updateThing\n", "      operationId: updateThing\n      security:\n      - bearerAuth: []\n      - otherKey: []\n",
		"  securitySchemes:\n", schemes)
	_, err = loadInputs(spec, catalog, list)
	if joined := strings.Join(problemsOf(t, err), "\n"); !strings.Contains(joined, "updateThing") {
		t.Errorf("another extra requirement is an override; problems:\n%s", joined)
	}
}

// TestLoadInputs_RejectsClassNone: rate-limit class none belongs to system actions and internal
// delivery checks (05 "Rate-limit classes"), which have no REST route (06 "Authorization in the
// API", item 7).
func TestLoadInputs_RejectsClassNone(t *testing.T) {
	spec, catalog, list := specWith(t, "      x-authz-action: thing.update\n      x-rate-limit-class: rl.write\n",
		"      x-authz-action: thing.deliver\n      x-rate-limit-class: none\n")
	data := readText(t, catalog)
	if !strings.Contains(data, "events: []\n") {
		t.Fatal("fixture catalogue has no events line")
	}
	writeFile(t, catalog, strings.Replace(data, "events: []\n", `- id: thing.deliver
  section: things
  description: deliver a thing (internal check)
  resource: thing
  right: read
  owner: false
  step_up: false
  audit: never
  rights_change: false
  rl_class: none
  effect: read
events: []
`, 1))
	_, err := loadInputs(spec, catalog, list)
	problems := problemsOf(t, err)
	if len(problems) != 1 {
		t.Fatalf("got %d problems, want exactly the class problem:\n%s", len(problems), strings.Join(problems, "\n"))
	}
	for _, w := range []string{"updateThing", "x-rate-limit-class none"} {
		if !strings.Contains(problems[0], w) {
			t.Errorf("problem %q does not mention %q", problems[0], w)
		}
	}
}

// TestLoadInputs_SelfReferencingAnchorFailsPromptly: a cyclic YAML alias ends in a load problem
// within a short time instead of an endless walk.
func TestLoadInputs_SelfReferencingAnchorFailsPromptly(t *testing.T) {
	spec, catalog, list := specWith(t, "security:\n- bearerAuth: []\n", "security:\n- bearerAuth: []\n\nx-loop: &loop\n  self: *loop\n")
	done := make(chan error, 1)
	go func() {
		_, err := loadInputs(spec, catalog, list)
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("a self-referencing anchor was accepted")
		}
	case <-time.After(testkit.Patience):
		t.Fatalf("loadInputs did not return within %s on a self-referencing anchor", testkit.Patience)
	}
}

// TestCheckFiltered_MatchesTheTable: the copy given to oapi-codegen holds exactly the enabled
// operations of the table; any other operation, however it got there, is a problem.
func TestCheckFiltered_MatchesTheTable(t *testing.T) {
	data := []byte(readText(t, fixtureDir+"/openapi.yaml"))
	want := []opMeta{{ID: "getThing", Method: "GET", Path: "/api/v1/things/{thing_id}"}}
	keepGet := func(id string) bool { return id == "getThing" }
	filtered, err := filterSpec(data, keepGet)
	if err != nil {
		t.Fatal(err)
	}
	if err := checkFiltered(filtered, want); err != nil {
		t.Errorf("checkFiltered on a correct copy: %v", err)
	}
	extra := strings.Replace(string(filtered), "paths:\n", "paths:\n  /extra:\n    get:\n      operationId: extraThing\n      responses:\n        '204':\n          description: Done.\n", 1)
	if err := checkFiltered([]byte(extra), want); err == nil || !strings.Contains(err.Error(), "extraThing") {
		t.Errorf("checkFiltered = %v, want an error naming extraThing", err)
	}
	if err := checkFiltered(filtered, append(want, opMeta{ID: "createThing", Method: "POST", Path: "/api/v1/things"})); err == nil || !strings.Contains(err.Error(), "createThing") {
		t.Errorf("checkFiltered = %v, want an error naming the missing createThing", err)
	}
}

// TestLoadInputs_RejectsMergeKeys: the filter edits the node tree, in which merged content is
// invisible, so a merge key could hide an operation or a parameter from it.
func TestLoadInputs_RejectsMergeKeys(t *testing.T) {
	spec, catalog, list := specWith(t,
		"security:\n- bearerAuth: []\n", "security:\n- bearerAuth: []\n\nx-shared: &idem\n  parameters:\n  - $ref: '#/components/parameters/IdempotencyKey'\n",
		"      parameters:\n      - $ref: '#/components/parameters/IdempotencyKey'\n", "      <<: *idem\n")
	_, err := loadInputs(spec, catalog, list)
	joined := strings.Join(problemsOf(t, err), "\n")
	for _, w := range []string{"merge keys", ".paths./things.post"} {
		if !strings.Contains(joined, w) {
			t.Errorf("problems do not mention %q:\n%s", w, joined)
		}
	}
}
