// SPDX-License-Identifier: Apache-2.0

package main

import (
	"slices"
	"strings"
)

// rightSystem is the right of system actions (05 rule 10): only the server runs them, so no
// operation may name one.
const rightSystem = "system"

// catalogAction is the part of an action in docs/spec/actions.yaml that the route table needs.
// The schema itself (every field, every rule) is held by `make catalog-check` (tools/catalog);
// the generator reads only these three fields and ignores the rest.
type catalogAction struct {
	ID    string // action id, for example thread.read
	Right string // column "Recht" of chapter 05, verbatim
	Class string // rate-limit class, or "none" for system actions and internal checks
}

// catalogFile is the decoded subset of actions.yaml.
type catalogFile struct {
	Version int `yaml:"version"`
	Actions []struct {
		ID      string `yaml:"id"`
		Right   string `yaml:"right"`
		RLClass string `yaml:"rl_class"`
	} `yaml:"actions"`
}

// readCatalog loads the actions of the catalogue at path, sorted by ID. Problems (unreadable
// file, unknown version, missing or duplicate IDs) go to p; the result holds the usable entries.
func readCatalog(path string, p *problemsError) []catalogAction {
	doc, _, err := readYAML(path)
	if err != nil {
		p.add(path, "%v", err)
		return nil
	}
	var file catalogFile
	if err := doc.Decode(&file); err != nil {
		p.add(path, "%v", err)
		return nil
	}
	if file.Version != 1 {
		p.add(path, "version %d is not supported (want 1)", file.Version)
		return nil
	}
	seen := map[string]bool{}
	var out []catalogAction
	for i, a := range file.Actions {
		switch {
		case a.ID == "":
			p.add(path, "actions[%d]: missing id", i)
		case seen[a.ID]:
			p.add(path, "action %s is listed more than once", a.ID)
		default:
			seen[a.ID] = true
			out = append(out, catalogAction{ID: a.ID, Right: a.Right, Class: a.RLClass})
		}
	}
	if len(file.Actions) == 0 {
		p.add(path, "no actions")
	}
	slices.SortFunc(out, func(a, b catalogAction) int { return strings.Compare(a.ID, b.ID) })
	return out
}
