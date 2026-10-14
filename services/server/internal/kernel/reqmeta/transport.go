// SPDX-License-Identifier: Apache-2.0

package reqmeta

import (
	"context"
	"net/netip"
	"net/textproto"
)

// transportKey is the private context key of the Transport of a request.
type transportKey struct{}

// Transport is what platform/httpserver knows about the connection and the route of a request,
// for the code that must look past the generated handler signature: the identity driver
// trusted_header reads its headers only from a trusted TCP peer, and the per-request binding
// audits the route's action (docs/spec/08-identitaet.md section 2.4, brief M1-43). The header
// values are personal data; they never go into logs, traces, audit details or errors (SEC-LOG-2).
type Transport struct {
	// Peer is the IP of the TCP connection, never an address from a forwarding header; invalid
	// when the chain could not parse it.
	Peer netip.Addr
	// Header are the request header fields with canonical names (the http.Header of the request).
	Header map[string][]string
	// Action is the action of the route ("" outside a route).
	Action string
}

// Values returns the values of the header field name without regard to case, nil when there is
// none.
func (t Transport) Values(name string) []string {
	return t.Header[textproto.CanonicalMIMEHeaderKey(name)]
}

// WithTransport returns a copy of ctx that carries t; platform/httpserver calls it once per
// routed request before the Authenticator runs.
//
// Example:
//
//	ctx = reqmeta.WithTransport(ctx, reqmeta.Transport{Peer: peer, Header: r.Header, Action: op.Action})
func WithTransport(ctx context.Context, t Transport) context.Context {
	return context.WithValue(ctx, transportKey{}, t)
}

// TransportOf returns the Transport stored with WithTransport and true, or the zero value and
// false outside a routed request (a job, the gateway before M1-64 sets one).
func TransportOf(ctx context.Context) (Transport, bool) {
	t, ok := ctx.Value(transportKey{}).(Transport)
	return t, ok
}
