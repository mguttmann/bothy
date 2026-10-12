// SPDX-License-Identifier: Apache-2.0

// Package authorize holds the actions (generated from docs/spec/actions.yaml, ADR 0049), the
// resource descriptor, Decision and the Authorizer (Authorize, Scope for lists, Observe for events
// of the log) with its filter and delivery modes, and the in-transaction event AccessChanged. The
// authz module implements it; every module uses it.
//
// The closed catalogue (M1-25): catalog_ids_gen.go, written by tools/catalogen, declares the
// identifier types Action, EventType, Op, ResourceKind and ChannelOp with one constant per
// catalogue entry and the functions ParseAction, ParseEventType, ParseOp, ParseResourceKind,
// ParseChannelOp and their All counterparts. These constants are the only declaration of the
// catalogue's identifiers in Go: code obtains a value from a constant or a Parse function, never
// by converting a string (Semgrep rule bothy-go-catalog-id-conversion; its limits are in
// internal/authz/README.md). The metadata of each entry
// lives in internal/authz/domain. Registry collects the event types, WebSocket ops and terminal
// channel ops a binary emits or accepts; modules register them through Register(r *Registry) in
// their port package, internal/wire calls these functions, and the start refuses to continue when
// a registration or a route binding (RouteBinding) breaks the catalogue (05 rules 3 to 5, 10;
// 07 AZ-P1 and AZ-P2).
//
// The check log (WithCheckLog, RecordCheck, Checks; M1-18) is the proof of
// docs/spec/02-architektur.md 9.3: every implementation of the Authorizer must call RecordCheck
// once per Authorize and once per Scope call, and the HTTP chain refuses a successful response of
// a handler that never checked its route's action. The Authorizer interface itself comes with
// M1-28 and repeats this contract in its doc comment; every implementation starts with the
// catalogue lookup of authz (Catalog.Spec), so an unknown action fails closed (AZ-P3).
package authorize

//go:generate go run ../../../tools/catalogen -catalog ../../../../../docs/spec/actions.yaml -ids catalog_ids_gen.go -domain ../../authz/domain/catalog_gen.go
