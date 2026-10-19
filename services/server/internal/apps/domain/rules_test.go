// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/apps/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
)

// catalogueScopes are the scopes of 05 section 2.6 (schema AppScope).
var catalogueScopes = []string{"messages:read", "messages:write", "files:read", "files:write", "commands:write",
	"components:write", "users:read", "events:subscribe", "costs:reconcile"}

// eventScope is a fake of authz.AppScopes.EventScope over a few rows of the catalogue.
func eventScope(t string) (string, bool) {
	switch t {
	case "message.created":
		return "messages:read", true
	case "instance.updated":
		return "", true
	case "command.invoked", "interaction.created":
		return "events:subscribe", true
	}
	return "", false
}

// testEnv is the environment of the rule tests: the hosts of the example and hooks.example.com
// allowed, the spec version 1.4.2.
func testEnv() domain.Env {
	return domain.Env{
		MaxBytes: 65536, HandleMaxChars: 32, DisplayNameMaxChars: 64, MaxCommands: 3, CommandDescriptionMaxChars: 100,
		SpecVersion: "1.4.2", AllowedHosts: []string{"bridge.example.com", "Hooks.Example.com"},
		KnownScope: func(s string) bool { return slices.Contains(catalogueScopes, s) }, EventScope: eventScope,
	}
}

// exampleManifest is the example of 11 section 3.3 as M2-24 changed it, as a JSON object.
func exampleManifest() map[string]any {
	return map[string]any{
		"schema_version":  1,
		"slug":            "repo-bridge",
		"name":            "Repo Bridge",
		"description":     "Brings events of a code hosting service into a forum.",
		"publisher":       "Example Org",
		"version":         "1.2.0",
		"homepage_url":    "https://bridge.example.com",
		"min_api_version": "1.0",
		"scopes": map[string]any{
			"required": []any{"messages:write", "commands:write", "components:write"},
			"optional": []any{"messages:read", "events:subscribe"},
		},
		"events": []any{"message.created"},
		"delivery": map[string]any{
			"mode": "webhook", "events_url": "https://bridge.example.com/hooks/events",
			"interactions_url": "https://bridge.example.com/hooks/interactions",
		},
		"commands": []any{map[string]any{
			"name": "issue", "description": "Create an issue", "required_level": "write",
			"options": []any{map[string]any{"name": "title", "description": "Title of the issue", "type": "string", "required": true}},
		}},
		"components":       []any{"button", "form"},
		"requested_grants": []any{map[string]any{"target_kind": "channel", "level": "write", "capabilities": []any{}, "purpose": "Posts per issue"}},
		"data_egress":      "Messages of the granted channels go to bridge.example.com when messages:read is granted.",
	}
}

// encode returns the JSON of m and its parse.
func encode(t *testing.T, m map[string]any) ([]byte, domain.Manifest) {
	t.Helper()
	raw, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := domain.ParseManifest(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return raw, parsed
}

// TestValidateManifest_Example: the example of 11 section 3.3 with its hosts allowed and
// min_api_version at the server's version passes every rule.
func TestValidateManifest_Example(t *testing.T) {
	t.Parallel()
	m := exampleManifest()
	m["min_api_version"] = "1.4"
	raw, parsed := encode(t, m)
	if fe := domain.ValidateManifest(parsed, raw, testEnv()); len(fe) != 0 {
		t.Fatalf("errors %+v", fe)
	}
}

// at returns the object at the path of keys and indexes inside m.
func at(m map[string]any, path ...any) map[string]any {
	var cur any = m
	for _, p := range path {
		switch k := p.(type) {
		case string:
			cur = cur.(map[string]any)[k]
		case int:
			cur = cur.([]any)[k]
		}
	}
	return cur.(map[string]any)
}

// TestValidateManifest_Rules has one row per rule of M2-24 design notes 4 and 5: each asserts
// the pointer and the code of the error and that its detail does not repeat the submitted value.
func TestValidateManifest_Rules(t *testing.T) {
	t.Parallel()
	long := strings.Repeat("a", 33)
	cases := []struct {
		name, pointer, code, value string
		mutate                     func(m map[string]any)
		env                        func(e *domain.Env)
	}{
		{name: "size above app.manifest_max_bytes", pointer: "", code: "too_large",
			mutate: func(map[string]any) {}, env: func(e *domain.Env) { e.MaxBytes = 100 }},
		{name: "slug above principal.handle_max_chars", pointer: "/slug", code: "too_long", value: long,
			mutate: func(m map[string]any) { m["slug"] = long }},
		{name: "name with a bidi override (SEC-T07)", pointer: "/name", code: "invalid", value: "Repo\u202eBridge",
			mutate: func(m map[string]any) { m["name"] = "Repo\u202eBridge" }},
		{name: "invalid SemVer", pointer: "/version", code: "invalid", value: "1.02.0",
			mutate: func(m map[string]any) { m["version"] = "1.02.0" }},
		{name: "min_api_version above the spec version", pointer: "/min_api_version", code: "invalid", value: "1.5",
			mutate: func(m map[string]any) { m["min_api_version"] = "1.5" }},
		{name: "scope in both lists", pointer: "/scopes/optional/0", code: "invalid",
			mutate: func(m map[string]any) { at(m, "scopes")["optional"] = []any{"messages:write", "events:subscribe"} }},
		{name: "event access.changed", pointer: "/events/0", code: "invalid", value: "access.changed",
			mutate: func(m map[string]any) { m["events"] = []any{"access.changed"} }},
		{name: "event draft.updated", pointer: "/events/0", code: "invalid", value: "draft.updated",
			mutate: func(m map[string]any) { m["events"] = []any{"draft.updated"} }},
		{name: "event agent.invocation", pointer: "/events/0", code: "invalid", value: "agent.invocation",
			mutate: func(m map[string]any) { m["events"] = []any{"agent.invocation"} }},
		{name: "event grant.changed", pointer: "/events/0", code: "invalid", value: "grant.changed",
			mutate: func(m map[string]any) { m["events"] = []any{"grant.changed"} }},
		{name: "message.created without messages:read", pointer: "/events/0", code: "invalid",
			mutate: func(m map[string]any) { at(m, "scopes")["optional"] = []any{"events:subscribe"} }},
		{name: "events without events:subscribe", pointer: "/events", code: "invalid",
			mutate: func(m map[string]any) { at(m, "scopes")["optional"] = []any{"messages:read"} }},
		{name: "events without delivery", pointer: "/delivery", code: "required",
			mutate: func(m map[string]any) { delete(m, "delivery"); delete(m, "commands"); delete(m, "components") }},
		{name: "webhook mode without events_url", pointer: "/delivery/events_url", code: "required",
			mutate: func(m map[string]any) { delete(at(m, "delivery"), "events_url") }},
		{name: "gateway mode with events_url", pointer: "/delivery/events_url", code: "invalid",
			mutate: func(m map[string]any) { at(m, "delivery")["mode"] = "gateway" }},
		{name: "commands without interactions_url in webhook mode", pointer: "/delivery/interactions_url", code: "required",
			mutate: func(m map[string]any) { delete(at(m, "delivery"), "interactions_url") }},
		{name: "events without data_egress", pointer: "/data_egress", code: "required",
			mutate: func(m map[string]any) { delete(m, "data_egress") }},
		{name: "http address", pointer: "/delivery/events_url", code: "invalid", value: "http://bridge.example.com/e",
			mutate: func(m map[string]any) { at(m, "delivery")["events_url"] = "http://bridge.example.com/e" }},
		{name: "address with user information", pointer: "/delivery/events_url", code: "invalid", value: "https://alex@bridge.example.com/e",
			mutate: func(m map[string]any) { at(m, "delivery")["events_url"] = "https://alex@bridge.example.com/e" }},
		{name: "host not allowed", pointer: "/delivery/events_url", code: "webhook_host_not_allowed", value: "hooks.other.example.com",
			mutate: func(m map[string]any) { at(m, "delivery")["events_url"] = "https://hooks.other.example.com/e" }},
		{name: "homepage_url with http", pointer: "/homepage_url", code: "invalid", value: "http://bridge.example.com",
			mutate: func(m map[string]any) { m["homepage_url"] = "http://bridge.example.com" }},
		{name: "requested grant with manage", pointer: "/requested_grants/0/capabilities/0", code: "invalid",
			mutate: func(m map[string]any) { at(m, "requested_grants", 0)["capabilities"] = []any{"manage"} }},
		{name: "requested grant with export", pointer: "/requested_grants/0/capabilities/0", code: "invalid",
			mutate: func(m map[string]any) { at(m, "requested_grants", 0)["capabilities"] = []any{"export"} }},
		{name: "requested grant with approve_actions", pointer: "/requested_grants/0/capabilities/0", code: "invalid",
			mutate: func(m map[string]any) { at(m, "requested_grants", 0)["capabilities"] = []any{"approve_actions"} }},
		{name: "requested grant with level trigger", pointer: "/requested_grants/0/level", code: "invalid",
			mutate: func(m map[string]any) { at(m, "requested_grants", 0)["level"] = "trigger" }},
		{name: "requested grant on a host", pointer: "/requested_grants/0/target_kind", code: "invalid",
			mutate: func(m map[string]any) { at(m, "requested_grants", 0)["target_kind"] = "host" }},
		{name: "reserved command stop", pointer: "/commands/0/name", code: "command_name_reserved",
			mutate: func(m map[string]any) { at(m, "commands", 0)["name"] = "stop" }},
		{name: "duplicate command names in other case", pointer: "/commands/1/name", code: "invalid",
			mutate: func(m map[string]any) {
				m["commands"] = append(m["commands"].([]any), map[string]any{"name": "Issue", "description": "Again"})
			}},
		{name: "more commands than command.max_per_principal", pointer: "/commands", code: "too_many_items",
			mutate: func(m map[string]any) {
				var cmds []any
				for _, n := range []string{"a", "b", "c", "d"} {
					cmds = append(cmds, map[string]any{"name": n, "description": "Command " + n})
				}
				m["commands"] = cmds
			}},
		{name: "command with scope_kind channel", pointer: "/commands/0/scope_kind", code: "invalid",
			mutate: func(m map[string]any) { at(m, "commands", 0)["scope_kind"] = "channel" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			m := exampleManifest()
			tc.mutate(m)
			raw, parsed := encode(t, m)
			env := testEnv()
			if tc.env != nil {
				tc.env(&env)
			}
			fe := domain.ValidateManifest(parsed, raw, env)
			if !hasError(fe, tc.pointer, tc.code) {
				t.Fatalf("want %s at %q, got %+v", tc.code, tc.pointer, fe)
			}
			for _, f := range fe {
				if tc.value != "" && strings.Contains(f.Detail, tc.value) {
					t.Errorf("detail %q repeats the submitted value", f.Detail)
				}
				if !strings.Contains(f.Detail, "(") {
					t.Errorf("detail %q names no rule", f.Detail)
				}
			}
		})
	}
}

// hasError reports an error at pointer with code.
func hasError(fe []errs.FieldError, pointer, code string) bool {
	return slices.ContainsFunc(fe, func(f errs.FieldError) bool { return f.Pointer == pointer && f.Code == code })
}

// TestValidateManifest_HostErrorsOnly: a manifest whose only failure is a host answers
// webhook_host_not_allowed; a mixed list does not.
func TestValidateManifest_HostErrorsOnly(t *testing.T) {
	t.Parallel()
	m := exampleManifest()
	at(m, "delivery")["events_url"] = "https://hooks.other.example.com/e"
	raw, parsed := encode(t, m)
	fe := domain.ValidateManifest(parsed, raw, testEnv())
	if !domain.HostErrorsOnly(fe) {
		t.Errorf("host error only: %+v", fe)
	}
	m["version"] = "x"
	raw, parsed = encode(t, m)
	if domain.HostErrorsOnly(domain.ValidateManifest(parsed, raw, testEnv())) || domain.HostErrorsOnly(nil) {
		t.Error("mixed or empty list counts as host errors only")
	}
}

// TestParseManifest_UnknownMember: an unknown member and trailing data are refused.
func TestParseManifest_UnknownMember(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{`{"slug":"a","webhooks":[]}`, `{"slug":"a"} {}`, `[]`} {
		if _, err := domain.ParseManifest([]byte(raw)); codeOf(err) != errs.CodeValidationFailed {
			t.Errorf("%s: %v", raw, err)
		}
	}
}

// TestSameContent: member order and white space do not count.
func TestSameContent(t *testing.T) {
	t.Parallel()
	if !domain.SameContent([]byte(`{"a":1,"b":[1,2]}`), []byte(`{ "b":[1,2], "a":1.0 }`)) {
		t.Error("reordered document differs")
	}
	if domain.SameContent([]byte(`{"a":1}`), []byte(`{"a":2}`)) || domain.SameContent([]byte(`{`), []byte(`{`)) {
		t.Error("different or broken documents are equal")
	}
}

// codeOf returns the code of a kernel/errs error, "" for nil or another error.
func codeOf(err error) errs.Code {
	if e, ok := errs.As(err); ok {
		return e.Code
	}
	return ""
}

// TestValidateManifest_StorableText (red team M2-24): the escape of NUL or of an unpaired
// surrogate anywhere in the manifest is invalid at the root, because the database refuses it; an
// escaped backslash before u and a surrogate pair are text.
func TestValidateManifest_StorableText(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		desc string
		ok   bool
	}{
		{`Brings events.`, true},
		{`Brings \\u0000 events 😀 \n.`, true},
		{`Brings \u0000 events.`, false},
		{`Brings \ud800 events.`, false},
		{`Brings \ud800\n events.`, false},
		{`Brings \ud800\ud800 events.`, false},
		{`Brings \udfff events.`, false},
		{`Brings \ud800`, false},
	} {
		raw, m := encode(t, exampleManifest())
		raw = []byte(strings.Replace(string(raw), "Brings events of a code hosting service into a forum.", c.desc, 1))
		fe := domain.ValidateManifest(m, raw, testEnv())
		bad := slices.ContainsFunc(fe, func(e errs.FieldError) bool { return e.Pointer == "" && e.Code == domain.CodeInvalid })
		if bad == c.ok {
			t.Errorf("%q: errors %v", c.desc, fe)
		}
	}
}
