// SPDX-License-Identifier: Apache-2.0

//go:build unix

package app_test

import (
	"context"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/audit"
	auditpg "github.com/OWNER/bothy/services/server/internal/audit/adapters/postgres"
	"github.com/OWNER/bothy/services/server/internal/audit/app"
	"github.com/OWNER/bothy/services/server/internal/audit/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

// trimOf returns the trim of e: units of bothy_maint over the throwaway database, the audit hook
// over the pool of bothy_app.
func trimOf(t *testing.T, e *pgEnv) *app.Trim {
	t.Helper()
	trim, err := app.NewTrim(maintUnits(t, e), auditpg.NewRepository(e.pool), e.audit.Recorder())
	if err != nil {
		t.Fatal(err)
	}
	return trim
}

// maintUnits returns units of work as bothy_maint over the database of e.
func maintUnits(t *testing.T, e *pgEnv) *wire.MaintUnits {
	t.Helper()
	open := func(context.Context) (*pg.Pool, error) { return e.db.OpenPool(t, e.db.Config(pg.RoleMaint)), nil }
	units, err := wire.NewMaintUnits(open, pg.UnitConfig{StatementTimeout: 30 * time.Second, LockTimeout: 10 * time.Second,
		IdleInTxTimeout: 60 * time.Second}, app.NewChain(wire.InstanceCounters(e.pool), auditpg.NewRepository(e.pool), e.rep))
	if err != nil {
		t.Fatal(err)
	}
	return units
}

func (e *pgEnv) bounds() (first, head int64) {
	e.t.Helper()
	if err := e.superConn.QueryRow(context.Background(), `SELECT min(seq), max(seq) FROM audit_events`).Scan(&first, &head); err != nil {
		e.t.Fatal(err)
	}
	return first, head
}

// TestTrimKeepsTheChainVerifiable (M7-05, 03 audit_events, SEC-AUD-2): the trim as bothy_maint
// removes the rows before the newest checkpoint and the checkpoints before them in one
// transaction with its anchor entry; audit.verify against the sink, the verification of a copy
// (VerifyRows) and the job audit.chain_verify all still pass and start at the anchor; batches
// report more until the cap; a row removed without an anchor is still a seq_gap.
func TestTrimKeepsTheChainVerifiable(t *testing.T) {
	t.Parallel()
	e := newPGEnv(t, true, false)
	e.write(5)
	if err := e.log.RunCheckpoint(context.Background()); err != nil {
		t.Fatal(err)
	}
	e.write(3)
	if err := e.log.RunCheckpoint(context.Background()); err != nil {
		t.Fatal(err)
	}
	e.write(2)
	var newest int64
	if err := e.superConn.QueryRow(context.Background(), `SELECT max(seq) FROM audit_checkpoints`).Scan(&newest); err != nil {
		t.Fatal(err)
	}
	trim := trimOf(t, e)
	future := time.Now().Add(400 * 24 * time.Hour)

	// Nothing occurred before a cutoff in the past.
	if n, more, err := trim.Purge(context.Background(), time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC), 100); err != nil || n != 0 || more {
		t.Fatalf("trim before every row: %d, %v, %v", n, more, err)
	}
	n, more, err := trim.Purge(context.Background(), future, 2)
	if err != nil || n != 2 || !more {
		t.Fatalf("first batch: %d, %v, %v; want 2, more", n, more, err)
	}
	for more {
		if _, more, err = trim.Purge(context.Background(), future, 2); err != nil {
			t.Fatal(err)
		}
	}
	first, _ := e.bounds()
	if first != newest {
		t.Fatalf("first retained row %d, want the row of the newest checkpoint %d", first, newest)
	}
	var left int
	if err := e.superConn.QueryRow(context.Background(), `SELECT count(*) FROM audit_checkpoints WHERE seq < $1`, first).Scan(&left); err != nil || left != 0 {
		t.Fatalf("checkpoints below the anchor: %d, %v", left, err)
	}
	res, err := e.log.Verify(e.ctx(), audit.VerifyRequest{})
	if err != nil || !res.OK || res.CheckedFromSeq != first || res.CheckpointsCompared == 0 {
		t.Fatalf("Log.Verify after the trim: %+v, %v", res, err)
	}
	rows, err := app.RowsVerifier{}.VerifyRows(context.Background(), copyInput(t, e))
	if err != nil || !rows.OK || rows.CheckedFromSeq != first {
		t.Fatalf("VerifyRows after the trim: %+v, %v", rows, err)
	}
	e.write(2)
	if err := e.log.RunCheckpoint(context.Background()); err != nil || e.metrics.failures != 0 {
		t.Fatalf("audit.chain_verify after the trim: %v, %+v", err, e.metrics)
	}

	// The anchor must name the row before the first retained one: a row deleted behind its back
	// is a seq_gap at 1.
	e.tamper("audit_events", `DELETE FROM audit_events WHERE seq = $1`, first)
	res, err = e.log.Verify(e.ctx(), audit.VerifyRequest{})
	if err != nil || res.OK || res.FailureReason != string(domain.SeqGap) || res.FirstBrokenSeq != 1 {
		t.Fatalf("Log.Verify after a deletion without anchor: %+v, %v", res, err)
	}
}

// TestTrimNeverWithoutExportedCheckpoint: without a checkpoint at audit.checkpoint_sink (here no
// sink at all, so a checkpoint is signed but never exported) the trim removes nothing, so no row
// leaves before a checkpoint outside the database sealed it (M7-05 red team).
func TestTrimNeverWithoutExportedCheckpoint(t *testing.T) {
	t.Parallel()
	e := newPGEnv(t, false, false)
	e.write(4)
	if err := e.log.RunCheckpoint(context.Background()); err != nil {
		t.Fatal(err)
	}
	if n, more, err := trimOf(t, e).Purge(context.Background(), time.Now().Add(400*24*time.Hour), 10); err != nil || n != 0 || more {
		t.Fatalf("trim without a checkpoint: %d, %v, %v", n, more, err)
	}
}

// TestForgedAnchorIsDetected (M7-05 red team): whoever holds bothy_maint writes a correctly chained
// anchor entry and erases the log up to the head with its checkpoints; the sink still holds the newest
// checkpoint, whose row is gone, so audit.verify reports checkpoint_mismatch instead of OK.
func TestForgedAnchorIsDetected(t *testing.T) {
	t.Parallel()
	e := newPGEnv(t, true, false)
	e.write(5)
	if err := e.log.RunCheckpoint(context.Background()); err != nil {
		t.Fatal(err)
	}
	e.write(2)
	_, head := e.bounds()
	last := head - 1 // the hook links the anchor to the head, which stays
	repo := auditpg.NewRepository(e.pool)
	if err := maintUnits(t, e).Do(context.Background(), func(ctx context.Context) error {
		h, _, err := repo.HashAt(ctx, last)
		if err != nil {
			return err
		}
		if err := e.audit.Recorder().Record(ctx, auditing.Entry{Action: domain.ActionRetentionTrim, Decision: auditing.DecisionAllow,
			Attrs: auditing.Attrs{domain.DetailAnchorSeq: auditing.Int(last), domain.DetailAnchorHash: auditing.Code(domain.HashText(h))}}); err != nil {
			return err
		}
		if _, err := repo.TrimEvents(ctx, 1, last); err != nil {
			return err
		}
		_, err = repo.TrimCheckpoints(ctx, last)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	res, err := e.log.Verify(e.ctx(), audit.VerifyRequest{})
	if err != nil || res.OK || res.FailureReason != string(domain.CheckpointMismatch) {
		t.Fatalf("Log.Verify after an erased log with a forged anchor: %+v, %v", res, err)
	}
}
