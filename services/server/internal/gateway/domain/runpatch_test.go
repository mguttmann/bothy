// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// TestMergeOps proves the merge rule of MergeOps on hand-written chains: a later replace takes
// the place of an earlier replace or add on exactly its path, everything else keeps its order,
// and the input is left untouched.
func TestMergeOps(t *testing.T) {
	v := func(s string) []byte { return []byte(s) }
	tests := []struct {
		name      string
		ops, next []PatchOp
		want      []PatchOp
	}{
		{
			name: "replace after replace keeps the later value at the later position",
			ops: []PatchOp{
				{Op: "replace", Path: "/model", Value: v(`"model-a"`)},
				{Op: "replace", Path: "/status_line", Value: v(`"reading"`)},
			},
			next: []PatchOp{{Op: "replace", Path: "/model", Value: v(`"model-b"`)}},
			want: []PatchOp{
				{Op: "replace", Path: "/status_line", Value: v(`"reading"`)},
				{Op: "replace", Path: "/model", Value: v(`"model-b"`)},
			},
		},
		{
			name: "add then replace stays one add with the later value at the later position",
			ops: []PatchOp{
				{Op: "add", Path: "/metrics/input_tokens", Value: v(`10`)},
				{Op: "add", Path: "/steps/s1", Value: v(`{"status":"running"}`)},
			},
			next: []PatchOp{{Op: "replace", Path: "/metrics/input_tokens", Value: v(`25`)}},
			want: []PatchOp{
				{Op: "add", Path: "/steps/s1", Value: v(`{"status":"running"}`)},
				{Op: "add", Path: "/metrics/input_tokens", Value: v(`25`)},
			},
		},
		{
			name: "ops on other paths keep their order",
			ops: []PatchOp{
				{Op: "replace", Path: "/a", Value: v(`1`)},
				{Op: "add", Path: "/b", Value: v(`2`)},
				{Op: "remove", Path: "/c"},
			},
			next: []PatchOp{
				{Op: "add", Path: "/d", Value: v(`3`)},
				{Op: "replace", Path: "/e", Value: v(`4`)},
			},
			want: []PatchOp{
				{Op: "replace", Path: "/a", Value: v(`1`)},
				{Op: "add", Path: "/b", Value: v(`2`)},
				{Op: "remove", Path: "/c"},
				{Op: "add", Path: "/d", Value: v(`3`)},
				{Op: "replace", Path: "/e", Value: v(`4`)},
			},
		},
		{
			name: "a path that only shares a prefix is another path",
			ops:  []PatchOp{{Op: "replace", Path: "/steps/s1/status", Value: v(`"running"`)}},
			next: []PatchOp{{Op: "replace", Path: "/steps/s10/status", Value: v(`"done"`)}},
			want: []PatchOp{
				{Op: "replace", Path: "/steps/s1/status", Value: v(`"running"`)},
				{Op: "replace", Path: "/steps/s10/status", Value: v(`"done"`)},
			},
		},
		{
			name: "remove followed by add on the same path keeps both",
			ops:  []PatchOp{{Op: "remove", Path: "/model"}},
			next: []PatchOp{{Op: "add", Path: "/model", Value: v(`"model-c"`)}},
			want: []PatchOp{
				{Op: "remove", Path: "/model"},
				{Op: "add", Path: "/model", Value: v(`"model-c"`)},
			},
		},
		{
			name: "remove, add, replace on one path folds the replace into the add after the remove",
			ops: []PatchOp{
				{Op: "remove", Path: "/model"},
				{Op: "add", Path: "/model", Value: v(`"model-c"`)},
			},
			next: []PatchOp{{Op: "replace", Path: "/model", Value: v(`"model-d"`)}},
			want: []PatchOp{
				{Op: "remove", Path: "/model"},
				{Op: "add", Path: "/model", Value: v(`"model-d"`)},
			},
		},
		{
			name: "replace after an op on an ancestor keeps the op before the ancestor",
			ops: []PatchOp{
				{Op: "replace", Path: "/steps/s1/status", Value: v(`"running"`)},
				{Op: "remove", Path: "/steps/s1"},
				{Op: "add", Path: "/steps/s1", Value: v(`{"status":"queued"}`)},
			},
			next: []PatchOp{{Op: "replace", Path: "/steps/s1/status", Value: v(`"done"`)}},
			want: []PatchOp{
				{Op: "replace", Path: "/steps/s1/status", Value: v(`"running"`)},
				{Op: "remove", Path: "/steps/s1"},
				{Op: "add", Path: "/steps/s1", Value: v(`{"status":"queued"}`)},
				{Op: "replace", Path: "/steps/s1/status", Value: v(`"done"`)},
			},
		},
		{
			name: "replace directly after a remove of its ancestor keeps the op before it",
			ops: []PatchOp{
				{Op: "add", Path: "/steps/s2/title", Value: v(`"Draft"`)},
				{Op: "remove", Path: "/steps/s2"},
			},
			next: []PatchOp{{Op: "replace", Path: "/steps/s2/title", Value: v(`"Final"`)}},
			want: []PatchOp{
				{Op: "add", Path: "/steps/s2/title", Value: v(`"Draft"`)},
				{Op: "remove", Path: "/steps/s2"},
				{Op: "replace", Path: "/steps/s2/title", Value: v(`"Final"`)},
			},
		},
		{
			name: "add and remove in next are appended without merging",
			ops:  []PatchOp{{Op: "replace", Path: "/model", Value: v(`"model-a"`)}},
			next: []PatchOp{
				{Op: "remove", Path: "/model"},
				{Op: "add", Path: "/model", Value: v(`"model-b"`)},
			},
			want: []PatchOp{
				{Op: "replace", Path: "/model", Value: v(`"model-a"`)},
				{Op: "remove", Path: "/model"},
				{Op: "add", Path: "/model", Value: v(`"model-b"`)},
			},
		},
		{
			name: "replaces inside next merge with each other",
			next: []PatchOp{
				{Op: "add", Path: "/status_line", Value: v(`"one"`)},
				{Op: "replace", Path: "/status_line", Value: v(`"two"`)},
				{Op: "replace", Path: "/status_line", Value: v(`"three"`)},
			},
			want: []PatchOp{{Op: "add", Path: "/status_line", Value: v(`"three"`)}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := clonePatchOps(tt.ops)
			got := MergeOps(tt.ops, tt.next)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("MergeOps = %s, want %s", formatPatchOps(got), formatPatchOps(tt.want))
			}
			if !reflect.DeepEqual(tt.ops, before) {
				t.Fatalf("ops modified: %s, was %s", formatPatchOps(tt.ops), formatPatchOps(before))
			}
		})
	}
}

// TestMergeOpsProperty proves the claim of MergeOps on random server-shaped chains: folding the
// normalized changes of a chain with MergeOps (and with Chain.Add) and applying the result with an
// independent RFC 6902 applier to the start document gives the document that sequential
// application of all changes gives, and every merged op list applies without error.
func TestMergeOpsProperty(t *testing.T) {
	t.Run("applier", testPatchApplier)
	for seed := uint64(1); seed <= 4; seed++ {
		t.Run(fmt.Sprintf("seed=%d", seed), func(t *testing.T) {
			rng := rand.New(rand.NewPCG(seed, seed*0x9e3779b97f4a7c15)) //nolint:gosec // a fixed seed makes the property test reproducible
			for iter := range 1000 {
				start := randomRunDoc(rng)
				doc := deepCopyJSON(t, start)
				var changes [][]PatchOp
				var change []PatchOp
				for range rng.IntN(30) + 1 {
					op := randomNormalizedOp(rng, doc)
					next, err := applyOne(doc, op)
					if err != nil {
						t.Fatalf("iter %d: generator produced an invalid op %s: %v", iter, formatPatchOps([]PatchOp{op}), err)
					}
					doc = next.(map[string]any)
					change = append(change, op)
					if rng.IntN(3) == 0 {
						changes = append(changes, change)
						change = nil
					}
				}
				if len(change) > 0 || rng.IntN(4) == 0 {
					changes = append(changes, change)
				}

				var merged []PatchOp
				var chain Chain
				for i, c := range changes {
					before := clonePatchOps(merged)
					folded := MergeOps(merged, c)
					if !reflect.DeepEqual(merged, before) {
						t.Fatalf("iter %d: MergeOps modified ops", iter)
					}
					merged = folded
					base := int64(10 + i)
					if !chain.Add(base, base+1, c, false) {
						t.Fatalf("iter %d: Chain.Add rejected change %d", iter, i)
					}
				}
				got, err := applyPatch(start, merged)
				if err != nil {
					t.Fatalf("iter %d: merged ops do not apply: %v\nchanges %s\nmerged %s",
						iter, err, formatChanges(changes), formatPatchOps(merged))
				}
				if !reflect.DeepEqual(got, doc) {
					t.Fatalf("iter %d: merged result differs\ngot  %s\nwant %s\nchanges %s\nmerged %s",
						iter, mustJSON(t, got), mustJSON(t, doc), formatChanges(changes), formatPatchOps(merged))
				}
				if len(merged) != len(chain.Ops) || (len(merged) > 0 && !reflect.DeepEqual(chain.Ops, merged)) {
					t.Fatalf("iter %d: Chain.Ops %s, MergeOps %s", iter, formatPatchOps(chain.Ops), formatPatchOps(merged))
				}
			}
		})
	}
}

// testPatchApplier checks the test's own RFC 6902 applier on the cases the property test relies
// on, so a fault in the oracle cannot hide a fault in MergeOps.
func testPatchApplier(t *testing.T) {
	start := map[string]any{"a~b": map[string]any{"c/d": "x"}, "m": 1.0}
	got, err := applyPatch(start, []PatchOp{
		{Op: "replace", Path: "/a~0b/c~1d", Value: []byte(`"y"`)},
		{Op: "add", Path: "/a~0b/~01", Value: []byte(`true`)},
		{Op: "remove", Path: "/m"},
		{Op: "add", Path: "/n", Value: []byte(`{"k":[1]}`)},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"a~b": map[string]any{"c/d": "y", "~1": true}, "n": map[string]any{"k": []any{1.0}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("applier = %v, want %v", got, want)
	}
	if !reflect.DeepEqual(start, map[string]any{"a~b": map[string]any{"c/d": "x"}, "m": 1.0}) {
		t.Fatalf("applier modified its input: %v", start)
	}
	for _, bad := range []PatchOp{
		{Op: "replace", Path: "/missing", Value: []byte(`1`)},
		{Op: "remove", Path: "/missing"},
		{Op: "add", Path: "/missing/child", Value: []byte(`1`)},
		{Op: "add", Path: "no-slash", Value: []byte(`1`)},
		{Op: "move", Path: "/m"},
	} {
		if _, err := applyPatch(start, []PatchOp{bad}); err == nil {
			t.Errorf("applier accepted %s", formatPatchOps([]PatchOp{bad}))
		}
	}
}

// TestChainAdd proves the folding rules of Chain: start at the first change, extend on a matching
// base, restart on a version step, drop stale changes, the snapshot mode of a truncated change,
// and Pending, Sendable and Reset.
func TestChainAdd(t *testing.T) {
	v := func(s string) []byte { return []byte(s) }
	opA := PatchOp{Op: "replace", Path: "/status_line", Value: v(`"one"`)}
	opB := PatchOp{Op: "replace", Path: "/status_line", Value: v(`"two"`)}
	opC := PatchOp{Op: "add", Path: "/model", Value: v(`"model-a"`)}

	t.Run("zero chain", func(t *testing.T) {
		var c Chain
		if c.Pending() {
			t.Fatal("zero chain is pending")
		}
		if !c.Add(5, 6, []PatchOp{opA}, false) {
			t.Fatal("zero chain rejected the first change")
		}
		assertChain(t, c, Chain{Base: 5, Version: 6, Ops: []PatchOp{opA}})
	})

	t.Run("base equal to version extends and merges", func(t *testing.T) {
		c := Chain{Base: 5, Version: 6, Ops: []PatchOp{opA}}
		if !c.Add(6, 7, []PatchOp{opC, opB}, false) {
			t.Fatal("extending change rejected")
		}
		assertChain(t, c, Chain{Base: 5, Version: 7, Ops: []PatchOp{opC, opB}})
	})

	t.Run("base above version restarts at the base", func(t *testing.T) {
		c := Chain{Base: 5, Version: 6, Ops: []PatchOp{opA}}
		if !c.Add(8, 9, []PatchOp{opC}, false) {
			t.Fatal("restarting change rejected")
		}
		assertChain(t, c, Chain{Base: 8, Version: 9, Ops: []PatchOp{opC}})
	})

	t.Run("base above version ends a snapshot", func(t *testing.T) {
		c := Chain{Base: 5, Version: 6, Snapshot: true}
		if !c.Add(7, 8, []PatchOp{opC}, false) {
			t.Fatal("restarting change rejected")
		}
		assertChain(t, c, Chain{Base: 7, Version: 8, Ops: []PatchOp{opC}})
	})

	t.Run("stale changes are dropped", func(t *testing.T) {
		for _, ch := range []struct{ base, version int64 }{
			{5, 6}, // the same change again
			{4, 5}, // older than the chain
			{6, 6}, // version not above
			{3, 7}, // version above, but base inside the chain
		} {
			c := Chain{Base: 5, Version: 6, Ops: []PatchOp{opA}}
			if c.Add(ch.base, ch.version, []PatchOp{opB}, false) {
				t.Errorf("Add(%d, %d) taken", ch.base, ch.version)
			}
			if c.Add(ch.base, ch.version, nil, true) {
				t.Errorf("truncated Add(%d, %d) taken", ch.base, ch.version)
			}
			assertChain(t, c, Chain{Base: 5, Version: 6, Ops: []PatchOp{opA}})
		}
	})

	t.Run("truncated sets Snapshot until Reset", func(t *testing.T) {
		c := Chain{Base: 5, Version: 6, Ops: []PatchOp{opA}}
		if !c.Add(6, 7, []PatchOp{opB}, true) {
			t.Fatal("truncated change rejected")
		}
		assertChain(t, c, Chain{Base: 5, Version: 7, Snapshot: true})
		if !c.Add(7, 8, []PatchOp{opC}, false) {
			t.Fatal("change after a snapshot rejected")
		}
		assertChain(t, c, Chain{Base: 5, Version: 8, Snapshot: true})
		if !c.Pending() {
			t.Fatal("snapshot chain not pending")
		}
		if c.Sendable(5) {
			t.Fatal("snapshot chain sendable as ops")
		}
		c.Reset()
		assertChain(t, c, Chain{Base: 8, Version: 8})
	})

	t.Run("truncated first change of a zero chain", func(t *testing.T) {
		var c Chain
		if !c.Add(3, 4, []PatchOp{opA}, true) {
			t.Fatal("truncated first change rejected")
		}
		assertChain(t, c, Chain{Base: 3, Version: 4, Snapshot: true})
	})

	t.Run("Pending, Sendable and Reset", func(t *testing.T) {
		c := Chain{Base: 5, Version: 7, Ops: []PatchOp{opA}}
		if !c.Pending() {
			t.Fatal("chain with a change not pending")
		}
		for _, lastSent := range []int64{0, 4, 6, 7, 8} {
			if c.Sendable(lastSent) {
				t.Errorf("Sendable(%d) true for base 5", lastSent)
			}
		}
		if !c.Sendable(5) {
			t.Fatal("Sendable(base) false")
		}
		c.Reset()
		assertChain(t, c, Chain{Base: 7, Version: 7})
		if c.Pending() {
			t.Fatal("reset chain pending")
		}
		if !c.Sendable(7) {
			t.Fatal("reset chain not sendable at its version")
		}
		if !c.Add(7, 8, []PatchOp{opB}, false) {
			t.Fatal("change after Reset rejected")
		}
		assertChain(t, c, Chain{Base: 7, Version: 8, Ops: []PatchOp{opB}})
	})
}

// assertChain fails when got differs from want; nil and empty Ops count as equal.
func assertChain(t *testing.T, got, want Chain) {
	t.Helper()
	if got.Base != want.Base || got.Version != want.Version || got.Snapshot != want.Snapshot ||
		len(got.Ops) != len(want.Ops) || (len(want.Ops) > 0 && !reflect.DeepEqual(got.Ops, want.Ops)) {
		t.Fatalf("chain = {Base %d Version %d Snapshot %t Ops %s}, want {Base %d Version %d Snapshot %t Ops %s}",
			got.Base, got.Version, got.Snapshot, formatPatchOps(got.Ops),
			want.Base, want.Version, want.Snapshot, formatPatchOps(want.Ops))
	}
}

// The members the generator draws from. Step ids include ~ and / so the pointers need escaping,
// and s1 and s10 share a prefix.
var (
	genTopMembers  = []string{"model", "status_line", "last_text_excerpt", "plan"}
	genMetrics     = []string{"input_tokens", "output_tokens", "context_pct"}
	genStepIDs     = []string{"s1", "s10", "s2", "a~b", "c/d"}
	genStepMembers = []string{"status", "title", "detail", "finished_at"}
)

// randomRunDoc returns a random document shaped like run_document.v1: a few scalar members, a
// metrics object and a steps object of step objects.
func randomRunDoc(rng *rand.Rand) map[string]any {
	doc := map[string]any{"id": "run-1"}
	for _, k := range genTopMembers {
		if rng.IntN(2) == 0 {
			doc[k] = randomValue(rng)
		}
	}
	metrics := map[string]any{}
	for _, f := range genMetrics {
		if rng.IntN(2) == 0 {
			metrics[f] = float64(rng.IntN(1000))
		}
	}
	doc["metrics"] = metrics
	steps := map[string]any{}
	for _, id := range genStepIDs {
		if rng.IntN(2) == 0 {
			steps[id] = randomStep(rng)
		}
	}
	doc["steps"] = steps
	return doc
}

// randomStep returns a step object with a random subset of its members.
func randomStep(rng *rand.Rand) map[string]any {
	step := map[string]any{"kind": "tool"}
	for _, m := range genStepMembers {
		if rng.IntN(2) == 0 {
			step[m] = randomValue(rng)
		}
	}
	return step
}

// randomValue returns a decoded JSON value: a string, a number, a bool, null or a small object.
func randomValue(rng *rand.Rand) any {
	switch rng.IntN(5) {
	case 0:
		return "value-" + strconv.Itoa(rng.IntN(100))
	case 1:
		return float64(rng.IntN(100))
	case 2:
		return rng.IntN(2) == 0
	case 3:
		return nil
	default:
		return map[string]any{"text": "item-" + strconv.Itoa(rng.IntN(100))}
	}
}

// randomNormalizedOp returns one op the way the server normalizes it against doc: add when the
// member is absent, replace or remove when it is present; whole steps are only added or removed,
// step members only added or replaced.
func randomNormalizedOp(rng *rand.Rand, doc map[string]any) PatchOp {
	enc := func(v any) []byte {
		b, err := json.Marshal(v)
		if err != nil {
			panic(err)
		}
		return b
	}
	member := func(obj map[string]any, key, path string, value any, removable bool) PatchOp {
		if _, ok := obj[key]; !ok {
			return PatchOp{Op: "add", Path: path, Value: enc(value)}
		}
		if removable && rng.IntN(4) == 0 {
			return PatchOp{Op: "remove", Path: path}
		}
		return PatchOp{Op: "replace", Path: path, Value: enc(value)}
	}
	switch rng.IntN(4) {
	case 0:
		k := genTopMembers[rng.IntN(len(genTopMembers))]
		return member(doc, k, "/"+escapePointer(k), randomValue(rng), true)
	case 1:
		f := genMetrics[rng.IntN(len(genMetrics))]
		return member(doc["metrics"].(map[string]any), f, "/metrics/"+f, float64(rng.IntN(1000)), true)
	default:
		steps := doc["steps"].(map[string]any)
		id := genStepIDs[rng.IntN(len(genStepIDs))]
		path := "/steps/" + escapePointer(id)
		step, ok := steps[id].(map[string]any)
		if !ok {
			return PatchOp{Op: "add", Path: path, Value: enc(randomStep(rng))}
		}
		if rng.IntN(5) == 0 {
			return PatchOp{Op: "remove", Path: path}
		}
		m := genStepMembers[rng.IntN(len(genStepMembers))]
		return member(step, m, path+"/"+m, randomValue(rng), false)
	}
}

// escapePointer escapes one reference token (RFC 6901 section 3).
func escapePointer(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "~", "~0"), "/", "~1")
}

// applyPatch applies ops to a copy of doc with RFC 6902 sections 4.1 add, 4.2 remove and 4.3
// replace and returns the result; doc is not modified. Pointers follow RFC 6901: "" is the whole
// document, every other pointer starts with "/" and its tokens decode ~1 to "/" and then ~0 to "~".
// replace and remove need an existing target, add an existing parent. Arrays support the index
// forms of add and remove only as far as section 4.1 needs ("-" appends).
func applyPatch(doc any, ops []PatchOp) (any, error) {
	out, err := cloneJSONValue(doc)
	if err != nil {
		return nil, err
	}
	for i, op := range ops {
		if out, err = applyOne(out, op); err != nil {
			return nil, fmt.Errorf("op %d %s %s: %w", i, op.Op, op.Path, err)
		}
	}
	return out, nil
}

// applyOne applies one op to doc in place where it can and returns the new document.
func applyOne(doc any, op PatchOp) (any, error) {
	var value any
	switch op.Op {
	case "add", "replace":
		if err := json.Unmarshal(op.Value, &value); err != nil {
			return nil, fmt.Errorf("value: %w", err)
		}
	case "remove":
	default:
		return nil, fmt.Errorf("unsupported op %q", op.Op)
	}
	tokens, err := parsePointer(op.Path)
	if err != nil {
		return nil, err
	}
	if len(tokens) == 0 {
		switch op.Op {
		case "remove":
			return nil, errors.New("remove of the whole document")
		default:
			return value, nil
		}
	}
	parent := doc
	for _, tok := range tokens[:len(tokens)-1] {
		if parent, err = childOf(parent, tok); err != nil {
			return nil, err
		}
	}
	last := tokens[len(tokens)-1]
	switch p := parent.(type) {
	case map[string]any:
		_, exists := p[last]
		switch op.Op {
		case "add":
			p[last] = value
		case "replace":
			if !exists {
				return nil, fmt.Errorf("replace of missing member %q", last)
			}
			p[last] = value
		case "remove":
			if !exists {
				return nil, fmt.Errorf("remove of missing member %q", last)
			}
			delete(p, last)
		}
		return doc, nil
	case []any:
		return applyToArray(doc, tokens, p, last, op.Op, value)
	default:
		return nil, fmt.Errorf("parent of %q is not a container", last)
	}
}

// applyToArray applies op on the element last of the array arr, the parent of the last token,
// and stores the changed array back into doc.
func applyToArray(doc any, tokens []string, arr []any, last, op string, value any) (any, error) {
	var idx int
	if last == "-" && op == "add" {
		idx = len(arr)
	} else {
		n, err := strconv.Atoi(last)
		if err != nil || n < 0 || (last != "0" && strings.HasPrefix(last, "0")) {
			return nil, fmt.Errorf("bad array index %q", last)
		}
		idx = n
	}
	limit := len(arr)
	if op == "add" {
		limit++
	}
	if idx >= limit {
		return nil, fmt.Errorf("array index %d out of range", idx)
	}
	switch op {
	case "add":
		arr = slices.Insert(arr, idx, value)
	case "replace":
		arr[idx] = value
	case "remove":
		arr = slices.Delete(arr, idx, idx+1)
	}
	return setAt(doc, tokens[:len(tokens)-1], arr)
}

// setAt stores value at the pointer tokens of doc (an existing location) and returns doc.
func setAt(doc any, tokens []string, value any) (any, error) {
	if len(tokens) == 0 {
		return value, nil
	}
	parent := doc
	var err error
	for _, tok := range tokens[:len(tokens)-1] {
		if parent, err = childOf(parent, tok); err != nil {
			return nil, err
		}
	}
	last := tokens[len(tokens)-1]
	switch p := parent.(type) {
	case map[string]any:
		p[last] = value
	case []any:
		n, err := strconv.Atoi(last)
		if err != nil || n < 0 || n >= len(p) {
			return nil, fmt.Errorf("bad array index %q", last)
		}
		p[n] = value
	default:
		return nil, fmt.Errorf("parent of %q is not a container", last)
	}
	return doc, nil
}

// childOf returns the member or element tok of v.
func childOf(v any, tok string) (any, error) {
	switch c := v.(type) {
	case map[string]any:
		child, ok := c[tok]
		if !ok {
			return nil, fmt.Errorf("missing member %q", tok)
		}
		return child, nil
	case []any:
		n, err := strconv.Atoi(tok)
		if err != nil || n < 0 || n >= len(c) {
			return nil, fmt.Errorf("bad array index %q", tok)
		}
		return c[n], nil
	default:
		return nil, fmt.Errorf("%q below a scalar", tok)
	}
}

// parsePointer splits an RFC 6901 pointer into its decoded reference tokens.
func parsePointer(p string) ([]string, error) {
	if p == "" {
		return nil, nil
	}
	if !strings.HasPrefix(p, "/") {
		return nil, fmt.Errorf("pointer %q does not start with /", p)
	}
	tokens := strings.Split(p[1:], "/")
	for i, tok := range tokens {
		tokens[i] = strings.ReplaceAll(strings.ReplaceAll(tok, "~1", "/"), "~0", "~")
	}
	return tokens, nil
}

// cloneJSONValue returns a deep copy of a decoded JSON value through an encode and decode.
func cloneJSONValue(v any) (any, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var out any
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// deepCopyJSON returns a deep copy of doc as a decoded JSON object.
func deepCopyJSON(t *testing.T, doc map[string]any) map[string]any {
	t.Helper()
	v, err := cloneJSONValue(doc)
	if err != nil {
		t.Fatal(err)
	}
	return v.(map[string]any)
}

// clonePatchOps returns a copy of ops that shares no memory with it.
func clonePatchOps(ops []PatchOp) []PatchOp {
	if ops == nil {
		return nil
	}
	out := make([]PatchOp, len(ops))
	for i, op := range ops {
		out[i] = PatchOp{Op: op.Op, Path: op.Path, Value: slices.Clone(op.Value)}
	}
	return out
}

// formatPatchOps renders ops readably for failure messages.
func formatPatchOps(ops []PatchOp) string {
	parts := make([]string, len(ops))
	for i, op := range ops {
		parts[i] = op.Op + " " + op.Path
		if op.Value != nil {
			parts[i] += " " + string(op.Value)
		}
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// formatChanges renders a chain of changes readably for failure messages.
func formatChanges(changes [][]PatchOp) string {
	parts := make([]string, len(changes))
	for i, c := range changes {
		parts[i] = formatPatchOps(c)
	}
	return strings.Join(parts, " | ")
}

// mustJSON encodes v for failure messages.
func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
