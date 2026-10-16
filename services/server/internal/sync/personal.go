// SPDX-License-Identifier: Apache-2.0

package sync

import (
	"context"
	"encoding/json"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Register declares the durable personal events sync appends (M1-25, docs/spec/05-aktionen.md
// section 6): read_state.updated, draft.updated, draft.deleted and settings.updated (M1-61).
// internal/wire lists it in moduleRegistrations.
func Register(r *authorize.Registry) {
	for _, t := range []authorize.EventType{
		authorize.EventReadStateUpdated, authorize.EventDraftUpdated, authorize.EventDraftDeleted, authorize.EventSettingsUpdated,
	} {
		r.Event("sync", t)
	}
}

// Precondition is the If-Match header of a request: Present when it was sent, Valid when it is
// an ETag of the form of 06 "Concurrency" point 1, Version its value.
type Precondition struct {
	Present, Valid bool
	Version        int64
}

// ReadState is the read position of the caller in one thread (schema ReadState). MentionCount is
// computed when the answer is built and never stored (R-DM-32). A thread without a row is the
// implicit state LastReadSeq 0, Version 0. Zero values are absent members.
type ReadState struct {
	ThreadID, ChannelID ids.ID
	LastReadSeq         int64
	MentionCount        int
	Version             int64
	// UpdatedByDeviceID is the device of the last change; zero for none.
	UpdatedByDeviceID ids.ID
	// UpdatedAt is zero for the implicit state.
	UpdatedAt time.Time
}

// AdvanceReadState is the request of putReadState.
type AdvanceReadState struct {
	ThreadID    ids.ID
	LastReadSeq int64
}

// MarkUnread is the request of markThreadUnread: the new, smaller position and the If-Match of
// the request (the version of the read state, 0 for a thread without a row).
type MarkUnread struct {
	ThreadID    ids.ID
	LastReadSeq int64
	IfMatch     Precondition
}

// ReadStates is the incoming port of the read positions (M1-61, 04 section 14). The caller is the
// principal in ctx (kernel/actor); errors are kernel/errs errors, and an unknown and an
// invisible thread give the same 404 not_found. Agents and apps get 403 subject_kind_not_allowed.
type ReadStates interface {
	// Advance moves the position forward (read_state.update): GREATEST(old, new), so it never
	// moves back. A request that changes nothing answers the current state without event and
	// keeps the version; a change raises the version and appends read_state.updated. Errors: 400
	// validation_failed on /last_read_seq (out_of_range) outside 0..threads.last_message_seq.
	//
	// Example:
	//
	//	rs, err := readStates.Advance(ctx, sync.AdvanceReadState{ThreadID: th, LastReadSeq: 42})
	Advance(ctx context.Context, cmd AdvanceReadState) (ReadState, error)
	// MarkUnread moves the position back (read_state.mark_unread) to exactly the given value.
	// Errors: 428 precondition_required without If-Match, 412 version_conflict with the current
	// state for another version, 400 out_of_range above the current position.
	MarkUnread(ctx context.Context, cmd MarkUnread) (ReadState, error)
	// ListForChannel returns the stored read states of the caller in a channel it may read
	// (settings.read on the caller, then thread.read per row in filter mode); an invisible and an
	// unknown channel give the same empty list.
	ListForChannel(ctx context.Context, channelID ids.ID) ([]ReadState, error)
}

// Draft is a draft of the caller (schema Draft): ThreadID is zero for a draft of a new forum post
// (a channel draft). Zero values are absent members; FileIDs is never nil.
type Draft struct {
	ID, ThreadID, ChannelID ids.ID
	ContentMD               string
	// ReplyToID is the quoted message; zero for none.
	ReplyToID ids.ID
	FileIDs   []ids.ID
	Version   int64
	// UpdatedByDeviceID is the device of the last write; zero for none.
	UpdatedByDeviceID ids.ID
	// UpdatedAt is the time of the last write.
	UpdatedAt time.Time
}

// PutDraft is the request of putThreadDraft (ThreadID) and putChannelDraft (ChannelID).
type PutDraft struct {
	ThreadID, ChannelID ids.ID
	ContentMD           string
	// ReplyToID is nil when absent.
	ReplyToID *ids.ID
	FileIDs   []ids.ID
	// BaseVersion is the version the client edited; nil when absent.
	BaseVersion *int64
}

// DraftPutResult is the answer of a draft write (schema DraftPutResult).
type DraftPutResult struct {
	Draft Draft
	// OverwroteNewer reports that the write replaced a version the client had not seen.
	OverwroteNewer bool
}

// Drafts is the incoming port of the drafts (M1-61, 04 section 14): the last write wins in server
// order and every write raises the version. The caller is the principal in ctx; errors are
// kernel/errs errors, an unknown and an invisible thread or channel give the same 404.
type Drafts interface {
	// PutThreadDraft saves the draft of a thread (draft.write, also in read-only and archived
	// channels). An empty draft deletes the row and answers the empty final state. Errors: 400 on
	// /file_ids/<i>, /file_ids or /reply_to_message_id (not_in_thread), 413 payload_too_large for
	// content above message.max_content_bytes.
	PutThreadDraft(ctx context.Context, cmd PutDraft) (DraftPutResult, error)
	// PutChannelDraft saves the draft of a new post of a forum (draft.write on the channel).
	// Errors as PutThreadDraft, 400 for a channel that is not a forum and on /reply_to_message_id
	// for any reply target.
	PutChannelDraft(ctx context.Context, cmd PutDraft) (DraftPutResult, error)
	// DeleteThreadDraft deletes the draft of a thread; no draft is no error (204).
	DeleteThreadDraft(ctx context.Context, threadID ids.ID) error
	// DeleteChannelDraft deletes the draft of a new post of a forum; no draft is no error.
	DeleteChannelDraft(ctx context.Context, channelID ids.ID) error
	// List returns the drafts of the caller in channels it may still read (draft.read); drafts
	// of lost channels stay stored but are not listed.
	List(ctx context.Context) ([]Draft, error)
}

// UserSetting is one stored personal setting (schema UserSetting); Value is one JSON value.
type UserSetting struct {
	Key     string
	Value   json.RawMessage
	Version int64
	// UpdatedByDeviceID is the device of the last write; zero for none.
	UpdatedByDeviceID ids.ID
	UpdatedAt         time.Time
}

// SidebarState is the personal sidebar state of one space (schema SidebarState).
type SidebarState struct {
	SpaceID   ids.ID
	Collapsed bool
	// SortKey is the personal order; "" uses the global order.
	SortKey string
	Version int64
	// UpdatedByDeviceID is the device of the last change; zero for none.
	UpdatedByDeviceID ids.ID
}

// Settings is the incoming port of the personal settings and the read of the sidebar (M1-61):
// last write wins per key; only the keys of 01 section 11 and 13 section 3.3. Defaults are never
// stored or returned; clients apply them. The caller is the principal in ctx.
type Settings interface {
	// List returns every stored setting of the caller (settings.read), ascending by key.
	List(ctx context.Context) ([]UserSetting, error)
	// Put sets key to value (settings.update). The stored value answers without event and
	// without a new version. Errors: 404 not_found for an unknown key, 400 validation_failed on
	// /value for a value outside the rules of the key.
	//
	// Example:
	//
	//	st, err := settings.Put(ctx, "appearance.theme", json.RawMessage(`"dark"`))
	Put(ctx context.Context, key string, value json.RawMessage) (UserSetting, error)
	// Delete removes a stored key and appends settings.updated with deleted; a known key that is
	// not stored is no error. Errors: 404 for an unknown key.
	Delete(ctx context.Context, key string) error
	// ListSidebar returns the sidebar states of the caller for the spaces it may read that are
	// not archived (settings.read, R-AZ-21).
	ListSidebar(ctx context.Context) ([]SidebarState, error)
	// ValuesFor returns the stored values of keys for the people userIDs in one read, keyed by
	// person and key (M3-18: notify gathers notify.default_level for every candidate of a thread;
	// M3-19 reads its switches the same way). It does not authorize: the caller asks for facts of
	// people it already selected and reveals none of them. Defaults are never filled in (04
	// section 14); a person or key without a stored row is absent. It reads in the ambient
	// transaction of ctx when there is one.
	//
	// Example:
	//
	//	vals, err := settings.ValuesFor(ctx, people, []string{"notify.default_level"})
	ValuesFor(ctx context.Context, userIDs []ids.ID, keys []string) (map[ids.ID]map[string]json.RawMessage, error)
}
