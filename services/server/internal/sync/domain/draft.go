// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// DraftInput is the body of putThreadDraft and putChannelDraft as the rules see it.
type DraftInput struct {
	// ContentMD is the text of the draft, stored as given.
	ContentMD string
	// ReplyToID is the quoted message; zero for none.
	ReplyToID ids.ID
	// FileIDs are the uploaded attachments, in order.
	FileIDs []ids.ID
	// Channel marks a channel draft (a new forum post), which takes no reply target.
	Channel bool
}

// DraftLimits are the bounds of a draft: message.max_content_bytes and message.max_attachments.
type DraftLimits struct {
	MaxContentBytes, MaxAttachments int
}

// Empty reports an empty draft (04 section 14, row Drafts): no character except whitespace and no
// files. A reply target alone does not keep a draft.
func (in DraftInput) Empty() bool {
	return len(in.FileIDs) == 0 && strings.IndexFunc(in.ContentMD, func(r rune) bool { return !unicode.IsSpace(r) }) < 0
}

// ValidateDraft checks the form of a draft (brief M1-61 note 8): content at most MaxContentBytes
// UTF-8 bytes (413 payload_too_large, the answer of M1-56 for an over-long message) and without
// U+0000 (PostgreSQL text cannot hold it); file_ids at most MaxAttachments UUIDv7 without
// duplicates; no reply target on a channel draft. Ownership and state of the files are checked
// on sending, not here (04 section 14).
func ValidateDraft(in DraftInput, lim DraftLimits) error {
	if len(in.ContentMD) > lim.MaxContentBytes {
		return errs.New(errs.CodePayloadTooLarge, "content_md is larger than message.max_content_bytes")
	}
	if strings.IndexByte(in.ContentMD, 0) >= 0 {
		return fieldError("/content_md", FieldInvalid, "content_md contains U+0000")
	}
	if len(in.FileIDs) > lim.MaxAttachments {
		return fieldError("/file_ids", FieldTooManyItems, "file_ids has more entries than message.max_attachments")
	}
	seen := make(map[ids.ID]struct{}, len(in.FileIDs))
	for i, id := range in.FileIDs {
		ptr := "/file_ids/" + strconv.Itoa(i)
		if id[6]>>4 != 7 || id[8]>>6 != 0b10 {
			return fieldError(ptr, FieldNotUUIDv7, "the file id is not a UUIDv7")
		}
		if _, dup := seen[id]; dup {
			return fieldError(ptr, FieldDuplicate, "the file id is listed twice")
		}
		seen[id] = struct{}{}
	}
	if in.Channel && !in.ReplyToID.IsZero() {
		return fieldError("/reply_to_message_id", FieldNotAllowed, "a draft of a new forum post has no reply target")
	}
	return nil
}

// NotInThread is the 400 of a reply target outside the thread of the draft; an unknown id gets
// the same answer, so nothing leaks.
func NotInThread() error {
	return fieldError("/reply_to_message_id", FieldNotInThread, "the message is not in this thread")
}

// NotAForum is the 400 of a channel draft in a channel that is not a forum: a text channel
// drafts on its main thread.
func NotAForum() error {
	return errs.New(errs.CodeValidationFailed, "only a forum channel has drafts of new posts; a text channel drafts on its main thread")
}

// ReplacedDraft is the stored draft that a write replaces or deletes, as overwrote_newer needs it.
type ReplacedDraft struct {
	// Found reports a stored row before the write.
	Found bool
	// Version is the version of that row.
	Version int64
	// Known reports that Device, ContentMD and FileIDs are those of the replaced row; a write that
	// raced a concurrent write of the same draft knows only Found and Version.
	Known bool
	// Device is the device of the last write of the row; zero for none.
	Device ids.ID
	// ContentMD and FileIDs are the text and the files of the row.
	ContentMD string
	FileIDs   []ids.ID
}

// OverwroteNewer is the hint of a draft write (04 section 14, brief M1-61 note 10, addendum of
// 2026-10-29): the write replaced a draft that another device saved after the version the client
// last saw. It needs a stored row (old.Found) whose version the client has not seen (base absent
// or older than old.Version); a write of this device (old.Device is device), a row with the same
// text and files as the write (in), and an empty write, which deletes the draft, never report it.
// A replaced row whose device and text are not known (a race of two writes) reports it.
func OverwroteNewer(old ReplacedDraft, base *int64, device ids.ID, in DraftInput) bool {
	if !old.Found || in.Empty() || (base != nil && *base >= old.Version) {
		return false
	}
	if !old.Known {
		return true
	}
	if !device.IsZero() && old.Device == device {
		return false
	}
	return old.ContentMD != in.ContentMD || !slices.Equal(old.FileIDs, in.FileIDs)
}

// TruncateContent returns content cut to the longest prefix at a character boundary for which
// fits reports true, and whether it cut: the reduced payload of draft.updated (04 payload rule
// 2), the same search as messaging uses for message.created. fits must be monotonic.
func TruncateContent(content string, fits func(prefix string) bool) (string, bool) {
	if fits(content) {
		return content, false
	}
	lo, hi := 0, len(content)
	for hi-lo > 1 {
		mid := lo + (hi-lo)/2
		if fits(content[:runeStart(content, mid)]) {
			lo = mid
		} else {
			hi = mid
		}
	}
	return content[:runeStart(content, lo)], true
}

// runeStart returns the largest character boundary at or below i.
func runeStart(s string, i int) int {
	if i >= len(s) {
		return len(s)
	}
	for i > 0 && !utf8.RuneStart(s[i]) {
		i--
	}
	return i
}
