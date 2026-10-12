// SPDX-License-Identifier: Apache-2.0

package app

import "time"

// Config holds the configuration of the identity module, bound in internal/wire from the keys of
// docs/spec/03-datenmodell.md section 9, docs/spec/08-identitaet.md section 18, docs/spec/06-api.md
// and docs/spec/16-betrieb.md (wire.IdentityConfig).
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
}
