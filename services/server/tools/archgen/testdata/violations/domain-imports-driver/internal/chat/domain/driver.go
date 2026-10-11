// SPDX-License-Identifier: Apache-2.0

// Package domain holds the entities of the chat module; this file breaks the layer rule.
package domain

import "github.com/jackc/pgx/v5"

// Load reads a channel straight from the driver, which the domain layer must never do.
func Load(c *pgx.Conn) Channel {
	_ = c
	return Channel{}
}
