// SPDX-License-Identifier: Apache-2.0

package runs

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Wake is one wake-up as the operations of 09 5.8.2 return it (schema RunWake, 03 run_wakes): a
// server record that continues a thread later without a new message. Zero values are absent
// members.
type Wake struct {
	ID, AgentID, ThreadID, SourceRunID, TriggeredBy ids.ID
	// Kind is at, pr_checks or command; Reason agent, recovery or usage_limit; State pending,
	// fired, cancelled, expired, refused or failed; StateReason the reason of the last change.
	Kind, Reason, State, StateReason string
	// Label is the short text of the agent; "" for none.
	Label string
	// Condition is run_wake_condition.v1: Repository, Number and HeadCommit of pr_checks,
	// CommandLine of command.
	Condition WakeCondition
	// Outage is the outage number of a usage_limit wake-up of run.wake.create; 0 for none.
	Outage int
	// NotBefore and Deadline are the window; CreatedAt the creation; EndedAt zero while pending.
	NotBefore, Deadline, CreatedAt, EndedAt time.Time
	// ResultText is the result of a fired check; "" for none.
	ResultText string
	// FiredRunID is the wake run, FiredIntoRunID the run of a wake input; zero for none.
	FiredRunID, FiredIntoRunID ids.ID
	// Version is run_wakes.version, the ETag of the wake-up.
	Version int64
}

// WakeCondition is run_wake_condition.v1 (schema RunWakeCondition).
type WakeCondition struct {
	Repository  string
	Number      int
	HeadCommit  string
	CommandLine string
}

// WakePage is one page of wake-ups; NextCursor is "" on the last page.
type WakePage struct {
	Items      []Wake
	NextCursor string
}

// CreateWake is the input of Wakes.Create (createRunWake, run.wake.create; schema RunWakeCreate).
type CreateWake struct {
	// RunID is the active run of the calling agent that asks.
	RunID ids.ID
	// Kind is at, pr_checks or command; Reason agent or usage_limit.
	Kind, Reason string
	Label        string
	Condition    WakeCondition
	// NotBefore is the time of an at wake-up; zero for "as early as allowed".
	NotBefore time.Time
	// Deadline is the requested deadline; zero for the longest one (run.wake_max_wait).
	Deadline time.Time
	// Outage is the run's outage number of a usage_limit request; 0 for none.
	Outage int
	// Deferred marks a request the SDK outbox sent later: a refusal is stored in state refused
	// with the reason, the note and the notification (09 18.19.8).
	Deferred bool
}

// RecoverWake is the input of Wakes.Recover (recoverRunWake, run.wake.recover; schema
// RunWakeRecover).
type RecoverWake struct {
	// RunID is the ended source run (interrupted, stale or failed) of the calling agent.
	RunID ids.ID
	// Reason is recovery or usage_limit.
	Reason string
	// Label is the short text of the runner; "" for none.
	Label string
	// NotBefore is the earliest fire time (for usage_limit the reset plus run.wake_min_delay);
	// zero for "as early as allowed".
	NotBefore time.Time
}

// FireWake is the input of Wakes.Fire (fireRunWake, run.wake.fire; schema RunWakeFire).
type FireWake struct {
	WakeID ids.ID
	// Outcome is fired (the condition holds), failed (after runner.wake_check_failures failures in
	// a row) or input_skipped (the runner did not write a delivered wake input).
	Outcome string
	// ResultText is the result of a fired check; Reason the reason of failed.
	ResultText, Reason string
	// RunID is the run of a skipped wake input; zero otherwise.
	RunID ids.ID
}

// CancelWake is the input of Wakes.Cancel (cancelRunWake, run.wake.cancel).
type CancelWake struct {
	WakeID ids.ID
	// Reason is the code of a cancel by the agent (for example usage_cleared); "" for none.
	Reason string
}

// WakeCheck is the answer of an allowed run.wake.check or run.launch.check (schema
// RunWakeCheckResult): the configured tool class and billing mode the runner compares with its own
// (compare_actual).
type WakeCheck struct {
	ToolClass, BillingMode string
}

// ThreadWakesQuery is the query of Wakes.ListByThread (listThreadWakes, run.wake.read).
type ThreadWakesQuery struct {
	ThreadID ids.ID
	Cursor   string
	Limit    int
}

// WakeResult is the answer of Wakes.Create and Wakes.Recover: the wake-up and whether this call
// created it (201) or found the one of the same outage or run and reason (200).
type WakeResult struct {
	Wake    Wake
	Created bool
}

// Wakes is the incoming port of the wake-ups (09 5.8.2, stage AR). The caller is the principal of
// ctx. Errors are kernel/errs errors: an unknown or invisible run, wake-up or thread gives the same
// 404 not_found (02 section 9.2); a refusal is 409 conflict with the reason of 09 5.8 and a wrong
// field 400 validation_failed (06 R-API-30).
type Wakes interface {
	// Create creates a wake-up for the active run of the calling agent (run.wake.create).
	//
	// Example:
	//
	//	w, err := ws.Create(ctx, runs.CreateWake{RunID: id, Kind: "at", Reason: "agent", Label: "CI of #12"})
	Create(ctx context.Context, in CreateWake) (WakeResult, error)
	// Recover creates the recovery or usage-limit wake-up of an ended run of the calling agent,
	// at most one per run and reason (run.wake.recover); the repetition answers the first one.
	Recover(ctx context.Context, in RecoverWake) (WakeResult, error)
	// ListPending returns every pending wake-up of the calling agent (run.wake.list_pending).
	ListPending(ctx context.Context) ([]Wake, error)
	// Check allows the next execution of a pending pr_checks or command wake-up, or ends it
	// refused and answers 409 with the reason (run.wake.check).
	Check(ctx context.Context, wakeID ids.ID) (WakeCheck, error)
	// Fire fires a checked wake-up with its result, ends it failed, or returns a skipped wake
	// input to pending (run.wake.fire).
	Fire(ctx context.Context, in FireWake) (Wake, error)
	// Cancel ends a pending wake-up cancelled (run.wake.cancel): the agent, the wake-up's person
	// or a person with manage on the channel.
	Cancel(ctx context.Context, in CancelWake) (Wake, error)
	// ListByThread pages the wake-ups of a thread, newest first (run.wake.read).
	ListByThread(ctx context.Context, q ThreadWakesQuery) (WakePage, error)
	// LaunchCheck allows a launch of a run by a runner that did not accept it, or answers 409 with
	// the reason of the failed check (run.launch.check, 09 18.19.6).
	LaunchCheck(ctx context.Context, runID ids.ID) (WakeCheck, error)
}
