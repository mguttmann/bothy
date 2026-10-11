// SPDX-License-Identifier: Apache-2.0

package domain

import "example.com/skeleton/internal/chat/app/commands"

// Apply uses a type of the app layer in the domain, which the gate rejects.
func Apply(c commands.Rename) string { return c.Name }
