// SPDX-License-Identifier: Apache-2.0

package app

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"

	"github.com/OWNER/bothy/services/server/internal/audit/domain"
	"github.com/OWNER/bothy/services/server/internal/instance"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/jose"
)

// errCheckpointRace ends the insert of a checkpoint whose predecessor changed between the walk
// and the lock: another run signed first, and this run must not fork the chain.
var errCheckpointRace = errors.New("audit: another run inserted a checkpoint")

// RunCheckpoint is one run of the job audit.chain_verify (02 section 12.3, 19 SEC-AUD-2):
//
//  1. without a sink it reports SinkNotConfigured (every run);
//  2. it reads the sink first, then checks the last checkpoint of audit_checkpoints (signature,
//     payload, its row), that the sink holds no newer checkpoint (a deleted database checkpoint)
//     and that no row lies below 1, then recomputes the chain from the row after it (or from
//     row 1) to the head; on any failure
//     it adds one to bothy.audit.verify.failures, records audit.verify with decision error,
//     details {first_broken_seq, failure_reason} and no actor, reports VerifyFailed, signs
//     nothing and returns nil: River must not retry a detected break, the alert
//     BothyAuditChainBroken takes over;
//  3. when the head moved it signs the next checkpoint before the transaction (the key is read
//     from the key store) and inserts it in one transaction under the checkpoint lock; a
//     checkpoint of a concurrent run in between makes it skip the insert;
//  4. after the commit it checks every checkpoint with exported_at NULL like step 2, hands it to
//     the sink in seq order and sets exported_at; an invalid checkpoint and a sink that holds
//     other content under its seq are breaks as in step 2, any other sink error fails the run
//     (River retries); nothing is signed twice;
//  5. it sets bothy.audit.verify.last_success.timestamp.
//
// Errors: storage, signing and sink errors, for the retry of River.
func (l *Log) RunCheckpoint(ctx context.Context) error {
	if !l.d.Sink.Configured() {
		l.d.Reporter.SinkNotConfigured(ctx)
	}
	// The sink is read before the last checkpoint of the database: a checkpoint reaches the sink
	// only after its row committed, so every checkpoint listed here is in the database read next
	// unless it was deleted. Read the other way round, a run that overlaps another run's export
	// would see the sink ahead of its stale database checkpoint and report a false break.
	newest, err := l.sinkNewest(ctx)
	if err != nil {
		return err
	}
	last, hasLast, err := l.d.Store.LastCheckpoint(ctx)
	if err != nil {
		return err
	}
	head, broken, err := l.verifySinceCheckpoint(ctx, last, hasLast, newest)
	if err != nil {
		return err
	}
	if broken != nil {
		return l.chainBroken(ctx, *broken)
	}
	if head != nil {
		if err := l.signCheckpoint(ctx, *head, last, hasLast); err != nil {
			return err
		}
	}
	broken, err = l.exportCheckpoints(ctx)
	if err != nil {
		return err
	}
	if broken != nil {
		return l.chainBroken(ctx, *broken)
	}
	l.d.Metrics.VerifySucceeded(ctx, l.now())
	return nil
}

// verifySinceCheckpoint checks the last checkpoint and walks the rows after it. It returns the
// head row when the walk reached new rows, or the failure.
func (l *Log) verifySinceCheckpoint(ctx context.Context, last StoredCheckpoint, hasLast bool, sinkNewest int64) (*domain.Row, *domain.Failure, error) {
	first, head, ok, err := l.d.Store.Bounds(ctx)
	if err != nil {
		return nil, nil, err
	}
	if ok && first < 1 {
		// A row below 1 lies outside the chain, which starts at row 1 (see Log.walk).
		return nil, &domain.Failure{Seq: first, Reason: domain.SeqGap}, nil
	}
	if sinkNewest > 0 && (!hasLast || sinkNewest > last.Seq) {
		// The database lost checkpoints it had signed and exported.
		return nil, &domain.Failure{Seq: sinkNewest, Reason: domain.CheckpointMismatch}, nil
	}
	// Without a checkpoint the chain starts at row 1 (see walk); the trim of M7-05 never removes
	// rows while no checkpoint was exported, so then no row is missing.
	w := domain.NewWalk(1, domain.ZeroHash, true, nil)
	if hasLast {
		if f, err := l.checkStored(ctx, last, head, ok); f != nil || err != nil {
			return nil, f, err
		}
		w = domain.NewWalk(last.Seq+1, last.Hash, true, nil)
	}
	if !ok || w.Next() > head {
		return nil, nil, nil
	}
	if err := l.walkRows(ctx, w, head); err != nil {
		return nil, nil, err
	}
	if f, failed := w.Failure(); failed {
		return nil, &f, nil
	}
	row, _ := w.Last()
	return &row, nil, nil
}

// sinkNewest returns the largest seq of the sink, 0 without a sink or without checkpoints. A
// checkpoint of the sink newer than the last one of audit_checkpoints means the database lost
// checkpoints it had signed and exported (checkpoint_mismatch at the newest one of the sink).
// Errors: a sink that cannot be read.
func (l *Log) sinkNewest(ctx context.Context) (int64, error) {
	if !l.d.Sink.Configured() {
		return 0, nil
	}
	stored, err := l.d.Sink.List(ctx)
	if err != nil {
		return 0, err
	}
	var newest int64
	for _, c := range stored {
		newest = max(newest, c.Seq)
	}
	return newest, nil
}

// checkStored checks a checkpoint of audit_checkpoints before the job trusts or exports it: the
// signature with a key of purpose audit_checkpoint, the payload of this instance naming the
// stored seq and hash, and the row it covers still in the log with that hash.
func (l *Log) checkStored(ctx context.Context, c StoredCheckpoint, head int64, nonEmpty bool) (*domain.Failure, error) {
	payload, err := l.verifyJWS(ctx, c.Signature)
	if err != nil {
		return nil, err
	}
	p, perr := domain.ParseCheckpoint(payload)
	if payload == nil || perr != nil || p.InstanceID != l.d.InstanceID || p.Seq != c.Seq || p.Hash != c.Hash {
		return &domain.Failure{Seq: c.Seq, Reason: domain.CheckpointInvalid}, nil
	}
	if !nonEmpty || c.Seq > head {
		return &domain.Failure{Seq: c.Seq, Reason: domain.CheckpointMismatch}, nil
	}
	h, found, err := l.storedHash(ctx, c.Seq)
	if err != nil {
		return nil, err
	}
	if !found || h != c.Hash {
		return &domain.Failure{Seq: c.Seq, Reason: domain.CheckpointMismatch}, nil
	}
	return nil, nil
}

// chainBroken handles a detected break: count it, report it, record it, end the run without
// error and without a checkpoint. An entry that cannot be written (a missing newest row breaks
// the audit hook) is reported and does not make River retry: the counter has already counted.
func (l *Log) chainBroken(ctx context.Context, f domain.Failure) error {
	l.d.Metrics.VerifyFailed(ctx)
	l.d.Reporter.VerifyFailed(ctx, f.Reason)
	if err := l.d.Audit.RecordDetached(ctx, auditing.Entry{
		Action: string(authorize.ActionAuditVerify), Decision: auditing.DecisionError,
		ResourceKind: string(authorize.ResourceInstance),
		Attrs:        auditing.Attrs{attrBrokenSeq: auditing.Int(f.Seq), attrReason: auditing.Code(string(f.Reason))},
	}); err != nil {
		l.d.Reporter.RecordFailed(ctx, err)
	}
	return nil
}

// signCheckpoint signs the checkpoint over head, linked to last, and inserts it.
func (l *Log) signCheckpoint(ctx context.Context, head domain.Row, last StoredCheckpoint, hasLast bool) error {
	var previous []byte
	if hasLast {
		p, err := jwsPayload(last.Signature)
		if err != nil {
			return errs.Wrap(errs.CodeInternalError, err, "audit: the payload of the last checkpoint")
		}
		previous = p
	}
	c := domain.NewCheckpoint(l.d.InstanceID, head, previous)
	payload, err := c.Canonical()
	if err != nil {
		return errs.Wrap(errs.CodeInternalError, err, "audit: checkpoint payload")
	}
	signer, err := l.d.Keys.Signer(ctx, instance.PurposeAuditCheckpoint)
	if err != nil {
		return err
	}
	jws, err := jose.SignCompact(jose.SignerFunc{Algorithm: signer.Alg(), Fn: func(in []byte) ([]byte, error) { return signer.Sign(ctx, in) }},
		jose.Header{Typ: domain.CheckpointType, Kid: signer.KeyID()}, payload)
	if err != nil {
		return err
	}
	row := StoredCheckpoint{Seq: c.Seq, Hash: c.Hash, Previous: c.Previous, HasPrevious: c.HasPrevious, Signature: []byte(jws), KID: signer.KeyID()}
	err = l.d.UoW.Do(ctx, func(ctx context.Context) error {
		if err := l.d.Store.LockCheckpoints(ctx); err != nil {
			return err
		}
		cur, has, err := l.d.Store.LastCheckpoint(ctx)
		if err != nil {
			return err
		}
		if has != hasLast || has && cur.Seq != last.Seq {
			return errCheckpointRace
		}
		return l.d.Store.InsertCheckpoint(ctx, row)
	})
	if errors.Is(err, errCheckpointRace) {
		return nil
	}
	return err
}

// exportCheckpoints hands the checkpoints without exported_at to the sink in seq order, batch by
// batch of api.page_size_max, after checking each one (checkStored) against the head read after
// its batch, and marks each one after the sink took it. It returns the failure of a checkpoint
// that does not check or whose seq holds other content in the sink.
func (l *Log) exportCheckpoints(ctx context.Context) (*domain.Failure, error) {
	if !l.d.Sink.Configured() {
		return nil, nil
	}
	for {
		pending, err := l.d.Store.UnexportedCheckpoints(ctx, l.cfg.PageSizeMax)
		if err != nil {
			return nil, err
		}
		// The head is read after the batch: a checkpoint is inserted only after its row committed,
		// so every checkpoint of the batch lies at or below this head unless rows were deleted. A
		// head read before the loop would put the checkpoint of an overlapping run above it.
		_, head, nonEmpty, err := l.d.Store.Bounds(ctx)
		if err != nil {
			return nil, err
		}
		for _, c := range pending {
			if f, err := l.exportOne(ctx, c, head, nonEmpty); f != nil || err != nil {
				return f, err
			}
		}
		if len(pending) < l.cfg.PageSizeMax {
			return nil, nil
		}
	}
}

// exportOne checks c, hands it to the sink and marks it exported.
func (l *Log) exportOne(ctx context.Context, c StoredCheckpoint, head int64, nonEmpty bool) (*domain.Failure, error) {
	if f, err := l.checkStored(ctx, c, head, nonEmpty); f != nil || err != nil {
		return f, err
	}
	err := l.d.Sink.Put(ctx, SignedCheckpoint{Seq: c.Seq, JWS: c.Signature})
	if errors.Is(err, ErrCheckpointConflict) {
		return &domain.Failure{Seq: c.Seq, Reason: domain.CheckpointMismatch}, nil
	}
	if err != nil {
		return nil, err
	}
	return nil, l.d.Store.MarkExported(ctx, c.Seq, l.now())
}

// jwsPayload returns the decoded payload segment of a compact JWS.
func jwsPayload(jws []byte) ([]byte, error) {
	parts := bytes.Split(jws, []byte("."))
	if len(parts) != 3 {
		return nil, fmt.Errorf("audit: a JWS has three segments, not %d", len(parts))
	}
	return base64.RawURLEncoding.DecodeString(string(parts[1]))
}
