// SPDX-License-Identifier: Apache-2.0

package wire_test

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"go.yaml.in/yaml/v3"
)

// jsonDepth is the nesting depth as the JSON guard counts it: each object or array is one level.
func jsonDepth(v any) int {
	switch t := v.(type) {
	case map[string]any:
		d := 0
		for _, x := range t {
			d = max(d, jsonDepth(x))
		}
		return d + 1
	case []any:
		d := 0
		for _, x := range t {
			d = max(d, jsonDepth(x))
		}
		return d + 1
	}
	return 0
}

// requestExamples returns every example of a request body in the contract: example and
// examples.*.value of each media type, under paths and components.requestBodies.
func requestExamples(doc map[string]any) map[string]any {
	out := map[string]any{}
	collect := func(where string, body any) {
		b, _ := body.(map[string]any)
		content, _ := b["content"].(map[string]any)
		for media, m := range content {
			mt, _ := m.(map[string]any)
			if ex, ok := mt["example"]; ok {
				out[where+" "+media] = ex
			}
			exs, _ := mt["examples"].(map[string]any)
			for name, e := range exs {
				if em, ok := e.(map[string]any); ok {
					out[where+" "+media+" "+name] = em["value"]
				}
			}
		}
	}
	paths, _ := doc["paths"].(map[string]any)
	for path, item := range paths {
		ops, _ := item.(map[string]any)
		for method, op := range ops {
			if o, ok := op.(map[string]any); ok {
				collect(method+" "+path, o["requestBody"])
			}
		}
	}
	comps, _ := doc["components"].(map[string]any)
	bodies, _ := comps["requestBodies"].(map[string]any)
	for name, b := range bodies {
		collect("requestBodies/"+name, b)
	}
	return out
}

// TestMaxJSONDepth_CoversAllRequestExamples holds the rule of 02 for http.max_json_depth: every
// request example of openapi.yaml stays below the default, so the guard never refuses a body the
// contract shows.
func TestMaxJSONDepth_CoversAllRequestExamples(t *testing.T) {
	t.Parallel()
	limit := -1
	for _, k := range schema(t).Keys() {
		if k.Name == "http.max_json_depth" {
			n, err := strconv.Atoi(k.Default)
			if err != nil {
				t.Fatal(err)
			}
			limit = n
		}
	}
	if limit < 1 {
		t.Fatal("http.max_json_depth is not in the schema")
	}
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "docs", "api", "openapi.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	examples := requestExamples(doc)
	if len(examples) == 0 {
		t.Fatal("no request examples found; the walk proves nothing")
	}
	for where, ex := range examples {
		if d := jsonDepth(ex); d >= limit {
			t.Errorf("%s: depth %d, not below http.max_json_depth %d", where, d, limit)
		}
	}
	t.Logf("%d request examples checked", len(examples))
}
