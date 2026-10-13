// SPDX-License-Identifier: Apache-2.0

package httpserver_test

import (
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
)

// fieldErrors returns the errors[] of a validation problem as "pointer code" strings.
func fieldErrors(t *testing.T, resp *httpservertest.Response) []string {
	t.Helper()
	p := wantProblem(t, resp, 400, "validation_failed")
	var out []string
	list, _ := p["errors"].([]any)
	for _, e := range list {
		m := e.(map[string]any)
		out = append(out, m["pointer"].(string)+" "+m["code"].(string))
		if d, _ := m["detail"].(string); d == "" {
			t.Errorf("field error %v has no detail", m)
		}
	}
	return out
}

func wantFields(t *testing.T, got []string, want ...string) {
	t.Helper()
	for _, w := range want {
		if !slices.Contains(got, w) {
			t.Errorf("errors[] %v lacks %q", got, w)
		}
	}
}

// validationHarness mounts every operation of the test contract that the probes need.
func validationHarness(t *testing.T) *httpservertest.Harness {
	t.Helper()
	ok := func(action string) http.HandlerFunc {
		return httpservertest.Authorized(action, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(204) })
	}
	shape := httpservertest.Op("createShape", http.MethodPost, "/api/v1/shapes", "shape.create")
	composed := httpservertest.Op("createComposed", http.MethodPost, "/api/v1/composed", "composed.create")
	create := httpservertest.Op("createThing", http.MethodPost, "/api/v1/things", "thing.create")
	get := getThing()
	return httpservertest.New(t, httpservertest.Options{Routes: []httpservertest.Route{
		{Op: shape, Handler: ok(shape.Action)}, {Op: composed, Handler: ok(composed.Action)},
		{Op: create, Handler: ok(create.Action)}, {Op: get, Handler: ok(get.Action)},
	}})
}

func post(h *httpservertest.Harness, path, body string) *httpservertest.Response {
	return h.Do(h.NewRequest(http.MethodPost, path, body))
}

func TestW7_UnknownMemberRejected(t *testing.T) {
	t.Parallel()
	h := validationHarness(t)
	wantFields(t, fieldErrors(t, post(h, "/api/v1/things", `{"name":"ab","color":"red","bogus":1}`)), "/bogus unknown_member")
	wantFields(t, fieldErrors(t, post(h, "/api/v1/things", `{"name":"ab","color":"red","nested":{"x~y/z":1}}`)), "/nested/x~0y~1z unknown_member")
	// JsonObject stays free form.
	if resp := post(h, "/api/v1/things", `{"name":"ab","color":"red","extra":{"anything":[1]}}`); resp.StatusCode != 204 {
		t.Fatalf("free-form member refused: %d %s", resp.StatusCode, resp.Body)
	}
}

func TestValidate_ErrorShape(t *testing.T) {
	t.Parallel()
	h := validationHarness(t)
	wantFields(t, fieldErrors(t, post(h, "/api/v1/things", `{"color":"blue","name":"a","owner_id":"nope","size":11,"tags":[],"nested":{"flag":"x"}}`)),
		"/name too_short", "/color invalid_enum", "/owner_id invalid_format", "/size too_large", "/tags too_few_items", "/nested/flag invalid_type")
	wantFields(t, fieldErrors(t, post(h, "/api/v1/things", `{"color":"red","name":"ABCDEFGHIJ","tags":["a","b","c"],"size":0}`)),
		"/name too_long", "/name pattern_mismatch", "/tags too_many_items", "/size too_small")
	wantFields(t, fieldErrors(t, post(h, "/api/v1/things", `{"color":"red"}`)), "/name required")
	wantFields(t, fieldErrors(t, post(h, "/api/v1/things", `{"name":null,"color":"red"}`)), "/name invalid_type")
	resp := h.Do(h.NewRequest(http.MethodGet, "/api/v1/things/not-a-uuid?limit=500", "", "X-Mode", "medium"))
	wantFields(t, fieldErrors(t, resp), "#path/thing_id invalid_format", "#query/limit too_large", "#header/X-Mode invalid_enum")
	resp = h.Do(h.NewRequest(http.MethodGet, "/api/v1/things/"+thingID, ""))
	wantFields(t, fieldErrors(t, resp), "#header/X-Mode required")
	for _, b := range [][]byte{resp.Body} {
		if strings.Contains(string(b), "medium") || strings.Contains(string(b), "not-a-uuid") {
			t.Fatalf("submitted value echoed: %s", b)
		}
	}
}

func TestValidator_Probe_AdditionalPropertiesFalse(t *testing.T) {
	t.Parallel()
	wantFields(t, fieldErrors(t, post(validationHarness(t), "/api/v1/things", `{"name":"ab","color":"red","x":1,"y":2}`)), "/x unknown_member", "/y unknown_member")
}

func TestValidator_Probe_FormatUUID(t *testing.T) {
	t.Parallel()
	h := validationHarness(t)
	wantFields(t, fieldErrors(t, post(h, "/api/v1/things", `{"name":"ab","color":"red","owner_id":"0192f4a0"}`)), "/owner_id invalid_format")
	if resp := post(h, "/api/v1/things", `{"name":"ab","color":"red","owner_id":"`+thingID+`"}`); resp.StatusCode != 204 {
		t.Fatalf("valid uuid refused: %s", resp.Body)
	}
}

func TestValidator_Probe_Enum(t *testing.T) {
	t.Parallel()
	wantFields(t, fieldErrors(t, post(validationHarness(t), "/api/v1/things", `{"name":"ab","color":"pink"}`)), "/color invalid_enum")
}

func TestValidator_Probe_MinLength(t *testing.T) {
	t.Parallel()
	wantFields(t, fieldErrors(t, post(validationHarness(t), "/api/v1/things", `{"name":"a","color":"red"}`)), "/name too_short")
}

func TestValidator_Probe_MaxItems(t *testing.T) {
	t.Parallel()
	wantFields(t, fieldErrors(t, post(validationHarness(t), "/api/v1/things", `{"name":"ab","color":"red","tags":["a","b","c"]}`)), "/tags too_many_items")
}

func TestValidator_Probe_OneOfDiscriminator(t *testing.T) {
	t.Parallel()
	h := validationHarness(t)
	if resp := post(h, "/api/v1/shapes", `{"kind":"circle","radius":2}`); resp.StatusCode != 204 {
		t.Fatalf("valid circle refused: %s", resp.Body)
	}
	if resp := post(h, "/api/v1/shapes", `{"kind":"square","side":2}`); resp.StatusCode != 204 {
		t.Fatalf("valid square refused: %s", resp.Body)
	}
	wantFields(t, fieldErrors(t, post(h, "/api/v1/shapes", `{"kind":"circle","side":2}`)), "/side unknown_member", "/radius required")
	fieldErrors(t, post(h, "/api/v1/shapes", `{"kind":"hexagon"}`))
}

func TestValidator_Probe_AllOfUnevaluated(t *testing.T) {
	t.Parallel()
	h := validationHarness(t)
	if resp := post(h, "/api/v1/composed", `{"id":"`+thingID+`","label":"x"}`); resp.StatusCode != 204 {
		t.Fatalf("valid composed refused: %s", resp.Body)
	}
	wantFields(t, fieldErrors(t, post(h, "/api/v1/composed", `{"id":"`+thingID+`","label":"x","bogus":1}`)), "/bogus unknown_member")
}

func TestValidator_Probe_HeaderAndPathParameters(t *testing.T) {
	t.Parallel()
	h := validationHarness(t)
	if resp := h.Do(h.NewRequest(http.MethodGet, "/api/v1/things/"+thingID+"?limit=5", "", "X-Mode", "fast")); resp.StatusCode != 204 {
		t.Fatalf("valid request refused: %s", resp.Body)
	}
	wantFields(t, fieldErrors(t, h.Do(h.NewRequest(http.MethodGet, "/api/v1/things/x", "", "X-Mode", "fast"))), "#path/thing_id invalid_format")
	wantFields(t, fieldErrors(t, h.Do(h.NewRequest(http.MethodGet, "/api/v1/things/"+thingID, "", "X-Mode", "slowest"))), "#header/X-Mode invalid_enum")
}

func TestNewValidatorErrors(t *testing.T) {
	t.Parallel()
	if _, err := httpserver.NewValidator([]byte("not: [openapi"), nil); err == nil {
		t.Error("broken document accepted")
	}
	if _, err := httpserver.NewValidator(httpservertest.Contract, []string{"noSuchOperation"}); err == nil || !strings.Contains(err.Error(), "noSuchOperation") {
		t.Errorf("unknown operation: %v", err)
	}
	if _, err := httpserver.NewValidator(httpservertest.Contract, []string{"getThing", "getThing"}); err == nil {
		t.Error("repeated operation accepted")
	}
	v, err := httpserver.NewValidator(httpservertest.Contract, []string{"getThing"})
	if err != nil {
		t.Fatal(err)
	}
	r, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/things/"+thingID, nil)
	if err := v.Validate(r, httpserver.Operation{ID: "createThing"}); err == nil {
		t.Error("an operation outside the validator must fail closed")
	}
}
