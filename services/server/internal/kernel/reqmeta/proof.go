// SPDX-License-Identifier: Apache-2.0

package reqmeta

import (
	"context"
	"slices"
)

type (
	proofKey  struct{}
	headerKey struct{}
)

// Proof are the DPoP facts of a request (RFC 9449; docs/spec/06-api.md section 3.1): the method
// (htm), the expected htu (the origin of server.public_url plus the escaped path, never built from
// the Host header) and every value of the DPoP header. platform/httpserver stores them for every
// request; the handlers of the login and refresh endpoints read them to verify the proof that
// binds new tokens (M1-34, M1-37 to M1-43).
type Proof struct {
	// Method is the request method.
	Method string
	// HTU is the expected htu.
	HTU string
	// Values are the values of the DPoP header in order; nil when there is none.
	Values []string
}

// WithProof returns a copy of ctx that carries p (Values copied).
func WithProof(ctx context.Context, p Proof) context.Context {
	p.Values = slices.Clone(p.Values)
	return context.WithValue(ctx, proofKey{}, p)
}

// ProofOf returns the proof facts stored with WithProof and true, or the zero Proof and false.
func ProofOf(ctx context.Context) (Proof, bool) {
	p, ok := ctx.Value(proofKey{}).(Proof)
	p.Values = slices.Clone(p.Values)
	return p, ok
}

// WithResponseHeaders returns a copy of ctx that carries set, the function that sets a header of
// the response of the request. platform/httpserver installs it, so that code without the response
// writer (the Authenticator, the refresh use case) can attach DPoP-Nonce (06 section 3.1 item 2).
func WithResponseHeaders(ctx context.Context, set func(key, value string)) context.Context {
	return context.WithValue(ctx, headerKey{}, set)
}

// SetResponseHeader sets key to value on the response of the request in ctx; without a setter
// (outside a request) it does nothing.
func SetResponseHeader(ctx context.Context, key, value string) {
	if set, ok := ctx.Value(headerKey{}).(func(key, value string)); ok && set != nil {
		set(key, value)
	}
}
