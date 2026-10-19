// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"context"
	"crypto/rand"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/status"
	statuspg "github.com/OWNER/bothy/services/server/internal/status/adapters/postgres"
)

// ErrNoBackupTable is status.ErrNoBackupTable for the commands, which import wire only: the
// schema has no table backup_runs yet.
var ErrNoBackupTable = status.ErrNoBackupTable

// LastBackupSuccess returns the started_at of the newest succeeded run of kind pg_full, pg_diff
// or logical in backup_runs and true, or false when there is none (docs/spec/16-betrieb.md 10
// item 1, the check of bothy migrate up). pool may connect as any role that may read the table,
// bothy migrate uses bothy_owner. It returns ErrNoBackupTable when the schema has no table
// backup_runs.
func LastBackupSuccess(ctx context.Context, pool *pg.Pool) (time.Time, bool, error) {
	// The clock and the id generator serve Start and Finish only; this read uses neither.
	runs := statuspg.NewRuns(pool, clock.System(), ids.NewGenerator(clock.System(), rand.Reader))
	return runs.LastSuccess(ctx, status.BackupPGFull, status.BackupPGDiff, status.BackupLogical)
}
