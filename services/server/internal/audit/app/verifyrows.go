// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/audit"
	"github.com/OWNER/bothy/services/server/internal/audit/domain"
	"github.com/OWNER/bothy/services/server/internal/instance"
	"github.com/OWNER/bothy/services/server/internal/kernel/jose"
)

// rowsBatch is how many rows VerifyRows hands to the walk at once.
const rowsBatch = 500

// RowsVerifier implements audit.RowsVerifier with VerifyRows. It holds no state.
type RowsVerifier struct{}

// Compile-time check: RowsVerifier is the port of the chain verification over supplied rows.
var _ audit.RowsVerifier = RowsVerifier{}

// VerifyRows implements audit.RowsVerifier; see VerifyRows.
func (RowsVerifier) VerifyRows(ctx context.Context, in audit.RowsInput) (audit.VerifyResult, error) {
	return VerifyRows(ctx, in)
}

// VerifyRows verifies the chain of in like Log.Verify verifies the whole log (verifyAuditChain,
// 19 SEC-AUD-3), with the checkpoints of audit_checkpoints in place of those of the sink
// (brief M7-03 design note 2):
//
//  1. every checkpoint is checked as Log.Verify checks those of the sink: the JWS header typ
//     audit-checkpoint+json, the signature of the key its kid names among in.Keys (purpose
//     audit_checkpoint, same algorithm), the payload of in.InstanceID with the seq and hash of its
//     row of audit_checkpoints, and the previous_checkpoint_hash links (domain.CheckCheckpoints);
//     the first one that fails is checkpoint_invalid and none after it is trusted;
//  2. the rows are walked from row 1 with ZeroHash to the last row (domain.Walk, as Log.walk for
//     a range from the first retained row: until the anchors of M7 no row may be missing at the
//     start), comparing the recomputed hash at every trusted checkpoint;
//  3. a trusted checkpoint whose row is gone is checkpoint_mismatch, a row below 1 is seq_gap;
//     the failure with the smallest seq wins (verdict).
//
// The rows are read once, in batches, so a log of any length needs memory for one batch only.
// The result carries no warnings: audit_checkpoints is the only checkpoint store of a copy.
//
// Errors: the first error an iterator yields; ctx ending between batches.
//
// Example:
//
//	res, err := app.VerifyRows(ctx, audit.RowsInput{InstanceID: id, Keys: keys, Events: rows, Checkpoints: cps})
func VerifyRows(ctx context.Context, in audit.RowsInput) (audit.VerifyResult, error) {
	chain, err := rowCheckpoints(in)
	if err != nil {
		return audit.VerifyResult{}, err
	}
	walk := domain.NewWalk(1, domain.ZeroHash, true, chain.Hashes)
	b, err := walkEvents(ctx, walk, in)
	if err != nil {
		return audit.VerifyResult{}, err
	}
	res := audit.VerifyResult{Warnings: []string{}, LastCheckpointSeq: chain.Last}
	var rowFailure domain.Failure
	var rowFailed bool
	if b.nonEmpty {
		walk.End(b.head)
		rowFailure, rowFailed = walk.Failure()
		res.CheckpointsCompared = walk.Compared()
		res.CheckedFromSeq, res.CheckedToSeq = b.first, b.head
	}
	failure, failed := verdict(rowFailure, rowFailed, chain, b.nonEmpty, true, true, b.first, b.head)
	res.OK = !failed
	if failed {
		res.FirstBrokenSeq, res.FailureReason = failure.Seq, string(failure.Reason)
	}
	return res, nil
}

// rowBounds are the first and the last seq the events of a RowsInput yielded.
type rowBounds struct {
	first, head int64
	nonEmpty    bool
}

// walkEvents feeds the events of in to walk in batches of rowsBatch and returns their bounds.
// After the first failure the rows are still read for the bounds, but no longer walked.
func walkEvents(ctx context.Context, walk *domain.Walk, in audit.RowsInput) (rowBounds, error) {
	var b rowBounds
	batch := make([]domain.Row, 0, rowsBatch)
	flush := func() error {
		walk.Add(batch)
		batch = batch[:0]
		return ctx.Err()
	}
	if in.Events == nil {
		return b, nil
	}
	for e, err := range in.Events {
		if err != nil {
			return rowBounds{}, err
		}
		if !b.nonEmpty {
			b.first, b.nonEmpty = e.Seq, true
		}
		b.head = e.Seq
		if walk.Failed() {
			continue
		}
		batch = append(batch, RowOf(e))
		if len(batch) == rowsBatch {
			if err := flush(); err != nil {
				return rowBounds{}, err
			}
		}
	}
	if err := flush(); err != nil {
		return rowBounds{}, err
	}
	return b, nil
}

// verdict merges the failures of a verification as Log.Verify does: the walk, a trusted
// checkpoint whose row is gone, the first invalid checkpoint and a row below 1; the smallest seq
// wins.
func verdict(rowFailure domain.Failure, rowFailed bool, chain domain.CheckpointChain, nonEmpty, fromFirst, toHead bool, first, head int64) (domain.Failure, bool) {
	missing, hasMissing := missingCheckpointRow(chain.Hashes, nonEmpty, fromFirst, toHead, first, head)
	failure, failed := domain.FirstFailure(rowFailure, rowFailed, missing, hasMissing)
	failure, failed = domain.FirstFailure(failure, failed, domain.Failure{Seq: chain.InvalidSeq, Reason: domain.CheckpointInvalid}, chain.Invalid)
	// A row below 1 lies outside the chain, which starts at row 1 (see Log.walk): never checked,
	// so never ok.
	return domain.FirstFailure(failure, failed, domain.Failure{Seq: first, Reason: domain.SeqGap}, nonEmpty && first < 1)
}

// rowCheckpoints reads the checkpoints of in and checks them like those of the sink.
func rowCheckpoints(in audit.RowsInput) (domain.CheckpointChain, error) {
	keys := map[string]audit.VerificationKey{}
	for _, k := range in.Keys {
		if k.Purpose == audit.PurposeAuditCheckpoint {
			keys[k.KID] = k
		}
	}
	var cps []domain.SinkCheckpoint
	if in.Checkpoints != nil {
		for c, err := range in.Checkpoints {
			if err != nil {
				return domain.CheckpointChain{}, err
			}
			cps = append(cps, domain.SinkCheckpoint{Seq: c.Seq, Payload: checkpointPayload(c, keys)})
		}
	}
	return domain.CheckCheckpoints(in.InstanceID, cps), nil
}

// checkpointPayload returns the payload of the checkpoint row c when its JWS verifies under the
// key its header names and the payload covers the seq and hash of the row; nil otherwise, which
// CheckCheckpoints reports as checkpoint_invalid.
func checkpointPayload(c audit.CheckpointRow, keys map[string]audit.VerificationKey) []byte {
	j, err := jose.ParseCompactJWS(string(c.Signature), instance.SignatureAlgs(), domain.CheckpointType)
	if err != nil {
		return nil
	}
	k, ok := keys[j.Header.Kid]
	if !ok || j.Header.Kid == "" || k.Alg != j.Header.Alg {
		return nil
	}
	jwk, err := jose.ParseJWK(k.PublicJWK)
	if err != nil || j.Verify(jwk) != nil {
		return nil
	}
	p, err := domain.ParseCheckpoint(j.Payload)
	if err != nil || p.Seq != c.Seq || p.Hash != c.Hash {
		return nil
	}
	return j.Payload
}
