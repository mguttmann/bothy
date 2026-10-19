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

// CheckAppScope decides the scope rule for an app principal (05 section 2.6, 07 section 2.10):
// the empty Reason when allowed, ReasonScopeMissing when the action needs a scope that the
// installation (granted) or, when the request carries an API token (hasToken), the token (token)
// lacks, and ReasonSubjectKindNotAllowed when the action is not open to apps. A check without a
// token, for example a webhook delivery, uses granted alone (M2-24 design note 10 c).
func CheckAppScope(action string, granted, token []string, hasToken bool) Reason {
	scope, listed := appScopes[action]
	switch {
	case !listed:
		return ReasonSubjectKindNotAllowed
	case scope == "":
		return ""
	case !slices.Contains(granted, scope), hasToken && !slices.Contains(token, scope):
		return ReasonScopeMissing
	}
	return ""
}

// AppScopeNames returns the scopes of the catalogue (05 section 2.6), ascending.
func AppScopeNames() []string {
	return []string{ScopeCommandsWrite, ScopeComponentsWrite, ScopeCostsReconcile, ScopeEventsSubscribe, ScopeFilesRead,
		ScopeFilesWrite, ScopeMessagesRead, ScopeMessagesWrite, ScopeUsersRead}
}

// receiversAgentOrApp is EventSpec.Receivers of the personal events an app may receive
// (interaction.created and command.invoked, E-EX-05).
const receiversAgentOrApp = "agent_or_app"

// AppEventScope decides whether an app may subscribe to the event type t (11 section 3.2 rules 1
// and 2, M2-24 design note 4) and returns the scope its receive action needs: a durable type
// without audience self whose one receive action the scope table lists (with a scope or without
// one), or a personal type an app receives (receivers agent_or_app), which needs
// events:subscribe. Every other type (unknown, ephemeral, personal events of other principals such
// as access.changed and draft.*, and receive actions apps may not run such as grant.read and
// host.read) is not subscribable.
func AppEventScope(t string) (scope string, subscribable bool) {
	spec, ok := LookupEvent(t)
	if !ok || !spec.Durable {
		return "", false
	}
	if spec.AudienceSelf {
		if spec.Receivers == receiversAgentOrApp {
			return ScopeEventsSubscribe, true
		}
		return "", false
	}
	if len(spec.MentionedActions) != 1 {
		return "", false
	}
	return AppScopeOf(spec.MentionedActions[0])
}
