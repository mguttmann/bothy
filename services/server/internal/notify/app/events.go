// SPDX-License-Identifier: Apache-2.0

package app

import (
	"bytes"
	"context"
	"encoding/json"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/notify"
)

// The values of the personal events (05 section 6: resource user, receive action self).
const (
	receiveSelf      = "self"
	resourceUser     = "user"
	payloadSchemaOne = 1
)

// timestampLayout is the form of every API time (06 section 2): UTC, milliseconds, suffix Z.
const timestampLayout = "2006-01-02T15:04:05.000Z"

// Stamp formats t as the API does; the zero time is "" (absent).
func Stamp(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Truncate(time.Millisecond).Format(timestampLayout)
}

// preferenceJSON is the schema NotificationPreference; absent members are omitted, never null.
type preferenceJSON struct {
	TargetKind string `json:"target_kind"`
	TargetID   string `json:"target_id"`
	Level      string `json:"level"`
	MutedUntil string `json:"muted_until,omitempty"`
	Version    int64  `json:"version"`
}

// ruleJSON is the schema QuietHoursRule.
type ruleJSON struct {
	ID         string `json:"id"`
	Weekdays   int    `json:"weekdays"`
	StartLocal string `json:"start_local"`
	EndLocal   string `json:"end_local"`
	TimeZone   string `json:"time_zone,omitempty"`
	Enabled    bool   `json:"enabled"`
	Version    int64  `json:"version"`
}

// prefPayload encodes NotificationPrefPayload; deleted marks a removed row.
func prefPayload(p notify.Preference, deleted bool) ([]byte, error) {
	return marshal(struct {
		Preference preferenceJSON `json:"preference"`
		Deleted    bool           `json:"deleted,omitempty"`
	}{preferenceJSON{TargetKind: p.Target.Kind, TargetID: p.Target.ID.String(), Level: p.Level, MutedUntil: Stamp(p.MutedUntil), Version: p.Version}, deleted})
}

// quietPayload encodes QuietHoursPayload: the complete list rules, ascending by id as given.
func quietPayload(rules []notify.QuietHoursRule) ([]byte, error) {
	list := make([]ruleJSON, 0, len(rules))
	for _, r := range rules {
		list = append(list, ruleJSON{ID: r.ID.String(), Weekdays: r.Weekdays, StartLocal: r.StartLocal, EndLocal: r.EndLocal,
			TimeZone: r.TimeZone, Enabled: r.Enabled, Version: r.Version})
	}
	return marshal(struct {
		Rules []ruleJSON `json:"rules"`
	}{list})
}

// marshal encodes v without HTML escaping, as the appender stores it.
func marshal(v any) ([]byte, error) {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, errs.Wrap(errs.CodeInternalError, err, "notify: encoding an event payload")
	}
	return bytes.TrimRight(b.Bytes(), "\n"), nil
}

// personalEvent returns the durable event typ for the person with payload p (brief M3-18 note 8):
// resource user without a version, empty scope, audience the person, the actor of the unit of
// work in ctx (system when there is none).
func personalEvent(ctx context.Context, typ authorize.EventType, person ids.ID, p []byte) eventlog.Event {
	kind, id := eventActor(ctx)
	return eventlog.Event{
		Type: string(typ), SchemaVersion: payloadSchemaOne, ActorKind: kind, ActorPrincipalID: id,
		ResourceKind: resourceUser, ResourceID: person, AudiencePrincipalID: person, ReceiveAction: receiveSelf, Payload: p,
	}
}

// prefEvent returns notification_pref.updated of p for person.
func prefEvent(ctx context.Context, person ids.ID, p notify.Preference, deleted bool) (eventlog.Event, error) {
	payload, err := prefPayload(p, deleted)
	if err != nil {
		return eventlog.Event{}, err
	}
	return personalEvent(ctx, authorize.EventNotificationPrefUpdated, person, payload), nil
}

// eventActor returns the actor of the unit of work in ctx; system when there is none.
func eventActor(ctx context.Context) (eventlog.ActorKind, ids.ID) {
	a, ok := actor.FromContext(ctx)
	if !ok || a.ID.IsZero() {
		return eventlog.ActorSystem, ids.ID{}
	}
	switch a.Kind {
	case actor.KindUser:
		return eventlog.ActorUser, a.ID
	case actor.KindAgent:
		return eventlog.ActorAgent, a.ID
	case actor.KindApp:
		return eventlog.ActorApp, a.ID
	case actor.KindHost, actor.KindAnonymous:
	}
	return eventlog.ActorSystem, ids.ID{}
}
