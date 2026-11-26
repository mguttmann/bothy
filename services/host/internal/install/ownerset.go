// SPDX-License-Identifier: Apache-2.0

package install

import "strings"

// ownerSet is a list of owner SIDs that install accepts for a path that exists before it creates
// or writes it on Windows (refuseSquatted in ops_windows.go). It lives in this untagged file so
// the comparison is tested on every build host (ownerset_test.go).
type ownerSet []string

// contains reports whether sid is in the set; SID strings compare without case.
func (s ownerSet) contains(sid string) bool {
	for _, o := range s {
		if o != "" && strings.EqualFold(o, sid) {
			return true
		}
	}
	return false
}
