// SPDX-License-Identifier: Apache-2.0

// Package webauthn is the WebAuthn adapter of the identity module (M1-40): Verifier implements
// app.PasskeyVerifier with go-webauthn (github.com/go-webauthn/webauthn, BSD-3-Clause, pinned in
// go.mod). It is the only package that imports the library (docs/spec/02-architektur.md
// "Schichten im Modul"). The relying party comes from the configuration
// (domain.ResolveRelyingParty); userVerification = required, residentKey = preferred,
// attestation = none and the algorithms ES256, EdDSA and RS256 are fixed here
// (docs/spec/08-identitaet.md "2.5"). The adapter keeps no ceremony state: the use cases store
// the session in auth_challenges and hand the checked challenge back in.
//
// Example:
//
//	v, err := webauthn.New(rp, cfg.LoginFlowTTL)
//	opts, err := v.RequestOptions(allow, challenge)
package webauthn

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	gowebauthn "github.com/go-webauthn/webauthn/webauthn"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/protocol/webauthncose"

	"github.com/OWNER/bothy/services/server/internal/identity/app"
	"github.com/OWNER/bothy/services/server/internal/identity/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Algorithms are the COSE algorithms offered and accepted, in order of preference: ES256 (-7),
// EdDSA (-8) and RS256 (-257, needed by Windows Hello); nothing else (08 "2.5").
var Algorithms = []webauthncose.COSEAlgorithmIdentifier{webauthncose.AlgES256, webauthncose.AlgEdDSA, webauthncose.AlgRS256}

// Verifier implements app.PasskeyVerifier for one relying party. It is safe for concurrent use.
type Verifier struct {
	wa   *gowebauthn.WebAuthn
	rpID string
}

// Compile-time check: the outgoing port of the ceremonies.
var _ app.PasskeyVerifier = (*Verifier)(nil)

// New returns the verifier of rp with timeout as the client hint of the options (the server
// enforces auth.login_flow_ttl through auth_challenges.expires_at). Errors: a disabled relying
// party, an app origin of a form no client conveys, or a configuration go-webauthn refuses; the
// start fails with them.
func New(rp domain.RelyingParty, timeout time.Duration) (*Verifier, error) {
	if !rp.Enabled {
		return nil, errors.New("identity: the WebAuthn relying party is not enabled")
	}
	var origins, opaque []string
	for _, o := range rp.Origins {
		if protocol.IsOpaqueOrigin(o) {
			opaque = append(opaque, o)
			continue
		}
		origins = append(origins, o)
	}
	no := false
	cfg := &gowebauthn.Config{
		// The display name is only a label; the product name lives in the instance settings
		// (docs/DECISIONS.md M1-40).
		RPID: rp.ID, RPDisplayName: rp.ID, RPOrigins: origins, RPOpaqueOrigins: opaque,
		AttestationPreference: protocol.PreferNoAttestation,
		AuthenticatorSelection: protocol.AuthenticatorSelection{
			RequireResidentKey: &no, ResidentKey: protocol.ResidentKeyRequirementPreferred, UserVerification: protocol.VerificationRequired,
		},
		Timeouts: gowebauthn.TimeoutsConfig{
			Login:        gowebauthn.TimeoutConfig{Timeout: timeout, TimeoutUVD: timeout},
			Registration: gowebauthn.TimeoutConfig{Timeout: timeout, TimeoutUVD: timeout},
		},
	}
	wa, err := gowebauthn.New(cfg)
	if err != nil {
		return nil, fmt.Errorf("identity: auth.webauthn_rp_id or auth.webauthn_origins: %w", err)
	}
	return &Verifier{wa: wa, rpID: rp.ID}, nil
}

// user adapts app.CeremonyUser to the user interface of go-webauthn.
type user struct{ u app.CeremonyUser }

// WebAuthnID returns the user handle, the 16 bytes of the principal ID.
func (u user) WebAuthnID() []byte { return domain.UserHandle(u.u.ID) }

// WebAuthnName returns the name of the creation options.
func (u user) WebAuthnName() string { return u.u.Name }

// WebAuthnDisplayName returns the display name of the creation options.
func (u user) WebAuthnDisplayName() string { return u.u.DisplayName }

// WebAuthnCredentials returns the stored passkeys as library credentials.
func (u user) WebAuthnCredentials() []gowebauthn.Credential {
	out := make([]gowebauthn.Credential, 0, len(u.u.Credentials))
	for _, p := range u.u.Credentials {
		out = append(out, credential(p))
	}
	return out
}

// credential converts a stored passkey. The flags are the stored ones, so the library's check
// that backup eligibility never changes holds against the registration.
func credential(p app.StoredPasskey) gowebauthn.Credential {
	c := gowebauthn.Credential{
		ID: p.CredentialID, PublicKey: p.PublicKeyCOSE,
		Flags:         gowebauthn.CredentialFlags{UserPresent: true, UserVerified: true, BackupEligible: p.BackupEligible, BackupState: p.BackupState},
		Authenticator: gowebauthn.Authenticator{SignCount: p.SignCount},
	}
	if !p.AAGUID.IsZero() {
		b := [16]byte(p.AAGUID)
		c.Authenticator.AAGUID = b[:]
	}
	for _, t := range p.Transports {
		c.Transport = append(c.Transport, protocol.AuthenticatorTransport(t))
	}
	return c
}

// parameters are the credential parameters of Algorithms.
func parameters() []protocol.CredentialParameter {
	out := make([]protocol.CredentialParameter, 0, len(Algorithms))
	for _, a := range Algorithms {
		out = append(out, protocol.CredentialParameter{Type: protocol.PublicKeyCredentialType, Algorithm: a})
	}
	return out
}

// withChallenge sets the challenge of the creation options to the value the use case stored as
// HMAC; go-webauthn has a login option for it but none for a registration.
func withChallenge(challenge []byte) gowebauthn.RegistrationOption {
	return func(o *protocol.PublicKeyCredentialCreationOptions) error {
		o.Challenge = challenge
		return nil
	}
}

// CreationOptions implements app.PasskeyVerifier.
func (v *Verifier) CreationOptions(u app.CeremonyUser, challenge []byte) ([]byte, error) {
	wu := user{u}
	excl := make([]protocol.CredentialDescriptor, 0, len(u.Credentials))
	for _, c := range wu.WebAuthnCredentials() {
		excl = append(excl, c.Descriptor())
	}
	creation, _, err := v.wa.BeginRegistration(wu, withChallenge(challenge), gowebauthn.WithExclusions(excl),
		gowebauthn.WithCredentialParameters(parameters()), gowebauthn.WithConveyancePreference(protocol.PreferNoAttestation),
		gowebauthn.WithResidentKeyRequirement(protocol.ResidentKeyRequirementPreferred))
	if err != nil {
		return nil, fmt.Errorf("identity: creation options: %w", err)
	}
	return json.Marshal(creation.Response)
}

// session rebuilds the session data of go-webauthn from the checked challenge: the RP ID of the
// configuration, required user verification, the user handle and, for a creation, the allowed
// algorithms. The allow list and the person are enforced by the use case through lookup.
func (v *Verifier) session(challenge, userHandle []byte, creation bool) gowebauthn.SessionData {
	s := gowebauthn.SessionData{
		Challenge: base64.RawURLEncoding.EncodeToString(challenge), RelyingPartyID: v.rpID, UserID: userHandle,
		UserVerification: protocol.VerificationRequired,
	}
	if creation {
		s.CredParams = parameters()
	}
	return s
}

// VerifyCreation implements app.PasskeyVerifier.
func (v *Verifier) VerifyCreation(u app.CeremonyUser, challenge []byte, responseJSON []byte) (app.NewPasskey, error) {
	parsed, err := protocol.ParseCredentialCreationResponseBytes(responseJSON)
	if err != nil {
		return app.NewPasskey{}, rejected(err)
	}
	wu := user{u}
	c, err := v.wa.CreateCredential(wu, v.session(challenge, wu.WebAuthnID(), true), parsed)
	if err != nil {
		return app.NewPasskey{}, rejected(err)
	}
	out := app.NewPasskey{
		CredentialID: c.ID, PublicKeyCOSE: c.PublicKey, SignCount: c.Authenticator.SignCount,
		BackupEligible: c.Flags.BackupEligible, BackupState: c.Flags.BackupState,
	}
	if len(c.Authenticator.AAGUID) == 16 {
		out.AAGUID = ids.ID([16]byte(c.Authenticator.AAGUID))
	}
	for _, t := range c.Transport {
		out.Transports = append(out.Transports, string(t))
	}
	return out, nil
}

// RequestOptions implements app.PasskeyVerifier.
func (v *Verifier) RequestOptions(allow []app.StoredPasskey, challenge []byte) ([]byte, error) {
	desc := make([]protocol.CredentialDescriptor, 0, len(allow))
	for _, p := range allow {
		c := credential(p)
		desc = append(desc, c.Descriptor())
	}
	assertion, _, err := v.wa.BeginDiscoverableLogin(gowebauthn.WithChallenge(challenge), gowebauthn.WithAllowedCredentials(desc),
		gowebauthn.WithUserVerification(protocol.VerificationRequired))
	if err != nil {
		return nil, fmt.Errorf("identity: request options: %w", err)
	}
	return json.Marshal(assertion.Response)
}

// VerifyAssertion implements app.PasskeyVerifier: it parses the response, resolves the person
// through lookup (an error of lookup passes unchanged), then lets go-webauthn verify origin, RP
// ID hash, the user present and user verified flags, the signature with the stored COSE key, the
// user handle against the person and the backup flags.
func (v *Verifier) VerifyAssertion(challenge []byte, responseJSON []byte,
	lookup func(credentialID, userHandle []byte) (app.CeremonyUser, error)) (app.AssertionResult, error) {
	parsed, err := protocol.ParseCredentialRequestResponseBytes(responseJSON)
	if err != nil {
		return app.AssertionResult{}, rejected(err)
	}
	var handle []byte
	if len(parsed.Response.UserHandle) > 0 {
		handle = parsed.Response.UserHandle
	}
	u, err := lookup(parsed.RawID, handle)
	if err != nil {
		return app.AssertionResult{}, err
	}
	var stored app.StoredPasskey
	found := false
	for _, p := range u.Credentials {
		if domain.SameBytes(p.CredentialID, parsed.RawID) {
			stored, found = p, true
			break
		}
	}
	if !found {
		return app.AssertionResult{}, fmt.Errorf("%w: the credential is not one of the person", app.ErrPasskeyRejected)
	}
	wu := user{u}
	if _, err := v.wa.ValidateLogin(wu, v.session(challenge, wu.WebAuthnID(), false), parsed); err != nil {
		return app.AssertionResult{}, rejected(err)
	}
	flags := parsed.Response.AuthenticatorData.Flags
	return app.AssertionResult{
		User: u, Passkey: stored, SignCount: parsed.Response.AuthenticatorData.Counter, BackupState: flags.HasBackupState(),
	}, nil
}

// rejected wraps a refusal of go-webauthn into app.ErrPasskeyRejected; the library text stays in
// the chain for the log, never in an answer.
func rejected(err error) error { return fmt.Errorf("%w: %w", app.ErrPasskeyRejected, err) }
