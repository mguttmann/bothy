// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
)

// fixtureDir holds the base fixture; negativeDir holds one directory per negative case with only
// the files that differ from the base.
const (
	fixtureDir  = "testdata/fixture"
	negativeDir = "testdata/negative"
)

// fixturePaths copies the base fixture into a fresh directory, lays the files of the negative
// case over it (none for ""), and returns the paths of spec, catalogue and enable list.
func fixturePaths(t *testing.T, negative string) (spec, catalog, list string) {
	t.Helper()
	dir := t.TempDir()
	copyFiles(t, fixtureDir, dir)
	if negative != "" {
		copyFiles(t, filepath.Join(negativeDir, negative), dir)
	}
	return filepath.Join(dir, "openapi.yaml"), filepath.Join(dir, "actions.yaml"), filepath.Join(dir, "operations.yaml")
}

// copyFiles copies the regular files of from into to, replacing files of the same name.
func copyFiles(t *testing.T, from, to string) {
	t.Helper()
	src, err := os.OpenRoot(from)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = src.Close() }()
	dst, err := os.OpenRoot(to)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = dst.Close() }()
	entries, err := os.ReadDir(from)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		data, err := src.ReadFile(e.Name())
		if err != nil {
			t.Fatal(err)
		}
		if err := dst.WriteFile(e.Name(), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

// writeFile replaces one input file of a fixture copy.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

// readText returns the content of a fixture file.
func readText(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// problemsOf returns the problems of err, failing the test when err is no problem list.
func problemsOf(t *testing.T, err error) []string {
	t.Helper()
	var list *problemsError
	if !errors.As(err, &list) {
		t.Fatalf("error = %v, want a problem list", err)
	}
	return list.items
}

// expectProblems loads the fixture with the negative case and requires exactly count problems,
// each containing every string of wants. Requiring the exact count proves that the case differs
// from the base fixture only in the rule it tests.
func expectProblems(t *testing.T, negative string, count int, wants ...string) {
	t.Helper()
	spec, catalog, list := fixturePaths(t, negative)
	_, err := loadInputs(spec, catalog, list)
	problems := problemsOf(t, err)
	if len(problems) != count {
		t.Fatalf("got %d problems, want %d:\n%s", len(problems), count, strings.Join(problems, "\n"))
	}
	for _, p := range problems {
		for _, w := range wants {
			if !strings.Contains(p, w) {
				t.Errorf("problem %q does not contain %q", p, w)
			}
		}
	}
}

func TestLoadInputs_FixtureIsValid(t *testing.T) {
	spec, catalog, list := fixturePaths(t, "")
	in, err := loadInputs(spec, catalog, list)
	if err != nil {
		t.Fatalf("loadInputs: %v", err)
	}
	var ids []string
	for _, op := range in.ops {
		ids = append(ids, op.ID)
	}
	if want := []string{"createThing", "getThing", "updateThing"}; !reflect.DeepEqual(ids, want) {
		t.Errorf("operations = %v, want %v (sorted by ID)", ids, want)
	}
	for _, id := range ids {
		if !in.enabled[id] {
			t.Errorf("%s is not enabled", id)
		}
	}
	if got := len(in.actions); got != 5 {
		t.Errorf("catalogue has %d actions, want 5", got)
	}
}

func TestGenerate_RejectsEnabledNotInSpec(t *testing.T) {
	expectProblems(t, "enabled-not-in-spec", 1, "deleteThing", "not an operation under paths")
}

func TestGenerate_RejectsOperationMissingFromList(t *testing.T) {
	expectProblems(t, "operation-missing-from-list", 1, "updateThing", "in neither enabled nor later")
}

func TestGenerate_RejectsDuplicateEntry(t *testing.T) {
	expectProblems(t, "duplicate-entry", 1, "getThing", "more than once")
}

func TestGenerate_RejectsEntryInBothSections(t *testing.T) {
	expectProblems(t, "entry-in-both-sections", 1, "updateThing", "enabled and in later")
}

func TestGenerate_RejectsUnsortedSection(t *testing.T) {
	expectProblems(t, "unsorted-section", 1, "createThing", "not sorted")
}

func TestGenerate_RejectsBadMilestone(t *testing.T) {
	expectProblems(t, "bad-milestone", 1, "updateThing", `milestone "M9"`)
}

func TestGenerate_RejectsTicketMilestoneMismatch(t *testing.T) {
	expectProblems(t, "ticket-milestone-mismatch", 1, "updateThing", `ticket "M3-01"`, "milestone M2")
}

func TestGenerate_RejectsUnknownAction(t *testing.T) {
	expectProblems(t, "unknown-action", 1, "createThing", `"thing.delete"`, "not in")
}

func TestGenerate_RejectsClassMismatch(t *testing.T) {
	expectProblems(t, "class-mismatch", 1, "createThing", `"rl.read"`, "rl.write")
}

func TestGenerate_RejectsSystemAction(t *testing.T) {
	expectProblems(t, "system-action", 1, "updateThing", `"thing.purge"`, "system")
}

func TestGenerate_RejectsMissingExtensions(t *testing.T) {
	expectProblems(t, "missing-extensions", 2, "updateThing", "missing x-")
}

// TestLoadInputs_RejectsListForms covers the list rules that need no fixture directory of
// their own: each case replaces the enable list of the base fixture.
func TestLoadInputs_RejectsListForms(t *testing.T) {
	cases := []struct {
		name, list string
		wants      []string
	}{
		{"version", "version: 2\nenabled: [createThing, getThing, updateThing]\nlater: []\n",
			[]string{"version 2", "want 1"}},
		{"unknown key", "version: 1\nenabled: [createThing, getThing, updateThing]\nlater: []\nextra: 1\n",
			[]string{"extra"}},
		{"ticket form", "version: 1\nenabled: [createThing, getThing]\nlater:\n  - operation_id: updateThing\n    milestone: M2\n    ticket: M2-1\n",
			[]string{"updateThing", `ticket "M2-1"`}},
		{"missing milestone", "version: 1\nenabled: [createThing, getThing]\nlater:\n  - operation_id: updateThing\n",
			[]string{"updateThing", "milestone"}},
		{"missing operation id", "version: 1\nenabled: [createThing, getThing, updateThing]\nlater:\n  - milestone: M2\n",
			[]string{"later[0]", "operation_id"}},
		{"webhook listed", "version: 1\nenabled: [createThing, getThing, thingChanged, updateThing]\nlater: []\n",
			[]string{"thingChanged", "not an operation under paths"}},
		{"later unsorted", "version: 1\nenabled: [createThing]\nlater:\n  - operation_id: updateThing\n    milestone: M2\n  - operation_id: getThing\n    milestone: M1\n",
			[]string{"getThing", "not sorted"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			spec, catalog, list := fixturePaths(t, "")
			writeFile(t, list, c.list)
			_, err := loadInputs(spec, catalog, list)
			problems := problemsOf(t, err)
			joined := strings.Join(problems, "\n")
			for _, w := range c.wants {
				if !strings.Contains(joined, w) {
					t.Errorf("problems do not mention %q:\n%s", w, joined)
				}
			}
		})
	}
}

// TestLoadInputs_ReportsEveryProblem proves that validation collects problems across rules and
// files instead of stopping at the first one.
func TestLoadInputs_ReportsEveryProblem(t *testing.T) {
	spec, catalog, list := fixturePaths(t, "unknown-action")
	writeFile(t, list, "version: 1\nenabled: [createThing, getThing]\nlater: []\n")
	_, err := loadInputs(spec, catalog, list)
	joined := strings.Join(problemsOf(t, err), "\n")
	for _, w := range []string{"thing.delete", "updateThing"} {
		if !strings.Contains(joined, w) {
			t.Errorf("problems do not mention %q:\n%s", w, joined)
		}
	}
}

func TestLoadInputs_MissingFile(t *testing.T) {
	spec, catalog, _ := fixturePaths(t, "")
	_, err := loadInputs(spec, catalog, filepath.Join(t.TempDir(), "absent.yaml"))
	if err == nil || !strings.Contains(err.Error(), "absent.yaml") {
		t.Fatalf("error = %v, want one naming the missing file", err)
	}
}

// realInputs loads the real spec, catalogue and enable list once per test binary: the kin-openapi
// load of the whole contract is the slowest step of these tests, above all under -race.
var realInputs = sync.OnceValues(func() (*inputs, error) {
	return loadInputs(
		"../../../../../docs/api/openapi.yaml",
		"../../../../../docs/spec/actions.yaml",
		"../../operations.yaml",
	)
})

// TestRealInputs_Valid is the gate check of the enable list: the real spec, catalogue and list
// pass every rule, so every listed operation exists in openapi.yaml and the list is complete.
func TestRealInputs_Valid(t *testing.T) {
	in, err := realInputs()
	if err != nil {
		t.Fatalf("real inputs: %v", err)
	}
	if len(in.ops) == 0 {
		t.Fatal("the real spec has no operation under paths")
	}
}

// TestRealSpec_NoHealthOperations holds W2 (Ruling L2, 06 R-API-16): liveness and readiness
// live on the internal listener, so no operation of the public API looks like a health probe.
func TestRealSpec_NoHealthOperations(t *testing.T) {
	in, err := realInputs()
	if err != nil {
		t.Fatal(err)
	}
	if found := healthOperations(in.ops); len(found) != 0 {
		t.Errorf("health operations in the public API: %v", found)
	}
}

func TestHealthOperations_FindsProbes(t *testing.T) {
	ops := []opMeta{
		{ID: "getLiveness", Path: "/api/v1/live"},
		{ID: "getStatus", Path: "/api/v1/readyz"},
		{ID: "getThing", Path: "/api/v1/things/{thing_id}"},
		{ID: "checkHealth", Path: "/api/v1/x"},
		{ID: "getPublicStatus", Path: "/api/v1/status/public"},
		{ID: "getStatsLive", Path: "/api/v1/stats/live"},
		{ID: "getHealthz", Path: "/api/v1/x"},
	}
	got := healthOperations(ops)
	if want := []string{"checkHealth", "getHealthz", "getLiveness", "getStatus"}; !reflect.DeepEqual(got, want) {
		t.Errorf("healthOperations = %v, want %v", got, want)
	}
}

func TestGenerate_RejectsKeyedOperationWithQuery(t *testing.T) {
	expectProblems(t, "keyed-query", 1, "createThing", "Idempotency-Key", "query parameter")
}

func TestGenerate_RejectsKeyedOperationWithNonJSONBody(t *testing.T) {
	expectProblems(t, "keyed-non-json-body", 1, "createThing", "Idempotency-Key", "application/json")
}

func TestGenerate_RejectsKeyedOperationWithNonJSONResponse(t *testing.T) {
	expectProblems(t, "keyed-non-json-response", 1, "createThing", "Idempotency-Key", "201")
}

func TestGenerate_RejectsKeyedETagOperationWithoutVersion(t *testing.T) {
	expectProblems(t, "keyed-etag-without-version", 1, "createThing", "ETag", "version")
}

// TestRealSpec_KeyedOperationsReplayable holds the generator checks for keyed operations on every
// operation of docs/api/openapi.yaml, enabled or not: a replay can rebuild each of their
// responses from the stored body (06 "Idempotency" point 4).
func TestRealSpec_KeyedOperationsReplayable(t *testing.T) {
	in, err := realInputs()
	if err != nil {
		t.Fatalf("real inputs: %v", err)
	}
	keyed := 0
	for _, op := range in.ops {
		if op.IdempotencyKey {
			keyed++
		}
	}
	if keyed == 0 {
		t.Fatal("the real spec declares no keyed operation")
	}
}

func TestGenerate_RejectsKeyedOperationWithNonJSONRangeResponse(t *testing.T) {
	expectProblems(t, "keyed-non-json-range", 1, "createThing", "Idempotency-Key", "2XX")
}
