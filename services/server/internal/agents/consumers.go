// SPDX-License-Identifier: Apache-2.0

package agents

import (
	"context"
	"encoding/json"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// ConsumerKey names one consumer: an agent and one of its consumer names (the primary key of
// agent_consumer_cursors).
type ConsumerKey struct {
	AgentID  ids.ID
	Consumer string
}

// ConsumerState is the stored state of a consumer after a takeover: its name, the acknowledged
// seq and whether the takeover created the row (the first connection of the consumer).
type ConsumerState struct {
	Consumer string
	AckedSeq int64
	Created  bool
}

// ErrConsumerReplaced is the error of a fenced save or restart that found no row held by the
// caller: another connection took the consumer over (R-SY-09, R-SY-23), and the caller closes its
// connection with 4015. It is a kernel/errs error of kind conflict; compare with errors.Is.
var ErrConsumerReplaced = errs.New(errs.CodeConflict, "another connection took the consumer over")

// Consumers is the incoming port of the consumer cursors (docs/spec/04-sync-protokoll.md section
// 15.1 and 15.2), implemented by agents/app; the gateway reaches agent_consumer_cursors only
// through it. Take and AcceptAck authorize agent.events.consume for the actor of ctx (agent-self);
// Save and Restart take the key, because the save ticker of the gateway has no request. Errors are
// kernel/errs errors.
type Consumers interface {
	// Take takes the consumer over for holder (a gateway session or a REST window): a missing row
	// is created with acked_seq = head (the first connection starts at head_seq, 04 section 15.1
	// item 2), an existing one keeps acked_seq and gets the new holder. Errors: forbidden for a
	// caller that is not the agent itself, validation_failed for an invalid consumer name.
	//
	// Example:
	//
	//	st, err := consumers.Take(ctx, "default", session.ID, ready.HeadSeq)
	Take(ctx context.Context, consumer string, holder ids.ID, head int64) (ConsumerState, error)
	// AcceptAck authorizes agent.events.consume and checks seq against head (brief M2-07 note 8)
	// without writing. Errors: forbidden, validation_failed on /seq.
	AcceptAck(ctx context.Context, consumer string, seq, head int64) error
	// Save stores the acknowledgement acked of key while holder still holds it (fenced and
	// monotone). Errors: ErrConsumerReplaced when another holder took over, internal otherwise.
	Save(ctx context.Context, key ConsumerKey, holder ids.ID, acked int64) error
	// Restart sets acked_seq of key to seq while holder holds it (resync_required with retention
	// or cursor_ahead). Errors: ErrConsumerReplaced, internal.
	Restart(ctx context.Context, key ConsumerKey, holder ids.ID, seq int64) error
}

// The values of FeedPage.Resync: empty, or a reason of resync_required that moves the consumer to
// the head (04 section 7.3).
const (
	ResyncRetention   = "retention"
	ResyncCursorAhead = "cursor_ahead"
)

// FeedPage is one REST poll of a consumer: the envelopes to deliver in seq order (with their
// delivery fields), the cursor to acknowledge after processing (also when Events is empty), the
// head of the serving process and the sync epoch. Resync is empty, or retention or cursor_ahead
// after the consumer was moved to HeadSeq (410 resync_required).
type FeedPage struct {
	Events     []json.RawMessage
	NextCursor int64
	HeadSeq    int64
	SyncEpoch  ids.ID
	Resync     string
}

// ConsumerFeed is the source port the REST use cases read the stream through (02 section 4.1 rule
// 4): agents declares it, the delivery engine of the gateway implements it, so one engine serves
// the WebSocket and REST with the same window rules (brief M2-07 note 1).
type ConsumerFeed interface {
	// Poll delivers the next events of consumer for the agent of ctx, at most limit and at most
	// agent.max_inflight_events, and takes the consumer over for the REST window of this process
	// (brief M2-07 note 10). Errors: those of Consumers.Take and of the event log.
	Poll(ctx context.Context, consumer string, limit int) (FeedPage, error)
	// Acked tells a window of key in this process about the stored acknowledgement acked.
	Acked(ctx context.Context, key ConsumerKey, acked int64)
	// Head returns the head of the serving process (the hub position) and the sync epoch.
	Head(ctx context.Context) (head int64, epoch ids.ID, err error)
}

// ConsumerView is the answer of getConsumerState and ackAgentEvents (schema ConsumerState).
type ConsumerView struct {
	Consumer string
	AckedSeq int64
	HeadSeq  int64
}

// ConsumerAPI is the incoming port of the three REST operations of a consumer (brief M2-07 note
// 10), implemented by agents/app. Each authorizes agent.events.consume for the agent of ctx.
type ConsumerAPI interface {
	// State reads the cursor of consumer (getConsumerState). Errors: not_found for a consumer that
	// never connected.
	State(ctx context.Context, consumer string) (ConsumerView, error)
	// Poll fetches the next events (pollAgentEvents); limit 0 means agent.max_inflight_events.
	// Errors: resync_required (410) with the reason after the consumer was moved to the head.
	Poll(ctx context.Context, consumer string, limit int) (FeedPage, error)
	// Ack stores the acknowledgement seq at once and monotonically (ackAgentEvents). Errors:
	// validation_failed on /seq, not_found for an unknown consumer.
	Ack(ctx context.Context, consumer string, seq int64) (ConsumerView, error)
}
