// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"time"

	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/instance"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/terminal/domain"
)

// Store is the outgoing port of the terminal use cases over hosts, host_session_users and
// terminal_sessions (implemented by adapters/postgres). Reads run in the ambient transaction of
// ctx when there is one and on the pool otherwise; writes and locks run only inside a unit of
// work and fail with kind internal outside one. Errors are kernel/errs errors without SQL text or
// values.
type Store interface {
	// Host returns the host id; found is false for an unknown id.
	Host(ctx context.Context, id ids.ID) (h domain.Host, found bool, err error)
	// LockHost returns the host like Host and locks its row (03 section 6.3).
	LockHost(ctx context.Context, id ids.ID) (h domain.Host, found bool, err error)
	// ListHosts returns up to limit hosts after the keyset position (lower(name), id) =
	// (afterName, afterID), ordered by lower(name) then id: every host with all, else those of
	// hostIDs.
	ListHosts(ctx context.Context, all bool, hostIDs []ids.ID, afterName string, afterID ids.ID, limit int) ([]domain.Host, error)
	// HostIDs returns every host, ascending.
	HostIDs(ctx context.Context) ([]ids.ID, error)
	// AssignedTo returns the hosts whose holder is userID, ascending.
	AssignedTo(ctx context.Context, userID ids.ID) ([]ids.ID, error)
	// InsertHost inserts a new host in pairing. A name taken by a host that is not retired
	// (without regard to case) is 409 conflict.
	InsertHost(ctx context.Context, h domain.Host) error
	// UpdateHost writes name, holder, state, disabled_at, disabled_reason, retired_at and version
	// of h.
	UpdateHost(ctx context.Context, h domain.Host) error
	// LockSessionUsers returns every session user row of hostID, valid and ended, locked in
	// ascending id.
	LockSessionUsers(ctx context.Context, hostID ids.ID) ([]domain.SessionUserRow, error)
	// SessionUsers returns every session user row of hostID like LockSessionUsers, without a
	// lock: the check before the rights change must not lock before permission_version (03
	// section 6.3).
	SessionUsers(ctx context.Context, hostID ids.ID) ([]domain.SessionUserRow, error)
	// ValidSessionUsers returns the valid assignments of the hosts, per host ordered by creation
	// then id; a host without one is absent.
	ValidSessionUsers(ctx context.Context, hostIDs []ids.ID) (map[ids.ID][]domain.SessionUser, error)
	// InsertSessionUser inserts the assignment id of a on hostID by by at at. A taken name or a
	// second valid assignment (the unique indexes) is 409 conflict with reason session_user_taken.
	InsertSessionUser(ctx context.Context, id, hostID ids.ID, a domain.SessionUserAssign, by ids.ID, at time.Time) error
	// RevokeSessionUser ends the valid assignment id (revoked_at, revoked_by); the row stays.
	RevokeSessionUser(ctx context.Context, id, by ids.ID, at time.Time) error
	// LockOpenSessions returns the sessions of hostID that are not closed, of userID or of every
	// person (zero userID), locked in ascending id.
	LockOpenSessions(ctx context.Context, hostID, userID ids.ID) ([]domain.Session, error)
	// CloseSession closes the session id as the server with reason at at: closed_at,
	// close_reason, detached_at cleared, no detail, no closer, version raised.
	CloseSession(ctx context.Context, id ids.ID, reason domain.CloseReason, at time.Time) error
	// UpdateLlmCredentials writes the declaration d of the host id with llm_credentials_declared_by
	// = by, llm_credentials_declared_at = at and version; credential_holder_id is NULL unless
	// personal (M2-40).
	UpdateLlmCredentials(ctx context.Context, id ids.ID, d domain.Declaration, by ids.ID, at time.Time, version int64) error
	// PersonalHosts returns the hosts with llm_credentials = personal, ascending (M2-40).
	PersonalHosts(ctx context.Context) ([]domain.Host, error)
	// UpdateReportedSessionUsers writes reported_session_users of the host id and version (M2-40).
	UpdateReportedSessionUsers(ctx context.Context, id ids.ID, report []domain.ReportedSessionUser, version int64) error
	// OwnSessionUsers returns the valid assignments of userID with the names their hosts report,
	// ascending by host (M2-40, 07 SR12).
	OwnSessionUsers(ctx context.Context, userID ids.ID) ([]domain.OwnSessionUser, error)
	// HostByPairingHash returns the host whose pairing_code_hash is hash, without a lock (M2-41);
	// found is false when no host holds it.
	HostByPairingHash(ctx context.Context, hash []byte) (h domain.Host, found bool, err error)
	// SetPairingCode puts the host id into pairing with a new code (M2-41): the hash, its expiry,
	// fingerprint_failures 0, the three key columns NULL when dropKeys (key_fingerprint stays),
	// disabled_at and disabled_reason NULL, and version.
	SetPairingCode(ctx context.Context, id ids.ID, hash []byte, expires time.Time, dropKeys bool, version int64) error
	// StorePairing writes the redemption p of the host id (M2-41): keys, fingerprint, the next
	// key generation, the report, the code used up, state confirming and version. A login key of
	// another host (the unique index on auth_jkt) is 409 conflict with reason host_key_in_use.
	StorePairing(ctx context.Context, id ids.ID, p PairingWrite, version int64) error
	// SetFingerprintFailures writes the failure counter of the host id; the version stays.
	SetFingerprintFailures(ctx context.Context, id ids.ID, n int) error
	// ConfirmPairing writes the confirmation of the host id (M2-41): declared_server_host,
	// confirmed_by, confirmed_at, fingerprint_failures 0, state active and version.
	ConfirmPairing(ctx context.Context, id ids.ID, declaredServerHost bool, by ids.ID, at time.Time, version int64) error
	// UpdateHostLogin writes the login of the host id (M2-43): reported_host_version,
	// last_seen_at and version (the caller passes the current version when the host version is
	// unchanged).
	UpdateHostLogin(ctx context.Context, id ids.ID, hostVersion string, seen time.Time, version int64) error
	// TouchLastSeen writes last_seen_at = at of the host id when it is older than olderThan or
	// NULL; the version stays (M2-43).
	TouchLastSeen(ctx context.Context, id ids.ID, at, olderThan time.Time) error
	// UpdateHostReport writes the reported columns of r besides the session users and version
	// (M2-43): reported_profiles, reported_doctor, reported_is_server_host,
	// reported_require_local_confirm and reported_os_version (NULL for "").
	UpdateHostReport(ctx context.Context, id ids.ID, r domain.Report, version int64) error
	// LockSession returns the session id and locks its row; found is false for an unknown id.
	LockSession(ctx context.Context, id ids.ID) (s domain.Session, found bool, err error)
	// SessionsByIDs returns the sessions of the ids that exist, ascending, without a lock.
	SessionsByIDs(ctx context.Context, list []ids.ID) ([]domain.Session, error)
	// StartSession turns the opening session id active with started_at = at.
	StartSession(ctx context.Context, id ids.ID, at time.Time) error
	// DetachSession turns the active session id detached with detached_at = at.
	DetachSession(ctx context.Context, id ids.ID, at time.Time) error
	// CloseSessionReported closes the session id as the host service ended it: reason, detail
	// and exit code of end, closed_at = at, detached_at cleared, version raised.
	CloseSessionReported(ctx context.Context, id ids.ID, end domain.HostSessionEnd, at time.Time) error
}

// PairingWrite is what host.pair stores (M2-41, Design note 3h): the checked keys, the
// fingerprint and the checked report.
type PairingWrite struct {
	Keys        domain.HostKeys
	Fingerprint string
	PairedAt    time.Time
	Report      domain.PairReport
}

// PairingCodes issues and hashes pairing codes of hosts; internal/wire binds it to
// identity.Tokens (NewHostPairingCode, HashHostPairingCode), so the format and the hash
// derivation exist once (M2-41, Design note 1).
type PairingCodes interface {
	// Issue returns a new code with its HMAC; it reads no database.
	Issue(ctx context.Context) (identity.PairingCode, error)
	// Hash returns the HMAC of raw; identity.ErrTokenMalformed for a value that is not a
	// well-formed code of kind hpc, without any database access.
	Hash(ctx context.Context, raw string) ([]byte, error)
}

// Instance gives the identity of the installation for the answer of pairHost (M2-41); internal/wire
// binds it to instance.State.
type Instance interface {
	// InstanceID returns instance.instance_id.
	InstanceID(ctx context.Context) (ids.ID, error)
}

// UserFacts are the facts of a principal that the holder and the session users need.
type UserFacts struct {
	Kind   actor.Kind
	Status identity.Status
}

// People reads principals for the checks of a holder and of the people of session users;
// internal/wire binds it to identity.Directory.
type People interface {
	// User returns the facts of the principal id; found is false for an unknown id.
	User(ctx context.Context, id ids.ID) (f UserFacts, found bool, err error)
	// IsActiveOwner reports a person with an active owners row and status active (M2-40, SR10 on a
	// change of the assignee); false for an unknown id. It calls no authorize.
	IsActiveOwner(ctx context.Context, userID ids.ID) (bool, error)
}

// Presence reports whether the control connection of a host is open and usable for sessions (24
// section 3.5). A process without the route /hosts/connect keeps NoConnections; the api roles
// bind the registry of adapters/wsrelay through BindControl (M2-43).
type Presence interface {
	// Connected reports an open control connection of hostID after its first report.
	Connected(ctx context.Context, hostID ids.ID) bool
}

// NoConnections is the Presence of a server without the route /hosts/connect: no host is
// connected, which is the truth there.
type NoConnections struct{}

// Connected implements Presence: always false.
func (NoConnections) Connected(context.Context, ids.ID) bool { return false }

// SessionOpen is the message session.open of the server (schema HostSessionOpen): the session, the
// path of its data connection, the profile and the instant after which the host service drops the
// request (terminal.open_timeout). M2-44 produces it.
type SessionOpen struct {
	SessionID ids.ID
	DataPath  string
	Profile   string
	ExpiresAt time.Time
}

// The answers of ControlLinks.OpenSession that M2-44 maps to 409 host_unavailable.
var (
	// ErrHostNotConnected is a host without a control connection in this process.
	ErrHostNotConnected = errors.New("terminal: the host service is not connected")
	// ErrHostNotReady is a control connection whose first report after ready has not been applied.
	ErrHostNotReady = errors.New("terminal: the host service has not reported yet")
)

// ControlLinks is the outgoing port to the control connections of this process (M2-43 Design note
// 7), implemented by the registry of adapters/wsrelay. M2-47 adds the path across nodes behind it
// without changing callers. Every method is safe for concurrent use and never waits on the
// network.
type ControlLinks interface {
	// OpenSession enqueues session.open on the link of hostID. Errors: ErrHostNotConnected without
	// a link, ErrHostNotReady before the first applied report after ready.
	OpenSession(ctx context.Context, hostID ids.ID, cmd SessionOpen) error
	// CloseSession enqueues session.close with reason on the link of hostID; without a link
	// nothing happens (the next login catches it, 24 section 6.2).
	CloseSession(ctx context.Context, hostID, sessionID ids.ID, reason domain.CloseReason) error
	// CloseHost closes the link of hostID with code; callers call it after their commit.
	CloseHost(hostID ids.ID, code domain.CloseCode)
	// Connected reports a link of hostID whose first report after ready was applied.
	Connected(hostID ids.ID) bool
	// SendReady sends ready again on the link of hostID with the declaration read now; without a
	// link nothing happens.
	SendReady(hostID ids.ID)
}

// NoLinks is the ControlLinks of a process without the route /hosts/connect: no host is
// connected; closing and sending do nothing.
type NoLinks struct{}

// OpenSession implements ControlLinks: ErrHostNotConnected.
func (NoLinks) OpenSession(context.Context, ids.ID, SessionOpen) error { return ErrHostNotConnected }

// CloseSession implements ControlLinks: nothing to close.
func (NoLinks) CloseSession(context.Context, ids.ID, ids.ID, domain.CloseReason) error { return nil }

// CloseHost implements ControlLinks: nothing to close.
func (NoLinks) CloseHost(ids.ID, domain.CloseCode) {
	// Without control connections no host is connected, so there is nothing to close.
}

// Connected implements ControlLinks: false.
func (NoLinks) Connected(ids.ID) bool { return false }

// SendReady implements ControlLinks: nothing to send.
func (NoLinks) SendReady(ids.ID) {
	// Without control connections there is no connection to send ready on.
}

// TerminalKeys reads the published signing keys of purpose terminal (instance.Keys); the control
// connection hands them to the host service in hello and heartbeat_ack (24 section 5.3).
type TerminalKeys interface { // NOSONAR: named after what it reads, like instance.Keys; see D-M2-43-24.
	// PublicKeys returns the next, active and retiring keys of p, read fresh.
	PublicKeys(ctx context.Context, p instance.Purpose) ([]instance.PublicKey, error)
}

// Cursors encodes and checks the opaque cursors of listHosts (implemented by adapters/httpapi
// with platform/httpserver).
type Cursors interface {
	// Encode returns the cursor of key for list and binding.
	Encode(list string, binding []byte, key any) (string, error)
	// Decode checks s against list and binding and unmarshals its key into key. Errors:
	// cursor_invalid.
	Decode(list string, binding []byte, s string, key any) error
}
