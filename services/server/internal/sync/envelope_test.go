// SPDX-License-Identifier: Apache-2.0

package sync_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"go.yaml.in/yaml/v3"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/sync"
)

// stored returns a durable event as the log tail reads it, with every column set.
func stored() sync.Event {
	return sync.Event{
		Seq: 42, ID: ids.New(), Type: "message.created", SchemaVersion: 1,
		OccurredAt: time.Date(2026, 10, 12, 8, 30, 0, 123_000_000, time.FixedZone("x", 3600)),
		ActorKind:  "user", ActorPrincipalID: ids.New(),
		SpaceID: ids.New(), ChannelID: ids.New(), ThreadID: ids.New(),
		ResourceKind: "message", ResourceID: ids.New(), ResourceVersion: 3,
		AudiencePrincipalID: ids.New(), ReceiveAction: "self",
		Payload: json.RawMessage(`{"content_md":"hi"}`), Truncated: true, Redacted: true,
		CorrelationID: ids.New(), TraceContext: "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01",
		ClientMsgID: ids.New(), PermissionVersion: 7,
	}
}

// decode marshals env and decodes it into a generic map.
func decode(t *testing.T, env sync.Envelope) map[string]any {
	t.Helper()
	b, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestEnvelope_AllMembersOfAStoredEvent(t *testing.T) {
	t.Parallel()
	e := stored()
	m := decode(t, e.Envelope(sync.DeliveryFields{IncludeTraceContext: true}))
	if m["occurred_at"] != "2026-10-12T07:30:00.123Z" {
		t.Errorf("occurred_at = %v, want RFC 3339 in UTC", m["occurred_at"])
	}
	actor, _ := m["actor"].(map[string]any)
	if actor["kind"] != "user" || actor["id"] != e.ActorPrincipalID.String() {
		t.Errorf("actor = %v", m["actor"])
	}
	scope, _ := m["scope"].(map[string]any)
	if scope["space_id"] != e.SpaceID.String() || scope["channel_id"] != e.ChannelID.String() || scope["thread_id"] != e.ThreadID.String() {
		t.Errorf("scope = %v", scope)
	}
	res, _ := m["resource"].(map[string]any)
	if res["kind"] != "message" || res["id"] != e.ResourceID.String() || res["version"] != float64(3) {
		t.Errorf("resource = %v", res)
	}
	if m["audience"] != "self" || m["redacted"] != true || m["truncated"] != true || m["seq"] != float64(42) ||
		m["permission_version"] != float64(7) || m["type"] != "message.created" || m["schema_version"] != float64(1) ||
		m["correlation_id"] != e.CorrelationID.String() || m["client_msg_id"] != e.ClientMsgID.String() ||
		m["trace_context"] != e.TraceContext || m["id"] != e.ID.String() {
		t.Errorf("envelope = %v", m)
	}
	if p, _ := m["payload"].(map[string]any); p["content_md"] != "hi" {
		t.Errorf("payload = %v", m["payload"])
	}
}

func TestEnvelope_AbsentColumnsAreAbsentMembers(t *testing.T) {
	t.Parallel()
	e := sync.Event{Seq: 1, ID: ids.New(), Type: "grant.expired", SchemaVersion: 1, OccurredAt: time.Now(),
		ResourceKind: "grant", ReceiveAction: "grant.read", Payload: json.RawMessage(`{}`)}
	m := decode(t, e.Envelope(sync.DeliveryFields{}))
	for _, k := range []string{"actor", "audience", "redacted", "truncated", "correlation_id", "trace_context",
		"client_msg_id", "invocation_token", "redelivered", "sync_epoch"} {
		if _, ok := m[k]; ok {
			t.Errorf("member %s is present for an absent column", k)
		}
	}
	if scope, ok := m["scope"].(map[string]any); !ok || len(scope) != 0 {
		t.Errorf("scope = %v, want an empty object", m["scope"])
	}
	if res, _ := m["resource"].(map[string]any); len(res) != 1 || res["kind"] != "grant" {
		t.Errorf("resource = %v, want only kind", m["resource"])
	}
	e.ActorKind = "system"
	if actor, _ := decode(t, e.Envelope(sync.DeliveryFields{}))["actor"].(map[string]any); len(actor) != 1 || actor["kind"] != "system" {
		t.Errorf("system actor = %v, want only kind", actor)
	}
}

func TestEnvelope_PermissionVersionZeroPresent(t *testing.T) {
	t.Parallel()
	e := stored()
	e.PermissionVersion = 0
	m := decode(t, e.Envelope(sync.DeliveryFields{}))
	if v, ok := m["permission_version"]; !ok || v != float64(0) {
		t.Fatalf("permission_version = %v (present %v), want 0", v, ok)
	}
	if _, ok := m["seq"]; !ok {
		t.Fatal("seq is missing")
	}
}

func TestEnvelope_DeliveryFieldsOnlyWhenSet(t *testing.T) {
	t.Parallel()
	e := stored()
	m := decode(t, e.Envelope(sync.DeliveryFields{}))
	for _, k := range []string{"invocation_token", "redelivered", "sync_epoch"} {
		if _, ok := m[k]; ok {
			t.Errorf("delivery member %s without a delivery field", k)
		}
	}
	epoch := ids.New()
	m = decode(t, e.Envelope(sync.DeliveryFields{InvocationToken: "invocation-example", Redelivered: true, SyncEpoch: epoch}))
	if m["invocation_token"] != "invocation-example" || m["redelivered"] != true || m["sync_epoch"] != epoch.String() {
		t.Fatalf("delivery members = %v", m)
	}
}

func TestEnvelope_TraceContextOnlyForAgents(t *testing.T) {
	t.Parallel()
	e := stored()
	if _, ok := decode(t, e.Envelope(sync.DeliveryFields{}))["trace_context"]; ok {
		t.Fatal("trace_context without IncludeTraceContext (a delivery to a human)")
	}
	if got := decode(t, e.Envelope(sync.DeliveryFields{IncludeTraceContext: true}))["trace_context"]; got != e.TraceContext {
		t.Fatalf("trace_context = %v for an agent delivery", got)
	}
	e.TraceContext = ""
	if _, ok := decode(t, e.Envelope(sync.DeliveryFields{IncludeTraceContext: true}))["trace_context"]; ok {
		t.Fatal("trace_context present although the request was not recorded")
	}
}

func TestEnvelope_EmptyPayloadIsAnObject(t *testing.T) {
	t.Parallel()
	e := stored()
	for _, p := range []json.RawMessage{nil, json.RawMessage("null"), json.RawMessage(" null ")} {
		e.Payload = p
		if got, ok := decode(t, e.Envelope(sync.DeliveryFields{}))["payload"].(map[string]any); !ok || len(got) != 0 {
			t.Fatalf("payload %q must render as {}, never null", p)
		}
	}
}

// hasNull reports whether the JSON text b contains a null token anywhere.
func hasNull(t *testing.T, b []byte) bool {
	t.Helper()
	dec := json.NewDecoder(bytes.NewReader(b))
	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			return false
		}
		if err != nil {
			t.Fatalf("envelope is not valid JSON: %v", err)
		}
		if tok == nil {
			return true
		}
	}
}

// FuzzEnvelopeHasNoJSONNull holds ADR 0053 and R-SY-16 for the envelope: whatever columns are
// set or absent and whatever delivery fields a delivery adds, the encoded envelope has no JSON
// null. The payload is a valid object as the appender stores it (CheckPayload rejects null).
func FuzzEnvelopeHasNoJSONNull(f *testing.F) {
	f.Add(uint32(0), int64(0), int64(0), int64(0), "", "", "")
	f.Add(uint32(0xffffffff), int64(1), int64(5), int64(9), "message.created", "self", "tok")
	f.Add(uint32(0x5a5a5a5a), int64(-1), int64(1<<40), int64(-3), "x", "", "00-ab-cd-01")
	f.Fuzz(func(t *testing.T, mask uint32, seq, pv, version int64, text, action, token string) {
		bit := func(i uint) bool { return mask&(1<<i) != 0 }
		id := func(i uint) ids.ID {
			if bit(i) {
				return ids.New()
			}
			return ids.ID{}
		}
		payload, err := json.Marshal(map[string]any{"text": text, "n": seq, "list": []string{action}})
		if err != nil {
			t.Fatal(err)
		}
		if bit(20) {
			payload = nil
		}
		e := sync.Event{
			Seq: seq, ID: id(0), Type: text, SchemaVersion: int(version % 1000), OccurredAt: time.Unix(seq%1e9, 0),
			ActorPrincipalID: id(2), SpaceID: id(3), ChannelID: id(4), ThreadID: id(5), ResourceKind: text,
			ResourceID: id(6), ResourceVersion: version, AudiencePrincipalID: id(7), ReceiveAction: action,
			Payload: payload, Truncated: bit(8), Redacted: bit(9), CorrelationID: id(10), ClientMsgID: id(11),
			PermissionVersion: pv,
		}
		if bit(1) {
			e.ActorKind = []string{"user", "agent", "app", "system"}[mask%4]
		}
		if bit(12) {
			e.TraceContext = token
		}
		d := sync.DeliveryFields{IncludeTraceContext: bit(13), Redelivered: bit(14), SyncEpoch: id(15)}
		if bit(16) {
			d.InvocationToken = token
		}
		b, err := json.Marshal(e.Envelope(d))
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		if hasNull(t, b) {
			t.Fatalf("envelope contains JSON null: %s", b)
		}
	})
}

// openAPIPath returns docs/api/openapi.yaml of the repository.
func openAPIPath(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("no caller")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "docs", "api", "openapi.yaml")
}

// jsonNames returns the JSON member names of the struct type t.
func jsonNames(t reflect.Type) []string {
	var out []string
	for f := range t.Fields() {
		name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
		if name != "" && name != "-" {
			out = append(out, name)
		}
	}
	slices.Sort(out)
	return out
}

func TestEnvelope_FieldsMatchOpenAPI(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile(openAPIPath(t))
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Components struct {
			Schemas map[string]struct {
				Properties map[string]any `yaml:"properties"`
			} `yaml:"schemas"`
		} `yaml:"components"`
	}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	props := func(schema string, extra ...string) []string {
		s, ok := doc.Components.Schemas[schema]
		if !ok || len(s.Properties) == 0 {
			t.Fatalf("schema %s has no properties in openapi.yaml", schema)
		}
		out := append([]string(nil), extra...)
		for k := range s.Properties {
			out = append(out, k)
		}
		slices.Sort(out)
		return out
	}
	cases := []struct {
		schema string
		typ    reflect.Type
		extra  []string
	}{
		{"EventEnvelope", reflect.TypeFor[sync.Envelope](), []string{"type", "payload"}},
		{"EventActor", reflect.TypeFor[sync.EnvelopeActor](), nil},
		{"EventScope", reflect.TypeFor[sync.EnvelopeScope](), nil},
		{"EventResource", reflect.TypeFor[sync.EnvelopeResource](), nil},
	}
	for _, c := range cases {
		if got, want := jsonNames(c.typ), props(c.schema, c.extra...); !slices.Equal(got, want) {
			t.Errorf("%s: members %v, OpenAPI %v", c.typ, got, want)
		}
	}
}
