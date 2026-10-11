// SPDX-License-Identifier: Apache-2.0

// Package app holds the use cases of the chat module.
package app

import (
	"example.com/skeleton/internal/chat"
	"example.com/skeleton/internal/chat/domain"
	"example.com/skeleton/internal/kernel/txn"
)

// Service runs the chat use cases.
type Service struct {
	Tx txn.Runner
}

// Expose maps a domain channel to the port DTO.
func (s *Service) Expose(c domain.Channel) chat.Channel {
	return chat.Channel{ID: c.ID}
}
