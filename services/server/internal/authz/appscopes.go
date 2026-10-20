// SPDX-License-Identifier: Apache-2.0

package authz

import "github.com/OWNER/bothy/services/server/internal/kernel/authorize"

// AppScopes is the port of authz over the scope table of 05 section 2.6 for the manifest rules of
// apps (M2-24 design notes 1 and 4): authz owns every rights rule, so the module apps asks here
// which events an app may subscribe to and which scopes exist. authz/app implements it
// (ScopeTable); internal/wire binds it into apps.
type AppScopes interface {
	// EventScope reports whether an app may subscribe to t and the scope the receive action of
	// t needs ("" for none): a durable type without audience self whose receive action the scope
	// table lists, or interaction.created and command.invoked (events:subscribe). Every other
	// type is not subscribable.
	EventScope(t authorize.EventType) (scope string, subscribable bool)
	// Scopes returns the scopes of the catalogue, ascending.
	Scopes() []string
}
