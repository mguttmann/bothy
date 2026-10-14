// SPDX-License-Identifier: Apache-2.0

// Package domain holds the pure rules of the terminal module (M2-39): the life cycle of a host as
// one function (Transition, docs/spec/24-terminal.md section 3.1), the closed lists of the end of
// a session (24 section 10.3), the names of hosts and session users (03 section 4.14, SEC-IN-4),
// the plan of session user assignments of ruling T12 (24 section 2.2) and the Go mirrors of the
// reported jsonb columns. It knows no clock, database or other module.
//
// Validation errors are kernel/errs errors of code validation_failed with one field error whose
// pointer names the member of the request body; they never repeat the rejected value.
//
// Example:
//
//	r, err := domain.Transition(domain.StateActive, "", domain.CauseDisable)
//	// r.To == StateDisabled, r.DisabledReason == "owner", r.EndSessions, r.CloseReason == host_disabled
package domain
