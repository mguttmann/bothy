// SPDX-License-Identifier: Apache-2.0

// Package device holds the device entity of the identity module (docs/spec/03-data-model.md
// tables devices and device_keys, docs/spec/08-identity.md sections 5 and 6.1): the states and
// their machine, the platforms, the key purposes and the validation of a registration with its
// two public device keys (brief M1-33 "Device registration").
//
// Validation errors are kernel/errs validation_failed with one field error per rule; pointers are
// relative to the schema DeviceRegistration of docs/api/openapi.yaml (/device_id, /name,
// /platform, /app_version, /public_jwk, /alg, /attestation, /step_up_key/...), plus /user_id,
// /state and /approved_by for the inputs of the login flow. They never repeat the input.
//
// Example:
//
//	d, err := device.New(device.Registration{ID: id, UserID: user, Name: "Alex iPhone",
//		Platform: device.PlatformIOS, DPoPKey: device.KeyInput{JWK: jwk, Alg: "ES256"}, State: device.StatePending})
package device

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/jose"
	"github.com/OWNER/bothy/services/server/internal/kernel/textcheck"
)

// State is devices.state.
type State string

// The device states of 03.
const (
	// StatePending is a new device waiting for approval (08 section 5.2).
	StatePending State = "pending"
	// StateActive is an approved device.
	StateActive State = "active"
	// StateRevoked is a revoked device; it never becomes active again.
	StateRevoked State = "revoked"
)

// Transition checks the state machine pending -> active, pending -> revoked, active -> revoked.
// Every other pair, and a state outside the three, is 409 invalid_state_transition. Approval
// (M1-38) and revocation (M1-45) call it before they write.
func (s State) Transition(to State) error {
	switch {
	case s == StatePending && (to == StateActive || to == StateRevoked):
		return nil
	case s == StateActive && to == StateRevoked:
		return nil
	}
	return errs.New(errs.CodeInvalidStateTransition, "the device state does not allow this change")
}

// Platform is devices.platform.
type Platform string

// The platforms of 03 devices.platform.
const (
	PlatformIOS     Platform = "ios"
	PlatformAndroid Platform = "android"
	PlatformMacOS   Platform = "macos"
	PlatformWindows Platform = "windows"
	PlatformWeb     Platform = "web"
	PlatformCLI     Platform = "cli"
)

// Valid reports whether p is a value of the check on devices.platform.
func (p Platform) Valid() bool {
	switch p {
	case PlatformIOS, PlatformAndroid, PlatformMacOS, PlatformWindows, PlatformWeb, PlatformCLI:
		return true
	}
	return false
}

// Purpose is device_keys.purpose.
type Purpose string

// The two key purposes of 08 section 6.1 (R-ID-04).
const (
	// PurposeDPoP signs every DPoP proof, without user verification.
	PurposeDPoP Purpose = "dpop"
	// PurposeStepUp signs step-up confirmations, with user verification per use.
	PurposeStepUp Purpose = "step_up"
)

// Valid reports whether p is a value of the check on device_keys.purpose.
func (p Purpose) Valid() bool { return p == PurposeDPoP || p == PurposeStepUp }

// KeyInput is one public key as the client sends it.
type KeyInput struct {
	// JWK is the public JWK.
	JWK json.RawMessage
	// Alg is ES256 or EdDSA.
	Alg string
	// Attestation is the platform attestation, nil for none; stored unchanged.
	Attestation json.RawMessage
}

// Key is a validated device key.
type Key struct {
	// Purpose is dpop or step_up.
	Purpose Purpose
	// JKT is the RFC 7638 thumbprint.
	JKT string
	// Alg is ES256 or EdDSA.
	Alg string
	// PublicJWK is the canonical public JWK (the required members only, jose.JWK.Canonical).
	PublicJWK json.RawMessage
	// Attestation is the attestation as received, nil for none.
	Attestation json.RawMessage
}

// Registration is the input of a new device (08 section 5.1).
type Registration struct {
	// ID is the device id the client generated, a UUIDv7.
	ID ids.ID
	// UserID is the person; that it is a user principal is checked against the store.
	UserID ids.ID
	// Name is the display name; it is trimmed.
	Name string
	// Platform is the platform.
	Platform Platform
	// AppVersion is the app version, "" for none.
	AppVersion string
	// DPoPKey is required; StepUpKey only on devices with a secure key store, never on cli.
	DPoPKey   KeyInput
	StepUpKey *KeyInput
	// State is pending or active.
	State State
	// ApprovedBy is the approving principal; required exactly for active.
	ApprovedBy ids.ID
}

// Device is a validated new device with its keys (dpop first).
type Device struct {
	ID, UserID ids.ID
	Name       string
	Platform   Platform
	AppVersion string
	State      State
	// ApprovedBy is set for an active device; the store sets approved_at.
	ApprovedBy ids.ID
	Keys       []Key
}

// The field error codes of this package besides the textcheck rules.
const (
	// FieldRequired marks a missing value.
	FieldRequired = "required"
	// FieldInvalid marks a value outside its rule.
	FieldInvalid = "invalid"
	// FieldNotAllowed marks a value that this kind of device may not have.
	FieldNotAllowed = "not_allowed"
	// FieldSameKey marks a step-up key equal to the DPoP key.
	FieldSameKey = "same_key"
)

// validator collects field errors.
type validator struct{ fields []errs.FieldError }

func (v *validator) add(pointer, code, detail string) {
	v.fields = append(v.fields, errs.FieldError{Pointer: pointer, Code: code, Detail: detail})
}

// New validates r (brief M1-33 "Device registration" items 1 and 3 to 10) and returns the
// device with its keys. Errors: validation_failed with every violated field. That UserID is a
// user principal and that ID and the thumbprints are new is the store's check.
func New(r Registration) (Device, error) {
	var v validator
	if _, err := ids.Parse(r.ID.String()); err != nil || r.ID.IsZero() {
		v.add("/device_id", FieldInvalid, "The device id must be a UUIDv7.")
	}
	if r.UserID.IsZero() {
		v.add("/user_id", FieldRequired, "The person is required.")
	}
	name := strings.TrimSpace(r.Name)
	displayText(&v, "/name", name, true)
	displayText(&v, "/app_version", r.AppVersion, false)
	if !r.Platform.Valid() {
		v.add("/platform", FieldInvalid, "The platform is not one of ios, android, macos, windows, web, cli.")
	}
	keys := keys(&v, r)
	approval(&v, r)
	if len(v.fields) > 0 {
		return Device{}, errs.New(errs.CodeValidationFailed, "The device registration is invalid.").WithFields(v.fields...)
	}
	return Device{
		ID: r.ID, UserID: r.UserID, Name: name, Platform: r.Platform, AppVersion: r.AppVersion,
		State: r.State, ApprovedBy: r.ApprovedBy, Keys: keys,
	}, nil
}

// keys validates the dpop key and the optional step-up key: none on a cli device, and never the
// same key as the dpop key, or the step-up key would sign without user verification.
func keys(v *validator, r Registration) []Key {
	dk := key(v, "", PurposeDPoP, r.DPoPKey)
	out := []Key{dk}
	if r.StepUpKey == nil {
		return out
	}
	if r.Platform == PlatformCLI {
		v.add("/step_up_key", FieldNotAllowed, "A command line device has no step-up key.")
		return out
	}
	sk := key(v, "/step_up_key", PurposeStepUp, *r.StepUpKey)
	if sk.JKT != "" && sk.JKT == dk.JKT {
		v.add("/step_up_key/public_jwk", FieldSameKey, "The step-up key must differ from the DPoP key.")
	}
	return append(out, sk)
}

// approval checks the registration state: pending without an approver, active with one;
// revoked is never a registration state.
func approval(v *validator, r Registration) {
	switch r.State {
	case StatePending:
		if !r.ApprovedBy.IsZero() {
			v.add("/approved_by", FieldNotAllowed, "A pending device has no approver.")
		}
	case StateActive:
		if r.ApprovedBy.IsZero() {
			v.add("/approved_by", FieldRequired, "An active device needs its approver.")
		}
	case StateRevoked:
		v.add("/state", FieldInvalid, "A device is registered as pending or active.")
	default:
		v.add("/state", FieldInvalid, "A device is registered as pending or active.")
	}
}

// displayText applies SEC-IN-4 to a display text at pointer; required rejects an empty text.
func displayText(v *validator, pointer, s string, required bool) {
	if s == "" {
		if required {
			v.add(pointer, FieldRequired, "The value must not be empty.")
		}
		return
	}
	if err := textcheck.DisplayText(s); err != nil {
		var te *textcheck.Error
		code := FieldInvalid
		if errors.As(err, &te) {
			code = string(te.Rule)
		}
		v.add(pointer, code, "The value contains a control or bidi control character or invalid UTF-8.")
	}
}

// key validates one key input below prefix and returns the key; JKT is empty when invalid.
func key(v *validator, prefix string, p Purpose, in KeyInput) Key {
	if len(in.JWK) == 0 {
		v.add(prefix+"/public_jwk", FieldRequired, "The public key is required.")
		return Key{}
	}
	if in.Alg != jose.AlgES256 && in.Alg != jose.AlgEdDSA {
		v.add(prefix+"/alg", FieldInvalid, "The algorithm must be ES256 or EdDSA.")
		return Key{}
	}
	jwk, err := jose.ParseJWK(in.JWK)
	if err != nil {
		v.add(prefix+"/public_jwk", FieldInvalid, "The public key is not a public EC P-256 or OKP Ed25519 JWK.")
		return Key{}
	}
	wantKty := "EC"
	if in.Alg == jose.AlgEdDSA {
		wantKty = "OKP"
	}
	if jwk.Kty != wantKty || jwk.Alg != "" && jwk.Alg != in.Alg {
		v.add(prefix+"/alg", FieldInvalid, "The algorithm does not match the key.")
		return Key{}
	}
	if len(in.Attestation) > 0 {
		var obj map[string]json.RawMessage
		if json.Unmarshal(in.Attestation, &obj) != nil || obj == nil {
			v.add(prefix+"/attestation", FieldInvalid, "The attestation must be a JSON object.")
		}
	}
	return Key{Purpose: p, JKT: jwk.Thumbprint(), Alg: in.Alg, PublicJWK: jwk.Canonical(), Attestation: in.Attestation}
}
