// SPDX-License-Identifier: Apache-2.0

package sync

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// ConsumerName names a worker consumer of the event log: one row of event_consumer_cursors
// (docs/spec/03-data-model.md 4.10, ADR 0051). These are not the agent consumers of the agents
// module (agent_consumer_cursors), which the agents port also calls Consumers.
type ConsumerName string

// The worker consumers of 03 4.10. A new name is an additive migration of the check of
// event_consumer_cursors and a constant here (R-DM-27, upgrade.md rule M8).
const (
	// ConsumerPush creates the push jobs (M3-19).
	ConsumerPush ConsumerName = "push"
	// ConsumerWebhooks creates the outgoing webhook deliveries (M2-26).
	ConsumerWebhooks ConsumerName = "webhooks"
)

// ConsumerNames returns every worker consumer name in a fresh slice: exactly the set of the
// check of event_consumer_cursors (TestEventConsumerCursorsTable holds both equal).
func ConsumerNames() []ConsumerName { return []ConsumerName{ConsumerPush, ConsumerWebhooks} }

// Valid reports whether n is one of ConsumerNames.
func (n ConsumerName) Valid() bool { return n == ConsumerPush || n == ConsumerWebhooks }

// ConsumerHandler decides the recipients of one event for a worker consumer and inserts its
// jobs (docs/spec/02-architecture.md "Worker consumers").
//
// Contract: HandleEvent runs inside the transaction of the round, in phase work, once per event
// and in seq order. It may read through ports and the ambient transaction, insert River jobs
// through jobs.Inserter.InsertTx (in the adapters of the handler's module) and write rows of its
// own module, including the audit entry of a system action of that module through
// kernel/auditing. It never calls anything outside the process (R-AR-07), never appends a
// durable event (the round fails then, docs/DECISIONS.md D-M2-01-4) and puts only IDs and
// sequence numbers into job arguments. Every job checks push.deliver or webhook_out.deliver with
// the current rights before it sends (R-AZ-11).
//
// Errors: any error, and a panic, roll back the whole round: no job and no cursor move, and the
// round is tried again at the next wake or poll (D-M2-01-3). A handler must therefore return the
// same jobs for the same event when it runs again.
type ConsumerHandler interface {
	// HandleEvent handles ev inside the round's transaction of ctx.
	HandleEvent(ctx context.Context, ev Event) error
}

// ConsumerLossHandler is implemented by a ConsumerHandler that must learn the events retention
// removed before its consumer read them (sync.event_retention; ADR 0051 Consequences, R-EX-07):
// the webhooks consumer disables the subscriptions that could have matched them (M2-26 design
// note 3 f). Consumers without it (push) are unaffected.
//
// Contract: when the round finds a lost range, HandleLost runs once in the round's transaction,
// before the first HandleEvent, with the first and last lost seq (inclusive). It follows the rules
// of HandleEvent; an error or panic rolls back the whole round.
type ConsumerLossHandler interface {
	// HandleLost handles the lost range from..to inside the round's transaction of ctx.
	HandleLost(ctx context.Context, from, to int64) error
}

// LogReader reads single events of the event log for the delivery jobs of the worker consumers
// (M2-26) and the interaction address of apps (M2-25): the event of a pending delivery or of a
// job by seq or id, and the smallest retained seq. It reads without the
// visible-channel predicate like the consumers; the job checks the rights of its receiver before
// it sends (R-AZ-11). Implementations read in the ambient transaction of ctx or on the pool.
type LogReader interface {
	// EventBySeq returns the event seq; found is false when it is not (or no longer) in the log.
	EventBySeq(ctx context.Context, seq int64) (Event, bool, error)
	// EventByID returns the event id; found is false when it is not (or no longer) in the log.
	// The interaction address of an app (M2-25) names its event by id, because the job of a
	// command or an interaction carries the id the app also sees as webhook-id.
	EventByID(ctx context.Context, id ids.ID) (Event, bool, error)
	// MinRetainedSeq returns min(seq) of event_log, 0 when it is empty.
	MinRetainedSeq(ctx context.Context) (int64, error)
}

// ConsumerState is the position of one worker consumer, as the lag gauges read it.
type ConsumerState struct {
	// Name is the consumer.
	Name ConsumerName
	// LastSeq is the highest seq the consumer processed (event_consumer_cursors.last_seq).
	LastSeq int64
	// HeadSeq is the head of the log (max(seq)) at the same read.
	HeadSeq int64
	// OldestPendingAt is occurred_at of the oldest event after LastSeq; zero when there is none.
	OldestPendingAt time.Time
	// UpdatedAt is event_consumer_cursors.updated_at.
	UpdatedAt time.Time
}

// Consumers is the inbound port of the worker consumers ("worker cursors", 02 "Ports, events
// and sources per module").
type Consumers interface {
	// Register adds the handler h of the consumer name. Only internal/wire calls it, before the
	// runner starts.
	//
	// Errors: an invalid name, a nil handler, a name registered before, and a registration after
	// the runner started.
	Register(name ConsumerName, h ConsumerHandler) error
	// States returns the state of every consumer that has a cursor row, in name order, within
	// database.statement_timeout. It needs no running worker, so every role can export the lag.
	//
	// Errors: those of the database reads.
	States(ctx context.Context) ([]ConsumerState, error)
}
