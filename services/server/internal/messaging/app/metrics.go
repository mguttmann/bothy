// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/messaging/domain"
)

// Metrics is the outgoing port of the messaging metrics (02 "Layers within a module" rule 5):
// bothy.message.created of signals.md 4.2. internal/wire passes the implementation of
// adapters/otelmetrics; the application layer knows no metrics library. Implementations are safe
// for concurrent use and never block.
type Metrics interface {
	// MessageCreated counts one committed message with its origin and kind (the CHECK values of
	// messages.origin and messages.kind in 03).
	MessageCreated(ctx context.Context, origin, kind string)
}

// noMetrics is the Metrics of a service built without one, for unit tests.
type noMetrics struct{}

// MessageCreated implements Metrics and counts nothing.
func (noMetrics) MessageCreated(context.Context, string, string) {}

// countCreated counts m in bothy.message.created once the unit of work of ctx commits, so a
// message whose unit rolls back counts nothing (signals.md 4.2).
func (s *Service) countCreated(ctx context.Context, m domain.Message) {
	metrics, mctx := s.d.Metrics, context.WithoutCancel(ctx)
	txn.AfterCommit(ctx, func() { metrics.MessageCreated(mctx, string(m.Origin), string(m.Kind)) })
}
