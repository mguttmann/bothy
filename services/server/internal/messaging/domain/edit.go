// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"bytes"
	"encoding/json"
	"slices"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Revision is one row of message_revisions: the content of a message before an edit.
type Revision struct {
	ID, MessageID ids.ID
	// Number is message_revisions.revision; 1 is the original content.
	Number int
	// ContentMD is the content before the edit.
	ContentMD string
	// EditedBy is the principal who edited.
	EditedBy ids.ID
	// CreatedAt is the time of the edit.
	CreatedAt time.Time
}

// Unchanged reports whether an edit leaves a message as it is (M1-58 design note 2 point 5): the
// same content byte for byte and the same mention set, in any order. An empty body keeps both,
// so it is no change either; the use case then writes nothing.
func Unchanged(oldContent string, oldMentions []ids.ID, newContent string, newMentions []ids.ID) bool {
	if oldContent != newContent || len(oldMentions) != len(newMentions) {
		return false
	}
	for _, id := range newMentions {
		if !slices.Contains(oldMentions, id) {
			return false
		}
	}
	return true
}

// StoresRevision reports whether an edit of a message of kind k stores its previous content in
// message_revisions: every kind except the run anchor, whose history is in run_steps (03
// message_revisions).
func StoresRevision(k Kind) bool { return k != KindRun }

// Tombstone returns m as deleted by by at at (03 section 6.5): the row with its thread_seq and
// every other field, content_md empty, deleted_at and deleted_by set and the next version. It is
// the one tombstone function of the module; history (M1-57) renders the stored row of it.
func Tombstone(m Message, by ids.ID, at time.Time) Message {
	m.ContentMD, m.DeletedAt, m.DeletedBy, m.Version = "", at, by, m.Version+1
	return m
}

// Event types with resource message and their redaction (04 section 2.2 rule 5).
const (
	eventMessageCreated  = "message.created"
	eventMessageUpdated  = "message.updated"
	eventReactionAdded   = "reaction.added"
	eventReactionRemoved = "reaction.removed"
	eventPinAdded        = "pin.added"
	eventPinRemoved      = "pin.removed"
)

// reactionRedacted is ReactionPayload after the deletion: the ids stay, emoji and count go.
type reactionRedacted struct {
	MessageID   json.RawMessage `json:"message_id"`
	ThreadID    json.RawMessage `json:"thread_id"`
	Emoji       string          `json:"emoji"`
	PrincipalID json.RawMessage `json:"principal_id"`
	Count       int             `json:"count"`
}

// RedactPayload returns the redaction of the stored events of a deleted message whose event form
// (schema Message, the tombstone after this deletion) is tombstone, used as the kernel
// eventlog.RedactFunc (M1-58 design note 6, 04 section 2.2 rule 5); the function type is spelled
// out because the domain imports no kernel port (go-arch-lint). It is pure:
//
//   - message.created and message.updated become {"message": tombstone};
//   - reaction.added and reaction.removed keep message_id, thread_id and principal_id and get
//     emoji "" and count 0;
//   - pin.added and pin.removed stay unchanged (they carry no content);
//   - any other type or a schema version other than 1 is an internal error, so the deletion
//     fails: a new event type on resource message defines its redaction before it can be
//     deleted.
//
// Example:
//
//	n, err := redactor.Redact(ctx, "message", id, domain.RedactPayload(tombstone))
func RedactPayload(tombstone json.RawMessage) func(eventType string, schemaVersion int, payload json.RawMessage) (json.RawMessage, error) {
	message := append(append([]byte(`{"message":`), tombstone...), '}')
	return func(eventType string, schemaVersion int, payload json.RawMessage) (json.RawMessage, error) {
		if schemaVersion != 1 {
			return nil, unredactable()
		}
		switch eventType {
		case eventMessageCreated, eventMessageUpdated:
			return bytes.Clone(message), nil
		case eventReactionAdded, eventReactionRemoved:
			return redactReaction(payload)
		case eventPinAdded, eventPinRemoved:
			return payload, nil
		}
		return nil, unredactable()
	}
}

// redactReaction keeps the ids of a ReactionPayload and empties emoji and count.
func redactReaction(payload json.RawMessage) (json.RawMessage, error) {
	var r reactionRedacted
	if err := json.Unmarshal(payload, &r); err != nil || r.MessageID == nil || r.ThreadID == nil || r.PrincipalID == nil {
		return nil, unredactable()
	}
	r.Emoji, r.Count = "", 0
	out, err := json.Marshal(r)
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternalError, err, "messaging: encoding a redacted reaction")
	}
	return out, nil
}

// unredactable is the error of an event that has no redaction; it names no content.
func unredactable() error {
	return errs.New(errs.CodeInternalError, "messaging: an event of a deleted message has no defined redaction")
}
