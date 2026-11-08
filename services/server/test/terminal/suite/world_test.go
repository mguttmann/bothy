// SPDX-License-Identifier: Apache-2.0

package suite

import (
	"regexp"
	"testing"
)

// TestSuffixesAreUniqueInTheProcess: the suffix of every world of a process differs from all
// others, so two cases on the one stack of a package never ask for the same handle (issue 238:
// tm-t23/older_silent failed with 409 handle taken when two random 24 bit suffixes met), and it
// keeps six hex digits, so handles stay within principal.handle_max_chars.
func TestSuffixesAreUniqueInTheProcess(t *testing.T) {
	const n = 1 << 16
	shape := regexp.MustCompile(`^[0-9a-f]{6}$`)
	seen := make(map[string]bool, n)
	for range n {
		s := newSuffix()
		if !shape.MatchString(s) {
			t.Fatalf("suffix %q is not six hex digits", s)
		}
		if seen[s] {
			t.Fatalf("suffix %q handed out twice within %d worlds", s, len(seen)+1)
		}
		seen[s] = true
	}
}
