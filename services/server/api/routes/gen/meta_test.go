// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"slices"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

// realAsyncAPI is the gateway contract api/generate.go passes with -asyncapi.
const realAsyncAPI = "../../../../../docs/api/asyncapi.yaml"

func TestRealInputs_SpecVersionAndProblems(t *testing.T) {
	in, err := realInputs()
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Info struct {
			Version string `yaml:"version"`
		} `yaml:"info"`
	}
	if err := yaml.Unmarshal(in.specData, &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Info.Version == "" || in.specVersion != doc.Info.Version {
		t.Errorf("spec version %q, want info.version %q", in.specVersion, doc.Info.Version)
	}
	codes := map[string]problemType{}
	for _, p := range in.problems {
		codes[p.Code] = p
	}
	for _, code := range []string{"validation_failed", "version_conflict", "precondition_required", "setup_required", "not_found"} {
		p, ok := codes[code]
		if !ok {
			t.Errorf("problem %s missing", code)
			continue
		}
		if p.Status < 400 || p.Title == "" || p.Description == "" {
			t.Errorf("problem %s incomplete: %+v", code, p)
		}
	}
	if p := codes["version_conflict"]; p.Status != 412 || p.ClientAction == "" {
		t.Errorf("version_conflict = %+v", p)
	}
	if !slices.IsSortedFunc(in.problems, func(a, b problemType) int { return strings.Compare(a.Code, b.Code) }) {
		t.Error("problems are not sorted by code")
	}
}

func TestReadGatewayVersions_Real(t *testing.T) {
	got, err := readGatewayVersions(realAsyncAPI)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got, []int{1}) {
		t.Errorf("gateway versions %v, want [1]", got)
	}
}

func TestReadGatewayVersions_Invalid(t *testing.T) {
	dir := t.TempDir()
	for name, body := range map[string]string{
		"missing":  "asyncapi: 3.0.0\ninfo:\n  title: x\n",
		"empty":    "asyncapi: 3.0.0\ninfo:\n  x-protocol-versions: []\n",
		"string":   "asyncapi: 3.0.0\ninfo:\n  x-protocol-versions: [one]\n",
		"zero":     "asyncapi: 3.0.0\ninfo:\n  x-protocol-versions: [0]\n",
		"repeated": "asyncapi: 3.0.0\ninfo:\n  x-protocol-versions: [1, 1]\n",
	} {
		path := dir + "/" + name + ".yaml"
		writeFile(t, path, body)
		if _, err := readGatewayVersions(path); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	if _, err := readGatewayVersions(dir + "/absent.yaml"); err == nil {
		t.Error("missing file accepted")
	}
}

func TestLoadInputs_ProblemCodeRules(t *testing.T) {
	spec, catalog, list := fixturePaths(t, "")
	base := readText(t, spec)
	for name, c := range map[string]struct {
		replace, with, want string
	}{
		"version":        {"version: 0.0.1", "version: one", "info.version"},
		"no-description": {"        description: 'The request violates the schema.'\n", "", "description"},
		"bad-status":     {"        status: 400", "        status: 200", "status"},
		"bad-code":       {"      - code: validation_failed", "      - code: Validation Failed", "code"},
		"duplicate":      {"      - code: not_found", "      - code: validation_failed", "twice"},
	} {
		if !strings.Contains(base, c.replace) {
			t.Fatalf("%s: fixture lacks %q", name, c.replace)
		}
		writeFile(t, spec, strings.Replace(base, c.replace, c.with, 1))
		_, err := loadInputs(spec, catalog, list)
		problems := problemsOf(t, err)
		if len(problems) != 1 || !strings.Contains(problems[0], c.want) {
			t.Errorf("%s: problems %v, want one naming %q", name, problems, c.want)
		}
	}
}

func TestRenderMeta_Fixture(t *testing.T) {
	spec, catalog, list := fixturePaths(t, "")
	in, err := loadInputs(spec, catalog, list)
	if err != nil {
		t.Fatal(err)
	}
	in.gatewayVersions = []int{1, 2}
	files, err := renderRoutes(in, corePkg, fixtureRestPkg)
	if err != nil {
		t.Fatal(err)
	}
	src := string(files["meta.gen.go"])
	for _, want := range []string{
		`const SpecVersion = "0.0.1"`,
		"return []int{1, 2}",
		`Code: "not_found", Status: 404`,
		"routes.ProblemType",
	} {
		if !strings.Contains(src, want) {
			t.Errorf("meta.gen.go lacks %q:\n%s", want, src)
		}
	}
	if _, err := os.Stat(spec); err != nil {
		t.Fatal(err)
	}
}
