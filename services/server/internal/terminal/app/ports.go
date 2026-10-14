// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/identity"
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
}

// PairingCodes issues pairing codes of hosts; internal/wire binds it to identity.Tokens
// (NewHostPairingCode). M2-41 reuses it for issueHostPairingCode.
type PairingCodes interface {
	// Issue returns a new code with its HMAC; it reads no database.
	Issue(ctx context.Context) (identity.PairingCode, error)
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
}

// Presence reports whether the control connection of a host is open (24 section 3.4). Until
// M2-43 builds the control connections, internal/wire binds NoConnections.
type Presence interface {
	// Connected reports an open control connection of hostID.
	Connected(ctx context.Context, hostID ids.ID) bool
}

// NoConnections is the Presence of a server without the route /hosts/connect: no host is
// connected, which is the truth there. M2-43 replaces the binding with its registry.
type NoConnections struct{}

// Connected implements Presence: always false.
func (NoConnections) Connected(context.Context, ids.ID) bool { return false }

// Cursors encodes and checks the opaque cursors of listHosts (implemented by adapters/httpapi
// with platform/httpserver).
type Cursors interface {
	// Encode returns the cursor of key for list and binding.
	Encode(list string, binding []byte, key any) (string, error)
	// Decode checks s against list and binding and unmarshals its key into key. Errors:
	// cursor_invalid.
	Decode(list string, binding []byte, s string, key any) error
}
