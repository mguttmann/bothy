// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// stepUpOnlyWithExistingCredential holds the condition of every action whose step-up is
// conditional ("bedingt" in 05 section 2.4): hat_anmeldedaten(p, action) of 07 section 4.2 step 8,
// asked through PrincipalSource. A function that returns false waives the step-up, because the
// person sets up the first sign-in data of that kind; "human with a session" stays required. A
// conditional action without an entry counts as "ja" (07 section 4.2). The rights logic stays here
// in authz (B7).
var stepUpOnlyWithExistingCredential = map[authorize.Action]func(ctx context.Context, src authz.PrincipalSource, userID ids.ID) (bool, error){
	// passkey.register: step-up once the person has an active passkey (M1-40).
	authorize.ActionPasskeyRegister: func(ctx context.Context, src authz.PrincipalSource, userID ids.ID) (bool, error) {
		return src.HasActivePasskey(ctx, userID)
	},
	// local_credential.set: step-up once the password path is established, TOTP confirmed (M1-39).
	// The first set-up and its confirmation work without step-up; a stolen session that adds a
	// password only gains a login whose new devices stay pending (brief M1-39 "Conditional
	// step-up").
	authorize.ActionLocalCredentialSet: func(ctx context.Context, src authz.PrincipalSource, userID ids.ID) (bool, error) {
		return src.HasEstablishedLocalCredential(ctx, userID)
	},
}

// stepUpWaived reports whether the conditional step-up of spec is waived for p: only for a person
// in a session, only for an action with a condition, and only when the condition is false. An
// error of the source fails the call (never an allow).
func (a *Authorizer) stepUpWaived(ctx context.Context, spec domain.ActionSpec, act authorize.Action, p domain.Principal) (bool, error) {
	if !spec.StepUpConditional || p.Kind != domain.KindUser || !p.HasSession {
		return false, nil
	}
	condition, ok := stepUpOnlyWithExistingCredential[act]
	if !ok {
		return false, nil
	}
	has, err := condition(ctx, a.principal, p.ID)
	if err != nil {
		return false, sourceError("principal source", err)
	}
	return !has, nil
}
