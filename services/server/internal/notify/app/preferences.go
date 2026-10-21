// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/notify"
	"github.com/OWNER/bothy/services/server/internal/notify/domain"
)

// ReasonTargetArchived is the problem reason of a preference on an archived target (brief M3-18
// note 4 c, docs/DECISIONS.md D-M3-18-2).
const ReasonTargetArchived = "target_archived"

// List implements notify.Preferences (brief M3-18 note 5 b): settings.read on the caller, then
// every own row whose target the caller can still read (filter mode, no denial audit). Rows of
// targets the caller lost stay stored and appear again when the right returns (04 section 14).
func (s Prefs) List(ctx context.Context) ([]notify.Preference, error) {
	a := caller(ctx)
	if err := s.check(ctx, a, authorize.ActionSettingsRead, self(a.ID)); err != nil {
		return nil, err
	}
	rows, err := s.d.Store.Preferences(ctx, a.ID)
	if err != nil {
		return nil, err
	}
	fctx := authorize.Filter(ctx)
	out := make([]notify.Preference, 0, len(rows))
	for _, p := range rows {
		ok, err := s.readable(fctx, a, p.Target)
		if err != nil {
			return nil, err
		}
		if ok {
			out = append(out, p)
		}
	}
	return out, nil
}

// readActions are the read actions of the target kinds of the list.
var readActions = map[string]authorize.Action{
	chat.TargetSpace: authorize.ActionSpaceRead, chat.TargetChannel: authorize.ActionChannelRead, chat.TargetThread: authorize.ActionThreadRead,
}

// readable reports whether a may read target; a target that no longer resolves is not readable.
func (s Prefs) readable(ctx context.Context, a actor.Actor, t notify.Target) (bool, error) {
	path, err := s.d.Paths.NotificationPath(ctx, t.Kind, t.ID)
	if errs.KindOf(err) == errs.KindNotFound {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	d, err := s.d.Authorizer.Authorize(ctx, a, readActions[t.Kind], resourceOf(path, t.ID, ids.ID{}))
	if err != nil {
		return false, err
	}
	return d.Allowed(), nil
}

// Put implements notify.Preferences (brief M3-18 note 4): lock the path, authorize
// notification_pref.update, refuse an archived target with conflict target_archived, then the
// single write path upsertPreference.
func (s Prefs) Put(ctx context.Context, cmd notify.PutPreference) (notify.Preference, error) {
	a := caller(ctx)
	if !domain.Level(cmd.Level).Valid() {
		return notify.Preference{}, errs.New(errs.CodeValidationFailed, "level").WithFields(errs.FieldError{
			Pointer: "/level", Code: "invalid", Detail: "level is all, mentions or mute",
		})
	}
	var out notify.Preference
	err := s.d.UoW.Do(ctx, func(ctx context.Context) error {
		path, err := s.lockTarget(ctx, a, cmd.Target)
		if err != nil {
			return err
		}
		if path.Archived {
			return errs.New(errs.CodeConflict, "the target is archived").WithReason(ReasonTargetArchived)
		}
		out, err = s.upsertPreference(ctx, a.ID, cmd.Target, cmd.Level, cmd.MutedUntil)
		return err
	})
	return out, err
}

// Delete implements notify.Preferences (brief M3-18 note 5 a): the path lock and authorize of
// Put (an archived target is allowed), then the delete; a removed row writes
// notification_pref.updated with the last version plus one and deleted, no row writes nothing. A
// later Put starts again at version 1 (docs/DECISIONS.md D-M3-18-4).
func (s Prefs) Delete(ctx context.Context, target notify.Target) error {
	a := caller(ctx)
	return s.d.UoW.Do(ctx, func(ctx context.Context) error {
		if _, err := s.lockTarget(ctx, a, target); err != nil {
			return err
		}
		old, found, err := s.d.Store.DeletePreference(ctx, a.ID, target)
		if err != nil || !found {
			return err
		}
		old.Version++
		e, err := prefEvent(ctx, a.ID, old, true)
		if err != nil {
			return err
		}
		return s.d.Events.Append(ctx, e)
	})
}

// lockTarget resolves the target with FOR SHARE on thread, channel and space (in that order,
// docs/DECISIONS.md D-M3-18-1) and authorizes notification_pref.update on it. An unknown kind or
// id, an id of another kind and a target without read right give the same not_found (AZ-P4); an
// agent or app gets forbidden subject_kind_not_allowed from authorize.
func (s Prefs) lockTarget(ctx context.Context, a actor.Actor, t notify.Target) (chat.NotificationPath, error) {
	if !domain.TargetKind(t.Kind).Valid() {
		return chat.NotificationPath{}, notFound()
	}
	path, err := s.d.Paths.LockNotificationPath(ctx, t.Kind, t.ID)
	if err != nil {
		return chat.NotificationPath{}, err
	}
	if err := s.check(ctx, a, authorize.ActionNotificationPrefUpdate, resourceOf(path, t.ID, a.ID)); err != nil {
		return chat.NotificationPath{}, err
	}
	return path, nil
}

// resourceOf is the authorize resource of the target id at path, owned by owner.
func resourceOf(p chat.NotificationPath, id, owner ids.ID) authorize.Resource {
	r := authorize.Resource{ID: id, SpaceID: p.SpaceID, OwnerID: owner}
	switch p.Kind {
	case chat.TargetSpace:
		r.Kind = authorize.ResourceSpace
	case chat.TargetChannel:
		r.Kind = authorize.ResourceChannel
	default:
		r.Kind, r.ChannelID = authorize.ResourceThread, p.ChannelID
	}
	return r
}

// upsertPreference is one of the two write paths of notification_preferences (brief M3-18 note
// 6; the other is insertPreferenceIfAbsent of the automatic following, M3-19):
// it cuts mutedUntil to whole microseconds, stores it as given (also when it has passed,
// docs/DECISIONS.md D-M3-18-3), writes with one statement and appends notification_pref.updated
// on a change. Equal stored values answer the stored row without event and without a new
// version. It neither authorizes nor locks the path: its caller Put does. It needs a unit of
// work.
func (s *Service) upsertPreference(ctx context.Context, user ids.ID, target notify.Target, level string, mutedUntil time.Time) (notify.Preference, error) {
	in := notify.Preference{Target: target, Level: level, MutedUntil: domain.TruncateMicro(mutedUntil).UTC()}
	if mutedUntil.IsZero() {
		in.MutedUntil = time.Time{}
	}
	row, changed, err := s.d.Store.UpsertPreference(ctx, user, in, s.d.Clock.Now().UTC())
	if err != nil {
		return notify.Preference{}, err
	}
	if !changed {
		stored, found, err := s.d.Store.Preference(ctx, user, target)
		if err != nil {
			return notify.Preference{}, err
		}
		if !found {
			return notify.Preference{}, errs.New(errs.CodeInternalError, "notify: a preference vanished inside its unit of work")
		}
		return stored, nil
	}
	e, err := prefEvent(ctx, user, row, false)
	if err != nil {
		return notify.Preference{}, err
	}
	return row, s.d.Events.Append(ctx, e)
}

// UpsertPreference exposes upsertPreference to the automatic following of M3-19, which runs in
// the transaction of the message and has authorized and locked the path itself.
func (s *Service) UpsertPreference(ctx context.Context, user ids.ID, target notify.Target, level string, mutedUntil time.Time) (notify.Preference, error) {
	return s.upsertPreference(ctx, user, target, level, mutedUntil)
}

// insertPreferenceIfAbsent is the second write path of notification_preferences (13 3.2, brief
// M3-19 design note 17, D-M3-19-14): it inserts level for user on target with version 1 at at
// only when the person has no own row on that target, and appends notification_pref.updated
// through events only for an inserted row. An upsert would overwrite an own mute or mentions,
// which 13 3.2 forbids ("no own setting for this thread yet"). It neither authorizes nor locks
// the path: the automatic following calls it in the unit of work of the message the person
// wrote. inserted is false when a row existed.
func insertPreferenceIfAbsent(ctx context.Context, store Store, events eventlog.Appender, at time.Time, user ids.ID, target notify.Target,
	level string) (notify.Preference, bool, error) {
	row, inserted, err := store.InsertPreferenceIfAbsent(ctx, user, notify.Preference{Target: target, Level: level}, at.UTC())
	if err != nil || !inserted {
		return notify.Preference{}, false, err
	}
	e, err := prefEvent(ctx, user, row, false)
	if err != nil {
		return notify.Preference{}, false, err
	}
	return row, true, events.Append(ctx, e)
}
