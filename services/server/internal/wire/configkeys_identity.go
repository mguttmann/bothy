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
// DPoP requirement of 08 section 18 (M1-44), and the enrollment code lifetime and attempt limit of
// 08 section 18 (M1-37). The page sizes of
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
			Name: "auth.login_fail_window", Kind: config.KindDuration, Default: "15m", Min: "1s",
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
	}
}

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
