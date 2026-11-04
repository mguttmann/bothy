// SPDX-License-Identifier: Apache-2.0

package wire_test

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/pb33f/libopenapi"
)

// TestSearchAssistKeys (14 4.10): the Max of search.assist.max_hits and search.assist.max_terms
// equal the maxItems of the schemas that carry them, so every configured value can be answered;
// search.assist.timeout must be below http.request_timeout and search.assist.agent_id empty or a
// UUID, and neither problem repeats a value.
func TestSearchAssistKeys(t *testing.T) {
	t.Parallel()
	b, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "docs", "api", "openapi.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	doc, err := libopenapi.NewDocument(b)
	if err != nil {
		t.Fatal(err)
	}
	model, err := doc.BuildV3Model()
	if err != nil {
		t.Fatal(err)
	}
	maxOf := map[string]string{}
	for _, k := range schema(t).Keys() {
		maxOf[k.Name] = k.Max
	}
	for _, c := range []struct{ key, schema, member string }{
		{"search.assist.max_hits", "SearchAssistResult", "hits"},
		{"search.assist.max_hits", "SearchAssistAnswer", "picks"},
		{"search.assist.max_terms", "SearchAssistAnswer", "terms"},
		{"search.assist.max_turns", "SearchAssistQuery", "history"},
	} {
		proxy, ok := model.Model.Components.Schemas.Get(c.schema)
		if !ok {
			t.Fatalf("no schema %s", c.schema)
		}
		member, ok := proxy.Schema().Properties.Get(c.member)
		if !ok || member.Schema().MaxItems == nil {
			t.Fatalf("%s.%s has no maxItems", c.schema, c.member)
		}
		if got := strconv.FormatInt(*member.Schema().MaxItems, 10); got != maxOf[c.key] {
			t.Errorf("%s.%s maxItems %s, the Max of %s %q", c.schema, c.member, got, c.key, maxOf[c.key])
		}
	}
	for _, c := range []struct {
		env  []string
		ok   bool
		keys []string
	}{
		{[]string{"BOTHY_SEARCH__ASSIST__TIMEOUT=14s", "BOTHY_HTTP__REQUEST_TIMEOUT=15s"}, true, nil},
		{[]string{"BOTHY_SEARCH__ASSIST__TIMEOUT=15s", "BOTHY_HTTP__REQUEST_TIMEOUT=15s"}, false, []string{"search.assist.timeout", "http.request_timeout"}},
		{[]string{"BOTHY_SEARCH__ASSIST__AGENT_ID=0192f4a0-7b1c-78f9-8074-fa94e84bab7d"}, true, nil},
		{[]string{"BOTHY_SEARCH__ASSIST__AGENT_ID=finder-bot"}, false, []string{"search.assist.agent_id"}},
	} {
		got := load(t, c.env...)
		if (len(got) == 0) != c.ok {
			t.Errorf("%v: problems %v, want ok %v", c.env, got, c.ok)
			continue
		}
		if len(got) == 0 {
			continue
		}
		msg := got[0].String()
		for _, k := range c.keys {
			if !strings.Contains(msg, k) {
				t.Errorf("problem %q does not name %s", msg, k)
			}
		}
		if strings.Contains(msg, "finder-bot") || strings.Contains(msg, "15s") {
			t.Errorf("problem %q repeats a value", msg)
		}
	}
}
