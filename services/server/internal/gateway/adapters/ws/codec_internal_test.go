// SPDX-License-Identifier: Apache-2.0

package ws

import (
	"encoding/json"
	"maps"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/api/routes"
	"github.com/OWNER/bothy/services/server/internal/gateway/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
)

// TestDecode_FocusAndTyping: the bodies of focus, typing.start and typing.stop decode into the
// frames of the app package after their schema; an unknown member and a non-UUID fail.
func TestDecode_FocusAndTyping(t *testing.T) {
	t.Parallel()
	schemas := map[string]string{"focus": "GatewayFocus", "typing.start": "GatewayTyping", "typing.stop": "GatewayTyping"}
	check, err := httpserver.NewSchemaValidator(rest.Spec, []string{"GatewayFocus", "GatewayTyping"})
	if err != nil {
		t.Fatal(err)
	}
	dec := NewDecoder(schemas, check)
	a, b := ids.New(), ids.New()
	raw, _ := json.Marshal(map[string]any{"thread_ids": []ids.ID{a, b}, "idle": true})
	got, err := dec.Decode("focus", raw)
	if f, ok := got.(app.Focus); err != nil || !ok || len(f.ThreadIDs) != 2 || f.ThreadIDs[0] != a || f.ThreadIDs[1] != b || !f.Idle {
		t.Errorf("focus: %#v %v", got, err)
	}
	got, err = dec.Decode("focus", json.RawMessage(`{"thread_ids":[]}`))
	if f, ok := got.(app.Focus); err != nil || !ok || len(f.ThreadIDs) != 0 || f.Idle || len(f.BackgroundThreadIDs) != 0 {
		t.Errorf("empty focus: %#v %v", got, err)
	}
	// background_thread_ids reaches the frame next to thread_ids (M2-51, R-SY-27).
	raw, _ = json.Marshal(map[string]any{"thread_ids": []ids.ID{a}, "background_thread_ids": []ids.ID{b}})
	got, err = dec.Decode("focus", raw)
	if f, ok := got.(app.Focus); err != nil || !ok || !slices.Equal(f.ThreadIDs, []ids.ID{a}) || !slices.Equal(f.BackgroundThreadIDs, []ids.ID{b}) {
		t.Errorf("focus with background threads: %#v %v", got, err)
	}
	for _, op := range []string{"typing.start", "typing.stop"} {
		raw, _ := json.Marshal(map[string]any{"thread_id": a})
		got, err := dec.Decode(op, raw)
		if f, ok := got.(app.Typing); err != nil || !ok || f.ThreadID != a {
			t.Errorf("%s: %#v %v", op, got, err)
		}
	}
	for op, body := range map[string]string{
		"focus":        `{"thread_ids":[],"extra":1}`,
		"typing.start": `{"thread_id":"not-a-uuid"}`,
		"typing.stop":  `{}`,
	} {
		if _, err := dec.Decode(op, json.RawMessage(body)); err == nil {
			t.Errorf("%s %s decoded", op, body)
		}
	}
}

// The two IDs of the sample bodies.
const (
	sampleA = "0192f4a0-7b1c-7000-8a00-0000000000a1"
	sampleB = "0192f4a0-7b1c-7000-8a00-0000000000b2"
)

// frameSample is, for one body schema of a client op, the generated type that names its
// properties and two valid bodies that differ in every property.
type frameSample struct {
	typ  any
	a, b map[string]any
}

// frameSamples holds the sample of every body schema of gateway_ops.enabled.
var frameSamples = map[string]frameSample{
	"GatewayAck": {rest.GatewayAck{}, map[string]any{"seq": 1}, map[string]any{"seq": 2}},
	"GatewayAutocompleteResult": {rest.GatewayAutocompleteResult{},
		map[string]any{"request_id": sampleA, "choices": []any{map[string]any{"name": "High", "value": "high"}}, "ref": "a"},
		map[string]any{"request_id": sampleB, "choices": []any{map[string]any{"name": "Low", "value": "low"}}, "ref": "b"}},
	"GatewayFocus": {rest.GatewayFocus{},
		map[string]any{"thread_ids": []string{sampleA}, "idle": false, "background_thread_ids": []string{sampleA}},
		map[string]any{"thread_ids": []string{sampleB}, "idle": true, "background_thread_ids": []string{sampleB}}},
	"GatewayHeartbeat": {rest.GatewayHeartbeat{},
		map[string]any{"last_seq": 1, "agent": map[string]any{"state": "online"}},
		map[string]any{"last_seq": 2, "agent": map[string]any{"state": "degraded"}}},
	"GatewayIdentify": {rest.GatewayIdentify{},
		map[string]any{"protocol_version": 1, "access_token": "access-a", "api_token": "api-a", "dpop": "proof-a", "device_id": sampleA,
			"consumer": "default", "cursor": 1, "sync_epoch": sampleA, "client": map[string]any{"platform": "macos", "version": "1.0.0"}},
		map[string]any{"protocol_version": 2, "access_token": "access-b", "api_token": "api-b", "dpop": "proof-b", "device_id": sampleB,
			"consumer": "second", "cursor": 2, "sync_epoch": sampleB, "client": map[string]any{"platform": "windows", "version": "2.0.0"}}},
	"GatewayReauth": {rest.GatewayReauth{},
		map[string]any{"access_token": "access-a", "dpop": "proof-a"}, map[string]any{"access_token": "access-b", "dpop": "proof-b"}},
	"GatewayResume": {rest.GatewayResume{},
		map[string]any{"gateway_session_id": sampleA, "access_token": "access-a", "api_token": "api-a", "dpop": "proof-a", "cursor": 1,
			"sync_epoch": sampleA},
		map[string]any{"gateway_session_id": sampleB, "access_token": "access-b", "api_token": "api-b", "dpop": "proof-b", "cursor": 2,
			"sync_epoch": sampleB}},
	"GatewayRunUpdate": {rest.GatewayRunUpdate{},
		map[string]any{"run_id": sampleA, "base_version": 1, "ops": []any{map[string]any{"op": "replace", "path": "/status_line", "value": "a"}},
			"ref": "a"},
		map[string]any{"run_id": sampleB, "base_version": 2, "ops": []any{map[string]any{"op": "add", "path": "/model", "value": "b"}},
			"ref": "b"}},
	"GatewayStatsSubscribe":   {rest.GatewayStatsSubscribe{}, map[string]any{"space_id": sampleA}, map[string]any{"space_id": sampleB}},
	"GatewayStatsUnsubscribe": {rest.GatewayStatsUnsubscribe{}, map[string]any{}, map[string]any{}},
	"GatewayTyping":           {rest.GatewayTyping{}, map[string]any{"thread_id": sampleA}, map[string]any{"thread_id": sampleB}},
}

// propertiesOf returns the property names of the generated body type v, sorted; a schema without
// properties is generated as a map and has none.
func propertiesOf(v any) []string {
	out := []string{}
	rt := reflect.TypeOf(v)
	if rt.Kind() != reflect.Struct {
		return out
	}
	for i := range rt.NumField() {
		if name, _, _ := strings.Cut(rt.Field(i).Tag.Get("json"), ","); name != "" && name != "-" {
			out = append(out, name)
		}
	}
	slices.Sort(out)
	return out
}

// TestDecode_EveryPropertyReachesTheFrame holds the class of the finding srv-gateway-sync-1 (audit
// of 2026-11-11, where focus lost background_thread_ids between the schema and the handler): for
// every op of gateway_ops.enabled the decoder of production hands every property of the body
// schema on to the frame of the app package. It decodes a body with every property, then the same
// body with one property changed, and requires another frame. The generated type names the
// properties, so a property added to a schema fails here until it has a sample and reaches the
// frame.
func TestDecode_EveryPropertyReachesTheFrame(t *testing.T) {
	t.Parallel()
	schemas := map[string]string{}
	var names []string
	for _, op := range routes.GatewayOps() {
		schemas[op.Op] = op.Schema
		if !slices.Contains(names, op.Schema) {
			names = append(names, op.Schema)
		}
	}
	check, err := httpserver.NewSchemaValidator(rest.Spec, names)
	if err != nil {
		t.Fatal(err)
	}
	dec := NewDecoder(schemas, check)
	decode := func(op string, body map[string]any) any {
		t.Helper()
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		f, err := dec.Decode(op, raw)
		if err != nil {
			t.Fatalf("%s %s: %v", op, raw, err)
		}
		return f
	}
	for op, schema := range schemas {
		s, ok := frameSamples[schema]
		if !ok {
			t.Errorf("%s: no sample for the schema %s", op, schema)
			continue
		}
		props := propertiesOf(s.typ)
		if got := slices.Sorted(maps.Keys(s.a)); !slices.Equal(got, props) {
			t.Errorf("%s: the sample has the properties %v, the schema %s has %v", op, got, schema, props)
			continue
		}
		base := decode(op, s.a)
		for _, p := range props {
			body := maps.Clone(s.a)
			body[p] = s.b[p]
			if reflect.DeepEqual(base, decode(op, body)) {
				t.Errorf("%s: the property %s of %s does not reach the frame %T", op, p, schema, base)
			}
		}
	}
}
