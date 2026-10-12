// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/adapters/identitytest"
	"github.com/OWNER/bothy/services/server/internal/identity/app"
	"github.com/OWNER/bothy/services/server/internal/identity/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/jose"
)

// Compile-time checks: the app types implement the device ports.
var (
	_ identity.Devices       = (*app.Devices)(nil)
	_ authz.CredentialSource = (*app.CredentialSource)(nil)
)

// denyAll is an Authorizer for the use cases of these tests that never authorize.
type denyAll struct{}

func (denyAll) Authorize(context.Context, authorize.Principal, authorize.Action, authorize.Resource) (authorize.Decision, error) {
	return authorize.Decision{Outcome: authorize.OutcomeNotFound, Reason: "no_grant"}, nil
}

func (denyAll) Scope(context.Context, authorize.Principal, authorize.Action) (authorize.Scope, error) {
	return authorize.Scope{}, nil
}

// nopCursors is a codec for use cases that list nothing in these tests.
type nopCursors struct{}

func (nopCursors) Encode(string, []byte, any) (string, error) { return "c", nil }
func (nopCursors) Decode(string, []byte, string, any) error   { return nil }

func newDevices(t *testing.T, f *fixture) *app.Devices {
	t.Helper()
	d, err := app.NewDevices(app.Config{PageSizeDefault: 50, PageSizeMax: 200}, app.DevicesDeps{
		Store: f.repo, People: f.repo, UoW: f.uow, Authorizer: denyAll{}, Cursors: nopCursors{},
	})
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func registration(user ids.ID) (identity.RegisterDevice, *identitytest.Key, *identitytest.Key) {
	dk, dpub := identitytest.NewDeviceKey(jose.AlgES256)
	sk, spub := identitytest.NewDeviceKey(jose.AlgEdDSA)
	dpub.Attestation = json.RawMessage(`{"fmt":"apple-appattest"}`)
	return identity.RegisterDevice{
		DeviceID: ids.New(), UserID: user, Name: "Alex iPhone", Platform: identity.PlatformIOS, AppVersion: "1.4.0",
		DPoPKey: dpub, StepUpKey: &spub, State: identity.DeviceStatePending,
	}, dk, sk
}

func TestRegister_Persists(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	devs := newDevices(t, f)
	ctx := context.Background()
	alex := f.add(domain.KindUser, "alex")
	r, dk, sk := registration(alex.ID)
	d, err := devs.Register(ctx, r)
	if err != nil {
		t.Fatal(err)
	}
	if d.ID != r.DeviceID || d.UserID != alex.ID || d.State != identity.DeviceStatePending || d.KeyThumbprint != dk.Thumbprint ||
		d.StepUpKeyThumbprint != sk.Thumbprint || d.CreatedAt.IsZero() || !d.ApprovedAt.IsZero() || d.Name != "Alex iPhone" {
		t.Fatalf("device %+v", d)
	}
	var att, jwk []byte
	var alg string
	if err := f.owner.QueryRow(ctx, `SELECT alg, public_jwk, attestation FROM device_keys WHERE jkt = $1`, dk.Thumbprint).Scan(&alg, &jwk, &att); err != nil {
		t.Fatal(err)
	}
	var stored map[string]any
	_ = json.Unmarshal(att, &stored)
	if alg != jose.AlgES256 || stored["fmt"] != "apple-appattest" || len(jwk) == 0 {
		t.Fatalf("stored key alg %s jwk %s attestation %s", alg, jwk, att)
	}
	k, err := devs.ActiveKey(ctx, d.ID, identity.KeyPurposeStepUp)
	if err != nil || k.JKT != sk.Thumbprint || k.Alg != jose.AlgEdDSA || k.DeviceID != d.ID || k.ID.IsZero() || !k.RevokedAt.IsZero() {
		t.Fatalf("ActiveKey(step_up) = %+v, %v", k, err)
	}

	// active sets approved_by and approved_at.
	owner := f.add(domain.KindUser, "sam")
	r2, _, _ := registration(alex.ID)
	r2.State, r2.ApprovedBy = identity.DeviceStateActive, &owner.ID
	d2, err := devs.Register(ctx, r2)
	if err != nil || d2.State != identity.DeviceStateActive || d2.ApprovedAt.IsZero() {
		t.Fatalf("active device %+v, %v", d2, err)
	}
	var by ids.ID
	if err := f.owner.QueryRow(ctx, `SELECT approved_by FROM devices WHERE id = $1`, d2.ID.String()).Scan(&by); err != nil || by != owner.ID {
		t.Fatalf("approved_by %v, %v", by, err)
	}

	// Duplicate device id and duplicate jkt: 409 conflict, nothing stored.
	dup, _, _ := registration(alex.ID)
	dup.DeviceID = d.ID
	if _, err := devs.Register(ctx, dup); errs.KindOf(err) != errs.KindConflict {
		t.Fatalf("duplicate device id: %v", err)
	}
	reuse, _, _ := registration(alex.ID)
	reuse.DPoPKey = r.DPoPKey
	if _, err := devs.Register(ctx, reuse); errs.KindOf(err) != errs.KindConflict {
		t.Fatalf("duplicate jkt: %v", err)
	}
	reuse2, _, _ := registration(alex.ID)
	reuse2.StepUpKey = r.StepUpKey
	if _, err := devs.Register(ctx, reuse2); errs.KindOf(err) != errs.KindConflict {
		t.Fatalf("duplicate step-up jkt: %v", err)
	}
	var n int
	if err := f.owner.QueryRow(ctx, `SELECT count(*) FROM devices`).Scan(&n); err != nil || n != 2 {
		t.Fatalf("%d devices after the conflicts, want 2 (a failed registration leaves nothing)", n)
	}
}

func TestRegister_Rules(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	devs := newDevices(t, f)
	ctx := context.Background()
	agent := f.add(domain.KindAgent, "helper")
	r, _, _ := registration(agent.ID)
	if _, err := devs.Register(ctx, r); errs.KindOf(err) != errs.KindNotFound {
		t.Fatalf("device for an agent: %v, want not_found", err)
	}
	r, _, _ = registration(ids.New())
	if _, err := devs.Register(ctx, r); errs.KindOf(err) != errs.KindNotFound {
		t.Fatalf("device for an unknown person: %v, want not_found", err)
	}
	alex := f.add(domain.KindUser, "alex")
	r, _, _ = registration(alex.ID)
	r.Name = "Alex\u202eiPhone"
	e, ok := errs.As(func() error { _, err := devs.Register(ctx, r); return err }())
	if !ok || e.PublicCode() != errs.CodeValidationFailed || len(e.Fields) != 1 || e.Fields[0].Pointer != "/name" {
		t.Fatalf("bidi name: %v", e)
	}
	r, _, _ = registration(alex.ID)
	r.Platform = identity.PlatformCLI
	if _, err := devs.Register(ctx, r); errs.KindOf(err) != errs.KindBadRequest {
		t.Fatalf("cli with a step-up key: %v", err)
	}
	if _, err := devs.ActiveKey(ctx, ids.New(), identity.KeyPurposeDPoP); errs.KindOf(err) != errs.KindNotFound {
		t.Fatalf("ActiveKey of an unknown device: %v", err)
	}
}

func TestRegister_JoinsTheAmbientUnit(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	devs := newDevices(t, f)
	alex := f.add(domain.KindUser, "alex")
	r, _, _ := registration(alex.ID)
	err := f.uow.Do(context.Background(), func(ctx context.Context) error {
		if _, err := devs.Register(ctx, r); err != nil {
			return err
		}
		return errs.New(errs.CodeConflict, "roll back")
	})
	if errs.KindOf(err) != errs.KindConflict {
		t.Fatal(err)
	}
	var n int
	if err := f.owner.QueryRow(context.Background(), `SELECT count(*) FROM devices`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("%d devices after the outer rollback, want 0", n)
	}
}

func TestCredentialSource_Devices(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	devs := newDevices(t, f)
	src := app.NewCredentialSource(f.repo)
	ctx := context.Background()
	alex := f.add(domain.KindUser, "alex")
	sam := f.add(domain.KindUser, "sam")
	reg := func(state identity.DeviceState) identity.Device {
		r, _, _ := registration(alex.ID)
		r.State = state
		if state == identity.DeviceStateActive {
			r.ApprovedBy = &sam.ID
		}
		d, err := devs.Register(ctx, r)
		if err != nil {
			t.Fatal(err)
		}
		return d
	}
	active, pending, revoked := reg(identity.DeviceStateActive), reg(identity.DeviceStatePending), reg(identity.DeviceStateActive)
	f.sql(`UPDATE devices SET state = 'revoked', revoked_at = now(), revoke_reason = 'revoked_by_self' WHERE id = $1`, revoked.ID.String())
	// A pending device older than auth.device_pending_ttl stays pending here: its expiry belongs
	// to M1-38 (docs/DECISIONS.md M1-33).
	expired := reg(identity.DeviceStatePending)
	f.sql(`UPDATE devices SET created_at = $2 WHERE id = $1`, expired.ID.String(), time.Now().Add(-30*24*time.Hour))
	for _, tc := range []struct {
		name      string
		principal ids.ID
		device    ids.ID
		want      authz.DeviceState
	}{
		{"active", alex.ID, active.ID, authz.DeviceActive},
		{"pending", alex.ID, pending.ID, authz.DevicePending},
		{"revoked", alex.ID, revoked.ID, authz.DeviceRevoked},
		{"old pending", alex.ID, expired.ID, authz.DevicePending},
		{"foreign principal", sam.ID, active.ID, authz.DeviceUnknown},
		{"unknown device", alex.ID, ids.New(), authz.DeviceUnknown},
	} {
		got, err := src.Credentials(ctx, authz.CredentialRef{PrincipalID: tc.principal, DeviceID: tc.device})
		if err != nil || got.DeviceState != tc.want || got.SessionActive || got.TokenActive {
			t.Errorf("%s: %+v, %v; want device %s and no session or token", tc.name, got, err, tc.want)
		}
	}
	if got, err := src.Credentials(ctx, authz.CredentialRef{PrincipalID: alex.ID}); err != nil || got != (authz.CredentialFacts{}) {
		t.Errorf("no device id: %+v, %v; want zero facts", got, err)
	}
}
