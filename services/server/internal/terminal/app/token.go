// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/instance"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/jose"
	"github.com/OWNER/bothy/services/server/internal/terminal"
	"github.com/OWNER/bothy/services/server/internal/terminal/domain"
)

// issueToken builds and signs the session token of g with the token id jti at now (Design note 5
// of M2-44, docs/spec/24-terminal.md section 5.3): a compact JWS with the header alg EdDSA, the kid
// of the active key of purpose terminal and typ terminal-session+jwt, the claims of
// domain.SessionClaims, valid terminal.session_token_ttl. It returns the redacting token and its
// expiry. Errors: unavailable without a signing key (the unit of work rolls back, so no session
// exists without a token), internal for a signer of another algorithm or an incomplete grant.
func (s *Service) issueToken(ctx context.Context, g domain.TokenGrant, jti ids.ID, now time.Time) (terminal.SessionToken, time.Time, error) {
	src := s.tokenSigner()
	if src == nil {
		return terminal.SessionToken{}, time.Time{}, errs.New(errs.CodeUnavailable, "").WithCause(errs.CauseDependencyUnavailable)
	}
	c, err := domain.NewSessionClaims(g, jti, now, s.cfg.SessionTokenTTL)
	if err != nil {
		return terminal.SessionToken{}, time.Time{}, errs.Wrap(errs.CodeInternalError, err, "terminal: session token claims")
	}
	payload, err := c.Payload()
	if err != nil {
		return terminal.SessionToken{}, time.Time{}, errs.Wrap(errs.CodeInternalError, err, "terminal: session token payload")
	}
	sg, err := src.Signer(ctx, instance.PurposeTerminal)
	if err != nil {
		return terminal.SessionToken{}, time.Time{}, err
	}
	if sg.Alg() != domain.TokenAlg {
		return terminal.SessionToken{}, time.Time{}, errs.New(errs.CodeInternalError, "terminal: the signer of purpose terminal is not EdDSA")
	}
	sign := jose.SignerFunc{Algorithm: sg.Alg(), Fn: func(in []byte) ([]byte, error) { return sg.Sign(ctx, in) }}
	raw, err := jose.SignCompact(sign, jose.Header{Alg: domain.TokenAlg, Kid: sg.KeyID(), Typ: domain.TokenTyp}, payload)
	if err != nil {
		if _, ok := errs.As(err); ok {
			return terminal.SessionToken{}, time.Time{}, err
		}
		return terminal.SessionToken{}, time.Time{}, errs.Wrap(errs.CodeInternalError, err, "terminal: signing a session token")
	}
	return terminal.NewSessionToken(raw), time.Unix(c.Exp, 0).UTC(), nil
}
