// SPDX-License-Identifier: Apache-2.0

package search

import "context"

// Probe is the test query of the status page (health_source search.query of
// docs/spec/16-operations.md 12, M7-09). It reads no content for anyone: the query names no
// visible channel. It is safe for concurrent use.
type Probe interface {
	// Probe builds the tsquery of a fixed term and runs the message search of the index with it
	// under search.statement_timeout. Errors: the timeout, the database error.
	//
	// Example:
	//
	//	if err := s.Probe(ctx); err != nil { /* no answer */ }
	Probe(ctx context.Context) error
}
