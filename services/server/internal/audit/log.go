// SPDX-License-Identifier: Apache-2.0

package audit

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// HashSize is the size of prev_hash and hash in bytes (SHA-256).
const HashSize = 32

// WarningSinkNotConfigured is the warning of every verification while audit.checkpoint_sink is
// empty (19 SEC-AUD-2): no checkpoint outside the database protects the chain.
const WarningSinkNotConfigured = "checkpoint_sink_not_configured"

// Log is the incoming port of the audit log for owners (05 audit.read and audit.verify, owner
// with fresh step-up, every call audited; 20 "jeder Lesezugriff steht selbst im Audit-Log"). The
// caller is the principal of ctx (kernel/actor), anonymous when there is none. app.Log implements
// it; adapters/httpapi serves listAuditEvents, exportAuditEvents and verifyAuditChain over it.
type Log interface {
	// List returns one page of the log, newest first by seq, filtered by q (listAuditEvents).
	// Errors: the decision of authorize (401, 403); kind validation_failed for a filter that is
	// no code; cursor_invalid for a cursor of other filters or another caller; the storage error.
	List(ctx context.Context, q Query) (Page, error)
	// Export streams the rows of the range from (inclusive) to (exclusive) on occurred_at to w in
	// ascending seq, in batches of api.page_size_max (exportAuditEvents). Errors before the first
	// WriteEvents are the decision of authorize, kind validation_failed for a missing bound, and
	// the storage error; an error after it means the stream is cut short.
	Export(ctx context.Context, from, to time.Time, w EventWriter) error
	// Verify recomputes the chain over the range of req and compares it with the checkpoints of
	// audit.checkpoint_sink, never with audit_checkpoints (verifyAuditChain, 19 SEC-AUD-3). A
	// broken chain is a result with OK false, not an error. Errors: the decision of authorize;
	// kind validation_failed for a range that is not positive or from above to; kind unavailable
	// (dependency_unavailable) when the sink cannot be read; the storage error.
	Verify(ctx context.Context, req VerifyRequest) (VerifyResult, error)
}

// Query are the filters of List; zero values filter nothing.
type Query struct {
	// Cursor is next_cursor of the previous page; it is bound to the filters and the caller.
	Cursor string
	// Limit is the page size; 0 means api.page_size_default, larger values are capped at
	// api.page_size_max.
	Limit int
	// ActorID filters actor_principal_id.
	ActorID ids.ID
	// Action, Decision and ResourceKind filter their columns exactly.
	Action, Decision, ResourceKind string
	// ResourceID filters resource_id.
	ResourceID ids.ID
	// From (inclusive) and To (exclusive) bound occurred_at; zero for none.
	From, To time.Time
}

// Page is one page of List.
type Page struct {
	// Items are the rows, newest first.
	Items []Event
	// NextCursor continues the list; empty on the last page.
	NextCursor string
}

// Event is one stored row of audit_events with every column (03 "4.12"), exactly as stored: the
// API representation AuditEvent. A zero ID and an empty string stand for NULL.
type Event struct {
	// Seq is the number in the chain.
	Seq int64
	// ID is the UUID of the row.
	ID ids.ID
	// OccurredAt is the time of the decision, UTC with millisecond precision.
	OccurredAt time.Time
	// ActorPrincipalID, ActorDeviceID and ActorSessionID identify the actor.
	ActorPrincipalID, ActorDeviceID, ActorSessionID ids.ID
	// Action, Decision, DenyReason and ResourceKind are the codes of the row.
	Action, Decision, DenyReason, ResourceKind string
	// ResourceID names the resource.
	ResourceID ids.ID
	// Details is the stored details object, integers as json.Number.
	Details map[string]any
	// CorrelationID is the trace ID of the request as UUID.
	CorrelationID ids.ID
	// PrevHash and Hash link the row into the chain.
	PrevHash, Hash [HashSize]byte
}

// EventWriter receives the rows of Export batch by batch. The first call starts the
// response; the writer flushes after every batch, so the range is never buffered whole.
type EventWriter interface {
	// WriteEvents writes one batch in ascending seq. Its error ends the export.
	WriteEvents(ctx context.Context, events []Event) error
}

// VerifyRequest is the range of Verify; nil means the default.
type VerifyRequest struct {
	// FromSeq defaults to the smallest retained seq. A range that starts after the first retained
	// row trusts the stored hash of row FromSeq - 1 as its starting point.
	FromSeq *int64
	// ToSeq defaults to the head and is capped at it.
	ToSeq *int64
}

// VerifyResult is AuditVerifyResult (docs/api/openapi.yaml).
type VerifyResult struct {
	// OK is true when the range verified.
	OK bool
	// CheckedFromSeq and CheckedToSeq are the range that was checked; 0 for an empty log.
	CheckedFromSeq, CheckedToSeq int64
	// FirstBrokenSeq and FailureReason name the first failure; zero and empty when OK.
	FirstBrokenSeq int64
	FailureReason  string
	// CheckpointsCompared counts the sink checkpoints compared with the recomputed chain.
	CheckpointsCompared int64
	// LastCheckpointSeq is the seq of the newest trusted sink checkpoint; 0 for none.
	LastCheckpointSeq int64
	// Warnings holds WarningSinkNotConfigured while no sink is configured; never nil.
	Warnings []string
}

// ActionRetentionTrim is the action of the anchor entry the retention trim writes (M7-05): its
// details name the last removed seq (anchor_seq) and hash (anchor_hash).
const ActionRetentionTrim = "audit.retention_trim"
