// SPDX-License-Identifier: Apache-2.0

// Package app holds the application layer of the authz module: the catalogue service (M1-25), the
// outgoing port Store over the tables groups, group_memberships and grants (M1-27), and the
// Authorizer (M1-28), which implements kernel/authorize.Authorizer with the rights snapshot, its
// cache per rights version and the visible sets of Scope; the RightsHook (M1-29), the one
// transactional path of every rights change with I1, SR8 and access.changed, and AdminService,
// the group, membership and grant use cases of the port authz.Admin (M1-29; routes in M1-48).
//
// Reads join the ambient transaction of kernel/txn when the context carries one; the Store
// implementation decides that (platform/pg.Querier), so this package stays free of drivers.
// Time filtering stays in the domain (domain.Grant.InEffect, domain.Membership.InEffect), so one
// clock decides validity.
//
// Example:
//
//	grants, err := store.ActiveGrantsOfPrincipal(ctx, sam)
//	for _, g := range grants {
//		if g.InEffect(now) { ... }
//	}
package app
