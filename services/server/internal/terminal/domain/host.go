// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Host is a row of hosts (03 section 4.14) as the use cases read and write it. The reported
// columns are kept as decoded lists; the key material only as far as the API shows it.
type Host struct {
	ID    ids.ID
	Name  string
	State State
	// DisabledReason is set exactly with StateDisabled; DisabledAt describes the current disabled
	// state (ruling of M2-39: cleared on enable and retire).
	DisabledReason string
	DisabledAt     time.Time
	RetiredAt      time.Time
	Platform       string
	AssignedUserID ids.ID
	LLMCredentials string
	// CredentialHolderID is set exactly with personal.
	CredentialHolderID   ids.ID
	DeclaredServerHost   bool
	ReportedIsServerHost *bool
	RequireLocalConfirm  *bool
	// PairingCodeHash is the HMAC of the pairing code, set exactly in pairing; never shown.
	PairingCodeHash  []byte
	PairingExpiresAt time.Time
	NoisePublicKey   []byte
	// AuthJKT is the RFC 7638 thumbprint of the login key; "" while the host has no keys.
	AuthJKT string
	// AuthPublicJWK is hosts.auth_public_jwk, the stored JWK of the login key; nil without keys
	// (M2-43: the proof of identify verifies with it).
	AuthPublicJWK []byte
	// KeyFingerprint is the fingerprint of the stored keys; after the keys are dropped it is kept
	// as the digest of the dropped keys (M2-41, Design note 9), never shown without keys.
	KeyFingerprint string
	KeyGeneration  int64
	// FingerprintFailures counts the failed confirmations since the last pairing (TM-T19).
	FingerprintFailures int
	PairedAt            time.Time
	Profiles            []Profile
	ReportedUsers       []ReportedSessionUser
	Doctor              []DoctorCheck
	OSVersion           string
	HostVersion         string
	LastSeenAt          time.Time
	CreatedBy           ids.ID
	CreatedAt           time.Time
	Version             int64
}

// Paired reports a host that has been paired at least once (paired_at set): only such a host has
// reported session users (schema Host, "Absent before pairing").
func (h Host) Paired() bool { return !h.PairedAt.IsZero() }

// HasKeys reports a host with stored keys (noise_public_key set): only then are the fingerprint
// and the Noise key part of its representation (M2-41, Design note 9).
func (h Host) HasKeys() bool { return len(h.NoisePublicKey) > 0 }

// SessionUser is a valid assignment of a person to a session user on a host, for the view.
type SessionUser struct {
	UserID    ids.ID
	OSUser    string
	CreatedAt time.Time
}

// Session is a row of terminal_sessions as the end of a session and terminal_session.changed read
// it (metadata only, never content).
type Session struct {
	ID       ids.ID
	HostID   ids.ID
	UserID   ids.ID
	State    string
	OpenedAt time.Time
	BytesIn  int64
	BytesOut int64
	// DeviceID and Profile are the device and the profile of the session (M2-43:
	// terminal_session.changed carries them).
	DeviceID ids.ID
	Profile  string
	// StartedAt, DetachedAt and ClosedAt are zero when NULL.
	StartedAt, DetachedAt, ClosedAt time.Time
	// CloseReason and CloseDetail are "" while the session is open; ExitCode is nil unless the
	// process exited with a code.
	CloseReason CloseReason
	CloseDetail CloseDetail
	ExitCode    *int
	// Version is terminal_sessions.version.
	Version int64
}

// Apply returns h after the transition r at t: the state, the disabled reason and disabled_at
// (set with disabled, cleared otherwise), retired_at with retired, and the next version.
func (h Host) Apply(r Result, t time.Time) Host {
	h.State, h.DisabledReason = r.To, r.DisabledReason
	h.DisabledAt = time.Time{}
	if r.To == StateDisabled {
		h.DisabledAt = t
	}
	if r.To == StateRetired {
		h.RetiredAt = t
	}
	h.Version++
	return h
}

// OwnSessionUser is a valid assignment of one person with the names its host reports (M2-40,
// 07 SR12): the input of the session users in the rights state of the person.
type OwnSessionUser struct {
	ID, HostID ids.ID
	OSUser     string
	// HostReport is hosts.reported_session_users of the host.
	HostReport []ReportedSessionUser
}
