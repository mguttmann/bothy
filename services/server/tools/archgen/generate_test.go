// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

var update = flag.Bool("update", false, "rewrite testdata/golden/go-arch-lint.yml from the golden register")

const (
	goldenRegister = "testdata/golden/modules.yaml"
	goldenConfig   = "testdata/golden/go-arch-lint.yml"
	repoRegister   = "../../architecture/modules.yaml"
	repoConfig     = "../../.go-arch-lint.yml"
)

func mustRead(t *testing.T, path string) *register {
	t.Helper()
	r, err := readRegister(path)
	if err != nil {
		t.Fatalf("readRegister(%s):\n%v", path, err)
	}
	return r
}

func mustGenerate(t *testing.T, r *register) []byte {
	t.Helper()
	out, err := generate(r)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	return out
}

func TestGenerateGolden(t *testing.T) {
	t.Parallel()
	got := mustGenerate(t, mustRead(t, goldenRegister))
	if *update {
		if err := os.WriteFile(goldenConfig, got, 0o600); err != nil {
			t.Fatalf("update golden file: %v", err)
		}
	}
	want, err := os.ReadFile(goldenConfig)
	if err != nil {
		t.Fatalf("read golden file (run go test -run TestGenerateGolden -update to create it): %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("generated config differs from %s (run go test -run TestGenerateGolden -update after checking the change)\n--- got\n%s\n--- want\n%s", goldenConfig, got, want)
	}
}

// reverseMappings parses the register text, reverses the key order of the named top-level
// mappings and encodes it again, so that a test can feed the generator the same register with
// another order of its maps.
func reverseMappings(t *testing.T, data []byte, keys ...string) []byte {
	t.Helper()
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parse: %v", err)
	}
	top := doc.Content[0]
	for i := 0; i+1 < len(top.Content); i += 2 {
		if !slices.Contains(keys, top.Content[i].Value) {
			continue
		}
		m := top.Content[i+1]
		pairs := make([][2]*yaml.Node, 0, len(m.Content)/2)
		for j := 0; j+1 < len(m.Content); j += 2 {
			pairs = append(pairs, [2]*yaml.Node{m.Content[j], m.Content[j+1]})
		}
		slices.Reverse(pairs)
		m.Content = m.Content[:0]
		for _, p := range pairs {
			m.Content = append(m.Content, p[0], p[1])
		}
	}
	out, err := yaml.Marshal(&doc)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	return out
}

func TestGenerateIsDeterministic(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile(goldenRegister)
	if err != nil {
		t.Fatal(err)
	}
	first := mustGenerate(t, mustRead(t, goldenRegister))
	second := mustGenerate(t, mustRead(t, goldenRegister))
	if !bytes.Equal(first, second) {
		t.Fatalf("two runs on the same register differ:\n%s\n---\n%s", first, second)
	}

	reordered := reverseMappings(t, data, "platform", "vendors")
	if bytes.Equal(reordered, data) {
		t.Fatal("reordering changed nothing; the test would prove nothing")
	}
	r, problems := parseRegister(reordered)
	if len(problems) != 0 {
		t.Fatalf("reordered register invalid:\n%s", formatProblems(problems))
	}
	if third := mustGenerate(t, r); !bytes.Equal(first, third) {
		t.Errorf("reordering platform and vendors changed the output:\n%s\n---\n%s", first, third)
	}
}

func TestGenerateMinimalRegister(t *testing.T) {
	t.Parallel()
	text := strings.Join([]string{
		"version: 1",
		"exclude: []",
		"kernel: {core: [ids], ports: [txn]}",
		"platform: {}",
		"wire: {vendors: []}",
		"cmd: {vendors: []}",
		"testsupport: {paths: [test]}",
		"modules:",
		"  chat: {tables: [], actions: [], events: [], ephemeral_events: [], ws_ops: [], depends_on: [], adapter_vendors: []}",
		"vendors: {}",
		"",
	}, "\n")
	r, problems := parseRegister([]byte(text))
	if len(problems) != 0 {
		t.Fatalf("problems:\n%s", formatProblems(problems))
	}
	out := string(mustGenerate(t, r))
	for _, want := range []string{
		"\nvendors: {}\n",
		"\nexclude: []\n",
		"  kernel-core:\n    mayDependOn: [kernel-core]\n",
		"  chat-domain:\n    mayDependOn: [chat-domain]\n",
		"  chat-adapters:\n    mayDependOn: [chat-adapters, chat, chat-app, chat-domain, kernel-ports]\n",
		"  testsupport:\n    in: [test/**]\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
	for _, unwanted := range []string{"api", "canUse", "platform-", "kernel-ports:\n    mayDependOn"} {
		if strings.Contains(out, unwanted) {
			t.Errorf("output contains %q:\n%s", unwanted, out)
		}
	}
}

func TestRepositoryRegisterIsValid(t *testing.T) {
	t.Parallel()
	r, err := readRegister(repoRegister)
	if err != nil {
		t.Fatalf("the repository register %s is not valid:\n%v", filepath.Clean(repoRegister), err)
	}
	problems, err := checkDirectories(r, filepath.Dir(repoConfig))
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) > 0 {
		t.Errorf("exclude entries of %s hit other directories:\n%s", filepath.Clean(repoRegister), formatProblems(problems))
	}
}

func TestRepositoryConfigIsCurrent(t *testing.T) {
	t.Parallel()
	r, err := readRegister(repoRegister)
	if err != nil {
		t.Fatalf("the repository register %s is not valid:\n%v", filepath.Clean(repoRegister), err)
	}
	want := mustGenerate(t, r)
	got, err := os.ReadFile(repoConfig)
	if err != nil {
		t.Fatalf("read %s (run go generate ./tools/archgen): %v", filepath.Clean(repoConfig), err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("%s is stale; run go generate ./tools/archgen and commit the result", filepath.Clean(repoConfig))
	}
}
