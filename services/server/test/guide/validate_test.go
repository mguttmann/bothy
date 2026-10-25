// SPDX-License-Identifier: Apache-2.0

package guide

import (
	"strings"
	"testing"
)

// testInputs are the inputs of the synthetic pages: one operation, one example file, a version.
func testInputs() Inputs {
	return Inputs{
		Operations: map[string]Operation{
			"createAgent":   {Method: "POST", Path: "/agents"},
			"getAgent":      {Method: "GET", Path: "/agents/{agent_id}"},
			"createToken":   {Method: "POST", Path: "/principals/{principal_id}/tokens"},
			"getDiscovery":  {Method: "GET", Path: "/instance/discovery"},
			"listThreadRun": {Method: "GET", Path: "/threads/{thread_id}/runs"},
		},
		Examples:   map[string][]byte{"sdk/python/examples/guide/agent.py": []byte("print(1)\nprint(2)\n")},
		SDKVersion: "1.2.3",
	}
}

// placeholderTable is the table of the runner-bound names.
const placeholderTable = "| `{server_url}` | a |\n| `{adapter_dir}` | b |\n| `{alex_id}` | c |\n| `{sam_id}` | d |\n| `{new_uuid}` | e |\n| `{expires_at}` | f |\n"

// validPage is a synthetic page that passes Validate with testInputs.
func validPage(extra map[int]string) string {
	steps := map[int]string{
		1: fenceOf("http", "Owner: createAgent", "POST /api/v1/agents HTTP/1.1\nContent-Type: application/json\n\n{\"handle\": \"lotse\"}") +
			fenceOf("json", "Response 201", `{"id": "{agent_id}"}`),
		2:  fenceOf("http", "Owner: getAgent", "GET /api/v1/agents/{agent_id} HTTP/1.1"),
		3:  fenceOf("python", "sdk/python/examples/guide/agent.py", "print(1)\nprint(2)"),
		10: fenceOf("toml", "pyproject.toml (excerpt)", "[project]\ndependencies = [\n  \"bothy-sdk==1.2.3\",\n]"),
	}
	for k, v := range extra {
		steps[k] = v
	}
	page := synthetic(testTitles, steps)
	return strings.Replace(page, "| `{server_url}` | address |\n", placeholderTable, 1)
}

// validateText parses and validates a page.
func validateText(t *testing.T, page string, in Inputs) error {
	t.Helper()
	p, err := Parse([]byte(page), testTitles)
	if err != nil {
		return err
	}
	return Validate(p, in)
}

// expectFailure fails t unless err names every part of want.
func expectFailure(t *testing.T, err error, want ...string) {
	t.Helper()
	if err == nil {
		t.Fatalf("no error, want one with %q", want)
	}
	for _, w := range want {
		if !strings.Contains(err.Error(), w) {
			t.Fatalf("err = %v, want one with %q", err, w)
		}
	}
}

func TestValidate_ValidPagePasses(t *testing.T) {
	if err := validateText(t, validPage(nil), testInputs()); err != nil {
		t.Fatal(err)
	}
}

func TestValidate_FileBlocksEqualExamples(t *testing.T) {
	changed := validPage(map[int]string{3: fenceOf("python", "sdk/python/examples/guide/agent.py", "print(1)\nprint(3)")})
	expectFailure(t, validateText(t, changed, testInputs()), "Step 3", "sdk/python/examples/guide/agent.py", "line 2")

	in := testInputs()
	in.Examples["sdk/python/examples/guide/run.py"] = []byte("x\n")
	expectFailure(t, validateText(t, validPage(nil), in), "sdk/python/examples/guide/run.py", "not shown")

	twice := validPage(map[int]string{4: fenceOf("python", "sdk/python/examples/guide/agent.py", "print(1)\nprint(2)")})
	expectFailure(t, validateText(t, twice, testInputs()), "Step 4", "shown twice")

	missing := validPage(map[int]string{4: fenceOf("python", "sdk/python/examples/guide/gone.py", "x")})
	expectFailure(t, validateText(t, missing, testInputs()), "Step 4", "gone.py", "no such file")
}

func TestValidate_RequestLinesMatchOpenAPI(t *testing.T) {
	request := func(title, line string) string {
		return validPage(map[int]string{5: fenceOf("http", title, line)})
	}
	if err := validateText(t, request("Owner: createAgent", "POST /api/v1/agents HTTP/1.1"), testInputs()); err != nil {
		t.Fatalf("createAgent failed: %v", err)
	}
	cases := []struct{ name, title, line, want string }{
		{"unknown operation", "Owner: makeAgent", "POST /api/v1/agents HTTP/1.1", "unknown operationId"},
		{"other method", "Owner: createAgent", "PUT /api/v1/agents HTTP/1.1", "method"},
		{"other path", "Owner: createAgent", "POST /api/v1/agent HTTP/1.1", "path"},
		{"no version", "Owner: createAgent", "POST /api/v1/agents", "HTTP/1.1"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			expectFailure(t, validateText(t, request(c.title, c.line), testInputs()), "Step 5", c.want)
		})
	}
}

func TestValidate_Placeholders(t *testing.T) {
	use := func(path string) string {
		return validPage(map[int]string{5: fenceOf("http", "Owner: getAgent", "GET /api/v1/agents/"+path+" HTTP/1.1")})
	}
	if err := validateText(t, use("{agent_id}"), testInputs()); err != nil {
		t.Fatalf("a captured name failed: %v", err)
	}
	before := validPage(map[int]string{
		1: fenceOf("http", "Owner: getAgent", "GET /api/v1/agents/{agent_id} HTTP/1.1"),
		2: fenceOf("http", "Owner: createAgent", "POST /api/v1/agents HTTP/1.1") + fenceOf("json", "Response 201", `{"id": "{agent_id}"}`),
	})
	expectFailure(t, validateText(t, before, testInputs()), "Step 1", "{agent_id}", "before it is captured")

	twice := validPage(map[int]string{
		6: fenceOf("http", "Owner: createAgent", "POST /api/v1/agents HTTP/1.1") + fenceOf("json", "Response 201", `{"id": "{other}", "x": "{other}"}`),
	})
	expectFailure(t, validateText(t, twice, testInputs()), "Step 6", "{other}", "captured twice")

	expectFailure(t, validateText(t, use("{nowhere}"), testInputs()), "Step 5", "unknown placeholder {nowhere}")

	secret := validPage(map[int]string{5: fenceOf("bash", "Shell: show", "cat {secret:agent.token}")})
	expectFailure(t, validateText(t, secret, testInputs()), "Step 5", "only in a response block")

	lacks := strings.Replace(validPage(nil), "| `{sam_id}` | d |\n", "", 1)
	expectFailure(t, validateText(t, lacks, testInputs()), "Placeholders", "sam_id")
	adds := strings.Replace(validPage(nil), "| `{sam_id}` | d |\n", "| `{sam_id}` | d |\n| `{robin_id}` | g |\n", 1)
	expectFailure(t, validateText(t, adds, testInputs()), "Placeholders", "robin_id")
}

func TestValidate_PinMatchesSDKVersion(t *testing.T) {
	pin := func(body string) string {
		return validPage(map[int]string{10: fenceOf("toml", "pyproject.toml (excerpt)", body)})
	}
	if err := validateText(t, pin("[project]\nname = \"my-adapter\"\ndependencies = [\"bothy-sdk==1.2.3\", \"httpx>=0.28\"]"), testInputs()); err != nil {
		t.Fatalf("a valid pin failed: %v", err)
	}
	cases := []struct{ name, body, want string }{
		{"other version", "[project]\ndependencies = [\"bothy-sdk==1.2.4\"]", "bothy-sdk==1.2.3"},
		{"second entry", "[project]\ndependencies = [\"bothy-sdk==1.2.3\", \"bothy-sdk>=1\"]", "exactly one"},
		{"invalid toml", "[project\ndependencies = [\"bothy-sdk==1.2.3\"]", "TOML"},
		{"unclosed array", "[project]\ndependencies = [\"bothy-sdk==1.2.3\"", "TOML"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			expectFailure(t, validateText(t, pin(c.body), testInputs()), "Step 10", c.want)
		})
	}
}
