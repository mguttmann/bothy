// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/sync/domain"
)

// TestOverwroteNewer: overwrote_newer = row exists AND (base absent OR base < row version).
func TestOverwroteNewer(t *testing.T) {
	t.Parallel()
	v := func(n int64) *int64 { return &n }
	for _, c := range []struct {
		name    string
		exists  bool
		base    *int64
		version int64
		want    bool
	}{
		{"no row", false, nil, 0, false},
		{"no row with base", false, v(3), 0, false},
		{"row and base absent", true, nil, 2, true},
		{"base equal", true, v(2), 2, false},
		{"base older", true, v(1), 2, true},
		{"base newer", true, v(5), 2, false},
	} {
		if got := domain.OverwroteNewer(c.exists, c.base, c.version); got != c.want {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
}

// TestDraftEmptiness: no text except whitespace and no files is empty (04 section 14); a reply
// target alone does not make a draft.
func TestDraftEmptiness(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		in   domain.DraftInput
		want bool
	}{
		{"nothing", domain.DraftInput{}, true},
		{"whitespace only", domain.DraftInput{ContentMD: " \n\t "}, true},
		{"files only", domain.DraftInput{FileIDs: []ids.ID{ids.New()}}, false},
		{"reply only", domain.DraftInput{ReplyToID: ids.New()}, true},
		{"text", domain.DraftInput{ContentMD: " hi "}, false},
	} {
		if got := c.in.Empty(); got != c.want {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
}

// TestDraftValidation: content larger than message.max_content_bytes (the answer of M1-56,
// 413 payload_too_large), too many and duplicate file_ids, ids that are not UUIDv7, U+0000 and a
// reply on a channel draft.
func TestDraftValidation(t *testing.T) {
	t.Parallel()
	lim := domain.DraftLimits{MaxContentBytes: 8, MaxAttachments: 2}
	a, b, c := ids.New(), ids.New(), ids.New()
	if err := domain.ValidateDraft(domain.DraftInput{ContentMD: "12345678", FileIDs: []ids.ID{a, b}, ReplyToID: c}, lim); err != nil {
		t.Fatalf("valid draft: %v", err)
	}
	var v4 ids.ID
	copy(v4[:], a[:])
	v4[6] = 0x40 | v4[6]&0x0f
	for _, tc := range []struct {
		name      string
		in        domain.DraftInput
		code      errs.Code
		ptr, fcod string
	}{
		{"too long", domain.DraftInput{ContentMD: strings.Repeat("x", 9)}, errs.CodePayloadTooLarge, "", ""},
		{"nul", domain.DraftInput{ContentMD: "a\x00"}, errs.CodeValidationFailed, "/content_md", domain.FieldInvalid},
		{"too many files", domain.DraftInput{FileIDs: []ids.ID{a, b, c}}, errs.CodeValidationFailed, "/file_ids", domain.FieldTooManyItems},
		{"duplicate file", domain.DraftInput{FileIDs: []ids.ID{a, a}}, errs.CodeValidationFailed, "/file_ids/1", domain.FieldDuplicate},
		{"not v7", domain.DraftInput{FileIDs: []ids.ID{a, v4}}, errs.CodeValidationFailed, "/file_ids/1", domain.FieldNotUUIDv7},
		{"reply on channel draft", domain.DraftInput{ContentMD: "x", ReplyToID: a, Channel: true}, errs.CodeValidationFailed, "/reply_to_message_id", domain.FieldNotAllowed},
	} {
		code, ptr, fc := fieldOf(t, domain.ValidateDraft(tc.in, lim))
		if code != tc.code || ptr != tc.ptr || fc != tc.fcod {
			t.Errorf("%s: %s %s %s", tc.name, code, ptr, fc)
		}
	}
}

// TestTruncateContent: the longest prefix at a character boundary that fits.
func TestTruncateContent(t *testing.T) {
	t.Parallel()
	got, cut := domain.TruncateContent("aäb", func(s string) bool { return len(s) <= 2 })
	if got != "a" || !cut {
		t.Fatalf("%q %v", got, cut)
	}
	got, cut = domain.TruncateContent("abc", func(string) bool { return true })
	if got != "abc" || cut {
		t.Fatalf("%q %v", got, cut)
	}
}
