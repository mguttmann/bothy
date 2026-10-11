// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

// TestChapter02ExamplesMatchGenerator holds the register example of docs/spec/02-architecture.md
// ("Module register as the source") and the generated excerpt ("go-arch-lint") against this generator:
// the example must be a valid register, and every entry the excerpt shows must equal, in the
// same order, what generate writes for that example. Without it the chapter drifts from the
// code as soon as one of them changes.
func TestChapter02ExamplesMatchGenerator(t *testing.T) {
	t.Parallel()
	spec := readChapter02(t)
	sample := yamlBlockAfter(t, spec, "### 7.1 ")
	excerpt := yamlBlockAfter(t, spec, "### 7.2 ")

	r, problems := parseRegister([]byte(withStubModules(t, sample)))
	if len(problems) > 0 {
		t.Fatalf("the register example of 02 section 7.1 is not valid:\n%s", formatProblems(problems))
	}
	out, err := generate(r)
	if err != nil {
		t.Fatal(err)
	}
	var generated, shown yaml.Node
	if err := yaml.Unmarshal(out, &generated); err != nil {
		t.Fatal(err)
	}
	if err := yaml.Unmarshal([]byte(excerpt), &shown); err != nil {
		t.Fatalf("the excerpt of 02 section 7.2 is not YAML: %v", err)
	}
	compareExcerpt(t, "", generated.Content[0], shown.Content[0])
}

// readChapter02 finds the repository root by walking up to the directory that contains
// docs/spec and returns chapter 02.
func readChapter02(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if info, err := os.Stat(filepath.Join(dir, "docs", "spec")); err == nil && info.IsDir() {
			root, err := os.OpenRoot(dir)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = root.Close() }()
			data, err := root.ReadFile("docs/spec/02-architecture.md")
			if err != nil {
				t.Fatal(err)
			}
			return string(data)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("no directory with docs/spec above the working directory")
		}
		dir = parent
	}
}

// yamlBlockAfter returns the first fenced yaml block after the line that starts with heading.
func yamlBlockAfter(t *testing.T, text, heading string) string {
	t.Helper()
	start := strings.Index(text, "\n"+heading)
	if start < 0 {
		t.Fatalf("02 has no heading %q", heading)
	}
	rest := text[start:]
	open := strings.Index(rest, "\n```yaml\n")
	if open < 0 {
		t.Fatalf("no yaml block after %q", heading)
	}
	rest = rest[open+len("\n```yaml\n"):]
	end := strings.Index(rest, "\n```")
	if end < 0 {
		t.Fatalf("unterminated yaml block after %q", heading)
	}
	return rest[:end+1]
}

// withStubModules adds an empty entry for every module the example names in depends_on but does
// not list: the example shows one module and leaves out the others.
func withStubModules(t *testing.T, sample string) string {
	t.Helper()
	var doc struct {
		Modules map[string]struct {
			DependsOn []string `yaml:"depends_on"`
		} `yaml:"modules"`
	}
	if err := yaml.Unmarshal([]byte(sample), &doc); err != nil {
		t.Fatalf("the register example of 02 section 7.1 is not YAML: %v", err)
	}
	var missing []string
	for _, m := range doc.Modules {
		for _, d := range m.DependsOn {
			if _, ok := doc.Modules[d]; !ok && !slices.Contains(missing, d) {
				missing = append(missing, d)
			}
		}
	}
	slices.Sort(missing)
	var stubs strings.Builder
	for _, m := range missing {
		stubs.WriteString("  " + m + ": {tables: [], actions: [], events: [], ephemeral_events: [], " +
			"ws_ops: [], depends_on: [], adapter_vendors: []}\n")
	}
	const marker = "\nvendors:"
	if !strings.Contains(sample, marker) {
		t.Fatal("the register example of 02 section 7.1 has no top-level vendors")
	}
	return strings.Replace(sample, marker, "\n"+strings.TrimSuffix(stubs.String(), "\n")+marker, 1)
}

// compareExcerpt checks that every key of the shown mapping exists in the generated one, in the
// same relative order. The excerpt may leave keys out at the top level and below components,
// deps and vendors; every other value it shows must equal the generated value as a whole.
func compareExcerpt(t *testing.T, path string, generated, shown *yaml.Node) {
	t.Helper()
	keys := keyOrder(generated)
	last := -1
	for i := 0; i+1 < len(shown.Content); i += 2 {
		key := shown.Content[i].Value
		at := slices.Index(keys, key)
		if at < 0 {
			t.Errorf("02 excerpt shows %s%s, the generator writes no such key", path, key)
			continue
		}
		if at < last {
			t.Errorf("02 excerpt shows %s%s out of the generated order", path, key)
		}
		last = at
		want := generated.Content[2*at+1]
		got := shown.Content[i+1]
		switch key {
		case "components", "deps", "vendors":
			compareExcerpt(t, path+key+".", want, got)
		default:
			if !reflect.DeepEqual(plain(t, want), plain(t, got)) {
				t.Errorf("02 excerpt %s%s = %v, generator writes %v", path, key, plain(t, got), plain(t, want))
			}
		}
	}
}

func keyOrder(m *yaml.Node) []string {
	var keys []string
	for i := 0; i+1 < len(m.Content); i += 2 {
		keys = append(keys, m.Content[i].Value)
	}
	return keys
}

// plain decodes a node into maps, lists and scalars; a lone string under "in" counts as a list
// with that string, because go-arch-lint reads both forms alike.
func plain(t *testing.T, n *yaml.Node) any {
	t.Helper()
	var v any
	if err := n.Decode(&v); err != nil {
		t.Fatal(err)
	}
	if m, ok := v.(map[string]any); ok {
		if s, ok := m["in"].(string); ok {
			m["in"] = []any{s}
		}
	}
	return v
}
