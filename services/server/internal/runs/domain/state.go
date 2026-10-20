// SPDX-License-Identifier: Apache-2.0

package domain

// State is a value of runs.state (03 runs, schema RunState).
type State string

// The states of a run (09 5.1).
const (
	// StateQueued: accepted, not started yet.
	StateQueued State = "queued"
	// StateRunning: the adapter started the run.
	StateRunning State = "running"
	// StateWaitingInput: a question or an approval is open.
	StateWaitingInput State = "waiting_input"
	// StateCompleted: the run ended successfully.
	StateCompleted State = "completed"
	// StateFailed: the run ended with an error; it carries a reason of 09 5.2.
	StateFailed State = "failed"
	// StateCancelled: the run was stopped; it carries a reason of 09 5.2.
	StateCancelled State = "cancelled"
)

// States returns the six states in the order of the CHECK of 03 runs.
func States() []State {
	return []State{StateQueued, StateRunning, StateWaitingInput, StateCompleted, StateFailed, StateCancelled}
}

// Valid reports whether s is one of the six states.
func (s State) Valid() bool {
	switch s {
	case StateQueued, StateRunning, StateWaitingInput, StateCompleted, StateFailed, StateCancelled:
		return true
	}
	return false
}

// Active reports queued, running and waiting_input: the states of the partial unique index of
// 03 runs (at most one active run per thread, R-DM-14).
func (s State) Active() bool {
	return s == StateQueued || s == StateRunning || s == StateWaitingInput
}

// Ended reports the end states completed, failed and cancelled; no transition leaves them.
func (s State) Ended() bool {
	return s == StateCompleted || s == StateFailed || s == StateCancelled
}

// transitions are the allowed pairs of 03 runs: everything else is rejected.
var transitions = map[State][]State{
	StateQueued:       {StateRunning, StateFailed, StateCancelled},
	StateRunning:      {StateWaitingInput, StateCompleted, StateFailed, StateCancelled},
	StateWaitingInput: {StateRunning, StateFailed, StateCancelled},
}

// CanTransition reports whether 03 runs allows the transition from -> to: true for exactly ten
// pairs, false for self loops, for every pair out of an end state and for unknown values.
//
// Example:
//
//	domain.CanTransition(domain.StateQueued, domain.StateRunning) // true
//	domain.CanTransition(domain.StateCompleted, domain.StateRunning) // false
func CanTransition(from, to State) bool {
	for _, t := range transitions[from] {
		if t == to {
			return true
		}
	}
	return false
}
