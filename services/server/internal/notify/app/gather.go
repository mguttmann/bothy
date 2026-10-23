// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"encoding/json"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/presence"
	"github.com/OWNER/bothy/services/server/internal/notify"
	"github.com/OWNER/bothy/services/server/internal/notify/domain"
)

// The personal switches of 13 3.3 the decision reads.
const (
	settingAdminEnabled    = "notify.admin_enabled"
	settingInputNeededLoud = "notify.input_needed_breaks_quiet_hours"
	skipReasonNoAccess     = "no_access"
	skipReasonPreference   = "preference"
	skipReasonActive       = "active_in_thread"
	skipReasonNoReg        = "no_registration"
	skipReasonRead         = "read"
	skipReasonExpired      = "expired"
	skipReasonNoProvider   = "no_provider"
	providerAPNs           = "apns"
	providerFCM            = "fcm"
	platformIOS            = "ios"
	platformAndroid        = "android"
)

// run decides the hints of one group of one event and inserts their jobs (13 12 points 2 to 5):
// only active humans are candidates, their facts are gathered in one batch, domain.Decide
// decides each of them. An error of Decide is a bug and rolls the round back (M2-01 note 7).
func (p *Push) run(ctx context.Context, g group) error {
	users, err := p.activeHumans(ctx, uniqueSorted(g.users))
	if err != nil || len(users) == 0 {
		return err
	}
	now := p.d.Clock.Now()
	facts, err := p.gather(ctx, g, users, now)
	if err != nil {
		return err
	}
	for _, u := range users {
		d, err := domain.Decide(facts[u])
		if err != nil {
			return err
		}
		h := Hint{RecipientID: u, ThreadID: g.threadID, MessageID: g.messageID, TargetID: g.targetID, Kind: g.kind, DueAt: g.ev.OccurredAt,
			MessageSeq: g.messageSeq}
		if err := p.apply(ctx, h, facts[u], d); err != nil {
			return err
		}
	}
	return nil
}

// gather returns the facts of every person of users for one hint of group g at now (design note
// 8): one call per port for all of them, the rights of push.deliver (or the recipient rule of
// R-AZ-30) and the window per person.
func (p *Push) gather(ctx context.Context, g group, users []ids.ID, now time.Time) (map[ids.ID]domain.RecipientFacts, error) {
	var levels map[ids.ID]domain.LevelFacts
	var err error
	if g.kind.HasThread() {
		if levels, err = p.d.Facts.LevelFacts(ctx, g.threadID, users); err != nil {
			return nil, err
		}
	}
	quiet, err := p.d.Facts.QuietRules(ctx, users)
	if err != nil {
		return nil, err
	}
	switches, err := p.switches(ctx, users)
	if err != nil {
		return nil, err
	}
	devices, err := p.devices(ctx, users, g.excludeDevice)
	if err != nil {
		return nil, err
	}
	policy := domain.Policy{CoalesceWindow: p.cfg.CoalesceWindow, MobileDelay: p.cfg.MobileDelay}
	out := make(map[ids.ID]domain.RecipientFacts, len(users))
	for _, u := range users {
		allowed, err := p.allowed(ctx, g, u)
		if err != nil {
			return nil, err
		}
		f := domain.RecipientFacts{
			Kind: g.kind, Now: now, Allowed: allowed, RunStarter: runKind(g.kind), Switches: switches[u],
			QuietHours: quiet[u], Devices: devices[u], Policy: policy,
		}
		if g.kind.HasThread() {
			if f, err = p.threadFacts(ctx, f, g, u, levels[u]); err != nil {
				return nil, err
			}
		}
		out[u] = f
	}
	return out, nil
}

// threadFacts adds the thread, the level facts and the window of user to f for a kind with
// thread. The window is one job query per person; a decision that ends before the window step
// (no right, level, active in the thread) does not read it (domain.NeedsWindow).
func (p *Push) threadFacts(ctx context.Context, f domain.RecipientFacts, g group, user ids.ID, level domain.LevelFacts) (domain.RecipientFacts, error) {
	f.ThreadID = g.threadID
	f.Level = level
	f.Level.DirectMessage = g.kind == domain.KindDM
	if !domain.NeedsWindow(f) {
		return f, nil
	}
	w, err := p.window(ctx, user, g.threadID, f.Now)
	if err != nil {
		return domain.RecipientFacts{}, err
	}
	f.Window = w
	return f, nil
}

// runKind reports the kinds of the run starter (RecipientFacts.RunStarter).
func runKind(k domain.Kind) bool {
	return k == domain.KindRunDone || k == domain.KindRunFailed || k == domain.KindInputNeeded
}

// allowed returns the fact Allowed of user for g (design note 7): push.deliver with the principal
// of the check without a request for kinds with thread (R-AZ-29, R-AZ-31), the recipient rule of
// R-AZ-30 for security and admin (the recipient is the audience of the event; it is an active
// human, which run checked before).
func (p *Push) allowed(ctx context.Context, g group, user ids.ID) (bool, error) {
	if !g.kind.HasThread() {
		return user == g.audience, nil
	}
	return p.canRead(ctx, user, g.threadID, g.channelID, g.spaceID)
}

// canRead decides push.deliver for user on the thread in filter mode (no denial audit), with the
// rights of this moment.
func (p *Push) canRead(ctx context.Context, user, thread, channel, space ids.ID) (bool, error) {
	d, err := p.d.Authorizer.Authorize(authorize.Filter(ctx), actor.Recheck(user), authorize.ActionPushDeliver,
		authorize.Resource{Kind: authorize.ResourceThread, ID: thread, ChannelID: channel, SpaceID: space})
	if err != nil {
		return false, err
	}
	return d.Allowed(), nil
}

// switches returns the personal switches of users with their defaults; a stored value that is no
// boolean counts as unset and logs one Warn line without person data (D-M3-19-6).
func (p *Push) switches(ctx context.Context, users []ids.ID) (map[ids.ID]domain.Switches, error) {
	vals, err := p.d.Settings.ValuesFor(ctx, users, []string{settingAdminEnabled, settingInputNeededLoud})
	if err != nil {
		return nil, err
	}
	out := make(map[ids.ID]domain.Switches, len(users))
	invalid := false
	for _, u := range users {
		admin, ok1 := boolSetting(vals[u], settingAdminEnabled, p.cfg.DefaultAdminEnabled)
		loud, ok2 := boolSetting(vals[u], settingInputNeededLoud, p.cfg.DefaultInputNeededBreaksQuietHours)
		invalid = invalid || !ok1 || !ok2
		out[u] = domain.Switches{AdminEnabled: admin, InputNeededBreaksQuietHours: loud}
	}
	if invalid {
		p.d.Log.WarnContext(ctx, "notify: a stored push switch is not a boolean and counts as unset")
	}
	return out, nil
}

// boolSetting reads key of kv; an absent key gives def and ok, a value that is no boolean def and
// not ok.
func boolSetting(kv map[string]json.RawMessage, key string, def bool) (bool, bool) {
	raw, present := kv[key]
	if !present {
		return def, true
	}
	var b bool
	if json.Unmarshal(raw, &b) != nil {
		return def, false
	}
	return b, true
}

// devices returns the active devices of users with their active registrations and live
// connections (design notes 8 and 10), without the device exclude (the pending device of a
// security hint). Registrations of devices that are not active and connections of unknown
// devices are dropped.
func (p *Push) devices(ctx context.Context, users []ids.ID, exclude ids.ID) (map[ids.ID][]domain.Device, error) {
	refs, err := p.d.Devices.ActiveForUsers(ctx, users)
	if err != nil {
		return nil, err
	}
	regs, err := p.d.Registrations.Active(ctx, users)
	if err != nil {
		return nil, err
	}
	conns, err := p.d.Presence.Connections(ctx, users)
	if err != nil {
		return nil, err
	}
	out := make(map[ids.ID][]domain.Device, len(users))
	for _, u := range users {
		for _, ref := range refs[u] {
			if ref.ID == exclude {
				continue
			}
			d := domain.Device{ID: ref.ID, Platform: domain.Platform(ref.Platform), Registrations: registrationsOf(regs[u], ref.ID)}
			if c, ok := connectionOf(conns[u], ref.ID); ok {
				d.Connection = &domain.Connection{Focus: c.Focus, Idle: c.Idle}
			}
			out[u] = append(out[u], d)
		}
	}
	return out, nil
}

// registrationsOf returns the IDs of the registrations of device, in the given order.
func registrationsOf(regs []notify.PushRegistration, device ids.ID) []ids.ID {
	var out []ids.ID
	for _, r := range regs {
		if r.DeviceID == device {
			out = append(out, r.ID)
		}
	}
	return out
}

// connectionOf returns the connection of device.
func connectionOf(conns []presence.Connection, device ids.ID) (presence.Connection, bool) {
	for _, c := range conns {
		if c.DeviceID == device {
			return c, true
		}
	}
	return presence.Connection{}, false
}

// apply carries out decision d of hint h (13 12 points 4 and 5): none counts its skip reason,
// collect inserts the flush job of the open window, send opens the next window and inserts one
// notify.push_send per registration of each push route.
func (p *Push) apply(ctx context.Context, h Hint, f domain.RecipientFacts, d domain.Decision) error {
	switch d.Outcome {
	case domain.OutcomeNone:
		p.skipped(ctx, d.Skip)
		return nil
	case domain.OutcomeCollect:
		_, err := p.d.Scheduler.Collect(ctx, h.RecipientID, h.ThreadID, f.Window.End)
		return err
	case domain.OutcomeSend:
		h.Quiet, h.Count = d.Quiet, d.Count
		return p.send(ctx, h, d)
	}
	return internal("notify: a decision with an unknown outcome")
}

// send opens the window a send opens (design note 9 c, the marker keyed by its end) and delivers
// h along the routes of d. One NotificationID per hint and recipient is
// shared by the jobs of its registrations.
func (p *Push) send(ctx context.Context, h Hint, d domain.Decision) error {
	if !d.OpensWindowUntil.IsZero() {
		if _, _, err := p.d.Scheduler.OpenWindow(ctx, h.RecipientID, h.ThreadID, d.OpensWindowUntil); err != nil {
			return err
		}
	}
	h.NotificationID = ids.New()
	for _, r := range d.Routes {
		var err error
		switch r.Via {
		case domain.ViaPush:
			err = p.enqueue(ctx, h, r.Registrations)
		case domain.ViaGateway:
			err = p.d.Routes.Gateway(ctx, h, r)
		case domain.ViaPushDelayed:
			err = p.d.Routes.Delayed(ctx, h, r)
		default:
			err = internal("notify: a route with an unknown path")
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// enqueue inserts one notify.push_send of h per registration, due now.
func (p *Push) enqueue(ctx context.Context, h Hint, regs []ids.ID) error {
	for _, reg := range regs {
		job := PushJob{
			NotificationID: h.NotificationID, RegistrationID: reg, RecipientID: h.RecipientID, ThreadID: h.ThreadID,
			MessageID: h.MessageID, TargetID: h.TargetID, Kind: h.Kind, Quiet: h.Quiet, Count: h.Count, DueAt: h.DueAt, Attempt: 1,
		}
		if err := p.d.Scheduler.Send(ctx, job, time.Time{}); err != nil {
			return err
		}
	}
	return nil
}

// skipped counts a decision none with the value of bothy.reason (design note 15); not_recipient
// and nothing_collected are not counted, because no hint existed for anyone.
func (p *Push) skipped(ctx context.Context, r domain.SkipReason) {
	if v := skipValue(r); v != "" {
		p.d.Metrics.Skipped(ctx, v)
	}
}

// skipValue maps a skip reason of the decision onto bothy.reason.
func skipValue(r domain.SkipReason) string {
	switch r {
	case domain.SkipNotAllowed:
		return skipReasonNoAccess
	case domain.SkipLevel, domain.SkipAdminDisabled:
		return skipReasonPreference
	case domain.SkipActiveInThread:
		return skipReasonActive
	case domain.SkipNoRoute:
		return skipReasonNoReg
	case domain.SkipRead:
		return skipReasonRead
	case domain.SkipNotRecipient, domain.SkipNothingCollected:
	}
	return ""
}
