// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// Pending returns the buffered events of the unit of ctx, for the tests of the appender.
func Pending(ctx context.Context) []PendingEvent {
	v, _ := txn.Load(ctx, pending)
	return v
}
