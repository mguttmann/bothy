// SPDX-License-Identifier: Apache-2.0

// Package app holds the application layer of the authz module. So far it declares only the
// outgoing port Store over the tables groups, group_memberships and grants (M1-27); the rights
// snapshot and authorize (M1-28), rights changes (M1-29) and the group and grant use cases
// (M1-48) add their use cases here.
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
