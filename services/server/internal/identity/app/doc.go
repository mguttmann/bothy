// SPDX-License-Identifier: Apache-2.0

// Package app holds the application layer of the identity module: the outgoing port Store over
// the repository, the read-only Directory of the identity port and the implementation of
// authz.PrincipalSource, and Config with the limits of handles and display names.
//
// Reads join the ambient transaction of kernel/txn when the context carries one; the Store
// implementation decides that (platform/pg.Querier), so this package stays free of drivers.
//
// Example:
//
//	dir := app.NewDirectory(store)
//	view, err := dir.Principal(ctx, id) // errs not_found for an unknown id
package app
