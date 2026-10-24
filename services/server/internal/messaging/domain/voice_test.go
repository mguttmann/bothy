// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"testing"

	"github.com/OWNER/bothy/services/server/internal/messaging/domain"
)

// TestVoiceBodyRules (brief M3-09 design note 7, M1-56 D2): kind voice is enabled like text and
// card, every other kind stays not_enabled on /kind; a voice message needs exactly one attachment,
// none is required and more than one limit_reached, each on the pointer of the list
// (/first_message/... in createThread); the content may be empty with the one attachment. The card
// rules (only agents send cards, apps not_enabled) stay in app (cardRules).
func TestVoiceBodyRules(t *testing.T) {
	t.Parallel()
	for _, k := range []string{"text", "voice", "card"} {
		if err := domain.CheckNewKind(k); err != nil {
			t.Errorf("kind %s: %v", k, err)
		}
	}
	for _, k := range []string{"system", "run", "image", ""} {
		wantField(t, domain.CheckNewKind(k), "/kind", domain.FieldNotEnabled)
	}

	createThread := domain.FirstMessagePointer + domain.AttachmentsPointer
	for _, pointer := range []string{domain.AttachmentsPointer, createThread} {
		wantField(t, domain.CheckVoiceAttachments(0, pointer), pointer, domain.FieldRequired)
		wantField(t, domain.CheckVoiceAttachments(2, pointer), pointer, domain.FieldLimitReached)
		wantField(t, domain.CheckVoiceAttachments(3, pointer), pointer, domain.FieldLimitReached)
		if err := domain.CheckVoiceAttachments(1, pointer); err != nil {
			t.Errorf("one attachment on %s: %v", pointer, err)
		}
	}
	// A voice message without caption: the one attachment allows an empty content.
	if err := domain.CheckNewContent("", 100, true); err != nil {
		t.Errorf("empty content of a voice message: %v", err)
	}
}
