// SPDX-License-Identifier: Apache-2.0

package rest_test

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"regexp"
	"testing"

	"go.yaml.in/yaml/v3"

	"github.com/OWNER/bothy/services/server/api/rest"
)

// Compile-time references: these schemas are referenced by no enabled operation (events,
// gateway frames, problems), so they exist only because of skip-prune.
var (
	_ rest.EventEnvelope
	_ rest.EventPage
	_ rest.Problem
	_ rest.GatewayHello
	_ rest.GatewayIdentify
	_ rest.GatewayReady
)

// listEventsExample returns the response example of listEvents from docs/api/openapi.yaml: an
// event page whose message.created event the API lint (G1) validates against the schemas.
func listEventsExample(t *testing.T) map[string]any {
	t.Helper()
	data, err := os.ReadFile("../../../../docs/api/openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var spec struct {
		Paths map[string]map[string]struct {
			OperationID string `yaml:"operationId"`
			Responses   map[string]struct {
				Content map[string]struct {
					Examples map[string]struct {
						Value map[string]any `yaml:"value"`
					} `yaml:"examples"`
				} `yaml:"content"`
			} `yaml:"responses"`
		} `yaml:"paths"`
	}
	if err := yaml.Unmarshal(data, &spec); err != nil {
		t.Fatal(err)
	}
	for _, item := range spec.Paths {
		for _, op := range item {
			if op.OperationID == "listEvents" {
				if v := op.Responses["200"].Content["application/json"].Examples["default"].Value; v != nil {
					return v
				}
			}
		}
	}
	t.Fatal("openapi.yaml has no example of listEvents")
	return nil
}

// TestEventUnion_UnknownTypeDecodes: a page with a known and an unknown event type decodes;
// clients skip unknown types and still advance their cursor (06 "Basic rules", "Unknown values").
// The known event is the schema-valid example of listEvents, the unknown one is added to it.
func TestEventUnion_UnknownTypeDecodes(t *testing.T) {
	example := listEventsExample(t)
	events, ok := example["events"].([]any)
	if !ok || len(events) == 0 {
		t.Fatalf("example events = %v", example["events"])
	}
	known, ok := events[0].(map[string]any)
	if !ok || known["type"] != "message.created" {
		t.Fatalf("first example event = %v, want message.created", events[0])
	}
	example["events"] = append(events, map[string]any{
		"type": "future.example", "id": "0190d6a8-0000-7000-8000-000000000103", "schema_version": 1,
		"occurred_at": "2026-10-11T08:00:01.000Z", "scope": map[string]any{},
		"resource": map[string]any{"kind": "future"}, "payload": map[string]any{"something": "new"},
	})
	body, err := json.Marshal(example)
	if err != nil {
		t.Fatal(err)
	}

	var page rest.EventPage
	if err := json.Unmarshal(body, &page); err != nil {
		t.Fatalf("a page with an unknown event type does not decode: %v", err)
	}
	if len(page.Events) != 2 {
		t.Fatalf("page has %d events, want 2", len(page.Events))
	}

	value, err := page.Events[0].ValueByDiscriminator()
	if err != nil {
		t.Fatalf("known event: %v", err)
	}
	created, ok := value.(rest.MessageCreatedEvent)
	if !ok {
		t.Fatalf("known event resolved to %T, want rest.MessageCreatedEvent", value)
	}
	if got, want := created.Payload.Message.Id.String(), "0192f4a0-7b1c-7353-8d8c-dea659467151"; got != want {
		t.Errorf("message id = %s, want %s", got, want)
	}

	if v, err := page.Events[1].ValueByDiscriminator(); err == nil {
		t.Errorf("unknown event resolved to %T without error", v)
	}
	if typ, err := page.Events[1].Discriminator(); err != nil || typ != "future.example" {
		t.Errorf("Discriminator() = %q, %v; want future.example", typ, err)
	}
}

// identifier is the form of a schema name that oapi-codegen keeps as the Go type name.
var identifier = regexp.MustCompile(`^[A-Z][A-Za-z0-9]*$`)

// TestSchemasGenerated proves skip-prune for the whole contract: every schema under
// components.schemas of docs/api/openapi.yaml has a Go type of the same name, including events
// and gateway frames that no enabled operation references.
func TestSchemasGenerated(t *testing.T) {
	data, err := os.ReadFile("../../../../docs/api/openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var spec struct {
		Components struct {
			Schemas map[string]any `yaml:"schemas"`
		} `yaml:"components"`
	}
	if err := yaml.Unmarshal(data, &spec); err != nil {
		t.Fatal(err)
	}
	// The generator splits the package into models.gen.go and rest.gen.go (INT-W14/4).
	declared := map[string]bool{}
	for _, name := range []string{"models.gen.go", "rest.gen.go"} {
		file, err := parser.ParseFile(token.NewFileSet(), name, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range file.Decls {
			if gen, ok := decl.(*ast.GenDecl); ok && gen.Tok == token.TYPE {
				for _, s := range gen.Specs {
					declared[s.(*ast.TypeSpec).Name.Name] = true
				}
			}
		}
	}
	if len(spec.Components.Schemas) == 0 {
		t.Fatal("openapi.yaml has no schemas")
	}
	for name := range spec.Components.Schemas {
		if !identifier.MatchString(name) {
			t.Errorf("schema %q is not a Go identifier in PascalCase; this test cannot map it", name)
			continue
		}
		if !declared[name] {
			t.Errorf("schema %s has no Go type in models.gen.go or rest.gen.go", name)
		}
	}
}
