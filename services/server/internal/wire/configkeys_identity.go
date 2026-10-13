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
// docs/spec/08-identity.md section 18 (M1-33). The page sizes of
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
			Name: "auth.token_pepper_ref", Kind: config.KindSecretRef, Default: "file:<secrets.dir>/token_pepper",
			Doc:     "Secret reference of the server pepper for the HMAC hashes of tokens and codes and the DPoP nonce key; 32 bytes as hex.",
			Chapter: chapter08,
		},
	}
}

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
