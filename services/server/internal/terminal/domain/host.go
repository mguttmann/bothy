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
	KeyFingerprint   string
	KeyGeneration    int64
	PairedAt         time.Time
	Profiles         []Profile
	ReportedUsers    []ReportedSessionUser
	Doctor           []DoctorCheck
	OSVersion        string
	HostVersion      string
	LastSeenAt       time.Time
	CreatedBy        ids.ID
	CreatedAt        time.Time
	Version          int64
}

// Paired reports a host that has been paired at least once (paired_at set): only such a host has
// reported session users (schema Host, "Absent before pairing").
func (h Host) Paired() bool { return !h.PairedAt.IsZero() }

// SessionUser is a valid assignment of a person to a session user on a host, for the view.
type SessionUser struct {
	UserID    ids.ID
	OSUser    string
	CreatedAt time.Time
}

// Session is a row of terminal_sessions as the end of a session reads it (metadata only, never
// content).
type Session struct {
	ID       ids.ID
	HostID   ids.ID
	UserID   ids.ID
	State    string
	OpenedAt time.Time
	BytesIn  int64
	BytesOut int64
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
