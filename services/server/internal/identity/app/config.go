// SPDX-License-Identifier: Apache-2.0

package app

import "time"

// Config holds the configuration of the identity module, bound in internal/wire from the keys of
// docs/spec/03-data-model.md section 9, docs/spec/08-identity.md section 18, docs/spec/06-api.md
// and docs/spec/16-operations.md (wire.IdentityConfig).
type Config struct {
	// HandleMaxChars is principal.handle_max_chars, the longest handle in characters.
	HandleMaxChars int `config:"principal.handle_max_chars"`
	// DisplayNameMaxChars is principal.display_name_max_chars, the longest display name in
	// Unicode characters (runes).
	DisplayNameMaxChars int `config:"principal.display_name_max_chars"`
	// DPoPNonceTTL is auth.dpop_nonce_ttl, the length of a nonce window (at least one second).
	DPoPNonceTTL time.Duration `config:"auth.dpop_nonce_ttl"`
	// DPoPProofMaxAge is auth.dpop_proof_max_age, the oldest iat a proof may have.
	DPoPProofMaxAge time.Duration `config:"auth.dpop_proof_max_age"`
	// ClockLeeway is auth.clock_leeway, how far iat may lie in the future.
	ClockLeeway time.Duration `config:"auth.clock_leeway"`
	// TokenPepperRef is auth.token_pepper_ref, the secret reference of the token pepper (32
	// bytes as hex); the nonce key derives from it.
	TokenPepperRef string `config:"auth.token_pepper_ref"`
	// PublicURL is server.public_url, the origin of every expected htu.
	PublicURL string `config:"server.public_url"`
	// PageSizeDefault is api.page_size_default, the page size without limit.
	PageSizeDefault int `config:"api.page_size_default"`
	// PageSizeMax is api.page_size_max; larger limits are capped.
	PageSizeMax int `config:"api.page_size_max"`
	// StepUpMaxAge is auth.step_up_max_age, how long a step-up confirmation holds; getMe
	// reports its end (M1-31).
	StepUpMaxAge time.Duration `config:"auth.step_up_max_age"`
	// AccessTokenTTL is auth.access_token_ttl, the lifetime of an access token (M1-34).
	AccessTokenTTL time.Duration `config:"auth.access_token_ttl"`
	// RefreshTokenTTL is auth.refresh_token_ttl, the lifetime of a refresh token, never beyond the
	// session end.
	RefreshTokenTTL time.Duration `config:"auth.refresh_token_ttl"`
	// SessionMaxAge is auth.session_max_age, the absolute end of an app session after its start.
	SessionMaxAge time.Duration `config:"auth.session_max_age"`
	// RefreshReuseGrace is auth.refresh_reuse_grace, how long the generation just before the
	// current one may rotate again with the family key (03 refresh_tokens item 2).
	RefreshReuseGrace time.Duration `config:"auth.refresh_reuse_grace"`
	// LastActiveGranularity is identity.last_active_granularity: last_seen_at of a session and its
	// device is written at most this often.
	LastActiveGranularity time.Duration `config:"identity.last_active_granularity"`
	// StepUpChallengeTTL is auth.step_up_challenge_ttl, how long a step-up challenge may be
	// completed (M1-35).
	StepUpChallengeTTL time.Duration `config:"auth.step_up_challenge_ttl"`
	// StepUpFailLimit is auth.step_up_fail_limit: this many failed step-ups of a session inside
	// LoginFailWindow revoke the session (08 section 13).
	StepUpFailLimit int `config:"auth.step_up_fail_limit"`
	// LoginFailWindow is auth.login_fail_window, the window failed attempts count in.
	LoginFailWindow time.Duration `config:"auth.login_fail_window"`
	// EnrollmentCodeTTL is auth.enrollment_code_ttl, how long an enrollment code is valid from its
	// issue (M1-37).
	EnrollmentCodeTTL time.Duration `config:"auth.enrollment_code_ttl"`
	// EnrollmentCodeMaxAttempts is auth.enrollment_code_max_attempts: a code with this many failed
	// attempts is invalid (08 section 3.2).
	EnrollmentCodeMaxAttempts int `config:"auth.enrollment_code_max_attempts"`
	// DevicePendingTTL is auth.device_pending_ttl: a pending device older than this counts as
	// revoked and is revoked with pending_expired (08 section 5.2, M1-38).
	DevicePendingTTL time.Duration `config:"auth.device_pending_ttl"`
	// DeviceVerificationCodeDigits is auth.device_verification_code_digits, the length of the code
	// a pending device shows.
	DeviceVerificationCodeDigits int `config:"auth.device_verification_code_digits"`
	// PairingCodeTTL is auth.pairing_code_ttl, how long a pairing code is valid and how long an
	// approved pairing may be collected (08 section 5.3).
	PairingCodeTTL time.Duration `config:"auth.pairing_code_ttl"`
	// PairingCodeChars is auth.pairing_code_chars, the Crockford Base32 characters of a pairing
	// code.
	PairingCodeChars int `config:"auth.pairing_code_chars"`
	// CleanupInterval is auth.cleanup_interval, the period of the job identity.auth_cleanup.
	CleanupInterval time.Duration `config:"auth.cleanup_interval"`
	// APITokenMaxTTL is auth.api_token_max_ttl, the longest lifetime of an agent or app token
	// (M1-44, 08 section 12).
	APITokenMaxTTL time.Duration `config:"auth.api_token_max_ttl"`
	// PersonalTokenMaxTTL is auth.personal_token_max_ttl, the longest lifetime of a personal
	// token.
	PersonalTokenMaxTTL time.Duration `config:"auth.personal_token_max_ttl"`
	// DPoPRequiredFor is auth.dpop_required_for: the token kinds (agent, app, personal) whose
	// tokens must be DPoP-bound.
	DPoPRequiredFor []string `config:"auth.dpop_required_for"`
	// LoginFlowTTL is auth.login_flow_ttl, how long a passkey ceremony (login, enrollment,
	// registration) may be completed after its begin (M1-40).
	LoginFlowTTL time.Duration `config:"auth.login_flow_ttl"`
	// WebAuthnRPID is auth.webauthn_rp_id, the RP ID of the passkeys; "" means the host of
	// server.public_url (domain.ResolveRelyingParty).
	WebAuthnRPID string `config:"auth.webauthn_rp_id"`
	// WebAuthnOrigins is auth.webauthn_origins, the allowed origins of the passkeys; empty means
	// the origin of server.public_url.
	WebAuthnOrigins []string `config:"auth.webauthn_origins"`
}
