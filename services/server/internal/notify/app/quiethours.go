// SPDX-License-Identifier: Apache-2.0

package app

import (
	"bytes"
	"context"
	"slices"
	"time"

	// The server image has no zone database (M1-31); time_zone names must load anyway.
	_ "time/tzdata"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/notify"
	"github.com/OWNER/bothy/services/server/internal/notify/domain"
)

// List implements notify.QuietHours (brief M3-18 note 9 e): settings.read on the caller, then
// every rule, ascending by id, disabled ones included.
func (s Quiet) List(ctx context.Context) ([]notify.QuietHoursRule, error) {
	a := caller(ctx)
	if err := s.check(ctx, a, authorize.ActionSettingsRead, self(a.ID)); err != nil {
		return nil, err
	}
	return s.d.Store.Rules(ctx, a.ID)
}

// Create implements notify.QuietHours (brief M3-18 note 9 d): a new rule with a UUIDv7 and
// version 1; quiet_hours.updated carries the complete list.
func (s Quiet) Create(ctx context.Context, in notify.QuietHoursInput) (notify.QuietHoursRule, error) {
	var out notify.QuietHoursRule
	err := s.write(ctx, in, func(ctx context.Context, a actor.Actor, r notify.QuietHoursRule) (notify.QuietHoursRule, bool, error) {
		r.ID = ids.New()
		rules, err := s.d.Store.Rules(ctx, a.ID)
		if err != nil {
			return notify.QuietHoursRule{}, false, err
		}
		r.Version = 1
		if err := s.fits(append(rules, r)); err != nil {
			return notify.QuietHoursRule{}, false, err
		}
		out, err = s.d.Store.InsertRule(ctx, a.ID, r)
		return out, true, err
	})
	return out, err
}

// Put implements notify.QuietHours (brief M3-18 note 9 d): replaces the whole rule (an absent
// time_zone stores NULL); an unknown or foreign rule is not_found; equal values answer the stored
// rule without event and without a new version.
func (s Quiet) Put(ctx context.Context, ruleID ids.ID, in notify.QuietHoursInput) (notify.QuietHoursRule, error) {
	var out notify.QuietHoursRule
	err := s.write(ctx, in, func(ctx context.Context, a actor.Actor, r notify.QuietHoursRule) (notify.QuietHoursRule, bool, error) {
		stored, found, err := s.d.Store.Rule(ctx, a.ID, ruleID)
		if err != nil {
			return notify.QuietHoursRule{}, false, err
		}
		if !found {
			return notify.QuietHoursRule{}, false, notFound()
		}
		if domain.RuleUnchanged(ruleInput(stored), ruleInput(r)) {
			out = stored
			return stored, false, nil
		}
		r.ID, r.Version = ruleID, stored.Version+1
		rules, err := s.d.Store.Rules(ctx, a.ID)
		if err != nil {
			return notify.QuietHoursRule{}, false, err
		}
		if err := s.fits(replaceRule(rules, r)); err != nil {
			return notify.QuietHoursRule{}, false, err
		}
		out, found, err = s.d.Store.UpdateRule(ctx, a.ID, r)
		if err == nil && !found {
			err = errs.New(errs.CodeInternalError, "notify: a rule vanished inside its unit of work")
		}
		return out, true, err
	})
	return out, err
}

// Delete implements notify.QuietHours (brief M3-18 note 9 d): the rule goes and
// quiet_hours.updated carries the list without it; an unknown, foreign or already deleted rule is
// not_found (06 section 7: no tombstone).
func (s Quiet) Delete(ctx context.Context, ruleID ids.ID) error {
	a := caller(ctx)
	return s.d.UoW.Do(ctx, func(ctx context.Context) error {
		if err := s.lockRules(ctx, a); err != nil {
			return err
		}
		_, found, err := s.d.Store.DeleteRule(ctx, a.ID, ruleID)
		if err != nil {
			return err
		}
		if !found {
			return notFound()
		}
		return s.appendList(ctx, a.ID)
	})
}

// write runs one create or put: quiet_hours.update on the caller, the per-person lock, the
// validation, then change; when change reports a change, quiet_hours.updated with the complete
// list as of this unit of work.
func (s Quiet) write(ctx context.Context, in notify.QuietHoursInput,
	change func(ctx context.Context, a actor.Actor, r notify.QuietHoursRule) (notify.QuietHoursRule, bool, error)) error {
	a := caller(ctx)
	return s.d.UoW.Do(ctx, func(ctx context.Context) error {
		if err := s.lockRules(ctx, a); err != nil {
			return err
		}
		r, err := validRule(in)
		if err != nil {
			return err
		}
		_, changed, err := change(ctx, a, r)
		if err != nil || !changed {
			return err
		}
		return s.appendList(ctx, a.ID)
	})
}

// lockRules authorizes quiet_hours.update on the caller (agents and apps: forbidden
// subject_kind_not_allowed) and takes the per-person advisory lock before any rule is read
// (docs/DECISIONS.md D-M3-18-7).
func (s Quiet) lockRules(ctx context.Context, a actor.Actor) error {
	if err := s.check(ctx, a, authorize.ActionQuietHoursUpdate, self(a.ID)); err != nil {
		return err
	}
	return s.d.Store.LockRules(ctx, a.ID)
}

// appendList appends quiet_hours.updated with every rule of person, ascending by id.
func (s *Service) appendList(ctx context.Context, person ids.ID) error {
	rules, err := s.d.Store.Rules(ctx, person)
	if err != nil {
		return err
	}
	p, err := quietPayload(rules)
	if err != nil {
		return err
	}
	return s.d.Events.Append(ctx, personalEvent(ctx, authorize.EventQuietHoursUpdated, person, p))
}

// fits refuses a list whose quiet_hours.updated payload would not stay below
// sync.event_payload_max_bytes (04 payload rule 2; docs/DECISIONS.md D-M3-18-8), measured as the
// appender measures it.
func (s *Service) fits(rules []notify.QuietHoursRule) error {
	slices.SortFunc(rules, func(a, b notify.QuietHoursRule) int { return compareIDs(a.ID, b.ID) })
	p, err := quietPayload(rules)
	if err != nil {
		return err
	}
	if len(p) >= s.cfg.EventPayloadMaxBytes {
		return errs.New(errs.CodeValidationFailed, "too many quiet-hour rules").WithFields(errs.FieldError{
			Pointer: "", Code: domain.FieldLimitReached, Detail: "the list of rules would exceed sync.event_payload_max_bytes",
		})
	}
	return nil
}

// replaceRule returns rules with the rule of r.ID replaced by r.
func replaceRule(rules []notify.QuietHoursRule, r notify.QuietHoursRule) []notify.QuietHoursRule {
	out := slices.Clone(rules)
	for i := range out {
		if out[i].ID == r.ID {
			out[i] = r
		}
	}
	return out
}

// validRule checks in (domain.ValidateRuleInput, then the zone load) and returns the rule
// without id and version. Errors: validation_failed; a zone that does not load is
// unknown_time_zone on /time_zone.
func validRule(in notify.QuietHoursInput) (notify.QuietHoursRule, error) {
	if err := domain.ValidateRuleInput(domain.RuleInput{Weekdays: in.Weekdays, StartLocal: in.StartLocal, EndLocal: in.EndLocal,
		TimeZone: in.TimeZone, Enabled: in.Enabled}); err != nil {
		return notify.QuietHoursRule{}, err
	}
	r := notify.QuietHoursRule{Weekdays: in.Weekdays, StartLocal: in.StartLocal, EndLocal: in.EndLocal, Enabled: in.Enabled}
	if in.TimeZone != nil {
		if _, err := time.LoadLocation(*in.TimeZone); err != nil {
			return notify.QuietHoursRule{}, errs.New(errs.CodeValidationFailed, "the quiet-hour rule is invalid").WithFields(domain.UnknownTimeZone())
		}
		r.TimeZone = *in.TimeZone
	}
	return r, nil
}

// ruleInput converts a stored or validated rule for domain.RuleUnchanged; "" is an absent zone.
func ruleInput(r notify.QuietHoursRule) domain.RuleInput {
	in := domain.RuleInput{Weekdays: r.Weekdays, StartLocal: r.StartLocal, EndLocal: r.EndLocal, Enabled: r.Enabled}
	if r.TimeZone != "" {
		tz := r.TimeZone
		in.TimeZone = &tz
	}
	return in
}

// compareIDs orders ids as PostgreSQL orders uuid values: bytewise.
func compareIDs(a, b ids.ID) int { return bytes.Compare(a[:], b[:]) }
