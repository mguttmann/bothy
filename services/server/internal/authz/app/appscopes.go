// SPDX-License-Identifier: Apache-2.0

package app

import (
	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
)

// ScopeTable implements authz.AppScopes over the generated catalogue and the scope table of 05
// section 2.6 (domain.AppEventScope). The zero value is ready and safe for concurrent use.
type ScopeTable struct{}

// Compile-time check: ScopeTable is the port.
var _ authz.AppScopes = ScopeTable{}

// EventScope implements authz.AppScopes.
func (ScopeTable) EventScope(t authorize.EventType) (string, bool) {
	return domain.AppEventScope(string(t))
}

// Scopes implements authz.AppScopes.
func (ScopeTable) Scopes() []string { return domain.AppScopeNames() }
