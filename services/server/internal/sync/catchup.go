// SPDX-License-Identifier: Apache-2.0

package sync

import (
	"context"
	"encoding/json"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// ResyncReason is the reason of resync_required (docs/spec/04-sync-protocol.md section 7.3); the
// empty value means catch-up is possible.
type ResyncReason string

// The reasons of resync_required.
const (
	// ResyncEpochChanged: the stored sync epoch is not the current one (R-SY-18).
	ResyncEpochChanged ResyncReason = "epoch_changed"
	// ResyncCursorAhead: the cursor is above the head (R-SY-04).
	ResyncCursorAhead ResyncReason = "cursor_ahead"
	// ResyncRetention: events after the cursor were removed by sync.event_retention.
	ResyncRetention ResyncReason = "retention"
	// ResyncTooFarBehind: more than sync.catchup_max_events events lie after the cursor.
	ResyncTooFarBehind ResyncReason = "too_far_behind"
)

// CheckInput is the start of a catch-up: the cursor of the client and the epoch it stored with
// it (nil when it sent none, before its first catch-up).
type CheckInput struct {
	After int64
	Epoch *ids.ID
}

// CheckResult is the state of the log at the start of a catch-up.
type CheckResult struct {
	// HeadSeq is the committed value of event_seq; every seq up to it is visible.
	HeadSeq int64
	// MinRetainedSeq is min(seq) of event_log, HeadSeq + 1 for an empty log.
	MinRetainedSeq int64
	// SyncEpoch is instance.sync_epoch.
	SyncEpoch ids.ID
	// Reason is empty when catch-up is possible, else the reason of resync_required.
	Reason ResyncReason
}

// PageInput asks for one page after the cursor After up to the head of Check; Limit 0 means
// sync.catchup_page_size, and a larger Limit is capped to it.
type PageInput struct {
	After, Head int64
	Limit       int
	// AgentConsumer marks the page read of the agent consumer engine of the gateway (WebSocket and
	// REST poll), which passes every invocation-bearing event through runs.Delivery before it is
	// sent (M2-08). Only then is a personal event whose receive action takes the agent itself
	// (agent.events.consume of agent.invocation) decided on its audience; every other caller, the
	// generic catch-up listEvents included, never receives such an event.
	AgentConsumer bool
}

// EnvelopeJSON is one event in its wire form: the JSON of Event.Envelope (ADR 0052), without
// JSON null.
type EnvelopeJSON = json.RawMessage

// PageResult is one page of catch-up: the events the caller may receive, in seq order, and the
// cursor for the next call. NextCursor is the highest seq checked, also when Events is empty
// (R-SY-03).
type PageResult struct {
	Events     []EnvelopeJSON
	NextCursor int64
}

// EventPage is one page of catch-up as the gateway reads it (M1-65): the admitted events
// themselves in seq order, so the gateway knows their scope, and the highest seq checked.
type EventPage struct {
	Events     []Event
	NextCursor int64
}

// CatchUp is the incoming port of catch-up (04 section 7, M1-63): GET /api/v1/events uses it, and
// the gateway uses it for ready, resume and the streaming catch-up (M1-64, M1-65), so REST and
// WebSocket follow the same rules. The caller is the principal in ctx (kernel/actor); errors are
// kernel/errs errors.
type CatchUp interface {
	// Check authorizes events.catch_up on the instance (an app needs events:subscribe), reads
	// head, minimum and epoch and decides resync_required in the order of 04 section 7.3: epoch,
	// cursor ahead, retention, too far behind.
	//
	// Example:
	//
	//	res, err := catchUp.Check(ctx, sync.CheckInput{After: cursor, Epoch: &epoch})
	//	if err == nil && res.Reason == "" {
	//		page, err = catchUp.Page(ctx, sync.PageInput{After: cursor, Head: res.HeadSeq})
	//	}
	Check(ctx context.Context, in CheckInput) (CheckResult, error)
	// Page returns the events with After < seq <= Head that the caller may receive at delivery
	// time: a personal event only to its audience, every other event only when authorize allows
	// its receive action (filter mode, no denial audit). It does not authorize events.catch_up;
	// the caller ran Check first.
	Page(ctx context.Context, in PageInput) (PageResult, error)
	// PageEvents is Page with the admitted events instead of their wire form, for the streaming
	// catch-up of the gateway (M1-65); ctx in delivery mode (authorize.Delivery) checks against
	// the rights cache of the log tail.
	PageEvents(ctx context.Context, in PageInput) (EventPage, error)
	// Admit is the one admission rule of a stored event for the receiver a at delivery time (07
	// section 8.1): a personal event only to its audience, any other only when authorize allows
	// its receive action on the event's resource. It runs in filter mode, so a refusal writes no
	// audit entry. Errors: an unauthenticated decision (the credential of a was revoked) as a
	// kernel/errs error of kind unauthenticated, and the errors of authorize.
	//
	// Example:
	//
	//	ok, err := catchUp.Admit(authorize.Delivery(ctx), recipient, ev)
	Admit(ctx context.Context, a actor.Actor, ev Event) (bool, error)
}

// SnapshotPrincipal is the caller in a snapshot (schema PrincipalRef).
type SnapshotPrincipal struct {
	ID ids.ID
	// Kind is user, agent or app.
	Kind, Handle, DisplayName string
	// AvatarFileID is zero for none.
	AvatarFileID ids.ID
}

// SnapshotSpace is a visible space with the caller's level and capabilities (schema SpaceView).
// Zero values are absent members.
type SnapshotSpace struct {
	ID                     ids.ID
	Name, Icon, Color      string
	SortKey                string
	DefaultOperatorAgentID ids.ID
	CreatedAt, ArchivedAt  time.Time
	Version                int64
	Level                  string
	Capabilities           []string
}

// SnapshotChannel is a visible channel with the caller's level and capabilities (schema
// ChannelView). Zero values are absent members.
type SnapshotChannel struct {
	ID, SpaceID ids.ID
	// Kind is text, forum or dm.
	Kind, Name, Topic, SortKey string
	// OperatorMode is inherit, none or agent; OperatorAgentID and EffectiveOperatorID zero for
	// absent.
	OperatorMode                         string
	OperatorAgentID, EffectiveOperatorID ids.ID
	ForumDefaultSort                     string
	DefaultNotificationLevel             string
	ReadOnly                             bool
	// ProjectBound reports a project binding of the forum (03 project_bindings, R-DM-54).
	ProjectBound             bool
	MainThreadID             ids.ID
	CreatedAt, ArchivedAt    time.Time
	Version, SettingsVersion int64
	Level                    string
	Capabilities             []string
}

// UnreadThread is a thread with something unread for the caller (schema UnreadThread):
// UnreadCount is last_message_seq - GREATEST(last_read_seq, unread_floor_seq), MentionCount the
// unread mentions; both are computed, never stored (R-DM-32).
type UnreadThread struct {
	ThreadID, ChannelID         ids.ID
	UnreadCount, MentionCount   int
	LastMessageSeq, LastReadSeq int64
}

// SnapshotThread is an active thread of the snapshot (schema Thread, as getThread without
// messages; 04 section 6.3 item 4, R-SY-28). Zero values are absent members. Its layout equals
// chat.Thread and rest.ThreadFields, so the conversions are plain type conversions.
type SnapshotThread struct {
	ID, ChannelID ids.ID
	// Kind is thread (the snapshot holds no main thread).
	Kind string
	// ParentMessageID is the message the thread started from; zero for a forum post.
	ParentMessageID ids.ID
	// Title may be empty; TitleSource is auto or manual.
	Title, TitleSource string
	// State is open; ClosedAt and ClosedBy are zero while open.
	State    string
	ClosedAt time.Time
	ClosedBy ids.ID
	// OperatorAgentID is the fixed operator, zero for none; BillingMode its policy, "" for none.
	OperatorAgentID ids.ID
	BillingMode     string
	// LastMessageSeq, UnreadFloorSeq and MessageCount are the counters.
	LastMessageSeq, UnreadFloorSeq int64
	MessageCount                   int
	// CreatedBy is the creator; CreatedAt, LastActivityAt and MovedAt the times.
	CreatedBy                          ids.ID
	CreatedAt, LastActivityAt, MovedAt time.Time
	// Version is the ETag.
	Version int64
	// TagIDs are the live tags of a forum post; never nil.
	TagIDs []ids.ID
}

// Money is an amount in millionths of the currency unit (schema Money).
type Money struct {
	AmountMicros int64
	Currency     string
	IsEstimate   bool
}

// RunSummary is a running run of a visible thread (schema RunSummary). Zero values are absent
// members.
type RunSummary struct {
	ID, ThreadID, ChannelID, AgentID, TriggeredBy ids.ID
	State, StateReason                            string
	Version                                       int64
	BillingModeExpected, BillingModeUsed          string
	StatusLine, Summary                           string
	Cost                                          Money
	StepCount                                     int
	DurationMS                                    int64
	QueuedAt, StartedAt, FinishedAt               time.Time
	// StallWarnedAt is the stall notice of runs.stall_check (09 5.5, M2-51); zero while unset.
	StallWarnedAt time.Time
}

// AgentPresence is the runtime state of a visible agent (schema AgentPresence).
type AgentPresence struct {
	AgentID ids.ID
	// State is unknown, online, degraded or offline.
	State string
	// ReportedBillingMode is the billing mode the adapter reported; "" for none.
	ReportedBillingMode string
}

// DraftRef names a draft and its version (schema DraftRef); ThreadID is zero for a channel draft.
type DraftRef struct {
	ID, ThreadID, ChannelID ids.ID
	Version                 int64
}

// EvaluatedFlag is a feature flag evaluated for the caller (schema EvaluatedFlag).
type EvaluatedFlag struct {
	Key     string
	Enabled bool
}

// SnapshotView is the state of the caller without thread contents (schema Snapshot, 04 section
// 6.3 step 4). Every list is non-nil; for agents and apps Sidebar, UnreadThreads, Settings and
// Drafts are empty.
type SnapshotView struct {
	// HeadSeq and SyncEpoch were read first: every change with seq <= HeadSeq is contained.
	HeadSeq           int64
	SyncEpoch         ids.ID
	PermissionVersion int64
	Principal         SnapshotPrincipal
	IsOwner           bool
	Spaces            []SnapshotSpace
	Channels          []SnapshotChannel
	Sidebar           []SidebarState
	// SidebarChannels are the personal post orders of the caller (M4-51); empty for agents and
	// apps.
	SidebarChannels []SidebarChannelState
	UnreadThreads   []UnreadThread
	ActiveRuns      []RunSummary
	// ActiveThreads are the active threads of 04 section 6.3 item 4 (R-SY-28), in the order of
	// Channels, then by descending last activity.
	ActiveThreads []SnapshotThread
	Agents        []AgentPresence
	Settings      []UserSetting
	Drafts        []DraftRef
	Features      []EvaluatedFlag
}

// Snapshot is the incoming port of the snapshot (04 section 6.3 step 4, M1-63): GET
// /api/v1/sync/snapshot and the ready frame of the gateway (M1-64). The caller is the principal
// in ctx; errors are kernel/errs errors.
type Snapshot interface {
	// Build authorizes sync.snapshot.read on the instance (a pending device gets 403
	// device_pending) and returns the snapshot of the caller. No part names a channel the caller
	// may not read (AZ-P5).
	//
	// Example:
	//
	//	v, err := snapshot.Build(ctx)
	Build(ctx context.Context) (SnapshotView, error)
}
