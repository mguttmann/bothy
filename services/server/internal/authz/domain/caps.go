// SPDX-License-Identifier: Apache-2.0

package domain

import "slices"

// Capability is one additional right of a grant (07 section 2.4).
type Capability uint8

const (
	// CapApproveActions allows operating approval cards; needs trigger.
	CapApproveActions Capability = iota
	// CapUpload allows uploading and attaching files; needs write.
	CapUpload
	// CapExport allows exporting a channel history; needs read.
	CapExport
	// CapManage allows administering the target and delegating within one's own frame.
	CapManage
	// CapTerminal allows opening a terminal; only on a host target, only for people and groups.
	CapTerminal
)

// capNames are the values of grants.capabilities, indexed by Capability.
var capNames = [...]string{
	CapApproveActions: "approve_actions",
	CapUpload:         "upload",
	CapExport:         "export",
	CapManage:         "manage", //nolint:goconst // the count includes the generated catalogue of M1-25 (docs/DECISIONS.md M1-27/10)
	CapTerminal:       "terminal",
}

// allCaps is the set of every defined capability.
const allCaps Caps = 1<<len(capNames) - 1

// String returns the column value of c, or "" for a value out of range.
func (c Capability) String() string {
	if int(c) >= len(capNames) {
		return ""
	}
	return capNames[c]
}

// Caps is a set of capabilities as a bit set; the zero value is the empty set. Sets compare with
// ==, so two sets with the same members are equal.
type Caps uint8

// CapsOf returns the set of the given capabilities.
func CapsOf(list ...Capability) Caps {
	var c Caps
	for _, x := range list {
		c |= 1 << x
	}
	return c
}

// ParseCaps returns the set of the given column or API values. Duplicates collapse; order does
// not matter.
//
// Errors: validation_failed on /capabilities when a name is not one of approve_actions, upload,
// export, manage and terminal.
//
// Example: ParseCaps([]string{"upload", "manage", "upload"}) has two members.
func ParseCaps(names []string) (Caps, error) {
	var c Caps
	for _, n := range names {
		i := slices.Index(capNames[:], n)
		if i < 0 {
			return 0, fieldError(pointerCapabilities, fieldInvalid,
				"capabilities must be from approve_actions, upload, export, manage and terminal")
		}
		c |= 1 << i
	}
	return c, nil
}

// Has reports whether x is in c.
func (c Caps) Has(x Capability) bool { return c&(1<<x) != 0 }

// Union returns the members of c or o.
func (c Caps) Union(o Caps) Caps { return c | o }

// Intersect returns the members of both c and o.
func (c Caps) Intersect(o Caps) Caps { return c & o }

// SubsetOf reports whether every member of c is in o; the empty set is a subset of every set.
func (c Caps) SubsetOf(o Caps) bool { return c&^o == 0 }

// valid reports whether c holds only defined capabilities.
func (c Caps) valid() bool { return c.SubsetOf(allCaps) }

// Names returns the column values of the members sorted by name, for the text[] column
// grants.capabilities; the empty set gives an empty, non-nil slice, so the column receives
// '{}' and not NULL. Bits outside the defined capabilities are left out.
func (c Caps) Names() []string {
	out := make([]string, 0, len(capNames))
	for i, n := range capNames {
		if c.Has(Capability(i)) {
			out = append(out, n)
		}
	}
	slices.Sort(out)
	return out
}
