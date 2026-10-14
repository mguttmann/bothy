// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"slices"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// The audit attribute names of group.member.sync (ids only: claim values can be names and never
// reach the audit log, brief M1-42).
const (
	attrProviderID = "provider_id"
	attrAdded      = "added"
	attrRemoved    = "removed"
	attrExtended   = "extended"
	attrGroupIDs   = "group_ids"
	attrAgentIDs   = "agent_ids"
)

// claimSyncState is what a claim sync tells the rights check of its unit (BeforeCommit): the
// person as the actor of the events and the deny entry, the resource of the deny entry (the unit
// is declared before the person is known), the details the deny entry adds and the hint of the
// refusal. Its presence in the unit marks a unit whose change is a claim sync.
type claimSyncState struct {
	actor      actor.Actor
	resource   txn.Ref
	providerID ids.ID
	groupIDs   []ids.ID
}

// claimSyncKey keys the claimSyncState in the unit of work.
var claimSyncKey = txn.NewKey[*claimSyncState]("authz claim sync")

// SyncClaimGroups implements authz.ClaimSync (brief M1-42, 08 section 3.4): it locks the person's
// claim memberships of the provider, reads the matched claim groups, computes the diff with
// domain.ReconcileClaimMemberships and, in a unit declared for group.member.sync, applies it as
// the person (added_by and removed_by), appends group.changed per added or removed membership and
// records one allow entry with the group ids. Outside a rights change a non-empty diff writes
// nothing and returns authz.ErrClaimSyncNeedsRightsChange.
func (s *AdminService) SyncClaimGroups(ctx context.Context, cmd authz.SyncClaimGroupsCmd) (authz.SyncClaimGroupsResult, error) {
	if cmd.UserID.IsZero() || cmd.ProviderID.IsZero() || cmd.Now.IsZero() || cmd.MaxAge <= 0 {
		return authz.SyncClaimGroupsResult{}, internal("a claim sync needs the person, the provider, the time and auth.group_claim_max_age")
	}
	if !txn.InUnit(ctx) {
		return authz.SyncClaimGroupsResult{}, internal("the claim sync runs only inside the unit of work of the login")
	}
	spec, declared := txn.RightsChangeOf(ctx)
	if declared && spec.Action != string(authorize.ActionGroupMemberSync) {
		return authz.SyncClaimGroupsResult{}, internal("the claim sync runs in a rights change of " + spec.Action)
	}
	current, err := s.d.Store.LockClaimMembershipsOfUser(ctx, cmd.UserID, cmd.ProviderID)
	if err != nil {
		return authz.SyncClaimGroupsResult{}, err
	}
	matched, err := s.d.Store.ClaimGroupsByRefs(ctx, cmd.ProviderID, cmd.ClaimValues)
	if err != nil {
		return authz.SyncClaimGroupsResult{}, err
	}
	diff := domain.ReconcileClaimMemberships(cmd.ProviderID, current, matched, cmd.ClaimValues, cmd.Now, cmd.MaxAge)
	if diff.Empty() {
		return authz.SyncClaimGroupsResult{}, nil
	}
	if !declared {
		return authz.SyncClaimGroupsResult{}, authz.ErrClaimSyncNeedsRightsChange
	}
	person := actor.Actor{Kind: actor.KindUser, ID: cmd.UserID}
	ctx = actor.WithActor(ctx, person)
	touched := domain.SortIDs(slices.Concat(diff.Add, diff.Extend, diff.Remove))
	if err := txn.Store(ctx, claimSyncKey, &claimSyncState{
		actor: person, resource: txn.Ref{Kind: string(authorize.ResourceUser), ID: cmd.UserID}, providerID: cmd.ProviderID, groupIDs: touched,
	}); err != nil {
		return authz.SyncClaimGroupsResult{}, err
	}
	// The view of the person before the change is recorded before any row of it is written.
	if err := s.d.Rights.AddAffected(ctx, cmd.UserID); err != nil {
		return authz.SyncClaimGroupsResult{}, err
	}
	if err := s.applyClaimDiff(ctx, cmd, diff, current, matched); err != nil {
		return authz.SyncClaimGroupsResult{}, err
	}
	err = s.record(ctx, authorize.ActionGroupMemberSync, authorize.ResourceUser, cmd.UserID, auditing.Attrs{
		attrProviderID: auditing.ID(cmd.ProviderID), attrAdded: idList(diff.Add), attrRemoved: idList(diff.Remove), attrExtended: idList(diff.Extend),
	})
	if err != nil {
		return authz.SyncClaimGroupsResult{}, err
	}
	return authz.SyncClaimGroupsResult{Added: diff.Add, Removed: diff.Remove, Extended: diff.Extend}, nil
}

// applyClaimDiff writes diff in ascending group id per kind: removals and extensions of the
// locked rows, then the new memberships, with group.changed for each addition and removal.
func (s *AdminService) applyClaimDiff(ctx context.Context, cmd authz.SyncClaimGroupsCmd, diff domain.ClaimDiff,
	current []domain.MembershipWithGroup, matched []domain.Group) error {
	byGroup := map[ids.ID]domain.MembershipWithGroup{}
	for _, m := range current {
		byGroup[m.Group.ID] = m
	}
	if err := s.removeClaimMembers(ctx, cmd, diff.Remove, byGroup); err != nil {
		return err
	}
	for _, g := range diff.Extend {
		if ok, err := s.d.Store.ExtendMembership(ctx, byGroup[g].Membership.ID, diff.ExpiresAt, cmd.Now); err != nil || !ok {
			return orInternal(err, "a locked claim membership vanished")
		}
	}
	return s.addClaimMembers(ctx, cmd, diff, matched)
}

// removeClaimMembers removes the locked memberships of the groups with removed_by the person and
// appends group.changed member_removed for each.
func (s *AdminService) removeClaimMembers(ctx context.Context, cmd authz.SyncClaimGroupsCmd, groups []ids.ID,
	byGroup map[ids.ID]domain.MembershipWithGroup) error {
	for _, g := range groups {
		m := byGroup[g]
		if ok, err := s.d.Store.RemoveMembership(ctx, m.Membership.ID, cmd.UserID, cmd.Now); err != nil || !ok {
			return orInternal(err, "a locked claim membership vanished")
		}
		if err := s.groupEvent(ctx, authorize.ActionGroupMemberSync, m.Group, changeMemberRemoved, cmd.UserID, cmd.Now); err != nil {
			return err
		}
	}
	return nil
}

// addClaimMembers inserts the memberships of diff.Add with added_by the person and the expiry of
// diff, and appends group.changed member_added for each.
func (s *AdminService) addClaimMembers(ctx context.Context, cmd authz.SyncClaimGroupsCmd, diff domain.ClaimDiff, matched []domain.Group) error {
	groups := map[ids.ID]domain.Group{}
	for _, g := range matched {
		groups[g.ID] = g
	}
	for _, g := range diff.Add {
		m, err := domain.NewMembership(ids.New(), g, cmd.UserID, cmd.UserID, diff.ExpiresAt, cmd.Now)
		if err != nil {
			return err
		}
		if err := s.d.Store.InsertMembership(ctx, m); err != nil {
			return err
		}
		if err := s.groupEvent(ctx, authorize.ActionGroupMemberSync, groups[g], changeMemberAdded, cmd.UserID, cmd.Now); err != nil {
			return err
		}
	}
	return nil
}

// orInternal returns err, or an internal error with detail for a nil err.
func orInternal(err error, detail string) error {
	if err != nil {
		return err
	}
	return internal(detail)
}

// idList is the audit list of ids.
func idList(list []ids.ID) auditing.Value {
	vs := make([]auditing.Value, 0, len(list))
	for _, id := range list {
		vs = append(vs, auditing.ID(id))
	}
	return auditing.List(vs...)
}

// claimSyncOf returns the claim sync state of the unit of ctx, or nil for another change.
func claimSyncOf(ctx context.Context) *claimSyncState {
	st, _ := txn.Load(ctx, claimSyncKey)
	return st
}

// claimSyncDenial adds the details of a refused claim sync to the deny entry e: the resource is
// the person, the details name the provider, the groups of the change and the agents in
// violation (ids only).
func claimSyncDenial(e auditing.Entry, cs *claimSyncState, v Violations) auditing.Entry {
	e.ResourceKind, e.ResourceID = cs.resource.Kind, cs.resource.ID
	agents := make([]ids.ID, 0, len(v.I1))
	for _, c := range v.I1 {
		agents = append(agents, c.AgentID)
	}
	attrs := merge(e.Attrs, auditing.Attrs{
		attrProviderID: auditing.ID(cs.providerID), attrGroupIDs: idList(cs.groupIDs), attrAgentIDs: idList(domain.SortIDs(agents)),
	})
	e.Attrs = attrs
	return e
}
