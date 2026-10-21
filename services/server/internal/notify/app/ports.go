// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/messaging"
	"github.com/OWNER/bothy/services/server/internal/notify"
	"github.com/OWNER/bothy/services/server/internal/notify/domain"
	"github.com/OWNER/bothy/services/server/internal/sync"
)

// KindSync marks the badge sync of 13 section 8 in Notification.Kind: no thread, no message, no
// text (docs/DECISIONS.md D-M3-19-15).
const KindSync = string(domain.KindSync)

// Registration is one push registration as a provider needs it (13 section 5.4). Token is the
// device token; it is never logged, printed or stored outside push_registrations (13 7.3).
type Registration struct {
	// ID is push_registrations.id; UserID the person; DeviceID the device.
	ID, UserID, DeviceID ids.ID
	// Provider is apns, fcm, unifiedpush or wns; Environment production or sandbox; AppID the
	// bundle or package of the app.
	Provider, Environment, AppID string
	// Token is the device token; never logged (13 7.3).
	Token string
	// CreatedAt is created_at (the 410 timestamp rule of 13 7.1).
	CreatedAt time.Time
}

// Notification holds only IDs, the kind, flags, a count and the badge (ADR 0031, T-PU-01): the
// provider adapters build their payload from it (M3-20, M3-21) and never add a text.
type Notification struct {
	// ID is the nid of the hint, shared by the jobs of its registrations; InstanceID the instance
	// record; ThreadID, MessageID and TargetID point at the content. Zero when absent.
	ID, InstanceID, ThreadID, MessageID, TargetID ids.ID
	// Kind is a NotificationKind, or KindSync for the badge sync of 13 8.
	Kind string
	// Quiet delivers without sound (13 3.4).
	Quiet bool
	// Count is 1 for an immediate hint and the unread count of a flush.
	Count int64
	// Badge is the badge number of 13 8 at send time.
	Badge int64
	// HasBadge is true for every hint and every sync this ticket sends.
	HasBadge bool
	// Cleared holds, for KindSync only, the threads read since the last sync, complete or nil.
	Cleared []ids.ID
}

// PushProvider sends one hint to one registration (13 section 5.4, D-M3-19-11). adapters/apns
// (M3-20), adapters/fcm (M3-21) and adapters/fakepush implement it; internal/wire builds the
// provider set, one per Name.
type PushProvider interface {
	// Name returns the value of push_registrations.provider the provider serves.
	Name() string
	// Send sends n to reg with a deadline of the adapter. A Go error counts as a transient
	// failure (domain.ResultRetry).
	Send(ctx context.Context, reg Registration, n Notification) (domain.SendResult, error)
}

// Hint is one decided hint for one recipient, before its push jobs exist.
type Hint struct {
	// NotificationID is the nid (UUIDv7), one per hint and recipient.
	NotificationID ids.ID
	// RecipientID is the person; ThreadID, MessageID and TargetID are zero when absent.
	RecipientID, ThreadID, MessageID, TargetID ids.ID
	// Kind is the kind of the hint.
	Kind domain.Kind
	// Quiet delivers without sound.
	Quiet bool
	// Count is 1, or the unread count of a flush.
	Count int64
	// DueAt is t0 of 13 9: occurred_at of the triggering event, or the window end of a flush.
	DueAt time.Time
}

// LiveRoutes receives the routes gateway and push_delayed of a decision (13 4.6). The default of
// this ticket (FailClosedRoutes) returns an internal error, so the round rolls back; M3-23
// replaces it with notification.created and the delayed push (D-M3-19-9).
type LiveRoutes interface {
	// Gateway delivers h over the live connection of the route's device.
	Gateway(ctx context.Context, h Hint, r domain.Route) error
	// Delayed schedules the push of h to the route's registrations at r.NotBefore.
	Delayed(ctx context.Context, h Hint, r domain.Route) error
}

// PushJob is the content of one job notify.push_send in the terms of the use cases; the jobs
// adapter maps it onto the stored arguments (only IDs and int64).
type PushJob struct {
	// NotificationID, RegistrationID and RecipientID identify the hint, the target and the
	// person; ThreadID, MessageID and TargetID are zero when absent.
	NotificationID, RegistrationID, RecipientID, ThreadID, MessageID, TargetID ids.ID
	// Cleared holds the threads of a badge sync; nil otherwise.
	Cleared []ids.ID
	// Kind is the kind of the hint, or domain.KindSync.
	Kind domain.Kind
	// Quiet delivers without sound.
	Quiet bool
	// Count is the count of the hint.
	Count int64
	// DueAt is t0 of 13 9.
	DueAt time.Time
	// Attempt counts the sends of this hint to this registration, from 1.
	Attempt int64
}

// Scheduler is the outgoing port over the job runtime (adapters/jobs over platform/jobs). Every
// method runs in the ambient unit of work of ctx; the window state of 13 4.5 lives only in these
// jobs (D-M3-19-7).
type Scheduler interface {
	// WindowEnd returns the ScheduledAt of the pending notify.coalesce_window of (user, thread);
	// ok is false when none is pending.
	WindowEnd(ctx context.Context, user, thread ids.ID) (end time.Time, ok bool, err error)
	// OpenWindow inserts the marker notify.coalesce_window of (user, thread) unique at end.
	// inserted is false when a marker is still pending; at is then its ScheduledAt.
	OpenWindow(ctx context.Context, user, thread ids.ID, end time.Time) (inserted bool, at time.Time, err error)
	// Collect inserts notify.coalesce_flush of (user, thread) unique at end; inserted is false
	// when one is pending (the window already collected).
	Collect(ctx context.Context, user, thread ids.ID, end time.Time) (inserted bool, err error)
	// Send inserts one job notify.push_send at at (zero: now).
	Send(ctx context.Context, job PushJob, at time.Time) error
	// BadgeSyncPending reports a pending notify.badge_sync of user.
	BadgeSyncPending(ctx context.Context, user ids.ID) (bool, error)
	// HoldEnd returns the ScheduledAt of the pending notify.badge_sync_hold of user.
	HoldEnd(ctx context.Context, user ids.ID) (end time.Time, ok bool, err error)
	// ScheduleBadgeSync inserts notify.badge_sync of user unique at at.
	ScheduleBadgeSync(ctx context.Context, user ids.ID, afterSeq int64, at time.Time) error
	// Hold inserts notify.badge_sync_hold of user unique at at.
	Hold(ctx context.Context, user ids.ID, at time.Time) error
}

// RegistrationStore is the outgoing port of the job notify.push_send over push_registrations
// (adapters/postgres). It reads the token, which the port notify.PushRegistrations never returns.
type RegistrationStore interface {
	// Load returns the registration id with its token and delivery state in the ambient
	// transaction; found is false when it does not exist.
	Load(ctx context.Context, id ids.ID) (reg Registration, state domain.RegistrationState, found bool, err error)
	// Lock returns the delivery state of id locked FOR UPDATE in the ambient transaction; found is
	// false when it does not exist.
	Lock(ctx context.Context, id ids.ID) (state domain.RegistrationState, found bool, err error)
	// Update writes the delivery columns (last_success_at, failure_count, disabled_at,
	// disabled_reason) of id; created_at is never written.
	Update(ctx context.Context, id ids.ID, state domain.RegistrationState) error
	// ActiveCounts returns the number of rows with disabled_at IS NULL per provider.
	ActiveCounts(ctx context.Context) (map[string]int64, error)
}

// ReaderSource is authz.Recipients: the candidate readers of a channel.
type ReaderSource interface {
	Readers(ctx context.Context, channelID ids.ID) ([]ids.ID, error)
}

// PrincipalSource is the part of identity.Directory the consumer uses: kind and status of the
// candidates (only active humans are recipients).
type PrincipalSource interface {
	Principals(ctx context.Context, ids []ids.ID) (map[ids.ID]identity.PrincipalView, error)
}

// DeviceSource is the part of identity.Devices the consumer uses (devices.state = 'active').
type DeviceSource interface {
	ActiveForUsers(ctx context.Context, userIDs []ids.ID) (map[ids.ID][]identity.DeviceRef, error)
}

// HistorySource is sync.History.
type HistorySource interface {
	ResourceEvents(ctx context.Context, q sync.ResourceQuery) ([]sync.Event, error)
	PersonalEvents(ctx context.Context, q sync.PersonalQuery) ([]sync.Event, error)
}

// ReadPositions is the part of sync.ReadStates the consumer uses: read_states.last_read_seq,
// an absent row read as 0.
type ReadPositions interface {
	PositionsFor(ctx context.Context, threadID ids.ID, userIDs []ids.ID) (map[ids.ID]int64, error)
	PositionsOfUser(ctx context.Context, userID ids.ID, threadIDs []ids.ID) (map[ids.ID]int64, error)
}

// ThreadSource is the part of chat.Threads the consumer uses; neither method authorizes.
type ThreadSource interface {
	Info(ctx context.Context, threadID ids.ID) (chat.ThreadInfo, error)
	UnreadCandidates(ctx context.Context, channelIDs []ids.ID) ([]chat.ThreadPosition, error)
}

// MessageSource is the part of messaging.Messages the consumer uses (design note 5); none of
// these reads authorizes.
type MessageSource interface {
	LastAnchorBefore(ctx context.Context, threadID ids.ID, beforeThreadSeq int64) (messaging.AnchorRef, bool, error)
	AnchorOf(ctx context.Context, runID ids.ID) (messaging.AnchorRef, bool, error)
	LastAgentMessageBefore(ctx context.Context, threadID, agentID ids.ID, afterThreadSeq, beforeThreadSeq int64) (messaging.MessageRef, bool, error)
	Newest(ctx context.Context, threadID ids.ID) (messaging.MessageRef, bool, error)
}

// MentionSource is the part of messaging.Mentions the badge uses.
type MentionSource interface {
	CountUnread(ctx context.Context, q messaging.UnreadMentionQuery) ([]messaging.ThreadMentionCount, error)
}

// RegistrationSource is notify.PushRegistrations.
type RegistrationSource = notify.PushRegistrations

// Metrics records the push signals of docs/observability/signals.md 4.4 (bothy.push.sent,
// bothy.push.send.duration, bothy.push.skipped). Attribute values are codes only, never a person,
// a thread, a token or a text.
type Metrics interface {
	// Sent counts one provider answer with bothy.push.provider and bothy.outcome.
	Sent(ctx context.Context, provider, outcome string)
	// SendDuration records the duration of one provider request.
	SendDuration(ctx context.Context, provider string, d time.Duration)
	// Skipped counts one hint or job that ends without sending, with bothy.reason.
	Skipped(ctx context.Context, reason string)
}

// NopMetrics records nothing; tests and processes without a meter use it.
type NopMetrics struct{}

// Sent implements Metrics.
func (NopMetrics) Sent(context.Context, string, string) {}

// SendDuration implements Metrics.
func (NopMetrics) SendDuration(context.Context, string, time.Duration) {}

// Skipped implements Metrics.
func (NopMetrics) Skipped(context.Context, string) {}
