// SPDX-License-Identifier: Apache-2.0

package chat

import "example.com/skeleton/internal/chat/domain"

// Expose puts a domain type into the port, which the gate rejects.
func Expose(c domain.Channel) Channel { return Channel{ID: c.ID} }
