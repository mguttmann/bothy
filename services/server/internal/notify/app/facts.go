// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"encoding/json"
	"slices"
	"time"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/notify"
	"github.com/OWNER/bothy/services/server/internal/notify/domain"
)

// settingDefaultLevel is the key of the personal default of 13 section 3.3.
const settingDefaultLevel = "notify.default_level"

// LevelFacts gathers the facts of 13 section 3.1 for the thread threadID and the people userIDs
// (brief M3-18 note 12, docs/DECISIONS.md D-M3-18-9): one path read through chat, one query on
// notification_preferences for all people and the three targets (13 section 12 point 2), one read
// of the stored notify.default_level through sync and the value of notify.user_default_level. It
// never resolves the level: domain.ResolveLevel does, at the time the caller passes. Every
// requested person gets an entry, unknown people without own rows. A stored personal default
// outside all, mentions and mute counts as unset and logs one Warn line without the value and
// without the person. It does not authorize: the consumer push of M3-19 asks for people it
// already selected.
//
// Errors: not_found for an unknown thread; the errors of the reads.
func (f Facts) LevelFacts(ctx context.Context, threadID ids.ID, userIDs []ids.ID) (map[ids.ID]domain.LevelFacts, error) {
	path, err := f.d.Paths.NotificationPath(ctx, chat.TargetThread, threadID)
	if err != nil {
		return nil, err
	}
	out := make(map[ids.ID]domain.LevelFacts, len(userIDs))
	base := domain.LevelFacts{
		ChannelDefault: domain.Level(path.DefaultNotificationLevel), DirectMessage: path.ChannelKind == "dm",
		InstanceDefault: domain.Level(f.cfg.UserDefaultLevel),
	}
	for _, u := range userIDs {
		out[u] = base
	}
	if len(userIDs) == 0 {
		return out, nil
	}
	rows, err := f.d.Store.LevelRows(ctx, userIDs, path.ThreadID, path.ChannelID, path.SpaceID)
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		if lf, ok := out[r.UserID]; ok {
			out[r.UserID] = withRow(lf, path, r)
		}
	}
	vals, err := f.d.Settings.ValuesFor(ctx, userIDs, []string{settingDefaultLevel})
	if err != nil {
		return nil, err
	}
	invalid := false
	for u, kv := range vals {
		lf, known := out[u]
		if !known {
			continue
		}
		level, ok := personalDefault(kv)
		invalid = invalid || !ok
		lf.PersonalDefault = level
		out[u] = lf
	}
	if invalid {
		f.d.Log.WarnContext(ctx, "notify: a stored notify.default_level is not a level and counts as unset")
	}
	return out, nil
}

// withRow puts the own row r into the slot of lf its target names on path.
func withRow(lf domain.LevelFacts, path chat.NotificationPath, r PreferenceRow) domain.LevelFacts {
	p := &domain.Preference{Level: domain.Level(r.Level), MutedUntil: r.MutedUntil}
	switch {
	case r.Target.Kind == chat.TargetThread && r.Target.ID == path.ThreadID:
		lf.Thread = p
	case r.Target.Kind == chat.TargetChannel && r.Target.ID == path.ChannelID:
		lf.Channel = p
	case r.Target.Kind == chat.TargetSpace && !path.SpaceID.IsZero() && r.Target.ID == path.SpaceID:
		lf.Space = p
	}
	return lf
}

// personalDefault reads the stored notify.default_level of kv; ok is false for a stored value
// that is not a level (it then counts as unset). An absent key is "" and ok.
func personalDefault(kv map[string]json.RawMessage) (domain.Level, bool) {
	raw, present := kv[settingDefaultLevel]
	if !present {
		return "", true
	}
	var s string
	if json.Unmarshal(raw, &s) != nil || !domain.Level(s).Valid() {
		return "", false
	}
	return domain.Level(s), true
}

// QuietRules returns the quiet-hour rules of the people userIDs with their zones loaded (brief
// M3-18 note 13): every rule, enabled or not (domain.InQuietHours ignores disabled ones), with
// Location from quiet_hours.time_zone or, when NULL, from users.time_zone through identity. Each
// zone name loads once per call. A name that no longer loads counts as UTC and logs one Warn line
// without the name and without any id (docs/DECISIONS.md D-M3-18-8). People without rules are
// absent. It does not authorize.
//
// Errors: the errors of the reads.
func (f Facts) QuietRules(ctx context.Context, userIDs []ids.ID) (map[ids.ID][]domain.QuietRule, error) {
	out := map[ids.ID][]domain.QuietRule{}
	if len(userIDs) == 0 {
		return out, nil
	}
	stored, err := f.d.Store.RulesOfUsers(ctx, userIDs)
	if err != nil {
		return nil, err
	}
	people, err := f.d.Users.Users(ctx, needingZone(stored))
	if err != nil {
		return nil, err
	}
	zones := &zoneCache{m: map[string]*time.Location{}}
	for u, rules := range stored {
		list := make([]domain.QuietRule, 0, len(rules))
		for _, r := range rules {
			name := r.TimeZone
			if name == "" {
				name = people[u].TimeZone
			}
			qr, err := quietRule(r, zones.load(name))
			if err != nil {
				return nil, err
			}
			list = append(list, qr)
		}
		out[u] = list
	}
	failed := zones.failed
	if failed {
		f.d.Log.WarnContext(ctx, "notify: a stored time zone no longer loads and counts as UTC")
	}
	return out, nil
}

// needingZone returns the people with at least one rule without its own zone.
func needingZone(stored map[ids.ID][]notify.QuietHoursRule) []ids.ID {
	var out []ids.ID
	for u, rules := range stored {
		if slices.ContainsFunc(rules, func(r notify.QuietHoursRule) bool { return r.TimeZone == "" }) {
			out = append(out, u)
		}
	}
	return out
}

// zoneCache loads each zone name once per call; a name that does not load is UTC and sets failed.
type zoneCache struct {
	m      map[string]*time.Location
	failed bool
}

// load returns the zone of name.
func (z *zoneCache) load(name string) *time.Location {
	if loc, ok := z.m[name]; ok {
		return loc
	}
	loc, err := time.LoadLocation(name)
	if err != nil || name == "" || name == "Local" {
		loc, z.failed = time.UTC, true
	}
	z.m[name] = loc
	return loc
}

// quietRule converts a stored rule for domain.InQuietHours.
func quietRule(r notify.QuietHoursRule, loc *time.Location) (domain.QuietRule, error) {
	start, err1 := domain.ParseLocalTime(r.StartLocal)
	end, err2 := domain.ParseLocalTime(r.EndLocal)
	if err1 != nil || err2 != nil || r.Weekdays < 1 || r.Weekdays > 127 {
		return domain.QuietRule{}, errs.New(errs.CodeInternalError, "notify: a stored quiet-hour rule breaks the table checks")
	}
	return domain.QuietRule{Weekdays: uint8(r.Weekdays), Start: start, End: end, Location: loc, Enabled: r.Enabled}, nil
}
