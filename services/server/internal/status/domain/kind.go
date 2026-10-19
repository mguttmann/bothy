// SPDX-License-Identifier: Apache-2.0

package domain

// Kind is a backup kind of docs/spec/16-betrieb.md 8.1, the value of backup_runs.kind. The
// restore test (M7-03) is not a backup kind of this package.
type Kind string

// The backup kinds, in the order DueKinds and the scheduler report them.
const (
	KindPGFull  Kind = "pg_full"
	KindPGDiff  Kind = "pg_diff"
	KindLogical Kind = "logical"
	KindObjects Kind = "objects"
)

// Kinds returns the four backup kinds in a fixed order: pg_full, pg_diff, logical, objects.
func Kinds() []Kind { return []Kind{KindPGFull, KindPGDiff, KindLogical, KindObjects} }

// ParseKind returns the kind named s and true, or false for any other string.
func ParseKind(s string) (Kind, bool) {
	for _, k := range Kinds() {
		if string(k) == s {
			return k, true
		}
	}
	return "", false
}

// Physical reports whether k is a pgBackRest backup (pg_full or pg_diff). Both share one slot:
// pgBackRest holds one backup lock per stanza, and a full and a differential due together run
// only the full (brief M7-02 design note 1).
func (k Kind) Physical() bool { return k == KindPGFull || k == KindPGDiff }

// Slot names the slot of k for "at most one run at a time": physical for pg_full and pg_diff,
// the kind itself for the others.
func (k Kind) Slot() string {
	if k.Physical() {
		return "physical"
	}
	return string(k)
}
