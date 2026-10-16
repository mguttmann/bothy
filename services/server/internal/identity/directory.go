// SPDX-License-Identifier: Apache-2.0

package identity

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Status is the lifecycle state of a principal (column principals.status, schema UserStatus of
// docs/api/openapi.yaml).
type Status string

const (
	// StatusPending is a provisioned person waiting for approval.
	StatusPending Status = "pending"
	// StatusActive is the normal state.
	StatusActive Status = "active"
	// StatusDisabled is a disabled principal.
	StatusDisabled Status = "disabled"
	// StatusDeleted is a deleted principal whose row stays for authorship.
	StatusDeleted Status = "deleted"
)

// PrincipalView is the read-only view of a principal for other modules (fields of
// PrincipalRef in docs/api/openapi.yaml plus status and creation time).
type PrincipalView struct {
	// ID is the principal.
	ID ids.ID
	// Kind is actor.KindUser, actor.KindAgent or actor.KindApp.
	Kind actor.Kind
	// Status is the lifecycle state.
	Status Status
	// Handle is the lower-case handle for mentions.
	Handle string
	// DisplayName is the shown name.
	DisplayName string
	// AvatarFileID is the avatar file; the zero ID means no avatar.
	AvatarFileID ids.ID
	// CreatedAt is the creation time.
	CreatedAt time.Time
}

// UserView is the read-only view of a person (schema User of docs/api/openapi.yaml).
type UserView struct {
	PrincipalView
	// Email is for display only; "" means none.
	Email string
	// Locale is a BCP 47 tag.
	Locale string
	// TimeZone is an IANA name.
	TimeZone string
	// IsOwner reports an active owners row (revoked_at IS NULL).
	IsOwner bool
}

// OwnerView is one active owner.
type OwnerView struct {
	// User is the person holding the owner role.
	User PrincipalView
	// GrantedBy is the granting principal; the zero ID marks the first owner, created by the
	// server command.
	GrantedBy ids.ID
	// GrantedAt is the time of the grant.
	GrantedAt time.Time
	// Reason is the reason given at the grant.
	Reason string
}

// Directory is the read-only port of identity for other modules. It does not call authorize:
// callers authorize their own actions. Reads join the ambient transaction of ctx when there is
// one. No error carries SQL text or values.
type Directory interface {
	// Principal returns the principal id. Errors: kind not_found for an unknown id.
	Principal(ctx context.Context, id ids.ID) (PrincipalView, error)
	// Principals returns the known principals among ids, keyed by id; unknown ids are absent.
	Principals(ctx context.Context, ids []ids.ID) (map[ids.ID]PrincipalView, error)
	// PrincipalByHandle returns the principal whose handle equals handle without regard to
	// case. Errors: kind not_found when there is none.
	PrincipalByHandle(ctx context.Context, handle string) (PrincipalView, error)
	// ActiveByHandles returns the active people and agents whose handle equals one of handles
	// without regard to case, keyed by the lower-case handle, in one read (the mention lookup of
	// messaging, M1-56); apps, other states and unknown handles are absent.
	ActiveByHandles(ctx context.Context, handles []string) (map[string]PrincipalView, error)
	// User returns the person id. Errors: kind not_found for an unknown id and for a principal
	// that is not a person.
	User(ctx context.Context, id ids.ID) (UserView, error)
	// Users returns the people among ids in one read, keyed by id (M3-18: notify loads the time
	// zones of the quiet hours of many people); unknown ids and principals that are not people
	// are absent, like Principals.
	Users(ctx context.Context, ids []ids.ID) (map[ids.ID]UserView, error)
	// ActiveOwners returns the owners with revoked_at IS NULL, ordered by granted_at, then id.
	ActiveOwners(ctx context.Context) ([]OwnerView, error)
	// SearchLanguage returns the text search configuration of a new message or thread by the
	// principal id (docs/spec/14-suche.md 3.1): for a person search.locale_map of the first
	// language subtag of users.locale, for an agent or an app the same of locale.default, simple
	// when the map has no entry. Errors: kind not_found for an unknown id; internal when the
	// directory has no language configuration.
	SearchLanguage(ctx context.Context, id ids.ID) (string, error)
	// InstanceSearchLanguage returns the text search configuration of locale.default through
	// search.locale_map (simple without entry): the language of system lines, imports and file
	// text (14 3.1). Errors: the database error; internal when the directory has no language
	// configuration.
	InstanceSearchLanguage(ctx context.Context) (string, error)
}
