// SPDX-License-Identifier: Apache-2.0

package identity

import (
	"context"
	"encoding/json"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
)

// The codes of DPoPError (06 section 5).
const (
	// DPoPInvalidProof is invalid_dpop_proof.
	DPoPInvalidProof = "invalid_dpop_proof"
	// DPoPUseNonce is use_dpop_nonce.
	DPoPUseNonce = "use_dpop_nonce"
)

// DPoPAlgs is the algs parameter of the DPoP challenge: the algorithms a proof may use.
const DPoPAlgs = "ES256 EdDSA"

// ProofRequest is what one request asks of its DPoP proof.
type ProofRequest struct {
	// Proofs holds every value of the DPoP header; exactly one is accepted.
	Proofs []string
	// Method is the request method (htm).
	Method string
	// URL is the absolute expected htu: the origin of server.public_url plus the escaped request
	// path (httpapi.ExpectedHTU), never built from the Host header.
	URL string
	// AccessToken is the presented token; "" when none is presented (login endpoints).
	AccessToken string
	// ExpectedJKT is the token's cnf.jkt; "" when no key is expected.
	ExpectedJKT string
}

// VerifiedProof is an accepted proof.
type VerifiedProof struct {
	// JKT is the RFC 7638 thumbprint of the proof key.
	JKT string
	// Alg is ES256 or EdDSA.
	Alg string
	// PublicJWK is the canonical public JWK of the proof key.
	PublicJWK json.RawMessage
	// IssuedAt is iat.
	IssuedAt time.Time
	// NonceStale is true when the proof used the previous nonce: the caller attaches
	// CurrentNonce to its response (06 section 3.1 item 2).
	NonceStale bool
}

// DPoP is the one DPoP verifier of the server (RFC 9449; 08 section 7.3). Sessions (M1-34),
// step-up (M1-35), logins (M1-37 to M1-43), API tokens (M1-44) and the gateway (M1-64) use it.
type DPoP interface {
	// CurrentNonce returns the nonce of the current window for the DPoP-Nonce header.
	CurrentNonce() string
	// Verify checks r in the order of the brief M1-33 "DPoP proof checks" and remembers the
	// jti only after every check passed. Errors: *DPoPError for a rejected proof; ctx errors.
	Verify(ctx context.Context, r ProofRequest) (VerifiedProof, error)
}

// DPoPError is a rejected proof. It maps to kind unauthenticated of kernel/errs: Unwrap returns
// the 401 problem with code Code, WWW-Authenticate: DPoP error="<code>", algs="ES256 EdDSA" and
// DPoP-Nonce: Nonce, so the problem writer renders it as it is. The internal reason of the
// rejection is logged by the verifier and never part of the error.
type DPoPError struct {
	// Code is DPoPInvalidProof or DPoPUseNonce.
	Code string
	// Nonce is the current nonce for the DPoP-Nonce header.
	Nonce string
}

// Error returns the code.
func (e *DPoPError) Error() string { return "identity: dpop proof rejected: " + e.Code }

// Unwrap returns the problem of the error (kind unauthenticated) with its two headers. A code
// outside the two becomes invalid_dpop_proof, so the error never maps to another kind.
func (e *DPoPError) Unwrap() error {
	code := errs.CodeInvalidDPoPProof
	if e.Code == DPoPUseNonce {
		code = errs.CodeUseDPoPNonce
	}
	return errs.New(code, "").
		WithHeader("WWW-Authenticate", `DPoP error="`+string(code)+`", algs="`+DPoPAlgs+`"`).
		WithHeader("DPoP-Nonce", e.Nonce)
}
