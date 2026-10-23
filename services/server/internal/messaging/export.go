// SPDX-License-Identifier: Apache-2.0

package messaging

import (
	"context"
	"errors"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Export is the port of the channel export (02 "4.3 Ports, events and sources per module";
// exportChannel, brief M3-11). The REST handler of messaging is its caller; the desktop channel
// menu (M4-33) reaches it through that operation.
type Export interface {
	// Channel validates the range, checks export.channel, records the audit entry and
	// returns the stream. On any error nothing has been sent to the client.
	//
	// The principal is the one of ctx (kernel/actor). Errors: validation_failed for a range whose
	// start is not before its end (before any lookup); the refusal of export.channel (not_found for
	// a missing and an invisible channel alike, forbidden, step_up_required, unauthenticated); the
	// error of the audit entry's unit of work (internal_error or unavailable).
	//
	// Example:
	//
	//	st, err := exp.Channel(ctx, messaging.ChannelExportQuery{ChannelID: ch, From: from, To: to})
	//	for err == nil {
	//		var batch []messaging.Message
	//		if batch, err = st.Next(ctx); err == nil {
	//			write(batch)
	//		}
	//	}
	Channel(ctx context.Context, q ChannelExportQuery) (ExportStream, error)
}

// ChannelExportQuery names the channel and the half-open range [From, To) on messages.created_at.
type ChannelExportQuery struct {
	ChannelID ids.ID
	From, To  time.Time
}

// ExportStream yields the export in batches in export order. Next returns io.EOF after the
// last batch and an error wrapping ErrExportAborted when a check during the stream fails.
//
// The order is threads ascending by id, then the messages of a thread ascending by thread_seq;
// every batch holds the full content and tombstones at their thread_seq, and no batch is empty.
// Other errors are read errors of the batch. A stream is not safe for concurrent use.
type ExportStream interface {
	Next(ctx context.Context) ([]MessageView, error)
}

// MessageView is the message DTO of the port, the one representation that History and Get return
// (the MessageView of briefs M1-56 and M1-57); the export adds no second one.
type MessageView = Message

// ErrExportAborted ends a stream whose re-check failed (Design note 7).
var ErrExportAborted = errors.New("messaging: export aborted")
