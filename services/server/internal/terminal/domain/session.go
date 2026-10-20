// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// NoiseMaxMessage is the largest Noise message, and so the largest binary WebSocket message of the
// device stream and the data connection: 65535 bytes. A protocol constant of the Noise
// specification (section 3) and of docs/spec/24-terminal.md section 5.2, not a configuration key.
const NoiseMaxMessage = 65535

// The protocol labels of a grant (24 section 5.2): protocol constants, not configuration keys.
const (
	// NoiseProtocol is the Noise protocol name of the stream.
	NoiseProtocol = "Noise_NK_25519_ChaChaPoly_SHA256"
	// PrologueVersion is the version label of the prologue.
	PrologueVersion = "bothy-terminal/1"
)

// ErrSessionClosed is an attach to a closed session: 409 invalid_state_transition.
var ErrSessionClosed = errs.New(errs.CodeInvalidStateTransition, "The terminal session has ended.")

// AttachKind returns the claim att of an attach to a session in state (Design note 6 of M2-44,
// ruling D-M2-44-4): open for a session in opening, whose PTY never started, so the host service
// must open it; attach for active and detached. A closed session is ErrSessionClosed.
func AttachKind(state string) (string, error) {
	switch state {
	case SessionOpening:
		return AttOpen, nil
	case SessionActive, SessionDetached:
		return AttAttach, nil
	}
	return "", ErrSessionClosed
}

// QuickReattachHolds reports whether the locked row still allows the quick reattach the decision
// of authorize found (Design note 6 b of M2-44): detached, with the device and the login session of
// the request.
func QuickReattachHolds(s Session, deviceID, authSessionID ids.ID) bool {
	return s.State == SessionDetached && s.DeviceID == deviceID && s.AuthSessionID == authSessionID
}

// hostStateEnds are the close reasons of a host that left active (24 section 10.3).
var hostStateEnds = map[State]CloseReason{
	StateDisabled: CloseHostDisabled, StateRetired: CloseHostRetired, StatePairing: CloseHostRekeyed, StateConfirming: CloseHostRekeyed,
}

// RecheckEnd maps the reason of a recheck that no longer allows an opening or attach (Design note
// 12 of M2-44) to the close reason of the new session: i6_holder_only gives i6_violation,
// host_not_active the reason of the host state (disabled host_disabled, retired host_retired,
// pairing and confirming host_rekeyed), every other denial access_revoked.
func RecheckEnd(reason string, hostState State) CloseReason {
	switch reason {
	case "i6_holder_only":
		return CloseI6Violation
	case "host_not_active":
		if r, ok := hostStateEnds[hostState]; ok {
			return r
		}
	}
	return CloseAccessRevoked
}
