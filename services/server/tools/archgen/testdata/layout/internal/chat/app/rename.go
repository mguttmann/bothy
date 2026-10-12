// SPDX-License-Identifier: Apache-2.0

package app

import "example.com/skeleton/internal/chat/app/commands"

// Rename runs the rename command.
func (s *Service) Rename(c commands.Rename) string {
	return c.Name
}
