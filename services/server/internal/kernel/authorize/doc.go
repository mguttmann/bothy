// SPDX-License-Identifier: Apache-2.0

// Package authorize holds the actions (generated from docs/spec/actions.yaml, ADR 0049), the
// resource descriptor, Decision and the Authorizer (Authorize, Scope for lists, Observe for events
// of the log) with its filter and delivery modes, and the in-transaction event AccessChanged. The
// authz module implements it; every module uses it.
package authorize
