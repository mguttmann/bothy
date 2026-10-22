// SPDX-License-Identifier: Apache-2.0

// Package httpserver is the HTTP layer of the server (docs/spec/02-architecture.md "Request path",
// docs/spec/06-api.md, docs/spec/16-operations.md "Two listeners"): it opens the listeners and
// builds their http.Server values (M1-11), it protects the transport of the public listener
// (M1-21), and it runs every request of the public listener through one ordered chain (M1-18).
//
// Transport (docs/spec/19-security.md "Transport"): ListenPublic opens server.listen_addr. With
// server.tls_cert_ref and server.tls_key_ref empty it is plaintext behind PeerFilter: only peers
// on loopback or in server.trusted_proxies are served, every other connection is closed at accept
// time before a byte is read (SEC-TLS-1), and the first refusal logs a hint without the peer
// address (SEC-LOG-5). With both references set the server terminates TLS itself: TLS 1.2 as the
// floor, the Go cipher defaults, a certificate that CheckTransport validates at start and that
// follows rotation through the secret provider, no plaintext path (SEC-TLS-2). The internal
// listener stays plaintext without a filter (R-SE-09). One parsed server.trusted_proxies decides
// both plaintext and forwarding headers. PROXY protocol, h2c and Unix sockets are not supported.
//
// Headers: every response of both listeners carries SecurityHeaders (SEC-HDR-1), HSTS with
// http.hsts_max_age while server.public_url uses https (SEC-TLS-3); API responses also carry
// SEC-HDR-2.
//
// The chain: recover (with SEC-HDR-1), server span and correlation ID (platform/otel), base headers and client IP,
// routing with 404 and 405 as problems; then per route StageRoute (operation, body limit,
// deadline, response writer), StageCORS, StageAuthenticate (kernel/actor), the slots
// StageRateLimit, StageWebhookToken, StageMaintenance and StageIdempotency of later tickets,
// StageBody (media type, JSON guard), StagePreconditions (If-Match), StageValidate (schema) and
// StageProof (the check log of kernel/authorize). Every error leaves as application/problem+json through WriteProblem; the
// response writer removes JSON null members, refuses integers beyond 2^53 - 1 and replaces a
// success without an authorize check of the route's action by 500 internal_error.
//
// Example:
//
//	v, err := httpserver.NewValidator(rest.Spec, enabledIDs)
//	chain, err := httpserver.NewChain(cfg, httpserver.Deps{Authenticator: actor.AnonymousOnly{}, Validator: v, Logger: log})
//	mux := http.NewServeMux()
//	mux.Handle(op.Pattern, chain.Wrap(op, handler))
//	srv := httpserver.NewServer(httpCfg, chain.Public(mux), log)
//	ln, err := httpserver.ListenPublic(ctx, transportCfg, secretProvider, log)
//	go srv.Serve(ln)
package httpserver
