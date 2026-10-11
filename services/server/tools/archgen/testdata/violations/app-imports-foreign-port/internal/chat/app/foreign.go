// SPDX-License-Identifier: Apache-2.0

// Package app holds the use cases of the chat module; this file imports a foreign port.
package app

import "example.com/skeleton/internal/messaging"

// Quote uses the port of messaging, a module that chat may not depend on.
func Quote(m messaging.Message) string {
	return m.Body
}
