// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"errors"
	"net/netip"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

// pendingDevice logs the person userID in on a new device that stays pending and returns it with
// the verification code the login answered.
func pendingDevice(t *testing.T, id *wire.Identity, userID ids.ID) (identity.Device, string) {
	t.Helper()
	k := freshKey(t)
	res, err := id.Logins.CompleteLogin(context.Background(), app.LoginCompletion{
		UserID: userID, AuthMethod: identity.AuthPasskey, ProofJKT: k.jkt, Trust: app.TrustUnconfirmed,
		Device: identity.RegisterDevice{DeviceID: ids.New(), Name: "Alex Laptop", Platform: identity.PlatformMacOS, DPoPKey: k.pub},
	})
	if err != nil {
		t.Fatal(err)
	}
	return res.Device, res.Tokens.Device.VerificationCode
}

// realDevice stores the device of the session of a, which the fake credential source of the
// world only describes: a pairing names the approving device (approved_by_device_id).
func (w *peopleWorld) realDevice(a ids.ID, dev ids.ID) {
	w.t.Helper()
	w.sql(`INSERT INTO devices (id, user_id, name, platform, state, approved_by, approved_at) VALUES ($1, $2, 'Alex Phone', 'ios', 'active', $2, now())`,
		dev.String(), a.String())
}

// wrongDigit returns code with its last digit changed.
func wrongDigit(code string) string {
	b := []byte(code)
	b[len(b)-1] = '0' + (b[len(b)-1]-'0'+1)%10
	return string(b)
}

// TestService_ApproveDevice runs approveDevice directly: anonymous, unknown, correct and wrong
// codes, an active and a revoked device, and a pending device past auth.device_pending_ttl, which
// the approval revokes as the system action device.expire (M1-38).
func TestService_ApproveDevice(t *testing.T) {
	t.Parallel()
	w := newPeopleWorld(t)
	id := w.identityOf()
	w.person("robin", "active", true)
	alex := w.person("alex", "active", false)
	svc := id.Logins

	dev, code := pendingDevice(t, id, alex.ID)
	if len(code) != loginCfg.DeviceVerificationCodeDigits {
		t.Fatalf("code %q", code)
	}
	_, err := svc.ApproveDevice(context.Background(), dev.ID, code)
	wantKind(t, err, errs.KindUnauthenticated)
	_, err = svc.ApproveDevice(ctxOf(alex), ids.New(), code)
	wantCode(t, err, errs.CodeNotFound)
	got, err := svc.ApproveDevice(ctxOf(alex), dev.ID, code)
	if err != nil || got.State != identity.DeviceStateActive || got.VerificationCode != "" {
		t.Fatalf("approve %+v, %v", got, err)
	}
	if again, err := svc.ApproveDevice(ctxOf(alex), dev.ID, "000000"); err != nil || again.State != identity.DeviceStateActive {
		t.Fatalf("approve an active device %+v, %v", again, err)
	}

	wrongDev, wrongCode := pendingDevice(t, id, alex.ID)
	got, err = svc.ApproveDevice(ctxOf(alex), wrongDev.ID, wrongDigit(wrongCode))
	if err != nil || got.State != identity.DeviceStateRevoked || got.RevokeReason != "verification_failed" {
		t.Fatalf("wrong code %+v, %v", got, err)
	}
	_, err = svc.ApproveDevice(ctxOf(alex), wrongDev.ID, wrongCode)
	wantCode(t, err, errs.CodeInvalidStateTransition)

	old, oldCode := pendingDevice(t, id, alex.ID)
	w.sql(`UPDATE devices SET created_at = created_at - make_interval(secs => $2) WHERE id = $1`, old.ID.String(), loginCfg.DevicePendingTTL.Seconds())
	_, err = svc.ApproveDevice(ctxOf(alex), old.ID, oldCode)
	wantCode(t, err, errs.CodeInvalidStateTransition)
	var state, reason string
	if err := w.owner.QueryRow(context.Background(), `SELECT state, coalesce(revoke_reason, '') FROM devices WHERE id = $1`, old.ID.String()).Scan(&state, &reason); err != nil {
		t.Fatal(err)
	}
	if state != "revoked" || reason != "pending_expired" {
		t.Fatalf("expired pending device %s %s", state, reason)
	}
}

// TestService_Pairing runs the pairing use cases directly: the registration rules of
// beginPairing, the codes of approvePairing, and completePairing while requested, with another
// key and after the approval (M1-38).
func TestService_Pairing(t *testing.T) {
	t.Parallel()
	w := newPeopleWorld(t)
	id := w.identityOf()
	w.person("robin", "active", true)
	alex := w.person("alex", "active", false)
	w.realDevice(alex.ID, alex.DeviceID)
	svc := id.Logins
	ctx := context.Background()
	ip := netip.MustParseAddr("192.0.2.1")

	k, step := freshKey(t), freshKey(t)
	reg := identity.RegisterDevice{DeviceID: ids.New(), Name: "Alex Laptop", Platform: identity.PlatformMacOS, DPoPKey: k.pub, StepUpKey: &step.pub}
	var dpopErr *identity.DPoPError
	if _, err := svc.BeginPairing(ctx, reg, step.jkt, ip); !errors.As(err, &dpopErr) {
		t.Fatalf("begin with the proof of another key: %v", err)
	}
	bad := reg
	bad.Name = ""
	_, err := svc.BeginPairing(ctx, bad, k.jkt, ip)
	wantKind(t, err, errs.KindBadRequest)
	started, err := svc.BeginPairing(ctx, reg, k.jkt, ip)
	if err != nil || started.Code.Reveal() == "" || started.Code.String() != "[redacted]" {
		t.Fatalf("begin %+v, %v", started, err)
	}

	out, err := svc.CompletePairing(ctx, started.ID, k.jkt, ip)
	if err != nil || out.Completed || out.PairingID != started.ID {
		t.Fatalf("complete while requested %+v, %v", out, err)
	}
	_, err = svc.CompletePairing(ctx, ids.New(), k.jkt, ip)
	wantCode(t, err, errs.CodeGone)

	_, err = svc.ApprovePairing(ctx, started.Code.Reveal())
	wantKind(t, err, errs.KindUnauthenticated)
	_, err = svc.ApprovePairing(ctxOf(alex), "UUUU-UUUU")
	wantCode(t, err, errs.CodeValidationFailed)
	_, err = svc.ApprovePairing(ctxOf(alex), "0000-0000")
	wantCode(t, err, errs.CodeNotFound)
	created, err := svc.ApprovePairing(ctxOf(alex), started.Code.Reveal())
	if err != nil || created.State != identity.DeviceStateActive || created.UserID != alex.ID || created.ID != reg.DeviceID || created.StepUpKeyThumbprint != step.jkt {
		t.Fatalf("approve %+v, %v", created, err)
	}
	// An approved pairing keeps its state on a proof of another key.
	if _, err := svc.CompletePairing(ctx, started.ID, step.jkt, ip); !errors.As(err, &dpopErr) {
		t.Fatalf("complete with another key: %v", err)
	}
	done, err := svc.CompletePairing(ctx, started.ID, k.jkt, ip)
	if err != nil || !done.Completed || done.Tokens.Session.AuthMethod != identity.AuthDevicePairing || done.Tokens.Device.State != identity.DeviceStateActive {
		t.Fatalf("complete %+v, %v", done, err)
	}
	_, err = svc.CompletePairing(ctx, started.ID, k.jkt, ip)
	wantCode(t, err, errs.CodeGone)
	// The device id is now in use.
	_, err = svc.BeginPairing(ctx, reg, k.jkt, ip)
	wantKind(t, err, errs.KindConflict)

	// A requested pairing with a proof of another key is rejected.
	k2 := freshKey(t)
	second, err := svc.BeginPairing(ctx, identity.RegisterDevice{DeviceID: ids.New(), Name: "Alex Phone", Platform: identity.PlatformCLI, DPoPKey: k2.pub}, k2.jkt, ip)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CompletePairing(ctx, second.ID, k.jkt, ip); !errors.As(err, &dpopErr) {
		t.Fatalf("reject: %v", err)
	}
	_, err = svc.CompletePairing(ctx, second.ID, k2.jkt, ip)
	wantCode(t, err, errs.CodeGone)
}

// TestService_Expiry runs the cleanup job with the expiry of pending devices and pairings: a
// pending device past auth.device_pending_ttl, a requested and an approved pairing past their end
// (M1-38).
func TestService_Expiry(t *testing.T) {
	t.Parallel()
	w := newPeopleWorld(t)
	id := w.identityOf()
	w.person("robin", "active", true)
	alex := w.person("alex", "active", false)
	w.realDevice(alex.ID, alex.DeviceID)
	svc := id.Logins
	ctx := context.Background()
	ip := netip.MustParseAddr("192.0.2.1")

	dev, _ := pendingDevice(t, id, alex.ID)
	requestedKey, approvedKey := freshKey(t), freshKey(t)
	requested, err := svc.BeginPairing(ctx, identity.RegisterDevice{DeviceID: ids.New(), Name: "Alex One", Platform: identity.PlatformCLI, DPoPKey: requestedKey.pub}, requestedKey.jkt, ip)
	if err != nil {
		t.Fatal(err)
	}
	approvedReg := identity.RegisterDevice{DeviceID: ids.New(), Name: "Alex Two", Platform: identity.PlatformCLI, DPoPKey: approvedKey.pub}
	approved, err := svc.BeginPairing(ctx, approvedReg, approvedKey.jkt, ip)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ApprovePairing(ctxOf(alex), approved.Code.Reveal()); err != nil {
		t.Fatal(err)
	}
	if n, err := svc.ExpirePendingDevices(ctx); err != nil || n != 0 {
		t.Fatalf("nothing due yet: %d, %v", n, err)
	}
	w.clk.Advance(loginCfg.DevicePendingTTL)
	n, err := id.AuthCleanup.Run(ctx)
	if err != nil || n < 3 {
		t.Fatalf("cleanup ended %d rows, %v", n, err)
	}
	rows := map[string]string{}
	for _, q := range []struct{ name, sql, id string }{
		{"device", `SELECT state || '/' || coalesce(revoke_reason, '') FROM devices WHERE id = $1`, dev.ID.String()},
		{"paired device", `SELECT state || '/' || coalesce(revoke_reason, '') FROM devices WHERE id = $1`, approvedReg.DeviceID.String()},
		{"requested", `SELECT status FROM device_pairings WHERE id = $1`, requested.ID.String()},
		{"approved", `SELECT status FROM device_pairings WHERE id = $1`, approved.ID.String()},
	} {
		var v string
		if err := w.owner.QueryRow(ctx, q.sql, q.id).Scan(&v); err != nil {
			t.Fatal(err)
		}
		rows[q.name] = v
	}
	if rows["device"] != "revoked/pending_expired" || rows["paired device"] != "revoked/pairing_expired" ||
		rows["requested"] != "expired" || rows["approved"] != "expired" {
		t.Fatalf("after the cleanup %v", rows)
	}
	if n, err := svc.ExpirePairings(ctx); err != nil || n != 0 {
		t.Fatalf("second run %d, %v", n, err)
	}
	var system int
	if err := w.owner.QueryRow(ctx, `SELECT count(*) FROM audit_events WHERE action = 'device.expire' AND actor_principal_id IS NULL`).Scan(&system); err != nil || system != 2 {
		t.Fatalf("device.expire entries %d, %v", system, err)
	}
}
