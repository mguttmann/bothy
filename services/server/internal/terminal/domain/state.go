// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"errors"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
)

// State is the state of a host (hosts.state, 24 section 3.1).
type State string

// The five states of 24 section 3.1; only active allows sessions.
const (
	StatePairing    State = "pairing"
	StateConfirming State = "confirming"
	StateActive     State = "active"
	StateDisabled   State = "disabled"
	StateRetired    State = "retired"
)

// The values of hosts.disabled_reason.
const (
	// DisabledOwner is a host an owner disabled; it can be enabled again.
	DisabledOwner = "owner"
	// DisabledDuplicateConnection is a host the server disabled at a second live control
	// connection (24 section 3.4); only re-pairing helps.
	DisabledDuplicateConnection = "duplicate_connection"
)

// Cause is what drives a transition.
type Cause string

// The causes of the table of 24 section 3.1 (brief M2-39, Design note 2).
const (
	CauseCreate                       Cause = "create"
	CausePair                         Cause = "pair"
	CauseNewPairingCode               Cause = "new_pairing_code"
	CauseConfirm                      Cause = "confirm"
	CauseFingerprintAttemptsExhausted Cause = "fingerprint_attempts_exhausted"
	CauseDisable                      Cause = "disable"
	CauseDuplicateConnection          Cause = "duplicate_connection"
	CauseEnable                       Cause = "enable"
	CauseRetire                       Cause = "retire"
)

// Causes returns every cause, in the order of the table.
func Causes() []Cause {
	return []Cause{CauseCreate, CausePair, CauseNewPairingCode, CauseConfirm, CauseFingerprintAttemptsExhausted,
		CauseDisable, CauseDuplicateConnection, CauseEnable, CauseRetire}
}

// Result is the outcome of a transition: the new state, its disabled reason (only with
// disabled), and whether and why the open sessions of the host end in the same transaction.
type Result struct {
	To             State
	DisabledReason string
	EndSessions    bool
	CloseReason    CloseReason
}

// ErrInvalidTransition is a transition the life cycle does not draw; it is a kernel/errs error
// of code invalid_state_transition (409).
var ErrInvalidTransition = errs.New(errs.CodeInvalidStateTransition, "The host cannot change into this state.")

// step is one allowed transition.
type step struct {
	from   State
	reason string // disabled reason of the state before; "" except for disabled
	cause  Cause
	to     State
	newRsn string
}

// steps is the life cycle of 24 section 3.1 with the fall back of 24 section 3.2; it is the only
// place that knows the transitions (M2-41 and M2-43 call Transition for theirs).
var steps = []step{
	{"", "", CauseCreate, StatePairing, ""},
	{StatePairing, "", CausePair, StateConfirming, ""},
	{StatePairing, "", CauseNewPairingCode, StatePairing, ""},
	{StateConfirming, "", CauseConfirm, StateActive, ""},
	{StateConfirming, "", CauseNewPairingCode, StatePairing, ""},
	{StateConfirming, "", CauseFingerprintAttemptsExhausted, StatePairing, ""},
	{StateActive, "", CauseNewPairingCode, StatePairing, ""},
	{StateActive, "", CauseDisable, StateDisabled, DisabledOwner},
	{StateActive, "", CauseDuplicateConnection, StateDisabled, DisabledDuplicateConnection},
	{StateActive, "", CauseRetire, StateRetired, ""},
	// A host disabled for a duplicate connection cannot be enabled, only paired again (24 3.4).
	{StateDisabled, DisabledOwner, CauseEnable, StateActive, ""},
	{StateDisabled, DisabledOwner, CauseNewPairingCode, StatePairing, ""},
	{StateDisabled, DisabledDuplicateConnection, CauseNewPairingCode, StatePairing, ""},
	{StateDisabled, DisabledOwner, CauseRetire, StateRetired, ""},
	{StateDisabled, DisabledDuplicateConnection, CauseRetire, StateRetired, ""},
}

// closeOnEntry is the close reason of the sessions a transition into a state ends (24 section
// 10.3). Ruling of M2-39: every transition into a state other than active ends the open sessions
// of the host; into pairing and confirming that is a re-pairing (host_rekeyed).
var closeOnEntry = map[State]CloseReason{
	StatePairing: CloseHostRekeyed, StateConfirming: CloseHostRekeyed, StateDisabled: CloseHostDisabled, StateRetired: CloseHostRetired,
}

// Transition returns the result of cause c on a host in state from (the empty state for a new
// host) with disabled reason disabledReason (only with disabled). Every pair the life cycle does
// not draw is ErrInvalidTransition, among them every cause from retired, enable after a
// duplicate connection, disable and retire from pairing and confirming, and every cause that
// would keep the state except new_pairing_code on pairing. A request for the state a host already
// has is not a transition; the use case answers it without effect. Every transition raises
// permission_version (03 section 4.3), so callers run it in a rights change.
func Transition(from State, disabledReason string, c Cause) (Result, error) {
	if from != StateDisabled {
		disabledReason = ""
	}
	for _, s := range steps {
		if s.from == from && s.reason == disabledReason && s.cause == c {
			r := Result{To: s.to, DisabledReason: s.newRsn}
			if from != "" && s.to != StateActive {
				r.EndSessions, r.CloseReason = true, closeOnEntry[s.to]
			}
			return r, nil
		}
	}
	return Result{}, ErrInvalidTransition
}

// IsInvalidTransition reports whether err is ErrInvalidTransition.
func IsInvalidTransition(err error) bool { return errors.Is(err, ErrInvalidTransition) }
