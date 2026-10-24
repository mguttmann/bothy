// SPDX-License-Identifier: Apache-2.0

package importtool

import (
	"reflect"
	"testing"
)

// TestFlattenCounters checks that counters print as plain numbers (no exponent form for large
// counts), flags as true or false, nested objects with dotted keys, and text not at all.
func TestFlattenCounters(t *testing.T) {
	stats := map[string]any{
		"tool": map[string]any{"messages_taken": float64(1_234_567), "files": float64(1_000_000)},
		"done": true,
		"note": "text never printed",
		"rate": 0.5,
	}
	want := []string{"done: true", "rate: 0.5", "tool.files: 1000000", "tool.messages_taken: 1234567"}
	if got := flatten("", stats); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}
