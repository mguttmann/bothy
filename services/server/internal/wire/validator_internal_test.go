// SPDX-License-Identifier: Apache-2.0

package wire

import "testing"

// TestEnabledValidatorBuiltOnce: the request validator of the enabled operations is compiled once
// per process, so every chain of a test binary (and the one of bothy serve) shares it.
func TestEnabledValidatorBuiltOnce(t *testing.T) {
	t.Parallel()
	a, err := enabledValidator()
	if err != nil {
		t.Fatal(err)
	}
	b, err := enabledValidator()
	if err != nil || a != b {
		t.Fatalf("second call built another validator (%p, %p) or failed: %v", a, b, err)
	}
}
