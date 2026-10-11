// SPDX-License-Identifier: Apache-2.0

package postgres

import "example.com/skeleton/internal/chat/adapters/postgres/sqlcgen"

// Store is the repository on top of the generated queries.
type Store struct {
	Q *sqlcgen.Queries
}
