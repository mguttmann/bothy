// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
)

// Preference is one own row of notification_preferences (03) for the thread, the channel or the
// space of a hint.
type Preference struct {
	// Level is the stored level of the row.
	Level Level
	// MutedUntil is the column muted_until; the zero time stands for NULL. While it is strictly
	// after "now" the row counts as mute (13 3.1).
	MutedUntil time.Time
}

// LevelFacts are the sources of 13 3.1 for the thread of one hint and one person. The caller
// gathers them: the own rows from notification_preferences (M3-18), ChannelDefault and
// DirectMessage from chat, PersonalDefault from user_settings through sync, InstanceDefault from
// the configuration key notify.user_default_level.
type LevelFacts struct {
	// Thread, Channel and Space are the own rows for the thread, its channel and its space; nil
	// when the person has no such row.
	Thread, Channel, Space *Preference
	// ChannelDefault is channels.default_notification_level; empty when NULL.
	ChannelDefault Level
	// DirectMessage reports channels.kind = 'dm'.
	DirectMessage bool
	// PersonalDefault is user_settings['notify.default_level']; empty when unset.
	PersonalDefault Level
	// InstanceDefault is the value of notify.user_default_level; required.
	InstanceDefault Level
}

// EffectiveLevel is the result of [ResolveLevel].
type EffectiveLevel struct {
	// Level is the effective level: mute while the chosen row is muted, else the chosen value.
	Level Level
	// Source names the step of 13 3.1 that won.
	Source LevelSource
	// Muted reports that the chosen row's muted_until is after now.
	Muted bool
}

// ResolveLevel returns the effective level of 13 3.1 at the time now: the first present value of
// the thread row, the channel row, the space row, the channel default, all for a DM channel, the
// personal default and the instance default. Only an own row has muted_until: if the chosen row's
// MutedUntil is strictly after now, the level is mute and Muted is true; once now reaches it, the
// row's stored level applies again. A muted_until on a row that was not chosen has no effect
// (D-M3-17-3).
//
// Errors: an internal_error of the kernel (errs.CodeInternalError) and the zero EffectiveLevel for
// a zero now, an unknown level in any present source (also below the chosen one) and an empty or
// unknown InstanceDefault; the text names the field, never the value.
//
// Example:
//
//	eff, err := domain.ResolveLevel(domain.LevelFacts{
//		Thread:          &domain.Preference{Level: domain.LevelAll, MutedUntil: evening},
//		InstanceDefault: domain.LevelMentions,
//	}, now)
//	// before evening: eff == {Level: mute, Source: thread, Muted: true}
func ResolveLevel(f LevelFacts, now time.Time) (EffectiveLevel, error) {
	if err := validateLevelFacts(f, now); err != nil {
		return EffectiveLevel{}, err
	}
	rows := []struct {
		row    *Preference
		source LevelSource
	}{{f.Thread, SourceThread}, {f.Channel, SourceChannel}, {f.Space, SourceSpace}}
	for _, r := range rows {
		if r.row == nil {
			continue
		}
		if r.row.MutedUntil.After(now) {
			return EffectiveLevel{Level: LevelMute, Source: r.source, Muted: true}, nil
		}
		return EffectiveLevel{Level: r.row.Level, Source: r.source}, nil
	}
	switch {
	case f.ChannelDefault != "":
		return EffectiveLevel{Level: f.ChannelDefault, Source: SourceChannelDefault}, nil
	case f.DirectMessage:
		return EffectiveLevel{Level: LevelAll, Source: SourceDirectMessage}, nil
	case f.PersonalDefault != "":
		return EffectiveLevel{Level: f.PersonalDefault, Source: SourcePersonalDefault}, nil
	}
	return EffectiveLevel{Level: f.InstanceDefault, Source: SourceInstanceDefault}, nil
}

// validateLevelFacts checks every source, not only the chosen one: a broken input fails closed,
// whichever row would win (D-M3-17-7).
func validateLevelFacts(f LevelFacts, now time.Time) error {
	if now.IsZero() {
		return invalid("now")
	}
	for _, r := range []struct {
		row  *Preference
		name string
	}{{f.Thread, "level.thread"}, {f.Channel, "level.channel"}, {f.Space, "level.space"}} {
		if r.row != nil && !r.row.Level.Valid() {
			return invalid(r.name)
		}
	}
	if f.ChannelDefault != "" && !f.ChannelDefault.Valid() {
		return invalid("level.channel_default")
	}
	if f.PersonalDefault != "" && !f.PersonalDefault.Valid() {
		return invalid("level.personal_default")
	}
	if !f.InstanceDefault.Valid() {
		return invalid("level.instance_default")
	}
	return nil
}

// invalid returns the internal error of design note 10 for a broken field. The detail names the
// field only, because values may be anything the caller gathered.
func invalid(field string) error {
	return errs.New(errs.CodeInternalError, "notify decision: invalid input field "+field)
}
