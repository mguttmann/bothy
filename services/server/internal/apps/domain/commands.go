// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"reflect"
	"slices"
	"strings"
)

// The commands of a manifest across the lifecycle of the app (docs/spec/11-extensions.md sections
// 4.1 to 4.3, brief M2-25 design note 11): a manifest is independent of the instance and names no
// target, so its commands are instance-wide; an upgrade applies the difference by name.

// CommandDiff is the difference of the commands of two manifests by lower-case name.
type CommandDiff struct {
	// Added are the commands only the new manifest has; Changed the commands both have with
	// another description, options, required_level or control (in the form of the new manifest).
	Added, Changed []Command
	// Removed are the lower-case names only the old manifest has; Unchanged the names of the
	// commands both have unchanged. Both sorted.
	Removed, Unchanged []string
}

// DiffCommands compares the commands of old and new by name without regard to case (brief M2-25
// design note 11): added, changed in description, options, required_level or control, removed and
// unchanged. Added and Changed keep the order of new.
//
// Example:
//
//	d := domain.DiffCommands(old, next)
//	for _, c := range append(d.Added, d.Changed...) { /* upsert */ }
func DiffCommands(old, new Manifest) CommandDiff {
	before := make(map[string]Command, len(old.Commands))
	for _, c := range old.Commands {
		before[strings.ToLower(c.Name)] = c
	}
	var d CommandDiff
	seen := make(map[string]bool, len(new.Commands))
	for _, c := range new.Commands {
		name := strings.ToLower(c.Name)
		seen[name] = true
		prev, ok := before[name]
		switch {
		case !ok:
			d.Added = append(d.Added, c)
		case SameCommand(prev, c):
			d.Unchanged = append(d.Unchanged, name)
		default:
			d.Changed = append(d.Changed, c)
		}
	}
	for name := range before {
		if !seen[name] {
			d.Removed = append(d.Removed, name)
		}
	}
	slices.Sort(d.Removed)
	slices.Sort(d.Unchanged)
	return d
}

// SameCommand reports whether a and b define the same command: the name compared without regard
// to case, and the description, options, required_level (absent means trigger) and control
// (absent means false) equal.
func SameCommand(a, b Command) bool {
	return strings.EqualFold(a.Name, b.Name) && a.Description == b.Description && level(a) == level(b) && control(a) == control(b) &&
		sameOptions(a.Options, b.Options)
}

// level is required_level with the default of 09 section 10.2.
func level(c Command) string {
	if c.RequiredLevel == "" {
		return "trigger"
	}
	return c.RequiredLevel
}

// control is control with its default false.
func control(c Command) bool { return c.Control != nil && *c.Control }

// sameOptions compares two option lists; an absent autocomplete is false and an absent choices
// list equals an empty one.
func sameOptions(a, b []CommandOption) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		x, y := a[i], b[i]
		if x.Name != y.Name || x.Description != y.Description || x.Type != y.Type || x.Required != y.Required ||
			(x.Autocomplete != nil && *x.Autocomplete) != (y.Autocomplete != nil && *y.Autocomplete) ||
			len(x.Choices) != len(y.Choices) || len(x.Choices) > 0 && !reflect.DeepEqual(x.Choices, y.Choices) {
			return false
		}
	}
	return true
}
