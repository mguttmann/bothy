// SPDX-License-Identifier: Apache-2.0

package identity

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Register declares what the identity module emits (M1-25, docs/spec/05-actions.md section 6):
// the durable events user.updated (M1-31), session.revoked (M1-34) and device.pending (M3-19,
// R-AZ-31: the source of security hints). internal/wire lists it in moduleRegistrations.
func Register(r *authorize.Registry) {
	r.Event("identity", authorize.EventUserUpdated)
	r.Event("identity", authorize.EventSessionRevoked)
	r.Event("identity", authorize.EventDevicePending)
}

// ListUsersQuery asks for one page of the directory (operation listUsers).
type ListUsersQuery struct {
	// Q is a case-insensitive prefix of the handle or the display name; "" for none. It is
	// matched literally ("%" and "_" are no wildcards).
	Q string
	// Status filters by status; "" for every status.
	Status Status
	// Cursor is next_cursor of the previous page, "" for the first page.
	Cursor string
	// Limit is the page size; 0 means api.page_size_default, larger values are capped at
	// api.page_size_max.
	Limit int
}

// UserPage is one page of people ordered by (lower(handle), id); NextCursor is "" on the last page.
type UserPage struct {
	Items      []UserView
	NextCursor string
}

// CreateUserCmd is the input of People.CreateUser (schema UserCreate).
type CreateUserCmd struct {
	// Handle and DisplayName are required.
	Handle, DisplayName string
	// Email, Locale and TimeZone are optional; nil means absent (column default for locale and
	// time zone), a given value is validated like in UpdateUserCmd.
	Email, Locale, TimeZone *string
	// LocalAccount creates the local identity (subject = handle) and a first_login enrollment
	// code with the person (M1-37).
	LocalAccount bool
}

// CreatedUser is the answer of People.CreateUser (schema UserCreated): the person and, with
// local_account, its enrollment code, shown once.
type CreatedUser struct {
	User UserView
	// Enrollment is the first_login code; nil without local_account.
	Enrollment *EnrollmentCode
}

// UpdateUserCmd is the input of People.UpdateUser (schema UserUpdate); nil fields stay unchanged.
type UpdateUserCmd struct {
	DisplayName, Email, Locale, TimeZone *string
}

// ProfileCmd is the input of People.UpdateMyProfile (schema ProfileUpdate); nil fields stay.
type ProfileCmd struct {
	DisplayName, Locale, TimeZone *string
}

// Me is the calling principal (schema Me). Zero values are absent members.
type Me struct {
	// Principal is the principal of the caller.
	Principal PrincipalView
	// IsOwner reports an active owners row of a person.
	IsOwner bool
	// Email, Locale and TimeZone are set for people only; Email "" for none.
	Email, Locale, TimeZone string
	// PermissionVersion is the rights version the request was authorized with.
	PermissionVersion int64
	// SessionID and DeviceID come from the request; zero for none.
	SessionID, DeviceID ids.ID
	// StepUpValidUntil is step_up_at plus auth.step_up_max_age when that lies in the future;
	// zero otherwise.
	StepUpValidUntil time.Time
}

// People is the incoming port of the directory use cases (PLAN M1-31): every method loads its
// resource, calls authorize and runs writes in one unit of work with audit and user.updated
// where 05 lists them. Errors are kernel/errs errors (an invariant violation is an
// *authz.InvariantError); a missing and an invisible person give the same not_found.
type People interface {
	// ListUsers returns one page of people of every status (user.read). Errors: cursor_invalid,
	// forbidden for an agent.
	ListUsers(ctx context.Context, a actor.Actor, q ListUsersQuery) (UserPage, error)
	// GetUser returns the person userID (user.read); an agent, app or unknown id is not_found.
	GetUser(ctx context.Context, a actor.Actor, userID ids.ID) (UserView, error)
	// CreateUser creates an active person (user.create, owner with step-up); with local_account
	// also the local identity and a first_login enrollment code in the same unit of work. Errors:
	// validation_failed with field pointers, conflict on /handle or /email, conflict with reason
	// local_provider_disabled without an enabled local provider.
	CreateUser(ctx context.Context, a actor.Actor, cmd CreateUserCmd) (CreatedUser, error)
	// UpdateUser changes the profile of userID (user.update, owner with step-up); the handle never
	// changes. Errors: as CreateUser, not_found.
	UpdateUser(ctx context.Context, a actor.Actor, userID ids.ID, cmd UpdateUserCmd) (UserView, error)
	// EnableUser moves pending or disabled to active (user.enable, a rights change); an active
	// person is returned unchanged. Errors: invalid_state_transition for deleted, i1_violation.
	EnableUser(ctx context.Context, a actor.Actor, userID ids.ID, reason string) (UserView, error)
	// GetMe returns the calling principal (self.read), also from a pending device.
	GetMe(ctx context.Context, a actor.Actor) (Me, error)
	// UpdateMyProfile changes display name, locale or time zone of the calling person
	// (self.profile.update). Errors: forbidden for an agent or app, validation_failed.
	UpdateMyProfile(ctx context.Context, a actor.Actor, cmd ProfileCmd) (UserView, error)
}

// ResolveI1 is the request member resolve_i1 of owner.add (OpenAPI ResolveI1, M2-21): the agents
// to switch to organization first, in request order, and the reason of the switch. identity/app
// converts it into authz.ResolveI1; the port keeps its own type (02 section 5 rule 1, R-AR-03).
type ResolveI1 struct {
	AgentIDs []ids.ID
	Reason   string
}

// Owners is the incoming port of the owner use cases (PLAN M1-31). Owner changes are rights
// changes: I1 and SR8 hold after every call.
type Owners interface {
	// ListOwners returns the active owners ordered by granted_at (user.read), unpaginated.
	ListOwners(ctx context.Context, a actor.Actor) ([]OwnerView, error)
	// AddOwner grants the owner role to the active person userID (owner.add). resolve is the
	// optional guided flow (05 rule 1.6, M2-21): the listed agents are switched to organization
	// in the same transaction before the owners row is written; nil for none. Errors: not_found,
	// conflict for a person that is not active or already an owner, i1_violation, and for resolve
	// validation_failed, not_found, forbidden or step_up_required of a listed agent.
	AddOwner(ctx context.Context, a actor.Actor, userID ids.ID, reason string, resolve *ResolveI1) (OwnerView, error)
	// RevokeOwner revokes the owner role of userID (owner.remove). Errors: not_found without an
	// active owner row, last_owner.
	RevokeOwner(ctx context.Context, a actor.Actor, userID ids.ID, reason string) error
}
