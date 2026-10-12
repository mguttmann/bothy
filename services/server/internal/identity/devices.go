// SPDX-License-Identifier: Apache-2.0

package identity

import (
	"context"
	"encoding/json"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// KeyPurpose is device_keys.purpose (docs/spec/08-identity.md section 6.1, R-ID-04).
type KeyPurpose string

// The two purposes of a device key.
const (
	// KeyPurposeDPoP signs every DPoP proof, without user verification.
	KeyPurposeDPoP KeyPurpose = "dpop"
	// KeyPurposeStepUp signs step-up confirmations, with user verification per use (M1-35).
	KeyPurposeStepUp KeyPurpose = "step_up"
)

// DeviceState is devices.state.
type DeviceState string

// The device states of docs/spec/03-data-model.md.
const (
	// DeviceStatePending waits for approval: tokens, but no content (05 section 2.5).
	DeviceStatePending DeviceState = "pending"
	// DeviceStateActive is an approved device.
	DeviceStateActive DeviceState = "active"
	// DeviceStateRevoked is a revoked device.
	DeviceStateRevoked DeviceState = "revoked"
)

// Platform is devices.platform (schema DevicePlatform).
type Platform string

// The platforms of devices.platform.
const (
	PlatformIOS     Platform = "ios"
	PlatformAndroid Platform = "android"
	PlatformMacOS   Platform = "macos"
	PlatformWindows Platform = "windows"
	PlatformWeb     Platform = "web"
	PlatformCLI     Platform = "cli"
)

// PublicKey is one public device key as a client registers it (schema DeviceKeyRegistration).
type PublicKey struct {
	// JWK is the public JWK (RFC 7517) without private members.
	JWK json.RawMessage
	// Alg is ES256 or EdDSA.
	Alg string
	// Attestation is the platform attestation, nil for none; stored unchanged, never required in M1.
	Attestation json.RawMessage
}

// RegisterDevice is the input of Devices.Register (08 section 5.1; schema DeviceRegistration).
type RegisterDevice struct {
	// DeviceID is the UUIDv7 the client generated.
	DeviceID ids.ID
	// UserID is the person the device belongs to.
	UserID ids.ID
	// Name is the display name; trimmed, not empty, without control or bidi characters.
	Name string
	// Platform is the platform.
	Platform Platform
	// AppVersion is the app version, "" for none.
	AppVersion string
	// DPoPKey is the key that signs the DPoP proofs; required.
	DPoPKey PublicKey
	// StepUpKey is the key with user verification; nil where no secure key store exists, never
	// for PlatformCLI.
	StepUpKey *PublicKey
	// State is DeviceStatePending or DeviceStateActive.
	State DeviceState
	// ApprovedBy is the approving principal, required exactly for DeviceStateActive.
	ApprovedBy *ids.ID
}

// Device is a device as the API shows it (schema Device without push_registrations, which M3
// adds, and without verification_code, which M1-38 adds). Zero values are absent members.
type Device struct {
	ID, UserID ids.ID
	Name       string
	Platform   Platform
	State      DeviceState
	// AppVersion is "" for none.
	AppVersion string
	// KeyThumbprint is the jkt of the active dpop key; StepUpKeyThumbprint that of the active
	// step_up key; "" when there is none.
	KeyThumbprint, StepUpKeyThumbprint string
	// RevokeReason is the code of devices.revoke_reason, "" for none.
	RevokeReason string
	// ApprovedAt, RevokedAt and LastSeenAt are zero for none.
	ApprovedAt, RevokedAt, CreatedAt, LastSeenAt time.Time
}

// DeviceKey is one row of device_keys.
type DeviceKey struct {
	ID, DeviceID ids.ID
	Purpose      KeyPurpose
	// JKT is the RFC 7638 thumbprint.
	JKT string
	// Alg is ES256 or EdDSA.
	Alg string
	// PublicJWK is the canonical public JWK.
	PublicJWK json.RawMessage
	// RevokedAt is zero for an active key.
	RevokedAt time.Time
}

// Page asks for one page of a list (06 "6. Pagination").
type Page struct {
	// Cursor is next_cursor of the previous page, "" for the first page.
	Cursor string
	// Limit is the requested page size; 0 means api.page_size_default, larger values are capped at
	// api.page_size_max.
	Limit int
}

// DevicePage is one page of devices, newest first; NextCursor is "" on the last page.
type DevicePage struct {
	Items      []Device
	NextCursor string
}

// Devices is the device port of identity (PLAN M1-33). Reads that name an actor call authorize
// with device.list; every error is a kernel/errs error without SQL or values.
type Devices interface {
	// Register validates and stores a new device with its keys (brief M1-33 "Device
	// registration"). It joins the ambient unit of work of ctx, or runs its own. The login
	// tickets (M1-37 to M1-43) call it after their own authorization. Errors: validation_failed
	// with field pointers, not_found when UserID is no person, conflict for a known device id or
	// jkt.
	Register(ctx context.Context, r RegisterDevice) (Device, error)
	// Get returns the device deviceID if a may read it (self or owner; a pending device only
	// itself). Errors: not_found for a missing or invisible device, device_pending for another
	// own device read from a pending device.
	Get(ctx context.Context, a actor.Actor, deviceID ids.ID) (Device, error)
	// ListMine returns the devices of a itself, all states, newest first. Errors: device_pending
	// from a pending device, cursor_invalid.
	ListMine(ctx context.Context, a actor.Actor, p Page) (DevicePage, error)
	// ListForUser returns the devices of userID for a (self or owner). Errors: not_found for an
	// unknown or invisible person, cursor_invalid.
	ListForUser(ctx context.Context, a actor.Actor, userID ids.ID, p Page) (DevicePage, error)
	// ActiveKey returns the key of deviceID and purpose with revoked_at IS NULL. It does not
	// authorize: the authentication path calls it. Errors: not_found when there is none.
	ActiveKey(ctx context.Context, deviceID ids.ID, purpose KeyPurpose) (DeviceKey, error)
}
