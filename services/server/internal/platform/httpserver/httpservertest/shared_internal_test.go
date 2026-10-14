// SPDX-License-Identifier: Apache-2.0

package httpservertest

import "testing"

// TestSharedRequestValidator: harnesses over the same contract and operations share one request
// validator, built once per process; another operation list gets its own.
func TestSharedRequestValidator(t *testing.T) {
	t.Parallel()
	a, err := sharedRequestValidator(Contract, []string{"getThing", "updateThing"})
	if err != nil {
		t.Fatal(err)
	}
	b, err := sharedRequestValidator(Contract, []string{"updateThing", "getThing"})
	if err != nil {
		t.Fatal(err)
	}
	if a != b {
		t.Fatal("the same contract and operations built a second validator")
	}
	c, err := sharedRequestValidator(Contract, []string{"getThing"})
	if err != nil || c == a {
		t.Fatalf("another operation list shares the validator: %v", err)
	}
	if _, err := sharedRequestValidator(Contract, []string{"unknownOperation"}); err == nil {
		t.Fatal("an unknown operation was accepted")
	}
}
