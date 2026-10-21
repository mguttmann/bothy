// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"slices"
	"sync"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/app"
	"github.com/OWNER/bothy/services/server/internal/identity/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// events is a fake eventlog.Appender that keeps every appended event.
type events struct {
	mu   sync.Mutex
	list []eventlog.Event
}

// Append implements eventlog.Appender.
func (e *events) Append(_ context.Context, evs ...eventlog.Event) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.list = append(e.list, evs...)
	return nil
}

// of returns the appended events of type typ.
func (e *events) of(typ authorize.EventType) []eventlog.Event {
	e.mu.Lock()
	defer e.mu.Unlock()
	var out []eventlog.Event
	for _, ev := range e.list {
		if ev.Type == string(typ) {
			out = append(out, ev)
		}
	}
	return out
}

// wantDevicePendingPayload checks that payload is DevicePendingPayload of docs/api/openapi.yaml with
// exactly the members device_id and platform (no name, no key, no app version, R-AZ-31).
func wantDevicePendingPayload(t *testing.T, payload []byte, device ids.ID, platform identity.Platform) {
	t.Helper()
	var members map[string]json.RawMessage
	if err := json.Unmarshal(payload, &members); err != nil {
		t.Fatalf("payload %s: %v", payload, err)
	}
	keys := make([]string, 0, len(members))
	for k := range members {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	if !slices.Equal(keys, []string{"device_id", "platform"}) {
		t.Fatalf("payload members %v, want exactly device_id and platform", keys)
	}
	var body struct {
		DeviceID string `json:"device_id"`
		Platform string `json:"platform"`
	}
	if err := json.Unmarshal(payload, &body); err != nil {
		t.Fatal(err)
	}
	if got, err := ids.Parse(body.DeviceID); err != nil || got != device {
		t.Fatalf("device_id %q, want %s", body.DeviceID, device)
	}
	if body.Platform != string(platform) {
		t.Fatalf("platform %q, want %q", body.Platform, platform)
	}
}

func newDevicesWith(t *testing.T, f *fixture, ev eventlog.Appender) *app.Devices {
	t.Helper()
	d, err := app.NewDevices(app.Config{PageSizeDefault: 50, PageSizeMax: 200}, app.DevicesDeps{
		Store: f.repo, People: f.repo, UoW: f.uow, Authorizer: denyAll{}, Cursors: nopCursors{}, Events: ev,
	})
	if err != nil {
		t.Fatal(err)
	}
	return d
}

// TestRegister_PendingAppendsDevicePending (R-AZ-31): a pending registration appends exactly one
// device.pending with resource device, audience the person, receive action self and only
// device_id and platform; an active one appends none.
func TestRegister_PendingAppendsDevicePending(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	ev := &events{}
	devs := newDevicesWith(t, f, ev)
	ctx := context.Background()
	alex := f.add(domain.KindUser, "alex")
	r, _, _ := registration(alex.ID)
	if _, err := devs.Register(ctx, r); err != nil {
		t.Fatal(err)
	}
	got := ev.of(authorize.EventDevicePending)
	if len(got) != 1 {
		t.Fatalf("%d device.pending events, want 1", len(got))
	}
	e := got[0]
	if e.SchemaVersion != 1 || e.ResourceKind != string(authorize.ResourceDevice) || e.ResourceID != r.DeviceID ||
		e.AudiencePrincipalID != alex.ID || e.ReceiveAction != "self" || !e.SpaceID.IsZero() || !e.ChannelID.IsZero() || !e.ThreadID.IsZero() {
		t.Fatalf("device.pending = %+v", e)
	}
	wantDevicePendingPayload(t, e.Payload, r.DeviceID, identity.PlatformIOS)

	active, _, _ := registration(alex.ID)
	active.State, active.ApprovedBy = identity.DeviceStateActive, &alex.ID
	if _, err := devs.Register(ctx, active); err != nil {
		t.Fatal(err)
	}
	if n := len(ev.of(authorize.EventDevicePending)); n != 1 {
		t.Fatalf("%d device.pending events after an active registration, want still 1", n)
	}
}

// TestRegister_PendingWithoutAppenderFailsClosed: without an appender a pending registration is an
// internal error and stores nothing, because a pending device without its event would never give
// the security hint; an active registration needs no appender.
func TestRegister_PendingWithoutAppenderFailsClosed(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	devs := newDevicesWith(t, f, nil)
	ctx := context.Background()
	alex := f.add(domain.KindUser, "alex")
	r, _, _ := registration(alex.ID)
	if _, err := devs.Register(ctx, r); errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("pending without appender: err %v, want internal", err)
	}
	if _, found, err := f.repo.Device(ctx, r.DeviceID); err != nil || found {
		t.Fatalf("device stored after the refused registration: %v, %v", found, err)
	}
	active, _, _ := registration(alex.ID)
	active.State, active.ApprovedBy = identity.DeviceStateActive, &alex.ID
	if _, err := devs.Register(ctx, active); err != nil {
		t.Fatalf("active without appender: %v", err)
	}
}

// TestActiveForUsers: only devices.state = 'active', grouped by person, each list ascending by id;
// a person without an active device is absent.
func TestActiveForUsers(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	devs := newDevicesWith(t, f, &events{})
	ctx := context.Background()
	alex, sam, robin := f.add(domain.KindUser, "alex"), f.add(domain.KindUser, "sam"), f.add(domain.KindUser, "robin")
	register := func(user ids.ID, platform identity.Platform, state identity.DeviceState) ids.ID {
		t.Helper()
		r, _, _ := registration(user)
		r.Platform, r.State = platform, state
		if state == identity.DeviceStateActive {
			r.ApprovedBy = &user
		}
		if platform == identity.PlatformCLI {
			r.StepUpKey = nil
		}
		if _, err := devs.Register(ctx, r); err != nil {
			t.Fatal(err)
		}
		return r.DeviceID
	}
	alexPhone := register(alex.ID, identity.PlatformIOS, identity.DeviceStateActive)
	register(alex.ID, identity.PlatformMacOS, identity.DeviceStatePending)
	samPhone := register(sam.ID, identity.PlatformAndroid, identity.DeviceStateActive)
	samCLI := register(sam.ID, identity.PlatformCLI, identity.DeviceStateActive)
	register(robin.ID, identity.PlatformWeb, identity.DeviceStatePending)

	got, err := devs.ActiveForUsers(ctx, []ids.ID{alex.ID, sam.ID, robin.ID, ids.New()})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("ActiveForUsers = %v, want Alex and Sam only", got)
	}
	if want := []identity.DeviceRef{{ID: alexPhone, Platform: "ios"}}; !slices.Equal(got[alex.ID], want) {
		t.Fatalf("Alex = %v, want %v", got[alex.ID], want)
	}
	want := []identity.DeviceRef{{ID: samPhone, Platform: "android"}, {ID: samCLI, Platform: "cli"}}
	slices.SortFunc(want, func(a, b identity.DeviceRef) int { return bytes.Compare(a.ID[:], b.ID[:]) })
	if !slices.Equal(got[sam.ID], want) {
		t.Fatalf("Sam = %v, want %v", got[sam.ID], want)
	}
	if empty, err := devs.ActiveForUsers(ctx, nil); err != nil || len(empty) != 0 {
		t.Fatalf("ActiveForUsers(nil) = %v, %v", empty, err)
	}
}

// TestCompleteLogin_DevicePending (TestSecurityHintFromDevicePending, the identity part): a login
// with TrustUnconfirmed commits exactly one device.pending in its unit of work, with audience the
// person, resource device and only device_id and platform; a confirmed login (active device) writes
// none; a veto later in the unit removes the device and the event.
func TestCompleteLogin_DevicePending(t *testing.T) {
	t.Parallel()
	w := newPeopleWorld(t)
	id := w.identityOf()
	ctx := context.Background()
	alex := w.person("alex", "active", false)
	login := func(ctx context.Context, trust app.DeviceTrust) (ids.ID, error) {
		k := freshKey(t)
		dev := ids.New()
		_, err := id.Logins.CompleteLogin(ctx, app.LoginCompletion{
			UserID: alex.ID, AuthMethod: identity.AuthPasskey, ProofJKT: k.jkt, Trust: trust,
			Device: identity.RegisterDevice{DeviceID: dev, Name: "Alex Laptop", Platform: identity.PlatformMacOS, AppVersion: "2.0.0", DPoPKey: k.pub},
		})
		return dev, err
	}
	pendingEvents := func(dev ids.ID) int {
		return w.count(`SELECT count(*) FROM event_log WHERE type = 'device.pending' AND resource_id = $1`, dev.String())
	}

	pending, err := login(ctx, app.TrustUnconfirmed)
	if err != nil {
		t.Fatal(err)
	}
	if n := w.count(`SELECT count(*) FROM event_log WHERE type = 'device.pending'`); n != 1 {
		t.Fatalf("%d device.pending events after one pending login, want 1", n)
	}
	var kind, audience, receive string
	var payload []byte
	if err := w.owner.QueryRow(ctx, `SELECT resource_kind, audience_principal_id::text, receive_action, payload::text FROM event_log
		WHERE type = 'device.pending' AND resource_id = $1`, pending.String()).Scan(&kind, &audience, &receive, &payload); err != nil {
		t.Fatal(err)
	}
	if kind != "device" || audience != alex.ID.String() || receive != "self" {
		t.Fatalf("device.pending: resource_kind %q, audience %q, receive_action %q", kind, audience, receive)
	}
	wantDevicePendingPayload(t, payload, pending, identity.PlatformMacOS)

	active, err := login(ctx, app.TrustConfirmed)
	if err != nil {
		t.Fatal(err)
	}
	if n := pendingEvents(active); n != 0 {
		t.Fatalf("%d device.pending events for an active device, want 0", n)
	}

	errVeto := errors.New("veto")
	var vetoed ids.ID
	err = w.uow.Do(ctx, func(ctx context.Context) error {
		var err error
		if vetoed, err = login(ctx, app.TrustUnconfirmed); err != nil {
			return err
		}
		return errVeto
	})
	if !errors.Is(err, errVeto) {
		t.Fatalf("vetoed unit: %v", err)
	}
	if n := w.count(`SELECT count(*) FROM devices WHERE id = $1`, vetoed.String()) + pendingEvents(vetoed); n != 0 {
		t.Fatalf("%d device rows and events after the veto, want 0", n)
	}
}
