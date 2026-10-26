// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"fmt"

	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/status/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/status/app"
	"github.com/OWNER/bothy/services/server/internal/status/domain"
)

// KeptRuns implements app.KeptRunsReader over backup_runs as bothy_app, which may read the table
// (M7-07). It is safe for concurrent use.
type KeptRuns struct {
	pool *pg.Pool
}

// Compile-time check.
var _ app.KeptRunsReader = (*KeptRuns)(nil)

// NewKeptRuns returns the reader over pool.
func NewKeptRuns(pool *pg.Pool) *KeptRuns { return &KeptRuns{pool: pool} }

// SucceededRuns implements app.KeptRunsReader.
func (r *KeptRuns) SucceededRuns(ctx context.Context) ([]domain.KeptRun, error) {
	rows, err := sqlcgen.New(pg.Querier(ctx, r.pool)).SucceededBackupRuns(ctx)
	if err != nil {
		return nil, fmt.Errorf("status: reading the succeeded backup runs: %w", err)
	}
	out := make([]domain.KeptRun, 0, len(rows))
	for _, row := range rows {
		out = append(out, domain.KeptRun{
			Kind: domain.Kind(row.Kind), StartedAt: row.StartedAt.Time.UTC(), FinishedAt: row.FinishedAt.Time.UTC(), JournalNext: row.JournalNext,
		})
	}
	return out, nil
}
