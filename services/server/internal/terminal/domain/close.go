// SPDX-License-Identifier: Apache-2.0

package domain

// CloseReason is terminal_sessions.close_reason (24 section 10.3).
type CloseReason string

// The closed list of 24 section 10.3, in the order of TerminalCloseReason in openapi.yaml.
const (
	CloseClientClosed       CloseReason = "client_closed"
	CloseClosedByOwner      CloseReason = "closed_by_owner"
	CloseProcessExited      CloseReason = "process_exited"
	CloseIdleTimeout        CloseReason = "idle_timeout"
	CloseMaxDuration        CloseReason = "max_duration"
	CloseDetachExpired      CloseReason = "detach_expired"
	CloseAccessRevoked      CloseReason = "access_revoked"
	CloseI6Violation        CloseReason = "i6_violation"
	CloseHostDisabled       CloseReason = "host_disabled"
	CloseHostRetired        CloseReason = "host_retired"
	CloseHostRekeyed        CloseReason = "host_rekeyed"
	CloseHostRestarted      CloseReason = "host_restarted"
	CloseUserDisabled       CloseReason = "user_disabled"
	CloseDeviceRevoked      CloseReason = "device_revoked"
	CloseSessionRevoked     CloseReason = "session_revoked"
	CloseLogout             CloseReason = "logout"
	CloseOpenFailed         CloseReason = "open_failed"
	CloseHostPolicyViolated CloseReason = "host_policy_violation"
)

// CloseReasons returns the closed list of close reasons.
func CloseReasons() []CloseReason {
	return []CloseReason{CloseClientClosed, CloseClosedByOwner, CloseProcessExited, CloseIdleTimeout, CloseMaxDuration,
		CloseDetachExpired, CloseAccessRevoked, CloseI6Violation, CloseHostDisabled, CloseHostRetired, CloseHostRekeyed,
		CloseHostRestarted, CloseUserDisabled, CloseDeviceRevoked, CloseSessionRevoked, CloseLogout, CloseOpenFailed,
		CloseHostPolicyViolated}
}

// CloseDetail is terminal_sessions.close_detail, a code of a closed list, never free text (24
// section 10.3).
type CloseDetail string

// The closed list of close details, in the order of TerminalCloseDetail in openapi.yaml.
const (
	DetailTokenInvalid              CloseDetail = "token_invalid"
	DetailTokenExpired              CloseDetail = "token_expired"
	DetailTokenReplayed             CloseDetail = "token_replayed"
	DetailHandshakeFailed           CloseDetail = "handshake_failed"
	DetailProfileUnknown            CloseDetail = "profile_unknown"
	DetailSessionUserUnknown        CloseDetail = "session_user_unknown"
	DetailSessionUserMismatch       CloseDetail = "session_user_mismatch"
	DetailServerHostOwnerOnly       CloseDetail = "server_host_owner_only"
	DetailLocalConfirmDenied        CloseDetail = "local_confirm_denied"
	DetailSubscriptionLoginDetected CloseDetail = "subscription_login_detected"
	DetailHelperUntrusted           CloseDetail = "helper_untrusted"
	DetailSharedLocation            CloseDetail = "shared_location"
	DetailPTYFailed                 CloseDetail = "pty_failed"
	DetailOpenTimeout               CloseDetail = "open_timeout"
)

// CloseDetails returns the closed list of close details.
func CloseDetails() []CloseDetail {
	return []CloseDetail{DetailTokenInvalid, DetailTokenExpired, DetailTokenReplayed, DetailHandshakeFailed,
		DetailProfileUnknown, DetailSessionUserUnknown, DetailSessionUserMismatch, DetailServerHostOwnerOnly,
		DetailLocalConfirmDenied, DetailSubscriptionLoginDetected, DetailHelperUntrusted, DetailSharedLocation,
		DetailPTYFailed, DetailOpenTimeout}
}
