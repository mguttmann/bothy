// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"maps"
	"slices"
)

// The app scopes of 05 section 2.6 (app_installations.granted_scopes).
const (
	ScopeMessagesRead    = "messages:read"
	ScopeMessagesWrite   = "messages:write"
	ScopeFilesRead       = "files:read"
	ScopeFilesWrite      = "files:write"
	ScopeCommandsWrite   = "commands:write"
	ScopeComponentsWrite = "components:write"
	ScopeUsersRead       = "users:read"
	ScopeEventsSubscribe = "events:subscribe"
	ScopeCostsReconcile  = "costs:reconcile"
)

// appScopes maps an action to the scope an app needs for it (05 section 2.6); "" means no scope
// is needed. An action outside the table is not open to apps at all. cost.reconcile.read comes
// with M2-28, which adds it to the catalogue (R-AZ-28). TestScopeTableActionsExist holds the table
// against the catalogue and its AppScope column.
//
//nolint:goconst // the keys are action ids of the generated catalogue, one row per action
var appScopes = map[string]string{
	"message.read":                 ScopeMessagesRead,
	"thread.read":                  ScopeMessagesRead,
	"channel.read":                 ScopeMessagesRead,
	"space.read":                   ScopeMessagesRead,
	"run.read":                     ScopeMessagesRead,
	"search.query":                 ScopeMessagesRead,
	"message.create":               ScopeMessagesWrite,
	"message.update":               ScopeMessagesWrite,
	"message.delete":               ScopeMessagesWrite,
	"reaction.add":                 ScopeMessagesWrite,
	"reaction.remove":              ScopeMessagesWrite,
	"thread.create":                ScopeMessagesWrite,
	"file.read":                    ScopeFilesRead,
	"file.upload":                  ScopeFilesWrite,
	"command.register":             ScopeCommandsWrite,
	"command.remove":               ScopeCommandsWrite,
	"command.autocomplete.respond": ScopeCommandsWrite,
	"component.update":             ScopeComponentsWrite,
	"interaction.respond":          ScopeComponentsWrite,
	"user.read":                    ScopeUsersRead,
	"gateway.connect":              ScopeEventsSubscribe,
	"events.catch_up":              ScopeEventsSubscribe,
	"cost.reconcile.read":          ScopeCostsReconcile,
	"cost.reconcile.submit":        ScopeCostsReconcile,
	"self.read":                    "",
	"instance.read":                "",
	"status.read":                  "",
	"command.read":                 "",
	"agent.read":                   "",
	"webhook_in.post":              "",
}

// AppScopeOf returns the scope an app needs for action; listed is false when apps may not run the
// action at all.
func AppScopeOf(action string) (scope string, listed bool) {
	scope, listed = appScopes[action]
	return scope, listed
}

// ScopeTableActions returns the actions of the scope table, ascending.
func ScopeTableActions() []string { return slices.Sorted(maps.Keys(appScopes)) }

// CheckAppScope decides the scope rule for an app principal with the granted scopes: the empty
// Reason when allowed, ReasonScopeMissing when the action needs a scope the app lacks, and
// ReasonSubjectKindNotAllowed when the action is not open to apps.
func CheckAppScope(action string, granted []string) Reason {
	scope, listed := appScopes[action]
	switch {
	case !listed:
		return ReasonSubjectKindNotAllowed
	case scope != "" && !slices.Contains(granted, scope):
		return ReasonScopeMissing
	}
	return ""
}
