// SPDX-License-Identifier: Apache-2.0

// Package chat is the port of the chat module.
package chat

import "example.com/skeleton/internal/kernel/ids"

// Channel is the port DTO of a channel.
type Channel struct {
	ID ids.ID
}
