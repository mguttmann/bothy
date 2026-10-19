// SPDX-License-Identifier: Apache-2.0

package domain

// StreamCode is a WebSocket close code of the device stream (asyncapi channel terminalStream,
// x-close-codes; docs/spec/24-terminal.md section 13).
type StreamCode int

// The close codes of the device stream.
const (
	StreamNormal          StreamCode = 1000
	StreamDecodeError     StreamCode = 4002
	StreamAuthFailed      StreamCode = 4004
	StreamRevoked         StreamCode = 4012
	StreamServerRestart   StreamCode = 4013
	StreamSlowConsumer    StreamCode = 4014
	StreamTerminalClosed  StreamCode = 4100
	StreamTokenInvalid    StreamCode = 4101
	StreamHostUnavailable StreamCode = 4102
	StreamReplaced        StreamCode = 4103
	StreamHandshakeFailed StreamCode = 4104
)

// streamNames are the names of x-close-codes of terminalStream, the reason text of a close frame
// when no close_reason or close_detail is given.
var streamNames = map[StreamCode]string{
	StreamNormal: "normal", StreamDecodeError: "decode_error", StreamAuthFailed: "authentication_failed", StreamRevoked: "revoked",
	StreamServerRestart: "server_restart", StreamSlowConsumer: "slow_consumer", StreamTerminalClosed: "terminal_closed",
	StreamTokenInvalid: "terminal_token_invalid", StreamHostUnavailable: "terminal_host_unavailable", StreamReplaced: "terminal_replaced",
	StreamHandshakeFailed: "terminal_handshake_failed",
}

// Name returns the name of c in x-close-codes; "" for a code outside the list.
func (c StreamCode) Name() string { return streamNames[c] }

// tokenDetails are the details of 24 section 5.3 whose code is 4101: an expired or replayed
// session token, which also happens without an attack.
var tokenDetails = map[CloseDetail]bool{DetailTokenExpired: true, DetailTokenReplayed: true}

// handshakeDetails are the details of 24 section 5.3 whose code is 4104: everything that points to
// manipulation.
var handshakeDetails = map[CloseDetail]bool{
	DetailTokenInvalid: true, DetailHandshakeFailed: true, DetailProfileUnknown: true, DetailSessionUserUnknown: true,
	DetailSessionUserMismatch: true, DetailServerHostOwnerOnly: true,
}

// StreamCodeForEnd returns the close code of the device stream for a session that ends with
// reason and detail while a device is connected (Design note 8 of M2-44, 24 section 5.3): 4101
// for token_expired and token_replayed, 4104 for token_invalid, handshake_failed,
// profile_unknown, session_user_unknown, session_user_mismatch and server_host_owner_only, 4100
// for every other end. The reason only names the end; the detail decides, so a refusal the host
// reported through session.ended reaches the device with the same code as a refused data
// connection (R-TM-16).
func StreamCodeForEnd(_ CloseReason, detail CloseDetail) StreamCode {
	switch {
	case tokenDetails[detail]:
		return StreamTokenInvalid
	case handshakeDetails[detail]:
		return StreamHandshakeFailed
	}
	return StreamTerminalClosed
}

// HostRefusal reports a close code of a data connection with which the host service refuses a
// handshake (ruling D-M2-44-6): 4101 or 4104 with a close_detail of 24 section 5.3 as the reason.
// It returns the detail and true, or false for any other close.
func HostRefusal(code int, reason string) (CloseDetail, bool) {
	d := CloseDetail(reason)
	switch StreamCode(code) {
	case StreamTokenInvalid:
		return d, tokenDetails[d]
	case StreamHandshakeFailed:
		return d, handshakeDetails[d]
	}
	return "", false
}
