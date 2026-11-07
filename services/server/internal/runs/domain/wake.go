// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"encoding/json"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/textcheck"
)

// WakeKind is run_wakes.kind (03 4.8, 09 5.8.1): what makes a wake-up fire.
type WakeKind string

// The kinds of 09 5.8.1.
const (
	// WakeAt fires at a time; the server job runs.wake_due evaluates it.
	WakeAt WakeKind = "at"
	// WakePRChecks fires when the head of a pull request has checks and none is pending; the runner
	// evaluates it with the host user's gh login (owners only).
	WakePRChecks WakeKind = "pr_checks"
	// WakeCommand fires when a command line exits 0 in the turn's folder; the runner evaluates it
	// (tool class host only).
	WakeCommand WakeKind = "command"
)

// Valid reports whether k is one of the three kinds.
func (k WakeKind) Valid() bool { return k == WakeAt || k == WakePRChecks || k == WakeCommand }

// WakeReason is run_wakes.reason: why the wake-up exists.
type WakeReason string

// The reasons of 09 5.8.1.
const (
	// WakeReasonAgent is a wake-up the agent asked for with bothy-wake.
	WakeReasonAgent WakeReason = "agent"
	// WakeReasonRecovery is a recovery run after lost background work (18.19.9).
	WakeReasonRecovery WakeReason = "recovery"
	// WakeReasonUsageLimit is a wake-up at the reset of the account's usage window (18.19.10).
	WakeReasonUsageLimit WakeReason = "usage_limit"
)

// Valid reports whether r is one of the three reasons.
func (r WakeReason) Valid() bool {
	return r == WakeReasonAgent || r == WakeReasonRecovery || r == WakeReasonUsageLimit
}

// WakeState is run_wakes.state.
type WakeState string

// The states of 09 5.8.1. Every state other than pending is an end (ended_at is set).
const (
	WakePending   WakeState = "pending"
	WakeFired     WakeState = "fired"
	WakeCancelled WakeState = "cancelled"
	WakeExpired   WakeState = "expired"
	WakeRefused   WakeState = "refused"
	WakeFailed    WakeState = "failed"
)

// Ended reports whether s is an end state.
func (s WakeState) Ended() bool { return s != WakePending && s != "" }

// Notifies reports whether an end in s notifies the wake-up's person (09 5.8.4, 13 section 2):
// refused, expired and failed do; fired and a cancel do not.
func (s WakeState) Notifies() bool { return s == WakeRefused || s == WakeExpired || s == WakeFailed }

// The reason codes of run_wakes.state_reason and of the 409 refusals of the wake-up operations
// (09 5.8.3, 5.8.6, 18.19.10). Codes that a refused check of agent.invoke yields are passed
// through as the authorize reason (for example agent_not_reachable, channel_read_only,
// tool_class_owner_only, capability_missing).
const (
	// WakeCodeMaxPending: the thread already has run.wake_max_pending pending wake-ups.
	WakeCodeMaxPending = "wake_max_pending"
	// WakeCodeAutonomousNotAllowed: the person is no owner and the channel has no
	// allow_autonomous_runs.
	WakeCodeAutonomousNotAllowed = "autonomous_not_allowed"
	// WakeCodeChainMax: the chain count reached run.autonomous_chain_max.
	WakeCodeChainMax = "autonomous_chain_max"
	// WakeCodeIdleMax: the idle count reached run.autonomous_idle_max.
	WakeCodeIdleMax = "autonomous_idle_max"
	// WakeCodePRChecksOwnerOnly: a pr_checks wake-up of a person who is no owner.
	WakeCodePRChecksOwnerOnly = "pr_checks_owner_only"
	// WakeCodeToolClassMismatch: a command wake-up of an agent whose tool class is not host.
	WakeCodeToolClassMismatch = "tool_class_mismatch"
	// WakeCodeUsageWakeMax: the run already has run.usage_wake_max usage-limit wake-ups.
	WakeCodeUsageWakeMax = "usage_wake_max"
	// WakeCodeRunNotActive: run.wake.create for a run that is not active (a deferred request).
	WakeCodeRunNotActive = "run_not_active"
	// WakeCodeRunNotEnded: run.wake.recover for a run that is not interrupted, stale or failed.
	WakeCodeRunNotEnded = "run_not_ended"
	// WakeCodeNotPending: an operation on a wake-up that already ended.
	WakeCodeNotPending = "wake_not_pending"
	// WakeCodeDeadline: the deadline passed before the wake-up fired (state expired).
	WakeCodeDeadline = "deadline_passed"
	// WakeCodeCancelledByAgent and WakeCodeCancelledByPerson are the reasons of a cancel.
	WakeCodeCancelledByAgent  = "cancelled_by_agent"
	WakeCodeCancelledByPerson = "cancelled_by_person"
	// WakeCodeInputSkipped: the runner skipped a wake input; the wake-up waits for the run's end
	// and fires as a wake run (09 5.8.3).
	WakeCodeInputSkipped = "wake_input_skipped"
	// WakeCodeCheckFailed: the default reason of run.wake.fire with outcome failed.
	WakeCodeCheckFailed = "wake_check_failed"
	// WakeCodeRunNotRunning: a wake input whose run no longer runs at its delivery (the wake-up
	// returns to pending), and run.launch.check for a run that is not running.
	WakeCodeRunNotRunning = "run_not_running"
	// WakeCodeNotDue: run.wake.check or run.wake.fire before not_before; the wake-up stays pending.
	WakeCodeNotDue = "wake_not_due"
	// WakeCodeFiredByServer: run.wake.fire for an at wake-up, which only runs.wake_due fires.
	WakeCodeFiredByServer = "wake_fired_by_server"
	// WakeCodeConditionMet marks a pending pr_checks or command wake-up whose condition the runner
	// reported while it could not fire yet (another person's run is active); runs.wake_due fires it.
	WakeCodeConditionMet = "condition_met"
)

// reasonCodePattern is the shape of a reason code a runner sends (RunWakeFire.reason,
// RunWakeCancel.reason).
var reasonCodePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_.:-]{0,63}$`)

// IsReasonCode reports whether s is a reason code of at most 64 characters.
func IsReasonCode(s string) bool { return reasonCodePattern.MatchString(s) }

// CheckLabel checks a label of a wake-up (03 run_wakes.label): at most maxChars characters
// (run.wake_label_max_chars) and a display text. Errors: validation_failed on /label.
func CheckLabel(label string, maxChars int) error {
	f := &wakeFindings{}
	checkLabel(f, label, maxChars)
	return f.err()
}

// WakeCondition is run_wake_condition.v1 (03 run_wakes.condition): the repository, number and
// head commit of pr_checks, the command line of command, nothing for at.
type WakeCondition struct {
	Repository  string `json:"repository,omitempty"`
	Number      int    `json:"number,omitempty"`
	HeadCommit  string `json:"head_commit,omitempty"`
	CommandLine string `json:"command_line,omitempty"`
}

// The shapes of a condition: owner/repo of GitHub and a commit as hex.
var (
	repositoryPattern = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,100}/[A-Za-z0-9_.-]{1,100}$`)
	commitPattern     = regexp.MustCompile(`^[0-9a-f]{7,64}$`)
)

// WakeLimits are the configuration values the checks of a wake-up read (09 21, stage AR).
type WakeLimits struct {
	// LabelMaxChars is run.wake_label_max_chars.
	LabelMaxChars int
	// TextMaxBytes is message.max_content_bytes, the bound of a command line and a result text.
	TextMaxBytes int
	// MinDelay is run.wake_min_delay; MaxWait is run.wake_max_wait.
	MinDelay, MaxWait time.Duration
}

// WakeRequest is the body of run.wake.create as the domain checks it.
type WakeRequest struct {
	Kind   WakeKind
	Reason WakeReason
	Label  string
	// Condition is the condition of pr_checks and command.
	Condition WakeCondition
	// NotBefore is the requested fire time of at; zero for none (now plus the minimum delay).
	NotBefore time.Time
	// Deadline is the requested deadline; zero for the longest one.
	Deadline time.Time
	// Outage is the run's outage number of a usage_limit request (at least 1); 0 for none.
	Outage int
}

// CheckWakeRequest checks the fields of r (06 R-API-30: a wrong field is 400 validation_failed):
// kind and reason known, reason agent or usage_limit, usage_limit only of kind at with an outage,
// an outage only with usage_limit, the label (at most lim.LabelMaxChars characters, a display text),
// the condition of the kind and no other member. Errors: one validation_failed with a field error
// per finding.
//
// Example:
//
//	err := domain.CheckWakeRequest(domain.WakeRequest{Kind: domain.WakeAt, Reason: domain.WakeReasonAgent, Label: "CI of #123"}, lim)
func CheckWakeRequest(r WakeRequest, lim WakeLimits) error {
	f := &wakeFindings{}
	if !r.Kind.Valid() {
		f.add("/kind", "invalid", "unknown kind")
	}
	switch r.Reason {
	case WakeReasonAgent:
		if r.Outage != 0 {
			f.add("/outage", "not_allowed", "an outage needs the reason usage_limit")
		}
	case WakeReasonUsageLimit:
		if r.Kind != WakeAt {
			f.add("/kind", "not_allowed", "a usage_limit wake-up is of kind at")
		}
		if r.Outage < 1 {
			f.add("/outage", "required", "a usage_limit request carries its outage number")
		}
	case WakeReasonRecovery:
		f.add("/reason", "not_allowed", "recovery comes only through run.wake.recover")
	default:
		f.add("/reason", "invalid", "unknown reason")
	}
	checkLabel(f, r.Label, lim.LabelMaxChars)
	checkCondition(f, r.Kind, r.Condition, lim.TextMaxBytes)
	if r.Kind != WakeAt && !r.NotBefore.IsZero() {
		f.add("/not_before", "not_allowed", "only an at wake-up takes a time")
	}
	return f.err()
}

// checkLabel checks a label: at most maxChars characters and a display text.
func checkLabel(f *wakeFindings, label string, maxChars int) {
	if utf8.RuneCountInString(label) > maxChars {
		f.add("/label", "too_long", "the label is longer than run.wake_label_max_chars")
	} else if textcheck.DisplayText(label) != nil {
		f.add("/label", "invalid", "the label contains characters that are not allowed")
	}
}

// checkCondition checks the condition of kind.
func checkCondition(f *wakeFindings, kind WakeKind, c WakeCondition, maxBytes int) {
	switch kind {
	case WakePRChecks:
		if !repositoryPattern.MatchString(c.Repository) {
			f.add("/condition/repository", "invalid", "the repository is not owner/name")
		}
		if c.Number < 1 {
			f.add("/condition/number", "invalid", "the pull request number is below 1")
		}
		if c.HeadCommit != "" && !commitPattern.MatchString(c.HeadCommit) {
			f.add("/condition/head_commit", "invalid", "the head commit is not a hexadecimal commit id")
		}
		if c.CommandLine != "" {
			f.add("/condition/command_line", "not_allowed", "a pr_checks wake-up has no command")
		}
	case WakeCommand:
		switch {
		case strings.TrimSpace(c.CommandLine) == "":
			f.add("/condition/command_line", "required", "a command wake-up needs a command line")
		case len(c.CommandLine) > maxBytes:
			f.add("/condition/command_line", "too_long", "the command line is longer than message.max_content_bytes")
		case MultiLineText(c.CommandLine) != nil:
			f.add("/condition/command_line", "invalid", "the command line contains characters that are not allowed")
		}
		if c.Repository != "" || c.Number != 0 || c.HeadCommit != "" {
			f.add("/condition", "not_allowed", "a command wake-up has no pull request")
		}
	case WakeAt:
		if c != (WakeCondition{}) {
			f.add("/condition", "not_allowed", "an at wake-up has no condition")
		}
	}
}

// MultiLineText checks a text that may span lines (a command line, a result text): the rules of
// textcheck.DisplayText, except that line feed and tab are allowed.
func MultiLineText(s string) error {
	return textcheck.DisplayText(strings.NewReplacer("\n", " ", "\t", " ").Replace(s))
}

// CheckResultText checks the result text of run.wake.fire: at most maxBytes bytes (the runner cut
// it to runner.wake_result_max_bytes already) and a multi-line text. Errors: validation_failed on
// /result_text.
func CheckResultText(s string, maxBytes int) error {
	f := &wakeFindings{}
	if len(s) > maxBytes {
		f.add("/result_text", "too_long", "the result text is longer than message.max_content_bytes")
	} else if MultiLineText(s) != nil {
		f.add("/result_text", "invalid", "the result text contains characters that are not allowed")
	}
	return f.err()
}

// WakeWindow returns not_before and deadline of a new wake-up created at now (09 5.8.1, 5.8.6):
// not_before is the requested time but never earlier than now plus lim.MinDelay (also a time in
// the past, --after 0 and a command that succeeds at once); the deadline is the requested one, at
// most lim.MaxWait after not_before for usage_limit and after now for every other reason, and the
// longest one when none is requested. A requested deadline before not_before is a field error.
//
// Example:
//
//	nb, dl, err := domain.WakeWindow(now, time.Time{}, time.Time{}, domain.WakeReasonAgent, lim) // now+min, now+max
func WakeWindow(now, notBefore, deadline time.Time, reason WakeReason, lim WakeLimits) (time.Time, time.Time, error) {
	earliest := now.Add(lim.MinDelay)
	if notBefore.Before(earliest) {
		notBefore = earliest
	}
	base := now
	if reason == WakeReasonUsageLimit {
		base = notBefore
	}
	latest := base.Add(lim.MaxWait)
	if latest.Before(notBefore) {
		latest = notBefore
	}
	switch {
	case deadline.IsZero(), deadline.After(latest):
		deadline = latest
	case deadline.Before(notBefore):
		return time.Time{}, time.Time{}, fieldErr("/deadline", "invalid", "the deadline lies before the earliest fire time")
	}
	return notBefore, deadline, nil
}

// EncodeCondition returns the stored run_wake_condition.v1 of c ("{}" for at).
func EncodeCondition(c WakeCondition) []byte {
	b, err := json.Marshal(c)
	if err != nil {
		// A struct of strings and an int always encodes.
		return []byte("{}")
	}
	return b
}

// DecodeCondition reads a stored run_wake_condition.v1; an unreadable value reads as empty.
func DecodeCondition(raw []byte) WakeCondition {
	var c WakeCondition
	_ = json.Unmarshal(raw, &c)
	return c
}

// FireOutcome is the outcome a runner reports with run.wake.fire.
type FireOutcome string

// The outcomes of run.wake.fire: fired with a result, failed after runner.wake_check_failures
// failures in a row, and input_skipped for a wake input the runner did not write (09 5.8.3; the
// name is a ruling of AR-04, D-AR-04-3).
const (
	FireFired        FireOutcome = "fired"
	FireFailed       FireOutcome = "failed"
	FireInputSkipped FireOutcome = "input_skipped"
)

// Valid reports whether o is one of the three outcomes.
func (o FireOutcome) Valid() bool {
	return o == FireFired || o == FireFailed || o == FireInputSkipped
}

// Autonomy is one row of run_autonomy (03 4.8): the chain and idle counts of one person in one
// thread and the reason of an ended chain ("" while it runs).
type Autonomy struct {
	Chain, Idle int
	EndedReason string
}

// AutonomyLimits are run.autonomous_chain_max (0 means no chain cap) and run.autonomous_idle_max
// (at least 1).
type AutonomyLimits struct {
	ChainMax, IdleMax int
}

// AutonomyRefusal returns the reason a further autonomous input of a, "" when it is allowed (09
// 5.8.6): an ended chain keeps its reason, the chain cap refuses at the cap (unless it is 0), and
// the idle cap refuses at the cap whatever the chain cap is.
//
// Example:
//
//	domain.AutonomyRefusal(domain.Autonomy{Chain: 25}, domain.AutonomyLimits{ChainMax: 25, IdleMax: 3}) // autonomous_chain_max
func AutonomyRefusal(a Autonomy, lim AutonomyLimits) string {
	switch {
	case a.EndedReason != "":
		return a.EndedReason
	case lim.ChainMax > 0 && a.Chain >= lim.ChainMax:
		return WakeCodeChainMax
	case a.Idle >= max(lim.IdleMax, 1):
		return WakeCodeIdleMax
	}
	return ""
}

// AfterInput returns a after one more autonomous input (a wake run or a wake input).
func AfterInput(a Autonomy) Autonomy {
	a.Chain++
	return a
}

// AfterIdleReport returns a after the idle flag of one input or notification turn (09 5.8.6): an
// idle one raises the idle count, one that did work resets it; reached reports that this report
// made the idle count reach the cap, which ends the chain with autonomous_idle_max.
//
// Example:
//
//	a, reached := domain.AfterIdleReport(domain.Autonomy{Idle: 2}, true, lim) // Idle 3, reached with IdleMax 3
func AfterIdleReport(a Autonomy, idle bool, lim AutonomyLimits) (Autonomy, bool) {
	if !idle {
		a.Idle = 0
		return a, false
	}
	a.Idle++
	if a.EndedReason == "" && a.Idle >= max(lim.IdleMax, 1) {
		a.EndedReason = WakeCodeIdleMax
		return a, true
	}
	return a, false
}

// AutonomyLast is the member /metrics/autonomy_last of a run (03 runs.metrics): the id of the last
// autonomous input or notification turn and whether it was idle.
type AutonomyLast struct {
	Input string `json:"input"`
	Idle  bool   `json:"idle"`
}

// AutonomyLastOf reads /autonomy_last of a stored run_metrics.v1; ok is false when it is absent or
// has no input.
func AutonomyLastOf(metrics []byte) (AutonomyLast, bool) {
	var m struct {
		Last *AutonomyLast `json:"autonomy_last"`
	}
	if err := json.Unmarshal(metrics, &m); err != nil || m.Last == nil || m.Last.Input == "" {
		return AutonomyLast{}, false
	}
	return *m.Last, true
}

// WakeNoteText returns the text of the system note of a wake-up end other than fired (09 5.8.4):
// it names the label and the reason. The text is English; clients localize system lines by
// their code.
func WakeNoteText(label string, state WakeState, reason string) string {
	name := "Wake-up"
	if label != "" {
		name = "Wake-up '" + label + "'"
	}
	switch state {
	case WakeExpired:
		return name + " expired: its deadline passed before it could fire"
	case WakeCancelled:
		return name + " cancelled (" + reason + ")"
	case WakeFailed:
		return name + " failed: its check could not run (" + reason + ")"
	case WakeRefused:
		switch reason {
		case WakeCodeChainMax, WakeCodeIdleMax:
			return name + " refused (" + reason + "): a reply of the person continues the work"
		case WakeCodeAutonomousNotAllowed:
			return name + " refused: agents may not run on alone in this channel"
		}
		return name + " refused (" + reason + ")"
	case WakePending, WakeFired:
	}
	return name + " ended (" + reason + ")"
}

// AutonomyNoteText returns the note of a chain that reached the idle cap (09 5.8.6).
func AutonomyNoteText(reason string) string {
	return "Autonomous work stopped (" + reason + "): a reply continues it"
}

// wakeFindings collects field errors of the wake-up operations.
type wakeFindings struct{ list []errs.FieldError }

// add records one finding.
func (f *wakeFindings) add(pointer, code, detail string) {
	f.list = append(f.list, errs.FieldError{Pointer: pointer, Code: code, Detail: detail})
}

// err returns nil or one validation_failed with every finding.
func (f *wakeFindings) err() error {
	if len(f.list) == 0 {
		return nil
	}
	return errs.New(errs.CodeValidationFailed, f.list[0].Detail).WithFields(slices.Clone(f.list)...)
}

// fieldErr returns one validation_failed with one field error.
func fieldErr(pointer, code, detail string) error {
	f := &wakeFindings{}
	f.add(pointer, code, detail)
	return f.err()
}
