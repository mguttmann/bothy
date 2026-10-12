// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// The column defaults of users (docs/spec/03-data-model.md section 4.2).
const (
	// DefaultLocale is the default of users.locale.
	DefaultLocale = "de"
	// DefaultTimeZone is the default of users.time_zone.
	DefaultTimeZone = "UTC"
)

// Principal is a row of principals: the shared identity of people, agents and apps.
type Principal struct {
	// ID is the UUIDv7 of the principal.
	ID ids.ID
	// Kind never changes after creation.
	Kind Kind
	// Status is the lifecycle state.
	Status Status
	// Handle is unique without regard to case.
	Handle Handle
	// DisplayName is the shown name.
	DisplayName DisplayName
	// AvatarFileID is the avatar file; the zero ID means none.
	AvatarFileID ids.ID
	// CreatedAt is set by the database on insert.
	CreatedAt time.Time
	// DisabledAt is the time of disabling; zero means not disabled.
	DisabledAt time.Time
	// DeletedAt is the time of deletion; zero means not deleted.
	DeletedAt time.Time
}

// NewPrincipal returns a new active principal. handle and displayName must come from NewHandle
// and NewDisplayName.
//
// Errors: kind internal for a zero id, an unknown kind or a zero handle or display name, because
// those are programming errors of the caller, not input errors.
func NewPrincipal(id ids.ID, kind Kind, handle Handle, displayName DisplayName) (Principal, error) {
	switch {
	case id.IsZero():
		return Principal{}, errs.New(errs.CodeInternalError, "identity: principal without id")
	case !kind.Valid():
		return Principal{}, errs.New(errs.CodeInternalError, "identity: unknown principal kind")
	case handle.IsZero():
		return Principal{}, errs.New(errs.CodeInternalError, "identity: principal without handle")
	case displayName.IsZero():
		return Principal{}, errs.New(errs.CodeInternalError, "identity: principal without display name")
	}
	return Principal{ID: id, Kind: kind, Status: StatusActive, Handle: handle, DisplayName: displayName}, nil
}

// User is a row of users together with its principal.
type User struct {
	// Principal is the principal row; its Kind is KindUser.
	Principal Principal
	// Email is for display only; "" means none. Its validation belongs to the use cases.
	Email string
	// Locale is a BCP 47 tag.
	Locale string
	// TimeZone is an IANA name.
	TimeZone string
	// LastActiveAt is coarse; zero means never.
	LastActiveAt time.Time
}

// NewUser returns the user data of p. An empty locale or time zone becomes the column default
// (DefaultLocale, DefaultTimeZone). Email, locale and time zone are not validated here; the use
// cases that accept them from a request do that (M1-31).
//
// Errors: kind internal when p is not of KindUser (the users trigger would refuse it as well).
func NewUser(p Principal, email, locale, timeZone string) (User, error) {
	if p.Kind != KindUser {
		return User{}, errs.New(errs.CodeInternalError, "identity: users row for a principal that is not a person")
	}
	if locale == "" {
		locale = DefaultLocale
	}
	if timeZone == "" {
		timeZone = DefaultTimeZone
	}
	return User{Principal: p, Email: email, Locale: locale, TimeZone: timeZone}, nil
}

// Owner is a row of owners: the grant of the owner role to a person.
type Owner struct {
	// ID is the UUIDv7 of the row.
	ID ids.ID
	// UserID is the person.
	UserID ids.ID
	// GrantedBy is the granting principal; the zero ID marks the first owner, created by the
	// server command (docs/spec/08-identity.md section 4).
	GrantedBy ids.ID
	// GrantedAt is set by the database on insert.
	GrantedAt time.Time
	// Reason is mandatory and goes into the audit.
	Reason string
	// RevokedAt is the time of revocation; zero means active.
	RevokedAt time.Time
	// RevokedBy is the revoking principal; zero while active.
	RevokedBy ids.ID
}

// NewOwner returns an active owner row for userID. grantedBy is the zero ID only for the first
// owner. reason is mandatory (03 owners: "Pflicht, steht im Audit") and, as the reason of an
// administrative action, a display text under SEC-IN-4.
//
// Errors: validation_failed on /reason with code required or a textcheck rule; kind internal
// for a zero id or userID.
func NewOwner(id, userID, grantedBy ids.ID, reason string) (Owner, error) {
	if id.IsZero() || userID.IsZero() {
		return Owner{}, errs.New(errs.CodeInternalError, "identity: owner row without id or user")
	}
	if err := displayText(pointerReason, "reason", reason); err != nil {
		return Owner{}, err
	}
	return Owner{ID: id, UserID: userID, GrantedBy: grantedBy, Reason: reason}, nil
}

// Active reports whether the owner row is not revoked.
func (o Owner) Active() bool { return o.RevokedAt.IsZero() }
