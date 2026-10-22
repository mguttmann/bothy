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

// TestAutocompleteKeys (M2-17 design note 10): the Max of command.autocomplete_max_choices equals
// the maxItems of GatewayAutocompleteResult.choices, AutocompleteAnswer.choices and
// AutocompleteResult.choices in docs/api/openapi.yaml, and the cross-check
// autocomplete_below_request_timeout refuses command.autocomplete_timeout not below
// http.request_timeout with both keys named and no value.
func TestAutocompleteKeys(t *testing.T) {
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
	var max string
	for _, k := range schema(t).Keys() {
		if k.Name == "command.autocomplete_max_choices" {
			max = k.Max
		}
	}
	if max == "" {
		t.Fatal("command.autocomplete_max_choices has no Max")
	}
	for _, name := range []string{"GatewayAutocompleteResult", "AutocompleteAnswer", "AutocompleteResult"} {
		proxy, ok := model.Model.Components.Schemas.Get(name)
		if !ok {
			t.Fatalf("no schema %s", name)
		}
		choices, ok := proxy.Schema().Properties.Get("choices")
		if !ok || choices.Schema().MaxItems == nil {
			t.Fatalf("%s.choices has no maxItems", name)
		}
		if got := strconv.FormatInt(*choices.Schema().MaxItems, 10); got != max {
			t.Errorf("%s.choices maxItems %s, the key's Max %s", name, got, max)
		}
	}
	for _, c := range []struct {
		timeout, request string
		ok               bool
	}{
		{"2500ms", "15s", true},
		{"14999ms", "15s", true},
		{"15s", "15s", false},
		{"20s", "15s", false},
	} {
		got := load(t, "BOTHY_COMMAND__AUTOCOMPLETE_TIMEOUT="+c.timeout, "BOTHY_HTTP__REQUEST_TIMEOUT="+c.request)
		if (len(got) == 0) != c.ok {
			t.Errorf("timeout %s, request %s: problems %v, want ok %v", c.timeout, c.request, got, c.ok)
			continue
		}
		if len(got) == 0 {
			continue
		}
		msg := got[0].String()
		if !strings.Contains(msg, "command.autocomplete_timeout") || !strings.Contains(msg, "http.request_timeout") || strings.Contains(msg, c.timeout) {
			t.Errorf("problem %q", msg)
		}
	}
}
