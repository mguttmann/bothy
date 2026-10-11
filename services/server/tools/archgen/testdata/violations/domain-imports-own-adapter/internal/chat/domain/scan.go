// SPDX-License-Identifier: Apache-2.0

package domain

import "example.com/skeleton/internal/chat/adapters/clamav"

// Scanned asks an adapter from the domain layer, which the gate rejects.
func Scanned() bool { return clamav.Clean() }
