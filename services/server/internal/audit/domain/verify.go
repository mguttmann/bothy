// SPDX-License-Identifier: Apache-2.0

package domain

// FailureReason is AuditVerifyResult.failure_reason: why a chain did not verify.
type FailureReason string

// The failure reasons of audit.verify (docs/api/openapi.yaml AuditVerifyResult).
const (
	// HashMismatch: the stored hash of a row differs from the recomputed one, or the row holds a
	// value outside the canonical form.
	HashMismatch FailureReason = "hash_mismatch"
	// LinkMismatch: prev_hash of a row differs from the stored hash of the row before it.
	LinkMismatch FailureReason = "link_mismatch"
	// SeqGap: a row is missing; the failure names the missing seq.
	SeqGap FailureReason = "seq_gap"
	// CheckpointMismatch: the recomputed hash at the seq of a trusted checkpoint differs from the
	// checkpoint, or the row of a checkpoint is gone.
	CheckpointMismatch FailureReason = "checkpoint_mismatch"
	// CheckpointInvalid: a checkpoint of the sink does not verify (signature, instance, link).
	CheckpointInvalid FailureReason = "checkpoint_invalid"
)

// Failure is the first failure of a verification: the seq it names and the reason.
type Failure struct {
	// Seq is first_broken_seq.
	Seq int64
	// Reason is failure_reason.
	Reason FailureReason
}

// FirstFailure returns the failure with the smaller seq of a (when hasA) and b (when hasB); a
// wins a tie. ok is false when there is none.
func FirstFailure(a Failure, hasA bool, b Failure, hasB bool) (Failure, bool) {
	switch {
	case hasA && hasB && b.Seq < a.Seq:
		return b, true
	case hasA:
		return a, true
	case hasB:
		return b, true
	}
	return Failure{}, false
}

// Walk recomputes the hash chain over rows given in seq order, in as many batches as the caller
// reads (ADR 0021, 19 SEC-AUD-3): every seq present without a gap, each prev_hash equal to the
// stored hash of the row before, each stored hash equal to SHA-256(prev_hash || canonical row),
// and at the seq of a trusted checkpoint the recomputed hash equal to the checkpoint. The first
// failure ends the walk. It is pure: no I/O, no clock. A Walk is not safe for concurrent use.
//
// Example:
//
//	w := domain.NewWalk(1, domain.ZeroHash, true, sinkHashes)
//	for batch := range batches {
//		if !w.Add(batch) {
//			break
//		}
//	}
//	w.End(head)
//	failure, failed := w.Failure()
type Walk struct {
	next        int64
	prev        [HashSize]byte
	linked      bool
	checkpoints map[int64][HashSize]byte
	compared    int64
	last        Row
	hasLast     bool
	failure     Failure
	failed      bool
}

// NewWalk starts a walk at row from. prev is the hash row from must link to: ZeroHash for row 1,
// the stored hash of row from - 1 for a range that starts later (that row is trusted), or the
// hash of the last checkpoint. linked false trusts the prev_hash of the first row instead (the
// first retained row of a trimmed log without anchor). checkpoints are the trusted checkpoint
// hashes by seq; nil for none.
func NewWalk(from int64, prev [HashSize]byte, linked bool, checkpoints map[int64][HashSize]byte) *Walk {
	return &Walk{next: from, prev: prev, linked: linked, checkpoints: checkpoints}
}

// Add walks the next rows, which continue the rows of earlier calls in seq order. It returns
// false once the walk has failed (also when it failed before).
func (w *Walk) Add(rows []Row) bool {
	for _, r := range rows {
		if w.failed {
			return false
		}
		w.step(r)
	}
	return !w.failed
}

// step checks one row.
func (w *Walk) step(r Row) {
	if r.Seq != w.next {
		w.fail(w.next, SeqGap)
		return
	}
	if w.linked && r.PrevHash != w.prev {
		w.fail(r.Seq, LinkMismatch)
		return
	}
	canonical, err := CanonicalRow(r)
	if err != nil || ChainHash(r.PrevHash, canonical) != r.Hash {
		w.fail(r.Seq, HashMismatch)
		return
	}
	if want, ok := w.checkpoints[r.Seq]; ok {
		w.compared++
		if want != r.Hash {
			w.fail(r.Seq, CheckpointMismatch)
			return
		}
	}
	w.prev, w.linked, w.next = r.Hash, true, r.Seq+1
	w.last, w.hasLast = r, true
}

// fail records the first failure.
func (w *Walk) fail(seq int64, reason FailureReason) {
	if !w.failed {
		w.failure, w.failed = Failure{Seq: seq, Reason: reason}, true
	}
}

// End closes a walk that should have reached row to: a row missing at the tail is a gap.
func (w *Walk) End(to int64) {
	if !w.failed && w.next <= to {
		w.fail(w.next, SeqGap)
	}
}

// Failure returns the first failure and true, or false while the walk holds.
func (w *Walk) Failure() (Failure, bool) { return w.failure, w.failed }

// Failed reports whether the walk has failed.
func (w *Walk) Failed() bool { return w.failed }

// Compared returns how many checkpoints the walk compared.
func (w *Walk) Compared() int64 { return w.compared }

// Next returns the seq the walk expects next.
func (w *Walk) Next() int64 { return w.next }

// Last returns the last row that verified and true, or false before the first.
func (w *Walk) Last() (Row, bool) { return w.last, w.hasLast }
