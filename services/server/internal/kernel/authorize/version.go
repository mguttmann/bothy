// SPDX-License-Identifier: Apache-2.0

package authorize

import (
	"context"
	"sync"
)

type (
	pinKey    struct{}
	memoKey   struct{}
	filterKey struct{}
)

// WithPermissionVersion returns a copy of ctx that pins the rights version v: the Authorizer uses
// v instead of reading the current version (the gateway with the version of the event log, 04
// section 8.1, and tests).
func WithPermissionVersion(ctx context.Context, v int64) context.Context {
	return context.WithValue(ctx, pinKey{}, v)
}

// PinnedPermissionVersion returns the version pinned with WithPermissionVersion.
func PinnedPermissionVersion(ctx context.Context) (int64, bool) {
	v, ok := ctx.Value(pinKey{}).(int64)
	return v, ok
}

// RequestMemo holds values computed once per request: the rights version read by the first check
// (07 section 7.3, R-RE-02) and the lists of Scope. It is safe for concurrent use.
type RequestMemo struct {
	mu   sync.Mutex
	vals map[any]any
}

// Load returns the value stored under key.
func (m *RequestMemo) Load(key any) (any, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.vals[key]
	return v, ok
}

// Store stores v under key; key must be comparable.
func (m *RequestMemo) Store(key, v any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.vals[key] = v
}

// WithRequestScope returns a copy of ctx with a new, empty request memo. platform/httpserver calls
// it once per request at the start of the chain (02 section 9.1 step 1); a ctx that already
// carries a memo is returned unchanged, so a nested call keeps the request's values.
func WithRequestScope(ctx context.Context) context.Context {
	if _, ok := RequestMemoFrom(ctx); ok {
		return ctx
	}
	return context.WithValue(ctx, memoKey{}, &RequestMemo{vals: map[any]any{}})
}

// WithoutRequestScope returns a copy of ctx without a request memo. A long-lived context derived
// from a request (the WebSocket connection, 04 section 7.2) uses it at its boundary, so every
// later check or WithRequestScope reads the current rights version instead of the one memoised by
// the request that opened it.
func WithoutRequestScope(ctx context.Context) context.Context {
	return context.WithValue(ctx, memoKey{}, (*RequestMemo)(nil))
}

// RequestMemoFrom returns the memo installed by WithRequestScope.
func RequestMemoFrom(ctx context.Context) (*RequestMemo, bool) {
	m, ok := ctx.Value(memoKey{}).(*RequestMemo)
	return m, ok && m != nil
}

// Filter returns a copy of ctx in filter mode (07 section 4.2, 05 section 2.4): checks for delivery
// and for the elements of lists, searches and snapshots decide exactly as otherwise but write no
// denial to the audit log. Requests and ops never use it.
//
// Example:
//
//	for _, m := range page {
//		d, err := az.Authorize(authorize.Filter(ctx), p, authorize.ActionMessageRead, res(m))
//		...
//	}
func Filter(ctx context.Context) context.Context {
	return context.WithValue(ctx, filterKey{}, true)
}

// IsFilter reports whether ctx is in filter mode.
func IsFilter(ctx context.Context) bool {
	v, _ := ctx.Value(filterKey{}).(bool)
	return v
}
