// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"encoding/json"
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
		lf, ok := out[r.UserID]
		if !ok {
			continue
		}
		p := &domain.Preference{Level: domain.Level(r.Level), MutedUntil: r.MutedUntil}
		switch {
		case r.Target.Kind == chat.TargetThread && r.Target.ID == path.ThreadID:
			lf.Thread = p
		case r.Target.Kind == chat.TargetChannel && r.Target.ID == path.ChannelID:
			lf.Channel = p
		case r.Target.Kind == chat.TargetSpace && !path.SpaceID.IsZero() && r.Target.ID == path.SpaceID:
			lf.Space = p
		}
		out[r.UserID] = lf
	}
	vals, err := f.d.Settings.ValuesFor(ctx, userIDs, []string{settingDefaultLevel})
	if err != nil {
		return nil, err
	}
	invalid := false
	for u, kv := range vals {
		raw, ok := kv[settingDefaultLevel]
		lf, known := out[u]
		if !ok || !known {
			continue
		}
		var s string
		if json.Unmarshal(raw, &s) != nil || !domain.Level(s).Valid() {
			invalid = true
			continue
		}
		lf.PersonalDefault = domain.Level(s)
		out[u] = lf
	}
	if invalid {
		f.d.Log.WarnContext(ctx, "notify: a stored notify.default_level is not a level and counts as unset")
	}
	return out, nil
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
	var needZone []ids.ID
	for u, rules := range stored {
		for _, r := range rules {
			if r.TimeZone == "" {
				needZone = append(needZone, u)
				break
			}
		}
	}
	people, err := f.d.Users.Users(ctx, needZone)
	if err != nil {
		return nil, err
	}
	zones, failed := map[string]*time.Location{}, false
	load := func(name string) *time.Location {
		if loc, ok := zones[name]; ok {
			return loc
		}
		loc, err := time.LoadLocation(name)
		if err != nil || name == "" || name == "Local" {
			loc, failed = time.UTC, true
		}
		zones[name] = loc
		return loc
	}
	for u, rules := range stored {
		list := make([]domain.QuietRule, 0, len(rules))
		for _, r := range rules {
			name := r.TimeZone
			if name == "" {
				name = people[u].TimeZone
			}
			qr, err := quietRule(r, load(name))
			if err != nil {
				return nil, err
			}
			list = append(list, qr)
		}
		out[u] = list
	}
	if failed {
		f.d.Log.WarnContext(ctx, "notify: a stored time zone no longer loads and counts as UTC")
	}
	return out, nil
}

// quietRule converts a stored rule for domain.InQuietHours.
func quietRule(r notify.QuietHoursRule, loc *time.Location) (domain.QuietRule, error) {
	start, err1 := domain.ParseLocalTime(r.StartLocal)
	end, err2 := domain.ParseLocalTime(r.EndLocal)
	if err1 != nil || err2 != nil || r.Weekdays < 1 || r.Weekdays > 127 {
		return domain.QuietRule{}, errs.New(errs.CodeInternalError, "notify: a stored quiet-hour rule breaks the table checks")
	}
	return domain.QuietRule{Weekdays: uint8(r.Weekdays), Start: start, End: end, Location: loc, Enabled: r.Enabled}, nil //nolint:gosec // G115: 1 to 127 checked above
}
