// SPDX-License-Identifier: Apache-2.0

package domain

import "slices"

// Kind is the kind of a hint, the schema NotificationKind of docs/api/openapi.yaml and the first
// column of the table in 13 section 2. The kind decides the recipients and the level a hint needs.
type Kind string

// The kinds of 13 section 2, in the order of NotificationKind.
const (
	// KindMessage is a new message for everyone with level all, except the author.
	KindMessage Kind = "message"
	// KindMention is a message that mentions the recipient.
	KindMention Kind = "mention"
	// KindDM is a message in a DM channel, also when it mentions the recipient.
	KindDM Kind = "dm"
	// KindRunDone is a completed run, for the person who triggered it.
	KindRunDone Kind = "run_done"
	// KindRunFailed is a failed run, for the person who triggered it.
	KindRunFailed Kind = "run_failed"
	// KindInputNeeded is a run waiting for input, for the person who triggered it; it comes at
	// every level (R-PN-03).
	KindInputNeeded Kind = "input_needed"
	// KindSecurity is a new device of the recipient waiting for approval; it has no thread.
	KindSecurity Kind = "security"
	// KindAdmin is a reminder of the administration; it has no thread and depends only on the
	// switch notify.admin_enabled.
	KindAdmin Kind = "admin"
)

var kinds = []Kind{KindMessage, KindMention, KindDM, KindRunDone, KindRunFailed, KindInputNeeded, KindSecurity, KindAdmin}

// Kinds returns every kind in the order of NotificationKind; the slice is a fresh copy.
func Kinds() []Kind { return slices.Clone(kinds) }

// Valid reports whether k is one of [Kinds].
func (k Kind) Valid() bool { return slices.Contains(kinds, k) }

// HasThread reports whether hints of this kind point at a thread. Only those have an effective
// level, an activity in the thread and a coalescing window; security and admin do not (13 2, 4.4,
// 4.5, D-M3-17-2).
func (k Kind) HasThread() bool { return k.Valid() && k != KindSecurity && k != KindAdmin }

// Urgent reports whether a hint of this kind goes at once even in an open coalescing window
// (13 4.5 point 3: mention, dm, input_needed, run_done, run_failed and security). Admin has no
// window at all, so it is not listed there and is not urgent.
func (k Kind) Urgent() bool { return k.Valid() && k != KindMessage && k != KindAdmin }

// Level is a notification level, the schema NotificationLevel and the column
// notification_preferences.level of 03.
type Level string

// The levels of 13 3.1 and 4.3.
const (
	// LevelAll allows every kind.
	LevelAll Level = "all"
	// LevelMentions allows every kind except message.
	LevelMentions Level = "mentions"
	// LevelMute allows only input_needed of an own run, security and admin.
	LevelMute Level = "mute"
)

var levels = []Level{LevelAll, LevelMentions, LevelMute}

// Levels returns every level in the order of NotificationLevel; the slice is a fresh copy.
func Levels() []Level { return slices.Clone(levels) }

// Valid reports whether l is one of [Levels]; the empty level is not valid.
func (l Level) Valid() bool { return slices.Contains(levels, l) }

// allows reports whether level l lets a hint of kind k with thread through (13 4.3). Security and
// admin are not asked: they have no thread and no level.
func (l Level) allows(k Kind) bool {
	switch l {
	case LevelAll:
		return true
	case LevelMentions:
		return k != KindMessage
	case LevelMute:
		return k == KindInputNeeded
	}
	return false
}

// Platform is the platform of a device, the check list of devices.platform in 03.
type Platform string

// The platforms of 03 devices.platform.
const (
	// PlatformIOS is an iPhone or iPad; mobile.
	PlatformIOS Platform = "ios"
	// PlatformAndroid is an Android device; mobile.
	PlatformAndroid Platform = "android"
	// PlatformMacOS is the macOS app; desktop.
	PlatformMacOS Platform = "macos"
	// PlatformWindows is the Windows app; desktop.
	PlatformWindows Platform = "windows"
	// PlatformWeb is the web client.
	PlatformWeb Platform = "web"
	// PlatformCLI is the command line client.
	PlatformCLI Platform = "cli"
)

var platforms = []Platform{PlatformIOS, PlatformAndroid, PlatformMacOS, PlatformWindows, PlatformWeb, PlatformCLI}

// Platforms returns every platform in the order of the check in 03; the slice is a fresh copy.
func Platforms() []Platform { return slices.Clone(platforms) }

// Valid reports whether p is one of [Platforms].
func (p Platform) Valid() bool { return slices.Contains(platforms, p) }

// Desktop reports whether a connected, not idle device of this platform makes the desktop active
// (13 4.4 "Desktop active": platform IN ('macos','windows')).
func (p Platform) Desktop() bool { return p == PlatformMacOS || p == PlatformWindows }

// Mobile reports whether pushes to this platform wait while the desktop is active (13 4.4).
func (p Platform) Mobile() bool { return p == PlatformIOS || p == PlatformAndroid }

// LevelSource names the step of 13 3.1 that gave the effective level.
type LevelSource string

// The seven steps of 13 3.1, first present value wins.
const (
	// SourceThread is the own row for the thread.
	SourceThread LevelSource = "thread"
	// SourceChannel is the own row for the channel.
	SourceChannel LevelSource = "channel"
	// SourceSpace is the own row for the space.
	SourceSpace LevelSource = "space"
	// SourceChannelDefault is channels.default_notification_level.
	SourceChannelDefault LevelSource = "channel_default"
	// SourceDirectMessage is the level all of a DM channel.
	SourceDirectMessage LevelSource = "direct_message"
	// SourcePersonalDefault is user_settings['notify.default_level'].
	SourcePersonalDefault LevelSource = "personal_default"
	// SourceInstanceDefault is the value of notify.user_default_level.
	SourceInstanceDefault LevelSource = "instance_default"
)

// Outcome is what happens with one hint for one recipient. The zero value is [OutcomeNone], so the
// zero [Decision] never sends.
type Outcome uint8

// The three outcomes of design note 3.
const (
	// OutcomeNone means no hint; Decision.Skip says why.
	OutcomeNone Outcome = iota
	// OutcomeCollect means the hint joins the open coalescing window of (recipient, thread); the
	// flush at the window end decides the one hint.
	OutcomeCollect
	// OutcomeSend means the hint goes now along Decision.Routes.
	OutcomeSend
)

// String returns none, collect or send, and "invalid" for any other value.
func (o Outcome) String() string {
	switch o {
	case OutcomeNone:
		return "none"
	case OutcomeCollect:
		return "collect"
	case OutcomeSend:
		return "send"
	}
	return "invalid"
}

// Valid reports whether o is one of the three outcomes.
func (o Outcome) Valid() bool { return o <= OutcomeSend }

// SkipReason says why a decision is [OutcomeNone]. The values are internal constants of this
// package, not API values; M3-19 maps them to the attribute values of bothy.push.skipped. No
// reason stands for quiet hours: they make a hint quiet and never drop it (R-PN-05).
type SkipReason string

// The skip reasons of Decide and Flush.
const (
	// SkipNotAllowed: push.deliver denied the recipient (13 4.2).
	SkipNotAllowed SkipReason = "not_allowed"
	// SkipNotRecipient: the recipient column of 13 2 excludes the person (the author of the
	// message, or not the person who triggered the run).
	SkipNotRecipient SkipReason = "not_recipient"
	// SkipLevel: the effective level does not allow the kind (13 4.3).
	SkipLevel SkipReason = "level"
	// SkipAdminDisabled: an admin hint while notify.admin_enabled is off.
	SkipAdminDisabled SkipReason = "admin_disabled"
	// SkipActiveInThread: a device of the person is active in the thread (13 4.4).
	SkipActiveInThread SkipReason = "active_in_thread"
	// SkipNoRoute: no active device has a live connection or a push registration (13 4.6).
	SkipNoRoute SkipReason = "no_route"
	// SkipNothingCollected: a flush of a window in which nothing was collected.
	SkipNothingCollected SkipReason = "nothing_collected"
	// SkipRead: a flush after the person read every collected message.
	SkipRead SkipReason = "read"
)

// Via is the delivery path of one route (13 4.6).
type Via string

// The delivery paths of design note 8.
const (
	// ViaGateway sends the ephemeral event notification.created over the live connection.
	ViaGateway Via = "gateway"
	// ViaPush sends a push to every registration of the device now.
	ViaPush Via = "push"
	// ViaPushDelayed sends the push at Route.NotBefore, unless ReleaseDelayed drops it.
	ViaPushDelayed Via = "push_delayed"
)

var (
	levelSources = []LevelSource{
		SourceThread, SourceChannel, SourceSpace, SourceChannelDefault,
		SourceDirectMessage, SourcePersonalDefault, SourceInstanceDefault,
	}
	skipReasons = []SkipReason{
		SkipNotAllowed, SkipNotRecipient, SkipLevel, SkipAdminDisabled,
		SkipActiveInThread, SkipNoRoute, SkipNothingCollected, SkipRead,
	}
	vias = []Via{ViaGateway, ViaPush, ViaPushDelayed}
)

// Valid reports whether s is one of the seven steps of 13 3.1.
func (s LevelSource) Valid() bool { return slices.Contains(levelSources, s) }

// Valid reports whether r is one of the skip reasons of this package; the empty reason, which a
// decision other than none carries, is not.
func (r SkipReason) Valid() bool { return slices.Contains(skipReasons, r) }

// Valid reports whether v is one of the three delivery paths.
func (v Via) Valid() bool { return slices.Contains(vias, v) }
