// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/personaldata"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// The section keys notify writes into the export package (docs/spec/20-privacy.md section 5.2;
// schemas under docs/api/gdpr-export/v1/sections).
const (
	// SectionNotificationPreferences is the member of personal.json with the notification levels.
	SectionNotificationPreferences = "notification_preferences"
	// SectionQuietHours is the member of personal.json with the quiet-hour rules.
	SectionQuietHours = "quiet_hours"
	// SectionPushRegistrations is the member of devices.json with the push targets.
	SectionPushRegistrations = "push_registrations"
)

// PersonalRegistration is one push registration of a person as the export shows it: the
// metadata of the push target, never the token or the preview hash (20 section 5.2).
type PersonalRegistration struct {
	// ID is push_registrations.id; DeviceID the device it belongs to.
	ID, DeviceID ids.ID
	// Provider is apns, fcm, unifiedpush or wns; Environment production or sandbox; AppID the
	// bundle or package of the app.
	Provider, Environment, AppID string
	// CreatedAt is the registration time; LastSuccessAt the last accepted send and DisabledAt
	// the deactivation, the zero time when absent.
	CreatedAt, LastSuccessAt, DisabledAt time.Time
	// DisabledReason is set exactly when DisabledAt is.
	DisabledReason string
}

// PersonalRegistrations is the outgoing port of PersonalData over push_registrations
// (adapters/postgres.Registrations). Neither method authorizes: the privacy job selects the
// person.
type PersonalRegistrations interface {
	// OfUser returns every registration of user, active and disabled, ascending by id, without
	// token or hash. It reads in the ambient transaction of ctx when there is one.
	OfUser(ctx context.Context, user ids.ID) ([]PersonalRegistration, error)
	// DeleteOfUser removes every registration of user in the unit of work of ctx and returns
	// their number.
	DeleteOfUser(ctx context.Context, user ids.ID) (int64, error)
}

// PersonalDataDeps are the collaborators of PersonalData.
type PersonalDataDeps struct {
	// Store reads and deletes notification_preferences and quiet_hours (adapters/postgres.Repository).
	Store Store
	// Registrations reads and deletes push_registrations (adapters/postgres.Registrations).
	Registrations PersonalRegistrations
	// UoW runs every erase step of notify in its own transaction.
	UoW txn.UnitOfWork
	// Events appends notification_pref.updated and quiet_hours.updated for the removed rows.
	Events eventlog.Appender
}

// PersonalData is the participant of notify in the privacy requests (M7-06, kernel port
// personaldata, docs/spec/20-privacy.md sections 4.4 and 5.2). It reads and writes only the three
// tables of notify. It is safe for concurrent use.
type PersonalData struct {
	d PersonalDataDeps
}

// Compile-time check.
var _ personaldata.Participant = (*PersonalData)(nil)

// NewPersonalData returns the participant of notify. internal/wire builds it with the repository
// and the registrations adapter over the application pool, the unit of work and the appender of
// sync. Errors: a missing collaborator.
func NewPersonalData(d PersonalDataDeps) (*PersonalData, error) {
	if d.Store == nil || d.Registrations == nil || d.UoW == nil || d.Events == nil {
		return nil, errors.New("notify: the personal data participant needs store, registrations, unit of work and appender")
	}
	return &PersonalData{d: d}, nil
}

// Module returns notify.
func (*PersonalData) Module() string { return "notify" }

// Tables returns the three tables of notify with personal data (20 section 2).
func (*PersonalData) Tables() []string {
	return []string{"notification_preferences", "quiet_hours", "push_registrations"}
}

// registrationJSON is one entry of devices.json push_registrations; absent members are omitted.
type registrationJSON struct {
	ID             string `json:"id"`
	DeviceID       string `json:"device_id"`
	Provider       string `json:"provider"`
	Environment    string `json:"environment"`
	AppID          string `json:"app_id"`
	State          string `json:"state"`
	CreatedAt      string `json:"created_at"`
	LastSuccessAt  string `json:"last_success_at,omitempty"`
	DisabledAt     string `json:"disabled_at,omitempty"`
	DisabledReason string `json:"disabled_reason,omitempty"`
}

// The values of registrationJSON.State.
const (
	registrationActive   = "active"
	registrationDisabled = "disabled"
)

// Export writes the notification levels and the quiet-hour rules of e.Subject into personal.json
// and the push registrations into devices.json, each an array (empty when nothing exists). The
// levels name targets only by kind and ID; the registrations carry no token and no hash. Every
// table is read with its own statement, so each read is a short transaction of its own.
//
// Errors: those of the storage and of w.
func (p *PersonalData) Export(ctx context.Context, e personaldata.Export, w personaldata.Writer) error {
	prefs, err := p.d.Store.Preferences(ctx, e.Subject)
	if err != nil {
		return err
	}
	prefList := make([]preferenceJSON, 0, len(prefs))
	for _, pr := range prefs {
		prefList = append(prefList, preferenceJSON{TargetKind: pr.Target.Kind, TargetID: pr.Target.ID.String(), Level: pr.Level,
			MutedUntil: Stamp(pr.MutedUntil), Version: pr.Version})
	}
	rules, err := p.d.Store.Rules(ctx, e.Subject)
	if err != nil {
		return err
	}
	ruleList := make([]ruleJSON, 0, len(rules))
	for _, r := range rules {
		ruleList = append(ruleList, ruleJSON{ID: r.ID.String(), Weekdays: r.Weekdays, StartLocal: r.StartLocal, EndLocal: r.EndLocal,
			TimeZone: r.TimeZone, Enabled: r.Enabled, Version: r.Version})
	}
	regs, err := p.d.Registrations.OfUser(ctx, e.Subject)
	if err != nil {
		return err
	}
	regList := make([]registrationJSON, 0, len(regs))
	for _, r := range regs {
		state := registrationActive
		if !r.DisabledAt.IsZero() {
			state = registrationDisabled
		}
		regList = append(regList, registrationJSON{ID: r.ID.String(), DeviceID: r.DeviceID.String(), Provider: r.Provider,
			Environment: r.Environment, AppID: r.AppID, State: state, CreatedAt: Stamp(r.CreatedAt),
			LastSuccessAt: Stamp(r.LastSuccessAt), DisabledAt: Stamp(r.DisabledAt), DisabledReason: r.DisabledReason})
	}
	if err := w.Section(personaldata.FilePersonal, SectionNotificationPreferences, prefList); err != nil {
		return err
	}
	if err := w.Section(personaldata.FilePersonal, SectionQuietHours, ruleList); err != nil {
		return err
	}
	return w.Section(personaldata.FileDevices, SectionPushRegistrations, regList)
}

// Erase runs the steps of the account deletion for the tables of notify (20 section 4.4), each in
// one unit of work, as the system: StepSecurity deletes the push registrations, StepPersonal the
// notification levels and quiet-hour rules; every other step is nil. A repeated step finds
// nothing left and appends nothing.
//
// Errors: those of the storage and of the appender (the job retries the step).
func (p *PersonalData) Erase(ctx context.Context, step personaldata.Step, e personaldata.Erasure) error {
	switch step {
	case personaldata.StepSecurity:
		// No event: removing a registration is invisible to clients, as unregisterPush is
		// (M3-22). notify runs before identity, which removes the devices in the same step;
		// the foreign keys would cascade the rows anyway.
		return p.d.UoW.Do(ctx, func(ctx context.Context) error {
			_, err := p.d.Registrations.DeleteOfUser(ctx, e.Subject)
			return err
		})
	case personaldata.StepPersonal:
		return p.d.UoW.Do(ctx, func(ctx context.Context) error {
			if err := p.erasePreferences(ctx, e.Subject); err != nil {
				return err
			}
			return p.eraseRules(ctx, e.Subject)
		})
	case personaldata.StepDisable, personaldata.StepAccess, personaldata.StepContent, personaldata.StepPseudonymize,
		personaldata.StepAudit:
		// notify has nothing to disable, no grants, no content and no clear data of the person.
		return nil
	default:
		return nil
	}
}

// erasePreferences deletes every notification level of user and appends, per removed row,
// notification_pref.updated with deleted and the last version plus one, as Prefs.Delete does, so
// the person's other clients drop the row.
func (p *PersonalData) erasePreferences(ctx context.Context, user ids.ID) error {
	prefs, err := p.d.Store.Preferences(ctx, user)
	if err != nil {
		return err
	}
	for _, pr := range prefs {
		old, found, err := p.d.Store.DeletePreference(ctx, user, pr.Target)
		if err != nil {
			return err
		}
		if !found {
			continue
		}
		old.Version++
		ev, err := prefEvent(ctx, user, old, true)
		if err != nil {
			return err
		}
		if err := p.d.Events.Append(ctx, asSystem(ev)); err != nil {
			return err
		}
	}
	return nil
}

// eraseRules deletes every quiet-hour rule of user under the per-person lock of the quiet-hour
// writes and appends one quiet_hours.updated with the now empty list when a rule went.
func (p *PersonalData) eraseRules(ctx context.Context, user ids.ID) error {
	if err := p.d.Store.LockRules(ctx, user); err != nil {
		return err
	}
	rules, err := p.d.Store.Rules(ctx, user)
	if err != nil || len(rules) == 0 {
		return err
	}
	for _, r := range rules {
		if _, _, err := p.d.Store.DeleteRule(ctx, user, r.ID); err != nil {
			return err
		}
	}
	payload, err := quietPayload(nil)
	if err != nil {
		return err
	}
	return p.d.Events.Append(ctx, asSystem(personalEvent(ctx, authorize.EventQuietHoursUpdated, user, payload)))
}

// asSystem marks ev as a system action (05 rule 10): the deletion runs for the privacy job, never
// with the rights or the name of a person, whatever actor ctx carries.
func asSystem(ev eventlog.Event) eventlog.Event {
	ev.ActorKind, ev.ActorPrincipalID = eventlog.ActorSystem, ids.ID{}
	return ev
}
