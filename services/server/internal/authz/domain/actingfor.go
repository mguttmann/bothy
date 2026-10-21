// SPDX-License-Identifier: Apache-2.0

package domain

import "github.com/OWNER/bothy/services/server/internal/kernel/ids"

// ActingForMode is how the acting-for binding of guard rule SR9 (07 section "SR9", brief M2-14
// design note 5) treats an action of an agent that acts inside a run.
type ActingForMode int

// The modes of the mode table. The zero value is ActingForBound, so an action nobody classified
// is bound (fail closed).
const (
	// ActingForBound: the agent's rights are intersected with the triggering person's (05
	// section 2.3 point 6). Every catalogue action that is neither an exception nor unbound,
	// including the actions without a path in the channel tree, whose requirement needs no level
	// on a path, so the intersection changes nothing for them.
	ActingForBound ActingForMode = iota
	// ActingForOwnException: only the agent's own rights apply, but only on the own thread (and,
	// for a resource of kind run, the own run); elsewhere the action is bound. 07 "SR9" names
	// replies in the run's thread, patches of the own run, run.finish, interaction.respond and
	// agent_session.report.
	ActingForOwnException
	// ActingForUnbound: the action neither reads content nor writes into a channel; it carries
	// its own invocation token in the body (09 sections 4.5 and 4.5.1), so the header changes
	// nothing for it, not even an ended run.
	ActingForUnbound
)

// actingForModes holds the two non-default sets of the mode table (golden file
// testdata/actingfor_modes.golden).
//
//nolint:goconst // action ids of the generated catalogue, listed once here
var actingForModes = map[string]ActingForMode{
	"message.create":       ActingForOwnException,
	"run.update":           ActingForOwnException,
	"run.finish":           ActingForOwnException,
	"interaction.respond":  ActingForOwnException,
	"agent_session.report": ActingForOwnException,
	"run.start":            ActingForUnbound,
	"run.token.renew":      ActingForUnbound,
}

// ActingForModeOf returns the mode of the action id; every action outside the two sets, also an
// unknown one, is ActingForBound.
//
// Example:
//
//	domain.ActingForModeOf("run.finish") == domain.ActingForOwnException
func ActingForModeOf(action string) ActingForMode { return actingForModes[action] }

// ActingForFacts are the facts of a bound acting-for binding that the exception rule reads: the
// run and its thread (runs.thread_id at authentication).
type ActingForFacts struct {
	RunID, ThreadID ids.ID
}

// kindRun is the resource kind of a run.
const kindRun = "run"

// ActingForApplies reports whether the intersection with the triggering person applies to one
// decision of an action with mode on the resolved path and resource r (brief M2-14 design note
// 4d): never in delivery mode (07 section 8.1, agent consumers are judged by their own grants),
// never for an unbound action, and for an own exception only when the resource is not on the own
// thread, or is a run other than the own run. "Own thread" needs a resolved thread equal to
// af.ThreadID; a zero thread is never the own one.
//
// Example:
//
//	ActingForApplies(ActingForOwnException, af, path, r, false) // false on the run's own thread
func ActingForApplies(mode ActingForMode, af ActingForFacts, path Path, r ResourceFacts, delivery bool) bool {
	switch {
	case delivery, mode == ActingForUnbound:
		return false
	case mode != ActingForOwnException:
		return true
	}
	ownThread := !path.ThreadID.IsZero() && path.ThreadID == af.ThreadID
	if r.Kind == kindRun {
		return !ownThread || r.ID != af.RunID
	}
	return !ownThread
}

// The statuses of an acting-for binding (kernel/authorize.ActingForBound and its siblings).
const (
	actingForBound       = "bound"
	actingForRunInactive = "run_inactive"
	actingForInvalid     = "invalid"
)

// CheckActingFor is the status check of 07 section 4.2 step 1 for a request that carried the
// Invocation-Token header (brief M2-14 design notes 4b and 4c), after the liveness and pending
// checks and before any resource is resolved, so an unknown and an invisible resource answer
// alike: no header ("") and bound pass; invalid, and any status not known here, are forbidden
// invocation_token_invalid with Detail invalid for every action; run_inactive is the same with
// Detail run_inactive, except for an unbound action, which answers as without the header.
//
// Example:
//
//	d := CheckActingFor("run_inactive", ActingForModeOf("message.read")) // forbidden, Detail run_inactive
func CheckActingFor(status string, mode ActingForMode) Decision {
	switch status {
	case "", actingForBound:
		return Allow
	case actingForRunInactive:
		if mode == ActingForUnbound {
			return Allow
		}
		return Decision{Outcome: OutcomeForbidden, Reason: ReasonInvocationTokenInvalid, Detail: actingForRunInactive}
	}
	return Decision{Outcome: OutcomeForbidden, Reason: ReasonInvocationTokenInvalid, Detail: actingForInvalid}
}
