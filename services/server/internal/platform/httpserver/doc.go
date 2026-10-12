// SPDX-License-Identifier: Apache-2.0

// Package httpserver is the HTTP layer of the server (docs/spec/02-architecture.md "Request path",
// docs/spec/06-api.md, docs/spec/16-operations.md "Two listeners"): it opens the listeners and
// builds their http.Server values (M1-11), and it runs every request of the public listener
// through one ordered chain (M1-18).
//
// The chain: recover, server span and correlation ID (platform/otel), base headers and client IP,
// routing with 404 and 405 as problems; then per route StageRoute (operation, body limit,
// deadline, response writer), StageCORS, StageAuthenticate (kernel/actor), the slots
// StageRateLimit, StageMaintenance and StageIdempotency of later tickets, StageBody (media type,
// JSON guard), StagePreconditions (If-Match), StageValidate (schema) and StageProof (the check log
// of kernel/authorize). Every error leaves as application/problem+json through WriteProblem; the
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
//	ln, err := httpserver.Listen(ctx, "127.0.0.1:8080")
//	go srv.Serve(ln)
package httpserver
