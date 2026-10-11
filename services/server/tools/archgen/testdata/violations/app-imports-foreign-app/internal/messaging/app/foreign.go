// SPDX-License-Identifier: Apache-2.0

// Package app holds the use cases of the messaging module; this file imports a foreign app.
package app

import chatapp "example.com/skeleton/internal/chat/app"

// Reach calls into the app layer of chat instead of its port.
func Reach(s *chatapp.Service) bool {
	return s != nil
}
