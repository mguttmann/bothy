// SPDX-License-Identifier: Apache-2.0

package identity

import (
	"context"
	"encoding/json"
	"net/netip"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// StepUpMethodPasskey is auth_sessions.step_up_method of a step-up confirmed with a passkey
// (docs/spec/08-identitaet.md section 6.2, M1-40).
const StepUpMethodPasskey = "passkey"

// WebAuthnChallenge is a stored ceremony challenge (schema WebAuthnChallenge): the challenge_id,
// the options for the browser or app and the end of the ceremony. The challenge value appears
// only inside PublicKey; the server keeps its HMAC.
type WebAuthnChallenge struct {
	// ID is auth_challenges.id, challenge_id in the API.
	ID ids.ID
	// PublicKey is PublicKeyCredentialRequestOptions or PublicKeyCredentialCreationOptions in JSON
	// form.
	PublicKey json.RawMessage
	// ExpiresAt is the end of the ceremony, auth.login_flow_ttl after its begin.
	ExpiresAt time.Time
}

// PasskeyLogin is the input of Passkeys.CompletePasskeyLogin (schema PasskeyLoginCompleteRequest
// plus the request facts).
type PasskeyLogin struct {
	// ChallengeID is the challenge of beginPasskeyLogin.
	ChallengeID ids.ID
	// Credential is the PublicKeyCredential with an assertion response in JSON form.
	Credential json.RawMessage
	// Device is the registration of the logging-in device.
	Device RegisterDevice
	// Proof is the DPoP proof of the request, verified as a login proof without ath.
	Proof ProofRequest
	// ClientIP is the client address of the request; the zero Addr for none.
	ClientIP netip.Addr
}

// PasskeyEnrollment is the input of Passkeys.CompleteEnrollment (schema EnrollCompleteRequest
// plus the request facts). It carries no enrollment code: the challenge row holds it.
type PasskeyEnrollment struct {
	// ChallengeID is the challenge of beginEnrollment.
	ChallengeID ids.ID
	// Credential is the PublicKeyCredential with an attestation response in JSON form.
	Credential json.RawMessage
	// PasskeyName is the name of the new passkey.
	PasskeyName string
	// Device is the registration of the device, which becomes active.
	Device RegisterDevice
	// Proof is the DPoP proof of the request, verified as a login proof without ath.
	Proof ProofRequest
	// ClientIP is the client address of the request; the zero Addr for none.
	ClientIP netip.Addr
}

// Passkeys is the incoming port of the passkey operations (PLAN M1-40; 08 section 2.5): login of
// local accounts, the first setup with an enrollment code, further passkeys and their revocation.
// The ceremony state lives in auth_challenges, so begin and complete may run on different nodes.
// Every error is a kernel/errs error without challenge values, signatures or codes.
type Passkeys interface {
	// BeginPasskeyLogin starts a passkey login (auth.login.begin). With loginName the options name
	// the passkeys of that local account, or one decoy credential ID for an unknown name or a name
	// without passkeys; without it the options are discoverable. Errors: feature_disabled on an
	// instance without a relying party.
	BeginPasskeyLogin(ctx context.Context, loginName string) (WebAuthnChallenge, error)
	// CompletePasskeyLogin verifies the assertion and logs in with a pending device for an unknown
	// device (auth.login.complete). Errors: invalid_dpop_proof or use_dpop_nonce; the same 401
	// unauthenticated for every rejected challenge, assertion or person; validation_failed or
	// conflict for a rejected device.
	CompletePasskeyLogin(ctx context.Context, in PasskeyLogin) (RedeemResult, error)
	// BeginEnrollment checks the enrollment code without counting an attempt and returns creation
	// options for its person. Errors: 401 unauthenticated for every rejected code.
	BeginEnrollment(ctx context.Context, code string) (WebAuthnChallenge, error)
	// CompleteEnrollment registers the first passkey, redeems the code of the challenge and logs
	// in with an active device. A failed verification counts as a failed attempt of the code.
	// Errors: as CompletePasskeyLogin.
	CompleteEnrollment(ctx context.Context, in PasskeyEnrollment) (RedeemResult, error)
	// BeginPasskeyRegistration returns creation options for the caller's further passkey
	// (passkey.register, step-up once an active passkey exists). Errors: the decision of
	// authorize.
	BeginPasskeyRegistration(ctx context.Context, a actor.Actor) (WebAuthnChallenge, error)
	// CompletePasskeyRegistration stores the passkey of the creation response. Errors: the
	// decision of authorize; 400 validation_failed for an unknown, used, expired or foreign
	// challenge and a rejected response; 409 conflict for a credential that exists.
	CompletePasskeyRegistration(ctx context.Context, a actor.Actor, challengeID ids.ID, credential json.RawMessage, name string) (Passkey, error)
	// RevokePasskey revokes the passkey passkeyID of userID (passkey.revoke, self or owner,
	// step-up); a revoked passkey answers without change. Errors: the decision of authorize,
	// not_found for an unknown or foreign passkey.
	RevokePasskey(ctx context.Context, a actor.Actor, userID, passkeyID ids.ID) error
}
