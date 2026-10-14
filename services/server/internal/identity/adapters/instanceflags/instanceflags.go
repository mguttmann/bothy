// SPDX-License-Identifier: Apache-2.0

// Package instanceflags implements the consumer port app.PersonalTokenSwitch of identity (M1-44)
// over the feature flags of instance (M1-47): createToken asks tokens.personal_enabled for the
// caller through instance.Flags, so a rollout for single people or groups applies. internal/wire
// builds it over the flags of the instance module.
//
// Example:
//
//	sw, err := instanceflags.New(settings.Admin) // app.TokensDeps.Switch
package instanceflags

import (
	"context"
	"errors"

	"github.com/OWNER/bothy/services/server/internal/identity/app"
	"github.com/OWNER/bothy/services/server/internal/instance"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Switch implements app.PersonalTokenSwitch. It is safe for concurrent use.
type Switch struct {
	flags instance.Flags
}

// Compile-time check: Switch is the consumer port of identity.
var _ app.PersonalTokenSwitch = (*Switch)(nil)

// New returns the switch over flags. Errors: nil flags.
func New(flags instance.Flags) (*Switch, error) {
	if flags == nil {
		return nil, errors.New("instanceflags: New needs the flags of instance")
	}
	return &Switch{flags: flags}, nil
}

// PersonalTokensEnabled implements app.PersonalTokenSwitch: tokens.personal_enabled evaluated for
// principalID. The kind is that of the actor of the request when it is principalID, else user
// (personal tokens belong to people). Errors: those of the flags; the caller fails closed.
func (s *Switch) PersonalTokensEnabled(ctx context.Context, principalID ids.ID) (bool, error) {
	subj := instance.FlagSubject{PrincipalID: principalID, Kind: string(actor.KindUser)}
	if a, ok := actor.FromContext(ctx); ok && a.ID == principalID {
		subj = instance.SubjectOf(a)
	}
	return s.flags.Enabled(ctx, subj, instance.FlagTokensPersonalEnabled)
}
