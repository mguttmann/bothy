// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// The values of groups.source.
const (
	// GroupSourceLocal is a group whose members are managed by hand.
	GroupSourceLocal = "local"
	// GroupSourceOIDCClaim is a group whose members follow a group claim of an OIDC provider
	// (docs/spec/08-identity.md section 3.4, M1-42).
	GroupSourceOIDCClaim = "oidc_claim"
)

// GroupLimits are the configured text limits of a group: group.name_max_chars and
// group.description_max_chars (docs/operations/configuration.md). The use case that creates
// groups (M1-48) binds them; a value of zero or below disables the length check.
type GroupLimits struct {
	NameMaxChars        int
	DescriptionMaxChars int
}

// Group is a row of groups: a named set of people as the subject of grants. Groups are soft
// deleted (DeletedAt), which frees their name.
type Group struct {
	// ID is the UUIDv7 of the group.
	ID ids.ID
	// Name is unique among live groups without regard to case.
	Name string
	// Description may be empty.
	Description string
	// Source is GroupSourceLocal or GroupSourceOIDCClaim.
	Source string
	// ProviderID is the OIDC provider of a claim group; zero for a local group.
	ProviderID ids.ID
	// ExternalRef is the claim value of a claim group; empty for a local group.
	ExternalRef string
	// CreatedBy is the principal who created the group.
	CreatedBy ids.ID
	// CreatedAt is the creation time.
	CreatedAt time.Time
	// DeletedAt is the deletion time; zero means live.
	DeletedAt time.Time
}

// NewGroup returns a new local group. The name must not be blank, both texts must be free of
// control and bidi characters (SEC-IN-4) and within limits. Claim groups (source oidc_claim)
// come from the claim sync of M1-42, not from this constructor.
//
// Errors: validation_failed on /name (required, too_long or a textcheck rule) or /description
// (too_long or a textcheck rule); internal for a zero id, creator or time.
func NewGroup(id ids.ID, name, description string, limits GroupLimits, by ids.ID, at time.Time) (Group, error) {
	switch {
	case id.IsZero():
		return Group{}, internalError("group without id")
	case by.IsZero():
		return Group{}, internalError("group without creator")
	case at.IsZero():
		return Group{}, internalError("group without creation time")
	}
	if err := CheckGroupText(name, description, limits); err != nil {
		return Group{}, err
	}
	return Group{ID: id, Name: name, Description: description, Source: GroupSourceLocal, CreatedBy: by, CreatedAt: at}, nil
}

// NewClaimGroup returns a new group whose members follow the group claim externalRef of the OIDC
// provider providerID (docs/spec/08-identitaet.md section 3.4; the claim sync is M1-42). Name and
// description are checked as in NewGroup; the claim value must not be blank and is free of
// control and bidi characters. Whether the provider exists is held by the foreign key
// groups_provider_id_fkey.
//
// Errors: those of NewGroup; validation_failed on /provider_id (required) or /external_ref
// (required or a textcheck rule).
func NewClaimGroup(id ids.ID, name, description string, limits GroupLimits, providerID ids.ID, externalRef string, by ids.ID, at time.Time) (Group, error) {
	g, err := NewGroup(id, name, description, limits, by, at)
	if err != nil {
		return Group{}, err
	}
	if err := CheckClaim(providerID, externalRef); err != nil {
		return Group{}, err
	}
	g.Source, g.ProviderID, g.ExternalRef = GroupSourceOIDCClaim, providerID, externalRef
	return g, nil
}

// CheckClaim checks the binding of a claim group: a provider and a claim value that is not blank
// and free of control and bidi characters. Errors: validation_failed on /provider_id or
// /external_ref.
func CheckClaim(providerID ids.ID, externalRef string) error {
	if providerID.IsZero() {
		return fieldError(pointerProviderID, fieldRequired, "a claim group needs the provider of its claim")
	}
	return checkText(pointerExternalRef, "claim value", externalRef, true, 0)
}

// CheckGroupText checks name and description of a group as NewGroup does: the name not blank,
// both free of control and bidi characters (SEC-IN-4) and within limits. group.update (M1-29)
// checks a rename with it. Errors: validation_failed on /name or /description.
func CheckGroupText(name, description string, limits GroupLimits) error {
	if err := checkText(pointerName, "group name", name, true, limits.NameMaxChars); err != nil {
		return err
	}
	return checkText(pointerDescription, "group description", description, false, limits.DescriptionMaxChars)
}

// Membership is a row of group_memberships: one person in one group, with history. Removal sets
// RemovedAt; the row stays.
type Membership struct {
	// ID is the UUIDv7 of the membership.
	ID ids.ID
	// GroupID is the group.
	GroupID ids.ID
	// UserID is the person (users.principal_id; groups contain only people, R-DM-07).
	UserID ids.ID
	// AddedBy is the principal who added the person.
	AddedBy ids.ID
	// AddedAt is the time of adding.
	AddedAt time.Time
	// ExpiresAt is the end of the membership; zero means no expiry.
	ExpiresAt time.Time
	// ExpiryRemindedAt is when the reminder before expiry was written; zero means not reminded.
	ExpiryRemindedAt time.Time
	// ExpiryReportedAt is when the expiry was reported; zero means not reported.
	ExpiryReportedAt time.Time
	// RemovedAt is the removal time; zero means not removed.
	RemovedAt time.Time
	// RemovedBy is the principal who removed the person.
	RemovedBy ids.ID
}

// MembershipWithGroup pairs a membership with the row of its group, read together so that
// validity (Membership.InEffect) sees the group's DeletedAt.
type MembershipWithGroup struct {
	Membership Membership
	Group      Group
}

// NewMembership returns a new membership of userID in groupID. Whether userID is a person is
// held by the foreign key to users (R-DM-07); whether the group is local and live is for the
// use case (M1-48) to check.
//
// Errors: validation_failed on /expires_at when expiresAt is set and not after at; internal for
// a zero id, group, user, adder or time.
func NewMembership(id, groupID, userID, addedBy ids.ID, expiresAt, at time.Time) (Membership, error) {
	switch {
	case id.IsZero(), groupID.IsZero(), userID.IsZero(), addedBy.IsZero():
		return Membership{}, internalError("membership with a zero id")
	case at.IsZero():
		return Membership{}, internalError("membership without time")
	}
	if err := checkExpiry(expiresAt, at); err != nil {
		return Membership{}, err
	}
	return Membership{ID: id, GroupID: groupID, UserID: userID, AddedBy: addedBy, AddedAt: at, ExpiresAt: expiresAt}, nil
}
