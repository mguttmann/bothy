// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"slices"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Switches are the personal switches of 13 3.3 that the decision reads, already resolved by the
// caller with their defaults (notify.default_admin_enabled,
// notify.default_input_needed_breaks_quiet_hours).
type Switches struct {
	// AdminEnabled is notify.admin_enabled: admin hints reach the person.
	AdminEnabled bool
	// InputNeededBreaksQuietHours is notify.input_needed_breaks_quiet_hours: input_needed is
	// loud also in quiet hours.
	InputNeededBreaksQuietHours bool
}

// Connection is the live gateway connection of a device; a device holds at most one (04 6.1).
// The presence port of the gateway supplies it (M3-23).
type Connection struct {
	// Focus holds the threads of the last focus op (04 6.2).
	Focus []ids.ID
	// Idle is the idle field of the last focus op: the window is open but unused.
	Idle bool
}

// Device is an active device of the recipient (devices.state = 'active').
type Device struct {
	// ID is devices.id; unique within RecipientFacts.Devices.
	ID ids.ID
	// Platform is devices.platform.
	Platform Platform
	// Connection is the live gateway connection; nil when the device is not connected.
	Connection *Connection
	// Registrations are the active push registrations of the device (disabled_at IS NULL).
	Registrations []ids.ID
}

// Window is the coalescing window of (recipient, thread) as the caller keeps it (M3-19).
type Window struct {
	// End is the end of the window; the zero time means no window. The window is open while
	// "now" is before End and closed from End on.
	End time.Time
}

// open reports whether the window is open at now.
func (w Window) open(now time.Time) bool { return !w.End.IsZero() && now.Before(w.End) }

// Policy holds the values of the configuration keys the decision needs; the caller reads them.
type Policy struct {
	// CoalesceWindow is notify.coalesce_window; zero opens no new window, so every hint goes at
	// once, except a message while a window opened earlier is still open.
	CoalesceWindow time.Duration
	// MobileDelay is notify.mobile_delay_when_desktop_active; zero delays nothing.
	MobileDelay time.Duration
}

// RecipientFacts are the facts about one hint for one person r. The README of the notify module
// lists which module or port supplies each field.
type RecipientFacts struct {
	// Kind is the kind of the hint.
	Kind Kind
	// Now is the decision time; required, the package reads no clock.
	Now time.Time
	// ThreadID is the thread the hint points at; zero for security and admin, required otherwise.
	ThreadID ids.ID
	// Allowed is the result of push.deliver for r on the thread, with the rights at decision time;
	// for security and admin it is the recipient rule of 13 2 as the caller evaluates it. It is
	// false for a principal that is not an active human.
	Allowed bool
	// Author reports that r wrote the message the hint points at.
	Author bool
	// RunStarter reports that r is runs.triggered_by of the run the hint points at.
	RunStarter bool
	// Level holds the sources of the effective level; read for kinds with thread only.
	Level LevelFacts
	// Switches are the personal switches of r.
	Switches Switches
	// QuietHours holds every quiet-hour row of r, enabled or not.
	QuietHours []QuietRule
	// Devices holds every active device of r.
	Devices []Device
	// Window is the coalescing window of (r, thread); read for kinds with thread only, and not by
	// Flush.
	Window Window
	// Policy holds the configuration values.
	Policy Policy
}

// Route is the delivery of a hint to one device (13 4.6).
type Route struct {
	// DeviceID is the device.
	DeviceID ids.ID
	// Via is the delivery path.
	Via Via
	// Registrations are all active registrations of the device for push and push_delayed; nil for
	// gateway. The slice is a copy of the input.
	Registrations []ids.ID
	// NotBefore is Now + Policy.MobileDelay for push_delayed, zero otherwise.
	NotBefore time.Time
}

// Decision is the outcome for one hint and one recipient. It carries only IDs, kinds, flags,
// counts and times, never a text (ADR 0031, T-PU-01).
type Decision struct {
	// Outcome is none, collect or send; the zero Decision is none.
	Outcome Outcome
	// Skip is the reason of a none; empty otherwise.
	Skip SkipReason
	// Kind is the kind of the hint; empty only in the zero Decision of an error.
	Kind Kind
	// Effective is the effective level for kinds with thread once the level step ran; zero before
	// that step and for security and admin.
	Effective EffectiveLevel
	// Quiet means deliver without sound (13 3.4); only with send.
	Quiet bool
	// Count is 1 for an immediate hint and the unread count for a flush; only with send.
	Count int64
	// OpensWindowUntil is the end of the coalescing window this hint opens; only with send, zero when
	// it opens none.
	OpensWindowUntil time.Time
	// Routes holds one route per device that has one, in the order of RecipientFacts.Devices;
	// only with send.
	Routes []Route
}

// Decide decides one hint for one person by the flow of 13 4.1 (docs/DECISIONS.md D-M3-17-1 to
// D-M3-17-7). The steps run in this order and the first that decides wins:
//
//  1. validate: a broken input returns an internal_error and the zero Decision;
//  2. allowed: Allowed false gives none with not_allowed;
//  3. recipient: message, mention or dm for their author, or run_done, run_failed or
//     input_needed for anyone but the person who triggered the run, give none with not_recipient;
//  4. level: a kind with thread that the effective level (ResolveLevel) does not allow gives none
//     with level; admin without Switches.AdminEnabled gives none with admin_disabled; security
//     passes at every level;
//  5. activity: a kind with thread while a device is active in the thread gives none with
//     active_in_thread;
//  6. quiet: quiet hours (InQuietHours) make a send quiet, except input_needed with
//     Switches.InputNeededBreaksQuietHours;
//  7. window: message in an open window gives collect without routes;
//  8. route: no device with a route gives none with no_route, and opens no window;
//  9. send: Count 1, the routes, and for a kind with thread outside an open window
//     OpensWindowUntil = Now + Policy.CoalesceWindow (zero window length: none).
//
// Security and admin have no thread: no level, no activity in a thread and no window apply, quiet
// hours do. Decide never mutates in and returns copies of its slices.
//
// Errors: an internal_error of the kernel (errs.CodeInternalError) and the zero Decision for an
// unknown kind, level or platform; a zero Now; a negative CoalesceWindow or MobileDelay; a kind
// with thread and a zero ThreadID, or security or admin with a ThreadID; an empty or unknown
// InstanceDefault for a kind with thread; dm outside a DM channel or another kind with thread in
// one; a zero or duplicate device ID, a zero registration ID or a zero thread ID in a focus; a
// broken quiet-hour rule (see InQuietHours). The text names the field, never a value.
func Decide(in RecipientFacts) (Decision, error) {
	if err := validate(in); err != nil {
		return Decision{}, err
	}
	return decide(in, false, 1)
}

// decide runs steps 2 to 9 on a valid input; flush leaves out the recipient and window steps and
// sends count instead of 1.
func decide(in RecipientFacts, flush bool, count int64) (Decision, error) {
	d := Decision{Kind: in.Kind}
	if !in.Allowed {
		return skip(d, SkipNotAllowed), nil
	}
	if !flush && !in.recipient() {
		return skip(d, SkipNotRecipient), nil
	}
	if reason, err := in.levelAndActivity(&d); err != nil || reason != "" {
		return skip(d, reason), err
	}
	quiet, err := InQuietHours(in.QuietHours, in.Now)
	if err != nil {
		return Decision{}, err
	}
	quiet = quiet && (in.Kind != KindInputNeeded || !in.Switches.InputNeededBreaksQuietHours)
	windowOpen := !flush && in.Kind.HasThread() && in.Window.open(in.Now)
	if windowOpen && !in.Kind.Urgent() {
		d.Outcome = OutcomeCollect
		return d, nil
	}
	routes := in.routes()
	if len(routes) == 0 {
		return skip(d, SkipNoRoute), nil
	}
	d.Outcome, d.Quiet, d.Count, d.Routes = OutcomeSend, quiet, count, routes
	if in.Kind.HasThread() && !windowOpen && in.Policy.CoalesceWindow > 0 {
		d.OpensWindowUntil = in.Now.Add(in.Policy.CoalesceWindow)
	}
	return d, nil
}

// skip returns d as none with the reason; an empty reason (an error path) gives the zero Decision.
func skip(d Decision, reason SkipReason) Decision {
	if reason == "" {
		return Decision{}
	}
	d.Outcome, d.Skip = OutcomeNone, reason
	return d
}

// recipient applies the recipient column of 13 2 (D-M3-17-2).
func (in RecipientFacts) recipient() bool {
	switch in.Kind {
	case KindMessage, KindMention, KindDM:
		return !in.Author
	case KindRunDone, KindRunFailed, KindInputNeeded:
		return in.RunStarter
	case KindSecurity, KindAdmin:
		return true
	}
	return false
}

// levelAndActivity runs steps 4 and 5; it sets d.Effective for kinds with thread and returns the
// skip reason, or "" when the hint passes.
func (in RecipientFacts) levelAndActivity(d *Decision) (SkipReason, error) {
	if !in.Kind.HasThread() {
		if in.Kind == KindAdmin && !in.Switches.AdminEnabled {
			return SkipAdminDisabled, nil
		}
		return "", nil
	}
	eff, err := ResolveLevel(in.Level, in.Now)
	if err != nil {
		return "", err
	}
	d.Effective = eff
	if !eff.Level.allows(in.Kind) {
		return SkipLevel, nil
	}
	if in.activeInThread() {
		return SkipActiveInThread, nil
	}
	return "", nil
}

// activeInThread reports 13 4.4 "Active in the thread": a device with a connection that is not idle and
// whose focus contains the thread, on any platform.
func (in RecipientFacts) activeInThread() bool {
	for _, d := range in.Devices {
		if d.Connection != nil && !d.Connection.Idle && slices.Contains(d.Connection.Focus, in.ThreadID) {
			return true
		}
	}
	return false
}

// desktopActive reports 13 4.4 "Desktop active": a macos or windows device connected and not idle.
func (in RecipientFacts) desktopActive() bool {
	for _, d := range in.Devices {
		if d.Platform.Desktop() && d.Connection != nil && !d.Connection.Idle {
			return true
		}
	}
	return false
}

// routes applies 13 4.6 per device, in the order of Devices (D-M3-17-6): a connection, idle or
// not, gets the gateway; otherwise the registrations get a push, delayed for ios and android while
// the desktop is active and the delay is above zero; a device with neither gets nothing.
func (in RecipientFacts) routes() []Route {
	delay := in.Policy.MobileDelay > 0 && in.desktopActive()
	var out []Route
	for _, d := range in.Devices {
		switch {
		case d.Connection != nil:
			out = append(out, Route{DeviceID: d.ID, Via: ViaGateway})
		case len(d.Registrations) == 0:
			continue
		case delay && d.Platform.Mobile():
			out = append(out, Route{
				DeviceID: d.ID, Via: ViaPushDelayed, Registrations: slices.Clone(d.Registrations),
				NotBefore: in.Now.Add(in.Policy.MobileDelay),
			})
		default:
			out = append(out, Route{DeviceID: d.ID, Via: ViaPush, Registrations: slices.Clone(d.Registrations)})
		}
	}
	return out
}

// validate applies design note 10 to the facts Decide and Flush share (D-M3-17-7).
func validate(in RecipientFacts) error {
	if !in.Kind.Valid() {
		return invalid("kind")
	}
	if in.Now.IsZero() {
		return invalid("now")
	}
	if in.Policy.CoalesceWindow < 0 {
		return invalid("policy.coalesce_window")
	}
	if in.Policy.MobileDelay < 0 {
		return invalid("policy.mobile_delay")
	}
	if err := validateThread(in); err != nil {
		return err
	}
	if err := validateRules(in.QuietHours, in.Now); err != nil {
		return err
	}
	return validateDevices(in.Devices)
}

// validateThread checks the thread, the level sources and the DM rule of design note 5.
func validateThread(in RecipientFacts) error {
	if !in.Kind.HasThread() {
		if !in.ThreadID.IsZero() {
			return invalid("thread_id")
		}
		return nil
	}
	if in.ThreadID.IsZero() {
		return invalid("thread_id")
	}
	if err := validateLevelFacts(in.Level, in.Now); err != nil {
		return err
	}
	if (in.Kind == KindDM) != in.Level.DirectMessage {
		return invalid("level.direct_message")
	}
	return nil
}

func validateDevices(devices []Device) error {
	seen := make(map[ids.ID]bool, len(devices))
	for _, d := range devices {
		if d.ID.IsZero() || seen[d.ID] {
			return invalid("devices.id")
		}
		seen[d.ID] = true
		if !d.Platform.Valid() {
			return invalid("devices.platform")
		}
		if slices.ContainsFunc(d.Registrations, ids.ID.IsZero) {
			return invalid("devices.registrations")
		}
		if d.Connection != nil && slices.ContainsFunc(d.Connection.Focus, ids.ID.IsZero) {
			return invalid("devices.connection.focus")
		}
	}
	return nil
}
