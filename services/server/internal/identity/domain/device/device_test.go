// SPDX-License-Identifier: Apache-2.0

package device_test

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"slices"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/identity/domain/device"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/jose"
)

func TestDevice_StateMachine(t *testing.T) {
	t.Parallel()
	allowed := [][2]device.State{
		{device.StatePending, device.StateActive},
		{device.StatePending, device.StateRevoked},
		{device.StateActive, device.StateRevoked},
	}
	all := []device.State{device.StatePending, device.StateActive, device.StateRevoked}
	for _, from := range all {
		for _, to := range all {
			err := from.Transition(to)
			if slices.Contains(allowed, [2]device.State{from, to}) {
				if err != nil {
					t.Errorf("%s -> %s: %v", from, to, err)
				}
				continue
			}
			e, ok := errs.As(err)
			if !ok || e.PublicCode() != errs.CodeInvalidStateTransition || e.Kind().Status() != 409 {
				t.Errorf("%s -> %s = %v, want 409 invalid_state_transition", from, to, err)
			}
		}
	}
	if err := device.State("lost").Transition(device.StateActive); errs.KindOf(err) != errs.KindConflict {
		t.Errorf("unknown state: %v", err)
	}
}

func ecKey(t *testing.T) json.RawMessage {
	t.Helper()
	k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	j, err := jose.JWKFromPublicKey(&k.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(j)
	return b
}

func edKey(t *testing.T) json.RawMessage {
	t.Helper()
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	j, err := jose.JWKFromPublicKey(pub)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(j)
	return b
}

func valid(t *testing.T) device.Registration {
	t.Helper()
	return device.Registration{
		ID: ids.New(), UserID: ids.New(), Name: "  Alex iPhone ", Platform: device.PlatformIOS, AppVersion: "1.4.0",
		DPoPKey:   device.KeyInput{JWK: ecKey(t), Alg: jose.AlgES256, Attestation: json.RawMessage(`{"fmt":"apple"}`)},
		StepUpKey: &device.KeyInput{JWK: edKey(t), Alg: jose.AlgEdDSA},
		State:     device.StatePending,
	}
}

func TestRegister_Valid(t *testing.T) {
	t.Parallel()
	r := valid(t)
	d, err := device.New(r)
	if err != nil {
		t.Fatal(err)
	}
	if d.Name != "Alex iPhone" || d.State != device.StatePending || len(d.Keys) != 2 {
		t.Fatalf("device %+v", d)
	}
	dk, sk := d.Keys[0], d.Keys[1]
	if dk.Purpose != device.PurposeDPoP || sk.Purpose != device.PurposeStepUp || dk.Alg != jose.AlgES256 || sk.Alg != jose.AlgEdDSA {
		t.Fatalf("keys %+v", d.Keys)
	}
	parsed, _ := jose.ParseJWK(r.DPoPKey.JWK)
	if dk.JKT != parsed.Thumbprint() || string(dk.PublicJWK) != string(parsed.Canonical()) || string(dk.Attestation) != `{"fmt":"apple"}` {
		t.Fatalf("dpop key %+v", dk)
	}
	approver := ids.New()
	r = valid(t)
	r.StepUpKey, r.Platform, r.State, r.ApprovedBy = nil, device.PlatformCLI, device.StateActive, approver
	d, err = device.New(r)
	if err != nil || d.State != device.StateActive || d.ApprovedBy != approver || len(d.Keys) != 1 {
		t.Fatalf("active cli device %+v, %v", d, err)
	}
}

// field returns the pointers of the field errors of a validation_failed error.
func fields(t *testing.T, err error) []string {
	t.Helper()
	e, ok := errs.As(err)
	if !ok || e.PublicCode() != errs.CodeValidationFailed {
		t.Fatalf("error %v, want validation_failed", err)
	}
	var out []string
	for _, f := range e.Fields {
		out = append(out, f.Pointer)
	}
	return out
}

func TestRegister_Validation(t *testing.T) {
	t.Parallel()
	same := ecKey(t)
	cases := []struct {
		name    string
		mod     func(*device.Registration)
		pointer string
	}{
		{"zero device id", func(r *device.Registration) { r.ID = ids.ID{} }, "/device_id"},
		{"device id not v7", func(r *device.Registration) { r.ID = ids.ID{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x40} }, "/device_id"},
		{"zero user", func(r *device.Registration) { r.UserID = ids.ID{} }, "/user_id"},
		{"empty name", func(r *device.Registration) { r.Name = "   " }, "/name"},
		{"control character", func(r *device.Registration) { r.Name = "Alex\u0007Phone" }, "/name"},
		{"newline", func(r *device.Registration) { r.Name = "Alex\nPhone" }, "/name"},
		{"bidi control", func(r *device.Registration) { r.Name = "Alex\u202ePhone" }, "/name"},
		{"app version bidi", func(r *device.Registration) { r.AppVersion = "1.\u2066" }, "/app_version"},
		{"unknown platform", func(r *device.Registration) { r.Platform = "tv" }, "/platform"},
		{"no dpop key", func(r *device.Registration) { r.DPoPKey = device.KeyInput{} }, "/public_jwk"},
		{"dpop alg RS256", func(r *device.Registration) { r.DPoPKey.Alg = "RS256" }, "/alg"},
		{"dpop alg mismatch", func(r *device.Registration) { r.DPoPKey.Alg = jose.AlgEdDSA }, "/alg"},
		{"dpop private key", func(r *device.Registration) {
			r.DPoPKey.JWK = json.RawMessage(`{"kty":"OKP","crv":"Ed25519","x":"11qYAYKxCrfVS_7TyWQHOg7hcvPapiMlrwIaaPcHURo","d":"AA"}`)
			r.DPoPKey.Alg = jose.AlgEdDSA
		}, "/public_jwk"},
		{"attestation not an object", func(r *device.Registration) { r.DPoPKey.Attestation = json.RawMessage(`[1]`) }, "/attestation"},
		{"step-up alg", func(r *device.Registration) { r.StepUpKey.Alg = "PS256" }, "/step_up_key/alg"},
		{"step-up jwk", func(r *device.Registration) { r.StepUpKey.JWK = json.RawMessage(`{"kty":"oct","k":"AA"}`) }, "/step_up_key/public_jwk"},
		{"equal dpop and step-up keys", func(r *device.Registration) {
			r.DPoPKey = device.KeyInput{JWK: same, Alg: jose.AlgES256}
			r.StepUpKey = &device.KeyInput{JWK: same, Alg: jose.AlgES256}
		}, "/step_up_key/public_jwk"},
		{"cli with step-up key (ID-T29)", func(r *device.Registration) { r.Platform = device.PlatformCLI }, "/step_up_key"},
		{"state revoked", func(r *device.Registration) { r.State = device.StateRevoked }, "/state"},
		{"state empty", func(r *device.Registration) { r.State = "" }, "/state"},
		{"active without approver", func(r *device.Registration) { r.State = device.StateActive }, "/approved_by"},
		{"pending with approver", func(r *device.Registration) { r.ApprovedBy = ids.New() }, "/approved_by"},
	}
	for _, tc := range cases {
		r := valid(t)
		tc.mod(&r)
		_, err := device.New(r)
		if err == nil {
			t.Errorf("%s: accepted", tc.name)
			continue
		}
		if got := fields(t, err); !slices.Contains(got, tc.pointer) {
			t.Errorf("%s: fields %v, want %s", tc.name, got, tc.pointer)
		}
	}
}

func TestPlatformAndPurpose(t *testing.T) {
	t.Parallel()
	for _, p := range []device.Platform{"ios", "android", "macos", "windows", "web", "cli"} {
		if !p.Valid() {
			t.Errorf("%s invalid", p)
		}
	}
	if device.Platform("IOS").Valid() || device.Purpose("other").Valid() || !device.PurposeStepUp.Valid() {
		t.Error("enumeration check wrong")
	}
}
