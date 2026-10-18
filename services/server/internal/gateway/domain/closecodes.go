// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"maps"
	"slices"
)

// CloseCode is a close code of the WebSocket gateway (docs/spec/04-sync-protocol.md section 6.4,
// x-close-codes of the channel gateway in docs/api/asyncapi.yaml). The doc comment of each
// constant names the reaction the client owes; TestCloseCodesMatchAsyncAPI holds codes and names
// equal to the contract.
type CloseCode int

// The close codes of the gateway.
const (
	// CloseNormal (normal): do not reconnect, unless the app is in the foreground.
	CloseNormal CloseCode = 1000
	// CloseGoingAway (going_away): reconnect with backoff, try resume.
	CloseGoingAway CloseCode = 1001
	// CloseUnknownError (unknown_error): reconnect with backoff.
	CloseUnknownError CloseCode = 4000
	// CloseUnknownOp (unknown_op): report the error, reconnect.
	CloseUnknownOp CloseCode = 4001
	// CloseDecodeError (decode_error): report the error, reconnect.
	CloseDecodeError CloseCode = 4002
	// CloseNotIdentified (not_identified): an op before identify, or no identify in time; reconnect.
	CloseNotIdentified CloseCode = 4003
	// CloseAuthenticationFailed (authentication_failed): renew the token; if that fails, log out.
	CloseAuthenticationFailed CloseCode = 4004
	// CloseAlreadyIdentified (already_identified): report the error, reconnect.
	CloseAlreadyIdentified CloseCode = 4005
	// CloseSessionInvalid (session_invalid): identify instead of resume.
	CloseSessionInvalid CloseCode = 4006
	// CloseProtocolVersionUnsupported (protocol_version_unsupported): show an app update, do not
	// reconnect.
	CloseProtocolVersionUnsupported CloseCode = 4007
	// CloseRateLimited (rate_limited): wait for the seconds of the close reason retry_after=<s>.
	CloseRateLimited CloseCode = 4008
	// CloseHeartbeatTimeout (heartbeat_timeout): reconnect with backoff, try resume.
	CloseHeartbeatTimeout CloseCode = 4009
	// CloseTokenExpired (token_expired): renew the token, then resume.
	CloseTokenExpired CloseCode = 4011
	// CloseRevoked (revoked): session, device or principal revoked; delete local data, log out,
	// do not reconnect.
	CloseRevoked CloseCode = 4012
	// CloseServerRestart (server_restart): reconnect at once with a random delay, try resume.
	CloseServerRestart CloseCode = 4013
	// CloseSlowConsumer (slow_consumer): reconnect, catch up from the cursor.
	CloseSlowConsumer CloseCode = 4014
	// CloseConsumerReplaced (consumer_replaced): agents only; do not reconnect.
	CloseConsumerReplaced CloseCode = 4015
	// CloseDevicePending (device_pending): do not reconnect until the device is active.
	CloseDevicePending CloseCode = 4016
	// CloseDeviceReplaced (device_replaced): a newer connection of the device is active; do not
	// reconnect on your own.
	CloseDeviceReplaced CloseCode = 4017
	// CloseForbidden (forbidden): credentials valid, gateway.connect denied; show the error, do not
	// renew the token, do not reconnect on your own.
	CloseForbidden CloseCode = 4018
)

// closeNames are the names of x-close-codes.
var closeNames = map[CloseCode]string{
	CloseNormal: "normal", CloseGoingAway: "going_away", CloseUnknownError: "unknown_error",
	CloseUnknownOp: "unknown_op", CloseDecodeError: "decode_error", CloseNotIdentified: "not_identified",
	CloseAuthenticationFailed: "authentication_failed", CloseAlreadyIdentified: "already_identified",
	CloseSessionInvalid: "session_invalid", CloseProtocolVersionUnsupported: "protocol_version_unsupported",
	CloseRateLimited: "rate_limited", CloseHeartbeatTimeout: "heartbeat_timeout", CloseTokenExpired: "token_expired",
	CloseRevoked: "revoked", CloseServerRestart: "server_restart", CloseSlowConsumer: "slow_consumer",
	CloseConsumerReplaced: "consumer_replaced", CloseDevicePending: "device_pending", CloseDeviceReplaced: "device_replaced",
	CloseForbidden: "forbidden",
}

// Name returns the contract name of c, for example "decode_error", or "" for a code the gateway
// does not know. The gateway uses it as the default close reason (machine data only).
func (c CloseCode) Name() string { return closeNames[c] }

// CloseCodes returns every close code of the gateway, sorted, in a new slice.
func CloseCodes() []CloseCode {
	return slices.Sorted(maps.Keys(closeNames))
}

// The values of bothy.outcome of the span gateway.connect (docs/observability/signals.md).
const (
	OutcomeOK          = "ok"
	OutcomeClientError = "client_error"
	OutcomeServerError = "server_error"
)

// clientCodes are the close codes that are the client's doing (signals.md: 4001 to 4008, 4011,
// 4012, 4016 to 4018); they never spend the error budget of reconnecting.
var clientCodes = map[CloseCode]bool{
	CloseUnknownOp: true, CloseDecodeError: true, CloseNotIdentified: true, CloseAuthenticationFailed: true,
	CloseAlreadyIdentified: true, CloseSessionInvalid: true, CloseProtocolVersionUnsupported: true,
	CloseRateLimited: true, CloseTokenExpired: true, CloseRevoked: true, CloseDevicePending: true,
	CloseDeviceReplaced: true, CloseForbidden: true,
}

// Outcome returns bothy.outcome of a connection that ended with code c (0 when the client went
// away without a server close) after ready or resumed (ready true) or before: a client code, and
// a client that leaves before ready, is client_error; 4000 and 4013 are server_error; every other
// end is ok after ready and server_error before it.
func Outcome(c CloseCode, ready bool) string {
	switch {
	case clientCodes[c]:
		return OutcomeClientError
	case c == CloseUnknownError || c == CloseServerRestart:
		return OutcomeServerError
	case ready:
		return OutcomeOK
	case c == 0:
		return OutcomeClientError
	}
	return OutcomeServerError
}
