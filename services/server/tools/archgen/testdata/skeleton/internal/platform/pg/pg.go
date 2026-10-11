// SPDX-License-Identifier: Apache-2.0

// Package pg is the platform package that owns the database driver.
package pg

import "github.com/jackc/pgx/v5"

// Pool wraps a driver connection.
type Pool struct {
	Conn *pgx.Conn
}
