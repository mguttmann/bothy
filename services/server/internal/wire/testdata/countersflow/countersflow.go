// SPDX-License-Identifier: Apache-2.0

// Package countersflow is the second negative fixture of TestCountersOnlyUsedByPhaseHooks: it
// never writes the name instance.Counters but passes a value of that type on, which the static
// check must report as well.
package countersflow

import (
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

// Keep hands the counters of wire to a function of its own.
func Keep(pool *pg.Pool, sink func(any)) {
	sink(wire.InstanceCounters(pool))
}
