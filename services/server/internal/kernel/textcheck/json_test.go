// SPDX-License-Identifier: Apache-2.0

package textcheck_test

import (
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/textcheck"
)

// TestStorableJSON: the escape of NUL and of an unpaired surrogate are refused anywhere in the
// document; an escaped backslash before u0000, a pair and plain text pass.
func TestStorableJSON(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		raw  string
		want bool
	}{
		{`{"a":"b"}`, true},
		{`{"a":"x\u0000y"}`, false},
		{`{"a\u0000":"b"}`, false},
		{`{"a":"\\u0000"}`, true},
		{`{"a":"😀"}`, true},
		{`{"a":"\ud83d"}`, false},
		{`{"a":"\ude00"}`, false},
		{`{"a":"\ud83dx"}`, false},
		{`["é", 1, true, null]`, true},
	} {
		if got := textcheck.StorableJSON([]byte(c.raw)); got != c.want {
			t.Errorf("StorableJSON(%s) = %v, want %v", c.raw, got, c.want)
		}
	}
}
