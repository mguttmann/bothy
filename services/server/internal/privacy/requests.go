// SPDX-License-Identifier: Apache-2.0

package privacy

import (
	"context"
	"io"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Request is a privacy request as the operations return it (schema GdprRequest).
type Request struct {
	ID            ids.ID
	SubjectUserID ids.ID
	// Kind is export, deletion or rectification; State one of the seven states of 03.
	Kind, State string
	RequestedBy ids.ID
	// ApprovedBy is zero while not approved and for an approval by the system.
	ApprovedBy ids.ID
	Reason     string
	DueAt      time.Time
	// ResultExpiresAt is the end of availability of an export package; zero without one.
	ResultExpiresAt time.Time
	// Progress holds steps and counters, never content (03 gdpr_requests.progress).
	Progress map[string]any
	// Error is the error code of a failed request; empty otherwise.
	Error     string
	CreatedAt time.Time
	// CompletedAt is set for a final state.
	CompletedAt time.Time
}

// CreateCmd is the input of createGdprRequest (schema GdprRequestCreate).
type CreateCmd struct {
	// SubjectUserID is the person; zero means the caller.
	SubjectUserID ids.ID
	// Kind is export, deletion or rectification.
	Kind string
	// Reason is the reason of the request, free text.
	Reason string
}

// DecideCmd is the input of decideGdprRequest (schema GdprDecision).
type DecideCmd struct {
	RequestID ids.ID
	// Decision is approve or reject.
	Decision string
	// Reason is communicated to the person and goes into the audit entry.
	Reason string
	// ExtendMonths is the extension of the deadline by one or two months (Art. 12(3) GDPR); 0
	// for none.
	ExtendMonths int
}

// ListQuery is the input of listGdprRequests.
type ListQuery struct {
	// SubjectUserID filters by person; zero for all the caller may read.
	SubjectUserID ids.ID
	// State filters by state; empty for all.
	State string
	// Cursor is the opaque position; Limit the page size (0 is api.page_size_default).
	Cursor string
	Limit  int
}

// Page is one page of requests, newest first; NextCursor is empty on the last page.
type Page struct {
	Items      []Request
	NextCursor string
}

// Package is an opened export package for downloadGdprExport.
type Package struct {
	// Name is export-<user_id>-<date>.zip.
	Name string
	// Size is the length in bytes, SHA256 the lower-case hex digest.
	Size   int64
	SHA256 string
	// Body streams the archive; the caller closes it.
	Body io.ReadCloser
}

// Requests is the inbound port of the privacy requests (docs/spec/20-privacy.md section 5, brief
// M7-06): the five REST operations. Every method authorizes with the action of 05 and returns
// kernel/errs errors.
type Requests interface {
	// Create serves createGdprRequest (gdpr.request.create, self or owner, step-up): 201 with the
	// new request; due_at is the receipt plus one calendar month. An export of one's own data is
	// approved by the system unless privacy.export_requires_approval. Errors: 409 conflict
	// (reason request_open) with the open request of the same person and kind named, 409
	// last_owner for the deletion of the last active owner, 404 for an unknown person, 422.
	Create(ctx context.Context, a actor.Actor, cmd CreateCmd) (Request, error)
	// Get serves getGdprRequest (gdpr.request.read, self or owner).
	Get(ctx context.Context, a actor.Actor, id ids.ID) (Request, error)
	// List serves listGdprRequests: the requests the caller may read, newest first.
	List(ctx context.Context, a actor.Actor, q ListQuery) (Page, error)
	// Decide serves decideGdprRequest (gdpr.request.decide, owner, step-up). Errors: 409
	// invalid_state_transition for a request that is not requested, 409 last_owner, 409 conflict
	// for an extension that is not allowed, 422.
	Decide(ctx context.Context, a actor.Actor, cmd DecideCmd) (Request, error)
	// Download serves downloadGdprExport (gdpr.export.download, self or owner, step-up). Errors:
	// 409 conflict while the package is not ready, 410 gone after privacy.export_ttl.
	Download(ctx context.Context, a actor.Actor, id ids.ID) (Package, error)
}
