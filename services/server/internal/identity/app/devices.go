// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"

	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/domain"
	"github.com/OWNER/bothy/services/server/internal/identity/domain/device"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// devicesList is the list name of the device cursors; one name for both lists, because the
// binding names the listed person.
const devicesList = "devices"

// DevicesDeps are the collaborators of Devices.
type DevicesDeps struct {
	// Store reads and writes devices and device_keys.
	Store DeviceStore
	// People reads the person a device belongs to (the Store of the principal tables).
	People Store
	// UoW runs Register when the caller has no unit of work.
	UoW txn.UnitOfWork
	// Authorizer decides device.list.
	Authorizer authorize.Authorizer
	// Cursors encodes and checks the page cursors.
	Cursors Cursors
	// Pepper is the raw token pepper, the key of the verification code a pending device reads
	// about itself (M1-38); empty means no code is shown.
	Pepper []byte
}

// Devices implements identity.Devices. It is safe for concurrent use.
type Devices struct {
	cfg  Config
	deps DevicesDeps
}

// Compile-time check: Devices is the device port of identity.
var _ identity.Devices = (*Devices)(nil)

// NewDevices returns the device use cases with the page sizes of cfg (PageSizeDefault and
// PageSizeMax at least 1, the default not above the maximum). Errors: a missing collaborator or
// a page size outside its range.
func NewDevices(cfg Config, deps DevicesDeps) (*Devices, error) {
	if deps.Store == nil || deps.People == nil || deps.UoW == nil || deps.Authorizer == nil || deps.Cursors == nil {
		return nil, errors.New("identity: NewDevices needs store, people, unit of work, authorizer and cursors")
	}
	if cfg.PageSizeDefault < 1 || cfg.PageSizeMax < cfg.PageSizeDefault {
		return nil, errors.New("identity: api.page_size_default must be at least 1 and not above api.page_size_max")
	}
	if len(deps.Pepper) > 0 && (cfg.DeviceVerificationCodeDigits < 1 || cfg.DeviceVerificationCodeDigits > domain.MaxVerificationDigits) {
		return nil, errors.New("identity: auth.device_verification_code_digits out of range")
	}
	deps.Pepper = append([]byte(nil), deps.Pepper...)
	return &Devices{cfg: cfg, deps: deps}, nil
}

// Register implements identity.Devices.
func (s *Devices) Register(ctx context.Context, r identity.RegisterDevice) (identity.Device, error) {
	reg := device.Registration{
		ID: r.DeviceID, UserID: r.UserID, Name: r.Name, Platform: device.Platform(r.Platform), AppVersion: r.AppVersion,
		DPoPKey: keyInput(r.DPoPKey), State: device.State(r.State),
	}
	if r.StepUpKey != nil {
		k := keyInput(*r.StepUpKey)
		reg.StepUpKey = &k
	}
	if r.ApprovedBy != nil {
		reg.ApprovedBy = *r.ApprovedBy
	}
	d, err := device.New(reg)
	if err != nil {
		return identity.Device{}, err
	}
	var out identity.Device
	err = s.deps.UoW.Do(ctx, func(ctx context.Context) error {
		if _, found, err := s.deps.People.User(ctx, d.UserID); err != nil || !found {
			if err == nil {
				err = notFound("person")
			}
			return err
		}
		out, err = s.deps.Store.InsertDevice(ctx, d)
		return err
	})
	return out, err
}

func keyInput(k identity.PublicKey) device.KeyInput {
	return device.KeyInput{JWK: k.JWK, Alg: k.Alg, Attestation: k.Attestation}
}

// Get implements identity.Devices: the device is loaded first, and a missing device gives the
// same not_found as an invisible one (02 section 9.2).
func (s *Devices) Get(ctx context.Context, a actor.Actor, deviceID ids.ID) (identity.Device, error) {
	d, found, err := s.deps.Store.Device(ctx, deviceID)
	if err != nil {
		return identity.Device{}, err
	}
	// A missing device is described without an owner, which only an owner sees; authorize still
	// runs first, so an anonymous caller gets 401 for a missing and an existing device alike
	// (07 section 4.2, red team M1-31).
	r := authorize.Resource{Kind: authorize.ResourceDevice, ID: deviceID, OwnerID: d.UserID}
	if err := s.authorize(ctx, a, r); err != nil {
		return identity.Device{}, err
	}
	if !found {
		return identity.Device{}, invisible()
	}
	s.withOwnCode(a, &d)
	return d, nil
}

// withOwnCode sets the verification code on d when d is the pending device of the caller's own
// session: the code is shown only to the device itself, never to another device or an owner
// (08 section 5.2).
func (s *Devices) withOwnCode(a actor.Actor, d *identity.Device) {
	if len(s.deps.Pepper) == 0 || a.SessionID.IsZero() || d.ID != a.DeviceID || d.State != identity.DeviceStatePending || d.KeyThumbprint == "" {
		return
	}
	d.VerificationCode = domain.DeriveVerificationCode(s.deps.Pepper, d.ID, d.KeyThumbprint, s.cfg.DeviceVerificationCodeDigits)
}

// ListMine implements identity.Devices: device.list on the resource user = the caller.
func (s *Devices) ListMine(ctx context.Context, a actor.Actor, p identity.Page) (identity.DevicePage, error) {
	if err := s.authorize(ctx, a, authorize.Resource{Kind: authorize.ResourceUser, ID: a.ID, OwnerID: a.ID}); err != nil {
		return identity.DevicePage{}, err
	}
	out, err := s.page(ctx, a, a.ID, p)
	for i := range out.Items {
		s.withOwnCode(a, &out.Items[i])
	}
	return out, err
}

// ListForUser implements identity.Devices. The resource is the device set of userID (kind
// device without id, owner userID): devices are visible to their person and to owners only, so
// another person gets not_found, never forbidden (05 rule 2; docs/DECISIONS.md M1-33). authorize
// runs before the person is looked up, so an unknown person gives the answer of authorize first
// and not_found only to a caller it allows (red team M1-31).
func (s *Devices) ListForUser(ctx context.Context, a actor.Actor, userID ids.ID, p identity.Page) (identity.DevicePage, error) {
	if err := s.authorize(ctx, a, authorize.Resource{Kind: authorize.ResourceDevice, OwnerID: userID}); err != nil {
		return identity.DevicePage{}, err
	}
	if _, found, err := s.deps.People.User(ctx, userID); err != nil || !found {
		if err == nil {
			err = invisible()
		}
		return identity.DevicePage{}, err
	}
	return s.page(ctx, a, userID, p)
}

// ActiveKey implements identity.Devices.
func (s *Devices) ActiveKey(ctx context.Context, deviceID ids.ID, purpose identity.KeyPurpose) (identity.DeviceKey, error) {
	k, found, err := s.deps.Store.ActiveKey(ctx, deviceID, purpose)
	if err != nil {
		return identity.DeviceKey{}, err
	}
	if !found {
		return identity.DeviceKey{}, notFound("device key")
	}
	return k, nil
}

// authorize calls the Authorizer with device.list and turns a denial into its error.
func (s *Devices) authorize(ctx context.Context, a actor.Actor, r authorize.Resource) error {
	d, err := s.deps.Authorizer.Authorize(ctx, a, authorize.ActionDeviceList, r)
	if err != nil {
		return err
	}
	return d.Err()
}

// invisible is the not_found of a missing device or person, with the same empty detail as the
// not_found of a denied authorize, so a client cannot tell "missing" from "not visible" (05 rule
// 2, 02 section 9.2).
func invisible() error { return errs.New(errs.CodeNotFound, "") }

// cursorKey is the keyset position of a device page: the id of its last row.
type cursorKey struct {
	ID ids.ID `json:"id"`
}

// page reads one page of the devices of userID for the caller a.
func (s *Devices) page(ctx context.Context, a actor.Actor, userID ids.ID, p identity.Page) (identity.DevicePage, error) {
	binding := []byte(a.ID.String() + "\x00user=" + userID.String() + "\x00sort=id_desc")
	var before ids.ID
	if p.Cursor != "" {
		var k cursorKey
		if err := s.deps.Cursors.Decode(devicesList, binding, p.Cursor, &k); err != nil {
			return identity.DevicePage{}, err
		}
		if k.ID.IsZero() {
			return identity.DevicePage{}, errs.New(errs.CodeCursorInvalid, "")
		}
		before = k.ID
	}
	limit := p.Limit
	if limit < 1 {
		limit = s.cfg.PageSizeDefault
	}
	limit = min(limit, s.cfg.PageSizeMax)
	rows, err := s.deps.Store.DevicesOfUser(ctx, userID, before, limit+1)
	if err != nil {
		return identity.DevicePage{}, err
	}
	out := identity.DevicePage{Items: rows}
	if len(rows) > limit {
		out.Items = rows[:limit]
		next, err := s.deps.Cursors.Encode(devicesList, binding, cursorKey{ID: rows[limit-1].ID})
		if err != nil {
			return identity.DevicePage{}, errs.Wrap(errs.CodeInternalError, err, "identity: device cursor")
		}
		out.NextCursor = next
	}
	return out, nil
}
