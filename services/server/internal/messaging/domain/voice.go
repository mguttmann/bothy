// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
)

// CheckNewKind checks MessageCreate.kind of a new message (M1-56 D2, M3-09 design note 7): text,
// voice and card are enabled; every other kind (system, run and unknown values) is 400
// validation_failed with errors[].code not_enabled on /kind. Who may send a card is decided by
// the card rules of app (M2-15), the attachment of a voice message by CheckVoiceAttachments.
//
// Example:
//
//	err := domain.CheckNewKind("voice") // nil
func CheckNewKind(kind string) error {
	switch Kind(kind) {
	case KindText, KindVoice, KindCard:
		return nil
	case KindSystem, KindRun:
	}
	return NotEnabled("/kind")
}

// CheckVoiceAttachments checks the attachment list of a voice message at pointer (docs/spec/
// 12-files.md section 8.1, M3-09 design note 7): a voice message has exactly one attachment, the
// recording. None is 400 validation_failed with errors[].code required, more than one
// limit_reached, both on pointer ("/attachment_file_ids", in createThread
// "/first_message/attachment_file_ids"). Whether the file is a voice recording is decided by files
// under the row lock (voice_format_unsupported, voice_too_long).
//
// Example:
//
//	err := domain.CheckVoiceAttachments(len(cmd.AttachmentIDs), domain.AttachmentsPointer)
func CheckVoiceAttachments(count int, pointer string) error {
	switch {
	case count == 0:
		const detail = "a voice message needs its recording as the one attachment"
		return errs.New(errs.CodeValidationFailed, detail).WithFields(errs.FieldError{Pointer: pointer, Code: FieldRequired, Detail: detail})
	case count > 1:
		const detail = "a voice message has exactly one attachment"
		return errs.New(errs.CodeValidationFailed, detail).WithFields(errs.FieldError{Pointer: pointer, Code: FieldLimitReached, Detail: detail})
	}
	return nil
}
