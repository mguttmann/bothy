// SPDX-License-Identifier: Apache-2.0

// Package authorize holds the actions (generated from docs/spec/actions.yaml, ADR 0049), the
// resource descriptor, Decision and the Authorizer (Authorize, Scope for lists, Observe for events
// of the log) with its filter and delivery modes, and the in-transaction event AccessChanged. The
// authz module implements it; every module uses it.
//
// The check log (WithCheckLog, RecordCheck, Checks; M1-18) is the proof of
// docs/spec/02-architecture.md 9.3: every implementation of the Authorizer must call RecordCheck
// once per Authorize and once per Scope call, and the HTTP chain refuses a successful response of
// a handler that never checked its route's action. The Authorizer interface itself comes with
// M1-28 and repeats this contract in its doc comment.
package authorize
