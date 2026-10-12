// SPDX-License-Identifier: Apache-2.0

// Package jsoncanon is the canonical JSON form of RFC 8785 (JSON Canonicalization Scheme), a thin
// wrapper over github.com/gowebpki/jcs. The Idempotency-Key stage of platform/httpserver hashes the
// canonical body, so that two requests that differ only in member order, whitespace or string
// escapes have the same fingerprint (docs/spec/02-architektur.md "Idempotenz im Unit of Work",
// docs/DECISIONS.md M1-20).
//
// Example:
//
//	canon, err := jsoncanon.Canonicalize([]byte(`{"b": 1, "a": "A"}`))
//	// canon == []byte(`{"a":"A","b":1}`), err == nil
package jsoncanon
