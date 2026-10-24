// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"slices"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/measure"
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

// countCreated counts m in bothy.message.created and in messages_created of the own measurement
// (docs/spec/25-visualization.md 3.2: dimension origin, the channel of m) once the unit of work of
// ctx commits, so a message whose unit rolls back counts nothing (signals.md 4.2,
// docs/DECISIONS.md D-M6-13-4). A system line (origin system) is outside the dimension list of
// messages_created and counts only in the telemetry.
func (s *Service) countCreated(ctx context.Context, m domain.Message) {
	metrics, rec, mctx := s.d.Metrics, s.d.Measure, context.WithoutCancel(ctx)
	measured := slices.Contains(measure.Dimensions(measure.MessagesCreated), string(m.Origin))
	txn.AfterCommit(ctx, func() {
		metrics.MessageCreated(mctx, string(m.Origin), string(m.Kind))
		if measured {
			rec.Record(measure.MessagesCreated, measure.Key{ChannelID: m.ChannelID, Dimension: string(m.Origin)}, measure.One())
		}
	})
}

// countThread counts a thread created through messaging in threads_created of the own
// measurement once the unit of work of ctx commits (25 3.2, D-M6-13-4).
func (s *Service) countThread(ctx context.Context, channel ids.ID) {
	rec := s.d.Measure
	txn.AfterCommit(ctx, func() { rec.Record(measure.ThreadsCreated, measure.Key{ChannelID: channel}, measure.One()) })
}
