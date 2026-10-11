// SPDX-License-Identifier: Apache-2.0

// Package pgtest is the test helper of the platform package pg.
package pgtest

import "example.com/skeleton/internal/platform/pg"

// Pool returns a pool for tests.
func Pool() *pg.Pool {
	return &pg.Pool{}
}
