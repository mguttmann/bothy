// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
)

// Metrics is the outgoing port of the authz metrics (02 "Layers within a module" rule 5):
// bothy.authz.decisions of signals.md 4.1. internal/wire passes the implementation of
// adapters/otelmetrics; the application layer knows no metrics library. Implementations are safe
// for concurrent use and never block.
type Metrics interface {
	// Decision counts one decision o on the action act of a request or op. It is never called
	// for a check in filter or delivery mode, nor for a check that failed with an error.
	Decision(ctx context.Context, act authorize.Action, o authorize.Outcome)
}

// noMetrics is the Metrics of an Authorizer built without one, for unit tests.
type noMetrics struct{}

// Decision implements Metrics and counts nothing.
func (noMetrics) Decision(context.Context, authorize.Action, authorize.Outcome) {}
