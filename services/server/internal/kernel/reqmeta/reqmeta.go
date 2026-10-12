// SPDX-License-Identifier: Apache-2.0

package reqmeta

import (
	"context"
	"net/netip"
)

// correlationKey and clientIPKey are the private context keys of this package, so no other
// package can overwrite or read the values except through the functions below.
type (
	correlationKey struct{}
	clientIPKey    struct{}
)

// WithCorrelationID returns a copy of ctx that carries id as the correlation_id of the request:
// the trace ID of the server span written as a lowercase UUID (8-4-4-4-12 hex). platform/httpserver
// calls it once per request; the value is never a UUIDv7, so kernel/ids never checks it.
//
// Example:
//
//	ctx = reqmeta.WithCorrelationID(ctx, "4bf92f35-77b3-4da6-a3ce-929d0e0e4736")
func WithCorrelationID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, correlationKey{}, id)
}

// CorrelationID returns the correlation_id stored with WithCorrelationID, or "" when ctx has none
// (code that runs outside a request, such as a job, sets its own).
func CorrelationID(ctx context.Context) string {
	id, _ := ctx.Value(correlationKey{}).(string)
	return id
}

// WithClientIP returns a copy of ctx that carries ip as the client IP of the request, as
// platform/httpserver resolved it from the peer and the forwarding headers of trusted proxies
// (docs/spec/06-api.md "Grundregeln", row Client-IP). An invalid address is not stored, so
// ClientIP reports it as absent.
func WithClientIP(ctx context.Context, ip netip.Addr) context.Context {
	if !ip.IsValid() {
		return ctx
	}
	return context.WithValue(ctx, clientIPKey{}, ip)
}

// ClientIP returns the client IP stored with WithClientIP and true, or the zero address and false
// when ctx has none. The value is for rate limits and the pseudonymised audit only; it never goes
// into general logs (19-sicherheit.md SEC-LOG-5).
func ClientIP(ctx context.Context) (netip.Addr, bool) {
	ip, ok := ctx.Value(clientIPKey{}).(netip.Addr)
	return ip, ok
}
