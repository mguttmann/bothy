// SPDX-License-Identifier: Apache-2.0

// Package countersuser is the negative fixture of TestCountersOnlyUsedByPhaseHooks: a package
// that is neither instance, wire nor a registered phase hook and names instance.Counters, so the
// static check must report it. It lives under testdata, which the go tool skips in ./... .
package countersuser

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/instance"
)

// Bump advances event_seq outside a phase hook, exactly what the rule forbids.
func Bump(ctx context.Context, c instance.Counters) (int64, error) {
	return c.Advance(ctx, instance.CounterEventSeq, 1)
}
