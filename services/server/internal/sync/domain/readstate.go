// SPDX-License-Identifier: Apache-2.0

package domain

import "github.com/OWNER/bothy/services/server/internal/kernel/errs"

// The field codes of the personal data rules (06 "Errors": use cases add their own codes).
const (
	// FieldOutOfRange is a read position outside 0..threads.last_message_seq or, for
	// markThreadUnread, above the current position.
	FieldOutOfRange = "out_of_range"
	// FieldInvalid is a value outside the rules of its member (a setting value, U+0000 in a draft).
	FieldInvalid = "invalid"
	// FieldTooManyItems is a list longer than its limit (message.max_attachments).
	FieldTooManyItems = "too_many_items"
	// FieldDuplicate is a repeated list element.
	FieldDuplicate = "duplicate"
	// FieldNotUUIDv7 is an id that is not a UUIDv7.
	FieldNotUUIDv7 = "not_uuid_v7"
	// FieldNotAllowed is a member the target does not take (a reply on a channel draft).
	FieldNotAllowed = "not_allowed"
	// FieldNotInThread is a reply target outside the thread; an unknown id gets the same.
	FieldNotInThread = "not_in_thread"
	// FieldTooLarge is a setting value whose event would exceed sync.event_payload_max_bytes.
	FieldTooLarge = "too_large"
	// FieldNotAForum is a channel draft in a channel that is not a forum.
	FieldNotAForum = "not_a_forum"
)

// fieldError is a validation_failed with one field error.
func fieldError(pointer, code, detail string) error {
	return errs.New(errs.CodeValidationFailed, detail).WithFields(errs.FieldError{Pointer: pointer, Code: code, Detail: detail})
}

// readSeqPointer is the JSON pointer of the body member of putReadState and markThreadUnread.
const readSeqPointer = "/last_read_seq"

// CheckReadSeq checks a requested read position against the thread (brief M1-61 note 1):
// 0 <= seq <= lastMessageSeq, otherwise 400 validation_failed on /last_read_seq with
// out_of_range.
func CheckReadSeq(seq, lastMessageSeq int64) error {
	if seq < 0 || seq > lastMessageSeq {
		return fieldError(readSeqPointer, FieldOutOfRange, "last_read_seq lies outside the messages of the thread")
	}
	return nil
}

// Advance returns the new stored position GREATEST(old, requested) and whether it changed: the
// read position only moves forward (04 section 14, R-SY-13). A missing row is old = 0.
//
// Example:
//
//	seq, changed := domain.Advance(10, 5) // 10, false
func Advance(old, requested int64) (int64, bool) {
	if requested > old {
		return requested, true
	}
	return old, false
}

// MarkUnread checks the target of markThreadUnread against the current position: a target
// above it is 400 out_of_range (advancing is putReadState), an equal one is no change (200
// without event), a smaller one changes the row.
func MarkUnread(current, target int64) (bool, error) {
	if target < 0 || target > current {
		return false, fieldError(readSeqPointer, FieldOutOfRange, "last_read_seq is above the current read position")
	}
	return target < current, nil
}
