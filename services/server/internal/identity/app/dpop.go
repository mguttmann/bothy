// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/domain/dpop"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/jose"
	"github.com/OWNER/bothy/services/server/internal/kernel/reqmeta"
)

// ReplayStore is the outgoing port of the verifier for the jti store (adapters/replay).
type ReplayStore interface {
	// CheckAndRemember reports true and remembers key until the given time when key is new or
	// expired, false when key was used before inside its window; one atomic step.
	CheckAndRemember(key [32]byte, until time.Time) bool
}

// dpopTyp is the explicit type of a proof (RFC 9449 section 4.2).
const dpopTyp = "dpop+jwt"

// The log message and keys of a rejected proof; the keys are constants for sloglint.
const (
	msgProofRejected = "dpop proof rejected"
	logKeyCode       = "code"
	logKeyReason     = "reason"
	logKeyCorrID     = "correlation_id"
)

// dpopAlgs is the allowlist of proof algorithms (08 "7.3").
var dpopAlgs = []string{jose.AlgES256, jose.AlgEdDSA}

// DPoPVerifier implements identity.DPoP: the checks of the brief M1-33 "DPoP proof checks" in
// their order, the stateless nonces and the jti store. It is safe for concurrent use.
type DPoPVerifier struct {
	cfg    Config
	key    []byte
	replay ReplayStore
	clk    clock.Clock
	log    *slog.Logger
}

// Compile-time check: the verifier is the DPoP port of identity.
var _ identity.DPoP = (*DPoPVerifier)(nil)

// NewDPoPVerifier returns the verifier with the window lengths of cfg (DPoPNonceTTL at least one
// second, DPoPProofMaxAge above zero, ClockLeeway not negative), the nonce key derived from pepper
// (the raw bytes of auth.token_pepper_ref), the jti store, the clock and the debug log of the
// internal reasons (nil means slog.Default()).
//
// Errors: an empty pepper or a window outside its range.
func NewDPoPVerifier(cfg Config, pepper []byte, replay ReplayStore, clk clock.Clock, log *slog.Logger) (*DPoPVerifier, error) {
	if cfg.DPoPNonceTTL < time.Second || cfg.DPoPProofMaxAge <= 0 || cfg.ClockLeeway < 0 {
		return nil, errors.New("identity: auth.dpop_nonce_ttl must be at least 1s, auth.dpop_proof_max_age above 0 and auth.clock_leeway not negative")
	}
	key, err := dpop.NonceKey(pepper)
	if err != nil {
		return nil, err
	}
	if log == nil {
		log = slog.Default()
	}
	return &DPoPVerifier{cfg: cfg, key: key, replay: replay, clk: clk, log: log}, nil
}

// CurrentNonce implements identity.DPoP.
func (v *DPoPVerifier) CurrentNonce() string {
	return v.nonceAt(v.clk.Now())
}

func (v *DPoPVerifier) nonceAt(t time.Time) string {
	return dpop.EncodeNonce(v.key, dpop.NonceWindow(t, v.cfg.DPoPNonceTTL))
}

// Verify implements identity.DPoP. A failure stops at the first failed check and remembers
// nothing; it is a *identity.DPoPError with the current nonce, and the internal reason goes to
// the debug log with the correlation id only. Errors of ctx come back unchanged.
func (v *DPoPVerifier) Verify(ctx context.Context, r identity.ProofRequest) (identity.VerifiedProof, error) {
	if err := ctx.Err(); err != nil {
		return identity.VerifiedProof{}, err
	}
	now := v.clk.Now()
	proof, err := v.check(r, now)
	if err != nil {
		var f *dpop.ProofError
		if !errors.As(err, &f) {
			f = &dpop.ProofError{Code: dpop.CodeInvalidProof, Reason: "internal"}
		}
		v.log.LogAttrs(ctx, slog.LevelDebug, msgProofRejected,
			slog.String(logKeyCode, f.Code), slog.String(logKeyReason, f.Reason),
			slog.String(logKeyCorrID, reqmeta.CorrelationID(ctx)))
		return identity.VerifiedProof{}, &identity.DPoPError{Code: f.Code, Nonce: v.nonceAt(now)}
	}
	return proof, nil
}

// check runs checks 1 to 9; every error is a *dpop.ProofError.
func (v *DPoPVerifier) check(r identity.ProofRequest, now time.Time) (identity.VerifiedProof, error) {
	invalid := func(reason string) error { return &dpop.ProofError{Code: dpop.CodeInvalidProof, Reason: reason} }
	if len(r.Proofs) != 1 {
		return identity.VerifiedProof{}, invalid("proof_count")
	}
	j, err := jose.ParseCompactJWS(r.Proofs[0], dpopAlgs, dpopTyp)
	if err != nil {
		return identity.VerifiedProof{}, invalid("jws_invalid")
	}
	if j.Header.JWK == nil {
		return identity.VerifiedProof{}, invalid("jwk_missing")
	}
	k, err := jose.ParseJWK(j.Header.JWK)
	if err != nil {
		return identity.VerifiedProof{}, invalid("jwk_invalid")
	}
	if err := j.Verify(k); err != nil {
		return identity.VerifiedProof{}, invalid("signature")
	}
	c, err := dpop.ParseClaims(j.Payload, r.AccessToken != "")
	if err != nil {
		return identity.VerifiedProof{}, err
	}
	htu, err := dpop.CheckRequest(c, dpop.Expect{
		Method: r.Method, URL: r.URL, Now: now, MaxAge: v.cfg.DPoPProofMaxAge, Leeway: v.cfg.ClockLeeway,
	})
	if err != nil {
		return identity.VerifiedProof{}, err
	}
	stale, err := dpop.CheckNonce(v.key, c.Nonce, now, v.cfg.DPoPNonceTTL)
	if err != nil {
		return identity.VerifiedProof{}, err
	}
	jkt := k.Thumbprint()
	if err := dpop.CheckBinding(c, r.AccessToken, jkt, r.ExpectedJKT); err != nil {
		return identity.VerifiedProof{}, err
	}
	until := now.Add(v.cfg.DPoPProofMaxAge + v.cfg.ClockLeeway)
	if !v.replay.CheckAndRemember(dpop.ReplayKey(htu, c.JTI), until) {
		return identity.VerifiedProof{}, invalid("jti_replayed")
	}
	return identity.VerifiedProof{JKT: jkt, Alg: j.Header.Alg, PublicJWK: k.Canonical(), IssuedAt: c.IAT, NonceStale: stale}, nil
}
