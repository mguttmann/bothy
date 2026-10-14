// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// The terminal rules of M2-40 (07 sections 2.12, 4.2 steps 8 and 9, SR10 to SR12; 08 section 6.3;
// 24 sections 6 to 8): the fresh step-up of terminal.open and terminal.attach, the quick reattach,
// the guards SR10, SR11 and SR12 and the effective right terminal with the I6 clause. Every
// function here is pure; the application layer gathers the facts.

// The step-up methods and the client component the fresh step-up names (03 auth_sessions).
const (
	// StepUpPasskey is a step-up confirmed with a passkey.
	StepUpPasskey = "passkey"
	// StepUpDeviceKey is a step-up confirmed with the step_up key of a native device.
	StepUpDeviceKey = "device_key"
	// ComponentConsole is the client component of the web console.
	ComponentConsole = "console"
)

// The terminal actions with a rule of their own (05 section 4.14).
const (
	ActionTerminalOpen   = "terminal.open"
	ActionTerminalAttach = "terminal.attach"
)

// LLMPersonal is the declaration of a personal LLM login on a host (hosts.llm_credentials).
const LLMPersonal = "personal"

// The session state that a quick reattach needs (terminal_sessions.state).
const sessionDetached = "detached"

// FreshFacts are the step-up facts of the session of the request (auth_sessions.step_up_at,
// step_up_consumed_at, step_up_method and client_component); zero times stand for NULL.
type FreshFacts struct {
	StepUpAt, StepUpConsumedAt    time.Time
	StepUpMethod, ClientComponent string
}

// FreshStepUp reports a step-up "frisch" (05 section 2.4, 08 section 6.3 points 1, 2, 4): set and
// not older than maxAge (terminal.step_up_max_age; equal is fresh), confirmed with a passkey or a
// device key, not consumed by another opening (step_up_consumed_at NULL or before step_up_at), and
// not in the web console, which fails closed until M6-18 adds the passkey rule with user
// verification (08 section 6.3 point 3).
//
// Example:
//
//	ok := domain.FreshStepUp(domain.FreshFacts{StepUpAt: t, StepUpMethod: "device_key"}, now, maxAge)
func FreshStepUp(f FreshFacts, now time.Time, maxAge time.Duration) bool {
	if f.StepUpAt.IsZero() || now.Sub(f.StepUpAt) > maxAge || f.ClientComponent == ComponentConsole {
		return false
	}
	if f.StepUpMethod != StepUpPasskey && f.StepUpMethod != StepUpDeviceKey {
		return false
	}
	return f.StepUpConsumedAt.IsZero() || f.StepUpConsumedAt.Before(f.StepUpAt)
}

// TerminalSessionFacts are the facts of a terminal session that the quick reattach needs
// (terminal_sessions.state, device_id, auth_session_id and detached_at); the domain mirror of
// kernel/authorize.TerminalFacts.
type TerminalSessionFacts struct {
	SessionState                          string
	SessionDeviceID, SessionAuthSessionID ids.ID
	DetachedAt                            time.Time
}

// ReattachFacts are the facts of a quick reattach: the action, the device and login session of the
// request, its client component and the session to attach.
type ReattachFacts struct {
	Action                                string
	PrincipalDeviceID, PrincipalSessionID ids.ID
	ClientComponent                       string
	Session                               TerminalSessionFacts
}

// QuickReattach reports the exception of 08 section 6.3 point 5 and 24 section 6.2:
// terminal.attach of a detached session from the same device in the same login session not later
// than window (terminal.quick_reattach_window; equal is in time) after the detachment needs no new
// step-up. Never in the web console, whose requests no device key proves. It extends no limit of
// the session.
func QuickReattach(f ReattachFacts, now time.Time, window time.Duration) bool {
	s := f.Session
	switch {
	case f.Action != ActionTerminalAttach, f.ClientComponent == ComponentConsole, s.SessionState != sessionDetached:
		return false
	case s.SessionDeviceID.IsZero() || s.SessionDeviceID != f.PrincipalDeviceID:
		return false
	case s.SessionAuthSessionID.IsZero() || s.SessionAuthSessionID != f.PrincipalSessionID:
		return false
	case s.DetachedAt.IsZero():
		return false
	}
	return now.Sub(s.DetachedAt) <= window
}

// SessionUserFact is a valid assignment of the principal to a session user of one host
// (host_session_users with revoked_at IS NULL); Reported reports that the host names OSUser in
// reported_session_users, compared exactly (24 section 5.3).
type SessionUserFact struct {
	HostID, HostSessionUserID ids.ID
	OSUser                    string
	Reported                  bool
}

// TerminalGuardFacts are the facts of the guards of terminal.open and terminal.attach: the
// principal, whether it is an effective owner, the host and the principal's session user there
// (nil for none).
type TerminalGuardFacts struct {
	PrincipalID ids.ID
	Owner       bool
	Host        HostFacts
	SessionUser *SessionUserFact
}

// TerminalGuard applies SR10, SR11 and SR12 in this order (07 section 4.2 step 9, 24 section 6.1
// step 2); the first failure wins:
//
//  1. SR10 server host: a person who is not an effective owner on a host declared or reported as
//     the server host gets terminal_server_host_owner_only;
//  2. SR10 assignee: such a person on a host not assigned to them gets terminal_not_host_assignee
//     (the branch of an own ready dev environment is false until V2);
//  3. SR11 (I6): on a host with a personal LLM login only its holder passes, other owners
//     included, else i6_holder_only;
//  4. SR12: without a valid assignment to a session user the host reports, owners included,
//     terminal_no_session_user.
//
// Extension point after SR12 (no stub here): M2-44 adds host_not_active and
// terminal_profile_unknown, M2-46 terminal_session_limit, in the order of 24 section 6.1 step 2.
// It returns the reason and false, or "" and true.
func TerminalGuard(f TerminalGuardFacts) (Reason, bool) {
	h := f.Host
	if !f.Owner && (h.DeclaredServerHost || h.ReportedIsServerHost) {
		return ReasonTerminalServerHostOwnerOnly, false
	}
	if !f.Owner && (h.AssignedUserID.IsZero() || h.AssignedUserID != f.PrincipalID) {
		return ReasonTerminalNotHostAssignee, false
	}
	if h.LLMCredentials == LLMPersonal && (h.CredentialHolderID.IsZero() || h.CredentialHolderID != f.PrincipalID) {
		return ReasonI6HolderOnly, false
	}
	if f.SessionUser == nil || !f.SessionUser.Reported || f.SessionUser.HostID != h.ID {
		return ReasonTerminalNoSessionUser, false
	}
	return "", true
}

// TerminalNotEffective returns the first guard of SR10 and SR12 that would deny the principal on
// the host, without SR11, whose I6 counterpart the preview reports as a violation (07 section 2.12
// point 4, Design note 13 of M2-40): terminal_server_host_owner_only,
// terminal_not_host_assignee or terminal_no_session_user; "" when both hold.
func TerminalNotEffective(f TerminalGuardFacts) Reason {
	f.Host.LLMCredentials = ""
	r, _ := TerminalGuard(f)
	return r
}

// TerminalEffective is the effective right terminal of the principal with snapshot s on hostID as
// 07 section 2.12 point 3 defines it, the I6 clause included: TerminalOn without the owner part
// when s.PersonalHosts names another holder than the principal for the host. The difference of
// access.changed and the preview effects use it (Design note 7 of M2-40); authorize keeps
// TerminalOn and denies such an owner with SR11 (i6_holder_only).
func TerminalEffective(s Snapshot, hostID ids.ID) bool {
	if holder, personal := s.PersonalHosts[hostID]; s.Owner && personal && holder != s.PrincipalID {
		return s.Hosts[hostID].Has(CapTerminal)
	}
	return TerminalOn(s, hostID)
}
