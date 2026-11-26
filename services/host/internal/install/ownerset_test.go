// SPDX-License-Identifier: Apache-2.0

package install

import "testing"

func TestOwnerSetContains(t *testing.T) {
	set := ownerSet{"S-1-5-18", "S-1-5-32-544", ""}
	for sid, want := range map[string]bool{
		"S-1-5-18": true, "s-1-5-32-544": true, "S-1-5-21-1-2-3-1001": false, "": false,
	} {
		if got := set.contains(sid); got != want {
			t.Errorf("contains(%q) = %v, want %v", sid, got, want)
		}
	}
}
