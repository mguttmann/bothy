// SPDX-License-Identifier: Apache-2.0

// Package noruns is the M1 binding of the outgoing port app.RunSummarySource of the snapshot: it
// returns no running run, because the table runs and the runs module exist only from M2-05.
// M2-05 binds the runs port in internal/wire instead and removes this binding.
//
// Example:
//
//	snap, err := app.NewSnapshot(app.SnapshotDeps{Runs: noruns.Source{}, ...})
package noruns

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/sync"
	"github.com/OWNER/bothy/services/server/internal/sync/app"
)

// Source implements app.RunSummarySource without runs.
type Source struct{}

// Compile-time check: Source is the outgoing port.
var _ app.RunSummarySource = Source{}

// ActiveRuns returns an empty list (never nil) and no error: before M2-05 no run exists.
func (Source) ActiveRuns(context.Context, actor.Actor, []ids.ID) ([]sync.RunSummary, error) {
	return []sync.RunSummary{}, nil
}
