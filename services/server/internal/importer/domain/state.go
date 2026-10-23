// SPDX-License-Identifier: Apache-2.0

package domain

// Op is an owner operation on the state of a source (import.manage).
type Op string

// The operations of the state machine (M8-03 design note 3).
const (
	OpStart    Op = "start"
	OpPause    Op = "pause"
	OpCancel   Op = "cancel"
	OpComplete Op = "complete"
)

// States of a source (03 import_sources.state); the same values as the importer port.
const (
	StateDraft     = "draft"
	StateRunning   = "running"
	StatePaused    = "paused"
	StateCompleted = "completed"
	StateFailed    = "failed"
)

// Step is the outcome of an operation on a state: Next is the state after it; NoEffect reports a
// repeated transition that changes nothing (06 section 2 "No effect"); Allowed is false for a
// transition the machine refuses (409 conflict, reason import_state).
type Step struct {
	Next     string
	NoEffect bool
	Allowed  bool
}

// transitions are the allowed changes: draft or paused to running by start, running to paused
// by pause, draft, running or paused to failed by cancel (the enum has no cancelled), running or
// paused to completed by complete.
var transitions = map[Op]map[string]string{
	OpStart:    {StateDraft: StateRunning, StatePaused: StateRunning},
	OpPause:    {StateRunning: StatePaused},
	OpCancel:   {StateDraft: StateFailed, StateRunning: StateFailed, StatePaused: StateFailed},
	OpComplete: {StateRunning: StateCompleted, StatePaused: StateCompleted},
}

// repeats are the operations whose repetition answers without effect: start on running, pause on
// paused.
var repeats = map[Op]string{OpStart: StateRunning, OpPause: StatePaused}

// Transition returns the step of op on state. It does not judge the completion condition (every
// imported channel has lead `bothy`); the use case checks it after an allowed complete.
//
// Example:
//
//	Transition(StateDraft, OpStart) // Step{Next: "running", Allowed: true}
func Transition(state string, op Op) Step {
	if next, ok := transitions[op][state]; ok {
		return Step{Next: next, Allowed: true}
	}
	if r, ok := repeats[op]; ok && r == state {
		return Step{Next: state, NoEffect: true, Allowed: true}
	}
	return Step{Next: state}
}

// SetsStartedAt reports a transition that sets started_at when it is still unset: the first
// start.
func SetsStartedAt(op Op) bool { return op == OpStart }

// SetsFinishedAt reports a transition that ends the source and sets finished_at.
func SetsFinishedAt(op Op) bool { return op == OpCancel || op == OpComplete }

// Ended reports a state from which no import change is possible: completed or failed.
func Ended(state string) bool { return state == StateCompleted || state == StateFailed }
