// SPDX-License-Identifier: Apache-2.0

package domain

import "example.com/skeleton/internal/chat/domain/policy"

// NameFits reports whether a channel name is short enough.
func NameFits(name string) bool {
	return len(name) <= policy.MaxName
}
