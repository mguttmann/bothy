// SPDX-License-Identifier: Apache-2.0

// Package harness is shared test support; it may use any package of the project.
package harness

import "example.com/skeleton/internal/chat/adapters/postgres"

// Repository builds an adapter for tests.
func Repository() *postgres.Repository {
	return &postgres.Repository{}
}
