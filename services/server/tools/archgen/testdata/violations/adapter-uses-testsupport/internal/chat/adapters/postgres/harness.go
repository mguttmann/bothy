// SPDX-License-Identifier: Apache-2.0

// Package postgres is the database adapter of chat; this file imports test support.
package postgres

import "example.com/skeleton/test/harness"

// Server names the test harness from production code, which the gate rejects.
func Server() string {
	return harness.Name
}
