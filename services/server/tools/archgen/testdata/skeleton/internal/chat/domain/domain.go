// SPDX-License-Identifier: Apache-2.0

// Package domain holds the entities of the chat module.
package domain

import "example.com/skeleton/internal/kernel/ids"

// Channel is the domain entity.
type Channel struct {
	ID   ids.ID
	Name string
}
