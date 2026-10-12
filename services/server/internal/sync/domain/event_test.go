// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/sync/domain"
)

// valid returns fields that pass every rule; each case of TestValidateEvent breaks one.
func valid() domain.EventFields {
	return domain.EventFields{
		Type: "message.created", SchemaVersion: 1, ActorKind: "user", HasActorPrincipal: true,
		ResourceKind: "message", ReceiveAction: "message.read", Payload: []byte(`{"content_md":"hi"}`),
	}
}

func TestValidateEvent(t *testing.T) {
	t.Parallel()
	const maxBytes = 64
	cases := []struct {
		name  string
		edit  func(*domain.EventFields)
		valid bool
	}{
		{"valid", func(*domain.EventFields) {}, true},
		{"no actor", func(f *domain.EventFields) { f.ActorKind, f.HasActorPrincipal = "", false }, true},
		{"system without principal", func(f *domain.EventFields) { f.ActorKind, f.HasActorPrincipal = "system", false }, true},
		{"agent with principal", func(f *domain.EventFields) { f.ActorKind = "agent" }, true},
		{"app with principal", func(f *domain.EventFields) { f.ActorKind = "app" }, true},
		{"self with audience", func(f *domain.EventFields) { f.ReceiveAction, f.HasAudience = "self", true }, true},
		{"audience with another action", func(f *domain.EventFields) { f.HasAudience = true }, true},
		{"three segment type", func(f *domain.EventFields) { f.Type = "agent_session.state.changed" }, true},
		{"empty object payload", func(f *domain.EventFields) { f.Payload = []byte(`{}`) }, true},
		{"type without dot", func(f *domain.EventFields) { f.Type = "message" }, false},
		{"type with upper case", func(f *domain.EventFields) { f.Type = "Message.created" }, false},
		{"type with digit", func(f *domain.EventFields) { f.Type = "message.created2" }, false},
		{"type with trailing dot", func(f *domain.EventFields) { f.Type = "message." }, false},
		{"empty type", func(f *domain.EventFields) { f.Type = "" }, false},
		{"schema version 0", func(f *domain.EventFields) { f.SchemaVersion = 0 }, false},
		{"schema version above smallint", func(f *domain.EventFields) { f.SchemaVersion = 32768 }, false},
		{"empty resource kind", func(f *domain.EventFields) { f.ResourceKind = "" }, false},
		{"empty receive action", func(f *domain.EventFields) { f.ReceiveAction = "" }, false},
		{"self without audience", func(f *domain.EventFields) { f.ReceiveAction, f.HasAudience = "self", false }, false},
		{"unknown actor kind", func(f *domain.EventFields) { f.ActorKind = "robot" }, false},
		{"user without principal", func(f *domain.EventFields) { f.HasActorPrincipal = false }, false},
		{"agent without principal", func(f *domain.EventFields) { f.ActorKind, f.HasActorPrincipal = "agent", false }, false},
		{"app without principal", func(f *domain.EventFields) { f.ActorKind, f.HasActorPrincipal = "app", false }, false},
		{"system with principal", func(f *domain.EventFields) { f.ActorKind = "system" }, false},
		{"principal without kind", func(f *domain.EventFields) { f.ActorKind = "" }, false},
		{"nil payload", func(f *domain.EventFields) { f.Payload = nil }, false},
		{"array payload", func(f *domain.EventFields) { f.Payload = []byte(`[1]`) }, false},
		{"string payload", func(f *domain.EventFields) { f.Payload = []byte(`"x"`) }, false},
		{"null payload", func(f *domain.EventFields) { f.Payload = []byte(`null`) }, false},
		{"top level null member", func(f *domain.EventFields) { f.Payload = []byte(`{"a":null}`) }, false},
		{"nested null", func(f *domain.EventFields) { f.Payload = []byte(`{"a":{"b":[1,{"c":null}]}}`) }, false},
		{"null in array", func(f *domain.EventFields) { f.Payload = []byte(`{"a":[null]}`) }, false},
		{"invalid json", func(f *domain.EventFields) { f.Payload = []byte(`{"a":`) }, false},
		{"trailing data", func(f *domain.EventFields) { f.Payload = []byte(`{} {}`) }, false},
		{"NUL character", func(f *domain.EventFields) { f.Payload = []byte(`{"a":"x\u0000y"}`) }, false},
		{"NUL in a key", func(f *domain.EventFields) { f.Payload = []byte(`{"\u0000":1}`) }, false},
		{"oversized payload", func(f *domain.EventFields) { f.Payload = []byte(`{"a":"` + strings.Repeat("x", maxBytes) + `"}`) }, false},
		{"payload of exactly the limit", func(f *domain.EventFields) {
			f.Payload = []byte(`{"a":"` + strings.Repeat("x", maxBytes-8) + `"}`)
		}, false},
		{"payload one below the limit", func(f *domain.EventFields) {
			f.Payload = []byte(`{"a":"` + strings.Repeat("x", maxBytes-9) + `"}`)
		}, true},
		{"string containing null text", func(f *domain.EventFields) { f.Payload = []byte(`{"a":"null"}`) }, true},
		{"exponent number", func(f *domain.EventFields) { f.Payload = []byte(`{"a":1e100000}`) }, false},
		{"upper case exponent", func(f *domain.EventFields) { f.Payload = []byte(`{"a":[2.5E3]}`) }, false},
		{"plain decimals", func(f *domain.EventFields) { f.Payload = []byte(`{"a":-12.50,"b":0}`) }, true},
		{"invalid UTF-8", func(f *domain.EventFields) { f.Payload = []byte("{\"a\":\"\xff\"}") }, false},
		{"lone high surrogate", func(f *domain.EventFields) { f.Payload = []byte(`{"a":"\ud800"}`) }, false},
		{"lone low surrogate", func(f *domain.EventFields) { f.Payload = []byte(`{"a":"x\udc00"}`) }, false},
		{"surrogate in a key", func(f *domain.EventFields) { f.Payload = []byte(`{"\ud83d":1}`) }, false},
		{"surrogate pair", func(f *domain.EventFields) { f.Payload = []byte(`{"a":"\ud83d\ude00"}`) }, true},
		{"escaped backslash before u", func(f *domain.EventFields) { f.Payload = []byte(`{"a":"\\ud800"}`) }, true},
	}
	for _, c := range cases {
		f := valid()
		c.edit(&f)
		err := domain.ValidateEvent(f, maxBytes)
		switch {
		case c.valid && err != nil:
			t.Errorf("%s: ValidateEvent = %v, want nil", c.name, err)
		case !c.valid && err == nil:
			t.Errorf("%s: ValidateEvent accepted the event", c.name)
		case !c.valid && errs.KindOf(err) != errs.KindInternal:
			t.Errorf("%s: ValidateEvent = %v, want kind internal (a programming error)", c.name, err)
		}
	}
}

func TestValidateEvent_ErrorNeverCarriesThePayload(t *testing.T) {
	t.Parallel()
	f := valid()
	f.Payload = []byte(`{"secret_text":null}`)
	err := domain.ValidateEvent(f, 1024)
	if err == nil || strings.Contains(err.Error(), "secret_text") {
		t.Fatalf("ValidateEvent = %v, want an error without payload content", err)
	}
}

func TestActorKinds(t *testing.T) {
	t.Parallel()
	got := strings.Join(domain.ActorKinds(), ",")
	if got != "user,agent,app,system" {
		t.Fatalf("ActorKinds = %s", got)
	}
}

// TestCheckPayload_NumericLimits: numbers jsonb (PostgreSQL numeric) refuses are refused at the
// append, not by the insert in the sync hook: at most 16383 digits after the point and 131072
// before it.
func TestCheckPayload_NumericLimits(t *testing.T) {
	t.Parallel()
	const limit = 1 << 20
	cases := []struct {
		name  string
		body  string
		valid bool
	}{
		{"16383 fractional digits", `{"a":0.` + strings.Repeat("1", 16383) + `}`, true},
		{"16384 fractional digits", `{"a":0.` + strings.Repeat("1", 16384) + `}`, false},
		{"zeros then a digit", `{"a":[0.` + strings.Repeat("0", 65000) + `1]}`, false},
		{"131072 integer digits", `{"a":-1` + strings.Repeat("0", 131071) + `}`, true},
		{"131073 integer digits", `{"a":1` + strings.Repeat("0", 131072) + `}`, false},
	}
	for _, c := range cases {
		err := domain.CheckPayload([]byte(c.body), limit)
		if (err == nil) != c.valid {
			t.Errorf("%s: CheckPayload = %v, want valid %v", c.name, err, c.valid)
		}
	}
}

// TestCheckPayload_DeepNestingNamesTheRule: nesting beyond what the decoder reads is refused with
// a message that names it, not as invalid JSON.
func TestCheckPayload_DeepNestingNamesTheRule(t *testing.T) {
	t.Parallel()
	body := `{"a":` + strings.Repeat("[", 20000) + strings.Repeat("]", 20000) + `}`
	err := domain.CheckPayload([]byte(body), 1<<20)
	if err == nil || !strings.Contains(err.Error(), "nested") {
		t.Fatalf("CheckPayload = %v, want an error about nesting", err)
	}
}
