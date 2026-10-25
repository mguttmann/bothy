// SPDX-License-Identifier: Apache-2.0

package audit

import (
	"context"
	"iter"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// PurposeAuditCheckpoint is server_signing_keys.purpose of the keys that sign the checkpoints;
// RowsVerifier uses only keys of this purpose.
const PurposeAuditCheckpoint = "audit_checkpoint"

// VerificationKey is the public part of one server signing key as server_signing_keys stores it
// (docs/spec/03-data-model.md 4.1): the restore test reads it from the restored copy.
type VerificationKey struct {
	// KID is the key id of the JWS header.
	KID string
	// Purpose is the purpose of the key; only PurposeAuditCheckpoint verifies checkpoints.
	Purpose string
	// Alg is EdDSA or ES256.
	Alg string
	// PublicJWK is public_jwk, the JWK without private part.
	PublicJWK []byte
}

// CheckpointRow is one row of audit_checkpoints as stored.
type CheckpointRow struct {
	// Seq and Hash are those of the covered row.
	Seq  int64
	Hash [HashSize]byte
	// Signature is the complete compact JWS as UTF-8 bytes.
	Signature []byte
}

// RowsInput is a chain that the caller reads itself, from a database other than the one of the
// running server (the restored copy of the restore test, brief M7-03 design note 2).
type RowsInput struct {
	// InstanceID is instance.instance_id of the database the rows come from; a checkpoint of
	// another instance does not verify.
	InstanceID ids.ID
	// Keys are the public signing keys of that database; keys of other purposes are ignored.
	Keys []VerificationKey
	// Events yields every row of audit_events in ascending seq; an error ends the verification
	// with that error.
	Events iter.Seq2[Event, error]
	// Checkpoints yields every row of audit_checkpoints in ascending seq; an error ends the
	// verification with that error.
	Checkpoints iter.Seq2[CheckpointRow, error]
	// LatestAnchor is the newest row of audit_events with the action audit.retention_trim, nil
	// when there is none (M7-05): after a trim the walk starts at the first retained row with the
	// hash this anchor names.
	LatestAnchor *Event
}

// RowsVerifier is the port of the chain verification over rows the caller supplies: the walk of
// audit.verify (19 SEC-AUD-3) from row 1 to the last row, with the checkpoints of
// audit_checkpoints in place of those of the sink. A checkpoint is trusted when its JWS has the
// header typ audit-checkpoint+json, verifies with a key of Keys of purpose audit_checkpoint, names
// InstanceID and links to the checkpoint before it. app.RowsVerifier implements it; the module
// status calls it for the restore test (brief M7-03 rule 14), so the chain is verified by one
// implementation only.
type RowsVerifier interface {
	// VerifyRows walks in and returns the result in the form of audit.verify: OK, the checked
	// range, the first broken seq and the failure reason (hash_mismatch, link_mismatch, seq_gap,
	// checkpoint_mismatch, checkpoint_invalid), the compared and the last trusted checkpoint. A
	// broken chain is a result, not an error; the error is the first error of an iterator.
	//
	// Example:
	//
	//	res, err := v.VerifyRows(ctx, audit.RowsInput{InstanceID: id, Keys: keys, Events: rows, Checkpoints: cps})
	//	if err == nil && !res.OK { /* res.FirstBrokenSeq, res.FailureReason */ }
	VerifyRows(ctx context.Context, in RowsInput) (VerifyResult, error)
}
