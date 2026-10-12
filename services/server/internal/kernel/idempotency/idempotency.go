// SPDX-License-Identifier: Apache-2.0

package idempotency

import (
	"context"
	"errors"
	"sync/atomic"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// ErrInProgress reports that the first request with this key is still running: its row is not
// committed and the wait at the primary key ended with database.lock_timeout. The store returns
// this error instead of the lock timeout, so the unit of work does not retry the wait (02
// "Lock order"); the stage answers 409 request_in_progress.
var ErrInProgress = errors.New("idempotency: first request still in progress")

// Request identifies one keyed request.
type Request struct {
	// PrincipalID is the ID of the actor; the key is valid per principal (06 "Idempotency" point 1).
	PrincipalID ids.ID
	// Key is the Idempotency-Key, a UUIDv7.
	Key ids.ID
	// Action is the x-authz-action of the route.
	Action string
	// SHA256 is the fingerprint of method, path and canonical body.
	SHA256 [32]byte
	// Now is the time of the request (kernel/clock); the row gets created_at Now.
	Now time.Time
	// TTL is sync.idempotency_key_ttl; the row expires at Now + TTL.
	TTL time.Duration
}

// Response is a stored final response.
type Response struct {
	// Status is the HTTP status.
	Status int
	// Body is the JSON body, or empty for a response without body (204).
	Body []byte
	// ResourceID is the top-level id of the body when it is a UUID; the zero value when none.
	ResourceID ids.ID
}

// Entry is the row found for a key.
type Entry struct {
	Response
	// SHA256 is the fingerprint of the first request.
	SHA256 [32]byte
	// Completed is true when the row holds a final response; false while the first request runs.
	Completed bool
}

// Store keeps keys and final responses (table idempotency_keys of the sync module).
type Store interface {
	// Claim claims the key in the ambient transaction of the caller's unit of work: a row that
	// expired at req.Now counts as absent and is removed, then a row in state in_progress is
	// inserted. found is false when the key is new; then the caller runs the operation and calls
	// Complete or rolls back. found is true when a committed row exists; existing is that row.
	//
	// A concurrent first request makes Claim wait at the primary key until its transaction
	// ends; a wait that ends with database.lock_timeout returns ErrInProgress. Other errors are
	// the database's, or kind internal outside a unit of work.
	Claim(ctx context.Context, req Request) (existing Entry, found bool, err error)
	// Complete stores the final response of the key Claim claimed, in the same ambient
	// transaction, so key and effect commit together.
	Complete(ctx context.Context, req Request, resp Response) error
	// SaveFinal stores a final error response in its own short transaction; it runs after the
	// unit of work rolled back. A row that exists already is kept.
	SaveFinal(ctx context.Context, req Request, resp Response) error
}

// secretMark is the flag one stage attempt reads after the handler returned.
type secretMark struct{ marked atomic.Bool }

// secretKey is the context key of the mark.
type secretKey struct{}

// WithSecretScope returns a context with a fresh, unmarked scope for MarkOneTimeSecret. The
// Idempotency-Key stage creates one per attempt, before it calls the handler.
func WithSecretScope(ctx context.Context) context.Context {
	return context.WithValue(ctx, secretKey{}, &secretMark{})
}

// MarkOneTimeSecret records that the response of the current request carries a one-time secret
// that must never be stored (03 principle 9, R-API-26). Outside a scope of WithSecretScope (no
// key, an operation without the parameter) nothing is stored anyway, and the call does nothing.
func MarkOneTimeSecret(ctx context.Context) {
	if m, ok := ctx.Value(secretKey{}).(*secretMark); ok {
		m.marked.Store(true)
	}
}

// OneTimeSecretMarked reports whether MarkOneTimeSecret was called in the scope of ctx.
func OneTimeSecretMarked(ctx context.Context) bool {
	m, ok := ctx.Value(secretKey{}).(*secretMark)
	return ok && m.marked.Load()
}
