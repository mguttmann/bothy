// SPDX-License-Identifier: Apache-2.0

// Package app holds the use cases of the messaging module.
package app

import (
	"example.com/skeleton/internal/chat"
	"example.com/skeleton/internal/messaging"
)

// Post builds a message for a channel; it uses only the port of chat.
func Post(c chat.Channel, body string) messaging.Message {
	_ = c
	return messaging.Message{Body: body}
}
