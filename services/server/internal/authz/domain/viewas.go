// SPDX-License-Identifier: Apache-2.0

package domain

import "github.com/OWNER/bothy/services/server/internal/kernel/ids"

// ExemptTarget is the resource of a decision as the rule of ViewAsExempt reads it: its kind, its
// id and the principal it belongs to.
type ExemptTarget struct {
	Kind        string
	ID, OwnerID ids.ID
}

// The actions and resource kinds of the rule of R-AZ-32.
const (
	actionViewAsEnd    = "access.view_as.end"
	actionLogout       = "auth.logout"
	actionStepUp       = "auth.step_up"
	actionSelfRead     = "self.read"
	kindAuthSessionRes = "auth_session"
)

// sessionExempt are the actions on the own session that "View as person" never decides
// (R-AZ-32): ending the view, logging out and the step-up.
var sessionExempt = map[string]bool{actionViewAsEnd: true, actionLogout: true, actionStepUp: true}

// ViewAsExempt reports whether a decision of act on r for the principal principal in the session
// session is made without its stored "View as person" (R-AZ-32, 07 section 4.2): access.view_as.end,
// auth.logout and auth.step_up on the own session (resource auth_session with the session's id)
// and self.read on oneself. Without this rule steps 4 and 7 of 07 section 4.2 would deny them,
// because they write and the viewed person has no right on the owner's session, and the owner
// could neither leave the view nor log out. A principal without a session is never exempt.
func ViewAsExempt(act string, r ExemptTarget, principal, session ids.ID) bool {
	if session.IsZero() || principal.IsZero() {
		return false
	}
	switch {
	case sessionExempt[act]:
		return r.Kind == kindAuthSessionRes && r.ID == session && r.OwnerID == principal
	case act == actionSelfRead:
		return (r.Kind == kindPrincipal || r.Kind == kindUserRes) && r.ID == principal
	}
	return false
}
