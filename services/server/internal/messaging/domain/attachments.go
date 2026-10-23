// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
)

// JSON pointers of the attachment list (brief M3-07 design note 5 a). createMessage carries the
// list at AttachmentsPointer; createThread carries it inside first_message, so its pointer is
// FirstMessagePointer + AttachmentsPointer.
const (
	AttachmentsPointer  = "/attachment_file_ids"
	FirstMessagePointer = "/first_message"
)

// CheckAttachments refuses more than maxAttachments (message.max_attachments) ids in the list at
// pointer with validation_failed, errors[].code limit_reached on pointer and a detail that names
// the key (docs/spec/12-files.md section 10). A key value of 0 refuses every attachment. Whether
// each file may be attached is decided later by files under the row locks (409
// file_not_attachable), not here.
//
// Example:
//
//	err := domain.CheckAttachments(len(cmd.AttachmentIDs), cfg.MaxAttachments, domain.AttachmentsPointer)
func CheckAttachments(count, maxAttachments int, pointer string) error {
	if count <= maxAttachments {
		return nil
	}
	const detail = "the message lists more files than message.max_attachments"
	return errs.New(errs.CodeValidationFailed, detail).WithFields(errs.FieldError{Pointer: pointer, Code: FieldLimitReached, Detail: detail})
}

// CheckNewContent checks the content of a new text message: the rules of CheckContent, except
// that a message with at least one attachment (withAttachments) may have an empty or white space
// only content, a picture without caption (docs/decisions.d/M3-07.md D-M3-07). U+0000 and the size limit
// message.max_content_bytes stay. Errors: those of CheckContent.
func CheckNewContent(content string, maxBytes int, withAttachments bool) error {
	err := CheckContent(content, maxBytes)
	if !withAttachments || err == nil {
		return err
	}
	if e, ok := errs.As(err); ok && len(e.Fields) == 1 && e.Fields[0].Code == FieldEmpty {
		return nil
	}
	return err
}
