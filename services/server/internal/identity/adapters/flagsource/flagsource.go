// SPDX-License-Identifier: Apache-2.0

// Package flagsource is the identity side of the source port instance.FlagUserSource (M1-47):
// putFeatureFlag of instance learns which IDs of a rollout are people through it, because instance
// may not import identity (docs/spec/02-architecture.md 4.4) and identity implements the ports it
// declares (02 4.1 item 4). A rollout holds only people (16 6.2); agents, apps and deleted people
// are refused. internal/wire builds it over the identity repository.
//
// Example:
//
//	src, err := flagsource.New(identityRepo) // passed to instance/app.NewAdmin
package flagsource

import (
	"context"
	"errors"

	"github.com/OWNER/bothy/services/server/internal/identity/domain"
	"github.com/OWNER/bothy/services/server/internal/instance"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// PrincipalReader reads principals by ID; the postgres repository of identity implements it.
type PrincipalReader interface {
	// Principals returns the known principals of list; unknown IDs are missing.
	Principals(ctx context.Context, list []ids.ID) ([]domain.Principal, error)
}

// Source implements instance.FlagUserSource. It is safe for concurrent use.
type Source struct {
	principals PrincipalReader
}

// Compile-time check: Source is the source port of instance.
var _ instance.FlagUserSource = (*Source)(nil)

// New returns the source over principals. Errors: a nil reader.
func New(principals PrincipalReader) (*Source, error) {
	if principals == nil {
		return nil, errors.New("flagsource: New needs the principal reader")
	}
	return &Source{principals: principals}, nil
}

// ExistingUserIDs implements instance.FlagUserSource: the IDs of principalIDs that are people
// with a status other than deleted. Errors: the database error.
func (s *Source) ExistingUserIDs(ctx context.Context, principalIDs []ids.ID) ([]ids.ID, error) {
	list, err := s.principals.Principals(ctx, principalIDs)
	if err != nil {
		return nil, err
	}
	var out []ids.ID
	for _, p := range list {
		if p.Kind == domain.KindUser && p.Status != domain.StatusDeleted {
			out = append(out, p.ID)
		}
	}
	return out, nil
}
