// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"net/netip"

	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/domain"
	"github.com/OWNER/bothy/services/server/internal/identity/domain/device"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/jose"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// pairingCodeAttempts bounds the draws of a new pairing code when the drawn code collides with
// another open pairing (docs/DECISIONS.md M1-38): with 40 bits and few open pairings a collision
// is rare, and several in a row mean a broken random source.
const pairingCodeAttempts = 5

// BeginPairing implements identity.DeviceApproval (brief M1-38 "BeginPairing"): authorize
// auth.login.begin for the anonymous principal; validate the registration with the device rules
// of M1-33; the proof key must be the registered dpop key; the device id must not be in use; then
// store the pairing with a new code and its end auth.pairing_code_ttl. The code is returned once
// behind the redacting Token and never logged.
func (s *Service) BeginPairing(ctx context.Context, reg identity.RegisterDevice, proofJKT string, _ netip.Addr) (identity.PairingStarted, error) {
	if err := s.authorize(ctx, actor.Anonymous(), authorize.ActionAuthLoginBegin, authorize.Resource{Kind: authorize.ResourceInstance}); err != nil {
		return identity.PairingStarted{}, err
	}
	// The person is unknown until the approval; the rules of a registration do not depend on it,
	// so the device id stands in for it during the check.
	d, err := device.New(device.Registration{
		ID: reg.DeviceID, UserID: reg.DeviceID, Name: reg.Name, Platform: device.Platform(reg.Platform), AppVersion: reg.AppVersion,
		DPoPKey: keyInput(reg.DPoPKey), StepUpKey: stepUpInput(reg.StepUpKey), State: device.StatePending,
	})
	if err != nil {
		return identity.PairingStarted{}, err
	}
	if d.Keys[0].JKT != proofJKT {
		return identity.PairingStarted{}, &identity.DPoPError{Code: identity.DPoPInvalidProof, Nonce: s.d.DPoP.CurrentNonce()}
	}
	var out identity.PairingStarted
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		out, err = s.insertPairing(ctx, d)
		return err
	})
	return out, err
}

// stepUpInput converts the optional step-up key of a registration.
func stepUpInput(k *identity.PublicKey) *device.KeyInput {
	if k == nil {
		return nil
	}
	in := keyInput(*k)
	return &in
}

// insertPairing stores the requested pairing of the validated device d with a fresh code; a code
// that collides with another open pairing is drawn again, at most pairingCodeAttempts times.
func (s *Service) insertPairing(ctx context.Context, d device.Device) (identity.PairingStarted, error) {
	if _, found, err := s.d.Devices.Device(ctx, d.ID); err != nil || found {
		if err == nil {
			err = errs.New(errs.CodeConflict, "").WithFields(errs.FieldError{Pointer: fieldDeviceID, Code: fieldCodeTaken, Detail: "The device id is taken."})
		}
		return identity.PairingStarted{}, err
	}
	now := s.d.Clock.Now()
	rec := PairingRecord{
		ID: ids.New(), Registration: registrationV1(d), Status: domain.PairingRequested, CreatedAt: now, ExpiresAt: now.Add(s.cfg.PairingCodeTTL),
	}
	for range pairingCodeAttempts {
		code, err := domain.NewPairingCode(s.d.Random, s.cfg.PairingCodeChars)
		if err != nil {
			return identity.PairingStarted{}, errs.Wrap(errs.CodeInternalError, err, "identity: random source")
		}
		rec.Hash = domain.PairingCodeHash(s.d.Pepper, code)
		inserted, err := s.d.Pairings.InsertPairing(ctx, rec)
		if err != nil {
			return identity.PairingStarted{}, err
		}
		if inserted {
			return identity.PairingStarted{ID: rec.ID, Code: identity.NewToken(domain.DisplayPairingCode(code)), ExpiresAt: rec.ExpiresAt}, nil
		}
	}
	return identity.PairingStarted{}, errs.New(errs.CodeInternalError, "identity: no free pairing code")
}

// registrationV1 is device_registration of the validated device d (schema device_registration.v1).
func registrationV1(d device.Device) domain.DeviceRegistrationV1 {
	out := domain.DeviceRegistrationV1{V: 1, DeviceID: d.ID.String(), Name: d.Name, Platform: string(d.Platform), AppVersion: d.AppVersion}
	for _, k := range d.Keys {
		rk := domain.RegisteredKey{JWK: k.PublicJWK, Alg: k.Alg}
		if k.Purpose == device.PurposeDPoP {
			out.DPoP, out.Attestation = rk, k.Attestation
			continue
		}
		rk.Attestation = k.Attestation
		out.StepUp = &rk
	}
	return out
}

// registerDevice is the registration of a pairing as the login completion takes it.
func registerDevice(r domain.DeviceRegistrationV1) (identity.RegisterDevice, error) {
	id, err := ids.Parse(r.DeviceID)
	if err != nil {
		return identity.RegisterDevice{}, errs.Wrap(errs.CodeInternalError, err, "identity: stored pairing registration")
	}
	out := identity.RegisterDevice{
		DeviceID: id, Name: r.Name, Platform: identity.Platform(r.Platform), AppVersion: r.AppVersion,
		DPoPKey: identity.PublicKey{JWK: r.DPoP.JWK, Alg: r.DPoP.Alg, Attestation: r.Attestation},
	}
	if r.StepUp != nil {
		out.StepUpKey = &identity.PublicKey{JWK: r.StepUp.JWK, Alg: r.StepUp.Alg, Attestation: r.StepUp.Attestation}
	}
	return out, nil
}

// pairingJKT is the thumbprint of the dpop key of a stored registration ("" when it cannot be
// parsed, which no proof matches).
func pairingJKT(r domain.DeviceRegistrationV1) string {
	k, err := jose.ParseJWK(r.DPoP.JWK)
	if err != nil {
		return ""
	}
	return k.Thumbprint()
}

// ApprovePairing implements identity.DeviceApproval (brief M1-38 "ApprovePairing"): authorize
// device.approve on a device of the caller (step-up); a malformed code is validation_failed with
// the step-up untouched. A well-formed code without an open pairing ends the step-up of the
// calling session in a committed unit of work, is audited as a denial and answers not_found, so
// every guess costs a new confirmation with biometrics. A match creates the device as an active
// device of the caller in a second unit declared as the caller's rights change and opens a fresh
// window auth.pairing_code_ttl to collect it.
func (s *Service) ApprovePairing(ctx context.Context, pairingCode string) (identity.Device, error) {
	a := caller(ctx)
	if err := s.authorize(ctx, a, authorize.ActionDeviceApprove, authorize.Resource{Kind: authorize.ResourceDevice, OwnerID: a.ID}); err != nil {
		return identity.Device{}, err
	}
	canonical, ok := domain.NormalizePairingCode(pairingCode, s.cfg.PairingCodeChars)
	if !ok {
		return identity.Device{}, invalidCode(fieldPairingCode)
	}
	hash := domain.PairingCodeHash(s.d.Pepper, canonical)
	var matched PairingRecord
	// The end of the step-up and its deny entry must commit although the answer is 404.
	err := s.d.UoW.Do(committed(ctx), func(ctx context.Context) error {
		matched = PairingRecord{}
		rec, found, err := s.d.Pairings.LockRequestedPairingByHash(ctx, hash)
		if err != nil {
			return err
		}
		if found && s.d.Clock.Now().Before(rec.ExpiresAt) {
			matched = rec
			return nil
		}
		return s.endStepUp(ctx, a)
	})
	if err != nil {
		return identity.Device{}, err
	}
	if matched.ID.IsZero() {
		return identity.Device{}, invisible()
	}
	var out identity.Device
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		out, err = s.approvePairingWrite(ctx, a, matched.ID)
		return err
	}, txn.DeclareRightsChange(txn.RightsChangeSpec{
		Action: string(authorize.ActionDeviceApprove), Resource: txn.Ref{Kind: resourceDevice, ID: deviceIDOf(matched)}, Principals: []ids.ID{a.ID},
	}))
	return out, err
}

// deviceIDOf is the device id of the registration of rec; zero when it cannot be parsed.
func deviceIDOf(rec PairingRecord) ids.ID {
	id, _ := ids.Parse(rec.Registration.DeviceID)
	return id
}

// endStepUp ends the step-up of the session of a and records the denial of a pairing code that
// matched no open pairing inside the ambient unit of work, which commits.
func (s *Service) endStepUp(ctx context.Context, a actor.Actor) error {
	if !a.SessionID.IsZero() {
		if err := s.d.Sessions.ClearSessionStepUp(ctx, a.SessionID); err != nil {
			return err
		}
	}
	return s.d.Audit.Record(ctx, auditing.Entry{
		Action: string(authorize.ActionDeviceApprove), Decision: auditing.DecisionDeny, Reason: auditing.ReasonNoGrant,
		ResourceKind: resourceDevice, Attrs: auditing.Attrs{attrResult: auditing.Code(resultPairingUnknown)},
	})
}

// approvePairingWrite is the second unit of ApprovePairing: lock the pairing, check that it is
// still requested and open, create the device for a (active, approved by a) with the keys of the
// registration, and mark the pairing approved with a fresh collection window.
func (s *Service) approvePairingWrite(ctx context.Context, a actor.Actor, pairingID ids.ID) (identity.Device, error) {
	rec, found, err := s.d.Pairings.LockPairing(ctx, pairingID)
	if err != nil {
		return identity.Device{}, err
	}
	now := s.d.Clock.Now()
	if !found || rec.Status != domain.PairingRequested || !now.Before(rec.ExpiresAt) {
		// Another request approved or ended it between the two units.
		return identity.Device{}, invisible()
	}
	reg, err := registerDevice(rec.Registration)
	if err != nil {
		return identity.Device{}, err
	}
	d, err := device.New(device.Registration{
		ID: reg.DeviceID, UserID: a.ID, Name: reg.Name, Platform: device.Platform(reg.Platform), AppVersion: reg.AppVersion,
		DPoPKey: keyInput(reg.DPoPKey), StepUpKey: stepUpInput(reg.StepUpKey), State: device.StateActive, ApprovedBy: a.ID,
	})
	if err != nil {
		return identity.Device{}, err
	}
	d.CreatedAt = now
	created, err := s.d.Devices.InsertDevice(ctx, d)
	if err != nil {
		return identity.Device{}, err
	}
	if err := rec.Status.To(domain.PairingApproved); err != nil {
		return identity.Device{}, err
	}
	rec.Status, rec.UserID, rec.ApprovedByDeviceID, rec.DeviceID = domain.PairingApproved, a.ID, a.DeviceID, created.ID
	rec.ExpiresAt = now.Add(s.cfg.PairingCodeTTL)
	if err := s.d.Pairings.UpdatePairing(ctx, rec); err != nil {
		return identity.Device{}, err
	}
	err = s.d.Audit.Record(ctx, auditing.Entry{
		Action: string(authorize.ActionDeviceApprove), Decision: auditing.DecisionAllow, ResourceKind: resourceDevice, ResourceID: created.ID,
		Attrs: auditing.Attrs{attrPairingID: auditing.ID(rec.ID), attrDeviceID: auditing.ID(created.ID)},
	})
	return created, err
}

// pairingStep is what the unit of CompletePairing decided.
type pairingStep int

// The decisions of CompletePairing.
const (
	stepWait pairingStep = iota + 1
	stepDone
	stepGone
	stepExpire
	stepReject
	stepWrongKey
)

// CompletePairing implements identity.DeviceApproval (brief M1-38 "CompletePairing"): authorize
// auth.login.complete for the anonymous principal, then in one unit of work lock the pairing: an
// unknown one is gone; a proof key other than the registered dpop key rejects a requested pairing
// (committed) and answers invalid_dpop_proof; a requested pairing answers its status; an approved
// one completes the login of the created device (TrustConfirmed, device_pairing) and is
// completed. An open pairing past expires_at expires (its device revoked with pairing_expired) and
// answers gone, like a rejected or completed one.
func (s *Service) CompletePairing(ctx context.Context, pairingID ids.ID, proofJKT string, clientIP netip.Addr) (identity.PairingOutcome, error) {
	if err := s.authorize(ctx, actor.Anonymous(), authorize.ActionAuthLoginComplete, authorize.Resource{Kind: authorize.ResourceInstance}); err != nil {
		return identity.PairingOutcome{}, err
	}
	var (
		out  identity.PairingOutcome
		step pairingStep
		rec  PairingRecord
	)
	// A rejection must commit although the answer is 401; the operation is exempt from
	// Idempotency-Key, so its unit is its own anyway.
	err := s.d.UoW.Do(committed(ctx), func(ctx context.Context) error {
		var err error
		out, rec, step, err = s.completeStep(ctx, pairingID, proofJKT, clientIP)
		return err
	})
	if err != nil {
		return identity.PairingOutcome{}, err
	}
	switch step {
	case stepWait, stepDone:
		return out, nil
	case stepExpire:
		if err := s.expirePairing(system(committed(ctx)), rec); err != nil {
			return identity.PairingOutcome{}, err
		}
	case stepReject, stepWrongKey:
		return identity.PairingOutcome{}, &identity.DPoPError{Code: identity.DPoPInvalidProof, Nonce: s.d.DPoP.CurrentNonce()}
	case stepGone:
	}
	return identity.PairingOutcome{}, errs.New(errs.CodeGone, "")
}

// completeStep is the unit of work of CompletePairing; it returns the decision and the pairing.
func (s *Service) completeStep(ctx context.Context, pairingID ids.ID, proofJKT string, clientIP netip.Addr) (identity.PairingOutcome, PairingRecord, pairingStep, error) {
	rec, found, err := s.d.Pairings.LockPairing(ctx, pairingID)
	if err != nil {
		return identity.PairingOutcome{}, rec, 0, err
	}
	if !found || !rec.Status.Open() {
		return identity.PairingOutcome{}, rec, stepGone, nil
	}
	if !s.d.Clock.Now().Before(rec.ExpiresAt) {
		return identity.PairingOutcome{}, rec, stepExpire, nil
	}
	if proofJKT == "" || pairingJKT(rec.Registration) != proofJKT {
		if rec.Status != domain.PairingRequested {
			// approved -> rejected is no transition: the device exists and waits for its own key.
			return identity.PairingOutcome{}, rec, stepWrongKey, nil
		}
		rec.Status = domain.PairingRejected
		return identity.PairingOutcome{}, rec, stepReject, s.d.Pairings.UpdatePairing(ctx, rec)
	}
	if rec.Status == domain.PairingRequested {
		return identity.PairingOutcome{PairingID: rec.ID, ExpiresAt: rec.ExpiresAt}, rec, stepWait, nil
	}
	reg, err := registerDevice(rec.Registration)
	if err != nil {
		return identity.PairingOutcome{}, rec, 0, err
	}
	res, err := s.completeLogin(ctx, LoginCompletion{
		UserID: rec.UserID, AuthMethod: identity.AuthDevicePairing, Device: reg, ProofJKT: proofJKT, Trust: TrustConfirmed, ClientIP: clientIP,
	})
	if err != nil {
		return identity.PairingOutcome{}, rec, 0, err
	}
	if err := rec.Status.To(domain.PairingCompleted); err != nil {
		return identity.PairingOutcome{}, rec, 0, err
	}
	rec.Status, rec.CompletedAt = domain.PairingCompleted, s.d.Clock.Now()
	if err := s.d.Pairings.UpdatePairing(ctx, rec); err != nil {
		return identity.PairingOutcome{}, rec, 0, err
	}
	return identity.PairingOutcome{Completed: true, PairingID: rec.ID, Tokens: res.Tokens}, rec, stepDone, nil
}

// expirePairing expires the open pairing rec past expires_at in an own unit of work: requested
// becomes expired; approved becomes expired and its never collected device is revoked with
// pairing_expired, which is the system action device.expire of the approving person's rights. A
// pairing that changed meanwhile stays unchanged.
func (s *Service) expirePairing(ctx context.Context, rec PairingRecord) error {
	var opts []txn.Option
	if rec.Status == domain.PairingApproved {
		opts = append(opts, expireChange(rec.DeviceID, rec.UserID))
	}
	return s.d.UoW.Do(ctx, func(ctx context.Context) error {
		cur, found, err := s.d.Pairings.LockPairing(ctx, rec.ID)
		if err != nil || !found || !cur.Status.Open() || s.d.Clock.Now().Before(cur.ExpiresAt) {
			return err
		}
		if cur.Status == domain.PairingApproved && !txn.InRightsChange(ctx) {
			// It was approved after rec was read; the next run expires it with its rights change.
			return nil
		}
		approved := cur.Status == domain.PairingApproved
		if err := cur.Status.To(domain.PairingExpired); err != nil {
			return err
		}
		cur.Status = domain.PairingExpired
		if err := s.d.Pairings.UpdatePairing(ctx, cur); err != nil || !approved || cur.DeviceID.IsZero() {
			return err
		}
		if err := s.revokeUnconfirmedDevice(ctx, cur.DeviceID, identity.DeviceRevokePairingExpired, nil); err != nil {
			return err
		}
		return s.recordExpire(ctx, cur.DeviceID, identity.DeviceRevokePairingExpired)
	}, opts...)
}
