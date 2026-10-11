// SPDX-License-Identifier: Apache-2.0

// Package txn declares the unit of work, its phases before the commit and the in-transaction event
// bus (docs/spec/02-architektur.md, "Transaktionen und Unit of Work"). platform/pg implements it
// with the phase hooks of authz, sync and audit; every app package uses it.
package txn
