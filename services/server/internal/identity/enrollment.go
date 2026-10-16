// SPDX-License-Identifier: Apache-2.0

package identity

import (
	"context"
	"net/netip"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// EnrollmentCode is a newly issued enrollment code (schema EnrollmentCode, 08 section 3.2). Code
// is the display form behind the redacting Token: it leaves the server once, in the response that
// issued it, and prints as [redacted] everywhere else.
type EnrollmentCode struct {
	// ID is enrollment_codes.id; logs name a code only by it.
	ID ids.ID
	// Code is the display form, en_ plus eight groups of four Crockford Base32 characters.
	Code Token
	// Purpose is first_login, recovery or identity_link.
	Purpose string
	// ExpiresAt is enrollment_codes.expires_at.
	ExpiresAt time.Time
}

// RedeemEnrollment is the input of Enrollment.RedeemEnrollmentCode (schema EnrollRedeemRequest plus the
// request facts).
type RedeemEnrollment struct {
	// Code is the enrollment code as the person typed it; case, hyphens and spaces do not count.
	Code string
	// Device is the registration of the redeeming device; UserID, State and ApprovedBy are set by
	// the login completion and ignored here.
	Device RegisterDevice
	// Proof is the DPoP proof of the request, verified as a login proof without ath.
	Proof ProofRequest
	// ClientIP is the client address of the request; the zero Addr for none.
	ClientIP netip.Addr
}

// RedeemResult is the answer of a redemption: the tokens of the new session on the active device.
type RedeemResult struct {
	// Tokens are the token pair with session and device (schema TokenResponse); zero for the
	// console form.
	Tokens TokenSet
	// Console is the result of the console form of a login (M6-01); nil for the app form.
	Console *ConsoleLogin
	// Nonce is the current DPoP nonce when the proof used the previous one, for the DPoP-Nonce
	// header of the response; "" otherwise.
	Nonce string
}

// Enrollment is the incoming port of the enrollment code operations (M1-37): redeeming a code with
// device keys only and issuing a code for a person. Errors are kernel/errs errors; every rejected
// code is the same 401 unauthenticated.
type Enrollment interface {
	// RedeemEnrollmentCode authorizes auth.login.complete for the anonymous principal, verifies the DPoP proof,
	// matches the code (first_login or recovery) and completes the login with a confirmed device.
	// Errors: invalid_dpop_proof or use_dpop_nonce (the code is not looked up), unauthenticated for
	// every rejected code, validation_failed or conflict for a rejected device (counted as a
	// failed attempt of the code).
	RedeemEnrollmentCode(ctx context.Context, in RedeemEnrollment) (RedeemResult, error)
	// CreateEnrollmentCode issues a code for the active person userID (user.update, owner with step-up); the
	// purpose follows from the local identity and the devices of the person. Errors: the decision
	// of authorize, not_found, conflict with reason user_not_active.
	CreateEnrollmentCode(ctx context.Context, a actor.Actor, userID ids.ID) (EnrollmentCode, error)
}
