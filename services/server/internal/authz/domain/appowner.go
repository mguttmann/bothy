// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"slices"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// The app as owner of a command or a card (brief M2-25 design note 3; docs/spec/05-actions.md
// section 5, last paragraph; 11 section 9; 09 section 10.3): a person who runs an app command or
// operates an app component is checked for the command's level or trigger, and the app for an
// active installation, the scope of the feature and write on the place. agent.invoke and I2 are
// never checked for app owners, because no agent runtime is involved (R-AZ-18).

// The actions of the app rules.
const (
	actionAgentEventsConsume = "agent.events.consume"
)

// AppOwnerFacts are the facts of an app that owns a command or a card, gathered by the
// application layer from the principal source (installation state and granted scopes, by the
// app's principal ID) and the app's own rights snapshot on the resolved path.
type AppOwnerFacts struct {
	// ID is the app.
	ID ids.ID
	// Active reports principal status active; InstallationActive an installation in state active.
	Active, InstallationActive bool
	// GrantedScopes are app_installations.granted_scopes.
	GrantedScopes []string
	// OnChannel reports that a channel is resolved for the check (list items, command.invoke,
	// command.autocomplete, interaction.create); Level is then the app's own effective level there
	// and DM whether the channel is a dm channel, which apps never reach (05 section 2.2).
	OnChannel bool
	Level     Level
	DM        bool
}

// AppOwnerReachable is the guard of an app owner for the feature scope (commands:write for
// commands, components:write for components): the app is active with an active installation, the
// scope is granted and, where a channel is resolved, the channel is no dm channel and the app holds
// at least write there (07 sections 2.1 and 2.10: apps hold at most write). Without a channel
// (getCommand of an instance-wide command) only the installation and the scope count (brief M2-25
// design note 3 b).
//
// Example:
//
//	ok := domain.AppOwnerReachable(f, domain.ScopeCommandsWrite)
func AppOwnerReachable(f AppOwnerFacts, scope string) bool {
	if !f.Active || !f.InstallationActive || !slices.Contains(f.GrantedScopes, scope) {
		return false
	}
	return !f.OnChannel || (!f.DM && f.Level >= LevelWrite)
}

// appOwnerHidden is the visibility part of the guard for command.invoke and command.autocomplete
// of an app command (design note 3 c and d): a false guard hides the command like an unknown one
// (not_found no_grant), before the level of the command is checked. The owning app itself is never
// hidden from itself. done is false for every other check.
func appOwnerHidden(in *Input) (Decision, bool) {
	if in.Action != actionCommandInvoke && in.Action != actionCommandAutocomplete {
		return Decision{}, false
	}
	c := in.Command
	if c == nil || c.OwnerKind != commandOwnerApp || in.AppOwner == nil || in.AppOwner.ID == in.Principal.ID {
		return Decision{}, false
	}
	if AppOwnerReachable(*in.AppOwner, ScopeCommandsWrite) {
		return Decision{}, false
	}
	return deny(OutcomeNotFound, ReasonNoGrant), true
}

// AppReceive is the receive rule of a personal event for an app in filter mode (brief M2-25 design
// note 4; 05 section 6 rows command.invoked and interaction.created: "app events:subscribe"; 03
// event_log.receive_action): the event of another principal is not_found not_self, an app without
// an active installation is unauthenticated app_not_installed (07 section 4.1), an app without
// events:subscribe forbidden scope_missing; the audience app with both is allowed.
//
// Example:
//
//	d := domain.AppReceive(app, true, []string{"events:subscribe"}, audience)
func AppReceive(app ids.ID, installationActive bool, granted []string, audience ids.ID) Decision {
	switch {
	case app.IsZero() || audience != app:
		return deny(OutcomeNotFound, ReasonNotSelf)
	case !installationActive:
		return deny(OutcomeUnauthenticated, ReasonAppNotInstalled)
	case !slices.Contains(granted, ScopeEventsSubscribe):
		return deny(OutcomeForbidden, ReasonScopeMissing)
	}
	return Allow
}

// IsAppReceive reports the check AppReceive decides: agent.events.consume for a principal of kind
// app in filter mode. In request mode (REST poll, ack, the consumer take) the requirement stays
// AgentSelf, so an app keeps forbidden subject_kind_not_allowed there (ruling D-M2-25).
func IsAppReceive(action, principalKind string, filter bool) bool {
	return filter && action == actionAgentEventsConsume && principalKind == KindApp
}
