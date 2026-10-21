// SPDX-License-Identifier: Apache-2.0

package notify_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/pb33f/libopenapi"
	"github.com/pb33f/libopenapi-validator/schema_validation"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/adapters/identitytest"
	identityapp "github.com/OWNER/bothy/services/server/internal/identity/app"
	instanceapp "github.com/OWNER/bothy/services/server/internal/instance/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/jose"
	"github.com/OWNER/bothy/services/server/internal/notify/domain"
	"github.com/OWNER/bothy/services/server/internal/platform/secrets"
	syncpkg "github.com/OWNER/bothy/services/server/internal/sync"
	syncpg "github.com/OWNER/bothy/services/server/internal/sync/adapters/postgres"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

// validateSchema checks the JSON document doc against the schema name of docs/api/openapi.yaml.
func validateSchema(t *testing.T, name string, doc []byte) {
	t.Helper()
	d, err := libopenapi.NewDocument(rest.Spec)
	must(t, err)
	model, err := d.BuildV3Model()
	must(t, err)
	proxy, ok := model.Model.Components.Schemas.Get(name)
	if !ok {
		t.Fatalf("no schema %s", name)
	}
	if valid, problems := schema_validation.NewSchemaValidator().ValidateSchemaString(proxy.Schema(), string(doc)); !valid {
		for _, p := range problems {
			t.Errorf("schema %s: %s %s", name, p.Message, p.Reason)
		}
		t.Fatalf("document does not match %s: %s", name, doc)
	}
}

// expiring appends one access.expiring of the grant to the audience to through the sync appender
// in a unit of work, as the reminder job of authz writes it (resource and audience the recipient,
// receive action self, no actor, payload AccessExpiringPayload): M1-30 is not in the closure of
// this ticket (brief M3-19 "Tests").
func (w *world) expiring(to, subject, grant, space ids.ID, level string) {
	w.t.Helper()
	payload, err := json.Marshal(map[string]any{
		"kind": "grant", "grant_id": grant.String(), "subject_id": subject.String(),
		"target":     map[string]any{"kind": "space", "id": space.String(), "level": level, "capabilities": []string{}},
		"expires_at": w.clk.Now().Add(72 * time.Hour).UTC().Format(time.RFC3339),
	})
	must(w.t, err)
	validateSchema(w.t, "AccessExpiringPayload", payload)
	must(w.t, w.uow.Do(context.Background(), func(ctx context.Context) error {
		return w.appender.Append(ctx, eventlog.Event{
			Type: string(authorize.EventAccessExpiring), SchemaVersion: 1, ResourceKind: "principal", ResourceID: to,
			AudiencePrincipalID: to, ReceiveAction: "self", Payload: payload,
		})
	}))
}

// TestAdminHintFromAccessExpiring (13 section 2 row admin, 07 9.4, design note 4 c, R-AZ-30): an
// appended access.expiring to Alex gives one admin hint with the grant_id as target and no thread
// or message; with notify.admin_enabled = false stored for Alex none (bothy.push.skipped
// preference); an access.expiring to Lotse gives none.
func TestAdminHintFromAccessExpiring(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	w.expiring(w.alex, w.alex, w.g1, w.support, "trigger")
	w.consume()
	hints := w.hintList(w.alex)
	if len(hints) != 1 {
		t.Fatalf("Alex has %d hints, want 1: %+v", len(hints), hints)
	}
	if h := hints[0]; h.Kind != domain.KindAdmin || h.Target != w.g1 || !h.Thread.IsZero() || !h.Message.IsZero() ||
		h.Registration != w.alexPhone.Registration {
		t.Fatalf("Alex's hint %+v, want admin on G-1 without thread for his phone", h)
	}
	if jobs := w.pushJobs(); len(jobs) != 1 {
		t.Fatalf("push jobs %+v, want only Alex's", jobs)
	}

	w.setMySetting("alex", "notify.admin_enabled", "false")
	before := w.skipped("preference")
	w.expiring(w.alex, w.alex, w.g1, w.support, "trigger")
	w.consume()
	if n := len(w.hintList(w.alex)); n != 1 {
		t.Fatalf("Alex has %d hints with notify.admin_enabled false, want still 1", n)
	}
	if got := w.skipped("preference") - before; got != 1 {
		t.Fatalf("bothy.push.skipped preference rose by %d, want 1", got)
	}

	w.expiring(w.lotse, w.lotse, w.g5, w.support, "write")
	w.consume()
	if jobs := w.pushJobs(); len(jobs) != 1 {
		t.Fatalf("push jobs after the reminder to Lotse %+v, want still only Alex's first", jobs)
	}
}

// bindLogins returns the login service of identity (M1-37) as serve builds it: a second identity
// module over the pool, the unit of work, the authorizer, the audit recorder and the sync appender
// of the world, with the session use cases bound (BindSessions) over real signing keys in a key
// store of the test. The world's identity module stays unbound: its configuration lacks
// auth.console_session_max_age, auth.console_session_idle_timeout and authz.view_as_max_duration,
// which BindSessions needs. Devices and events land in the same tables either way.
func (w *world) bindLogins() *identityapp.Service {
	w.t.Helper()
	cfg := idConfig
	cfg.ConsoleSessionMaxAge, cfg.ConsoleSessionIdleTimeout, cfg.ViewAsMaxDuration = 12*time.Hour, 30*time.Minute, 15*time.Minute
	dir := filepath.Join(w.t.TempDir(), "signing-keys")
	must(w.t, os.Mkdir(dir, 0o700))
	sp, err := secrets.NewProvider(secrets.Options{Dir: filepath.Dir(dir), SigningKeyDir: dir, CacheTTL: time.Minute})
	must(w.t, err)
	keys, err := wire.NewSigningKeys(wire.SigningKeyConfig{Keys: instanceapp.KeysConfig{
		CacheTTL: time.Minute, InvocationRotation: 30 * 24 * time.Hour, AccessTokenRotation: 30 * 24 * time.Hour,
		JWKSMaxAge: time.Hour, AccessTokenTTL: idConfig.AccessTokenTTL, ClockLeeway: idConfig.ClockLeeway,
	}, Dir: dir}, w.pool, sp, w.uow, w.az, w.rec, w.clk, slog.New(slog.DiscardHandler))
	must(w.t, err)
	must(w.t, keys.Keys.EnsureActive(context.Background()))
	settings, err := wire.NewInstanceSettings(wire.DefaultInstanceConfig(), true, w.pool, w.uow, w.az,
		wire.SettingsWrites{Audit: w.rec, Events: w.appender}, w.clk)
	must(w.t, err)
	id, err := wire.NewIdentity(cfg, w.pool, w.uow, w.az, wire.IdentityWrites{Audit: w.rec, Events: w.appender, Settings: settings.Admin,
		Flags: settings.Admin}, pepper, w.clk, slog.New(slog.DiscardHandler))
	must(w.t, err)
	must(w.t, id.BindSessions(keys.Keys, w.instanceID))
	return id.Logins
}

// login completes a login of user through logins from a new device of platform through CompleteLogin with trust
// (the passkey path for TrustUnconfirmed, an enrollment code for TrustConfirmed) and returns the
// result.
func (w *world) login(logins *identityapp.Service, user ids.ID, platform identity.Platform, trust identityapp.DeviceTrust) identityapp.LoginResult {
	w.t.Helper()
	key, pub := identitytest.NewDeviceKey(jose.AlgES256)
	method := identity.AuthPasskey
	if trust == identityapp.TrustConfirmed {
		method = identity.AuthEnrollmentCode
	}
	res, err := logins.CompleteLogin(context.Background(), identityapp.LoginCompletion{
		UserID: user, AuthMethod: method, ProofJKT: key.Thumbprint, Trust: trust,
		Device: identity.RegisterDevice{DeviceID: ids.New(), Name: "Tablet", Platform: platform, AppVersion: "1.4.0", DPoPKey: pub},
	})
	must(w.t, err)
	return res
}

// devicePending returns the device.pending events with audience user, in seq order, as the
// history of sync reads them.
func (w *world) devicePending(user ids.ID) []syncpkg.Event {
	w.t.Helper()
	evs, err := syncpg.NewRepository(w.pool, slog.New(slog.DiscardHandler)).PersonalEvents(context.Background(),
		syncpkg.PersonalQuery{AudienceID: user, Types: []string{string(authorize.EventDevicePending)}})
	must(w.t, err)
	return evs
}

// TestSecurityHintFromDevicePending (R-AZ-31, 13 section 2 row security, design note 4 d): Alex has
// the active phone; a login of Alex through CompleteLogin with TrustUnconfirmed registers a second
// device as pending (it gets a push registration as a fixture row, so the test can see that it
// is never a route). Its unit of work wrote exactly one device.pending with audience Alex,
// resource device and a payload that validates against DevicePendingEvent and holds only
// device_id and platform. The consumer gives Alex exactly one security hint with the device_id as
// target, one notify.push_send for the registration of Alex's phone and none for the pending
// device; Sam and the Owner get nothing. A device registered as active writes no device.pending.
// With Alex disabled before the job runs the job counts no_access.
func TestSecurityHintFromDevicePending(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	logins := w.bindLogins()
	head := w.head()
	res := w.login(logins, w.alex, identity.PlatformAndroid, identityapp.TrustUnconfirmed)
	pending := res.Device.ID
	if !res.NewDevice || res.Device.State != identity.DeviceStatePending {
		t.Fatalf("the login registered %+v (new %v), want a new pending device", res.Device, res.NewDevice)
	}
	// A registration on the pending device, so a route to it would show as a job.
	w.sql(`INSERT INTO push_registrations (id, user_id, device_id, auth_session_id, provider, token, app_id, created_at)
		VALUES ($1, $2, $3, $4, 'fcm', $5, 'com.example.bothy', $6)`,
		ids.New().String(), w.alex.String(), pending.String(), res.Session.ID.String(), "device-token-"+ids.New().String(), w.clk.Now())

	evs := w.devicePending(w.alex)
	if len(evs) != 1 {
		t.Fatalf("%d device.pending events of Alex, want 1", len(evs))
	}
	ev := evs[0]
	if ev.Seq <= head || ev.ResourceKind != "device" || ev.ResourceID != pending || ev.AudiencePrincipalID != w.alex ||
		ev.ReceiveAction != "self" || !ev.SpaceID.IsZero() || !ev.ChannelID.IsZero() || !ev.ThreadID.IsZero() {
		t.Fatalf("device.pending %+v", ev)
	}
	if n := w.count(`SELECT count(*) FROM event_log WHERE type = 'device.pending' AND seq > $1`, head); n != 1 {
		t.Fatalf("%d device.pending events in the log after the login, want 1", n)
	}
	env, err := json.Marshal(ev.Envelope(syncpkg.DeliveryFields{}))
	must(t, err)
	validateSchema(t, "DevicePendingEvent", env)
	var members map[string]json.RawMessage
	must(t, json.Unmarshal(ev.Payload, &members))
	keys := make([]string, 0, len(members))
	for k := range members {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	if !slices.Equal(keys, []string{"device_id", "platform"}) {
		t.Fatalf("payload members %v, want only device_id and platform: %s", keys, ev.Payload)
	}
	var body struct {
		DeviceID ids.ID `json:"device_id"`
		Platform string `json:"platform"`
	}
	must(t, json.Unmarshal(ev.Payload, &body))
	if body.DeviceID != pending || body.Platform != string(identity.PlatformAndroid) {
		t.Fatalf("payload %s, want device %s on android", ev.Payload, pending)
	}

	w.consume()
	hints := w.hintList(w.alex)
	if len(hints) != 1 {
		t.Fatalf("Alex has %d hints, want 1: %+v", len(hints), hints)
	}
	if h := hints[0]; h.Kind != domain.KindSecurity || h.Target != pending || !h.Thread.IsZero() || !h.Message.IsZero() {
		t.Fatalf("Alex's hint %+v, want security on the pending device without thread", h)
	}
	jobs := w.pushJobs()
	if len(jobs) != 1 || jobs[0].Args.RecipientID != w.alex || jobs[0].Args.RegistrationID != w.alexPhone.Registration {
		t.Fatalf("push jobs %+v, want exactly one for the registration of Alex's phone", jobs)
	}
	for _, who := range []ids.ID{w.sam, w.owner} {
		if got := w.pushJobsFor(who); len(got) != 0 {
			t.Fatalf("push jobs for someone else than Alex: %+v", got)
		}
	}

	// A device registered as active writes no device.pending and gives no hint.
	if act := w.login(logins, w.alex, identity.PlatformIOS, identityapp.TrustConfirmed); act.Device.State != identity.DeviceStateActive {
		t.Fatalf("the confirmed login registered %+v, want an active device", act.Device)
	}
	if n := len(w.devicePending(w.alex)); n != 1 {
		t.Fatalf("%d device.pending events after an active registration, want still 1", n)
	}
	w.consume()
	if n := len(w.pushJobs()); n != 1 {
		t.Fatalf("%d push jobs after an active registration, want still 1", n)
	}

	// Alex disabled before the job runs: only the status changes (the rights check of the job is
	// under test; the lifecycle use case would also revoke the devices, which the job reports as
	// no_registration).
	w.sql(`UPDATE principals SET status = 'disabled', disabled_at = now() WHERE id = $1`, w.alex.String())
	before := w.skipped("no_access")
	if err := w.send(jobs[0]); err != nil {
		t.Fatalf("the job failed: %v", err)
	}
	if calls := w.apns.Calls(); len(calls) != 0 {
		t.Fatalf("the provider was called for a disabled person: %v", calls)
	}
	if got := w.skipped("no_access") - before; got != 1 {
		t.Fatalf("bothy.push.skipped no_access rose by %d, want 1", got)
	}
}
