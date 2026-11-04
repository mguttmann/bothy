// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// The wait reasons of a queued run in runs.state_reason (09 5.2.1, F-WARTESCHLANGE-GRUND).
const (
	// WaitInvocationHeld: the server holds agent.invocation until an attachment or the server
	// transcript of the triggering message is final (runs.invocation_held_until).
	WaitInvocationHeld = "invocation_held"
	// WaitSlotsFull: every slot of the runner (runner.max_parallel_runs) is in use.
	WaitSlotsFull = "slots_full"
	// WaitThreadBusy: the runner waits for the thread or session lock of another turn.
	WaitThreadBusy = "thread_busy"
)

// runnerReasons are the wait reasons an adapter reports in the heartbeat member waiting_runs.
var runnerReasons = map[string]bool{WaitSlotsFull: true, WaitThreadBusy: true}

// HeldReason is the wait reason of a new run: invocation_held when heldUntil is set (the
// acceptance holds its invocation), "" otherwise.
func HeldReason(heldUntil time.Time) string {
	if heldUntil.IsZero() {
		return ""
	}
	return WaitInvocationHeld
}

// QueuedReason is a queued run with its stored state_reason ("" for none).
type QueuedReason struct {
	RunID  ids.ID
	Reason string
}

// WaitChanges returns the wait reasons to store for the queued runs of one agent after its
// heartbeat reported reported (run id to wait reason; 09 4.8 and 5.2.1): a listed run gets the
// reported reason unless it is held (invocation_held stays until the release), a run that is not
// listed loses a runner reason (slots_full, thread_busy), and nothing else changes. A reported
// code that is not a runner reason is ignored (the decoder refuses it before). The result keeps
// the order of queued and holds only real changes; Reason "" clears the column.
func WaitChanges(queued []QueuedReason, reported map[ids.ID]string) []QueuedReason {
	var out []QueuedReason
	for _, q := range queued {
		want, listed := reported[q.RunID]
		switch {
		case q.Reason == WaitInvocationHeld:
			continue
		case listed && runnerReasons[want]:
		case runnerReasons[q.Reason]:
			want = ""
		default:
			continue
		}
		if want != q.Reason {
			out = append(out, QueuedReason{RunID: q.RunID, Reason: want})
		}
	}
	return out
}
