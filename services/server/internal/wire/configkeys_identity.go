// SPDX-License-Identifier: Apache-2.0

package wire

import "github.com/OWNER/bothy/services/server/internal/platform/config"

// dpopProofMaxAgeDefault is the standard of auth.dpop_proof_max_age as 08 section 18 writes it.
const dpopProofMaxAgeDefault = "60s"

// identityConfigKeys returns the keys owned by the module identity: the limits of handles and
// display names from docs/spec/03-datenmodell.md that the domain checks (M1-26), the DPoP windows
// and the token pepper from docs/spec/08-identitaet.md section 18 (M1-33). The page sizes of
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
			Name: "auth.clock_leeway", Kind: config.KindDuration, Default: "30s", Min: "0s",
			Doc:     "Allowed clock skew for exp, nbf and iat.",
			Chapter: chapter08,
		},
		{
			Name: "auth.token_pepper_ref", Kind: config.KindSecretRef, Default: "file:<secrets.dir>/token_pepper",
			Doc:     "Secret reference of the server pepper for the HMAC hashes of tokens and codes and the DPoP nonce key; 32 bytes as hex.",
			Chapter: chapter08,
		},
	}
}

// apiConfigKeys returns the page sizes of docs/spec/06-api.md "6. Paginierung" (M1-33, the first
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
