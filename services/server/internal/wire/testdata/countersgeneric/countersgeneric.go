// SPDX-License-Identifier: Apache-2.0

// Package countersgeneric is the fourth negative fixture of TestCountersOnlyUsedByPhaseHooks: a
// generic function receives the counters through type parameters, so neither the name
// instance.Counters nor CounterName appears; the instantiation with Counters must be reported.
package countersgeneric

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

// adv has the shape of Counters.Advance with the counter name as a type parameter.
type adv[N any] interface {
	Advance(ctx context.Context, name N, n int64) (int64, error)
}

// call builds the counters with build and advances name.
func call[F ~func(*pg.Pool) C, C adv[N], N ~string](ctx context.Context, build F, pool *pg.Pool, name N) (int64, error) {
	return build(pool).Advance(ctx, name, 1)
}

// Bump advances event_seq outside a phase hook.
func Bump(ctx context.Context, pool *pg.Pool) (int64, error) {
	return call(ctx, wire.InstanceCounters, pool, "event_seq")
}
