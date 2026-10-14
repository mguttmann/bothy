// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/domain/device"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/jose"
)

func deviceKey(t *testing.T, ed bool) device.KeyInput {
	t.Helper()
	var pub any
	alg := jose.AlgES256
	if ed {
		p, _, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		pub, alg = p, jose.AlgEdDSA
	} else {
		k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		pub = &k.PublicKey
	}
	j, err := jose.JWKFromPublicKey(pub)
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(j)
	if err != nil {
		t.Fatal(err)
	}
	return device.KeyInput{JWK: b, Alg: alg}
}

func newDevice(t *testing.T, userID ids.ID, stepUp *device.KeyInput) device.Device {
	t.Helper()
	d, err := device.New(device.Registration{
		ID: ids.New(), UserID: userID, Name: "Laptop", Platform: device.PlatformMacOS, AppVersion: "1.0.0",
		DPoPKey: deviceKey(t, false), StepUpKey: stepUp, State: device.StatePending,
	})
	if err != nil {
		t.Fatal(err)
	}
	return d
}

// TestDevices_InsertReadAndList stores a device with both keys and reads it back through every
// read of the device store.
func TestDevices_InsertReadAndList(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	u, err := e.addUser(t, "devowner", "devowner@example.com")
	if err != nil {
		t.Fatal(err)
	}
	stepUp := deviceKey(t, true)
	d := newDevice(t, u.Principal.ID, &stepUp)
	var stored identity.Device
	if err := e.write(t, func(ctx context.Context) error {
		var err error
		stored, err = e.repo.InsertDevice(ctx, d)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if stored.KeyThumbprint == "" || stored.StepUpKeyThumbprint == "" || stored.State != identity.DeviceState(device.StatePending) {
		t.Fatalf("InsertDevice = %+v", stored)
	}
	ctx := context.Background()
	got, ok, err := e.repo.Device(ctx, d.ID)
	if err != nil || !ok || got.KeyThumbprint != stored.KeyThumbprint || got.StepUpKeyThumbprint != stored.StepUpKeyThumbprint || got.Name != "Laptop" {
		t.Fatalf("Device = %+v, %v, %v", got, ok, err)
	}
	if _, ok, err := e.repo.Device(ctx, ids.New()); ok || err != nil {
		t.Fatalf("Device of an unknown id = %v, %v", ok, err)
	}
	list, err := e.repo.DevicesOfUser(ctx, u.Principal.ID, ids.ID{}, 10)
	if err != nil || len(list) != 1 || list[0].ID != d.ID {
		t.Fatalf("DevicesOfUser = %+v, %v", list, err)
	}
	if list, err := e.repo.DevicesOfUser(ctx, u.Principal.ID, d.ID, 10); err != nil || len(list) != 0 {
		t.Fatalf("DevicesOfUser before the only device = %+v, %v", list, err)
	}
	key, ok, err := e.repo.ActiveKey(ctx, d.ID, identity.KeyPurposeDPoP)
	if err != nil || !ok || key.JKT != stored.KeyThumbprint || key.DeviceID != d.ID {
		t.Fatalf("ActiveKey = %+v, %v, %v", key, ok, err)
	}
	if _, ok, err := e.repo.ActiveKey(ctx, ids.New(), identity.KeyPurposeDPoP); ok || err != nil {
		t.Fatalf("ActiveKey of an unknown device = %v, %v", ok, err)
	}
	byJKT, ok, err := e.repo.KeyByJKT(ctx, stored.StepUpKeyThumbprint)
	if err != nil || !ok || byJKT.Purpose != identity.KeyPurposeStepUp {
		t.Fatalf("KeyByJKT = %+v, %v, %v", byJKT, ok, err)
	}
	if _, ok, err := e.repo.KeyByJKT(ctx, "unknown"); ok || err != nil {
		t.Fatalf("KeyByJKT of an unknown thumbprint = %v, %v", ok, err)
	}
}

// TestDevices_TakenStepUpKeyNamesTheStepUpField: a second device with the step-up key of the
// first is a conflict whose field pointer lies below /step_up_key.
func TestDevices_TakenStepUpKeyNamesTheStepUpField(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	u, err := e.addUser(t, "keyowner", "keyowner@example.com")
	if err != nil {
		t.Fatal(err)
	}
	stepUp := deviceKey(t, true)
	if err := e.write(t, func(ctx context.Context) error {
		_, err := e.repo.InsertDevice(ctx, newDevice(t, u.Principal.ID, &stepUp))
		return err
	}); err != nil {
		t.Fatal(err)
	}
	err = e.write(t, func(ctx context.Context) error {
		_, err := e.repo.InsertDevice(ctx, newDevice(t, u.Principal.ID, &stepUp))
		return err
	})
	ee, ok := errs.As(err)
	if !ok || len(ee.Fields) == 0 {
		t.Fatalf("second device with the same step-up key: %v, want a field error", err)
	}
	for _, f := range ee.Fields {
		if len(f.Pointer) < len("/step_up_key") || f.Pointer[:len("/step_up_key")] != "/step_up_key" {
			t.Errorf("field pointer %q, want one below /step_up_key", f.Pointer)
		}
	}
}

// TestDevices_InsertNeedsAUnitOfWork: the write refuses to run outside a unit of work.
func TestDevices_InsertNeedsAUnitOfWork(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	if _, err := e.repo.InsertDevice(context.Background(), newDevice(t, ids.New(), nil)); err == nil {
		t.Fatal("InsertDevice outside a unit of work succeeded")
	}
}
