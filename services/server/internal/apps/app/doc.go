// SPDX-License-Identifier: Apache-2.0

// Package app holds the use cases of the module apps (M2-24): one per operation of the port
// apps.Apps, the outgoing Repository port over apps and app_installations, and the consumer ports
// of identity (principals, tokens) and authz (grant revocation, the scope table, the rights
// change). It authorizes every call with kernel/authorize first and runs every write in one unit
// of work of kernel/txn.
package app
