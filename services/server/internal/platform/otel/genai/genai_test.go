// SPDX-License-Identifier: Apache-2.0

package genai_test

import (
	"flag"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/platform/otel/genai"
)

var update = flag.Bool("update", false, "rewrite testdata/keys.golden from the package")

// golden is the snapshot file of the keys.
const golden = "testdata/keys.golden"

// TestGenAIKeysSnapshot holds docs/observability/signals.md "GenAI conventions" item 3: a change
// of the convention changes this package and the snapshot in one change. Run
// `go test ./internal/platform/otel/genai -update` after a deliberate change, then update the
// collector allowlist and the dashboards in the same change.
func TestGenAIKeysSnapshot(t *testing.T) {
	got := snapshot()
	if *update {
		if err := os.WriteFile(golden, []byte(got), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("read %s: %v (run with -update to create it)", golden, err)
	}
	if got != string(want) {
		t.Errorf("gen_ai keys differ from %s (run with -update after a deliberate change):\n--- got\n%s--- want\n%s", golden, got, want)
	}
}

// snapshot renders the convention reference, the sent keys and the forbidden keys, one per line.
func snapshot() string {
	var b strings.Builder
	b.WriteString("# gen_ai keys of internal/platform/otel/genai; regenerate with go test -update\n")
	b.WriteString("convention " + genai.ConventionRef + "\n")
	for _, k := range genai.Keys() {
		b.WriteString("key " + k + "\n")
	}
	forbidden := slices.Clone(genai.Forbidden)
	slices.Sort(forbidden)
	for _, k := range forbidden {
		b.WriteString("forbidden " + k + "\n")
	}
	return b.String()
}

// TestKeysAreGenAIAndDistinct checks the shape the snapshot cannot: every key and every forbidden
// key starts with gen_ai., Keys is sorted without duplicates, and no key is also forbidden.
func TestKeysAreGenAIAndDistinct(t *testing.T) {
	t.Parallel()
	keys := genai.Keys()
	if len(keys) != 13 {
		t.Errorf("Keys() has %d keys, the collector allowlist has 13", len(keys))
	}
	if !slices.IsSorted(keys) || len(slices.Compact(slices.Clone(keys))) != len(keys) {
		t.Errorf("Keys() is not sorted or has duplicates: %v", keys)
	}
	for _, k := range slices.Concat(keys, genai.Forbidden) {
		if !strings.HasPrefix(k, "gen_ai.") {
			t.Errorf("%q does not start with gen_ai.", k)
		}
	}
	for _, k := range genai.Forbidden {
		if slices.Contains(keys, k) {
			t.Errorf("%q is both sent and forbidden", k)
		}
	}
	if genai.ConventionRef == "" {
		t.Error("ConventionRef is empty")
	}
}
