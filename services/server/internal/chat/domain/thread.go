// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"slices"
	"time"
	"unicode/utf8"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Field error codes of the thread rules (M1-55, brief D1, D4 and D5).
const (
	// FieldMainThreadHasNoTitle marks a title change of a main thread.
	FieldMainThreadHasNoTitle = "main_thread_has_no_title"
	// FieldNotAllowedInForum marks a parent message for a forum post.
	FieldNotAllowedInForum = "not_allowed_in_forum"
	// FieldRequiredInTextChannel marks a thread of a text channel without a parent message.
	FieldRequiredInTextChannel = "required_in_text_channel"
	// FieldNotInMainThread marks a parent message outside the main thread of the channel.
	FieldNotInMainThread = "not_in_main_thread"
	// FieldTagsOnlyInForum marks tags for a thread of a text channel.
	FieldTagsOnlyInForum = "tags_only_in_forum"
	// FieldNotAForumPost marks a tag change of a thread that is no forum post.
	FieldNotAForumPost = "not_a_forum_post"
	// FieldDuplicate marks a tag named twice.
	FieldDuplicate = "duplicate"
	// FieldUnknownTag marks a tag that is not a live tag of the thread's forum.
	FieldUnknownTag = "unknown_tag"
	// FieldLimitReached marks a set above its configured maximum (06 section 5 point 3).
	FieldLimitReached = "limit_reached"
)

// ThreadKind is threads.kind (03 section 4.5).
type ThreadKind string

// The thread kinds: main is the continuous history of a text channel (R-DM-11), thread a thread
// from a message or a forum post.
const (
	ThreadKindMain   ThreadKind = "main"
	ThreadKindThread ThreadKind = "thread"
)

// ThreadState is threads.state.
type ThreadState string

// The thread states: closed is "done" (glossary "Erledigt", 01 section 6.1).
const (
	ThreadOpen   ThreadState = "open"
	ThreadClosed ThreadState = "closed"
)

// TitleSource is threads.title_source (09 section 12); package title decides it.
type TitleSource string

// The title sources: auto until a principal other than the thread's operator agent sets a title,
// then manual for good (package title).
const (
	TitleAuto   TitleSource = "auto"
	TitleManual TitleSource = "manual"
)

// Thread is one row of threads (03 section 4.5). Every change of title, state or tags bumps
// Version once; so does every appended message (03 section 6.2).
type Thread struct {
	ID, ChannelID ids.ID
	Kind          ThreadKind
	// ParentMessageID is the message of the main thread the thread started from; zero for a
	// forum post and a main thread.
	ParentMessageID ids.ID
	// Title may be empty; TitleSource says who set it.
	Title       string
	TitleSource TitleSource
	State       ThreadState
	// ClosedAt and ClosedBy are set exactly while the thread is closed.
	ClosedAt time.Time
	ClosedBy ids.ID
	// OperatorAgentID is the operator fixed at creation (R-DM-12); zero for none and for every
	// main thread (R-DM-21).
	OperatorAgentID ids.ID
	// LastMessageSeq, UnreadFloorSeq and MessageCount are the counters, 0 for a new thread.
	LastMessageSeq, UnreadFloorSeq int64
	MessageCount                   int
	// CreatedBy is the creator; CreatedAt, LastActivityAt and MovedAt the times (MovedAt zero
	// until moved).
	CreatedBy                          ids.ID
	CreatedAt, LastActivityAt, MovedAt time.Time
	// Version is 1 for a new thread.
	Version int64
	// SearchLanguage is the text search configuration of the title (threads.search_language,
	// docs/spec/14-suche.md 3.1): the use case sets it from the creator before the insert; the
	// store writes it once and never reads it back, so a loaded thread has "" (docs/DECISIONS.md
	// D-M3-13-10).
	SearchLanguage string
}

// NewMainThread returns the main thread of channel with the defaults of 03: title ”, title
// source auto, state open, no operator, no parent message (R-DM-11, R-DM-21).
func NewMainThread(id, channel, by ids.ID, at time.Time) Thread {
	return Thread{
		ID: id, ChannelID: channel, Kind: ThreadKindMain, TitleSource: TitleAuto, State: ThreadOpen, CreatedBy: by,
		CreatedAt: at, LastActivityAt: at, Version: 1,
	}
}

// Main reports the main thread of a channel.
func (t Thread) Main() bool { return t.Kind == ThreadKindMain }

// Close marks the thread done by by at at and bumps the version; a closed thread stays as it is
// (changed false). Errors: invalid_state_transition for a main thread, which is never done.
func (t *Thread) Close(by ids.ID, at time.Time) (bool, error) {
	if t.Main() {
		return false, errs.New(errs.CodeInvalidStateTransition, "a main thread cannot be closed")
	}
	if t.State == ThreadClosed {
		return false, nil
	}
	t.State, t.ClosedAt, t.ClosedBy = ThreadClosed, at, by
	t.Version++
	return true, nil
}

// Reopen opens a closed thread, clears closed_at and closed_by and bumps the version; it reports
// false for an open thread, which stays as it is.
func (t *Thread) Reopen() bool {
	if t.State != ThreadClosed {
		return false
	}
	t.State, t.ClosedAt, t.ClosedBy = ThreadOpen, time.Time{}, ids.ID{}
	t.Version++
	return true
}

// SetTitle sets title and source and bumps the version when either changed; it reports whether
// it did.
func (t *Thread) SetTitle(title string, source TitleSource) bool {
	if t.Title == title && t.TitleSource == source {
		return false
	}
	t.Title, t.TitleSource = title, source
	t.Version++
	return true
}

// CheckThreadTitle checks a thread title (SEC-IN-4, 03 threads): at most maxChars Unicode
// characters (thread.title_max_chars) and no control or bidi characters; empty is allowed, and the
// title is stored as given. Errors: validation_failed on /title; the error never repeats the input.
func CheckThreadTitle(title string, maxChars int) error {
	if err := displayText("/title", "title", title); err != nil {
		return err
	}
	if utf8.RuneCountInString(title) > maxChars {
		return fieldError("/title", FieldTooLong, "the title is too long")
	}
	return nil
}

// CheckTagSet checks a complete tag set of a thread (brief D5): no tag twice and at most maxTags
// tags (thread.max_tags). Errors: validation_failed on /tag_ids with duplicate or limit_reached.
func CheckTagSet(tags []ids.ID, maxTags int) error {
	seen := make(map[ids.ID]bool, len(tags))
	for _, t := range tags {
		if seen[t] {
			return fieldError("/tag_ids", FieldDuplicate, "a tag is named twice")
		}
		seen[t] = true
	}
	if len(tags) > maxTags {
		return fieldError("/tag_ids", FieldLimitReached, "too many tags")
	}
	return nil
}

// TagDiff is the difference between the current and the requested tag set of a thread.
type TagDiff struct {
	// Added and Removed are the tags that come and go, in the order of the requested and the
	// current set.
	Added, Removed []ids.ID
	// Restricted reports that an added or removed tag is restricted: the change needs manage
	// (05 thread.tags.set).
	Restricted bool
}

// Changed reports a difference.
func (d TagDiff) Changed() bool { return len(d.Added) > 0 || len(d.Removed) > 0 }

// DiffTags compares current with next; restricted reports a restricted tag. A tag kept in both
// sets never sets the restricted flag.
func DiffTags(current, next []ids.ID, restricted func(ids.ID) bool) TagDiff {
	var d TagDiff
	for _, t := range next {
		if !slices.Contains(current, t) {
			d.Added = append(d.Added, t)
			d.Restricted = d.Restricted || restricted(t)
		}
	}
	for _, t := range current {
		if !slices.Contains(next, t) {
			d.Removed = append(d.Removed, t)
			d.Restricted = d.Restricted || restricted(t)
		}
	}
	return d
}
