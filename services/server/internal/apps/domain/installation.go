// SPDX-License-Identifier: Apache-2.0

package domain

import "github.com/OWNER/bothy/services/server/internal/kernel/errs"

// State is app_installations.state (03 section 4.4).
type State string

// The states of an installation.
const (
	// StateActive is an installation whose app may act (with grants and scopes).
	StateActive State = "active"
	// StateSuspended is the emergency stop: every grant stays, the tokens stop working
	// (11 section 4.3).
	StateSuspended State = "suspended"
	// StateUninstalled is the end: tokens and grants revoked, the principal disabled.
	StateUninstalled State = "uninstalled"
)

// Transition names an action that changes the state of an installation.
type Transition string

// The transitions of 05 section 4.11.
const (
	// TransitionSuspend is app.suspend.
	TransitionSuspend Transition = "suspend"
	// TransitionResume is app.resume.
	TransitionResume Transition = "resume"
	// TransitionUninstall is app.uninstall.
	TransitionUninstall Transition = "uninstall"
)

// Next returns the state after t from s and whether the state changes (M2-24 design note 9):
// suspend moves active to suspended, resume suspended to active, uninstall active or suspended to
// uninstalled. A transition to the state an installation already has is a call without effect
// (change false, nil error; 06 section 2 "Ohne Wirkung"). Suspend and resume of an uninstalled
// installation fail with 409 invalid_state_transition. An unknown state or transition is an
// internal error.
func Next(s State, t Transition) (State, bool, error) {
	switch t {
	case TransitionSuspend:
		return move(s, StateActive, StateSuspended)
	case TransitionResume:
		return move(s, StateSuspended, StateActive)
	case TransitionUninstall:
		switch s {
		case StateActive, StateSuspended:
			return StateUninstalled, true, nil
		case StateUninstalled:
			return s, false, nil
		}
	}
	return s, false, errs.New(errs.CodeInternalError, "apps: unknown installation state or transition")
}

// move is the transition from to into: the state into is no change, uninstalled is refused.
func move(s, from, into State) (State, bool, error) {
	if s == from {
		return into, true, nil
	}
	if s == into {
		return s, false, nil
	}
	if s == StateUninstalled {
		return s, false, errs.New(errs.CodeInvalidStateTransition, "The app is uninstalled.")
	}
	return s, false, errs.New(errs.CodeInternalError, "apps: unknown installation state")
}
