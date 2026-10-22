// SPDX-License-Identifier: Apache-2.0

package identity

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/netip"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// ClientComponent is auth_sessions.client_component (docs/spec/03-data-model.md section 4.2).
type ClientComponent string

// The client components of a session.
const (
	// ComponentApp is the session of an app; the login tickets start it.
	ComponentApp ClientComponent = "app"
	// ComponentShareExt is the child session of the share extension (08 section 7.4).
	ComponentShareExt ClientComponent = "share_ext"
	// ComponentConsole is the cookie session of the web console (M6-01).
	ComponentConsole ClientComponent = "console"
)

// AuthMethod is auth_sessions.auth_method, the login path of a session and the only member of the
// amr claim.
type AuthMethod string

// The six login paths of 03 auth_sessions.auth_method.
const (
	AuthOIDC           AuthMethod = "oidc"
	AuthTrustedHeader  AuthMethod = "trusted_header"
	AuthPasskey        AuthMethod = "passkey"
	AuthPasswordTOTP   AuthMethod = "password_totp"
	AuthEnrollmentCode AuthMethod = "enrollment_code"
	AuthDevicePairing  AuthMethod = "device_pairing"
)

// Valid reports whether m is one of the six login paths.
func (m AuthMethod) Valid() bool {
	switch m {
	case AuthOIDC, AuthTrustedHeader, AuthPasskey, AuthPasswordTOTP, AuthEnrollmentCode, AuthDevicePairing:
		return true
	}
	return false
}

// SessionRevokeReason is auth_sessions.revoke_reason and the reason of the event session.revoked
// (schema SessionRevokedPayload).
type SessionRevokeReason string

// The revocation reasons of 03 auth_sessions.revoke_reason.
const (
	// RevokeLogout: the person ended the own session.
	RevokeLogout SessionRevokeReason = "logout"
	// RevokeAdmin: someone else ended it (session.revoke, M1-45).
	RevokeAdmin SessionRevokeReason = "admin"
	// RevokeDeviceRevoked: the device was revoked (M1-45).
	RevokeDeviceRevoked SessionRevokeReason = "device_revoked"
	// RevokeRefreshReuse: a rotated refresh token was used again (08 section 7.4).
	RevokeRefreshReuse SessionRevokeReason = "refresh_reuse"
	// RevokeUserDisabled: the person was disabled (M1-45).
	RevokeUserDisabled SessionRevokeReason = "user_disabled"
	// RevokeExpired: the session reached its end.
	RevokeExpired SessionRevokeReason = "expired"
	// RevokeInactive: the device was inactive too long.
	RevokeInactive SessionRevokeReason = "inactive"
	// RevokeStepUpFailed: too many failed step-ups (M1-35).
	RevokeStepUpFailed SessionRevokeReason = "step_up_failed"
	// RevokeTrustedHeaderMismatch: the header login of a later request differed (M1-39).
	RevokeTrustedHeaderMismatch SessionRevokeReason = "trusted_header_mismatch"
)

// Valid reports whether r is one of the nine reasons.
func (r SessionRevokeReason) Valid() bool {
	switch r {
	case RevokeLogout, RevokeAdmin, RevokeDeviceRevoked, RevokeRefreshReuse, RevokeUserDisabled, RevokeExpired,
		RevokeInactive, RevokeStepUpFailed, RevokeTrustedHeaderMismatch:
		return true
	}
	return false
}

// redacted is the only rendering of a Token.
const redacted = "[redacted]"

// Token is a secret token value (access or refresh token) that renders as "[redacted]" through
// fmt, slog, encoding/json and encoding.TextMarshaler, so it never reaches a log, an event, an
// audit entry or an error by accident (SEC-LOG-1, SEC-SEC-3). Only Reveal hands out the value; the
// REST handler calls it to write the response body once.
type Token struct {
	v *string
}

// NewToken wraps v.
func NewToken(v string) Token { return Token{v: &v} }

// Reveal returns the value; "" for the zero Token.
func (t Token) Reveal() string {
	if t.v == nil {
		return ""
	}
	return *t.v
}

// IsZero reports whether t holds no value.
func (t Token) IsZero() bool { return t.v == nil }

// String returns "[redacted]".
func (Token) String() string { return redacted }

// GoString returns "[redacted]".
func (Token) GoString() string { return redacted }

// Format writes "[redacted]" for every verb.
func (Token) Format(f fmt.State, _ rune) { _, _ = io.WriteString(f, redacted) }

// LogValue implements slog.LogValuer.
func (Token) LogValue() slog.Value { return slog.StringValue(redacted) }

// MarshalJSON returns the JSON string "[redacted]".
func (Token) MarshalJSON() ([]byte, error) { return []byte(`"` + redacted + `"`), nil }

// MarshalText returns "[redacted]".
func (Token) MarshalText() ([]byte, error) { return []byte(redacted), nil }

// StartSession is the input of Auth.StartSession: a new app session of a person on one of its
// devices, after the login ticket checked the login (M1-37 to M1-43).
type StartSession struct {
	// UserID is the person.
	UserID ids.ID
	// DeviceID is a device of the person that is not revoked; pending is allowed (08 section 5.2).
	DeviceID ids.ID
	// AuthMethod is the login path.
	AuthMethod AuthMethod
	// IdentityID is the external identity used for the login; nil for none.
	IdentityID *ids.ID
	// ClientIP is the client address of the login request, stored as auth_sessions.last_ip; the
	// zero Addr for none (M1-37).
	ClientIP netip.Addr
}

// Session is a session as the API shows it (schema Session). Zero values are absent members;
// Current is set by the handler for the session of the request.
type Session struct {
	ID, UserID, DeviceID ids.ID
	AuthMethod           AuthMethod
	ClientComponent      ClientComponent
	// ParentSessionID is the parent of a child session; zero otherwise (not in the schema).
	ParentSessionID ids.ID
	CreatedAt       time.Time
	ExpiresAt       time.Time
	// StepUpAt and StepUpMethod describe the last step-up; zero and "" for none.
	StepUpAt     time.Time
	StepUpMethod string
	// LastSeenAt is zero for none.
	LastSeenAt time.Time
	// Current reports the session of the request (REST only).
	Current bool
}

// SessionPage is one page of sessions, newest first; NextCursor is "" on the last page.
type SessionPage struct {
	Items      []Session
	NextCursor string
}

// TokenSet is the result of a login, a refresh or a child session (schema TokenResponse).
type TokenSet struct {
	// AccessToken is the JWS of 08 section 7.2; RefreshToken the 256-bit refresh token.
	AccessToken, RefreshToken Token
	// ExpiresIn is the number of seconds until the access token expires.
	ExpiresIn int
	// RefreshExpiresAt is the end of the refresh token, never after the session end.
	RefreshExpiresAt time.Time
	// Session is the session the tokens belong to.
	Session Session
	// Device is the device of the session.
	Device Device
}

// SessionEnded is the in-transaction event of every revocation (docs/spec/02-architecture.md
// section 4.3): the sessions of one device that ended in one step of a unit of work, their
// person, device and reason. A use case that ends sessions on several devices (user.disable)
// publishes one event per device, so every ended session is in exactly one event. notify
// subscribes from M3 on (push registrations of the sessions).
type SessionEnded struct {
	// SessionIDs are the revoked sessions, ascending.
	SessionIDs []ids.ID
	// UserID is their person; DeviceID their device.
	UserID, DeviceID ids.ID
	// Reason is the revocation reason of the sessions.
	Reason SessionRevokeReason
	// At is auth_sessions.revoked_at of the sessions (M1-45).
	At time.Time
}

// Auth is the session port of identity (PLAN M1-34; 08 section 7). Every error is a kernel/errs
// error without tokens, hashes or SQL.
type Auth interface {
	// StartSession starts an app session with its refresh family and first token pair. It joins
	// the ambient unit of work of ctx, or runs its own, and authorizes nothing: the calling login
	// ticket authorized and audits auth.login.complete. Errors: validation_failed for an unknown
	// auth method; not_found for a device that is not the person's or is revoked, or has no active
	// dpop key.
	StartSession(ctx context.Context, s StartSession) (TokenSet, error)
	// StartChildSession starts the child session of component for the active app session
	// parentSessionID, with its own family on the parent family's key, and revokes an active
	// child of the same component (logout). It joins the ambient unit of work and authorizes
	// nothing. Errors: validation_failed for a component other than share_ext; not_found for a
	// parent that is not an active app session.
	StartChildSession(ctx context.Context, parentSessionID ids.ID, component ClientComponent) (TokenSet, error)
	// CreateExtensionSession is the use case of session.extension.create for a (the caller's
	// session is the parent): authorize, StartChildSession and the audit entry in one unit of
	// work.
	CreateExtensionSession(ctx context.Context, a actor.Actor, component ClientComponent) (TokenSet, error)
	// Refresh rotates refreshToken under the verified DPoP proof (auth.token.refresh). Errors:
	// unauthenticated for an unknown or expired token, invalid_dpop_proof for a proof of another
	// key, session_revoked for a revoked family, session, device or person and after a reuse
	// (whose revocation commits first).
	Refresh(ctx context.Context, refreshToken string, proof VerifiedProof) (TokenSet, error)
	// Logout ends the session of a with its children and families (auth.logout). Errors: the
	// decision of authorize (not_found for an actor without a session).
	Logout(ctx context.Context, a actor.Actor) error
	// RevokeSession revokes sessionID and its children with reason, by the principal by (nil for
	// none), revokes their active families, appends session.revoked per session and publishes
	// SessionEnded. Already revoked sessions stay unchanged. It joins the ambient unit of work and
	// authorizes nothing (M1-35 and M1-45 call it after their own authorization). Errors:
	// validation_failed for an unknown reason; not_found for an unknown session.
	RevokeSession(ctx context.Context, sessionID ids.ID, reason SessionRevokeReason, by *ids.ID) error
	// ListSessions returns the active sessions of userID for a (session.list, self or owner),
	// children included, newest first. Errors: not_found for another person or an unknown one,
	// device_pending from a pending device, cursor_invalid.
	ListSessions(ctx context.Context, a actor.Actor, userID ids.ID, p Page) (SessionPage, error)
	// SessionActor builds the actor of a download ticket of userID issued in sessionID (M3-05
	// design note 6): kind user with the session, its device, device state, client component and
	// auth method, read per call so a revocation takes effect on the next fetch (04 section 8.5).
	// SessionActive is true only for a session of userID that is not revoked and has not ended;
	// DeviceKeyActive only when the device's active dpop key is the key of the session's active
	// refresh family; a pending device past auth.device_pending_ttl is revoked; there is no
	// step-up time and no key thumbprint. It never rejects on these flags, so authorize
	// (check_active) refuses and audits the reason: an unknown session or one of another person
	// gives an actor without session, device or key (DeviceState unknown). Errors: only store
	// failures. Example: a, err := auth.SessionActor(ctx, ticket.PrincipalID, ticket.SessionID).
	SessionActor(ctx context.Context, userID, sessionID ids.ID) (actor.Actor, error)
}
