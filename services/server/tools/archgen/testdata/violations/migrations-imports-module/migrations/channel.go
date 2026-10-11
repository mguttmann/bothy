// SPDX-License-Identifier: Apache-2.0

package migrations

import "example.com/skeleton/internal/chat"

// Owner uses a module port, which the migrations package must never do.
var Owner = chat.Channel{}
