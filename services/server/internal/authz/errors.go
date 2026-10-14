// SPDX-License-Identifier: Apache-2.0

package authz

import (
	"bytes"
	"slices"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// The hints of the invariant violations (06 section 5 "Errors", 07 section 4.4 step 5).
const (
	// HintI1 names the way out of i1_violation.
	HintI1 = "Switch the agent to organization first."
	// HintLastOwner names the way out of last_owner.
	HintLastOwner = "Appoint another owner first."
	// HintI6 names the way out of i6_violation (M2-40, 24 section 7.3).
	HintI6 = "Revoke the other grants on this host or declare organization first."
	// HintI1ClaimSync is the hint of an OIDC login refused because its group claim would violate
	// I1 (brief M1-42 "I1 violation").
	HintI1ClaimSync = "Sign-in blocked by a group mapping. An owner must switch the affected agent to organization first."
)

// I1Conflict is one personal agent whose trigger set is not exactly its credential holder
// (07 SR1, 10 section 5.4). IDs only, never names.
type I1Conflict struct {
	// AgentID is the personal agent.
	AgentID ids.ID
	// CredentialHolderID is its credential holder; zero when the agent has none.
	CredentialHolderID ids.ID
	// PrincipalIDs are the principals with effective trigger next to the holder, ascending.
	PrincipalIDs []ids.ID
}

// I6Conflict is one host with a personal LLM login on which a valid grant names another subject
// than its credential holder (03 section 6.8, 24 section 7.3; M2-40). IDs only, never names.
type I6Conflict struct {
	// HostID is the host; CredentialHolderID its credential holder.
	HostID, CredentialHolderID ids.ID
	// PrincipalIDs are the user subjects other than the holder and the members in effect of the
	// group grants on the host, ascending; empty for a group without members.
	PrincipalIDs []ids.ID
}

// InvariantError is the rejection of a rights change by VerifyInvariants (07 section 4.4): it
// unwraps to the kernel/errs error of kind invariant_violation with code i1_violation,
// i6_violation or last_owner and the hint, and it carries the I1 or I6 conflicts, which the problem writer adds as the
// member conflicts (errs.ProblemExtender, 10 section 5.4).
type InvariantError struct {
	// Code is errs.CodeI1Violation, errs.CodeI6Violation or errs.CodeLastOwner.
	Code errs.Code
	// Conflicts are the I1 conflicts sorted by AgentID; empty for last_owner.
	Conflicts []I1Conflict
	// HostConflicts are the I6 conflicts sorted by HostID, set with code i6_violation (M2-40).
	HostConflicts []I6Conflict
	err           *errs.Error
}

// Compile-time check: the problem body carries the conflicts.
var _ errs.ProblemExtender = (*InvariantError)(nil)

// NewI1Violation returns the error of an I1 violation with conflicts, sorted by agent and with
// sorted principal ids (the input is not changed).
//
// Example:
//
//	return authz.NewI1Violation([]authz.I1Conflict{{AgentID: a, CredentialHolderID: h, PrincipalIDs: extra}})
func NewI1Violation(conflicts []I1Conflict) *InvariantError {
	cs := make([]I1Conflict, len(conflicts))
	for i, c := range conflicts {
		c.PrincipalIDs = slices.SortedFunc(slices.Values(c.PrincipalIDs), compareIDs)
		cs[i] = c
	}
	slices.SortFunc(cs, func(a, b I1Conflict) int { return compareIDs(a.AgentID, b.AgentID) })
	return &InvariantError{
		Code: errs.CodeI1Violation, Conflicts: cs,
		err: errs.New(errs.CodeI1Violation, "The change would let someone other than the credential holder trigger a personal agent.").
			WithReason(string(errs.CodeI1Violation)).WithHint(HintI1),
	}
}

// NewI6Violation returns the error of an I6 violation with conflicts, sorted by host and with
// sorted principal ids (the input is not changed); the hint is HintI6 (M2-40).
//
// Example:
//
//	return authz.NewI6Violation([]authz.I6Conflict{{HostID: h, CredentialHolderID: holder, PrincipalIDs: others}})
func NewI6Violation(conflicts []I6Conflict) *InvariantError {
	cs := make([]I6Conflict, len(conflicts))
	for i, c := range conflicts {
		c.PrincipalIDs = slices.SortedFunc(slices.Values(c.PrincipalIDs), compareIDs)
		cs[i] = c
	}
	slices.SortFunc(cs, func(a, b I6Conflict) int { return compareIDs(a.HostID, b.HostID) })
	return &InvariantError{
		Code: errs.CodeI6Violation, HostConflicts: cs,
		err: errs.New(errs.CodeI6Violation, "The change would let someone other than the holder of a personal LLM login use the host.").
			WithReason(string(errs.CodeI6Violation)).WithHint(HintI6),
	}
}

// WithHint returns a copy of e with hint instead of its hint; the code and the conflicts stay.
func (e *InvariantError) WithHint(hint string) *InvariantError {
	inner := *e.err
	inner.Hint = hint
	out := *e
	out.err = &inner
	return &out
}

// NewLastOwnerViolation returns the error of a change that leaves no active owner (07 SR8).
func NewLastOwnerViolation() *InvariantError {
	return &InvariantError{
		Code: errs.CodeLastOwner,
		err: errs.New(errs.CodeLastOwner, "At least one active owner must remain.").
			WithReason(string(errs.CodeLastOwner)).WithHint(HintLastOwner),
	}
}

// Error returns the text of the underlying errs error.
func (e *InvariantError) Error() string { return e.err.Error() }

// Unwrap returns the errs error, so errs.As and errs.KindOf see code and kind.
func (e *InvariantError) Unwrap() error { return e.err }

// conflictJSON is one entry of the problem member conflicts (OpenAPI InvariantViolation).
type conflictJSON struct {
	Code               string   `json:"code"`
	AgentID            string   `json:"agent_id"`
	CredentialHolderID string   `json:"credential_holder_id,omitempty"`
	PrincipalIDs       []string `json:"principal_ids"`
}

// hostConflictJSON is one entry of conflicts of i6_violation (OpenAPI InvariantViolation with
// host_id and hint).
type hostConflictJSON struct {
	Code               string   `json:"code"`
	HostID             string   `json:"host_id"`
	CredentialHolderID string   `json:"credential_holder_id,omitempty"`
	PrincipalIDs       []string `json:"principal_ids"`
	Hint               string   `json:"hint"`
}

// ProblemExtensions implements errs.ProblemExtender: the member conflicts for i1_violation (one
// entry per agent) and i6_violation (one entry per host), none for last_owner. principal_ids is
// always an array, credential_holder_id absent for none.
func (e *InvariantError) ProblemExtensions() map[string]any {
	if e.Code == errs.CodeI6Violation {
		out := make([]hostConflictJSON, 0, len(e.HostConflicts))
		for _, c := range e.HostConflicts {
			j := hostConflictJSON{Code: string(errs.CodeI6Violation), HostID: c.HostID.String(), PrincipalIDs: []string{}, Hint: HintI6}
			if !c.CredentialHolderID.IsZero() {
				j.CredentialHolderID = c.CredentialHolderID.String()
			}
			for _, p := range c.PrincipalIDs {
				j.PrincipalIDs = append(j.PrincipalIDs, p.String())
			}
			out = append(out, j)
		}
		return map[string]any{"conflicts": out}
	}
	if e.Code != errs.CodeI1Violation {
		return nil
	}
	out := make([]conflictJSON, 0, len(e.Conflicts))
	for _, c := range e.Conflicts {
		j := conflictJSON{Code: string(errs.CodeI1Violation), AgentID: c.AgentID.String(), PrincipalIDs: []string{}}
		if !c.CredentialHolderID.IsZero() {
			j.CredentialHolderID = c.CredentialHolderID.String()
		}
		for _, p := range c.PrincipalIDs {
			j.PrincipalIDs = append(j.PrincipalIDs, p.String())
		}
		out = append(out, j)
	}
	return map[string]any{"conflicts": out}
}

// compareIDs orders ids by their bytes, the order of their text.
func compareIDs(a, b ids.ID) int { return bytes.Compare(a[:], b[:]) }
