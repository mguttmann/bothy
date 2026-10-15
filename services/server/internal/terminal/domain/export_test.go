// SPDX-License-Identifier: Apache-2.0

package domain

// CrockfordForTest exposes the encoder of the fingerprint to the tests of the package.
func CrockfordForTest(b []byte) string { return crockford(b) }
