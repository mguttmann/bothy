// SPDX-License-Identifier: Apache-2.0

package computer

// Result codes of an action (docs/spec/27-computer-use.md 7 and 9.4).
const (
	CodeOK              = "ok"
	CodeScreenLocked    = "screen_locked"
	CodeSecureDesktop   = "secure_desktop"
	CodeOSRefused       = "os_refused"
	CodeInvalidAction   = "invalid_action"
	CodeStaleScreenshot = "stale_screenshot"
)

// Close reasons the host service sends in computer.ended (docs/spec/27-computer-use.md 12.3).
const (
	ReasonDone           = "done"
	ReasonStoppedByUser  = "stopped_by_user"
	ReasonDeviceDisabled = "device_disabled"
	ReasonConnectionLost = "connection_lost"
	ReasonOpenFailed     = "open_failed"
	ReasonError          = "error"
)

// Close details of docs/spec/27-computer-use.md 8.3.
const (
	DetailLocalEscape     = "local_escape"
	DetailIndicatorStop   = "indicator_stop"
	DetailTokenInvalid    = "token_invalid"
	DetailTokenExpired    = "token_expired"
	DetailTokenReplayed   = "token_replayed"
	DetailHandshakeFailed = "handshake_failed"
	DetailHelperAbsent    = "helper_absent"
	DetailHelperUntrusted = "helper_untrusted"
	DetailNotEnabled      = "not_enabled"
	DetailBusy            = "busy"
	DetailOpenTimeout     = "open_timeout"
	DetailStreamFault     = "stream_fault"
	DetailHelperCrashed   = "helper_crashed"
)

// The reason of cu_state paused.
const reasonLocked = "locked"

// Close codes of the data connection (docs/spec/27-computer-use.md 5.3, 24 13).
const (
	codeNormal        = 1000
	codeTokenStale    = 4101
	codeHandshakeFail = 4104
)

// CloseCodeOf returns the close code of a refusal of message 1 with detail (27 5.3): 4101 for
// token_expired, token_replayed, helper_absent, not_enabled and busy, 4104 for every other detail.
func CloseCodeOf(detail string) int {
	switch detail {
	case DetailTokenExpired, DetailTokenReplayed, DetailHelperAbsent, DetailNotEnabled, DetailBusy:
		return codeTokenStale
	default:
		return codeHandshakeFail
	}
}
