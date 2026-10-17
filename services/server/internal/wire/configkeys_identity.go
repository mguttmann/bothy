// SPDX-License-Identifier: Apache-2.0

package wire

import "github.com/OWNER/bothy/services/server/internal/platform/config"

// Standard values as 03 section 9 and 08 section 18 write them.
const (
	// dpopProofMaxAgeDefault is the standard of auth.dpop_proof_max_age.
	dpopProofMaxAgeDefault = "60s"
	// thirtySeconds is the standard of auth.clock_leeway and auth.refresh_reuse_grace.
	thirtySeconds = "30s"
	// sessionMaxAgeDefault is the standard of auth.session_max_age.
	sessionMaxAgeDefault = "30d"
)

// identityConfigKeys returns the keys owned by the module identity: the limits of handles and
// display names from docs/spec/03-data-model.md that the domain checks (M1-26), the session and
// refresh durations of 03 section 9 (M1-34), the DPoP windows and the token pepper from
// docs/spec/08-identity.md section 18 (M1-33), and the step-up challenge lifetime, failure limit
// and window and the cleanup interval of 08 section 18 (M1-35), the API token limits and the
// DPoP requirement of 08 section 18 (M1-44), the enrollment code lifetime and attempt limit of
// 08 section 18 (M1-37), the OIDC flow, code, cache, assertion and recheck durations of 08
// section 18 (M1-41), the binding switch of the trusted_header driver of 08 section 18 (M1-43),
// and the inactivity window of devices (M1-45). The page sizes of
// 06-api.md live in apiConfigKeys, because every list reads them.
func identityConfigKeys() []config.Key {
	return []config.Key{
		{
			Name: "principal.handle_max_chars", Kind: config.KindInt, Default: "32", Min: "1",
			Doc:     "Longest handle of a principal in characters.",
			Chapter: chapter03,
		},
		{
			Name: "principal.display_name_max_chars", Kind: config.KindInt, Default: "64", Min: "1",
			Doc:     "Longest display name of a principal in Unicode characters.",
			Chapter: chapter03,
		},
		{
			// At least 1s: the nonce window divides the Unix seconds by it.
			Name: "auth.dpop_nonce_ttl", Kind: config.KindDuration, Default: "5m", Min: "1s",
			Doc:     "Length of a DPoP nonce window; the current and the previous nonce are accepted.",
			Chapter: chapter08,
		},
		{
			Name: "auth.dpop_proof_max_age", Kind: config.KindDuration, Default: dpopProofMaxAgeDefault, Min: "1s",
			Doc:     "Oldest iat a DPoP proof may have.",
			Chapter: chapter08,
		},
		{
			Name: "auth.clock_leeway", Kind: config.KindDuration, Default: thirtySeconds, Min: "0s",
			Doc:     "Allowed clock skew for exp, nbf and iat.",
			Chapter: chapter08,
		},
		{
			Name: "auth.session_max_age", Kind: config.KindDuration, Default: sessionMaxAgeDefault, Min: "1m",
			Doc:     "Absolute end of an app session after its start; then a new login.",
			Chapter: chapter03,
		},
		{
			Name: "auth.refresh_token_ttl", Kind: config.KindDuration, Default: "14d", Min: "1m",
			Doc:     "Lifetime of a refresh token, never beyond the end of its session.",
			Chapter: chapter03,
		},
		{
			Name: "auth.refresh_reuse_grace", Kind: config.KindDuration, Default: thirtySeconds, Min: "0s",
			Doc:     "Grace for the refresh generation just before the current one with the same DPoP key.",
			Chapter: chapter03,
		},
		{
			Name: "identity.last_active_granularity", Kind: config.KindDuration, Default: "5m", Min: "0s",
			Doc:     "At most this often the last activity of a session and its device is written.",
			Chapter: chapter03,
		},
		{
			Name: "auth.step_up_challenge_ttl", Kind: config.KindDuration, Default: "2m", Min: "1s",
			Doc:     "How long a step-up challenge may be completed.",
			Chapter: chapter08,
		},
		{
			Name: "auth.step_up_fail_limit", Kind: config.KindInt, Default: "5", Min: "1",
			Doc:     "Failed step-ups of a session per auth.login_fail_window; then the session is revoked.",
			Chapter: chapter08,
		},
		{
			// Discovery reports it with an enabled provider of kind local (M1-46); the password
			// login of M1-38 reads it too.
			Name: "auth.local_password_enabled", Kind: config.KindBool, Default: defaultTrue,
			Doc:     "Whether a local account may sign in with password and TOTP as fallback.",
			Chapter: chapter08,
		},
		{
			Name: "auth.login_fail_window", Kind: config.KindDuration, Default: defaultQuarterHour, Min: "1s",
			Doc:     "Window in which failed attempts count.",
			Chapter: chapter08,
		},
		{
			Name: "auth.enrollment_code_ttl", Kind: config.KindDuration, Default: "72h", Min: "1m",
			Doc:     "Validity of an enrollment code from its issue.",
			Chapter: chapter08,
		},
		{
			Name: "auth.enrollment_code_max_attempts", Kind: config.KindInt, Default: "5", Min: "1",
			Doc:     "Failed attempts with a matching enrollment code; then the code is invalid.",
			Chapter: chapter08,
		},
		{
			Name: "auth.device_pending_ttl", Kind: config.KindDuration, Default: "24h", Min: "1m",
			Doc:     "Age after which a pending device without approval counts as revoked.",
			Chapter: chapter08,
		},
		{
			Name: "auth.device_verification_code_digits", Kind: config.KindInt, Default: "6", Min: "6", Max: "12",
			Doc:     "Digits of the verification code a pending device shows.",
			Chapter: chapter08,
		},
		{
			Name: "auth.pairing_code_ttl", Kind: config.KindDuration, Default: "5m", Min: "30s",
			Doc:     "Validity of a pairing code, and the window to collect an approved pairing.",
			Chapter: chapter08,
		},
		{
			Name: "auth.pairing_code_chars", Kind: config.KindInt, Default: "8", Min: "8", Max: "16",
			Doc:     "Crockford Base32 characters of a pairing code (5 bits each).",
			Chapter: chapter08,
		},
		{
			Name: "auth.cleanup_interval", Kind: config.KindDuration, Default: "1h", Min: "1s",
			Doc:     "Interval of the job identity.auth_cleanup.",
			Chapter: chapter08,
		},
		{
			Name: "auth.token_pepper_ref", Kind: config.KindSecretRef, Default: "file:<secrets.dir>/token_pepper",
			Doc:     "Secret reference of the server pepper for the HMAC hashes of tokens and codes and the DPoP nonce key; 32 bytes as hex.",
			Chapter: chapter08,
		},
		{
			// The cross-check dpop_required_for refuses a value outside agent, app and personal.
			Name: keyDPoPRequiredFor, Kind: config.KindStringList, Default: `["agent"]`,
			Doc:     "Token kinds (agent, app, personal) whose API tokens must be DPoP-bound.",
			Chapter: chapter08,
		},
		{
			Name: "auth.api_token_max_ttl", Kind: config.KindDuration, Default: "365d", Min: "1s",
			Doc:     "Longest lifetime of an agent or app token.",
			Chapter: chapter08,
		},
		{
			Name: "auth.personal_token_max_ttl", Kind: config.KindDuration, Default: "90d", Min: "1s",
			Doc:     "Longest lifetime of a personal token.",
			Chapter: chapter08,
		},
		{
			Name: "auth.login_flow_ttl", Kind: config.KindDuration, Default: defaultLoginFlowTTL, Min: "1s",
			Doc:     "Validity of an OIDC login flow and of a passkey ceremony (login, enrollment, registration).",
			Chapter: chapter08,
		},
		{
			// The cross-check webauthn_relying_party holds it against the origins (M1-40).
			Name: keyWebAuthnRPID, Kind: config.KindString, Default: "",
			Doc:     "RP ID of the passkeys; empty means the host of server.public_url (an IP address there means no passkeys).",
			Chapter: chapter08,
		},
		{
			Name: keyWebAuthnOrigins, Kind: config.KindStringList, Default: "",
			Doc:     "Allowed origins of the passkeys; empty means the origin of server.public_url; entries may be app origins such as android:apk-key-hash:...",
			Chapter: chapter08,
		},
		{
			Name: "auth.server_code_ttl", Kind: config.KindDuration, Default: denyAggregateWindowDefault, Min: "1s",
			Doc:     "Validity of the one-time login_code of the OIDC callback.",
			Chapter: chapter08,
		},
		{
			Name: "auth.oidc_jwks_cache_ttl", Kind: config.KindDuration, Default: "1h", Min: "1s",
			Doc:     "Cache of the provider metadata (discovery) and keys (JWKS).",
			Chapter: chapter08,
		},
		{
			Name: "auth.oidc_client_assertion_ttl", Kind: config.KindDuration, Default: "2m", Min: "1s",
			Doc:     "Lifetime of a private_key_jwt client assertion.",
			Chapter: chapter08,
		},
		{
			Name: "auth.group_claim_max_age", Kind: config.KindDuration, Default: sessionMaxAgeDefault, Min: "1m",
			Doc:     "Expiry of a membership from a group claim after the last OIDC login that carried it.",
			Chapter: chapter08,
		},
		{
			Name: "auth.idp_recheck_interval", Kind: config.KindDuration, Default: sessionMaxAgeDefault, Min: "1m",
			Doc:     "Age of an OIDC session after which the next refresh needs a new login at the provider.",
			Chapter: chapter08,
		},
		{
			Name: "auth.trusted_header_bind_requests", Kind: config.KindBool, Default: defaultTrue,
			Doc:     "Every later request of a session started with the trusted_header driver must carry the same header login.",
			Chapter: chapter08,
		},
		{
			// At least an hour: a shorter window would revoke a device between two uses.
			Name: "auth.device_inactive_revoke_after", Kind: config.KindDuration, Default: "180d", Min: "1h",
			Doc:     "An active device without contact for this long is revoked (reason inactive).",
			Chapter: chapter08,
		},
	}
}

// The keys of the WebAuthn relying party (M1-40); the cross-check webauthn_relying_party reads
// them.
const (
	keyWebAuthnRPID    = "auth.webauthn_rp_id"
	keyWebAuthnOrigins = "auth.webauthn_origins"
)

// defaultLoginFlowTTL is the standard of auth.login_flow_ttl (08 section 18).
const defaultLoginFlowTTL = "10m"

// keyDPoPRequiredFor names the token kinds that must be DPoP-bound; the cross-check
// dpop_required_for reads it.
const keyDPoPRequiredFor = "auth.dpop_required_for"

// apiConfigKeys returns the page sizes of docs/spec/06-api.md "6. Pagination" (M1-33, the first
// list).
func apiConfigKeys() []config.Key {
	return []config.Key{
		{
			Name: "api.page_size_default", Kind: config.KindInt, Default: "50", Min: "1",
			Doc:     "Page size of a list without limit.",
			Chapter: chapter06,
		},
		{
			Name: "api.page_size_max", Kind: config.KindInt, Default: "200", Min: "1",
			Doc:     "Largest page size; a larger limit is capped.",
			Chapter: chapter06,
		},
	}
}
