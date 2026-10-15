// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/audit/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Store is the storage of the audit module (adapters/postgres). Every method runs in the
// ambient transaction of ctx when there is one, otherwise on the pool. Errors are those of the
// database.
type Store interface {
	// InsertRows inserts rows into audit_events in one statement, in the order given.
	InsertRows(ctx context.Context, rows []domain.Row) error
	// HashAt returns the hash of row seq and true, or false when the row is missing.
	HashAt(ctx context.Context, seq int64) ([domain.HashSize]byte, bool, error)
	// CurrentIPKey returns the row of audit_ip_keys whose period contains at, and false when
	// there is none.
	CurrentIPKey(ctx context.Context, at time.Time) (domain.IPKey, bool, error)
	// InsertIPKey inserts k unless a row with the same valid_from exists
	// (ON CONFLICT (valid_from) DO NOTHING).
	InsertIPKey(ctx context.Context, k domain.IPKey) error
	// IPKeyByID returns the row id of audit_ip_keys and true, or false when it is missing.
	IPKeyByID(ctx context.Context, id ids.ID) (domain.IPKey, bool, error)
	// Rows returns the rows fromSeq .. toSeq in seq order, as stored (M1-50).
	Rows(ctx context.Context, fromSeq, toSeq int64) ([]domain.Row, error)
	// Bounds returns the smallest retained and the largest seq of audit_events, false for an
	// empty log.
	Bounds(ctx context.Context) (first, head int64, ok bool, err error)
	// ListEvents returns at most f.Limit rows matching f, newest first by seq.
	ListEvents(ctx context.Context, f EventFilter) ([]domain.Row, error)
	// ExportSpan returns the smallest and the largest seq of the rows with occurred_at in
	// [from, to), false when there is none.
	ExportSpan(ctx context.Context, from, to time.Time) (first, last int64, ok bool, err error)
	// LastCheckpoint returns the checkpoint of audit_checkpoints with the largest seq, false
	// when there is none.
	LastCheckpoint(ctx context.Context) (StoredCheckpoint, bool, error)
	// LockCheckpoints takes the transaction lock that serialises the insert of checkpoints; it
	// needs a unit of work.
	LockCheckpoints(ctx context.Context) error
	// InsertCheckpoint inserts c into audit_checkpoints; it needs a unit of work.
	InsertCheckpoint(ctx context.Context, c StoredCheckpoint) error
	// UnexportedCheckpoints returns at most limit checkpoints with exported_at NULL in seq order.
	UnexportedCheckpoints(ctx context.Context, limit int) ([]StoredCheckpoint, error)
	// MarkExported sets exported_at of checkpoint seq to at, once.
	MarkExported(ctx context.Context, seq int64, at time.Time) error
}

// EventFilter is the query of Store.ListEvents; zero values filter nothing.
type EventFilter struct {
	// ActorID, ResourceID, Action, Decision and ResourceKind filter their columns exactly.
	ActorID, ResourceID            ids.ID
	Action, Decision, ResourceKind string
	// From (inclusive) and To (exclusive) bound occurred_at; zero for none.
	From, To time.Time
	// BeforeSeq returns only rows with a smaller seq (the keyset of a page); 0 for none.
	BeforeSeq int64
	// Limit is the most rows returned.
	Limit int
}

// StoredCheckpoint is one row of audit_checkpoints.
type StoredCheckpoint struct {
	// Seq and Hash are those of the covered row.
	Seq  int64
	Hash [domain.HashSize]byte
	// Previous is previous_checkpoint_hash; HasPrevious is false for NULL (the first).
	Previous    [domain.HashSize]byte
	HasPrevious bool
	// Signature is the complete compact JWS as UTF-8 bytes.
	Signature []byte
	// KID is the key that signed it.
	KID string
}

// SignedCheckpoint is a checkpoint as the sink stores it: the seq and the compact JWS.
type SignedCheckpoint struct {
	// Seq is the seq of the covered row; the sink files it under this number.
	Seq int64
	// JWS is the compact JWS (header with kid, payload, signature) as UTF-8 bytes.
	JWS []byte
}

// CheckpointSink is the place outside the database where the checkpoints lie
// (audit.checkpoint_sink, 19 SEC-AUD-2 and SEC-AUD-7); adapters/filesink implements file://,
// adapters/s3sink s3:// (M7-22), NoSink stands for the empty value. Every implementation passes
// the conformance suite app/sinktest. Implementations are safe for concurrent use.
type CheckpointSink interface {
	// Configured reports whether a sink is set; without one no checkpoint leaves the database
	// and every verification carries the warning checkpoint_sink_not_configured.
	Configured() bool
	// Put stores c once. The same content again is accepted; other content for a stored seq is
	// an error wrapping ErrCheckpointConflict and the stored checkpoint stays.
	Put(ctx context.Context, c SignedCheckpoint) error
	// List returns every stored checkpoint in seq order. It verifies nothing; verification
	// stays in audit/domain.
	List(ctx context.Context) ([]SignedCheckpoint, error)
}

// Cursors encodes and checks the keyset cursors of listAuditEvents (06 "Paginierung");
// adapters/httpapi implements it with the codec of platform/httpserver.
type Cursors interface {
	// Encode returns the cursor of key for list, bound to binding.
	Encode(list string, binding []byte, key any) (string, error)
	// Decode checks s against list and binding and fills key; every mismatch is cursor_invalid.
	Decode(list string, binding []byte, s string, key any) error
}

// Metrics are the instruments of the chain verification (docs/observability/signals.md "4.6");
// adapters/otelmetrics implements them, without attributes.
type Metrics interface {
	// VerifyFailed adds one to bothy.audit.verify.failures: a run of audit.chain_verify found a
	// broken chain.
	VerifyFailed(ctx context.Context)
	// VerifySucceeded sets bothy.audit.verify.last_success.timestamp to at.
	VerifySucceeded(ctx context.Context, at time.Time)
}

// KEKSource returns the raw content of the secret behind a reference: the key-encryption key of
// the IP keys (privacy.audit_ip_kek_ref, 64 hex characters). internal/wire implements it with the
// secret provider; app never sees the provider.
type KEKSource interface {
	// KEK returns the secret behind ref. The error never contains the secret.
	KEK(ctx context.Context, ref string) ([]byte, error)
}

// Reporter receives what the module must report but cannot return (02 "14.1": details only in
// the log, with the correlation ID of ctx). adapters/logs implements it with the server logger;
// no report ever carries an audit row, an IP address or a name (17 rule 6, SEC-LOG-4).
type Reporter interface {
	// ChainBroken reports that row missingSeq, the predecessor of a new row, is missing.
	ChainBroken(ctx context.Context, missingSeq int64)
	// UnknownReason reports a reason code outside the list of 07 "4.1"; the entry is written.
	UnknownReason(ctx context.Context, action, reason string)
	// DenialsDropped reports count aggregated denials that were dropped because the cap of
	// audit.deny_aggregate_max_keys was reached and their row could not be written.
	DenialsDropped(ctx context.Context, action, reason string, count int64)
	// WriteFailed reports a failed write of aggregated denials; the buckets stay for a retry.
	WriteFailed(ctx context.Context, err error)
	// IPKeyFailed reports that the IP pseudonym of a denial could not be computed; the denial is
	// aggregated without IP fields.
	IPKeyFailed(ctx context.Context, err error)
	// SinkNotConfigured warns that audit.checkpoint_sink is empty (19 SEC-AUD-2): every run of
	// audit.chain_verify and the start of every process that runs jobs.
	SinkNotConfigured(ctx context.Context)
	// VerifyFailed reports a broken chain found by audit.chain_verify; no checkpoint was signed.
	VerifyFailed(ctx context.Context, reason domain.FailureReason)
	// ExportCut reports an export that ended after its stream had started.
	ExportCut(ctx context.Context, err error)
	// RecordFailed reports an audit entry of the verification that could not be written; the
	// result or the run goes on.
	RecordFailed(ctx context.Context, err error)
}
