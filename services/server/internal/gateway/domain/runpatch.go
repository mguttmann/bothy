// SPDX-License-Identifier: Apache-2.0

package domain

import "strings"

// The coalescing of live-box patches (M2-10 design note 9; 04 section 11.3 point 3): the changes
// of one run inside sync.run_patch_coalesce_interval form one chain from the oldest base_version
// to the newest version, and a later replace on a path replaces the earlier op on exactly that
// path. The gateway owns these types so the merge rule needs nothing of runs.

// PatchOp is one normalized RFC 6902 operation of run.updated: Op add, replace or remove, Path a
// JSON Pointer into run_document.v1, Value the encoded value (nil for remove).
type PatchOp struct {
	Op    string
	Path  string
	Value []byte
}

// The operations the merge rule distinguishes.
const (
	patchAdd     = "add"
	patchReplace = "replace"
)

// MergeOps appends next to ops with the merge rule of 04 section 11.3 point 3 and returns the
// result: a replace in next removes an earlier replace or add on exactly its path, and a removed
// add turns the later op into an add at the later position (the member was absent before the
// chain); ops on other paths keep their order. ops is not modified.
//
// Applying the result with RFC 6902 to the document at the chain's base gives the same document
// as applying ops and then next, because the server normalizes every op against the state just
// before it (a replace always targets a present member, and nothing between the two ops can
// remove that member without a remove op, which a replace never follows).
//
// Example:
//
//	MergeOps([]PatchOp{{Op: "add", Path: "/model", Value: a}}, []PatchOp{{Op: "replace", Path: "/model", Value: b}})
//	// []PatchOp{{Op: "add", Path: "/model", Value: b}}
func MergeOps(ops, next []PatchOp) []PatchOp {
	out := make([]PatchOp, len(ops), len(ops)+len(next))
	copy(out, ops)
	for _, n := range next {
		if n.Op == patchReplace {
			out, n.Op = dropEarlier(out, n.Path)
		}
		out = append(out, n)
	}
	return out
}

// dropEarlier removes the latest add or replace on exactly path from out and returns out with the
// op the later replace becomes: add when the removed op was an add (the member was absent before
// the chain), replace otherwise. The search ends at an op on an ancestor of path (a step removed
// or added: the member before it belongs to another life of the object) and at a remove on path.
func dropEarlier(out []PatchOp, path string) ([]PatchOp, string) {
	for i := len(out) - 1; i >= 0; i-- {
		p := out[i]
		if strings.HasPrefix(path, p.Path+"/") {
			break
		}
		if p.Path != path {
			continue
		}
		if p.Op != patchAdd && p.Op != patchReplace {
			break
		}
		return append(out[:i], out[i+1:]...), p.Op
	}
	return out, patchReplace
}

// Chain is the gathered change of one run since the last window end: the version the chain
// starts at (Base), the newest version (Version), the merged ops and whether the chain can only go
// out as the whole document (Snapshot: a change published without ops, or a version step without
// ops in between). A Chain with Base == Version and no ops is empty.
type Chain struct {
	Base, Version int64
	Ops           []PatchOp
	Snapshot      bool
}

// Add folds the change base -> version with ops into the chain (Design note 9; ruling
// D-M2-10-9) and reports whether it was taken. A change whose base equals the chain's version
// extends it with MergeOps; a change whose base is above the chain's version (a version step
// without patch, for example a state transition) discards the gathered ops and restarts the chain
// at its base; a change at or below the chain's version is stale (a publish that arrived late) and
// is dropped. truncated marks a change published without ops: the chain then ends as a snapshot.
// The zero Chain takes any change and starts at its base.
func (c *Chain) Add(base, version int64, ops []PatchOp, truncated bool) bool {
	switch {
	case version <= c.Version:
		return false
	case c.Version == 0 || base > c.Version:
		c.Base, c.Ops, c.Snapshot = base, nil, false
	case base < c.Version:
		return false
	}
	c.Version = version
	if truncated {
		c.Snapshot, c.Ops = true, nil
		return true
	}
	if !c.Snapshot {
		c.Ops = MergeOps(c.Ops, ops)
	}
	return true
}

// Pending reports whether the chain holds a change that is not sent yet.
func (c *Chain) Pending() bool { return c.Version > c.Base }

// Sendable reports whether a connection that last received lastSent of this run can take the
// chain's ops frame; otherwise it needs the whole document (04 section 11.3 points 3 and 4: a
// client with another version would see a gap).
func (c *Chain) Sendable(lastSent int64) bool { return !c.Snapshot && lastSent == c.Base }

// Reset starts the next window: the chain becomes empty at its version.
func (c *Chain) Reset() { c.Base, c.Ops, c.Snapshot = c.Version, nil, false }
