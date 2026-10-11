// SPDX-License-Identifier: Apache-2.0

// Package util is a package beside the layers of chat; no component claims it.
package util

// Trim shortens a name.
func Trim(s string) string {
	if len(s) > 8 {
		return s[:8]
	}
	return s
}
