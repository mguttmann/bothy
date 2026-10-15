// SPDX-License-Identifier: Apache-2.0

// Package reqmeta carries the correlation_id of a request (its trace ID written as a UUID, never a
// UUIDv7, so kernel/ids never checks it), the client IP, the DPoP facts of the request (Proof), the
// TCP peer, header and action of a routed request (Transport, M1-43) and a setter of response
// headers in the context, with the standard library only.
// platform/httpserver sets the values; audit, sync, identity and the platform packages read them.
package reqmeta
