// SPDX-License-Identifier: Apache-2.0

package identity

import (
	"context"
	"net/netip"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
)

// SetLocalCredential is the input of LocalCredentials.SetLocalCredential (schema
// LocalCredentialSet). Nil members are absent; at least one must be set.
type SetLocalCredential struct {
	// Password is the new password; it never reaches a log, an audit detail or an event.
	Password *string
	// TOTPEnroll starts a new TOTP enrollment when true.
	TOTPEnroll *bool
	// TOTPCode confirms the pending or current TOTP secret.
	TOTPCode *string
}

// LocalCredentialResult is the answer of SetLocalCredential (schema LocalCredentialResult).
type LocalCredentialResult struct {
	// PasswordSet reports a stored password.
	PasswordSet bool
	// TOTPEnabled reports a confirmed TOTP enrollment (the password path is established).
	TOTPEnabled bool
	// TOTPSecretURI is the otpauth URI of a new enrollment behind the redacting Token; the zero
	// Token when the request did not enroll. It leaves the server once.
	TOTPSecretURI Token
}

// PasswordLogin is the input of LocalCredentials.LoginWithPassword (schema PasswordLoginRequest
// plus the request facts).
type PasswordLogin struct {
	// Console selects the console form (client = console, M6-01): no Device and no Proof; the
	// login registers a web device and starts a console session with a cookie instead of tokens.
	Console bool
	// LoginName is the subject of the local identity as typed; it is trimmed and lowercased.
	LoginName string
	// Password and TOTPCode are the secrets of the attempt; they are never logged.
	Password string
	TOTPCode string
	// Device is the registration of the logging-in device.
	Device RegisterDevice
	// Proof is the DPoP proof of the request, verified as a login proof without ath.
	Proof ProofRequest
	// ClientIP is the client address of the request; the zero Addr for none.
	ClientIP netip.Addr
}

// LocalCredentials is the incoming port of local accounts with password and TOTP (M1-39,
// docs/spec/08-identitaet.md section 2.5). Errors are kernel/errs errors.
type LocalCredentials interface {
	// SetLocalCredential sets the password, starts a TOTP enrollment or confirms it for the
	// caller's own local identity (local_credential.set, step-up once the password path is
	// established). Errors: feature_disabled when auth.local_password_enabled is false, the
	// decision of authorize, conflict with reason no_local_identity, validation_failed with
	// errors[] (too_short, common_password, totp_invalid, required).
	SetLocalCredential(ctx context.Context, a actor.Actor, in SetLocalCredential) (LocalCredentialResult, error)
	// LoginWithPassword logs in with login name, password and TOTP code; a new device is
	// pending. Errors: not_found when the password login is off, invalid_dpop_proof or
	// use_dpop_nonce, the one unauthenticated of every rejected attempt, validation_failed or
	// conflict for a rejected device registration.
	LoginWithPassword(ctx context.Context, in PasswordLogin) (RedeemResult, error)
}
