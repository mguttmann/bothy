// SPDX-License-Identifier: Apache-2.0

// Package httpapi holds the REST handlers of the notify module as part of
// rest.StrictServerInterface: the seven operations of the group Preferences (M3-18) of
// docs/api/openapi.yaml. The use cases authorize; the handlers only convert and pass errors to the
// problem writer of the chain unchanged. Absent muted_until and time_zone are omitted, never null.
//
// Example:
//
//	h := httpapi.New(svc.Preferences(), svc.QuietHours())
//	built, err := routes.Build(wire.Handlers{Notify: h}, wire.RouteOptions(), cat)
package httpapi

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/notify"
)

// Preferences implements the REST operations of the notification levels and quiet hours.
type Preferences struct {
	prefs notify.Preferences
	quiet notify.QuietHours
}

// New returns the handlers over the ports of the levels p and the quiet hours q.
func New(p notify.Preferences, q notify.QuietHours) *Preferences {
	return &Preferences{prefs: p, quiet: q}
}

// ListMyNotificationPreferences implements rest.StrictServerInterface (GET
// /me/notification-preferences).
func (h *Preferences) ListMyNotificationPreferences(ctx context.Context, _ rest.ListMyNotificationPreferencesRequestObject) (rest.ListMyNotificationPreferencesResponseObject, error) {
	list, err := h.prefs.List(ctx)
	if err != nil {
		return nil, err
	}
	out := rest.NotificationPreferenceList{Items: make([]rest.NotificationPreference, 0, len(list))}
	for _, p := range list {
		out.Items = append(out.Items, preferenceREST(p))
	}
	return rest.ListMyNotificationPreferences200JSONResponse{Body: out}, nil
}

// PutNotificationPreference implements rest.StrictServerInterface (PUT
// /me/notification-preferences/{target_kind}/{target_id}).
func (h *Preferences) PutNotificationPreference(ctx context.Context, req rest.PutNotificationPreferenceRequestObject) (rest.PutNotificationPreferenceResponseObject, error) {
	cmd := notify.PutPreference{Target: notify.Target{Kind: string(req.TargetKind), ID: ids.ID(req.TargetId)}, Level: string(req.Body.Level)}
	if req.Body.MutedUntil != nil {
		cmd.MutedUntil = *req.Body.MutedUntil
	}
	p, err := h.prefs.Put(ctx, cmd)
	if err != nil {
		return nil, err
	}
	return rest.PutNotificationPreference200JSONResponse{Body: preferenceREST(p)}, nil
}

// DeleteNotificationPreference implements rest.StrictServerInterface (DELETE
// /me/notification-preferences/{target_kind}/{target_id}): 204, also without a stored row.
func (h *Preferences) DeleteNotificationPreference(ctx context.Context, req rest.DeleteNotificationPreferenceRequestObject) (rest.DeleteNotificationPreferenceResponseObject, error) {
	if err := h.prefs.Delete(ctx, notify.Target{Kind: string(req.TargetKind), ID: ids.ID(req.TargetId)}); err != nil {
		return nil, err
	}
	return rest.DeleteNotificationPreference204Response{}, nil
}

// ListMyQuietHours implements rest.StrictServerInterface (GET /me/quiet-hours).
func (h *Preferences) ListMyQuietHours(ctx context.Context, _ rest.ListMyQuietHoursRequestObject) (rest.ListMyQuietHoursResponseObject, error) {
	list, err := h.quiet.List(ctx)
	if err != nil {
		return nil, err
	}
	out := rest.QuietHoursList{Items: make([]rest.QuietHoursRule, 0, len(list))}
	for _, r := range list {
		out.Items = append(out.Items, ruleREST(r))
	}
	return rest.ListMyQuietHours200JSONResponse{Body: out}, nil
}

// CreateQuietHoursRule implements rest.StrictServerInterface (POST /me/quiet-hours); the chain
// answers a repeated Idempotency-Key with the stored response.
func (h *Preferences) CreateQuietHoursRule(ctx context.Context, req rest.CreateQuietHoursRuleRequestObject) (rest.CreateQuietHoursRuleResponseObject, error) {
	r, err := h.quiet.Create(ctx, ruleInput(*req.Body))
	if err != nil {
		return nil, err
	}
	return rest.CreateQuietHoursRule201JSONResponse{Body: ruleREST(r)}, nil
}

// PutQuietHoursRule implements rest.StrictServerInterface (PUT /me/quiet-hours/{rule_id}).
func (h *Preferences) PutQuietHoursRule(ctx context.Context, req rest.PutQuietHoursRuleRequestObject) (rest.PutQuietHoursRuleResponseObject, error) {
	r, err := h.quiet.Put(ctx, ids.ID(req.RuleId), ruleInput(*req.Body))
	if err != nil {
		return nil, err
	}
	return rest.PutQuietHoursRule200JSONResponse{Body: ruleREST(r)}, nil
}

// DeleteQuietHoursRule implements rest.StrictServerInterface (DELETE /me/quiet-hours/{rule_id}).
func (h *Preferences) DeleteQuietHoursRule(ctx context.Context, req rest.DeleteQuietHoursRuleRequestObject) (rest.DeleteQuietHoursRuleResponseObject, error) {
	if err := h.quiet.Delete(ctx, ids.ID(req.RuleId)); err != nil {
		return nil, err
	}
	return rest.DeleteQuietHoursRule204Response{}, nil
}

// preferenceREST converts a preference; a zero muted_until is omitted.
func preferenceREST(p notify.Preference) rest.NotificationPreference {
	out := rest.NotificationPreference{
		TargetKind: rest.NotificationPreferenceTargetKind(p.Target.Kind), TargetId: rest.Uuid(p.Target.ID),
		Level: rest.NotificationLevel(p.Level), Version: p.Version,
	}
	if !p.MutedUntil.IsZero() {
		t := p.MutedUntil.UTC().Truncate(time.Millisecond)
		out.MutedUntil = &t
	}
	return out
}

// ruleREST converts a rule; a NULL time zone is omitted.
func ruleREST(r notify.QuietHoursRule) rest.QuietHoursRule {
	out := rest.QuietHoursRule{Id: rest.Uuid(r.ID), Weekdays: r.Weekdays, StartLocal: r.StartLocal, EndLocal: r.EndLocal,
		Enabled: r.Enabled, Version: r.Version}
	if r.TimeZone != "" {
		tz := r.TimeZone
		out.TimeZone = &tz
	}
	return out
}

// ruleInput converts the body of a create or put.
func ruleInput(b rest.QuietHoursRuleInput) notify.QuietHoursInput {
	return notify.QuietHoursInput{Weekdays: b.Weekdays, StartLocal: b.StartLocal, EndLocal: b.EndLocal, TimeZone: b.TimeZone, Enabled: b.Enabled}
}
