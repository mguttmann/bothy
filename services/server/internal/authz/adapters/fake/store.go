// SPDX-License-Identifier: Apache-2.0

package fake

import (
	"context"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/OWNER/bothy/services/server/internal/authz/app"
	"github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Store is an in-memory app.Store. Reads counts the calls of ActiveGrantsOfPrincipal, so a test
// sees whether a snapshot was rebuilt.
type Store struct {
	failer
	mu          sync.Mutex
	groups      map[ids.ID]domain.Group
	memberships map[ids.ID]domain.Membership
	grants      map[ids.ID]domain.Grant
	reads       atomic.Int64
}

// Compile-time check.
var _ app.Store = (*Store)(nil)

// NewStore returns an empty store.
func NewStore() *Store {
	return &Store{groups: map[ids.ID]domain.Group{}, memberships: map[ids.ID]domain.Membership{}, grants: map[ids.ID]domain.Grant{}}
}

// Reads returns how often ActiveGrantsOfPrincipal ran.
func (s *Store) Reads() int64 { return s.reads.Load() }

// InsertGroup implements app.Store.
func (s *Store) InsertGroup(_ context.Context, g domain.Group) error {
	if err := s.err(); err != nil {
		return err
	}
	s.mu.Lock()
	s.groups[g.ID] = g
	s.mu.Unlock()
	return nil
}

// GetGroup implements app.Store.
func (s *Store) GetGroup(_ context.Context, id ids.ID) (domain.Group, bool, error) {
	if err := s.err(); err != nil {
		return domain.Group{}, false, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	g, ok := s.groups[id]
	return g, ok, nil
}

// UpdateGroup implements app.Store; it does not check the uniqueness of names.
func (s *Store) UpdateGroup(_ context.Context, id ids.ID, name, description string) (bool, error) {
	if err := s.err(); err != nil {
		return false, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	g, ok := s.groups[id]
	if !ok || !g.DeletedAt.IsZero() {
		return false, nil
	}
	g.Name, g.Description = name, description
	s.groups[id] = g
	return true, nil
}

// SoftDeleteGroup implements app.Store.
func (s *Store) SoftDeleteGroup(_ context.Context, id ids.ID, at time.Time) (bool, error) {
	if err := s.err(); err != nil {
		return false, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	g, ok := s.groups[id]
	if !ok || !g.DeletedAt.IsZero() {
		return false, nil
	}
	g.DeletedAt = at
	s.groups[id] = g
	return true, nil
}

// InsertMembership implements app.Store.
func (s *Store) InsertMembership(_ context.Context, m domain.Membership) error {
	if err := s.err(); err != nil {
		return err
	}
	s.mu.Lock()
	s.memberships[m.ID] = m
	s.mu.Unlock()
	return nil
}

// RemoveMembership implements app.Store.
func (s *Store) RemoveMembership(_ context.Context, id, by ids.ID, at time.Time) (bool, error) {
	if err := s.err(); err != nil {
		return false, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	m, ok := s.memberships[id]
	if !ok || !m.RemovedAt.IsZero() {
		return false, nil
	}
	m.RemovedAt, m.RemovedBy = at, by
	s.memberships[id] = m
	return true, nil
}

// ActiveMembershipsOfUser implements app.Store.
func (s *Store) ActiveMembershipsOfUser(_ context.Context, userID ids.ID) ([]domain.MembershipWithGroup, error) {
	if err := s.err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []domain.MembershipWithGroup
	for _, m := range s.memberships {
		if m.UserID == userID && m.RemovedAt.IsZero() {
			out = append(out, domain.MembershipWithGroup{Membership: m, Group: s.groups[m.GroupID]})
		}
	}
	slices.SortFunc(out, func(a, b domain.MembershipWithGroup) int {
		return slices.Compare(a.Membership.ID[:], b.Membership.ID[:])
	})
	return out, nil
}

// ActiveMembershipsOfGroups implements app.Store.
func (s *Store) ActiveMembershipsOfGroups(_ context.Context, groupIDs []ids.ID) ([]domain.MembershipWithGroup, error) {
	if err := s.err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []domain.MembershipWithGroup
	for _, m := range s.memberships {
		if slices.Contains(groupIDs, m.GroupID) && m.RemovedAt.IsZero() {
			out = append(out, domain.MembershipWithGroup{Membership: m, Group: s.groups[m.GroupID]})
		}
	}
	slices.SortFunc(out, func(a, b domain.MembershipWithGroup) int {
		return slices.Compare(a.Membership.ID[:], b.Membership.ID[:])
	})
	return out, nil
}

// MembershipsOfUserInGroup implements app.Store.
func (s *Store) MembershipsOfUserInGroup(_ context.Context, groupID, userID ids.ID) ([]domain.Membership, error) {
	if err := s.err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []domain.Membership
	for _, m := range s.memberships {
		if m.GroupID == groupID && m.UserID == userID {
			out = append(out, m)
		}
	}
	slices.SortFunc(out, func(a, b domain.Membership) int { return slices.Compare(a.ID[:], b.ID[:]) })
	return out, nil
}

// LockClaimMembershipsOfUser implements app.Store (no locks in memory).
func (s *Store) LockClaimMembershipsOfUser(ctx context.Context, userID, providerID ids.ID) ([]domain.MembershipWithGroup, error) {
	all, err := s.ActiveMembershipsOfUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	return slices.DeleteFunc(all, func(m domain.MembershipWithGroup) bool {
		return m.Group.Source != domain.GroupSourceOIDCClaim || m.Group.ProviderID != providerID || !m.Group.DeletedAt.IsZero()
	}), nil
}

// ClaimGroupsByRefs implements app.Store.
func (s *Store) ClaimGroupsByRefs(_ context.Context, providerID ids.ID, refs []string) ([]domain.Group, error) {
	if err := s.err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []domain.Group
	for _, g := range s.groups {
		if g.Source == domain.GroupSourceOIDCClaim && g.ProviderID == providerID && g.DeletedAt.IsZero() && slices.Contains(refs, g.ExternalRef) {
			out = append(out, g)
		}
	}
	slices.SortFunc(out, func(a, b domain.Group) int { return slices.Compare(a.ID[:], b.ID[:]) })
	return out, nil
}

// ExtendMembership implements app.Store.
func (s *Store) ExtendMembership(_ context.Context, id ids.ID, expiresAt, _ time.Time) (bool, error) {
	if err := s.err(); err != nil {
		return false, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	m, ok := s.memberships[id]
	if !ok || !m.RemovedAt.IsZero() {
		return false, nil
	}
	m.ExpiresAt, m.ExpiryRemindedAt, m.ExpiryReportedAt = expiresAt, time.Time{}, time.Time{}
	s.memberships[id] = m
	return true, nil
}

// ActiveTriggerGrants implements app.Store.
func (s *Store) ActiveTriggerGrants(_ context.Context) ([]domain.Grant, error) {
	if err := s.err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.active(func(g domain.Grant) bool { return g.Level == domain.LevelTrigger }), nil
}

// ActiveGrantsOnTargets implements app.Store.
func (s *Store) ActiveGrantsOnTargets(_ context.Context, spaceIDs, channelIDs []ids.ID) ([]domain.Grant, error) {
	if err := s.err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.active(func(g domain.Grant) bool {
		switch g.Target.Kind {
		case domain.TargetInstance:
			return true
		case domain.TargetSpace:
			return slices.Contains(spaceIDs, g.Target.SpaceID)
		case domain.TargetChannel:
			return slices.Contains(channelIDs, g.Target.ChannelID)
		case domain.TargetHost:
			return false
		}
		return false
	}), nil
}

// ActiveGrantsOnHosts implements app.Store.
func (s *Store) ActiveGrantsOnHosts(_ context.Context, hostIDs []ids.ID) ([]domain.Grant, error) {
	if err := s.err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.active(func(g domain.Grant) bool {
		return g.Target.Kind == domain.TargetHost && slices.Contains(hostIDs, g.Target.HostID)
	}), nil
}

// InsertGrant implements app.Store.
func (s *Store) InsertGrant(_ context.Context, g domain.Grant) error {
	if err := s.err(); err != nil {
		return err
	}
	s.mu.Lock()
	s.grants[g.ID] = g
	s.mu.Unlock()
	return nil
}

// GetGrant implements app.Store.
func (s *Store) GetGrant(_ context.Context, id ids.ID) (domain.Grant, bool, error) {
	if err := s.err(); err != nil {
		return domain.Grant{}, false, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	g, ok := s.grants[id]
	return g, ok, nil
}

// RevokeGrant implements app.Store.
func (s *Store) RevokeGrant(_ context.Context, id, by ids.ID, reason string, at time.Time) (domain.Grant, bool, error) {
	if err := s.err(); err != nil {
		return domain.Grant{}, false, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	g, ok := s.grants[id]
	if !ok || !g.RevokedAt.IsZero() {
		return domain.Grant{}, false, nil
	}
	g.RevokedAt, g.RevokedBy, g.RevokeReason = at, by, reason
	s.grants[id] = g
	return g, true, nil
}

func (s *Store) active(match func(domain.Grant) bool) []domain.Grant {
	var out []domain.Grant
	for _, g := range s.grants {
		if g.RevokedAt.IsZero() && match(g) {
			out = append(out, g)
		}
	}
	slices.SortFunc(out, func(a, b domain.Grant) int { return slices.Compare(a.ID[:], b.ID[:]) })
	return out
}

// ActiveGrantsOfPrincipal implements app.Store and counts the call.
func (s *Store) ActiveGrantsOfPrincipal(_ context.Context, principalID ids.ID) ([]domain.Grant, error) {
	s.reads.Add(1)
	if err := s.err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.active(func(g domain.Grant) bool {
		return g.Subject.Kind != domain.SubjectGroup && g.Subject.PrincipalID == principalID
	}), nil
}

// ActiveGrantsOfGroups implements app.Store.
func (s *Store) ActiveGrantsOfGroups(_ context.Context, groupIDs []ids.ID) ([]domain.Grant, error) {
	if err := s.err(); err != nil {
		return nil, err
	}
	if len(groupIDs) == 0 {
		return nil, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.active(func(g domain.Grant) bool {
		return g.Subject.Kind == domain.SubjectGroup && slices.Contains(groupIDs, g.Subject.GroupID)
	}), nil
}

// Versions is an in-memory instance.Versions. Reads counts the reads.
type Versions struct {
	failer
	v     atomic.Int64
	reads atomic.Int64
}

// Set sets the current rights version.
func (v *Versions) Set(n int64) { v.v.Store(n) }

// Bump raises the rights version by one and returns it.
func (v *Versions) Bump() int64 { return v.v.Add(1) }

// Reads returns how often PermissionVersion ran.
func (v *Versions) Reads() int64 { return v.reads.Load() }

// PermissionVersion implements instance.Versions.
func (v *Versions) PermissionVersion(_ context.Context) (int64, error) {
	v.reads.Add(1)
	if err := v.err(); err != nil {
		return 0, err
	}
	return v.v.Load(), nil
}

// EventSeq implements instance.Versions; authz never reads it, so the fake answers 0.
func (v *Versions) EventSeq(_ context.Context) (int64, error) { return 0, nil }

// ListLiveGroups implements app.Store.
func (s *Store) ListLiveGroups(_ context.Context, after, memberID ids.ID, t time.Time, limit int) ([]domain.Group, error) {
	if err := s.err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []domain.Group
	for _, g := range s.groups {
		if !g.DeletedAt.IsZero() || compareIDs(g.ID, after) <= 0 || (!memberID.IsZero() && !s.memberAt(g, memberID, t)) {
			continue
		}
		out = append(out, g)
	}
	slices.SortFunc(out, func(a, b domain.Group) int { return compareIDs(a.ID, b.ID) })
	return out[:min(len(out), max(limit, 0))], nil
}

// memberAt reports a membership of user in g in effect at t; s.mu is held.
func (s *Store) memberAt(g domain.Group, user ids.ID, t time.Time) bool {
	for _, m := range s.memberships {
		if m.GroupID == g.ID && m.UserID == user && m.InEffect(g, t) {
			return true
		}
	}
	return false
}

// CountMembersInEffect implements app.Store.
func (s *Store) CountMembersInEffect(_ context.Context, groupIDs []ids.ID, t time.Time) (map[ids.ID]int, error) {
	if err := s.err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	out := map[ids.ID]int{}
	for _, m := range s.memberships {
		if slices.Contains(groupIDs, m.GroupID) && m.RemovedAt.IsZero() && (m.ExpiresAt.IsZero() || m.ExpiresAt.After(t)) {
			out[m.GroupID]++
		}
	}
	return out, nil
}

// ListMembersInEffect implements app.Store.
func (s *Store) ListMembersInEffect(_ context.Context, groupID, after ids.ID, t time.Time, limit int) ([]domain.Membership, error) {
	if err := s.err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []domain.Membership
	for _, m := range s.memberships {
		if m.GroupID == groupID && compareIDs(m.ID, after) > 0 && m.RemovedAt.IsZero() && (m.ExpiresAt.IsZero() || m.ExpiresAt.After(t)) {
			out = append(out, m)
		}
	}
	slices.SortFunc(out, func(a, b domain.Membership) int { return compareIDs(a.ID, b.ID) })
	return out[:min(len(out), max(limit, 0))], nil
}

// ListGrants implements app.Store with the semantics of the PostgreSQL adapter.
func (s *Store) ListGrants(_ context.Context, f app.GrantFilter, after ids.ID, t time.Time, limit int) ([]domain.Grant, error) {
	if err := s.err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []domain.Grant
	for _, g := range s.grants {
		if compareIDs(g.ID, after) > 0 && grantMatches(f, g, t) {
			out = append(out, g)
		}
	}
	slices.SortFunc(out, func(a, b domain.Grant) int { return compareIDs(a.ID, b.ID) })
	return out[:min(len(out), max(limit, 0))], nil
}

// grantMatches applies f to g at t.
func grantMatches(f app.GrantFilter, g domain.Grant, t time.Time) bool {
	switch {
	case !f.IncludeRevoked && !g.InEffect(t):
		return false
	case f.TargetKind != "" && g.Target.Kind != f.TargetKind:
		return false
	case !f.TargetID.IsZero() && f.TargetID != g.Target.SpaceID && f.TargetID != g.Target.ChannelID && f.TargetID != g.Target.HostID:
		return false
	case f.SubjectKind != "" && g.Subject.Kind != f.SubjectKind:
		return false
	case !f.SubjectID.IsZero() && f.SubjectID != g.Subject.PrincipalID && f.SubjectID != g.Subject.GroupID:
		return false
	}
	if f.AllTargets {
		return true
	}
	return (g.Target.Kind == domain.TargetSpace && slices.Contains(f.ManageSpaces, g.Target.SpaceID)) ||
		(g.Target.Kind == domain.TargetChannel && slices.Contains(f.ManageChannels, g.Target.ChannelID))
}
