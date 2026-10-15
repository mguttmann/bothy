// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/messaging/domain"
)

// ErrClientMsgIDTaken is the error of Store.InsertMessage when the author already has a message
// with the client_msg_id: a concurrent identical request committed first (04 section 5.1 point 4).
// The use case rolls back and answers like a repetition.
var ErrClientMsgIDTaken = errors.New("messaging: the author already used this client_msg_id")

// Store is the outgoing port of the message rows; adapters/postgres implements it over messages
// and mentions. Reads join the ambient transaction of ctx; writes need one. Errors are
// kernel/errs errors except ErrClientMsgIDTaken.
type Store interface {
	// Message reads the message id; found is false for an unknown id.
	Message(ctx context.Context, id ids.ID) (m domain.Message, found bool, err error)
	// MessageByClientID reads the message of author with clientMsgID (the idempotency lookup).
	MessageByClientID(ctx context.Context, author, clientMsgID ids.ID) (m domain.Message, found bool, err error)
	// InsertMessage writes m and returns it as stored. Errors: ErrClientMsgIDTaken (wrapped) for
	// a second (author, client_msg_id).
	InsertMessage(ctx context.Context, m domain.Message) (domain.Message, error)
	// InsertMentions writes one mentions row per principal of message in thread, in one statement.
	InsertMentions(ctx context.Context, message, thread ids.ID, principals []ids.ID, at time.Time) error
	// Mentions returns the mentioned principals of message, sorted.
	Mentions(ctx context.Context, message ids.ID) ([]ids.ID, error)
}
