// SPDX-License-Identifier: Apache-2.0

package app

import "example.com/skeleton/internal/chat/app/commands"

// Forward reaches into a sub-package of the chat app layer instead of its port.
func Forward(c commands.Rename) string { return c.Name }
