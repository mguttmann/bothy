// SPDX-License-Identifier: Apache-2.0

// Package chattest is the test helper of the chat module that tests of several packages share.
package chattest

import (
	"example.com/skeleton/internal/chat"
	"example.com/skeleton/internal/chat/adapters/postgres"
)

// Channel builds a port DTO and a repository for tests.
func Channel() (chat.Channel, *postgres.Repository) {
	return chat.Channel{}, &postgres.Repository{}
}
