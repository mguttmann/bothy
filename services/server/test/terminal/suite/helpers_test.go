// SPDX-License-Identifier: Apache-2.0

package suite

import "testing"

// mustRepo returns the repository of the test or fails it.
func mustRepo(t *testing.T) Repo {
	t.Helper()
	r, err := FindRepo()
	if err != nil {
		t.Fatal(err)
	}
	return r
}
