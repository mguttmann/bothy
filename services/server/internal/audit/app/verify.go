// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"time"

	"github.com/OWNER/bothy/services/server/internal/audit"
	"github.com/OWNER/bothy/services/server/internal/audit/domain"
	"github.com/OWNER/bothy/services/server/internal/instance"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/jose"
)

// Verify implements audit.Log (verifyAuditChain, 19 SEC-AUD-3 and SEC-AUD-7):
//
//  1. authorize audit.verify, check the range (both bounds at least 1);
//  2. read the checkpoints from the sink only, never from audit_checkpoints, and check each one:
//     JWS header typ audit-checkpoint+json, signature of its kid with purpose audit_checkpoint
//     in any state, instance_id of this instance, the previous_checkpoint_hash links; a sink that
//     cannot be read is 503 (dependency_unavailable);
//  3. walk the range in batches of api.page_size_max and compare the recomputed hash at every
//     trusted checkpoint seq;
//  4. a trusted checkpoint whose row is gone (below the first retained row of a range that
//     starts there, or above the head of a range that ends there) is checkpoint_mismatch;
//  5. the failure with the smallest seq wins; record the allow entry {from_seq, to_seq, ok}.
//
// from_seq defaults to the smallest retained seq and is raised to it; to_seq defaults to the head
// and is capped at it; from above to is 400 validation_failed.
func (l *Log) Verify(ctx context.Context, req audit.VerifyRequest) (audit.VerifyResult, error) {
	if err := l.check(ctx, authorize.ActionAuditVerify); err != nil {
		return audit.VerifyResult{}, err
	}
	if err := checkRange(req); err != nil {
		return audit.VerifyResult{}, err
	}
	res := audit.VerifyResult{Warnings: []string{}}
	chain := domain.CheckpointChain{}
	if l.d.Sink.Configured() {
		var err error
		if chain, err = l.sinkChain(ctx); err != nil {
			return audit.VerifyResult{}, err
		}
	} else {
		res.Warnings = append(res.Warnings, audit.WarningSinkNotConfigured)
	}
	res.LastCheckpointSeq = chain.Last
	first, head, nonEmpty, err := l.d.Store.Bounds(ctx)
	if err != nil {
		return audit.VerifyResult{}, err
	}
	from, to, err := rangeOf(req, first, head, nonEmpty)
	if err != nil {
		return audit.VerifyResult{}, err
	}
	start, anchorFailure, anchorFailed, err := l.start(ctx, first, nonEmpty, &chain)
	if err != nil {
		return audit.VerifyResult{}, err
	}
	var rowFailure domain.Failure
	var rowFailed bool
	if nonEmpty {
		walk, err := l.walk(ctx, from, to, first, start, chain.Hashes)
		if err != nil {
			return audit.VerifyResult{}, err
		}
		rowFailure, rowFailed = walk.Failure()
		res.CheckpointsCompared = walk.Compared()
	}
	failure, failed := verdict(rowFailure, rowFailed, chain, nonEmpty, from == first, to == head, first, head)
	failure, failed = domain.FirstFailure(failure, failed, anchorFailure, anchorFailed)
	res.CheckedFromSeq, res.CheckedToSeq, res.OK = from, to, !failed
	if failed {
		res.FirstBrokenSeq, res.FailureReason = failure.Seq, string(failure.Reason)
	}
	if err := l.record(ctx, authorize.ActionAuditVerify, auditing.Attrs{
		attrFromSeq: auditing.Int(from), attrToSeq: auditing.Int(to), attrOK: auditing.Bool(res.OK),
	}); err != nil {
		// A log that cannot take a new row (a missing newest row breaks the audit hook) must still
		// show its owner the result; the failed entry is reported (docs/DECISIONS.md M1-50).
		l.d.Reporter.RecordFailed(ctx, err)
	}
	return res, nil
}

// rangeOf returns the range of req over the log first .. head: from raised to the first retained
// row, to capped at the head; from above to is 400. An empty log has the range 0 .. 0.
func rangeOf(req audit.VerifyRequest, first, head int64, nonEmpty bool) (int64, int64, error) {
	if !nonEmpty {
		return 0, 0, nil
	}
	from, to := first, head
	if req.FromSeq != nil {
		from = max(*req.FromSeq, first)
	}
	if req.ToSeq != nil {
		to = min(*req.ToSeq, head)
	}
	if from > to {
		return 0, 0, rangeError("/from_seq", "from_seq is above to_seq or above the head of the log.")
	}
	return from, to, nil
}

// checkRange refuses bounds below 1 and an explicit from above an explicit to.
func checkRange(req audit.VerifyRequest) error {
	if req.FromSeq != nil && *req.FromSeq < 1 {
		return rangeError("/from_seq", "from_seq is at least 1.")
	}
	if req.ToSeq != nil && *req.ToSeq < 1 {
		return rangeError("/to_seq", "to_seq is at least 1.")
	}
	if req.FromSeq != nil && req.ToSeq != nil && *req.FromSeq > *req.ToSeq {
		return rangeError("/from_seq", "from_seq is above to_seq.")
	}
	return nil
}

// rangeError is 400 validation_failed for the member at pointer.
func rangeError(pointer, detail string) error {
	return errs.New(errs.CodeValidationFailed, "").WithFields(errs.FieldError{Pointer: pointer, Code: "out_of_range", Detail: detail})
}

// walkStart is where a walk from the first retained row starts (domain.StartOf).
type walkStart struct {
	from int64
	prev [domain.HashSize]byte
}

// start returns where a walk from the first retained row starts: row 1 with ZeroHash, or, after a
// retention trim (M7-05), the first retained row with the hash of the newest anchor entry when
// that anchor names the row before it. After a trim the checkpoints of chain up to the anchor are
// set aside: below it nothing can be compared, at it the checkpoint must equal the anchor, else
// the returned failure is checkpoint_mismatch there. Without an anchor for the first retained
// row the walk starts at row 1, so the missing rows are a seq_gap at 1. A checkpoint set aside
// that is younger than retention.audit, or a newest checkpoint at or below the anchor, is
// checkpoint_mismatch (domain.TrimmedCheckpoints).
func (l *Log) start(ctx context.Context, first int64, nonEmpty bool, chain *domain.CheckpointChain) (walkStart, domain.Failure, bool, error) {
	var anchor domain.Anchor
	var has bool
	if nonEmpty && first > 1 {
		var err error
		if anchor, has, err = l.d.Store.LatestAnchor(ctx); err != nil {
			return walkStart{}, domain.Failure{}, false, err
		}
	}
	from, prev, trimmed := domain.StartOf(first, anchor, has)
	if !trimmed {
		return walkStart{from: from, prev: prev}, domain.Failure{}, false, nil
	}
	// A checkpoint of the sink the trim set aside must be older than retention.audit: the sink
	// keeps every checkpoint, so it proves that no row left the log before its period ended, even
	// against whoever can write an anchor entry into the database.
	var notAfter time.Time
	if l.cfg.Retention > 0 {
		notAfter = l.now().Add(-l.cfg.Retention)
	}
	var f domain.Failure
	var failed bool
	chain.Hashes, f, failed = domain.TrimmedCheckpoints(*chain, anchor, notAfter)
	return walkStart{from: from, prev: prev}, f, failed, nil
}

// walk recomputes the rows from .. to. A range that starts at the first retained row starts at
// start (row 1 with ZeroHash, or the first retained row with the hash of its anchor); a first
// retained row above 1 without its anchor is a seq_gap at 1. A range that starts later trusts the
// stored hash of row from - 1.
func (l *Log) walk(ctx context.Context, from, to, first int64, start walkStart, checkpoints map[int64][domain.HashSize]byte) (*domain.Walk, error) {
	var w *domain.Walk
	if from == first {
		w = domain.NewWalk(start.from, start.prev, true, checkpoints)
	} else {
		prev, ok, err := l.storedHash(ctx, from-1)
		if err != nil {
			return nil, err
		}
		w = domain.NewWalk(from, prev, true, checkpoints)
		if !ok {
			// Row from - 1 lies between the first retained row and the range, so it is missing:
			// the walk starts there and reports the gap.
			w = domain.NewWalk(from-1, prev, false, checkpoints)
		}
	}
	if err := l.walkRows(ctx, w, to); err != nil {
		return nil, err
	}
	return w, nil
}

// storedHash returns the stored hash of row seq and true, false when the row is missing. A hash
// of another size reads as zero, so the walk reports the tampering instead of the read failing.
func (l *Log) storedHash(ctx context.Context, seq int64) ([domain.HashSize]byte, bool, error) {
	rows, err := l.d.Store.Rows(ctx, seq, seq)
	if err != nil || len(rows) == 0 {
		return [domain.HashSize]byte{}, false, err
	}
	return rows[0].Hash, true, nil
}

// walkRows feeds the walk with the rows from its next seq up to to, in batches of
// api.page_size_max, and closes it at to.
func (l *Log) walkRows(ctx context.Context, w *domain.Walk, to int64) error {
	for !w.Failed() && w.Next() <= to {
		end := min(w.Next()+int64(l.cfg.PageSizeMax)-1, to)
		rows, err := l.d.Store.Rows(ctx, w.Next(), end)
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			break
		}
		w.Add(rows)
	}
	w.End(to)
	return nil
}

// missingCheckpointRow returns the smallest trusted checkpoint whose row is gone: below the
// first retained row when the range starts there, above the head when the range ends there, or
// any checkpoint of an empty log.
func missingCheckpointRow(cps map[int64][domain.HashSize]byte, nonEmpty, fromFirst, toHead bool, first, head int64) (domain.Failure, bool) {
	var out domain.Failure
	found := false
	for seq := range cps {
		gone := !nonEmpty || fromFirst && seq < first || toHead && seq > head
		if gone && (!found || seq < out.Seq) {
			out, found = domain.Failure{Seq: seq, Reason: domain.CheckpointMismatch}, true
		}
	}
	return out, found
}

// sinkChain reads the sink and checks its checkpoints. Errors: kind unavailable
// (dependency_unavailable) for a sink that cannot be read; the database error of a key lookup.
func (l *Log) sinkChain(ctx context.Context) (domain.CheckpointChain, error) {
	stored, err := l.d.Sink.List(ctx)
	if err != nil {
		return domain.CheckpointChain{}, errs.Wrap(errs.CodeUnavailable, err, "audit: the checkpoint sink cannot be read").
			WithCause(errs.CauseDependencyUnavailable)
	}
	cps := make([]domain.SinkCheckpoint, 0, len(stored))
	for _, s := range stored {
		payload, err := l.verifyJWS(ctx, s.JWS)
		if err != nil {
			return domain.CheckpointChain{}, err
		}
		cps = append(cps, domain.SinkCheckpoint{Seq: s.Seq, Payload: payload})
	}
	return domain.CheckCheckpoints(l.d.InstanceID, cps), nil
}

// verifyJWS returns the payload of a checkpoint JWS whose header typ is audit-checkpoint+json and
// whose signature verifies with a key of purpose audit_checkpoint, nil for any other JWS; only a
// database error is an error.
func (l *Log) verifyJWS(ctx context.Context, jws []byte) ([]byte, error) {
	if _, err := jose.ParseCompactJWS(string(jws), instance.SignatureAlgs(), domain.CheckpointType); err != nil {
		return nil, nil
	}
	payload, _, err := l.d.Keys.Verify(ctx, instance.PurposeAuditCheckpoint, jws)
	if errors.Is(err, instance.ErrJWSInvalid) {
		return nil, nil
	}
	return payload, err
}
