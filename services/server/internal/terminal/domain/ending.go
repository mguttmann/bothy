// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"slices"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// errEndPair is an end whose reason and detail break the pair rule of 24 section 10.3. It never
// names the values: a broken pair is a server fault, and the values are not for the caller.
var errEndPair = errs.New(errs.CodeInternalError, "The end of the terminal session has an invalid reason or detail.")

// ValidateEnd applies the pair rule of 24 section 10.3 to every end (Design note 2 of M2-46),
// server ends included, so no writer can store a pair the host could not report either: r must be
// one of CloseReasons; d must be empty or one of CloseDetails; a detail is required exactly with
// open_failed (any detail) and host_policy_violation (only the details of policyDetails), and
// every other reason carries none.
//
// Parameters: r is the close reason, d the close detail ("" for none).
// Returns nil for an allowed pair.
// Errors: an internal_error that names neither value for every other pair.
func ValidateEnd(r CloseReason, d CloseDetail) error {
	if !slices.Contains(CloseReasons(), r) || (d != "" && !slices.Contains(CloseDetails(), d)) {
		return errEndPair
	}
	allowed, needs := detailRule()[r]
	if needs && !slices.Contains(allowed, d) || !needs && d != "" {
		return errEndPair
	}
	return nil
}

// detailRule returns the reasons that require a close detail with the details each allows (24
// section 10.3): open_failed any detail, host_policy_violation those of policyDetails.
func detailRule() map[CloseReason][]CloseDetail {
	return map[CloseReason][]CloseDetail{CloseOpenFailed: CloseDetails(), CloseHostPolicyViolated: policyDetails}
}

// actionUserDisable is the authorize action that disables a person. terminal/domain may not
// import kernel/authorize (.go-arch-lint.yml), so the action arrives as its string.
const actionUserDisable = "user.disable"

// ReasonForLostHost returns the close reason of a session on h that the person userID lost through
// a rights change outside terminal (Design note 4 c of M2-46): user.disable gives user_disabled,
// because the identity is gone and not only the right; a host with personal LLM credentials of
// another holder gives i6_violation, because I6 is what forbids it; every other loss
// access_revoked.
//
// Parameters: action is the authorize action of the change (authorize.Action as a string), h the
// host as read in the transaction, userID the person whose sessions end.
// Returns the close reason.
func ReasonForLostHost(action string, h Host, userID ids.ID) CloseReason {
	switch {
	case action == actionUserDisable:
		return CloseUserDisabled
	case h.LLMCredentials == LLMPersonal && h.CredentialHolderID != userID:
		return CloseI6Violation
	}
	return CloseAccessRevoked
}

// ReasonForDeviceRevoke maps devices.revoke_reason (03) to the close reason of the sessions the
// device held (Design note 5 a of M2-46): user_disabled keeps the identity reason, every other
// revocation is device_revoked.
//
// Parameters: revokeReason is the revoke reason of the device.
// Returns the close reason.
func ReasonForDeviceRevoke(revokeReason string) CloseReason {
	if revokeReason == "user_disabled" {
		return CloseUserDisabled
	}
	return CloseDeviceRevoked
}

// ReasonForSessionEnd maps auth_sessions.revoke_reason (03) to the close reason of the terminal
// sessions of the ended login session (Design note 5 b of M2-46): logout, user_disabled and
// device_revoked keep their reason, so the person sees why; every other revocation is
// session_revoked.
//
// Parameters: revokeReason is the revoke reason of the login session.
// Returns the close reason.
func ReasonForSessionEnd(revokeReason string) CloseReason {
	switch revokeReason {
	case "logout":
		return CloseLogout
	case "user_disabled":
		return CloseUserDisabled
	case "device_revoked":
		return CloseDeviceRevoked
	}
	return CloseSessionRevoked
}

// HostStateEnd returns the close reason of a session on a host in state (24 section 10.3): disabled
// gives host_disabled, retired host_retired, pairing and confirming host_rekeyed. It shares
// hostStateEnds with RecheckEnd, so both paths name a host state the same way.
//
// Parameters: state is hosts.state.
// Returns the reason and true when the state ends sessions; "" and false for active and unknown
// states.
func HostStateEnd(state string) (CloseReason, bool) {
	r, ok := hostStateEnds[State(state)]
	return r, ok
}

// SR10Lost reports whether a person may no longer hold a session on h under 07 SR10 (Design note
// 4 b of M2-46): an active owner keeps every session; anyone else keeps it only on a host assigned
// to them that is no server host, neither declared nor reported. A host without assignee is
// lost even for a zero userID.
//
// Parameters: isActiveOwner is the person's owner status read in the transaction, userID the
// person, h the host read fresh.
// Returns true when the session must end.
func SR10Lost(isActiveOwner bool, userID ids.ID, h Host) bool {
	if isActiveOwner {
		return false
	}
	serverHost := h.DeclaredServerHost || (h.ReportedIsServerHost != nil && *h.ReportedIsServerHost)
	return h.AssignedUserID.IsZero() || h.AssignedUserID != userID || serverHost
}
