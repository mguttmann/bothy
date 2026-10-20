// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"time"

	"github.com/OWNER/bothy/services/server/internal/instance"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/jose"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// InvocationTokenConfig holds the configuration of the invocation tokens, bound in internal/wire
// (wire.InvocationTokenConfigFrom).
type InvocationTokenConfig struct {
	// TTL is agent.invocation_token_ttl: exp = iat + TTL, truncated to whole seconds.
	TTL time.Duration `config:"agent.invocation_token_ttl"`
	// ClockLeeway is auth.clock_leeway, applied to iat and exp of the server's own tokens (08 7.2,
	// ruling D-M2-06-8).
	ClockLeeway time.Duration `config:"auth.clock_leeway"`
}

// InvocationKeys is the part of instance.Keys (M1-32, ADR 0066) the invocation tokens use: the
// active signer of purpose invocation, read fresh per call, and the published key of a kid from
// the per-process verification cache.
type InvocationKeys interface {
	Signer(ctx context.Context, p instance.Purpose) (instance.Signer, error)
	VerificationKey(ctx context.Context, p instance.Purpose, kid string) (instance.PublicKey, error)
}

// InstanceState is the part of instance.State the tokens read: the instance_id, the issuer.
type InstanceState interface {
	Load(ctx context.Context) (instance.Record, error)
}

// IDSource issues UUIDv7 values (kernel/ids.Generator).
type IDSource interface {
	New() ids.ID
}

// InvocationTokenDeps are the collaborators of InvocationTokens.
type InvocationTokenDeps struct {
	// Keys signs and resolves the keys of purpose invocation.
	Keys InvocationKeys
	// State loads the instance row once per process for iss.
	State InstanceState
	// Clock is the time of iat and of every check; nil means clock.System().
	Clock clock.Clock
	// IDs issues the jti of every token.
	IDs IDSource
}

// redacted is what an InvocationToken renders as.
const redacted = "[redacted]"

// InvocationToken is a signed invocation token (09 4.5). It renders as "[redacted]" through fmt,
// slog, encoding/json and encoding/text, so it never reaches a log, a trace, an audit entry, an
// event or an error by accident (SEC-LOG-2, SEC-SEC-3); only Reveal hands out the compact JWS,
// which the delivery (M2-08) calls once when it fills the envelope.
type InvocationToken struct {
	v *string
}

// Reveal returns the compact JWS; "" for the zero token.
func (t InvocationToken) Reveal() string {
	if t.v == nil {
		return ""
	}
	return *t.v
}

// String returns "[redacted]".
func (InvocationToken) String() string { return redacted }

// GoString returns "[redacted]".
func (InvocationToken) GoString() string { return redacted }

// Format writes "[redacted]" for every verb.
func (InvocationToken) Format(f fmt.State, _ rune) { _, _ = io.WriteString(f, redacted) }

// LogValue implements slog.LogValuer.
func (InvocationToken) LogValue() slog.Value { return slog.StringValue(redacted) }

// MarshalJSON returns the JSON string "[redacted]".
func (InvocationToken) MarshalJSON() ([]byte, error) { return []byte(`"` + redacted + `"`), nil }

// MarshalText returns "[redacted]".
func (InvocationToken) MarshalText() ([]byte, error) { return []byte(redacted), nil }

// IssuedInvocationToken is the result of Issue: the token, its jti for runs.invocation_jti, the
// kid of the signing key and the validity (ExpiresAt feeds InvocationTokenGrant.expires_at in
// M2-09).
type IssuedInvocationToken struct {
	Token               InvocationToken
	JTI                 ids.ID
	KeyID               string
	IssuedAt, ExpiresAt time.Time
}

// VerifyInvocation is what Verify compares a token with: the agent of the request (aud) and, when
// set, the run (run_id). IgnoreExpiry skips only the expiry (renewInvocationToken, 09 4.5.1 step
// 1; later SR9).
type VerifyInvocation struct {
	Audience     ids.ID
	RunID        ids.ID
	IgnoreExpiry bool
}

// InvocationTokens issues and verifies invocation tokens (invariant I3, 09 4.5, ADR 0023): compact
// JWS with alg EdDSA, the kid of the active key of purpose invocation and typ
// agent-invocation+jwt over the claims of 05 section 5. Nothing is stored: only the jti goes back
// to the caller. Safe for concurrent use.
type InvocationTokens struct {
	cfg   InvocationTokenConfig
	keys  InvocationKeys
	state InstanceState
	clk   clock.Clock
	ids   IDSource

	// mu guards iss: instance_id never changes (M1-15), so it is loaded once; a failed load is
	// retried on the next call.
	mu  sync.Mutex
	iss ids.ID
}

// NewInvocationTokens builds the service. Errors: a missing collaborator, a TTL below one second
// or a negative leeway.
//
// Example:
//
//	tokens, err := app.NewInvocationTokens(app.InvocationTokenConfig{TTL: ttl, ClockLeeway: leeway},
//		app.InvocationTokenDeps{Keys: keys, State: state, Clock: clk, IDs: ids.NewGenerator(clk, rand.Reader)})
func NewInvocationTokens(cfg InvocationTokenConfig, d InvocationTokenDeps) (*InvocationTokens, error) {
	if d.Keys == nil || d.State == nil || d.IDs == nil {
		return nil, errors.New("runs: the invocation tokens need the signing keys, the instance state and an ID source")
	}
	if cfg.TTL < time.Second || cfg.ClockLeeway < 0 {
		return nil, errors.New("runs: agent.invocation_token_ttl must be at least one second and auth.clock_leeway not negative")
	}
	if d.Clock == nil {
		d.Clock = clock.System()
	}
	return &InvocationTokens{cfg: cfg, keys: d.Keys, state: d.State, clk: d.Clock, ids: d.IDs}, nil
}

// issuer returns the instance_id, loading it on the first call that succeeds.
func (s *InvocationTokens) issuer(ctx context.Context) (ids.ID, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.iss.IsZero() {
		return s.iss, nil
	}
	rec, err := s.state.Load(ctx)
	if err != nil {
		return ids.ID{}, err
	}
	if rec.InstanceID.IsZero() {
		return ids.ID{}, errs.New(errs.CodeInternalError, "runs: the instance row has no instance_id")
	}
	s.iss = rec.InstanceID
	return s.iss, nil
}

// Issue signs a new token for g (Design note 4 of M2-06) with a new jti and now from the clock,
// iss from the instance row and the active key of purpose invocation, read fresh, so a promotion
// in another process takes effect on the next token. It starts no transaction and takes no lock
// and may run in the caller's unit of work (M2-08). Errors: kind internal for a grant that breaks
// the table of 05 section 5 or a signer that is not EdDSA (nothing is signed); the errors of the
// instance row and of the key store with their kinds (unavailable when the store is unusable).
func (s *InvocationTokens) Issue(ctx context.Context, g domain.InvocationGrant) (IssuedInvocationToken, error) {
	jti := s.ids.New()
	c, err := domain.NewInvocationClaims(g, jti, s.clk.Now(), s.cfg.TTL)
	if err != nil {
		return IssuedInvocationToken{}, errs.Wrap(errs.CodeInternalError, err, "runs: incomplete invocation grant")
	}
	iss, err := s.issuer(ctx)
	if err != nil {
		return IssuedInvocationToken{}, err
	}
	payload, err := c.WithIssuer(iss).Payload()
	if err != nil {
		return IssuedInvocationToken{}, errs.Wrap(errs.CodeInternalError, err, "runs: invocation claims")
	}
	sg, err := s.keys.Signer(ctx, instance.PurposeInvocation)
	if err != nil {
		return IssuedInvocationToken{}, err
	}
	if sg.Alg() != domain.InvocationAlg {
		return IssuedInvocationToken{}, errs.New(errs.CodeInternalError, "runs: the signer of purpose invocation is not EdDSA")
	}
	sign := jose.SignerFunc{Algorithm: sg.Alg(), Fn: func(in []byte) ([]byte, error) { return sg.Sign(ctx, in) }}
	raw, err := jose.SignCompact(sign, jose.Header{Alg: domain.InvocationAlg, Kid: sg.KeyID(), Typ: domain.InvocationTyp}, payload)
	if err != nil {
		if _, ok := errs.As(err); ok {
			return IssuedInvocationToken{}, err
		}
		return IssuedInvocationToken{}, errs.Wrap(errs.CodeInternalError, err, "runs: signing an invocation token")
	}
	return IssuedInvocationToken{
		Token: InvocationToken{v: &raw}, JTI: jti, KeyID: sg.KeyID(),
		IssuedAt: time.Unix(c.Iat, 0).UTC(), ExpiresAt: time.Unix(c.Exp, 0).UTC(),
	}, nil
}

// invalid is the error of every failed check except the expiry; detail names the step only.
func invalid(detail string) error {
	return errs.New(errs.CodeInvocationTokenInvalid, detail).WithReason("invalid")
}

// Verify checks token for startRun, resumeRun and renewInvocationToken (Design note 5 of M2-06,
// used by M2-09) in this order: header (alg, typ byte for byte, kid), the published key of
// purpose invocation that kid names (next, active or retiring; a retired key, a key of another
// purpose and an unknown kid fail), signature, payload, iss, aud and run, iat and exp after iat,
// and last, unless v.IgnoreExpiry, the expiry with auth.clock_leeway. Whether the jti is current or
// consumed is the caller's step after Verify (M2-09). It returns the claims. Errors: 403
// invocation_token_invalid with reason invalid for every failed check, with reason expired when
// only exp has passed; the error of the key store when it cannot be read (no verdict about the
// token). No error contains the token or a segment of it.
func (s *InvocationTokens) Verify(ctx context.Context, token string, v VerifyInvocation) (domain.InvocationClaims, error) {
	p, err := domain.ParseInvocationToken(token)
	if err != nil {
		return domain.InvocationClaims{}, invalid("runs: invocation token header")
	}
	key, err := s.keys.VerificationKey(ctx, instance.PurposeInvocation, p.KeyID())
	if errs.KindOf(err) == errs.KindNotFound {
		return domain.InvocationClaims{}, invalid("runs: invocation token key unknown")
	}
	if err != nil {
		return domain.InvocationClaims{}, err
	}
	if key.Purpose != instance.PurposeInvocation || key.Alg != domain.InvocationAlg || key.State == instance.KeyRetired {
		return domain.InvocationClaims{}, invalid("runs: invocation token key unknown")
	}
	jwk, err := jose.JWKFromPublicKey(key.Key)
	if err != nil {
		return domain.InvocationClaims{}, invalid("runs: invocation token key unusable")
	}
	if err := p.Verify(jwk); err != nil {
		return domain.InvocationClaims{}, invalid("runs: invocation token signature")
	}
	c, err := p.Claims()
	if err != nil {
		return domain.InvocationClaims{}, invalid("runs: invocation token claims")
	}
	iss, err := s.issuer(ctx)
	if err != nil {
		return domain.InvocationClaims{}, err
	}
	err = domain.CheckInvocationClaims(c, domain.InvocationExpectation{Issuer: iss, Audience: v.Audience, RunID: v.RunID,
		IgnoreExpiry: v.IgnoreExpiry}, s.clk.Now(), s.cfg.ClockLeeway)
	switch {
	case errors.Is(err, domain.ErrInvocationExpired):
		return domain.InvocationClaims{}, errs.New(errs.CodeInvocationTokenInvalid, "runs: invocation token expired").WithReason("expired")
	case err != nil:
		return domain.InvocationClaims{}, invalid("runs: invocation token iss, aud, run or time")
	}
	return c, nil
}
