// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/messaging"
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
	// MyMentions returns up to q.Limit mentions of q.PrincipalID in q.Visible without tombstones,
	// newest first, after the keyset position of q when q.HasAfter.
	MyMentions(ctx context.Context, q MentionQuery) ([]MentionRow, error)
	// CountUnreadMentions implements the statement of messaging.Mentions.CountUnread.
	CountUnreadMentions(ctx context.Context, principal ids.ID, visible []ids.ID, lastRead map[ids.ID]int64) ([]messaging.ThreadMentionCount, error)

	// PersonalTemplates returns the templates of owner without deleted ones, by lower(name), id.
	PersonalTemplates(ctx context.Context, owner ids.ID) ([]domain.Template, error)
	// ChannelTemplates returns the personal templates of caller and the templates of channel
	// when it is in visible, without deleted ones, by lower(name), id.
	ChannelTemplates(ctx context.Context, caller, channel ids.ID, visible []ids.ID) ([]domain.Template, error)
	// LockVisibleTemplate reads and locks the template id when it belongs to caller or to a
	// channel in visible; withDeleted includes a deleted one. found is false otherwise.
	LockVisibleTemplate(ctx context.Context, id, caller ids.ID, visible []ids.ID, withDeleted bool) (t domain.Template, found bool, err error)
	// LockTemplateScope takes the transaction advisory lock of one scope (brief note 9).
	LockTemplateScope(ctx context.Context, key int64) error
	// CountTemplates counts the templates of the scope of t (its owner or its channel) without
	// deleted ones.
	CountTemplates(ctx context.Context, t domain.Template) (int, error)
	// InsertTemplate writes t with version 1 and returns it as stored.
	InsertTemplate(ctx context.Context, t domain.Template) (domain.Template, error)
	// UpdateTemplate writes name and content of t and raises the version.
	UpdateTemplate(ctx context.Context, t domain.Template) (domain.Template, error)
	// DeleteTemplate sets deleted_at of the template id and raises the version.
	DeleteTemplate(ctx context.Context, id ids.ID, at time.Time) error
}

// MentionQuery is one batch of the mention list (Store.MyMentions).
type MentionQuery struct {
	PrincipalID ids.ID
	Visible     []ids.ID
	// HasAfter starts after the keyset position (AfterAt, AfterID).
	HasAfter bool
	AfterAt  time.Time
	AfterID  ids.ID
	Limit    int
}

// MentionRow is one mention of the list: the message and the time of the mention.
type MentionRow struct {
	Message     domain.Message
	MentionedAt time.Time
}
