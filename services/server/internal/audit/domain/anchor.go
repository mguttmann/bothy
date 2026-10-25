// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"encoding/hex"
	"encoding/json"
	"strconv"
	"time"
)

// ActionRetentionTrim is the action of the anchor entry the audit trim writes (03 audit_events,
// M7-05): a system step without an actor, like the other audit codes of system steps that are no
// entries of the action catalogue (docs/DECISIONS.md D-M7-04-2).
const ActionRetentionTrim = "audit.retention_trim"

// The details of an anchor entry: the last removed seq and its hash (03 audit_events).
const (
	DetailAnchorSeq  = "anchor_seq"
	DetailAnchorHash = "anchor_hash"
)

// Anchor is the last row the trim removed: the chain continues at Seq + 1 with prev_hash Hash.
type Anchor struct {
	// Seq is the last removed seq.
	Seq int64
	// Hash is the hash of that row.
	Hash [HashSize]byte
}

// AnchorOf returns the anchor of row r and true when r is an anchor entry with well-formed
// details; false for any other row.
func AnchorOf(r Row) (Anchor, bool) {
	if r.Action != ActionRetentionTrim {
		return Anchor{}, false
	}
	seq, ok := intDetail(r.Details[DetailAnchorSeq])
	if !ok || seq < 1 {
		return Anchor{}, false
	}
	text, _ := r.Details[DetailAnchorHash].(string)
	b, err := hex.DecodeString(text)
	if err != nil || len(b) != HashSize || hex.EncodeToString(b) != text {
		return Anchor{}, false
	}
	a := Anchor{Seq: seq}
	copy(a.Hash[:], b)
	return a, true
}

// HashText returns h as the lower-case hex text of an anchor entry.
func HashText(h [HashSize]byte) string { return hex.EncodeToString(h[:]) }

// intDetail reads an integer of the details: json.Number as read from the database, int64 as
// built by the recorder.
func intDetail(v any) (int64, bool) {
	switch n := v.(type) {
	case json.Number:
		i, err := strconv.ParseInt(n.String(), 10, 64)
		return i, err == nil
	case int64:
		return n, true
	case int:
		return int64(n), true
	}
	return 0, false
}

// StartOf returns where a walk from the first retained row first starts: row 1 with ZeroHash, or,
// when the log was trimmed and anchor (hasAnchor) names the row before first, row first with the
// anchor's hash. trimmed reports the second case. A first row above 1 without its anchor starts at
// row 1, so the missing rows are a seq_gap at 1.
func StartOf(first int64, anchor Anchor, hasAnchor bool) (from int64, prev [HashSize]byte, trimmed bool) {
	if first > 1 && hasAnchor && anchor.Seq == first-1 {
		return first, anchor.Hash, true
	}
	return 1, ZeroHash, false
}

// TrimmedCheckpoints returns the checkpoints of chain that a walk from the anchor can check, and
// the first failure among those it sets aside (M7-05): the rows up to the anchor are gone, so a
// checkpoint below it cannot be compared, but it must be older than notAfter (now minus
// retention.audit; zero skips the check), or the trim removed rows before their period ended; one
// at the anchor must equal the anchor; and the newest checkpoint must lie after the anchor,
// because the trim never removes the row of the newest exported checkpoint. Each failure is
// checkpoint_mismatch at the checkpoint's seq; the smallest seq wins.
func TrimmedCheckpoints(chain CheckpointChain, anchor Anchor, notAfter time.Time) (map[int64][HashSize]byte, Failure, bool) {
	out := make(map[int64][HashSize]byte, len(chain.Hashes))
	var f Failure
	failed := false
	fail := func(seq int64) {
		f, failed = FirstFailure(f, failed, Failure{Seq: seq, Reason: CheckpointMismatch}, true)
	}
	for seq, h := range chain.Hashes {
		if seq > anchor.Seq {
			out[seq] = h
			continue
		}
		if seq == anchor.Seq && h != anchor.Hash {
			fail(seq)
		}
		if at, ok := chain.Times[seq]; !notAfter.IsZero() && ok && !at.Before(notAfter) {
			fail(seq)
		}
	}
	if chain.Last > 0 && chain.Last <= anchor.Seq {
		fail(chain.Last)
	}
	return out, f, failed
}
