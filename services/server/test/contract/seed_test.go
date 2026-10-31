// SPDX-License-Identifier: Apache-2.0

package contract

import "testing"

// TestSeed: a G7 run takes its seed only from CONTRACT_SEED, a decimal unsigned integer, and never
// draws one of its own; an empty or malformed value is an error, so the cases of every run are the
// ones the seed names (flake list #238).
func TestSeed(t *testing.T) {
	t.Parallel()
	for value, want := range map[string]string{"238": "238", "0": "0", "4153049471880267": "4153049471880267"} {
		got, err := Seed(value)
		if err != nil || got != want {
			t.Errorf("Seed(%q) = %q, %v; want %q", value, got, err, want)
		}
	}
	for _, value := range []string{"", " ", "-1", "1.5", "abc", "184467440737095516160"} {
		if got, err := Seed(value); err == nil {
			t.Errorf("Seed(%q) = %q, want an error", value, got)
		}
	}
}
