// SPDX-License-Identifier: Apache-2.0

package files

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Attachments binds uploaded files to messages and answers attachment facts
// (docs/spec/12-files.md section 10, 03 message_attachments, M3-07). messaging calls it in the
// create and delete paths and for the representation of a message, runs for the hold of an agent
// invocation. No method calls authorize: the caller authorized the action (message.create,
// message.delete, message.read or the acceptance of a run). The writing methods and Pending run
// only inside the caller's unit of work (internal error otherwise); ForMessages and MessageOf read
// in the ambient transaction when there is one and on the pool otherwise. Errors are kernel/errs
// errors.
type Attachments interface {
	// Attach checks every file against 12 10 point 1 and inserts one row per file in request
	// order with the thread and channel of the message just inserted (R-DM-43). The files rows
	// are locked FOR UPDATE in ascending id before the check. The first file that is not
	// attachable ends the call with 409 conflict, reason file_not_attachable and detail
	// "<Pointer>/<index>"; an unknown, foreign, deleted, other-channel, open, aborted, expired or
	// failed file and a file listed twice or already attached give the same answer. It returns
	// the attachments in position order with the attached form of File (ScopeChannelID zero).
	//
	// Example:
	//
	//	list, err := att.Attach(ctx, files.AttachCmd{MessageID: m, ThreadID: th, ChannelID: ch, AuthorID: alex,
	//		FileIDs: []ids.ID{f1, f2}, Pointer: "/attachment_file_ids"})
	Attach(ctx context.Context, cmd AttachCmd) ([]Attachment, error)
	// ForMessages returns the attachments of the given messages in position order, keyed by the
	// message id; a message without attachments is absent. The caller passes only messages it
	// already read (message.read).
	ForMessages(ctx context.Context, messageIDs []ids.ID) (map[ids.ID][]Attachment, error)
	// DeleteForMessage removes the attachments of a message deleted in the same unit of work and
	// marks their files deleted (12 section 12): the files rows FOR UPDATE in ascending id, then
	// the rows of message_attachments, then files.deleted_at at the clock time. It writes no
	// file.updated (docs/decisions.d/M3-07.md D-M3-07). A message without attachments writes nothing.
	DeleteForMessage(ctx context.Context, messageID ids.ID) error
	// MessageOf reports where a file is attached; ok is false for an unattached file.
	MessageOf(ctx context.Context, fileID ids.ID) (ref AttachmentRef, ok bool, err error)
	// Pending reports whether an attachment of the message is not final (12 section 2
	// "endgültig"); false for a message without attachments.
	Pending(ctx context.Context, messageID ids.ID) (bool, error)
}

// AttachCmd is the input of Attachments.Attach.
type AttachCmd struct {
	// MessageID is the message inserted in the same unit of work; ThreadID and ChannelID are its
	// thread and channel (the channel AppendToThread returned under the thread lock), which the
	// rows copy; AuthorID is its author, who must have uploaded every file.
	MessageID, ThreadID, ChannelID, AuthorID ids.ID
	// FileIDs are the files in request order; position = index + 1.
	FileIDs []ids.ID
	// Pointer is the JSON pointer of the list in the request body, for example
	// "/attachment_file_ids" or "/first_message/attachment_file_ids".
	Pointer string
}

// Attachment is one attachment of a message (schema Attachment): its position from 1 and the
// file in its attached form, without ScopeChannelID (docs/decisions.d/M3-07.md D-M3-07).
type Attachment struct {
	Position int
	File     File
}

// AttachmentRef is where a file is attached: the message, the copies of its thread and channel
// (R-DM-43) and the position.
type AttachmentRef struct {
	MessageID, ThreadID, ChannelID ids.ID
	Position                       int
}
