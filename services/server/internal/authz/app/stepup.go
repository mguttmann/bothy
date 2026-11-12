// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// conditionalStepUp are the actions whose step-up is conditional ("bedingt" in 05 section 2.4):
// they set up sign-in data that confirms a step-up or signs in to the console. A conditional action
// without an entry counts as "ja" (07 section 4.2).
var conditionalStepUp = map[authorize.Action]bool{
	// passkey.register adds a passkey (M1-40).
	authorize.ActionPasskeyRegister: true,
	// local_credential.set sets the password and TOTP of a local account (M1-39).
	authorize.ActionLocalCredentialSet: true,
}

// signInData are the facts of the person behind can_confirm of 07 section 4.2 step 8, asked through
// PrincipalSource: sign-in data that confirms a step-up, whatever kind the action sets up. A
// password alone must not add the first passkey, which opens the terminal (08 section 6.3), and a
// passkey alone not the first password.
var signInData = []func(ctx context.Context, src authz.PrincipalSource, userID ids.ID) (bool, error){
	func(ctx context.Context, src authz.PrincipalSource, userID ids.ID) (bool, error) {
		return src.HasActivePasskey(ctx, userID)
	},
	func(ctx context.Context, src authz.PrincipalSource, userID ids.ID) (bool, error) {
		return src.HasEstablishedLocalCredential(ctx, userID)
	},
}

// stepUpWaived reports whether the conditional step-up of spec is waived for s (07 section 4.2
// step 8, can_confirm): only for a person in a session, only for an action with a condition, and
// only while the session has nothing to confirm a step-up with. That is a session outside the
// console on a device that is not cli and has no active step_up key, of a person without an active
// passkey and without an established password path; such a person sets up the first sign-in data
// with the session alone. Every other session confirms first, so a session never issues the factor of its own
// step-up: a device with a step_up key confirms with it, a console session passed its first
// step-up before it reaches the action (08 section 7.5), and a cli device never confirms and never
// sets up sign-in data (08 section 6.4). An error of the source fails the call (never an allow).
// The rights logic stays here in authz (B7).
func (a *Authorizer) stepUpWaived(ctx context.Context, spec domain.ActionSpec, act authorize.Action, s *actingPrincipal) (bool, error) {
	if !spec.StepUpConditional || s.kind != domain.KindUser || s.ref.SessionID.IsZero() || !conditionalStepUp[act] {
		return false, nil
	}
	if c := s.entry.creds; c.DeviceCLI || c.DeviceStepUpKey || s.p.ClientComponentName() == consoleComponent {
		return false, nil
	}
	for _, has := range signInData {
		ok, err := has(ctx, a.principal, s.id)
		if err != nil {
			return false, sourceError("principal source", err)
		}
		if ok {
			return false, nil
		}
	}
	return true, nil
}
