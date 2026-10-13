// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"bytes"
	"cmp"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// CheckpointType is the typ of the JWS header of a checkpoint (19 SEC-AUD-7).
const CheckpointType = "audit-checkpoint+json"

// The members of the checkpoint payload (19 "Format der Prüfpunkte").
const (
	memberInstanceID   = "instance_id"
	memberSeq          = "seq"
	memberHash         = "hash"
	memberOccurredAt   = "occurred_at"
	memberPrevious     = "previous_checkpoint_hash"
	checkpointRequired = 4
)

// Checkpoint is the payload of a signed checkpoint (19 SEC-AUD-7, 03 audit_checkpoints): the
// audit row it covers (seq, hash, occurred_at), the instance and the digest of the previous
// checkpoint's canonical payload.
type Checkpoint struct {
	// InstanceID is instance.instance_id of the installation that signed it.
	InstanceID ids.ID
	// Seq and Hash are those of the covered row of audit_events.
	Seq  int64
	Hash [HashSize]byte
	// OccurredAt is the occurred_at of the covered row, in millisecond precision.
	OccurredAt time.Time
	// Previous is the SHA-256 of the previous checkpoint's canonical payload; HasPrevious is
	// false for the first checkpoint, whose payload has no previous_checkpoint_hash.
	Previous    [HashSize]byte
	HasPrevious bool
}

// NewCheckpoint returns the checkpoint over row for instance; previous is the canonical payload
// of the previous checkpoint, nil for the first.
func NewCheckpoint(instance ids.ID, row Row, previous []byte) Checkpoint {
	c := Checkpoint{InstanceID: instance, Seq: row.Seq, Hash: row.Hash, OccurredAt: TruncateTime(row.OccurredAt)}
	if previous != nil {
		c.Previous, c.HasPrevious = CheckpointDigest(previous), true
	}
	return c
}

// CheckpointDigest returns the SHA-256 of a canonical checkpoint payload: the
// previous_checkpoint_hash of the next checkpoint.
func CheckpointDigest(canonical []byte) [HashSize]byte { return sha256.Sum256(canonical) }

// Canonical returns the payload after RFC 8785: the members sorted, hashes as lower-case hex,
// occurred_at in the canonical time form of SEC-AUD-4. Errors wrap ErrOutOfDomain (a seq outside
// 1 .. 2^53 - 1, a time outside the years 0000 to 9999).
func (c Checkpoint) Canonical() ([]byte, error) {
	if c.Seq < 1 || c.Seq > MaxSafeInteger {
		return nil, fmt.Errorf("%w: checkpoint seq %d outside 1 .. 2^53 - 1", ErrOutOfDomain, c.Seq)
	}
	if y := c.OccurredAt.UTC().Year(); y < 0 || y > 9999 {
		return nil, fmt.Errorf("%w: checkpoint time outside the years 0000 to 9999", ErrOutOfDomain)
	}
	obj := map[string]any{
		memberInstanceID: c.InstanceID.String(),
		memberSeq:        c.Seq,
		memberHash:       hex.EncodeToString(c.Hash[:]),
		memberOccurredAt: FormatTime(c.OccurredAt),
	}
	if c.HasPrevious {
		obj[memberPrevious] = hex.EncodeToString(c.Previous[:])
	}
	return AppendCanonical(nil, obj)
}

// ParseCheckpoint reads a payload that must be exactly the canonical bytes of a checkpoint: the
// four required members, previous_checkpoint_hash optional, nothing else, every value in its
// form. Errors wrap ErrOutOfDomain.
func ParseCheckpoint(payload []byte) (Checkpoint, error) {
	bad := func(rule string) (Checkpoint, error) {
		return Checkpoint{}, fmt.Errorf("%w: checkpoint payload: %s", ErrOutOfDomain, rule)
	}
	dec := json.NewDecoder(bytes.NewReader(payload))
	dec.UseNumber()
	var m map[string]any
	if err := dec.Decode(&m); err != nil || dec.More() {
		return bad("not one JSON object")
	}
	_, hasPrev := m[memberPrevious]
	want := checkpointRequired
	if hasPrev {
		want++
	}
	if len(m) != want {
		return bad("unexpected members")
	}
	var c Checkpoint
	inst, _ := m[memberInstanceID].(string)
	id, err := ParseUUID(inst)
	if err != nil {
		return bad("instance_id")
	}
	c.InstanceID = id
	n, _ := m[memberSeq].(json.Number)
	if c.Seq, err = parseInteger(string(n)); err != nil {
		return bad("seq")
	}
	if c.Hash, err = parseHash(m[memberHash]); err != nil {
		return bad("hash")
	}
	at, _ := m[memberOccurredAt].(string)
	if c.OccurredAt, err = ParseTime(at); err != nil {
		return bad("occurred_at")
	}
	if hasPrev {
		if c.Previous, err = parseHash(m[memberPrevious]); err != nil {
			return bad("previous_checkpoint_hash")
		}
		c.HasPrevious = true
	}
	again, err := c.Canonical()
	if err != nil || !bytes.Equal(again, payload) {
		return bad("not in canonical form")
	}
	return c, nil
}

// parseHash reads a hash in lower-case hex of HashSize bytes.
func parseHash(v any) ([HashSize]byte, error) {
	var out [HashSize]byte
	s, _ := v.(string)
	b, err := hex.DecodeString(s)
	if err != nil || len(b) != HashSize || hex.EncodeToString(b) != s {
		return out, fmt.Errorf("%w: not a lower-case hex hash", ErrOutOfDomain)
	}
	copy(out[:], b)
	return out, nil
}

// SinkCheckpoint is one checkpoint as read from the sink: the seq of its file name and the
// payload of its JWS after the signature verified, nil when it did not.
type SinkCheckpoint struct {
	// Seq is the seq the sink stores the checkpoint under.
	Seq int64
	// Payload is the verified payload; nil means the signature did not verify.
	Payload []byte
}

// CheckpointChain is the result of CheckCheckpoints.
type CheckpointChain struct {
	// Hashes are the hashes of the trusted checkpoints by seq: those before the first invalid one.
	Hashes map[int64][HashSize]byte
	// Last is the highest seq of a trusted checkpoint, 0 without one.
	Last int64
	// Invalid reports an invalid checkpoint at InvalidSeq.
	Invalid    bool
	InvalidSeq int64
}

// CheckCheckpoints checks the checkpoints of the sink in seq order (19 SEC-AUD-7): each one's
// signature verified (a nil payload did not), its payload is canonical, names instance and the
// seq it is stored under, and links the previous one by previous_checkpoint_hash; the first one
// has no link. The first checkpoint that breaks a rule is invalid, and no checkpoint from it on is
// trusted: a deleted or replaced file in the middle breaks the next link.
func CheckCheckpoints(instance ids.ID, cps []SinkCheckpoint) CheckpointChain {
	sorted := slices.Clone(cps)
	slices.SortFunc(sorted, func(a, b SinkCheckpoint) int { return cmp.Compare(a.Seq, b.Seq) })
	out := CheckpointChain{Hashes: make(map[int64][HashSize]byte, len(sorted))}
	var previous []byte
	for _, sc := range sorted {
		c, err := ParseCheckpoint(sc.Payload)
		linked := previous == nil && !c.HasPrevious || previous != nil && c.HasPrevious && c.Previous == CheckpointDigest(previous)
		if sc.Payload == nil || err != nil || c.InstanceID != instance || c.Seq != sc.Seq || !linked {
			out.Invalid, out.InvalidSeq = true, sc.Seq
			return out
		}
		out.Hashes[c.Seq] = c.Hash
		out.Last = c.Seq
		previous = sc.Payload
	}
	return out
}
