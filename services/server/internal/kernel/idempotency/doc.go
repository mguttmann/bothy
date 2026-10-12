// SPDX-License-Identifier: Apache-2.0

// Package idempotency declares the Store for Idempotency-Key inside the unit of work
// (docs/spec/02-architektur.md "Idempotenz im Unit of Work", 04 "Alle anderen Schreibvorgänge",
// 06 "Idempotenz"). The sync module implements it over the table idempotency_keys; the
// Idempotency-Key stage of platform/httpserver uses it. The package imports only the standard
// library and kernel/ids.
//
// A handler whose success response carries a one-time secret (an enrollment code, an API token)
// calls MarkOneTimeSecret; the stage then stores a placeholder instead of the response, and a
// repeat answers 409 conflict with reason one_time_secret (R-API-26).
//
// Example (a handler that returns a new API token):
//
//	token, err := uc.CreateToken(ctx, in)
//	if err != nil {
//		return nil, err
//	}
//	idempotency.MarkOneTimeSecret(ctx)
//	return toResponse(token), nil
package idempotency
