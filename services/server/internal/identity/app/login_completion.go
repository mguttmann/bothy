// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"io"
	"net/netip"
	"slices"

	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/domain"
	"github.com/OWNER/bothy/services/server/internal/identity/domain/device"
	"github.com/OWNER/bothy/services/server/internal/instance"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// DeviceTrust says whether the login path confirms a new device (08 section 5.2).
type DeviceTrust int

// The two trust levels of a login path.
const (
	// TrustConfirmed is an enrollment code or a device pairing: a new device is active.
	TrustConfirmed DeviceTrust = iota + 1
	// TrustUnconfirmed is a passkey, password, OIDC or trusted header login: a new device is
	// pending until it is approved.
	TrustUnconfirmed
)

// LoginCompletion is the input of CompleteLogin: who logged in, how, and from which device.
type LoginCompletion struct {
	// UserID is the person.
	UserID ids.ID
	// IdentityID is the provider identity of the login; nil for a login without one.
	IdentityID *ids.ID
	// AuthMethod is one of auth_sessions.auth_method.
	AuthMethod identity.AuthMethod
	// Device is the registration of the request (schema DeviceRegistration); UserID, State and
	// ApprovedBy are set here.
	Device identity.RegisterDevice
	// ProofJKT is the RFC 7638 thumbprint of the key that signed the DPoP proof of the request.
	ProofJKT string
	// Trust decides the state of a new device.
	Trust DeviceTrust
	// ClientIP is the client address of the request, stored as auth_sessions.last_ip.
	ClientIP netip.Addr
	// ProviderID is the identity provider of a driver login, recorded as provider_id in the allow
	// entry; zero for none (M1-43).
	ProviderID ids.ID
	// AllowPending lets a person in status pending log in (a person provisioned as pending by OIDC
	// or a provider driver gets tokens but only the self information, 08 section 3.1 item 3 and
	// R-ID-14); disabled and deleted never log in.
	AllowPending bool
	// Provisioned marks a login that created the person (08 section 3.1 step 3); the allow entry
	// says so.
	Provisioned bool
	// Sync runs inside the unit of work after the person check and before the device and the
	// session are written: the claim group sync of an OIDC login (M1-42); nil for none. Its
	// error fails the login.
	Sync func(ctx context.Context, userID ids.ID) error
}

// LoginResult is a completed login: the session, its device and the token pair. NewDevice reports
// that the login registered the device.
type LoginResult struct {
	Session   identity.Session
	Device    identity.Device
	Tokens    identity.TokenSet
	NewDevice bool
}

// SigningKeys is the part of instance.Keys that bothy setup needs: the active keys must exist
// before the first owner can log in (ADR 0066).
type SigningKeys interface {
	// EnsureActive creates an active key for every managed purpose that has none.
	EnsureActive(ctx context.Context) error
}

// ServiceDeps are the collaborators of Service (M1-37).
type ServiceDeps struct {
	// People reads, inserts and locks principals, users and owners.
	People Store
	// Devices reads and inserts devices and device keys.
	Devices DeviceStore
	// Providers reads and inserts the local identity provider.
	Providers ProviderStore
	// Identities reads and inserts identities and marks their last login.
	Identities IdentityStore
	// Enrollment issues and matches the enrollment codes.
	Enrollment *EnrollmentCodes
	// Auth starts the sessions (Sessions).
	Auth identity.Auth
	// DPoP verifies the proof of a redemption.
	DPoP identity.DPoP
	// Instance locks the instance row and completes the setup (bothy setup).
	Instance instance.State
	// Keys creates the signing keys before the bootstrap.
	Keys SigningKeys
	// Locales parses the locale of the first owner.
	Locales Locales
	// UoW runs every write; its registry holds the rights, sync and audit hooks.
	UoW txn.UnitOfWork
	// Authorizer decides auth.login.complete and user.update.
	Authorizer authorize.Authorizer
	// Audit writes the allow entries and the denials.
	Audit auditing.Recorder
	// Clock is the time of logins and setup; nil means clock.System().
	Clock clock.Clock
	// Sessions lists the sessions of a revoked device and ends the step-up of a session (M1-38).
	Sessions SessionStore
	// Pairings reads and writes device_pairings (M1-38).
	Pairings PairingStore
	// Pepper is the raw token pepper: the HMAC key of verification and pairing codes.
	Pepper []byte
	// TrustedHeader is the driver trusted_header (M1-43); nil keeps the header login off (404).
	TrustedHeader *TrustedHeaderDriver
	// Events appends user.updated for a person the header login provisions; nil appends nothing.
	Events eventlog.Appender
	// Random is the source of pairing codes (M1-38) and of the handle suffixes of people the header
	// login provisions (M1-43); nil means domain.EnrollmentRandom (crypto/rand).
	Random io.Reader
}

// Service holds the login use cases every login path shares (CompleteLogin), the enrollment code
// operations and the host commands of bothy setup (brief M1-37). It implements identity.Enrollment
// and is safe for concurrent use.
type Service struct {
	cfg Config
	d   ServiceDeps
}

// Compile-time check: the incoming port of the enrollment code operations.
var _ identity.Enrollment = (*Service)(nil)

// NewService returns the login service with the principal limits of cfg. Errors: a missing
// collaborator or a limit below 1.
func NewService(cfg Config, d ServiceDeps) (*Service, error) {
	if d.People == nil || d.Devices == nil || d.Providers == nil || d.Identities == nil || d.Enrollment == nil || d.Auth == nil ||
		d.DPoP == nil || d.Instance == nil || d.Keys == nil || d.Locales == nil || d.UoW == nil || d.Authorizer == nil || d.Audit == nil ||
		d.Sessions == nil || d.Pairings == nil || len(d.Pepper) == 0 {
		return nil, errors.New("identity: NewService needs every collaborator and the token pepper")
	}
	if cfg.HandleMaxChars < 1 || cfg.DisplayNameMaxChars < 1 {
		return nil, errors.New("identity: principal limits must be at least 1")
	}
	if cfg.DevicePendingTTL <= 0 || cfg.PairingCodeTTL <= 0 || cfg.PairingCodeChars < 1 ||
		cfg.DeviceVerificationCodeDigits < 1 || cfg.DeviceVerificationCodeDigits > domain.MaxVerificationDigits {
		return nil, errors.New("identity: auth.device_pending_ttl, auth.pairing_code_ttl, auth.pairing_code_chars and auth.device_verification_code_digits out of range")
	}
	if d.Clock == nil {
		d.Clock = clock.System()
	}
	if d.Random == nil {
		d.Random = domain.EnrollmentRandom
	}
	d.Pepper = append([]byte(nil), d.Pepper...)
	return &Service{cfg: cfg, d: d}, nil
}

// Audit attribute names of auth.login.complete (codes and IDs only, 19 SEC-AUD-5).
const (
	attrAuthMethod       = "auth_method"
	attrDeviceID         = "device_id"
	attrSessionID        = "session_id"
	attrNewDevice        = "new_device"
	attrDeviceState      = "device_state"
	attrEnrollmentCodeID = "enrollment_code_id"
	attrProvisioned      = "provisioned"
)

// fieldDeviceID is the field pointer of DeviceRegistration.device_id.
const fieldDeviceID = "/device_id"

// fieldCodeTaken is the field code of a taken device id or handle (M1-37, M1-38, M1-41, M1-43;
// one constant since INT-W14/10).
const fieldCodeTaken = "taken"

// CompleteLogin completes a login of any path in one unit of work (joining the ambient one): the
// person must be active (or pending with AllowPending), the registration valid and its dpop key the key of the DPoP proof; an
// unknown device is created (active for TrustConfirmed, pending otherwise), a known device of the
// same person with the same active key is reused unchanged, any other known device id is
// 409 conflict. Then the session, its refresh family bound to ProofJKT and the token pair are
// created, last_login_at of the identity is set, and exactly one allow entry of
// auth.login.complete is recorded with the person as actor. It is the only place that creates
// devices, sessions and tokens for logins. Errors: unauthenticated for a person that is not
// active, validation_failed for the registration, invalid_dpop_proof for another key, conflict,
// and the store errors.
func (s *Service) CompleteLogin(ctx context.Context, in LoginCompletion) (LoginResult, error) {
	var out LoginResult
	err := s.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		out, err = s.completeLogin(ctx, in)
		return err
	})
	return out, err
}

// completeLogin is CompleteLogin inside its unit of work.
func (s *Service) completeLogin(ctx context.Context, in LoginCompletion) (LoginResult, error) {
	if in.Trust != TrustConfirmed && in.Trust != TrustUnconfirmed || !in.AuthMethod.Valid() {
		return LoginResult{}, errs.New(errs.CodeInternalError, "identity: login completion without trust or auth method")
	}
	if err := s.loginPerson(ctx, in); err != nil {
		return LoginResult{}, err
	}
	dev, isNew, err := s.loginDevice(ctx, in)
	if err != nil {
		return LoginResult{}, err
	}
	set, err := s.d.Auth.StartSession(ctx, identity.StartSession{
		UserID: in.UserID, DeviceID: dev.ID, AuthMethod: in.AuthMethod, IdentityID: in.IdentityID, ClientIP: in.ClientIP,
	})
	if err != nil {
		return LoginResult{}, err
	}
	if in.IdentityID != nil {
		ok, err := s.d.Identities.MarkIdentityLogin(ctx, *in.IdentityID, s.d.Clock.Now())
		if err != nil {
			return LoginResult{}, err
		}
		if !ok {
			return LoginResult{}, errs.New(errs.CodeInternalError, "identity: the identity of the login is missing")
		}
	}
	// The entry names the person who logged in, with the new session and device, even though the
	// request itself is anonymous: a login is the person's action.
	err = s.d.Audit.Record(loginActor(ctx, in.UserID, set), auditing.Entry{
		Action: string(authorize.ActionAuthLoginComplete), Decision: auditing.DecisionAllow,
		ResourceKind: resourceAuthSession, ResourceID: set.Session.ID, Attrs: loginAttrs(in, set, isNew),
	})
	if err != nil {
		return LoginResult{}, err
	}
	// A device that stays pending shows its verification code; the answer goes to that device
	// only (08 section 5.2).
	set.Device.VerificationCode = s.verificationCode(set.Device)
	return LoginResult{Session: set.Session, Device: set.Device, Tokens: set, NewDevice: isNew}, nil
}

// loginPerson checks that the person of in may log in (unauthenticated otherwise) and runs
// in.Sync, the claim group sync of an OIDC login (M1-42), before the device and the session.
func (s *Service) loginPerson(ctx context.Context, in LoginCompletion) error {
	rec, found, err := s.d.People.User(ctx, in.UserID)
	if err != nil {
		return err
	}
	if !found || !mayLogIn(rec.User.Principal.Status, in.AllowPending) {
		return errs.New(errs.CodeUnauthenticated, "")
	}
	if in.Sync == nil {
		return nil
	}
	return in.Sync(ctx, in.UserID)
}

// mayLogIn reports a person status that may log in: active, and pending when allowPending.
func mayLogIn(st domain.Status, allowPending bool) bool {
	return st == domain.StatusActive || allowPending && st == domain.StatusPending
}

// loginAttrs are the details of the allow entry of a login: method, device, session, whether the
// device is new and its state, whether the login provisioned the person and the provider of a
// driver login (M1-43); codes and IDs only (SEC-AUD-5).
func loginAttrs(in LoginCompletion, set identity.TokenSet, isNew bool) auditing.Attrs {
	attrs := auditing.Attrs{
		attrAuthMethod: auditing.Code(string(in.AuthMethod)), attrDeviceID: auditing.ID(set.Device.ID),
		attrSessionID: auditing.ID(set.Session.ID), attrNewDevice: auditing.Bool(isNew),
		attrDeviceState: auditing.Code(string(set.Device.State)),
	}
	if in.Provisioned {
		attrs[attrProvisioned] = auditing.Bool(true)
	}
	if !in.ProviderID.IsZero() {
		attrs[attrProviderID] = auditing.ID(in.ProviderID)
	}
	return attrs
}

// loginActor returns ctx with the person who logged in as the principal, with the new session and
// device: the audit entries of a login name the person, not the anonymous request.
func loginActor(ctx context.Context, userID ids.ID, set identity.TokenSet) context.Context {
	return actor.WithActor(ctx, actor.Actor{Kind: actor.KindUser, ID: userID, SessionID: set.Session.ID, DeviceID: set.Device.ID})
}

// loginDevice validates the registration of in, checks its dpop key against the proof key and
// returns the device of the login: a new one, or the known device of the same person with the
// same active key (isNew false, state unchanged).
func (s *Service) loginDevice(ctx context.Context, in LoginCompletion) (identity.Device, bool, error) {
	reg := device.Registration{
		ID: in.Device.DeviceID, UserID: in.UserID, Name: in.Device.Name, Platform: device.Platform(in.Device.Platform),
		AppVersion: in.Device.AppVersion, DPoPKey: keyInput(in.Device.DPoPKey), State: device.StatePending,
	}
	if in.Device.StepUpKey != nil {
		k := keyInput(*in.Device.StepUpKey)
		reg.StepUpKey = &k
	}
	if in.Trust == TrustConfirmed {
		// The code or the pairing is the person's own confirmation, so the person is the approver.
		reg.State, reg.ApprovedBy = device.StateActive, in.UserID
	}
	d, err := device.New(reg)
	if err != nil {
		return identity.Device{}, false, err
	}
	// The pending window counts from the clock of the request (auth.device_pending_ttl).
	d.CreatedAt = s.d.Clock.Now()
	if d.Keys[0].JKT != in.ProofJKT {
		return identity.Device{}, false, &identity.DPoPError{Code: identity.DPoPInvalidProof, Nonce: s.d.DPoP.CurrentNonce()}
	}
	known, found, err := s.d.Devices.Device(ctx, d.ID)
	if err != nil {
		return identity.Device{}, false, err
	}
	if found {
		// A pending device past auth.device_pending_ttl counts as revoked (08 section 5.2).
		if known.UserID != in.UserID || known.State == identity.DeviceStateRevoked || known.KeyThumbprint != in.ProofJKT ||
			pendingExpired(known, s.d.Clock.Now(), s.cfg.DevicePendingTTL) {
			return identity.Device{}, false, errs.New(errs.CodeConflict, "").
				WithFields(errs.FieldError{Pointer: fieldDeviceID, Code: fieldCodeTaken, Detail: "The device id is taken."})
		}
		return known, false, nil
	}
	created, err := s.d.Devices.InsertDevice(ctx, d)
	return created, true, err
}

// countsAsAttempt reports whether the error of a login completion with a matching code is a
// rejection of the request (a failed attempt of the code), not a failure of the server.
func countsAsAttempt(err error) bool {
	return slices.Contains([]errs.Kind{errs.KindBadRequest, errs.KindUnauthenticated, errs.KindConflict, errs.KindUnprocessable}, errs.KindOf(err))
}
