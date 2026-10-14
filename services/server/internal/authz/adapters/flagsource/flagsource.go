// SPDX-License-Identifier: Apache-2.0

// Package flagsource is the authz side of the source port instance.FlagGroupSource (M1-47): the
// feature flag evaluation of instance learns the valid groups of a person, and putFeatureFlag the
// existing groups, through it, because instance may not import authz
// (docs/spec/02-architektur.md 4.4) and authz implements the ports it declares (02 4.1 item 4).
// Validity is the one rule of M1-27 (domain.Membership.InEffect): a membership with
// expires_at = t is already invalid at t (07 2.5), and a deleted group has no valid member.
// internal/wire builds it over the authz repository.
//
// Example:
//
//	src, err := flagsource.New(authzRepo) // passed to instance/app.NewAdmin
package flagsource

import (
	"context"
	"errors"
	"slices"
	"time"

	"github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/instance"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Store reads memberships and groups; the postgres repository of authz implements it.
type Store interface {
	// ActiveMembershipsOfUser returns the memberships of userID that are not removed, each with its
	// group.
	ActiveMembershipsOfUser(ctx context.Context, userID ids.ID) ([]domain.MembershipWithGroup, error)
	// GetGroup reads a group, also a deleted one; found is false for an unknown ID.
	GetGroup(ctx context.Context, id ids.ID) (domain.Group, bool, error)
}

// Source implements instance.FlagGroupSource. It is safe for concurrent use.
type Source struct {
	store Store
}

// Compile-time check: Source is the source port of instance.
var _ instance.FlagGroupSource = (*Source)(nil)

// New returns the source over store. Errors: a nil store.
func New(store Store) (*Source, error) {
	if store == nil {
		return nil, errors.New("flagsource: New needs the authz store")
	}
	return &Source{store: store}, nil
}

// ValidGroupIDs implements instance.FlagGroupSource: the groups in which userID has a membership
// in effect at at. Errors: the database error.
func (s *Source) ValidGroupIDs(ctx context.Context, userID ids.ID, at time.Time) ([]ids.ID, error) {
	ms, err := s.store.ActiveMembershipsOfUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	var out []ids.ID
	for _, m := range ms {
		if m.Membership.UserID == userID && m.Membership.InEffect(m.Group, at) && !slices.Contains(out, m.Group.ID) {
			out = append(out, m.Group.ID)
		}
	}
	return out, nil
}

// ExistingGroupIDs implements instance.FlagGroupSource: the IDs of groupIDs that name a group that
// is not deleted, each once. A rollout is entered by hand and stays short, so one read per ID is
// enough. Errors: the database error.
func (s *Source) ExistingGroupIDs(ctx context.Context, groupIDs []ids.ID) ([]ids.ID, error) {
	var out []ids.ID
	for _, id := range groupIDs {
		if slices.Contains(out, id) {
			continue
		}
		g, found, err := s.store.GetGroup(ctx, id)
		if err != nil {
			return nil, err
		}
		if found && g.DeletedAt.IsZero() {
			out = append(out, id)
		}
	}
	return out, nil
}
