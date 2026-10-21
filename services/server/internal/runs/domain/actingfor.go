// SPDX-License-Identifier: Apache-2.0

package domain

import "github.com/OWNER/bothy/services/server/internal/kernel/ids"

// ActingForStatus is the outcome of the resolution of the Invocation-Token header (guard rule
// SR9 of 07, brief M2-14 Design notes 2 and 3). It carries the values of kernel/actor's
// ActingForStatus but is declared here, because the domain imports only kernel-core;
// adapters/actingfor maps it.
type ActingForStatus string

// The statuses of the resolution.
const (
	// ActingForBound: the token belongs to an active run of the agent; SR9 binds the request.
	ActingForBound ActingForStatus = "bound"
	// ActingForRunInactive: the token belongs to a run of the agent that has ended.
	ActingForRunInactive ActingForStatus = "run_inactive"
	// ActingForInvalid: the token names no run of the agent (or failed verification).
	ActingForInvalid ActingForStatus = "invalid"
)

// RunBinding is what the resolution reads of a run (query GetRunBinding): the run, its agent,
// the person who triggered it, its current thread and its state as stored.
type RunBinding struct {
	ID, AgentID, TriggeredBy, ThreadID ids.ID
	// State is runs.state as stored; a value outside the six states is treated as invalid.
	State string
}

// ActingForResult is the outcome of the resolution: with ActingForBound and ActingForRunInactive
// UserID is runs.triggered_by, RunID the run, ThreadID runs.thread_id and JTI the jti claim of
// the token; with ActingForInvalid every other field is zero.
type ActingForResult struct {
	Status                  ActingForStatus
	UserID, RunID, ThreadID ids.ID
	JTI                     string
}

// ActingForVerdict decides the binding of verified claims c to run (Design note 3 b to d of
// M2-14). found reports whether the run of the claim run_id exists; agentID is the agent of the
// request. Claims without run_id, an unknown run, a run of another agent, a sub other than
// runs.triggered_by and a thread_id other than runs.thread_id give ActingForInvalid, because SR9
// binds a run and "run active" cannot hold without one. An active state (queued, running,
// waiting_input) gives ActingForBound, an end state ActingForRunInactive; any other stored state
// gives ActingForInvalid (fail closed). Expiry and the currency of the jti are not checked here
// (Design note 3e): the token never adds a right.
//
// Example:
//
//	res := domain.ActingForVerdict(claims, binding, found, agentID)
//	if res.Status == domain.ActingForBound { /* intersect with res.UserID */ }
func ActingForVerdict(c InvocationClaims, run RunBinding, found bool, agentID ids.ID) ActingForResult {
	invalid := ActingForResult{Status: ActingForInvalid}
	runID, err := ids.Parse(c.RunID)
	if err != nil || !found || agentID.IsZero() || run.ID != runID || run.AgentID != agentID {
		return invalid
	}
	sub, err := ids.Parse(c.Sub)
	if err != nil || run.TriggeredBy != sub {
		return invalid
	}
	thread, err := ids.Parse(c.ThreadID)
	if err != nil || run.ThreadID != thread {
		return invalid
	}
	var status ActingForStatus
	switch s := State(run.State); {
	case s.Active():
		status = ActingForBound
	case s.Ended():
		status = ActingForRunInactive
	default:
		return invalid
	}
	return ActingForResult{Status: status, UserID: run.TriggeredBy, RunID: run.ID, ThreadID: run.ThreadID, JTI: c.Jti}
}
