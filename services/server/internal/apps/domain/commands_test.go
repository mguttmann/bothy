// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/apps/domain"
)

// manifestWith returns a manifest with the commands cmds.
func manifestWith(cmds ...domain.Command) domain.Manifest { return domain.Manifest{Commands: cmds} }

// TestDiffCommands (M2-25 design note 11): added, changed in each field, unchanged and removed,
// names compared without regard to case.
func TestDiffCommands(t *testing.T) {
	t.Parallel()
	yes := true
	base := domain.Command{Name: "ticket", Description: "Open a ticket", RequiredLevel: "trigger",
		Options: []domain.CommandOption{{Name: "title", Description: "Title", Type: "string", Required: true}}}
	changes := map[string]func(c *domain.Command){
		"description":    func(c *domain.Command) { c.Description = "Open a support ticket" },
		"required_level": func(c *domain.Command) { c.RequiredLevel = "write" },
		"control":        func(c *domain.Command) { c.Control = &yes },
		"option type": func(c *domain.Command) {
			c.Options = []domain.CommandOption{{Name: "title", Description: "Title", Type: "integer", Required: true}}
		},
		"option added": func(c *domain.Command) {
			c.Options = append(slices.Clone(c.Options), domain.CommandOption{Name: "queue", Description: "Queue", Type: "string"})
		},
		"autocomplete": func(c *domain.Command) {
			c.Options = []domain.CommandOption{{Name: "title", Description: "Title", Type: "string", Required: true, Autocomplete: &yes}}
		},
	}
	for name, mod := range changes {
		next := base
		next.Options = slices.Clone(base.Options)
		mod(&next)
		d := domain.DiffCommands(manifestWith(base), manifestWith(next))
		if len(d.Changed) != 1 || len(d.Added) != 0 || len(d.Removed) != 0 || len(d.Unchanged) != 0 {
			t.Errorf("%s: %+v", name, d)
		}
	}
	upper := base
	upper.Name = "TICKET"
	// The default level and an explicit trigger are the same command; the name case does not count.
	implicit := upper
	implicit.RequiredLevel = ""
	d := domain.DiffCommands(manifestWith(base), manifestWith(implicit))
	if len(d.Unchanged) != 1 || d.Unchanged[0] != "ticket" || len(d.Changed)+len(d.Added)+len(d.Removed) != 0 {
		t.Fatalf("unchanged: %+v", d)
	}
	link := domain.Command{Name: "Link", Description: "Link an issue"}
	d = domain.DiffCommands(manifestWith(base, link), manifestWith(domain.Command{Name: "archive", Description: "Archive"}, upper))
	if len(d.Added) != 1 || d.Added[0].Name != "archive" || !slices.Equal(d.Removed, []string{"link"}) || !slices.Equal(d.Unchanged, []string{"ticket"}) {
		t.Fatalf("added and removed: %+v", d)
	}
}

// TestManifestCommandRules (M2-25 design note 11): a manifest command with scope_id, with
// scope_kind channel, a reserved name, more commands than command.max_per_principal and a
// description above command.description_max_chars each give validation_failed with its pointer.
func TestManifestCommandRules(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, pointer, code string
		mutate              func(m map[string]any)
	}{
		{"scope_id", "/commands/0/scope_id", "invalid", func(m map[string]any) {
			at(m, "commands", 0)["scope_id"] = "0192f3a4-0000-7000-8000-000000000001"
		}},
		{"scope_kind channel", "/commands/0/scope_kind", "invalid", func(m map[string]any) { at(m, "commands", 0)["scope_kind"] = "channel" }},
		{"reserved name", "/commands/0/name", "command_name_reserved", func(m map[string]any) { at(m, "commands", 0)["name"] = "stop" }},
		{"too many", "/commands", "too_many_items", func(m map[string]any) {
			list := []any{}
			for _, n := range []string{"one", "two", "three", "four"} {
				list = append(list, map[string]any{"name": n, "description": "A command"})
			}
			m["commands"] = list
		}},
		{"description too long", "/commands/0/description", "too_long", func(m map[string]any) {
			at(m, "commands", 0)["description"] = strings.Repeat("d", 101)
		}},
	}
	for _, c := range cases {
		m := exampleManifest()
		m["min_api_version"] = "1.4"
		c.mutate(m)
		raw, parsed := encode(t, m)
		fe := domain.ValidateManifest(parsed, raw, testEnv())
		found := false
		for _, f := range fe {
			found = found || f.Pointer == c.pointer && f.Code == c.code
		}
		if !found {
			t.Errorf("%s: %+v, want %s %s", c.name, fe, c.pointer, c.code)
		}
	}
	// scope_kind instance stays allowed: it is the scope every manifest command has.
	m := exampleManifest()
	m["min_api_version"] = "1.4"
	at(m, "commands", 0)["scope_kind"] = "instance"
	raw, parsed := encode(t, m)
	if fe := domain.ValidateManifest(parsed, raw, testEnv()); len(fe) != 0 {
		t.Fatalf("instance: %+v", fe)
	}
}
