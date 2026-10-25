// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/messaging/domain"
)

// TestAttachmentBodyRules (brief M3-07 design note 5 a): more than message.max_attachments ids are
// limit_reached on the pointer of the list (/first_message/... in createThread); with at least one
// attachment the content may be empty or white space only, without one the M1-56 rule (empty)
// stays; the rules of kind voice are TestVoiceBodyRules (M3-09).
func TestAttachmentBodyRules(t *testing.T) {
	t.Parallel()
	const maxAttachments = 3
	createThread := domain.FirstMessagePointer + domain.AttachmentsPointer
	if createThread != "/first_message/attachment_file_ids" || domain.AttachmentsPointer != "/attachment_file_ids" {
		t.Fatalf("pointers %q %q", domain.AttachmentsPointer, createThread)
	}
	for _, n := range []int{0, 1, maxAttachments} {
		if err := domain.CheckAttachments(n, maxAttachments, domain.AttachmentsPointer); err != nil {
			t.Errorf("%d attachments: %v", n, err)
		}
	}
	for _, pointer := range []string{domain.AttachmentsPointer, createThread} {
		err := domain.CheckAttachments(maxAttachments+1, maxAttachments, pointer)
		wantField(t, err, pointer, domain.FieldLimitReached)
		if e, _ := errs.As(err); !strings.Contains(e.Fields[0].Detail, "message.max_attachments") {
			t.Errorf("detail %q does not name the key", e.Fields[0].Detail)
		}
	}
	// message.max_attachments = 0 refuses every attachment (its minimum is 0).
	wantField(t, domain.CheckAttachments(1, 0, domain.AttachmentsPointer), domain.AttachmentsPointer, domain.FieldLimitReached)

	for _, c := range []string{"", " \n\t "} {
		if err := domain.CheckNewContent(c, 100, true); err != nil {
			t.Errorf("content %q with an attachment: %v", c, err)
		}
		wantField(t, domain.CheckNewContent(c, 100, false), "/content_md", domain.FieldEmpty)
	}
	// The other content rules stay with attachments.
	wantField(t, domain.CheckNewContent("a\x00", 100, true), "/content_md", domain.FieldInvalid)
	if e, _ := errs.As(domain.CheckNewContent(strings.Repeat("a", 101), 100, true)); e == nil || e.Code != errs.CodePayloadTooLarge {
		t.Errorf("too large content with an attachment: %v", e)
	}
	if err := domain.CheckNewContent("caption", 100, true); err != nil {
		t.Errorf("caption: %v", err)
	}
}
